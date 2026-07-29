package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionmanager "github.com/openkcm/session-manager"
)

// fakeAppModule is used to verify that App.UnmarshalExtension routes
// per-app YAML fields into the target module struct.
type fakeAppModule struct {
	TriggerInterval string `yaml:"triggerInterval"`
	Endpoint        string `yaml:"endpoint"`
}

func (*fakeAppModule) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{ID: "config_test.fake.app"}
}

func TestLoad_AppsSection(t *testing.T) {
	yaml := `
apps:
  housekeeper:
    module: app.module.housekeeper
    triggerInterval: 5m
  audit-shipper:
    module: app.module.audit-shipper
    endpoint: https://example.invalid/audit
appsOrder:
  - housekeeper
  - audit-shipper
`

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFile), []byte(yaml), 0o600))

	cfg, err := Load("", dir)
	require.NoError(t, err)

	require.Len(t, cfg.Apps, 2)
	require.Contains(t, cfg.Apps, "housekeeper")
	require.Contains(t, cfg.Apps, "audit-shipper")

	assert.Equal(t, "app.module.housekeeper", cfg.Apps["housekeeper"].Module())
	assert.Equal(t, "app.module.audit-shipper", cfg.Apps["audit-shipper"].Module())
	assert.Equal(t, []string{"housekeeper", "audit-shipper"}, cfg.AppsOrder)

	// Per-app fields must be reachable via UnmarshalExtension into the target
	// module type — confirms the koanf subtree is wired up per entry.
	hk := &fakeAppModule{}
	require.NoError(t, cfg.Apps["housekeeper"].UnmarshalExtension(hk))
	assert.Equal(t, "5m", hk.TriggerInterval)

	as := &fakeAppModule{}
	require.NoError(t, cfg.Apps["audit-shipper"].UnmarshalExtension(as))
	assert.Equal(t, "https://example.invalid/audit", as.Endpoint)
}

func TestLoad_AppsAbsentIsNoop(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFile), []byte("# empty\n"), 0o600))

	cfg, err := Load("", dir)
	require.NoError(t, err)
	assert.Empty(t, cfg.Apps)
	assert.Empty(t, cfg.AppsOrder)
}

// fakeServiceModule mirrors the per-service YAML fields a gRPC service module
// would expose. Used to assert per-entry koanf subtree wiring under
// apps[].services[].
type fakeServiceModule struct {
	Trust           string `yaml:"trust"`
	AllowHttpScheme bool   `yaml:"allowHttpScheme"`
}

func (*fakeServiceModule) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{ID: "config_test.fake.service"}
}

func TestLoad_ValkeyDefaultModule(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFile), []byte("valkey:\n  prefix: foo\n"), 0o600))

	cfg, err := Load("", dir)
	require.NoError(t, err)
	assert.Equal(t, "sessionstore.module.valkey", cfg.ValKey.Module())
	assert.Equal(t, "foo", cfg.ValKey.Prefix)
}

func TestLoad_ValkeyCustomModule(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFile), []byte("valkey:\n  module: my.custom.sessionstore\n"), 0o600))

	cfg, err := Load("", dir)
	require.NoError(t, err)
	assert.Equal(t, "my.custom.sessionstore", cfg.ValKey.Module())
}

func TestLoad_CredentialsDefaultModule(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFile), []byte("# empty\n"), 0o600))

	cfg, err := Load("", dir)
	require.NoError(t, err)
	assert.Equal(t, "credentials.module.oauth2", cfg.Credentials.Module())
}

func TestLoad_CredentialsCustomModule(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFile), []byte("credentials:\n  module: my.custom.credentials\n"), 0o600))

	cfg, err := Load("", dir)
	require.NoError(t, err)
	assert.Equal(t, "my.custom.credentials", cfg.Credentials.Module())
}

func TestLoad_AppServicesPerEntryKoanf(t *testing.T) {
	yaml := `
apps:
  grpc:
    module: app.module.grpcserver
    services:
      - module: service.module.grpc.session
        trust: trust.module.oidc
        allowHttpScheme: true
      - module: service.module.grpc.trustmapping
        trust: trust.module.alt
`

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFile), []byte(yaml), 0o600))

	cfg, err := Load("", dir)
	require.NoError(t, err)

	require.Contains(t, cfg.Apps, "grpc")
	app := cfg.Apps["grpc"]
	assert.Equal(t, "app.module.grpcserver", app.Module())
	require.Len(t, app.Services, 2)

	assert.Equal(t, "service.module.grpc.session", app.Services[0].Module())
	assert.Equal(t, "service.module.grpc.trustmapping", app.Services[1].Module())

	svc0 := &fakeServiceModule{}
	require.NoError(t, app.Services[0].UnmarshalExtension(svc0))
	assert.Equal(t, "trust.module.oidc", svc0.Trust)
	assert.True(t, svc0.AllowHttpScheme)

	svc1 := &fakeServiceModule{}
	require.NoError(t, app.Services[1].UnmarshalExtension(svc1))
	assert.Equal(t, "trust.module.alt", svc1.Trust)
}
