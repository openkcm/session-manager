package sessionwiring

import (
	"context"
	"errors"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMakeAuditLogger_EmptyEndpointIsNil is a regression test: an empty audit
// configuration (no endpoint) must yield a nil logger rather than an
// unconfigured one that fails later when it tries to deliver events. The
// session.Manager treats a nil audit logger as "auditing disabled"; a non-nil
// logger pointing at no endpoint used to be constructed and then fail on every
// send.
func TestMakeAuditLogger_EmptyEndpointIsNil(t *testing.T) {
	logger, err := makeAuditLogger(&commoncfg.Audit{})

	require.NoError(t, err)
	assert.Nil(t, logger, "empty audit config must produce a nil logger, not an unconfigured one")
}

// TestMakeAuditLogger_WithEndpoint verifies the configured path still builds a
// usable queued logger.
func TestMakeAuditLogger_WithEndpoint(t *testing.T) {
	logger, err := makeAuditLogger(&commoncfg.Audit{Endpoint: "http://audit.example.com"})

	require.NoError(t, err)
	require.NotNil(t, logger, "a configured audit endpoint must produce a logger")
	t.Cleanup(func() { _ = logger.Close(context.Background()) })
}

// TestMakeAuditLogger_InvalidConfig verifies that a construction failure from
// the underlying logger (here: malformed additionalProperties YAML) surfaces as
// an error rather than a half-built logger.
func TestMakeAuditLogger_InvalidConfig(t *testing.T) {
	logger, err := makeAuditLogger(&commoncfg.Audit{
		Endpoint:             "http://audit.example.com",
		AdditionalProperties: "not: [valid: yaml",
	})

	require.Error(t, err)
	assert.Nil(t, logger)
}

// TestCloseAuditLogger_Nil verifies draining a disabled (nil) audit logger is a
// no-op and does not panic — this is the empty-config path from makeAuditLogger.
func TestCloseAuditLogger_Nil(t *testing.T) {
	assert.NotPanics(t, func() { closeAuditLogger(nil) })
}

// TestCloseAuditLogger_Drains verifies a configured logger is closed cleanly on
// shutdown.
func TestCloseAuditLogger_Drains(t *testing.T) {
	logger, err := makeAuditLogger(&commoncfg.Audit{Endpoint: "http://audit.example.com"})
	require.NoError(t, err)
	require.NotNil(t, logger)

	assert.NotPanics(t, func() { closeAuditLogger(logger) })

	// Close is idempotent; a second shutdown must still be safe.
	assert.NotPanics(t, func() { closeAuditLogger(logger) })
}

// TestLogAuditDeliveryError exercises the OnError handler passed to the queued
// logger; it only logs, so we assert it runs without panicking.
func TestLogAuditDeliveryError(t *testing.T) {
	assert.NotPanics(t, func() { logAuditDeliveryError(errors.New("boom")) })
}
