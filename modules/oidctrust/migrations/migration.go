package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"math"

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
	DBModule string `yaml:"dbModule" default:"database.module.pgxpool" dep:"sessionmanager.Database"`

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

func (m *MigrationModule) ValidateSchema(ctx context.Context) error {
	goose.SetBaseFS(FS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	return validateSchema(ctx, m.db.STDAdapter(), maxMigrationVersion, goose.GetDBVersionContext)
}

func maxMigrationVersion() (int64, error) {
	available, err := goose.CollectMigrations(".", 0, math.MaxInt64)
	if err != nil {
		return 0, fmt.Errorf("collecting available migrations: %w", err)
	}
	if len(available) == 0 {
		return 0, fmt.Errorf("no migrations found")
	}
	return available[len(available)-1].Version, nil
}

func validateSchema(
	ctx context.Context,
	db *sql.DB,
	maxVersion func() (int64, error),
	getVersion func(context.Context, *sql.DB) (int64, error),
) error {
	required, err := maxVersion()
	if err != nil {
		return err
	}

	current, err := getVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("reading DB schema version: %w", err)
	}

	if current != required {
		return fmt.Errorf("DB schema version mismatch: required %d, got %d (run 'session-manager migrate' to update)", required, current)
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
	db, err := sessionmanager.GetModuleAs[sessionmanager.Database](ctx, m.DBModule)
	if err != nil {
		return fmt.Errorf("getting postgres module: %w", err)
	}

	m.db = db
	return nil
}
