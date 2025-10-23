package business

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/valkey-io/valkey-go"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/business/server"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/grpc"
	"github.com/openkcm/session-manager/internal/oidc"
	oidcsql "github.com/openkcm/session-manager/internal/oidc/sql"
	"github.com/openkcm/session-manager/pkg/session"
	sessionvalkey "github.com/openkcm/session-manager/pkg/session/valkey"
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
	if err := <-errChan; err != nil {
		slogctx.Error(ctx, "Shutting down servers", "error", err)
	}
	cancel()

	// wait for all servers to shutdown
	wg.Wait()

	return nil
}

// publicMain starts the HTTP REST public API server.
func publicMain(ctx context.Context, cfg *config.Config) error {
	sessionManager, err := initSessionManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialising the session manager: %w", err)
	}

	return server.StartHTTPServer(ctx, cfg, sessionManager)
}

func TokenRefresherMain(ctx context.Context, cfg *config.Config) error {
	sessionManager, err := initSessionManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialising the session manager: %w", err)
	}

	slogctx.Info(ctx, "Starting token refresh job")
	return startTokenRefresher(ctx, sessionManager, cfg)
}

// internalMain starts the gRPC private API server.
func internalMain(ctx context.Context, cfg *config.Config) error {
	connStr, err := config.MakeConnStr(cfg.Database)
	if err != nil {
		return fmt.Errorf("making dsn from config: %w", err)
	}

	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return fmt.Errorf("initialising pgxpool connection: %w", err)
	}

	// Create the database repository.
	repo := oidcsql.NewRepository(db)
	service := oidc.NewService(repo)

	// Initialize the gRPC servers.
	oidcprovidersrv := grpc.NewOIDCProviderServer(service)
	oidcmappingsrv := grpc.NewOIDCMappingServer(service)
	return server.StartGRPCServer(ctx, cfg, oidcprovidersrv, oidcmappingsrv)
}

func startTokenRefresher(ctx context.Context, sessionManager *session.Manager, cfg *config.Config) error {
	c := time.Tick(cfg.TokenRefresher.RefreshInterval)
	for {
		slogctx.Info(ctx, "Triggering tokens refresh")
		if err := sessionManager.RefreshExpiringSessions(ctx); err != nil {
			slogctx.Error(ctx, "Failed to refresh tokens", "error", err)
		}

		select {
		case <-c:
			continue
		case <-ctx.Done():
			return nil
		}
	}
}

func initSessionManager(ctx context.Context, cfg *config.Config) (*session.Manager, error) {
	connStr, err := config.MakeConnStr(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("making dsn from config: %w", err)
	}

	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("initialising pgxpool connection: %w", err)
	}

	valkeyHost, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.Host)
	if err != nil {
		return nil, fmt.Errorf("loading valkey host: %w", err)
	}

	valkeyUsername, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.User)
	if err != nil {
		return nil, fmt.Errorf("loading valkey username: %w", err)
	}

	valkeyPassword, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.Password)
	if err != nil {
		return nil, fmt.Errorf("loading valkey password: %w", err)
	}

	valkeyOpts := valkey.ClientOption{
		InitAddress: []string{string(valkeyHost)},
		Username:    string(valkeyUsername),
		Password:    string(valkeyPassword),
	}

	if cfg.ValKey.SecretRef.Type == commoncfg.MTLSSecretType {
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.ValKey.SecretRef.MTLS)
		if err != nil {
			return nil, fmt.Errorf("loading valkey mTLS config from secret ref: %w", err)
		}

		valkeyOpts.TLSConfig = tlsConfig
	}

	valkeyClient, err := valkey.NewClient(valkeyOpts)
	if err != nil {
		return nil, fmt.Errorf("creating a new valkey client: %w", err)
	}

	defer valkeyClient.Close()

	oidcProviderRepo := oidcsql.NewRepository(db)
	sessionRepo := sessionvalkey.NewRepository(valkeyClient, cfg.ValKey.Prefix)

	clientID, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.ClientID)
	if err != nil {
		return nil, fmt.Errorf("reading client id from source ref: %w", err)
	}

	auditLogger, err := otlpaudit.NewLogger(&cfg.Audit)
	if err != nil {
		return nil, fmt.Errorf("creating audit logger: %w", err)
	}

	csrfSecret, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.CSRFSecret)
	if err != nil {
		return nil, fmt.Errorf("loading csrf token from source ref: %w", err)
	}

	if len(csrfSecret) < 32 {
		return nil, errors.New("CSRF secret must be at least 32 bytes")
	}

	return session.NewManager(
		oidcProviderRepo,
		sessionRepo,
		auditLogger,
		cfg.SessionManager.SessionDuration,
		cfg.SessionManager.RedirectURI,
		string(clientID),
		string(csrfSecret),
		cfg.SessionManager.JWSSigAlgs,
	), nil
}
