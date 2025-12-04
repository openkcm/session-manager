//go:build integration

package integration_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHousekeeper(t *testing.T) {
	const cmdName = "housekeeper"

	ctx := t.Context()

	istat := initInfra(t, cmdName)
	defer istat.Close(ctx)

	istat.PreparePostgres(t)
	istat.PrepareValKey(t)
	istat.PrepareConfig(t)

	currdir, err := os.Getwd()
	require.NoError(t, err, "failed to get wd")

	os.Chdir(istat.Procdir)
	defer os.Chdir(currdir)

	commandCtx, cancelCommand := context.WithTimeout(ctx, 10*time.Second)
	defer cancelCommand()

	cmd := exec.CommandContext(commandCtx, filepath.Join(currdir, "./session-manager"), cmdName)

	cmdOutPath := filepath.Join(currdir, cmdName+".log")
	cmdOut, err := os.Create(cmdOutPath)
	if err != nil {
		t.Fatalf("failed to create a log file")
	}
	defer cmdOut.Close()

	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut
	t.Logf("starting housekeeper process. Logs will be saved into %s", cmdOutPath)
	if err := cmd.Run(); err != nil && !errors.Is(err, context.Canceled) {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && !exitErr.Sys().(syscall.WaitStatus).Signaled() {
			t.Fatalf("housekeeper process exited abnormally: %s", err)
		}
	}
}
