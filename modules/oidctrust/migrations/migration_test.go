package migrations

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSchema_VersionMatch(t *testing.T) {
	err := validateSchema(
		t.Context(),
		new(sql.DB),
		func() (int64, error) { return 6, nil },
		func(_ context.Context, _ *sql.DB) (int64, error) { return 6, nil },
	)
	require.NoError(t, err)
}

func TestValidateSchema_DBBehindRequired(t *testing.T) {
	err := validateSchema(
		t.Context(),
		new(sql.DB),
		func() (int64, error) { return 6, nil },
		func(_ context.Context, _ *sql.DB) (int64, error) { return 4, nil },
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required 6")
	assert.Contains(t, err.Error(), "got 4")
	assert.Contains(t, err.Error(), "session-manager migrate")
}

func TestValidateSchema_DBAheadOfRequired(t *testing.T) {
	err := validateSchema(
		t.Context(),
		new(sql.DB),
		func() (int64, error) { return 6, nil },
		func(_ context.Context, _ *sql.DB) (int64, error) { return 7, nil },
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required 6")
	assert.Contains(t, err.Error(), "got 7")
}

func TestValidateSchema_GetVersionError(t *testing.T) {
	dbErr := errors.New("connection refused")
	err := validateSchema(
		t.Context(),
		new(sql.DB),
		func() (int64, error) { return 6, nil },
		func(_ context.Context, _ *sql.DB) (int64, error) { return -1, dbErr },
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading DB schema version")
	assert.ErrorIs(t, err, dbErr)
}

func TestValidateSchema_MaxVersionError(t *testing.T) {
	maxErr := errors.New("no migrations found")
	err := validateSchema(
		t.Context(),
		new(sql.DB),
		func() (int64, error) { return 0, maxErr },
		func(_ context.Context, _ *sql.DB) (int64, error) { return 6, nil },
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, maxErr)
}

func TestMaxMigrationVersion_ReturnsLatest(t *testing.T) {
	// Uses the real embedded FS — verifies CollectMigrations sees all migrations
	// and that the returned version equals the highest numbered file (currently 6).
	version, err := maxMigrationVersion()
	require.NoError(t, err)
	assert.Equal(t, int64(6), version)
}
