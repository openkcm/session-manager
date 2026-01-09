package server

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/config"
)

func TestInitMeters(t *testing.T) {
	t.Run("initializes meters successfully", func(t *testing.T) {
		ctx := t.Context()
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name: "test-app",
				},
			},
		}

		err := initMeters(ctx, cfg)
		assert.NoError(t, err)
	})
}

func TestNewTraceMiddleware(t *testing.T) {
	t.Run("creates trace middleware", func(t *testing.T) {
		cfg := &config.Config{
			BaseConfig: commoncfg.BaseConfig{
				Application: commoncfg.Application{
					Name: "test-app",
				},
			},
		}

		middleware := newTraceMiddleware(cfg)
		assert.NotNil(t, middleware)
	})
}
