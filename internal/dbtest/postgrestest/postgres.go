package postgrestest

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"

	slogctx "github.com/veqryn/slog-context"

	migrations "github.com/openkcm/session-manager/sql"
)

const (
	DBHost     = "localhost"
	DBUser     = "postgres"
	DBPassword = "secret"
	DBName     = "session_manager"
	DBSSLMode  = "disable"
)

// ExpiryTime is the time used as "expiry" for the inserted data
var ExpiryTime time.Time

func init() {
	// initialise time without monotonic time
	now := time.Now()
	ExpiryTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(30 * 24 * time.Hour).Truncate(0)
}

// Start initialises a database instance and returns a connection pool, database port, and termination function.
//
// Database credentials are available as exported variables.
// The database contains pre-defined test data. See INSERT statements in the prepareDB.
func Start(ctx context.Context) (*pgxpool.Pool, nat.Port, func(ctx context.Context)) {
	pgContainer, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase(DBName),
		postgres.WithUsername(DBUser),
		postgres.WithPassword(DBPassword),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		slogctx.Error(ctx, "Failed to start PostgreSQL", slog.String("error", err.Error()))
		panic(err)
	}

	port, err := pgContainer.MappedPort(ctx, nat.Port("5432"))
	if err != nil {
		slogctx.Error(ctx, "Failed to get mapped port for the PosgtgreSQL container", slog.String("error", err.Error()))
		panic(err)
	}

	dbPool := makeDBConn(ctx, port)
	prepareDB(ctx, dbPool, port)

	terminate := func(ctx context.Context) {
		if err := pgContainer.Terminate(ctx); err != nil {
			slogctx.Error(ctx, "Failed to terminate PosgtgreSQL container", slog.String("error", err.Error()))
			panic(err)
		}
	}

	return dbPool, port, terminate
}

func connStr(port nat.Port) string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", DBHost, DBUser, DBPassword, DBName, port.Port(), DBSSLMode)
}

func makeDBConn(ctx context.Context, port nat.Port) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, connStr(port))
	if err != nil {
		panic(err)
	}

	return pool
}

func migrateDB(ctx context.Context, port nat.Port) {
	db, err := sql.Open("pgx", connStr(port))
	if err != nil {
		panic(err)
	}

	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("pgx"); err != nil {
		panic(err)
	}

	if err := goose.UpContext(ctx, db, "."); err != nil {
		panic(err)
	}
}

func prepareDB(ctx context.Context, dbPool *pgxpool.Pool, port nat.Port) {
	migrateDB(ctx, port)

	b := new(pgx.Batch)
	b.Queue(`INSERT INTO oidc_providers (issuer_url) VALUES ('url-one');`)
	b.Queue(`SELECT set_config('app.tenant_id', 'tenant1-id', false);`)
	b.Queue(`INSERT INTO oidc_provider_map (tenant_id, issuer_url) VALUES (current_setting('app.tenant_id'), 'url-one');`)
	b.Queue(`INSERT INTO oidc_providers (issuer_url) VALUES ('url-two');`)
	b.Queue(`SELECT set_config('app.tenant_id', 'tenant2-id', false);`)
	b.Queue(`INSERT INTO oidc_provider_map (tenant_id, issuer_url) VALUES (current_setting('app.tenant_id'), 'url-two');`)
	b.Queue(`INSERT INTO oidc_providers (issuer_url) VALUES ('url-three');`)
	b.Queue(`SELECT set_config('app.tenant_id', 'tenant3-id', false);`)
	b.Queue(`INSERT INTO oidc_provider_map (tenant_id, issuer_url) VALUES (current_setting('app.tenant_id'), 'url-three');`)

	res := dbPool.SendBatch(ctx, b)
	if err := res.Close(); err != nil {
		panic(err)
	}
}
