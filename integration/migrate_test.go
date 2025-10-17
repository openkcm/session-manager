//go:build integration

package integration_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/go-viper/mapstructure/v2"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/session-manager/internal/config"
)

func TestMigrate(t *testing.T) {
	const cmdName = "migrate"
	const configFilePath = "./" + cmdName + "-test/config.yaml"
	const dbuser = "postgres"
	const dbpass = "secret"
	const dbname = "session_manager"

	ctx := t.Context()
	testdir := filepath.Dir(configFilePath)

	// This test doesn't utilise infraStat like the others because it needs an empty DB
	pgContainer, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase(dbname),
		postgres.WithUsername(string(dbuser)),
		postgres.WithPassword(string(dbpass)),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("failed to start PostgreSQL: %s", err)
	}

	port, err := pgContainer.MappedPort(ctx, nat.Port("5432"))
	if err != nil {
		t.Fatalf("failed to get mapped port for the PostgreSQL container: %s", err)
	}

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
	cmd := exec.CommandContext(ctx, filepath.Join(currdir, "./session-manager"), cmdName)

	cmdOutPath := filepath.Join(currdir, cmdName+".log")
	cmdOut, err := os.Create(cmdOutPath)
	if err != nil {
		t.Fatalf("failed to create an log file")
	}
	defer cmdOut.Close()

	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut
	t.Logf("starting an app process. Logs will be saved into %s", cmdOutPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("process exited abnormally: %s", err)
	}
}
