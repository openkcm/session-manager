package migrations

import (
	"context"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"

	sessionmanager "github.com/openkcm/session-manager"
)

//go:embed *.sql
var FS embed.FS

const moduleID = "trust.migration.module.oidc"

func newModule() sessionmanager.Module {
	return new(MigrationModule)
}

func init() {
	sessionmanager.RegisterModule(new(MigrationModule))
}

type MigrationModule struct {
	DBModule string `yaml:"dbModule" default:"database.module.pgxpool"`

	db sessionmanager.Database
}

func (m *MigrationModule) Migrate(ctx context.Context) error {
	goose.SetBaseFS(FS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, m.db.STDAdapter(), "."); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	return nil
}

func (m *MigrationModule) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  moduleID,
		New: newModule,
	}
}

func (m *MigrationModule) Provision(ctx *sessionmanager.Context) error {
	mod, err := ctx.GetModule(m.DBModule)
	if err != nil {
		return fmt.Errorf("getting postgres module: %w", err)
	}

	//nolint:forcetypeassert
	m.db = mod.(sessionmanager.Database)
	return nil
}
