package business

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.com/openkcm/session-manager/internal/business/server"
	"github.com/openkcm/session-manager/internal/config"
	oidcsql "github.com/openkcm/session-manager/internal/oidc/sql"
	"github.com/openkcm/session-manager/internal/session"
	sessionsql "github.com/openkcm/session-manager/internal/session/sql"
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

	oidcProviderRepo := oidcsql.NewRepository(db)
	sessionRepo := sessionsql.NewRepository(db)

	clientID, err := commoncfg.LoadValueFromSourceRef(cfg.SessionManager.ClientID)
	if err != nil {
		return fmt.Errorf("readign client id from source ref: %w", err)
	}

	sessionManager := session.NewManager(oidcProviderRepo, sessionRepo, cfg.SessionManager.SessionDuration, cfg.SessionManager.RedirectURI, string(clientID))

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
