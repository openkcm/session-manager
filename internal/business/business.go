package business

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/valkey-io/valkey-go"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/business/server"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/grpc"
	"github.com/openkcm/session-manager/internal/session"
	sessionvalkey "github.com/openkcm/session-manager/internal/session/valkey"
	"github.com/openkcm/session-manager/internal/trust"
	"github.com/openkcm/session-manager/internal/trust/trustsql"
)

// Main starts both API servers
func Main(ctx context.Context, cfg *config.Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// errChan is used to capture the first error and shutdown the servers.
	errChan := make(chan error, 1)

	// wg is used to wait for all servers to shutdown.
	var wg sync.WaitGroup

	// start public HTTP REST API server
	wg.Go(func() {
		errChan <- publicMain(ctx, cfg)
	})

	// start internal gRPC API server
	wg.Go(func() {
		errChan <- internalMain(ctx, cfg)
	})

	// wait for any error to initiate the shutdown
	err := <-errChan
	if err != nil {
		slogctx.Error(ctx, "Shutting down servers", "error", err)
	}
	cancel()

	// wait for all servers to shutdown
	wg.Wait()

	return nil
}

// publicMain starts the HTTP REST public API server.
func publicMain(ctx context.Context, cfg *config.Config) error {
	csrfSecret, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.CSRFSecret)
	if err != nil {
		return fmt.Errorf("loading csrf token from source ref: %w", err)
	}
	if len(csrfSecret) < 32 {
		return errors.New("CSRF secret must be at least 32 bytes")
	}

	cfg.SessionManager.CSRFSecretParsed = csrfSecret

	sessionManager, closeFn, err := initSessionManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialise the session manager: %w", err)
	}

	defer closeFn()

	return server.StartHTTPServer(ctx, cfg, sessionManager)
}

// internalMain starts the gRPC private API server.
func internalMain(ctx context.Context, cfg *config.Config) error {
	// Create OIDC service
	oidcProviderRepo, err := oidcProviderRepoFromConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create OIDC service: %w", err)
	}
	oidcService := trust.NewService(oidcProviderRepo)

	// Create session repository
	valkeyClient, err := valkeyClientFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create valkey client: %w", err)
	}
	defer valkeyClient.Close()
	sessionRepo := sessionvalkey.NewRepository(valkeyClient, cfg.ValKey.Prefix)

	// Create HTTP client
	httpClient, err := loadHTTPClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to load http client: %w", err)
	}

	// Initialize the gRPC servers.
	oidcmappingsrv := grpc.NewOIDCMappingServer(oidcService)
	opts := []grpc.SessionServerOption{
		grpc.WithQueryParametersIntrospect(cfg.SessionManager.AdditionalQueryParametersIntrospect),
	}
	sessionsrv := grpc.NewSessionServer(sessionRepo, oidcProviderRepo, httpClient, cfg.SessionManager.IdleSessionTimeout, opts...)
	return server.StartGRPCServer(ctx, cfg, oidcmappingsrv, sessionsrv)
}

func initSessionManager(ctx context.Context, cfg *config.Config) (_ *session.Manager, closeFn func(), _ error) {
	// Create OIDC provider repository
	oidcProviderRepo, err := oidcProviderRepoFromConfig(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OIDC service: %w", err)
	}

	// Create session repository
	valkeyClient, err := valkeyClientFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create valkey client: %w", err)
	}
	sessionRepo := sessionvalkey.NewRepository(valkeyClient, cfg.ValKey.Prefix)

	// Create HTTP client
	httpClient, err := loadHTTPClient(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load http client: %w", err)
	}

	auditLogger, err := otlpaudit.NewLogger(&cfg.Audit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	sessManager, err := session.NewManager(
		&cfg.SessionManager,
		oidcProviderRepo,
		sessionRepo,
		auditLogger,
		httpClient,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return sessManager, valkeyClient.Close, nil
}

func oidcProviderRepoFromConfig(ctx context.Context, cfg *config.Config) (*trustsql.Repository, error) {
	connStr, err := config.MakeConnStr(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to make dsn from config: %w", err)
	}

	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise pgxpool connection: %w", err)
	}

	return trustsql.NewRepository(db), nil
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

func loadHTTPClient(cfg *config.Config) (*http.Client, error) {
	clientID := cfg.SessionManager.ClientAuth.ClientID

	switch cfg.SessionManager.ClientAuth.Type {
	case "mtls":
		tlsConfig, err := commoncfg.LoadMTLSConfig(cfg.SessionManager.ClientAuth.MTLS)
		if err != nil {
			return nil, fmt.Errorf("failed to load mTLS config: %w", err)
		}

		return &http.Client{
			Transport: &clientAuthRoundTripper{
				clientID: clientID,
				next: &http.Transport{
					TLSClientConfig: tlsConfig,
				},
			},
		}, nil
	case "client_secret":
		secret, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.ClientAuth.ClientSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to load client secret: %w", err)
		}

		return &http.Client{
			Transport: &clientAuthRoundTripper{
				clientID:     clientID,
				clientSecret: string(secret),
				next:         http.DefaultTransport,
			},
		}, nil
	case "insecure":
		return http.DefaultClient, nil
	default:
		return nil, errors.New("unknown Client Auth type")
	}
}

type clientAuthRoundTripper struct {
	clientID     string
	clientSecret string
	next         http.RoundTripper
}

func (t *clientAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	q := req.URL.Query()

	q.Set("client_id", t.clientID)
	if t.clientSecret != "" {
		q.Set("client_secret", t.clientSecret)
	}
	req.URL.RawQuery = q.Encode()

	return t.next.RoundTrip(req)
}
