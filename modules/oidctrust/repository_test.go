package oidctrust_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	trustv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/v1"

	"github.com/openkcm/session-manager/modules/oidctrust"
	sqltrust "github.com/openkcm/session-manager/modules/oidctrust/internal/sql"
	migrations "github.com/openkcm/session-manager/modules/oidctrust/migrations"
)

const (
	DBHost     = "localhost"
	DBUser     = "postgres"
	DBPassword = "secret"
	DBName     = "session_manager"
	DBSSLMode  = "disable"
)

type RepoWrapper struct {
	Repo       oidctrust.TrustRepository
	MockGet    func(ctx context.Context, tenantID string) (*trustv1.Trust, error)
	MockCreate func(ctx context.Context, trust *trustv1.Trust) error
	MockDelete func(ctx context.Context, tenantID string) error
	MockUpdate func(ctx context.Context, trust *trustv1.Trust) error
}

var _ oidctrust.TrustRepository = &RepoWrapper{}

// Create implements oidc.OIDCTrustRepository.
func (m *RepoWrapper) Create(ctx context.Context, trust *trustv1.Trust) error {
	if m.MockCreate != nil {
		err := m.MockCreate(ctx, trust)
		if err != nil {
			return err
		}
	}

	return m.Repo.Create(ctx, trust)
}

// Delete implements oidc.OIDCTrustRepository.
func (m *RepoWrapper) Delete(ctx context.Context, tenantID string) error {
	if m.MockDelete != nil {
		err := m.MockDelete(ctx, tenantID)
		if err != nil {
			return err
		}
	}

	return m.Repo.Delete(ctx, tenantID)
}

// Get implements oidc.OIDCTrustRepository.
func (m *RepoWrapper) Get(ctx context.Context, tenantID string) (*trustv1.Trust, error) {
	if m.MockGet != nil {
		mapping, err := m.MockGet(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		return mapping, nil
	}
	return m.Repo.Get(ctx, tenantID)
}

// Update implements oidc.OIDCTrustRepository.
func (m *RepoWrapper) Update(ctx context.Context, trust *trustv1.Trust) error {
	if m.MockUpdate != nil {
		err := m.MockUpdate(ctx, trust)
		if err != nil {
			return err
		}
	}
	return m.Repo.Update(ctx, trust)
}

func createRepo(ctx context.Context) (oidctrust.TrustRepository, error) {
	pgContainer, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase(DBName),
		postgres.WithUsername(DBUser),
		postgres.WithPassword(DBPassword),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, err
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, err
	}

	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", DBHost, DBUser, DBPassword, DBName, port.Port(), DBSSLMode)

	dbPool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		panic(err)
	}

	err = migrateDB(ctx, connStr)
	if err != nil {
		return nil, err
	}

	return sqltrust.NewRepository(&dbWrapper{dbPool}), nil
}

func migrateDB(ctx context.Context, connStr string) error {
	// Create a test config file for the migration
	configCleanup := createTestConfig()
	defer configCleanup()

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}

	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	err = goose.SetDialect("pgx")
	if err != nil {
		return err
	}

	err = goose.UpContext(ctx, db, ".")
	if err != nil {
		return err
	}
	return nil
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

type dbWrapper struct {
	*pgxpool.Pool
}

func (w *dbWrapper) STDAdapter() *sql.DB {
	return stdlib.OpenDBFromPool(w.Pool)
}
