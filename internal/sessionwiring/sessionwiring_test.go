package sessionwiring

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
)

func TestCredsBuilder_MTLS(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type:     "mtls",
				ClientID: "test-client",
				MTLS: &commoncfg.MTLS{
					Cert:    commoncfg.SourceRef{File: commoncfg.CredentialFile{Path: "/nonexistent/cert.pem"}},
					CertKey: commoncfg.SourceRef{File: commoncfg.CredentialFile{Path: "/nonexistent/key.pem"}},
				},
			},
		},
	}

	_, err := CredsBuilder(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load mTLS config")
}

func TestCredsBuilder_ClientSecret(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type:         "client_secret",
				ClientID:     "test-client",
				ClientSecret: commoncfg.SourceRef{Source: "embedded", Value: "test-secret"},
			},
		},
	}

	builder, err := CredsBuilder(cfg)
	require.NoError(t, err)
	require.NotNil(t, builder)

	creds := builder(cfg.SessionManager.ClientAuth.ClientID)
	clientSecretCreds, ok := creds.(*credentials.ClientSecretPost)
	require.True(t, ok)

	assert.Equal(t, "test-client", clientSecretCreds.ClientID)
	assert.Equal(t, "test-secret", clientSecretCreds.ClientSecret)
}

func TestCredsBuilder_Insecure(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type:     "insecure",
				ClientID: "test-client",
			},
		},
	}

	builder, err := CredsBuilder(cfg)
	require.NoError(t, err)
	assert.IsType(t, &credentials.Insecure{}, builder(""))
}

func TestCredsBuilder_UnknownType(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type:     "unknown",
				ClientID: "test-client",
			},
		},
	}

	_, err := CredsBuilder(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown Client Auth type")
}

func TestValkeyClient_InvalidHostRef(t *testing.T) {
	cfg := &config.Config{
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	_, err := ValkeyClient(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load valkey host")
}

func TestValkeyClient_InvalidUserRef(t *testing.T) {
	cfg := &config.Config{
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost:6379"},
			User:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	_, err := ValkeyClient(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load valkey username")
}

func TestValkeyClient_InvalidPasswordRef(t *testing.T) {
	cfg := &config.Config{
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost:6379"},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
		},
	}

	_, err := ValkeyClient(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load valkey password")
}

func TestValkeyClient_WithMTLS(t *testing.T) {
	cfg := &config.Config{
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost:6379"},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
			SecretRef: commoncfg.SecretRef{
				Type: commoncfg.MTLSSecretType,
				MTLS: commoncfg.MTLS{
					Cert:    commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/cert.pem"}},
					CertKey: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/key.pem"}},
				},
			},
		},
	}

	_, err := ValkeyClient(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load valkey mTLS config from secret ref")
}
