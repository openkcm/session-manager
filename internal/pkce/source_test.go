package pkce

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSource_PKCE(t *testing.T) {
	p := Source{}
	pkce := p.PKCE()
	assert.NotEmpty(t, pkce.Verifier, "Empty pkce verifier")
	assert.NotEmpty(t, pkce.Challenge, "Empty pkce challenge")
	assert.Equal(t, MethodS256, pkce.Method, "Unexpected PKCE method")
}

func TestSource_State(t *testing.T) {
	p := Source{}
	state := p.State()
	assert.NotEmpty(t, state, "Empty state generated")
	assert.Len(t, state, 64, "State should be 64 characters long")
}

func TestSource_SessionID(t *testing.T) {
	p := Source{}
	sessionID := p.SessionID()
	assert.NotEmpty(t, sessionID, "Empty session ID generated")
	assert.Len(t, sessionID, 32, "Session ID should be 32 characters long")

	// Test uniqueness - generate multiple session IDs and ensure they're different
	sessionID2 := p.SessionID()
	assert.NotEqual(t, sessionID, sessionID2, "Session IDs should be unique")
}
