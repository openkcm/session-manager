package sessionsql_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/dbtest/postgrestest"
	"github.com/openkcm/session-manager/internal/session"
	sessionsql "github.com/openkcm/session-manager/internal/session/sql"
)

var dbPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pool, _, terminate := postgrestest.Start(ctx)
	defer terminate(ctx)

	dbPool = pool

	code := m.Run()
	os.Exit(code)
}

func TestRepository_LoadState(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		stateID   string
		wantState session.State
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:     "Select existing state",
			tenantID: "tenant1-id",
			stateID:  "stateid-one",
			wantState: session.State{
				ID:           "stateid-one",
				TenantID:     "tenant1-id",
				Fingerprint:  "fingerprint-one",
				PKCEVerifier: "verifier-one",
				RequestURI:   "http://localhost",
				Expiry:       postgrestest.DBTime,
			},
			assertErr: assert.NoError,
		},
		{
			name:      "Error does not exist",
			tenantID:  "does-not-exist",
			stateID:   "stateid-one",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := sessionsql.NewRepository(dbPool)

			gotState, err := r.LoadState(t.Context(), tt.tenantID, tt.stateID)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.LoadState() error %v", err)) || err != nil {
				return
			}

			assert.Equal(t, tt.wantState, gotState, "Repository.LoadState()")
		})
	}
}

func TestRepository_StoreState(t *testing.T) {
	const upsertTenantID = "tenant-id-upsert"

	upsertState := session.State{
		ID:           "stateid-to-upsert",
		TenantID:     upsertTenantID,
		Fingerprint:  "fingerprint-upsert",
		PKCEVerifier: "verifier",
		RequestURI:   "example.com",
		Expiry:       postgrestest.DBTime,
	}

	r := sessionsql.NewRepository(dbPool)
	err := r.StoreState(t.Context(), upsertTenantID, upsertState)
	require.NoError(t, err)

	tests := []struct {
		name      string
		tenantID  string
		state     session.State
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:     "Success",
			tenantID: "tenant-id-store-success",
			state: session.State{
				ID:           "state-id-store-success",
				TenantID:     "tenant-id-store-success",
				Fingerprint:  "fingerprint",
				PKCEVerifier: "verifier",
				RequestURI:   "http://example.com",
				Expiry:       postgrestest.DBTime,
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Upsert successfully",
			tenantID: upsertTenantID,
			state: session.State{
				ID:           upsertState.ID,
				TenantID:     upsertState.TenantID,
				Fingerprint:  "fingerprint-upsert",
				PKCEVerifier: "verifier-upsert",
				RequestURI:   "upsert.example.com",
				Expiry:       postgrestest.DBTime,
			},
			assertErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.StoreState(t.Context(), tt.tenantID, tt.state)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.StoreState() error %v", err)) || err != nil {
				return
			}

			state, err := r.LoadState(t.Context(), tt.tenantID, tt.state.ID)
			require.NoError(t, err)

			assert.Equal(t, tt.state, state, "Inserted state is not equal")
		})
	}
}

func TestRepository_LoadSession(t *testing.T) {
	tests := []struct {
		name        string
		tenantID    string
		stateID     string
		wantSession session.Session
		assertErr   assert.ErrorAssertionFunc
	}{
		{
			name:     "Select existing session",
			tenantID: "tenant1-id",
			stateID:  "stateid-one",
			wantSession: session.Session{
				StateID:     "stateid-one",
				TenantID:    "tenant1-id",
				Fingerprint: "fingerprint-one",
				Token:       "token-one",
				Expiry:      postgrestest.DBTime,
			},
			assertErr: assert.NoError,
		},
		{
			name:      "Error does not exist",
			tenantID:  "does-not-exist",
			stateID:   "stateid-one",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := sessionsql.NewRepository(dbPool)

			gotSession, err := r.LoadSession(t.Context(), tt.tenantID, tt.stateID)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.LoadSession() error %v", err)) || err != nil {
				return
			}

			assert.Equal(t, tt.wantSession, gotSession, "Repository.LoadSession()")
		})
	}
}

func TestRepository_StoreSession(t *testing.T) {
	const upsertTenantID = "tenant-id-upsert"

	upsertSession := session.Session{
		StateID:     "stateid-to-upsert",
		TenantID:    upsertTenantID,
		Fingerprint: "fingerprint-upsert",
		Token:       "token-upsert",
		Expiry:      postgrestest.DBTime,
	}

	r := sessionsql.NewRepository(dbPool)
	err := r.StoreSession(t.Context(), upsertTenantID, upsertSession)
	require.NoError(t, err)

	tests := []struct {
		name      string
		tenantID  string
		session   session.Session
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:     "Success",
			tenantID: "tenant-id-store-session-success",
			session: session.Session{
				StateID:     "state-id-store-session-success",
				TenantID:    "tenant-id-store-session-success",
				Fingerprint: "fingerprint-one",
				Token:       "token-one",
				Expiry:      postgrestest.DBTime,
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Upsert successfully",
			tenantID: upsertTenantID,
			session: session.Session{
				StateID:     upsertSession.StateID,
				TenantID:    upsertSession.TenantID,
				Fingerprint: "fingerprint-upsert-new",
				Token:       "token-upsert-new",
				Expiry:      postgrestest.DBTime,
			},
			assertErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.StoreSession(t.Context(), tt.tenantID, tt.session)
			if tt.assertErr(t, err, fmt.Sprintf("Repository.StoreSession() error %v", err)) || err != nil {
				return
			}

			session, err := r.LoadSession(t.Context(), tt.tenantID, tt.session.StateID)
			require.NoError(t, err)

			assert.Equal(t, tt.session, session, "Inserted session is not equal")
		})
	}
}
