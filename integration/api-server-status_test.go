//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStatusServer(t *testing.T) {
	const cmdName = "api-server"

	ctx := t.Context()

	istat := initInfra(t, cmdName)
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

	cmdOutPath := filepath.Join(currdir, cmdName+"-status.log")
	cmdOut, err := os.Create(cmdOutPath)
	if err != nil {
		t.Fatalf("failed to create an log file")
	}
	defer cmdOut.Close()

	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut

	// start the service in the background
	if err = cmd.Start(); err != nil {
		t.Fatalf("could not start command: %s", err)
	}
	// defer the graceful stop of the service so that coverprofiles are written
	defer func() {
		syscall.Kill(cmd.Process.Pid, syscall.SIGTERM)
		cmd.Wait()
	}()

	// create the test cases
	tests := []struct {
		name      string
		endpoint  string
		wantError bool
	}{
		{
			name:      "get version",
			endpoint:  "version",
			wantError: false,
		}, {
			name:      "get readiness",
			endpoint:  "probe/readiness",
			wantError: false,
		}, {
			name:      "get liveness",
			endpoint:  "probe/liveness",
			wantError: false,
		},
	}

	// give the server some time to start before running the test
	for i := 100; i > 0; i-- {
		if i < 1 {
			t.Fatalf("could not connect to server: %s", err)
		}
		if _, err := http.Get("http://localhost:8888/"); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			resp, err := http.Get("http://localhost:8888/" + tc.endpoint)
			if err != nil {
				t.Fatalf("could not send request: %s", err)
			}
			defer resp.Body.Close()
			got, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("could not read response body: %s", err)
			}

			// Assert
			if tc.wantError {
				if err == nil {
					t.Error("expected error, but got nil")
				}
				if got != nil {
					t.Errorf("expected nil response, but got: %+v", got)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				} else {
					t.Logf("response: %s", got)
					var js json.RawMessage
					if json.Unmarshal([]byte(got), &js) != nil {
						t.Errorf("response is not valid json: %s", got)
					}
				}
			}
		})
	}
}
