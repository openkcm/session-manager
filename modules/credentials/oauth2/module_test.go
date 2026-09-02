package oauth2_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	credentialsoauth2 "github.com/openkcm/session-manager/modules/credentials/oauth2"
)

// newDiscoveryServer starts an OIDC discovery endpoint whose issuer matches its
// own URL, so it can be passed as the issuer to the module's ResourceServer.
func newDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()

	var issuer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuer,
			"introspection_endpoint": issuer + "/introspect",
			"token_endpoint":         issuer + "/token",
		})
	}))
	issuer = srv.URL
	t.Cleanup(srv.Close)

	return srv
}

func TestModule_RegistrationAndID(t *testing.T) {
	info, err := sessionmanager.GetModule("credentials.module.oauth2")
	require.NoError(t, err)
	assert.Equal(t, "credentials.module.oauth2", info.ID)

	mod := info.New()
	require.NotNil(t, mod)
	_, ok := mod.(*credentialsoauth2.Module)
	assert.True(t, ok, "New() must return *Module")
}

func provisionWithAuth(t *testing.T, auth config.ClientAuth) (*credentialsoauth2.Module, error) {
	t.Helper()
	cfg := &config.Config{}
	cfg.SessionManager.ClientAuth = auth

	ctx, cancel := sessionmanager.NewContext(t.Context())
	t.Cleanup(func() { cancel(nil) })
	ctx = config.WithContext(ctx, cfg)

	m := new(credentialsoauth2.Module)
	return m, m.Provision(ctx)
}

func TestModule_ProvisionInsecure(t *testing.T) {
	m, err := provisionWithAuth(t, config.ClientAuth{Type: "insecure"})
	require.NoError(t, err)

	srv := newDiscoveryServer(t)
	creds, err := m.ResourceServer("client-id", srv.URL)
	require.NoError(t, err)
	assert.NotNil(t, creds)
}

func TestModule_ProvisionClientSecret(t *testing.T) {
	m, err := provisionWithAuth(t, config.ClientAuth{
		Type:         "client_secret",
		ClientSecret: commoncfg.SourceRef{Source: "embedded", Value: "shh"},
	})
	require.NoError(t, err)
	srv := newDiscoveryServer(t)
	creds, err := m.ResourceServer("cid", srv.URL)
	require.NoError(t, err)
	assert.NotNil(t, creds)
}

func TestModule_ProvisionClientSecretPost(t *testing.T) {
	m, err := provisionWithAuth(t, config.ClientAuth{
		Type:         "client_secret_post",
		ClientSecret: commoncfg.SourceRef{Source: "embedded", Value: "shh"},
	})
	require.NoError(t, err)
	srv := newDiscoveryServer(t)
	creds, err := m.ResourceServer("cid", srv.URL)
	require.NoError(t, err)
	assert.NotNil(t, creds)
}

func TestModule_ProvisionUnknownTypeFails(t *testing.T) {
	_, err := provisionWithAuth(t, config.ClientAuth{Type: "totally-bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "totally-bogus")
}

func TestModule_ProvisionWithoutConfigFails(t *testing.T) {
	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	m := new(credentialsoauth2.Module)
	err := m.Provision(ctx)
	require.Error(t, err)
}

func oidcFor(issuer, clientID string) *oidcv1.OIDC {
	return oidcv1.OIDC_builder{Issuer: &issuer, ClientId: &clientID}.Build()
}

func TestModule_RelyingPartyInsecure(t *testing.T) {
	m, err := provisionWithAuth(t, config.ClientAuth{Type: "insecure"})
	require.NoError(t, err)

	srv := newDiscoveryServer(t)
	rp, err := m.RelyingParty(oidcFor(srv.URL, "client-id"), "https://app.example.com/callback")
	require.NoError(t, err)
	assert.NotNil(t, rp)
}

func TestModule_RelyingPartyClientSecret(t *testing.T) {
	m, err := provisionWithAuth(t, config.ClientAuth{
		Type:         "client_secret",
		ClientSecret: commoncfg.SourceRef{Source: "embedded", Value: "shh"},
	})
	require.NoError(t, err)

	srv := newDiscoveryServer(t)
	rp, err := m.RelyingParty(oidcFor(srv.URL, "cid"), "https://app.example.com/callback")
	require.NoError(t, err)
	assert.NotNil(t, rp)
}

func TestModule_ResourceServerUnknownType(t *testing.T) {
	// A Module that was never provisioned has an empty auth type and must fall
	// through to the default (error) branch.
	m := new(credentialsoauth2.Module)

	_, err := m.ResourceServer("client-id", "https://issuer.example.com")
	require.Error(t, err)
}

func TestModule_RelyingPartyUnknownType(t *testing.T) {
	m := new(credentialsoauth2.Module)

	_, err := m.RelyingParty(oidcFor("https://issuer.example.com", "client-id"), "https://app.example.com/callback")
	require.Error(t, err)
}
