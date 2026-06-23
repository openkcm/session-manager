package oauth2_test

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	credentialsoauth2 "github.com/openkcm/session-manager/modules/credentials/oauth2"
)

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
	require.NotNil(t, m.Builder())

	creds := m.Builder()("client-id")
	assert.NotNil(t, creds)
}

func TestModule_ProvisionClientSecret(t *testing.T) {
	m, err := provisionWithAuth(t, config.ClientAuth{
		Type:         "client_secret",
		ClientSecret: commoncfg.SourceRef{Source: "embedded", Value: "shh"},
	})
	require.NoError(t, err)
	creds := m.Builder()("cid")
	assert.NotNil(t, creds)
}

func TestModule_ProvisionClientSecretPost(t *testing.T) {
	m, err := provisionWithAuth(t, config.ClientAuth{
		Type:         "client_secret_post",
		ClientSecret: commoncfg.SourceRef{Source: "embedded", Value: "shh"},
	})
	require.NoError(t, err)
	creds := m.Builder()("cid")
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
