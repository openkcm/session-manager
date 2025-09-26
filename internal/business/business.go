package business

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/valkey-io/valkey-go"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

	"github.com/openkcm/session-manager/internal/business/server"
	"github.com/openkcm/session-manager/internal/config"
	oidcsql "github.com/openkcm/session-manager/internal/oidc/sql"
	"github.com/openkcm/session-manager/pkg/session"
	sessionvalkey "github.com/openkcm/session-manager/pkg/session/valkey"
)

// PublicMain starts the HTTP REST public API server.
func PublicMain(ctx context.Context, cfg *config.Config) error {
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

	sessionManager := session.NewManager(
		oidcProviderRepo,
		sessionRepo,
		auditLogger,
		cfg.SessionManager.SessionDuration,
		cfg.SessionManager.RedirectURI,
		string(clientID),
	)

	return server.StartHTTPServer(ctx, cfg, sessionManager)
}

// InternalMain starts the gRPC private API server.
func InternalMain(ctx context.Context, cfg *config.Config) error {
	connStr, err := config.MakeConnStr(cfg.Database)
	if err != nil {
		return fmt.Errorf("making dsn from config: %w", err)
	}

	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return fmt.Errorf("initialising pgxpool connection: %w", err)
	}

	// TODO: Initialise the private API service.
	// oidcProviderRepo := oidcsql.NewRepository(db)
	_ = oidcsql.NewRepository(db)

	return server.StartGRPCServer(ctx, cfg)
}
