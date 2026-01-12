package business

import (
	"context"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/config"
)

func TestHousekeeperMain_InvalidDatabaseConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	err := HousekeeperMain(t.Context(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialise the session manager")
}

func TestHousekeeperMain_InvalidValkeyConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost"},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	err := HousekeeperMain(t.Context(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialise the session manager")
}

func TestHousekeeperMain_CancelledContext(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost"},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost:6379"},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type: "insecure",
			},
		},
	}

	// Use an already cancelled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := HousekeeperMain(ctx, cfg)
	// When context is already cancelled, initSessionManager will fail
	assert.Error(t, err)
}
