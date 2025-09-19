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
}
