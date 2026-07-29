package valkey_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionmanager "github.com/openkcm/session-manager"
	sessionstorevalkey "github.com/openkcm/session-manager/modules/sessionstore/valkey"
)

func TestModule_RegistrationAndID(t *testing.T) {
	info, err := sessionmanager.GetModule("sessionstore.module.valkey")
	require.NoError(t, err)
	assert.Equal(t, "sessionstore.module.valkey", info.ID)

	mod := info.New()
	require.NotNil(t, mod)
	_, ok := mod.(*sessionstorevalkey.Module)
	assert.True(t, ok, "New() must return *Module")
}

func TestModule_CloseBeforeProvisionIsSafe(t *testing.T) {
	m := new(sessionstorevalkey.Module)
	require.NoError(t, m.Close(), "Close before Provision must not error")
}
