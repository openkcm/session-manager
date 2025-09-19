//go:build integration

package integration_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-viper/mapstructure/v2"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/dbtest"
	"gopkg.in/yaml.v3"
)

func TestApiPub(t *testing.T) {
	const configFilePath = "./api_pub_test/config.yaml"
	const dbuser = "postgres"
	const dbpass = "secret"
	const dbname = "session_manager"

	ctx := t.Context()
	testdir := filepath.Dir(configFilePath)

	_, port, terminate := dbtest.Start(ctx)
	defer terminate(ctx)

	// Prepare config
	os.MkdirAll(testdir, fs.ModePerm)
	defer os.RemoveAll(testdir)

	if err := os.WriteFile(configFilePath, []byte(validConfig), fs.ModePerm); err != nil {
		t.Fatalf("failed to write config file: %s", err)
	}
	defer os.Remove(configFilePath)

	var cfg config.Config
	if err := commoncfg.LoadConfig(&cfg, nil, testdir); err != nil {
		t.Fatalf("failed to load config: %s", err)
	}

	currdir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %s", err)
	}

	cfg.Database.Name = dbname
	cfg.Database.User = commoncfg.SourceRef{Source: "embedded", Value: dbuser}
	cfg.Database.Password = commoncfg.SourceRef{Source: "embedded", Value: dbpass}
	cfg.Database.Host = commoncfg.SourceRef{Source: "embedded", Value: "localhost"}
	cfg.Database.Port = port.Port()
	cfg.Migrate.Source = "file://" + filepath.Join(currdir, "../sql")

	// Let OS choose a free port
	cfg.HTTP.Address = ":0"

	cfgMap := make(map[any]any)
	if err := mapstructure.Decode(cfg, &cfgMap); err != nil {
		t.Fatalf("failed to decode mapstructure: %s", err)
	}

	f, err := os.Create(configFilePath)
	if err != nil {
		t.Fatalf("failed to create config file: %s", err)
	}
	defer f.Close()

	if err := yaml.NewEncoder(f).Encode(cfgMap); err != nil {
		t.Fatalf("failed to write config: %s", err)
	}

	os.Chdir(testdir)
	defer os.Chdir(currdir)

	// Run the migrations
	cmd := exec.CommandContext(ctx, filepath.Join(currdir, "./api-pub"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to execute api-pub: %s\nOutput: %s", err, out)
	}
}
