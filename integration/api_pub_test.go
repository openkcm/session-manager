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

func TestApiPub(t *testing.T) {
	const exeName = "api-pub"

	ctx := t.Context()

	istat := initInfra(t, exeName)
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

	cmd := exec.CommandContext(commandCtx, filepath.Join(currdir, "./"+exeName))

	cmdOutPath := filepath.Join(currdir, exeName+".log")
	cmdOut, err := os.Create(cmdOutPath)
	if err != nil {
		t.Fatalf("failed to create an log file")
	}
	defer cmdOut.Close()

	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut
	t.Logf("starting an app process. Logs will be saved into %s", cmdOutPath)
	if err := cmd.Run(); err != nil && !errors.Is(err, context.Canceled) {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && !exitErr.Sys().(syscall.WaitStatus).Signaled() {
			t.Fatalf("process exited abnormally: %s", err)
		}
	}
}
