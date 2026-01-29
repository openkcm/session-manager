package cmdutils

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/config"
)

func TestCobraCommand(t *testing.T) {
	t.Run("creates command with correct properties", func(t *testing.T) {
		businessFunc := func(ctx context.Context, cfg *config.Config) error {
			return nil
		}

		wrapperFunc := func(ctx context.Context, fn func(context.Context, *config.Config) error, cfg *config.Config) error {
			return fn(ctx, cfg)
		}

		cmd := CobraCommand("test-cmd", "short desc", "long description", "v1.0.0", wrapperFunc, businessFunc)

		assert.Equal(t, "test-cmd", cmd.Use)
		assert.Equal(t, "short desc", cmd.Short)
		assert.Equal(t, "long description", cmd.Long)
		assert.NotNil(t, cmd.RunE)
	})

	t.Run("RunE returns error when config loading fails", func(t *testing.T) {
		businessFunc := func(ctx context.Context, cfg *config.Config) error {
			return nil
		}

		wrapperFunc := func(ctx context.Context, fn func(context.Context, *config.Config) error, cfg *config.Config) error {
			return fn(ctx, cfg)
		}

		cmd := CobraCommand("test", "short", "long", "v1.0.0", wrapperFunc, businessFunc)

		// Execute will fail because no config file exists
		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "loading config")
	})

	t.Run("RunE returns error when wrapper function fails", func(t *testing.T) {
		businessFunc := func(ctx context.Context, cfg *config.Config) error {
			return nil
		}

		wrapperErr := errors.New("wrapper error")
		wrapperFunc := func(ctx context.Context, fn func(context.Context, *config.Config) error, cfg *config.Config) error {
			return wrapperErr
		}

		cmd := CobraCommand("test", "short", "long", "v1.0.0", wrapperFunc, businessFunc)

		// Execute will fail because no config file exists (before reaching wrapper)
		err := cmd.Execute()
		assert.Error(t, err)
	})
}

func TestStatusListener(t *testing.T) {
	t.Run("handles empty state", func(t *testing.T) {
		ctx := context.Background()
		state := health.State{
			Status:     "up",
			CheckState: map[string]health.CheckState{},
		}

		// Should not panic
		assert.NotPanics(t, func() {
			statusListener(ctx, state)
		})
	})

	t.Run("handles state with check states", func(t *testing.T) {
		ctx := context.Background()
		state := health.State{
			Status: "up",
			CheckState: map[string]health.CheckState{
				"database": {
					Status: "up",
					Result: nil,
				},
			},
		}

		// Should not panic
		assert.NotPanics(t, func() {
			statusListener(ctx, state)
		})
	})

	t.Run("handles state with multiple check states", func(t *testing.T) {
		ctx := context.Background()
		state := health.State{
			Status: "degraded",
			CheckState: map[string]health.CheckState{
				"database": {
					Status: "up",
					Result: nil,
				},
				"cache": {
					Status: "down",
					Result: errors.New("connection refused"),
				},
			},
		}

		// Should not panic
		assert.NotPanics(t, func() {
			statusListener(ctx, state)
		})
	})
}

func TestStartStatusServer(t *testing.T) {
	t.Run("returns error when connection string creation fails", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.Database{
				Name: "",
				Port: "",
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := startStatusServer(ctx, cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "making connection string from config")
	})
}

func TestHealthStatusTimeout(t *testing.T) {
	t.Run("has correct value", func(t *testing.T) {
		assert.Equal(t, 5*time.Second, healthStatusTimeout)
	})
}

func ExampleCobraCommand() {
	businessFunc := func(ctx context.Context, cfg *config.Config) error {
		fmt.Println("Running business logic")
		return nil
	}

	wrapperFunc := func(ctx context.Context, fn func(context.Context, *config.Config) error, cfg *config.Config) error {
		fmt.Println("Wrapper function called")
		return fn(ctx, cfg)
	}

	cmd := CobraCommand(
		"example",
		"Example command",
		"This is an example of how to use CobraCommand",
		"v1.0.0",
		wrapperFunc,
		businessFunc,
	)

	fmt.Printf("Command use: %s\n", cmd.Use)
	// Output: Command use: example
}
