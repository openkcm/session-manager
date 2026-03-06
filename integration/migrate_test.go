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
	_ = os.MkdirAll(testdir, fs.ModePerm)
	defer os.RemoveAll(testdir)

	err = os.WriteFile(configFilePath, []byte(validConfig), fs.ModePerm)
	if err != nil {
		t.Fatalf("failed to write config file: %s", err)
	}
	defer os.Remove(configFilePath)

	var cfg config.Config
	err = commoncfg.LoadConfig(&cfg, nil, testdir)
	if err != nil {
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
	err = mapstructure.Decode(cfg, &cfgMap)
	if err != nil {
		t.Fatalf("failed to decode mapstructure: %s", err)
	}

	f, err := os.Create(configFilePath)
	if err != nil {
		t.Fatalf("failed to create config file: %s", err)
	}
	defer f.Close()

	err = yaml.NewEncoder(f).Encode(cfgMap)
	if err != nil {
		t.Fatalf("failed to write config: %s", err)
	}

	wd, _ := os.Getwd()
	t.Chdir(testdir)
	defer os.Chdir(wd)

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
	err = cmd.Run()
	if err != nil {
		t.Fatalf("process exited abnormally: %s", err)
	}

	t.Log("migrations completed successfully")
}

func TestMigrateIdempotent(t *testing.T) {
	const cmdName = "migrate"
	const configFilePath = "./" + cmdName + "-idempotent-test/config.yaml"
	const dbuser = "postgres"
	const dbpass = "secret"
	const dbname = "session_manager_idempotent"

	ctx := t.Context()
	testdir := filepath.Dir(configFilePath)

	// Create a fresh PostgreSQL container
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
	defer func() { _ = pgContainer.Terminate(ctx) }()

	port, err := pgContainer.MappedPort(ctx, nat.Port("5432"))
	if err != nil {
		t.Fatalf("failed to get mapped port for the PostgreSQL container: %s", err)
	}

	// Prepare config
	_ = os.MkdirAll(testdir, fs.ModePerm)
	defer os.RemoveAll(testdir)

	err = os.WriteFile(configFilePath, []byte(validConfig), fs.ModePerm)
	if err != nil {
		t.Fatalf("failed to write config file: %s", err)
	}
	defer os.Remove(configFilePath)

	var cfg config.Config
	err = commoncfg.LoadConfig(&cfg, nil, testdir)
	if err != nil {
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
	err = mapstructure.Decode(cfg, &cfgMap)
	if err != nil {
		t.Fatalf("failed to decode mapstructure: %s", err)
	}

	f, err := os.Create(configFilePath)
	if err != nil {
		t.Fatalf("failed to create config file: %s", err)
	}
	defer f.Close()

	err = yaml.NewEncoder(f).Encode(cfgMap)
	if err != nil {
		t.Fatalf("failed to write config: %s", err)
	}

	wd, _ := os.Getwd()
	t.Chdir(testdir)
	defer os.Chdir(wd)

	// Run migrations the first time
	cmd := exec.CommandContext(ctx, filepath.Join(currdir, "./session-manager"), cmdName)
	cmdOutPath := filepath.Join(currdir, cmdName+"-idempotent-1.log")
	cmdOut, err := os.Create(cmdOutPath)
	if err != nil {
		t.Fatalf("failed to create a log file: %s", err)
	}
	defer cmdOut.Close()

	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut
	t.Logf("running migrations first time. Logs will be saved into %s", cmdOutPath)
	err = cmd.Run()
	if err != nil {
		t.Fatalf("first migration run failed: %s", err)
	}

	// Run migrations again to test idempotence
	cmd2 := exec.CommandContext(ctx, filepath.Join(currdir, "./session-manager"), cmdName)
	cmdOutPath2 := filepath.Join(currdir, cmdName+"-idempotent-2.log")
	cmdOut2, err := os.Create(cmdOutPath2)
	if err != nil {
		t.Fatalf("failed to create a log file: %s", err)
	}
	defer cmdOut2.Close()

	cmd2.Stdout = cmdOut2
	cmd2.Stderr = cmdOut2
	t.Logf("running migrations second time (idempotence test). Logs will be saved into %s", cmdOutPath2)
	err = cmd2.Run()
	if err != nil {
		t.Fatalf("second migration run failed (idempotence issue): %s", err)
	}

	t.Log("migrations are idempotent - running twice succeeded")
}
