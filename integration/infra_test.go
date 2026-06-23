//go:build integration

package integration_test

import (
	"context"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/moby/moby/api/types/network"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/dbtest/postgrestest"
	"github.com/openkcm/session-manager/internal/dbtest/valkeytest"
	"github.com/openkcm/session-manager/modules/database/pgxpool"
)

type closeFunc func(ctx context.Context)

type Config struct {
	config.Config `yaml:",inline"`

	Database pgxpool.PostgresModule `yaml:"database"`
}

func loadExtendedConfig(t *testing.T, dir string) *Config {
	t.Helper()

	cfg, err := config.Load("", dir)
	require.NoError(t, err, "failed to load config")

	return &Config{Config: *cfg}
}

type infraStat struct {
	PostgresPort   network.Port
	ValKeyPort     network.Port
	ConfigFilePath string
	Procdir        string
	Cfg            *Config

	closeFuncs []closeFunc
}

func initInfra(t *testing.T) (istat infraStat) {
	t.Helper()

	// Since the config is read from the file $PWD/config.yaml,
	// we're running a process in a temporary subdirectory
	// so that we aren't interfering with the other tests.
	procDir := t.TempDir()
	istat.Procdir = procDir
	istat.ConfigFilePath = filepath.Join(procDir, "config.yaml")

	err := os.WriteFile(istat.ConfigFilePath, []byte(validConfig), fs.ModePerm)
	require.NoError(t, err, "failed to write config file")

	istat.Cfg = loadExtendedConfig(t, istat.Procdir)

	// Let OS choose a free port
	istat.Cfg.HTTP.Address = "unix://" + filepath.Join(procDir, "unix.sock")
	istat.Cfg.GRPC.Address = ":0"
	istat.Cfg.Logger.Format = commoncfg.TextLoggerFormat
	istat.Cfg.Logger.Level = "debug"
	istat.Cfg.Logger.Formatter.Time.Type = commoncfg.PatternTimeLogger
	istat.Cfg.Logger.Formatter.Time.Pattern = time.Stamp

	// There's a hard limit of 108 symbols on a unix socket filepath on Linux/macOS.
	if len(istat.Cfg.HTTP.Address) > 108 {
		t.Fatal("Unix socket path is too long")
	}

	return istat
}

func (istat *infraStat) PreparePostgres(t *testing.T) {
	t.Helper()

	const dbuser = "postgres"
	const dbpass = "secret"
	const dbname = "session_manager"

	pgClient, pgPort, pgTerminate := postgrestest.Start(t.Context())
	pgClient.Close()

	istat.PostgresPort = pgPort
	istat.closeFuncs = append(istat.closeFuncs, pgTerminate)

	istat.Cfg.Database.Mod = "database.module.pgxpool"
	istat.Cfg.Database.Name = dbname
	istat.Cfg.Database.User = commoncfg.SourceRef{Source: "embedded", Value: dbuser}
	istat.Cfg.Database.Password = commoncfg.SourceRef{Source: "embedded", Value: dbpass}
	istat.Cfg.Database.Host = commoncfg.SourceRef{Source: "embedded", Value: "localhost"}
	istat.Cfg.Database.Port = pgPort.Port()
}

func (istat *infraStat) PrepareValKey(t *testing.T) valkey.Client {
	t.Helper()

	vkClient, vkPort, vkTerminate := valkeytest.Start(t.Context())

	istat.ValKeyPort = vkPort
	istat.closeFuncs = append(istat.closeFuncs, vkTerminate, func(_ context.Context) { vkClient.Close() })

	istat.Cfg.ValKey.Host = commoncfg.SourceRef{Source: "embedded", Value: net.JoinHostPort("localhost", vkPort.Port())}
	istat.Cfg.ValKey.User = commoncfg.SourceRef{Source: "embedded", Value: ""}
	istat.Cfg.ValKey.Password = commoncfg.SourceRef{Source: "embedded", Value: ""}

	return vkClient
}

// PrepareConfig writes a config file for running the test into the ConfigFilePath.
func (istat *infraStat) PrepareConfig(t *testing.T) {
	t.Helper()

	configFile, err := os.Create(istat.ConfigFilePath)
	require.NoError(t, err, "failed to create config file")

	err = yaml.NewEncoder(configFile).Encode(istat.Cfg)
	require.NoError(t, err, "failed to write config")
	configFile.Close()
}

func (istat *infraStat) Close(ctx context.Context) {
	os.Remove(istat.ConfigFilePath)
	os.RemoveAll(istat.Procdir)

	for _, close := range istat.closeFuncs {
		close(ctx)
	}
}
