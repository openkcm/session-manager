//go:build helmtests

package main_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-viper/mapstructure/v2"
	"github.com/goccy/go-yaml"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	_ "github.com/openkcm/session-manager/modules/standard"
)

// configInput is a set of values to render the chart with.
type configInput struct {
	name string
	args []string
}

// configInputs are the values the configuration tests render the chart with: the defaults,
// the values for local development, every documented optional setting, and the documented
// alternatives that cannot be combined with them.
var configInputs = []configInput{
	{name: "default values"},
	{name: "development values", args: []string{"-f", path + "/values-dev.yaml"}},
	{name: "all documented settings", args: []string{"-f", "testdata/values-all-config.yaml"}},
	{name: "documented alternatives", args: []string{"-f", "testdata/values-config-alternatives.yaml"}},
	{name: "insecure client authentication", args: []string{"--set", "config.sessionManager.clientAuth.type=insecure"}},
}

// TestRenderedConfigLoadsStrictly loads the rendered config.yaml as Session Manager does at
// startup, and fails if it contains keys that Session Manager does not read. Session Manager
// itself ignores such keys without a warning, and matches keys ignoring case.
func TestRenderedConfigLoadsStrictly(t *testing.T) {
	for _, tt := range configInputs {
		t.Run(tt.name, func(t *testing.T) {
			configMap, _ := configMaps(t, renderManifests(t, tt.args...))
			if unknown := unknownConfigKeys(t, configMap.Data["config.yaml"]); len(unknown) > 0 {
				t.Errorf("config.yaml has keys that Session Manager does not read: %v\n%s", unknown, configMap.Data["config.yaml"])
			}
		})
	}
}

// TestStrictConfigCheckFindsUnknownKeys makes sure that the strict check of
// TestRenderedConfigLoadsStrictly reports misspelled keys, also in module sections and in
// the services of an app.
func TestStrictConfigCheckFindsUnknownKeys(t *testing.T) {
	configMap, _ := configMaps(t, renderManifests(t,
		"--set", "config.database.hots.value=db",
		"--set", "config.logger.formatter.fields.otel.traceID=trace.id",
		"--set", "config.valkey.secretRef.mTLS.cert.value=cert",
		"--set", "config.sessionManager.clientAuth.mtls.cert.value=cert",
		"--set", "config.apps.grpc.services[0].module=service.module.grpc.session",
		"--set", "config.apps.grpc.services[0].allowHttpScheme=true",
		"--set", "config.apps.grpc.services[0].allowHTTPScheme=true",
	))

	want := []string{
		"apps[grpc].services[0].allowHTTPScheme",
		"database.hots",
		"logger.formatter.fields.otel.traceID",
		"sessionManager.clientAuth.mtls",
		"valkey.secretRef.mTLS",
	}
	if got := unknownConfigKeys(t, configMap.Data["config.yaml"]); !slices.Equal(got, want) {
		t.Errorf("unknown keys: got %v, want %v", got, want)
	}
}

// TestConfigMapsHaveSameConfig makes sure that templates/configmap.yaml and
// templates/migrate/configmap.yaml write the same config.yaml.
func TestConfigMapsHaveSameConfig(t *testing.T) {
	inputs := append(slices.Clone(configInputs),
		configInput{name: "custom settings", args: []string{
			"--set", "config.environment=",
			"--set", "config.application.name=sm-eu10",
			"--set", "config.audit.endpoint=http://collector:4318/v1/logs",
			"--set", "config.appsOrder[0]=grpc",
			"--set", "config.logger.level=debug",
			"--set", "config.unknown=x",
		}},
		configInput{name: "immutable configuration", args: []string{
			"--set", "configMap.immutable=true",
			"--set", "config.sessionManager.allowedRedirectBaseURLs[0]=https://ui.example.com/",
		}},
	)
	for _, tt := range inputs {
		t.Run(tt.name, func(t *testing.T) {
			configMap, hookConfigMap := configMaps(t, renderManifests(t, tt.args...))
			if configMap.Data["config.yaml"] == "" {
				t.Fatal("config.yaml is empty")
			}
			if configMap.Data["config.yaml"] != hookConfigMap.Data["config.yaml"] {
				t.Errorf("the ConfigMaps %s and %s have different config.yaml:\n%s\n---\n%s",
					configMap.Metadata.Name, hookConfigMap.Metadata.Name,
					configMap.Data["config.yaml"], hookConfigMap.Data["config.yaml"])
			}
		})
	}
}

// TestConfigChecksum makes sure that the pods restart when the configuration changes, and
// only then.
func TestConfigChecksum(t *testing.T) {
	checksums := func(t *testing.T, args ...string) (apiServer, housekeeper, migrate string) {
		t.Helper()
		manifests := renderManifests(t, args...)
		apiServer = manifests["Deployment/session-manager"].Spec.Template.Metadata.Annotations["checksum/config"]
		housekeeper = manifests["Deployment/session-manager-housekeeper"].Spec.Template.Metadata.Annotations["checksum/config"]
		migrate = manifests["Job/session-manager-migrate"].Spec.Template.Metadata.Annotations["checksum/config"]
		if apiServer == "" || housekeeper == "" || migrate == "" {
			t.Fatalf("missing checksum/config: API server %q, housekeeper %q, migration job %q", apiServer, housekeeper, migrate)
		}
		if apiServer != housekeeper {
			t.Errorf("the API server (%s) and the housekeeper (%s) have different checksums", apiServer, housekeeper)
		}
		return apiServer, housekeeper, migrate
	}

	base, _, baseMigrate := checksums(t)

	t.Run("unchanged when the configuration does not change", func(t *testing.T) {
		checksum, _, migrate := checksums(t, "--set", "replicaCount=3", "--set", "housekeeper.resources.requests.cpu=100m")
		if checksum != base || migrate != baseMigrate {
			t.Errorf("checksums changed with replicaCount: %s -> %s, migration job %s -> %s", base, checksum, baseMigrate, migrate)
		}
	})

	for _, change := range []string{
		"config.logger.level=debug",
		"config.sessionManager.callbackURL=https://api.example.com/sm/callback",
		"config.database.host.value=postgresql",
	} {
		t.Run("changed with "+change, func(t *testing.T) {
			checksum, _, migrate := checksums(t, "--set", change)
			if checksum == base {
				t.Errorf("checksum/config of the API server and the housekeeper did not change: %s", checksum)
			}
			if migrate == baseMigrate {
				t.Errorf("checksum/config of the migration job did not change: %s", migrate)
			}
		})
	}
}

// TestImmutableConfigMapName makes sure that an immutable ConfigMap gets a new name when the
// configuration changes, and only then, and that the pods mount it.
func TestImmutableConfigMapName(t *testing.T) {
	name := func(t *testing.T, args ...string) string {
		t.Helper()
		manifests := renderManifests(t, append([]string{"--set", "configMap.immutable=true"}, args...)...)
		configMap, _ := configMaps(t, manifests)
		if !strings.HasPrefix(configMap.Metadata.Name, appName+"-") {
			t.Fatalf("unexpected name of the immutable ConfigMap: %s", configMap.Metadata.Name)
		}
		for _, deployment := range []string{"Deployment/session-manager", "Deployment/session-manager-housekeeper"} {
			volumes := manifests[deployment].Spec.Template.Spec.Volumes
			if !slices.ContainsFunc(volumes, func(v volume) bool {
				return v.Name == "config" && v.ConfigMap.Name == configMap.Metadata.Name
			}) {
				t.Errorf("%s does not mount the ConfigMap %s: %+v", deployment, configMap.Metadata.Name, volumes)
			}
		}
		return configMap.Metadata.Name
	}

	base := name(t)

	t.Run("unchanged when the configuration does not change", func(t *testing.T) {
		if got := name(t, "--set", "replicaCount=3"); got != base {
			t.Errorf("name changed with replicaCount: %s -> %s", base, got)
		}
	})

	t.Run("changed when the configuration changes", func(t *testing.T) {
		if got := name(t, "--set", "config.logger.level=debug"); got == base {
			t.Errorf("name did not change with config.logger.level: %s", got)
		}
	})
}

// unknownConfigKeys loads configYAML as Session Manager does at startup, and returns the
// keys that neither the configuration (internal/config) nor the module of their section
// reads. Keys must match the names in the Go yaml tags exactly, which is stricter than
// Session Manager: it ignores the case of keys.
func unknownConfigKeys(t *testing.T, configYAML string) []string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	// As at startup (internal/cmdutils), with the build information of a build without
	// version information (cmd/session-manager).
	cfg, err := config.Load("{}", dir)
	if err != nil {
		t.Fatalf("loading config.yaml: %v\n%s", err, configYAML)
	}

	// As the API server (business.Main) loads its modules: create each module and read its
	// section, then validate the dependencies between the modules, without provisioning.
	// The housekeeper and the migration job load a subset of these modules.
	specs := make([]sessionmanager.LoadSpec, 0, 5+len(cfg.Apps))
	specs = append(specs,
		sessionmanager.LoadSpec{Cfg: &cfg.Database},
		sessionmanager.LoadSpec{Cfg: &cfg.Migrate},
		sessionmanager.LoadSpec{Cfg: &cfg.Trust},
		sessionmanager.LoadSpec{Cfg: &cfg.ValKey},
		sessionmanager.LoadSpec{Cfg: &cfg.Credentials},
	)
	for _, name := range cfg.AppsOrder {
		if _, ok := cfg.Apps[name]; !ok {
			t.Fatalf("appsOrder references unknown app %q", name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(cfg.Apps)) {
		spec := sessionmanager.LoadSpec{Cfg: cfg.Apps[name], IsApp: true}
		for _, service := range cfg.Apps[name].Services {
			spec.Children = append(spec.Children, sessionmanager.LoadSpec{Cfg: service})
		}
		specs = append(specs, spec)
	}
	c, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)
	if err := config.WithContext(c, cfg).ValidateAll(specs); err != nil {
		t.Fatalf("loading the modules: %v\n%s", err, configYAML)
	}

	// Strictly: decode config.yaml into the configuration and each module section into its
	// module, and collect the keys that a decoder does not read.
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(configYAML), &raw); err != nil {
		t.Fatal(err)
	}
	type decoding struct {
		section string // "" for the whole configuration
		unused  []string
	}
	decodings := []decoding{{section: "", unused: strictDecode(t, raw, new(config.Config))}}
	decodeSection := func(section string, raw any, moduleID string) {
		if raw == nil {
			return
		}
		info, err := sessionmanager.GetModule(moduleID)
		if err != nil {
			t.Fatal(err)
		}
		var unused []string
		for _, key := range strictDecode(t, raw, info.New()) {
			unused = append(unused, section+"."+key)
		}
		decodings = append(decodings, decoding{section: section, unused: unused})
	}
	decodeSection("database", raw["database"], cfg.Database.Module())
	decodeSection("migrate", raw["migrate"], cfg.Migrate.Module())
	decodeSection("trust", raw["trust"], cfg.Trust.Module())
	decodeSection("valkey", raw["valkey"], cfg.ValKey.Module())
	decodeSection("credentials", raw["credentials"], cfg.Credentials.Module())
	rawApps, _ := raw["apps"].(map[string]any)
	for name, rawApp := range rawApps {
		section := "apps[" + name + "]"
		decodeSection(section, rawApp, cfg.Apps[name].Module())
		rawServices, _ := rawApp.(map[string]any)["services"].([]any)
		for i, rawService := range rawServices {
			decodeSection(fmt.Sprintf("%s.services[%d]", section, i), rawService, cfg.Apps[name].Services[i].Module())
		}
	}

	// A key is unknown if no decoder that covers it reads it.
	notRead := func(d decoding, key string) bool {
		return slices.ContainsFunc(d.unused, func(unused string) bool {
			return key == unused || strings.HasPrefix(key, unused+".") || strings.HasPrefix(key, unused+"[")
		})
	}
	unknown := make(map[string]bool)
	for _, d := range decodings {
		for _, key := range d.unused {
			unknown[key] = !slices.ContainsFunc(decodings, func(other decoding) bool {
				covers := other.section == "" || strings.HasPrefix(key, other.section+".")
				return covers && !notRead(other, key)
			})
		}
	}
	var keys []string
	for key, isUnknown := range unknown {
		if isUnknown {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// strictDecode decodes in into out with the settings of Session Manager's configuration
// loader (koanfUnmarshalConf in internal/config), except that keys must match exactly, and
// returns the keys of in that out does not read, such as "database.host" or
// "apps[grpc].services[0].allowHttpScheme".
func strictDecode(t *testing.T, in, out any) []string {
	t.Helper()
	var metadata mapstructure.Metadata
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc()),
		Metadata:         &metadata,
		Result:           out,
		TagName:          "yaml",
		WeaklyTypedInput: true,
		SquashTagOption:  "inline",
		MatchName:        func(mapKey, fieldName string) bool { return mapKey == fieldName },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(in); err != nil {
		t.Fatalf("decoding into %T: %v", out, err)
	}
	return metadata.Unused
}
