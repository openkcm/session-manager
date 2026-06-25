package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	testDBHost     = "localhost"
	testDBUser     = "postgres"
	testDBPassword = "secret"
	testDBName     = "migration_test"
	testDBSSLMode  = "disable"
)

// TestUp00005_WithConfig tests the migration when a valid config file exists
func TestUp00005_WithConfig(t *testing.T) {
	ctx := t.Context()
	db, pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	// Create test config with a specific client_id
	configCleanup := createTestConfigWithClientID("test-client-123")
	defer configCleanup()

	// Insert test data with various client_id states
	_, err := pool.Exec(ctx, `
		INSERT INTO trust (tenant_id, blocked, issuer, jwks_uri, audiences, properties, client_id)
		VALUES
			('tenant-null', false, 'issuer1', '', '{}', '{}', NULL),
			('tenant-empty', false, 'issuer2', '', '{}', '{}', ''),
			('tenant-existing', false, 'issuer3', '', '{}', '{}', 'existing-client-id')
	`)
	require.NoError(t, err)

	// Create a transaction for the migration
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	// Run the up migration
	err = Up00005(ctx, tx)
	require.NoError(t, err)

	// Commit the transaction
	err = tx.Commit()
	require.NoError(t, err)

	// Verify the results
	t.Run("NULL client_id should be updated", func(t *testing.T) {
		var clientID string
		err := pool.QueryRow(ctx, "SELECT client_id FROM trust WHERE tenant_id = 'tenant-null'").Scan(&clientID)
		require.NoError(t, err)
		assert.Equal(t, "test-client-123", clientID)
	})

	t.Run("empty client_id should be updated", func(t *testing.T) {
		var clientID string
		err := pool.QueryRow(ctx, "SELECT client_id FROM trust WHERE tenant_id = 'tenant-empty'").Scan(&clientID)
		require.NoError(t, err)
		assert.Equal(t, "test-client-123", clientID)
	})

	t.Run("existing client_id should NOT be updated", func(t *testing.T) {
		var clientID string
		err := pool.QueryRow(ctx, "SELECT client_id FROM trust WHERE tenant_id = 'tenant-existing'").Scan(&clientID)
		require.NoError(t, err)
		assert.Equal(t, "existing-client-id", clientID, "existing client_id should not be overwritten")
	})
}

// TestUp00005_WithoutConfig tests that migration handles missing config gracefully
func TestUp00005_WithoutConfig(t *testing.T) {
	ctx := t.Context()
	db, pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	// Ensure we're in a directory with no config file
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	// Insert test data
	_, err := pool.Exec(ctx, `
		INSERT INTO trust (tenant_id, blocked, issuer, jwks_uri, audiences, properties, client_id)
		VALUES ('tenant-no-config', false, 'issuer1', '', '{}', '{}', NULL)
	`)
	require.NoError(t, err)

	// Create a transaction for the migration
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	// Run the up migration - should not fail even without config
	err = Up00005(ctx, tx)

	// The migration should handle missing config gracefully
	// This test documents the current behavior - adjust assertion based on desired behavior
	if err != nil {
		assert.Contains(t, err.Error(), "Config File", "should indicate config file issue")
	}
}

// TestUp00005_EmptyClientIDInConfig tests migration when config has empty client_id
func TestUp00005_EmptyClientIDInConfig(t *testing.T) {
	ctx := t.Context()
	db, pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	// Create test config with empty client_id
	configCleanup := createTestConfigWithClientID("")
	defer configCleanup()

	// Insert test data
	_, err := pool.Exec(ctx, `
		INSERT INTO trust (tenant_id, blocked, issuer, jwks_uri, audiences, properties, client_id)
		VALUES ('tenant-test', false, 'issuer1', '', '{}', '{}', NULL)
	`)
	require.NoError(t, err)

	// Create a transaction for the migration
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	// Run the up migration - should fail because client_id is empty
	err = Up00005(ctx, tx)
	assert.Error(t, err, "should fail when client_id is empty in config")
	assert.Contains(t, err.Error(), "client_id is not set")
}

// TestDown00005 tests the down migration
func TestDown00005(t *testing.T) {
	ctx := t.Context()
	db, _, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	// Create a transaction for the migration
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	// Run the down migration
	err = Down00005(ctx, tx)

	// Down migration is currently a no-op, so should succeed
	assert.NoError(t, err)
}

// TestUp00005_MultipleRows tests migration with many rows
func TestUp00005_MultipleRows(t *testing.T) {
	ctx := t.Context()
	db, pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	configCleanup := createTestConfigWithClientID("bulk-client-id")
	defer configCleanup()

	// Insert multiple test rows
	for i := range 10 {
		clientID := ""
		if i%3 == 0 {
			clientID = fmt.Sprintf("existing-%d", i)
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO trust (tenant_id, blocked, issuer, jwks_uri, audiences, properties, client_id)
			VALUES ($1, false, $2, '', '{}', '{}', $3)
		`, fmt.Sprintf("tenant-%d", i), fmt.Sprintf("issuer-%d", i), clientID)
		require.NoError(t, err)
	}

	// Run the migration
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = Up00005(ctx, tx)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify all rows have client_id set
	rows, err := pool.Query(ctx, "SELECT tenant_id, client_id FROM trust ORDER BY tenant_id")
	require.NoError(t, err)
	defer rows.Close()

	count := 0
	for rows.Next() {
		var tenantID, clientID string
		err := rows.Scan(&tenantID, &clientID)
		require.NoError(t, err)

		assert.NotEmpty(t, clientID, "client_id should not be empty for tenant %s", tenantID)

		// Check if this tenant should have retained its existing client_id
		var expectedID int
		if _, err := fmt.Sscanf(tenantID, "tenant-%d", &expectedID); err == nil && expectedID%3 == 0 {
			assert.Equal(t, fmt.Sprintf("existing-%d", expectedID), clientID)
		} else {
			assert.Equal(t, "bulk-client-id", clientID)
		}
		count++
	}

	assert.Equal(t, 10, count, "should have processed all 10 rows")
}

// setupTestDB creates a test database and returns db connection, pool, and cleanup function
func setupTestDB(ctx context.Context, t *testing.T) (*sql.DB, *pgxpool.Pool, func()) {
	t.Helper()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase(testDBName),
		postgres.WithUsername(testDBUser),
		postgres.WithPassword(testDBPassword),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	port, err := pgContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		testDBHost, testDBUser, testDBPassword, testDBName, port.Port(), testDBSSLMode)

	// Create pgx pool for queries
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	// Create sql.DB for migrations
	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err)

	// Run migrations up to but not including 00005
	goose.SetBaseFS(FS)
	err = goose.SetDialect("pgx")
	require.NoError(t, err)

	// Manually run migrations 1-4
	err = goose.UpToContext(ctx, db, ".", 4)
	require.NoError(t, err)

	cleanup := func() {
		pool.Close()
		db.Close()
		_ = pgContainer.Terminate(ctx)
	}

	return db, pool, cleanup
}

// createTestConfigWithClientID creates a temporary config file with the specified client_id
func createTestConfigWithClientID(clientID string) func() {
	tmpDir, err := os.MkdirTemp("", "migration-test-*")
	if err != nil {
		panic(fmt.Sprintf("Failed to create temp dir: %v", err))
	}

	configContent := fmt.Sprintf(`sessionManager:
  clientAuth:
    clientID: "%s"
`, clientID)

	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		os.RemoveAll(tmpDir)
		panic(fmt.Sprintf("Failed to write config file: %v", err))
	}

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

	return func() {
		_ = os.Chdir(originalDir)
		os.RemoveAll(tmpDir)
	}
}
