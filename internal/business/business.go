package business

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	sessionManager, closeFn, err := initSessionManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialising the session manager: %w", err)
	}

	defer closeFn()

	return server.StartHTTPServer(ctx, cfg, sessionManager)
}

func TokenRefresherMain(ctx context.Context, cfg *config.Config) error {
	sessionManager, closeFn, err := initSessionManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialising the session manager: %w", err)
	}

	defer closeFn()

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

func initSessionManager(ctx context.Context, cfg *config.Config) (_ *session.Manager, closeFn func(), _ error) {
	connStr, err := config.MakeConnStr(cfg.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("making dsn from config: %w", err)
	}

	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, nil, fmt.Errorf("initialising pgxpool connection: %w", err)
	}

	valkeyHost, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.Host)
	if err != nil {
		return nil, nil, fmt.Errorf("loading valkey host: %w", err)
	}

	valkeyUsername, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.User)
	if err != nil {
		return nil, nil, fmt.Errorf("loading valkey username: %w", err)
	}

	valkeyPassword, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("loading valkey password: %w", err)
	}

	valkeyOpts := valkey.ClientOption{
		InitAddress: []string{string(valkeyHost)},
		Username:    string(valkeyUsername),
		Password:    string(valkeyPassword),
	}

	if cfg.ValKey.SecretRef.Type == commoncfg.MTLSSecretType {
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.ValKey.SecretRef.MTLS)
		if err != nil {
			return nil, nil, fmt.Errorf("loading valkey mTLS config from secret ref: %w", err)
		}

		valkeyOpts.TLSConfig = tlsConfig
	}

	valkeyClient, err := valkey.NewClient(valkeyOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("creating a new valkey client: %w", err)
	}

	oidcProviderRepo := oidcsql.NewRepository(db)
	sessionRepo := sessionvalkey.NewRepository(valkeyClient, cfg.ValKey.Prefix)
	httpClient, err := loadHTTPClient(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("loading http client: %w", err)
	}

	auditLogger, err := otlpaudit.NewLogger(&cfg.Audit)
	if err != nil {
		return nil, nil, fmt.Errorf("creating audit logger: %w", err)
	}

	sessManager, err := session.NewManager(
		&cfg.SessionManager,
		oidcProviderRepo,
		sessionRepo,
		auditLogger,
		httpClient,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating session manager: %w", err)
	}

	return sessManager, valkeyClient.Close, nil
}

func loadHTTPClient(cfg *config.Config) (*http.Client, error) {
	clientID := cfg.SessionManager.ClientAuth.ClientID

	switch cfg.SessionManager.ClientAuth.Type {
	case "mtls":
		tlsConfig, err := commoncfg.LoadMTLSConfig(cfg.SessionManager.ClientAuth.MTLS)
		if err != nil {
			return nil, fmt.Errorf("loading mTLS config: %w", err)
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
			return nil, fmt.Errorf("loading client secret: %w", err)
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

	return t.next.RoundTrip(req)
}
