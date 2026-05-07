package business

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/valkey-io/valkey-go"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/business/server"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
	"github.com/openkcm/session-manager/internal/grpc"
	"github.com/openkcm/session-manager/internal/session"
	sessionvalkey "github.com/openkcm/session-manager/internal/session/valkey"
)

const (
	insecure         = "insecure"
	mtls             = "mtls"
	clientSecret     = "client_secret" // An alias to clientSecretPost. Prefer using clientSecretPost.
	clientSecretPost = "client_secret_post"
)

// Main starts both API servers
func Main(ctx context.Context, cfg *config.Config) error {
	c, cancelCause := sessionmanager.NewContext(ctx)
	defer cancelCause(nil)

	if _, err := c.LoadModule(&cfg.Database); err != nil {
		return fmt.Errorf("loading database module: %w", err)
	}

	if _, err := c.LoadModule(&cfg.Trust); err != nil {
		return fmt.Errorf("loading trust module: %w", err)
	}

	// errChan is used to capture the first error and shutdown the servers.
	errChan := make(chan error, 1)

	// wg is used to wait for all servers to shutdown.
	var wg sync.WaitGroup

	// start public HTTP REST API server
	wg.Go(func() {
		errChan <- publicMain(c, cfg)
	})

	// start internal gRPC API server
	wg.Go(func() {
		errChan <- internalMain(c, cfg)
	})

	err := <-errChan
	if err != nil {
		slogctx.Error(ctx, "Shutting down servers", "error", err)
	}
	cancelCause(err)

	// wait for all servers to shutdown
	wg.Wait()

	return err
}

// publicMain starts the HTTP REST public API server.
func publicMain(ctx *sessionmanager.Context, cfg *config.Config) error {
	csrfSecret, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.CSRFSecret)
	if err != nil {
		return fmt.Errorf("loading csrf token from source ref: %w", err)
	}
	if len(csrfSecret) < 32 {
		return errors.New("CSRF secret must be at least 32 bytes")
	}

	cfg.SessionManager.CSRFSecretParsed = csrfSecret

	trustMod, err := ctx.GetModule(cfg.Trust.Module())
	if err != nil {
		return fmt.Errorf("getting trust module: %w", err)
	}

	//nolint:forcetypeassert
	trust := trustMod.(sessionmanager.Trust)

	sessionManager, closeFn, err := initSessionManager(ctx, cfg, trust)
	if err != nil {
		return fmt.Errorf("failed to initialise the session manager: %w", err)
	}

	defer closeFn()

	return server.StartHTTPServer(ctx, cfg, sessionManager)
}

// internalMain starts the gRPC private API server.
func internalMain(ctx *sessionmanager.Context, cfg *config.Config) error {
	// Create session repository
	valkeyClient, err := valkeyClientFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create valkey client: %w", err)
	}
	defer valkeyClient.Close()
	sessionRepo := sessionvalkey.NewRepository(valkeyClient, cfg.ValKey.Prefix)

	credsBuilder, err := newCredsBuilder(cfg)
	if err != nil {
		return fmt.Errorf("failed to create a credentials builder: %w", err)
	}

	trustMod, err := ctx.GetModule(cfg.Trust.Module())
	if err != nil {
		return fmt.Errorf("getting trust module: %w", err)
	}

	//nolint:forcetypeassert
	trust := trustMod.(sessionmanager.Trust)

	// Initialize the gRPC servers.
	oidcmappingsrv := grpc.NewTrustMappingServer(trust)
	sessionsrv := grpc.NewSessionServer(ctx,
		sessionRepo,
		trust,
		cfg.SessionManager.IdleSessionTimeout,
		cfg.SessionManager.ClientAuth.ClientID,
		grpc.WithTransportCredentials(credsBuilder),
	)

	return server.StartGRPCServer(ctx, cfg, oidcmappingsrv, sessionsrv)
}

func initSessionManager(ctx context.Context, cfg *config.Config, trust sessionmanager.Trust) (_ *session.Manager, closeFn func(), _ error) {
	// Create session repository
	valkeyClient, err := valkeyClientFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create valkey client: %w", err)
	}
	sessionRepo := sessionvalkey.NewRepository(valkeyClient, cfg.ValKey.Prefix)

	credsBuilder, err := newCredsBuilder(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load http client: %w", err)
	}

	auditLogger, err := otlpaudit.NewLogger(&cfg.Audit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	sessManager, err := session.NewManager(ctx,
		&cfg.SessionManager,
		trust,
		sessionRepo,
		auditLogger,
		session.WithTransportCredentials(credsBuilder),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return sessManager, valkeyClient.Close, nil
}

func valkeyClientFromConfig(cfg *config.Config) (valkey.Client, error) {
	valkeyHost, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to load valkey host: %w", err)
	}

	valkeyUsername, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.User)
	if err != nil {
		return nil, fmt.Errorf("failed to load valkey username: %w", err)
	}

	valkeyPassword, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to load valkey password: %w", err)
	}

	valkeyOpts := valkey.ClientOption{
		InitAddress: []string{string(valkeyHost)},
		Username:    string(valkeyUsername),
		Password:    string(valkeyPassword),
	}

	if cfg.ValKey.SecretRef.Type == commoncfg.MTLSSecretType {
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.ValKey.SecretRef.MTLS)
		if err != nil {
			return nil, fmt.Errorf("failed to load valkey mTLS config from secret ref: %w", err)
		}

		valkeyOpts.TLSConfig = tlsConfig
	}

	valkeyClient, err := valkey.NewClient(valkeyOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new valkey client: %w", err)
	}
	return valkeyClient, nil
}

func newCredsBuilder(cfg *config.Config) (credentials.Builder, error) {
	switch cfg.SessionManager.ClientAuth.Type {
	case mtls:
		tlsConfig, err := commoncfg.LoadMTLSConfig(cfg.SessionManager.ClientAuth.MTLS)
		if err != nil {
			return nil, fmt.Errorf("failed to load mTLS config: %w", err)
		}

		return func(clientID string) credentials.TransportCredentials { return credentials.NewTLS(clientID, tlsConfig) }, nil
	case clientSecretPost, clientSecret:
		secret, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.ClientAuth.ClientSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to load client secret: %w", err)
		}

		return func(clientID string) credentials.TransportCredentials {
			return credentials.NewClientSecretPost(clientID, string(secret))
		}, nil
	case insecure:
		slog.Warn("insecure credentials are used. Do not use this in production")
		return func(clientID string) credentials.TransportCredentials { return credentials.NewInsecure(clientID) }, nil
	default:
		return nil, errors.New("unknown Client Auth type")
	}
}
