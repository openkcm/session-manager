package business

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/config"
)

func TestMigrateMain_InvalidDatabaseConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	err := MigrateMain(t.Context(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "making connection string from config")
}

func TestMigrateMain_InvalidUserRef(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost"},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
	}

	err := MigrateMain(t.Context(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "making connection string from config")
}

func TestMigrateMain_InvalidPasswordRef(t *testing.T) {
	cfg := &config.Config{
		Database: config.Database{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost"},
			Port:     "5432",
			Name:     "testdb",
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "file", File: commoncfg.CredentialFile{Path: "/nonexistent/file"}},
		},
	}

	err := MigrateMain(t.Context(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "making connection string from config")
}
