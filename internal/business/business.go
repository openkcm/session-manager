package business

import (
	"context"
	"fmt"
	"sync"

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
	connStr, err := config.MakeConnStr(cfg.Database)
	if err != nil {
		return fmt.Errorf("making dsn from config: %w", err)
	}

	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return fmt.Errorf("initialising pgxpool connection: %w", err)
	}

	valkeyHost, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.Host)
	if err != nil {
		return fmt.Errorf("loading valkey host: %w", err)
	}

	valkeyUsername, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.User)
	if err != nil {
		return fmt.Errorf("loading valkey username: %w", err)
	}

	valkeyPassword, err := commoncfg.LoadValueFromSourceRef(cfg.ValKey.Password)
	if err != nil {
		return fmt.Errorf("loading valkey password: %w", err)
	}

	valkeyClient, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{string(valkeyHost)},
		Username:    string(valkeyUsername),
		Password:    string(valkeyPassword),
	})
	if err != nil {
		return fmt.Errorf("creating a new valkey client: %w", err)
	}

	defer valkeyClient.Close()

	oidcProviderRepo := oidcsql.NewRepository(db)
	sessionRepo := sessionvalkey.NewRepository(valkeyClient, cfg.ValKey.Prefix)

	clientID, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.ClientID)
	if err != nil {
		return fmt.Errorf("reading client id from source ref: %w", err)
	}

	auditLogger, err := otlpaudit.NewLogger(&cfg.Audit)
	if err != nil {
		return fmt.Errorf("creating audit logger: %w", err)
	}

	if len(cfg.SessionManager.CSRFSecret) < 32 {
		return fmt.Errorf("sessionManager.csrfSecret must be at least 32 bytes")
	}

	sessionManager := session.NewManager(
		oidcProviderRepo,
		sessionRepo,
		auditLogger,
		cfg.SessionManager.SessionDuration,
		cfg.SessionManager.RedirectURI,
		string(clientID),
		cfg.SessionManager.CSRFSecret,
	)

	return server.StartHTTPServer(ctx, cfg, sessionManager)
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
