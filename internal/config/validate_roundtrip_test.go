package config_test

// This test blank-imports modules/standard so the dep-tag interfaces and
// target module IDs referenced by the default config actually exist in the
// registry when ValidateAll runs.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	_ "github.com/openkcm/session-manager/modules/standard"
)

// TestValidateAll_DefaultConfig loads a default config and validates the
// top-level module graph without provisioning anything. It guards against a
// dep-tag typo or an unregistered interface key: any `dep:"..."` that does not
// resolve, or points at a module that does not implement the required
// interface, fails here rather than at production startup.
func TestValidateAll_DefaultConfig(t *testing.T) {
	yaml := `
database:
    module: database.module.pgxpool
trust:
    module: trust.module.oidc
valkey:
    module: sessionstore.module.valkey
credentials:
    module: credentials.module.oauth2
`
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o600))

	cfg, err := config.Load("", dir)
	require.NoError(t, err)

	c, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)
	c = config.WithContext(c, cfg)

	// The same top-level set business.Main loads. ValidateAll runs Phases A–C
	// (enumerate + prepare + validate) with no provisioning, so no real
	// database/valkey connections are attempted.
	err = c.ValidateAll([]sessionmanager.LoadSpec{
		{Cfg: &cfg.Database},
		{Cfg: &cfg.Trust},
		{Cfg: &cfg.ValKey},
		{Cfg: &cfg.Credentials},
	})
	require.NoError(t, err, "default module graph must validate cleanly")
}
