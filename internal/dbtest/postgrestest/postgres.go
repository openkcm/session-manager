package postgrestest

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moby/moby/api/types/network"
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
func Start(ctx context.Context) (*pgxpool.Pool, network.Port, func(ctx context.Context)) {
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

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		slogctx.Error(ctx, "Failed to get mapped port for the PosgtgreSQL container", slog.String("error", err.Error()))
		panic(err)
	}

	dbPool := makeDBConn(ctx, port)
	prepareDB(ctx, dbPool, port)

	terminate := func(ctx context.Context) {
		err := pgContainer.Terminate(ctx)
		if err != nil {
			slogctx.Error(ctx, "Failed to terminate PosgtgreSQL container", slog.String("error", err.Error()))
			panic(err)
		}
	}

	return dbPool, port, terminate
}

func connStr(port network.Port) string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", DBHost, DBUser, DBPassword, DBName, port.Port(), DBSSLMode)
}

func makeDBConn(ctx context.Context, port network.Port) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, connStr(port))
	if err != nil {
		panic(err)
	}

	return pool
}

func migrateDB(ctx context.Context, port network.Port) {
	// Create a test config file for the migration
	configCleanup := createTestConfig()
	defer configCleanup()

	db, err := sql.Open("pgx", connStr(port))
	if err != nil {
		panic(err)
	}

	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	err = goose.SetDialect("pgx")
	if err != nil {
		panic(err)
	}

	err = goose.UpContext(ctx, db, ".")
	if err != nil {
		panic(err)
	}
}

func prepareDB(ctx context.Context, dbPool *pgxpool.Pool, port network.Port) {
	migrateDB(ctx, port)

	b := new(pgx.Batch)
	b.Queue(`INSERT INTO trust (tenant_id, blocked, issuer, jwks_uri, audiences, properties) VALUES ('tenant1-id', false, 'url-one', '', '{}', '{}');`)
	b.Queue(`INSERT INTO trust (tenant_id, blocked, issuer, jwks_uri, audiences, properties) VALUES ('tenant2-id', false, 'url-two', '', '{}', '{}');`)
	b.Queue(`INSERT INTO trust (tenant_id, blocked, issuer, jwks_uri, audiences, properties) VALUES ('tenant3-id', false, 'url-three', '', '{}', '{}');`)

	res := dbPool.SendBatch(ctx, b)
	err := res.Close()
	if err != nil {
		panic(err)
	}
}

// createTestConfig creates a temporary config file for the migration tests.
// It returns a cleanup function that should be called to remove the config file.
func createTestConfig() func() {
	// Create a temporary directory for the config
	tmpDir, err := os.MkdirTemp("", "session-manager-test-*")
	if err != nil {
		panic(fmt.Sprintf("Failed to create temp dir: %v", err))
	}

	// Create config.yaml with test client_id
	configContent := `sessionManager:
  clientAuth:
    clientID: "test-client-id"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		os.RemoveAll(tmpDir)
		panic(fmt.Sprintf("Failed to write config file: %v", err))
	}

	// Change to the temp directory so the config loader can find it
	originalDir, err := os.Getwd()
	if err != nil {
		os.RemoveAll(tmpDir)
		panic(fmt.Sprintf("Failed to get current directory: %v", err))
	}

	err = os.Chdir(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		panic(fmt.Sprintf("Failed to change directory: %v", err))
	}

	// Return cleanup function
	return func() {
		_ = os.Chdir(originalDir)
		os.RemoveAll(tmpDir)
	}
}
