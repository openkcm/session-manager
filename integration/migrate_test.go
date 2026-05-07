//go:build integration

package integration_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/goccy/go-yaml"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
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
	defer func() { _ = pgContainer.Terminate(ctx) }()

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get mapped port for the PostgreSQL container: %s", err)
	}

	// Prepare config
	currdir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %s", err)
	}

	abstestdir := filepath.Join(currdir, testdir)
	_ = os.MkdirAll(abstestdir, fs.ModePerm)
	defer os.RemoveAll(abstestdir)

	absConfigFilePath := filepath.Join(currdir, configFilePath)
	err = os.WriteFile(absConfigFilePath, []byte(validConfig), fs.ModePerm)
	if err != nil {
		t.Fatalf("failed to write config file: %s", err)
	}

	cfg := loadExtendedConfig(t, abstestdir)
	cfg.Logger.Level = "debug"
	cfg.Logger.Format = commoncfg.TextLoggerFormat
	cfg.Logger.Formatter.Time.Type = commoncfg.PatternTimeLogger
	cfg.Logger.Formatter.Time.Pattern = time.Stamp

	cfg.Database.Name = dbname
	cfg.Database.User = commoncfg.SourceRef{Source: "embedded", Value: dbuser}
	cfg.Database.Password = commoncfg.SourceRef{Source: "embedded", Value: dbpass}
	cfg.Database.Host = commoncfg.SourceRef{Source: "embedded", Value: "localhost"}
	cfg.Database.Port = port.Port()

	cfgMap := make(map[any]any)
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &cfgMap,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc()),
		WeaklyTypedInput: true,
		TagName:          "yaml",
		SquashTagOption:  "inline",
	})
	if err != nil {
		t.Fatalf("failed to create mapstructure decoder: %s", err)
	}
	if err := decoder.Decode(cfg); err != nil {
		t.Fatalf("failed to decode mapstructure: %s", err)
	}

	f, err := os.Create(absConfigFilePath)
	if err != nil {
		t.Fatalf("failed to create config file: %s", err)
	}
	defer f.Close()

	err = yaml.NewEncoder(f).Encode(cfgMap)
	if err != nil {
		t.Fatalf("failed to write config: %s", err)
	}

	t.Chdir(abstestdir)

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

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get mapped port for the PostgreSQL container: %s", err)
	}

	// Prepare config
	currdir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %s", err)
	}

	abstestdir := filepath.Join(currdir, testdir)
	_ = os.MkdirAll(abstestdir, fs.ModePerm)
	defer os.RemoveAll(abstestdir)

	absConfigFilePath := filepath.Join(currdir, configFilePath)
	err = os.WriteFile(absConfigFilePath, []byte(validConfig), fs.ModePerm)
	if err != nil {
		t.Fatalf("failed to write config file: %s", err)
	}

	cfg := loadExtendedConfig(t, abstestdir)
	cfg.Logger.Level = "debug"
	cfg.Logger.Format = commoncfg.TextLoggerFormat
	cfg.Logger.Formatter.Time.Type = commoncfg.PatternTimeLogger
	cfg.Logger.Formatter.Time.Pattern = time.Stamp

	cfg.Database.Name = dbname
	cfg.Database.User = commoncfg.SourceRef{Source: "embedded", Value: dbuser}
	cfg.Database.Password = commoncfg.SourceRef{Source: "embedded", Value: dbpass}
	cfg.Database.Host = commoncfg.SourceRef{Source: "embedded", Value: "localhost"}
	cfg.Database.Port = port.Port()

	cfgMap := make(map[any]any)
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &cfgMap,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc()),
		WeaklyTypedInput: true,
		TagName:          "yaml",
		SquashTagOption:  "inline",
	})
	if err != nil {
		t.Fatalf("failed to create mapstructure decoder: %s", err)
	}
	if err := decoder.Decode(cfg); err != nil {
		t.Fatalf("failed to decode mapstructure: %s", err)
	}

	f, err := os.Create(absConfigFilePath)
	if err != nil {
		t.Fatalf("failed to create config file: %s", err)
	}
	defer f.Close()

	err = yaml.NewEncoder(f).Encode(cfgMap)
	if err != nil {
		t.Fatalf("failed to write config: %s", err)
	}

	t.Chdir(abstestdir)

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
