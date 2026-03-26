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

	istat := initInfra(t)
	defer istat.Close(ctx)

	istat.PreparePostgres(t)
	istat.PrepareValKey(t)
	istat.PrepareConfig(t)

	currdir, err := os.Getwd()
	require.NoError(t, err, "failed to get wd")

	t.Chdir(istat.Procdir)

	commandCtx, cancelCommand := context.WithTimeout(ctx, 10*time.Second)
	defer cancelCommand()

	cmd := exec.CommandContext(commandCtx, filepath.Join(currdir, "./session-manager"), cmdName)
	cmd.WaitDelay = 5 * time.Second
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }

	cmdOutPath := filepath.Join(currdir, cmdName+".log")
	cmdOut, err := os.Create(cmdOutPath)
	if err != nil {
		t.Fatalf("failed to create a log file")
	}
	defer cmdOut.Close()

	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut
	t.Logf("starting housekeeper process. Logs will be saved into %s", cmdOutPath)
	err = cmd.Run()
	if err != nil && !errors.Is(err, context.Canceled) {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && !ws.Signaled() {
				t.Fatalf("process exited abnormally: %s", err)
			}
		}
	}
}
