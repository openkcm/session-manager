package sessionmanager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionmanager "github.com/openkcm/session-manager"
)

// stubModule is a minimal Module used across tests in this file.
type stubModule struct{ id string }

func (s *stubModule) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  s.id,
		New: func() sessionmanager.Module { return &stubModule{id: s.id} },
	}
}

// newRegistry resets the global module registry by registering a fresh set of
// modules under unique IDs so parallel/serial tests don't interfere with each other.
// It returns a cleanup function that can be deferred.
//
// Because the global map is package-level state, each test that touches the
// registry must use IDs that are unique within the whole test binary run.
func uniqueID(t *testing.T, suffix string) string {
	t.Helper()
	return t.Name() + "/" + suffix
}

func TestRegisterModule_Success(t *testing.T) {
	id := uniqueID(t, "mod")
	sessionmanager.RegisterModule(&stubModule{id: id})

	info, err := sessionmanager.GetModule(id)
	require.NoError(t, err)
	assert.Equal(t, id, info.ID)
}

func TestRegisterModule_DuplicatePanics(t *testing.T) {
	id := uniqueID(t, "mod")
	sessionmanager.RegisterModule(&stubModule{id: id})

	assert.Panics(t, func() {
		sessionmanager.RegisterModule(&stubModule{id: id})
	})
}

func TestGetModule_NotRegistered(t *testing.T) {
	_, err := sessionmanager.GetModule("module-that-does-not-exist")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestModules_ContainsRegistered(t *testing.T) {
	id := uniqueID(t, "mod")
	sessionmanager.RegisterModule(&stubModule{id: id})

	found := false
	for info := range sessionmanager.Modules() {
		if info.ID == id {
			found = true
			break
		}
	}
	assert.True(t, found, "registered module should appear in Modules()")
}

func TestModuleInfo_New(t *testing.T) {
	id := uniqueID(t, "mod")
	sessionmanager.RegisterModule(&stubModule{id: id})

	info, err := sessionmanager.GetModule(id)
	require.NoError(t, err)
	require.NotNil(t, info.New)

	instance := info.New()
	require.NotNil(t, instance)
	assert.Equal(t, id, instance.Module().ID)
}
