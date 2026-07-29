package business

import (
	"context"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/config"
)

func TestHousekeeperMain_CancelledContext(t *testing.T) {
	cfg := &config.Config{
		ValKey: config.ValKey{
			Host:     commoncfg.SourceRef{Source: "embedded", Value: "localhost:6379"},
			User:     commoncfg.SourceRef{Source: "embedded", Value: "user"},
			Password: commoncfg.SourceRef{Source: "embedded", Value: "pass"},
		},
		SessionManager: config.SessionManager{
			ClientAuth: config.ClientAuth{
				Type: "insecure",
			},
		},
	}

	// Use an already cancelled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := HousekeeperMain(ctx, cfg)
	// When context is already cancelled, initSessionManager will fail
	assert.Error(t, err)
}
