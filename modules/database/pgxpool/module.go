package pgxpool

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	sessionmanager "github.com/openkcm/session-manager"
)

const moduleID = "database.module.pgxpool"

func newModule() sessionmanager.Module {
	return new(PostgresModule)
}

func init() {
	sessionmanager.RegisterModule(new(PostgresModule))
}

type PostgresModule struct {
	Mod      string              `yaml:"module"`
	Name     string              `yaml:"name"`
	Port     string              `yaml:"port"`
	Host     commoncfg.SourceRef `yaml:"host"`
	User     commoncfg.SourceRef `yaml:"user"`
	Password commoncfg.SourceRef `yaml:"password"`

	db *pgxpool.Pool
}

func (m *PostgresModule) STDAdapter() *sql.DB {
	return stdlib.OpenDBFromPool(m.db)
}

func (m *PostgresModule) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return m.db.Exec(ctx, sql, args...)
}

func (m *PostgresModule) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.db.Query(ctx, sql, args...)
}

func (m *PostgresModule) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return m.db.QueryRow(ctx, sql, args...)
}

func (m *PostgresModule) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  moduleID,
		New: newModule,
	}
}

func (m *PostgresModule) Provision(ctx *sessionmanager.Context) error {
	connStr, err := m.makeConnStr()
	if err != nil {
		return fmt.Errorf("making dsn from config: %w", err)
	}

	pgxpoolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("parsing pgxpool config: %w", err)
	}

	pgxpoolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	m.db, err = pgxpool.NewWithConfig(ctx, pgxpoolCfg)
	if err != nil {
		return fmt.Errorf("failed to initialise pgxpool connection: %w", err)
	}

	if err := otelpgx.RecordStats(m.db); err != nil {
		return fmt.Errorf("recording database stat: %w", err)
	}

	return nil
}

func (m *PostgresModule) makeConnStr() (string, error) {
	host, err := commoncfg.LoadValueFromSourceRef(m.Host)
	if err != nil {
		return "", fmt.Errorf("loading db host: %w", err)
	}

	user, err := commoncfg.LoadValueFromSourceRef(m.User)
	if err != nil {
		return "", fmt.Errorf("loading db user: %w", err)
	}

	password, err := commoncfg.LoadValueFromSourceRef(m.Password)
	if err != nil {
		return "", fmt.Errorf("loading db password: %w", err)
	}

	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s",
		host, user, string(password), m.Name, m.Port), nil
}

var _ sessionmanager.Database = (*PostgresModule)(nil)
