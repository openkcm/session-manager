//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/dbtest/postgrestest"
	"github.com/openkcm/session-manager/internal/dbtest/valkeytest"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type closeFunc func(ctx context.Context)

type infraStat struct {
	PostgresPort   nat.Port
	ValKeyPort     nat.Port
	ConfigFilePath string
	Procdir        string
	Cfg            config.Config

	closeFuncs []closeFunc
}

func initInfra(t *testing.T, exeName string) (istat infraStat) {
	t.Helper()

	// Since the config is read from the file $PWD/config.yaml,
	// we're running a process in a subdirectory so that we aren't interferring with the other tests.
	wd, err := os.Getwd()
	require.NoError(t, err, "failed to get wd")
	istat.Procdir = filepath.Join(wd, exeName+"-test")
	istat.ConfigFilePath = filepath.Join(istat.Procdir, "config.yaml")

	// Prepare a directory for the test
	err = os.MkdirAll(istat.Procdir, fs.ModePerm)
	require.NoError(t, err, "failed to create a dir for the process")

	err = os.WriteFile(istat.ConfigFilePath, []byte(validConfig), fs.ModePerm)
	require.NoError(t, err, "failed to write config file")

	err = commoncfg.LoadConfig(&istat.Cfg, nil, istat.Procdir)
	require.NoError(t, err, "failed to load config")

	// Let OS choose a free port
	istat.Cfg.HTTP.Address = "unix://" + filepath.Join(istat.Procdir, exeName+".sock")
	fmt.Println("HTTP Address is: ", istat.Cfg.HTTP.Address)
	istat.Cfg.GRPC.Address = ":0"

	return istat
}

func (istat *infraStat) PreparePostgres(t *testing.T) {
	const dbuser = "postgres"
	const dbpass = "secret"
	const dbname = "session_manager"

	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err, "getting wd")

	pgClient, pgPort, pgTerminate := postgrestest.Start(t.Context())
	pgClient.Close()

	istat.PostgresPort = pgPort
	istat.closeFuncs = append(istat.closeFuncs, pgTerminate)

	istat.Cfg.Database.Name = dbname
	istat.Cfg.Database.User = commoncfg.SourceRef{Source: "embedded", Value: dbuser}
	istat.Cfg.Database.Password = commoncfg.SourceRef{Source: "embedded", Value: dbpass}
	istat.Cfg.Database.Host = commoncfg.SourceRef{Source: "embedded", Value: "localhost"}
	istat.Cfg.Database.Port = pgPort.Port()
	istat.Cfg.Migrate.Source = "file://" + filepath.Join(wd, "../sql")
}

func (istat *infraStat) PrepareValKey(t *testing.T) {
	t.Helper()

	vkClient, vkPort, vkTerminate := valkeytest.Start(t.Context())
	vkClient.Close()

	istat.ValKeyPort = vkPort
	istat.closeFuncs = append(istat.closeFuncs, vkTerminate)

	istat.Cfg.ValKey.Host = commoncfg.SourceRef{Source: "embedded", Value: net.JoinHostPort("localhost", vkPort.Port())}
	istat.Cfg.ValKey.User = commoncfg.SourceRef{Source: "embedded", Value: ""}
	istat.Cfg.ValKey.Password = commoncfg.SourceRef{Source: "embedded", Value: ""}
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
