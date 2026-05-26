package business

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
)

func TestPublicMain_InvalidCSRFSecret(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			CSRFSecret: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
		},
	}

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	err := publicMain(ctx, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loading csrf token from source ref")
}

func TestPublicMain_ShortCSRFSecret(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			CSRFSecret: commoncfg.SourceRef{Source: "embedded", Value: "short"},
		},
	}

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	err := publicMain(ctx, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CSRF secret must be at least 32 bytes")
}

func TestMain_InvalidCSRFSecret(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			CSRFSecret: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
		},
	}

	err := Main(t.Context(), cfg)
	assert.Error(t, err)
}

func TestMain_PublicServerInvalidCSRF(t *testing.T) {
	cfg := &config.Config{
		SessionManager: config.SessionManager{
			CSRFSecret: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
		},
	}

	err := Main(t.Context(), cfg)
	assert.Error(t, err)
}
