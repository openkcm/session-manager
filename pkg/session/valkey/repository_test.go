package sessionvalkey_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"

	"github.com/openkcm/session-manager/internal/dbtest/valkeytest"
	"github.com/openkcm/session-manager/pkg/session"
	sessionvalkey "github.com/openkcm/session-manager/pkg/session/valkey"
)

var client valkey.Client
var testTime time.Time

func init() {
	now := time.Now()
	testTime = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location()).Add(30 * 24 * time.Hour)
}

func init() {
	// There's a little inconsistency with the timezone when RFC3339 is parsed from a JSON object.
	// So we do a workaround here
	t, _ := testTime.MarshalJSON()
	_ = testTime.UnmarshalJSON(t)
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	valkeyClient, _, terminate := valkeytest.Start(ctx)
	client = valkeyClient

	code := m.Run()
	terminate(ctx)

	os.Exit(code)
}

func prepareState(t *testing.T, prefix string, state session.State) {
	t.Helper()

	key := fmt.Sprintf("%s:state:%s", prefix, state.ID)
	err := client.Do(t.Context(), client.B().Set().Key(key).Value(valkey.JSON(state)).Build()).Error()
	require.NoError(t, err, "inserting state")
}

func prepareSession(t *testing.T, prefix string, s session.Session) {
	t.Helper()

	key := fmt.Sprintf("%s:session:%s", prefix, s.ID)
	err := client.Do(t.Context(), client.B().Set().Key(key).Value(valkey.JSON(s)).Build()).Error()
	require.NoError(t, err, "inserting session")
}

func TestRepository_LoadState(t *testing.T) {
	const prefix = "session-manager-load-state-test"

	prepareState(t, prefix, session.State{
		ID:           "stateid-one",
		TenantID:     "tenant1-id",
		Fingerprint:  "fingerprint-one",
		PKCEVerifier: "verifier-one",
		RequestURI:   "http://localhost",
		Expiry:       testTime,
	})

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
				Expiry:       testTime,
			},
			assertErr: assert.NoError,
		},
		{
			name:      "Error does not exist",
			tenantID:  "does-not-exist",
			stateID:   "does-not-exist",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := sessionvalkey.NewRepository(client, prefix)

			gotState, err := r.LoadState(t.Context(), tt.stateID)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.LoadState() error %v", err)) || err != nil {
				return
			}

			assert.Equal(t, tt.wantState, gotState, "Repository.LoadState()")
		})
	}
}

func TestRepository_StoreState(t *testing.T) {
	const prefix = "session-manager-store-state-test"
	const upsertTenantID = "tenant-id-upsert"

	upsertState := session.State{
		ID:           "stateid-to-upsert",
		TenantID:     upsertTenantID,
		Fingerprint:  "fingerprint-upsert",
		PKCEVerifier: "verifier",
		RequestURI:   "example.com",
		Expiry:       testTime,
	}

	prepareState(t, prefix, upsertState)

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
				Expiry:       testTime,
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
				Expiry:       testTime,
			},
			assertErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := sessionvalkey.NewRepository(client, prefix)
			err := r.StoreState(t.Context(), tt.state)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.StoreState() error %v", err)) || err != nil {
				return
			}

			state, err := r.LoadState(t.Context(), tt.state.ID)
			require.NoError(t, err)

			assert.Equal(t, tt.state, state, "Inserted state is not equal")
		})
	}
}

func TestRepository_LoadSession(t *testing.T) {
	const prefix = "session-manager-load-session-test"

	prepareSession(t, prefix, session.Session{
		ID:                "sessionid-one",
		TenantID:          "tenant1-id",
		Fingerprint:       "fingerprint-one",
		AccessToken:       "access-token-one",
		RefreshToken:      "refresh-token-one",
		Expiry:            testTime,
		AccessTokenExpiry: testTime,
	})

	tests := []struct {
		name        string
		tenantID    string
		sessionID   string
		wantSession session.Session
		assertErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "Select existing session",
			tenantID:  "tenant1-id",
			sessionID: "sessionid-one",
			wantSession: session.Session{
				ID:                "sessionid-one",
				TenantID:          "tenant1-id",
				Fingerprint:       "fingerprint-one",
				AccessToken:       "access-token-one",
				RefreshToken:      "refresh-token-one",
				Expiry:            testTime,
				AccessTokenExpiry: testTime,
			},
			assertErr: assert.NoError,
		},
		{
			name:      "Error does not exist",
			tenantID:  "does-not-exist",
			sessionID: "does-not-exist",
			assertErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := sessionvalkey.NewRepository(client, prefix)

			gotSession, err := r.LoadSession(t.Context(), tt.sessionID)
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.LoadSession() error %v", err)) || err != nil {
				return
			}

			assert.Equal(t, tt.wantSession, gotSession, "Repository.LoadSession()")
		})
	}
}

func TestRepository_StoreSession_Success(t *testing.T) {
	const prefix = "session-manager-store-session-test"

	const upsertTenantID = "tenant-id-upsert"
	upsertSession := session.Session{
		ID:                "sessionid-to-upsert",
		TenantID:          upsertTenantID,
		Fingerprint:       "fingerprint-upsert",
		AccessToken:       "access-token-upsert",
		RefreshToken:      "refresh-token-upsert",
		Expiry:            testTime,
		AccessTokenExpiry: testTime,
	}

	prepareSession(t, prefix, upsertSession)

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
				ID:                "sessionid-id-store-session-success",
				TenantID:          "tenant-id-store-session-success",
				Fingerprint:       "fingerprint-one",
				AccessToken:       "access-token-one",
				RefreshToken:      "refresh-token-one",
				AccessTokenExpiry: testTime,
			},
			assertErr: assert.NoError,
		},
		{
			name:     "Upsert successfully",
			tenantID: upsertTenantID,
			session: session.Session{
				ID:                upsertSession.ID,
				TenantID:          upsertSession.TenantID,
				Fingerprint:       "fingerprint-upsert-new",
				AccessToken:       "access-token-upsert-new",
				RefreshToken:      "refresh-token-upsert-new",
				AccessTokenExpiry: testTime,
			},
			assertErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			shortTestTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location()).Add(10 * time.Second).UTC()
			tt.session.Expiry = shortTestTime
			duration := time.Until(tt.session.Expiry)
			r := sessionvalkey.NewRepository(client, prefix)
			err := r.StoreSession(t.Context(), tt.session)
			tt.assertErr(t, err, fmt.Sprintf("Repository.StoreSession() error %v", err))
			session, err := r.LoadSession(t.Context(), tt.session.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.session, session, "Inserted session is not equal")
			t.Log("test getting session ID by provider ID")
			sessionID, err := r.GetSessIDByProviderID(t.Context(), tt.session.ProviderID)
			assert.NoError(t, err)
			assert.Equal(t, tt.session.ID, sessionID)
			t.Log("test getting access token")
			accessToken, err := r.GetAccessTokenForSession(t.Context(), tt.session.ID)
			assert.NoError(t, err)
			assert.Equal(t, tt.session.AccessToken, accessToken)
			t.Log("test getting refresh token")
			refreshToken, err := r.GetRefreshTokenForSession(t.Context(), tt.session.ID)
			assert.NoError(t, err)
			assert.Equal(t, tt.session.RefreshToken, refreshToken)
			t.Log("wait to see if session is deleted after expiration")
			time.Sleep(duration)
			_, err = r.LoadSession(t.Context(), tt.session.ID)
			require.Error(t, err)
		})
	}
}

func TestRepository_StoreSession_Fail(t *testing.T) {
	const prefix = "session-manager-store-session-cleanup-test"
	tests := []struct {
		name     string
		tenantID string
		session  session.Session
	}{
		{
			name: "Successful_CleanUp_OnError",
			session: session.Session{
				ID:                "sessionid-id-store-session-successful-cleanup",
				TenantID:          "tenant-id-store-session-successful-cleanup",
				Fingerprint:       "fingerprint-upsert-new",
				AccessToken:       "access-token-upsert-new",
				RefreshToken:      "refresh-token-upsert-new",
				Expiry:            time.Now().Add(-100 * time.Second).UTC(),
				AccessTokenExpiry: testTime,
			},
			tenantID: "tenant-id-store-session-successful-cleanup",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := sessionvalkey.NewRepository(client, prefix)
			err := r.StoreSession(t.Context(), tt.session)
			assert.Error(t, err)
			_, err = r.LoadSession(t.Context(), tt.session.ID)
			assert.Error(t, err)
			_, err = r.GetAccessTokenForSession(t.Context(), tt.session.ID)
			assert.Error(t, err)
			_, err = r.GetSessIDByProviderID(t.Context(), tt.session.ProviderID)
			assert.Error(t, err)
			_, err = r.GetRefreshTokenForSession(t.Context(), tt.session.ID)
			assert.Error(t, err)
		})
	}
}

func TestRepository_ListSessions(t *testing.T) {
	const prefix = "session-manager-list-sessions-test"

	prepareSession(t, prefix, session.Session{
		ID:                "sessionid-one",
		TenantID:          "tenant1-id",
		Fingerprint:       "fingerprint-one",
		AccessToken:       "access-token-one",
		RefreshToken:      "refresh-token-one",
		Expiry:            testTime,
		AccessTokenExpiry: testTime,
	})
	prepareSession(t, prefix, session.Session{
		ID:                "sessionid-two",
		TenantID:          "tenant2-id",
		Fingerprint:       "fingerprint-two",
		AccessToken:       "access-token-two",
		RefreshToken:      "refresh-token-two",
		Expiry:            testTime,
		AccessTokenExpiry: testTime,
	})
	prepareSession(t, prefix, session.Session{
		ID:                "sessionid-three",
		TenantID:          "tenant3-id",
		Fingerprint:       "fingerprint-three",
		AccessToken:       "access-token-three",
		RefreshToken:      "refresh-token-three",
		Expiry:            testTime,
		AccessTokenExpiry: testTime,
	})

	tests := []struct {
		name         string
		tenantID     string
		sessionID    string
		wantSessions []session.Session
		assertErr    assert.ErrorAssertionFunc
	}{
		{
			name: "List all sessions",
			wantSessions: []session.Session{
				{
					ID:                "sessionid-one",
					TenantID:          "tenant1-id",
					Fingerprint:       "fingerprint-one",
					AccessToken:       "access-token-one",
					RefreshToken:      "refresh-token-one",
					Expiry:            testTime,
					AccessTokenExpiry: testTime,
				},
				{
					ID:                "sessionid-two",
					TenantID:          "tenant2-id",
					Fingerprint:       "fingerprint-two",
					AccessToken:       "access-token-two",
					RefreshToken:      "refresh-token-two",
					Expiry:            testTime,
					AccessTokenExpiry: testTime,
				},
				{
					ID:                "sessionid-three",
					TenantID:          "tenant3-id",
					Fingerprint:       "fingerprint-three",
					AccessToken:       "access-token-three",
					RefreshToken:      "refresh-token-three",
					Expiry:            testTime,
					AccessTokenExpiry: testTime,
				},
			},
			assertErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := sessionvalkey.NewRepository(client, prefix)

			gotSessions, err := r.ListSessions(t.Context())
			if !tt.assertErr(t, err, fmt.Sprintf("Repository.ListSessions() error %v", err)) || err != nil {
				return
			}

			sort.Slice(gotSessions, func(i, j int) bool { return gotSessions[i].ID < gotSessions[j].ID })
			sort.Slice(tt.wantSessions, func(i, j int) bool { return tt.wantSessions[i].ID < tt.wantSessions[j].ID })

			assert.Equal(t, tt.wantSessions, gotSessions, "Repository.ListSessions()")
		})
	}
}

func TestRepository_DeleteState(t *testing.T) {
	const tenantID = "tenant-delete"
	const stateID = "stateid-delete"
	const prefix = "session-manager-delete-state-test"

	state := session.State{
		ID:          stateID,
		TenantID:    tenantID,
		Fingerprint: "fingerprint-delete",
		Expiry:      testTime,
	}

	prepareState(t, prefix, state)

	tests := []struct {
		name      string
		tenantID  string
		stateID   string
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "Delete existing state",
			tenantID:  tenantID,
			stateID:   stateID,
			assertErr: assert.NoError,
		},
		{
			name:      "Delete non-existing state",
			tenantID:  "non-existent-tenant",
			stateID:   "non-existent-state",
			assertErr: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := sessionvalkey.NewRepository(client, prefix)
			err := r.DeleteState(t.Context(), tt.stateID)
			tt.assertErr(t, err, "Repository.DeleteState() error")

			_, err = r.LoadState(t.Context(), tt.stateID)
			assert.Error(t, err, "State should not exist after deletion")
		})
	}
}
