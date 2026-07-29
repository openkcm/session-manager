package sessionmanager_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionmanager "github.com/openkcm/session-manager"
)

// provisionableModule records whether Provision was called.
type provisionableModule struct {
	stubModule

	provisioned bool
}

func (m *provisionableModule) Provision(_ *sessionmanager.Context) error {
	m.provisioned = true
	return nil
}

// failingProvisionerModule always returns an error from Provision.
type failingProvisionerModule struct{ stubModule }

func (m *failingProvisionerModule) Provision(_ *sessionmanager.Context) error {
	return errors.New("provision failed")
}

// closableModule records whether Close was called.
type closableModule struct {
	stubModule

	closed bool
}

func (m *closableModule) Close() error {
	m.closed = true
	return nil
}

// closeErrModule returns an error from Close (exercises the error-log path).
type closeErrModule struct{ stubModule }

func (m *closeErrModule) Close() error { return errors.New("close error") }

// simpleExtensionConfig is a minimal ExtensionConfig that references a registered module.
type simpleExtensionConfig struct{ moduleID string }

func (c *simpleExtensionConfig) Module() string                                   { return c.moduleID }
func (c *simpleExtensionConfig) UnmarshalExtension(_ sessionmanager.Module) error { return nil }

// failingUnmarshalConfig returns an error from UnmarshalExtension.
type failingUnmarshalConfig struct{ moduleID string }

func (c *failingUnmarshalConfig) Module() string { return c.moduleID }
func (c *failingUnmarshalConfig) UnmarshalExtension(_ sessionmanager.Module) error {
	return errors.New("unmarshal failed")
}

// customNewModule registers a module whose New() function delegates to newFn.
type customNewModule struct {
	id    string
	newFn func() sessionmanager.Module
}

func (m *customNewModule) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{ID: m.id, New: m.newFn}
}

func TestNewContext_CancelCloseModules(t *testing.T) {
	id := uniqueID(t, "closable")
	cm := &closableModule{stubModule: stubModule{id: id}}

	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return cm },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())

	_, err := ctx.LoadModule(&simpleExtensionConfig{moduleID: id})
	require.NoError(t, err)

	cancel(nil)
	assert.True(t, cm.closed, "Close() should be called when context is cancelled")
}

func TestNewContext_CancelWithCause(t *testing.T) {
	ctx, cancel := sessionmanager.NewContext(t.Context())

	cause := errors.New("test cause")
	cancel(cause)

	assert.ErrorIs(t, context.Cause(ctx), cause)
}

func TestNewContext_CloseErrorIsHandled(t *testing.T) {
	id := uniqueID(t, "closeerr")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &closeErrModule{stubModule: stubModule{id: id}} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	_, err := ctx.LoadModule(&simpleExtensionConfig{moduleID: id})
	require.NoError(t, err)

	// Should not panic even though Close() returns an error.
	assert.NotPanics(t, func() { cancel(nil) })
}

func TestContext_WithValue(t *testing.T) {
	type ctxKey struct{}
	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	ctx2 := ctx.WithValue(ctxKey{}, "hello")
	assert.Equal(t, "hello", ctx2.Value(ctxKey{}))
	// Original context should not carry the value.
	assert.Nil(t, ctx.Value(ctxKey{}))
}

func TestLoadModule_Success(t *testing.T) {
	id := uniqueID(t, "prov")
	pm := &provisionableModule{stubModule: stubModule{id: id}}

	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return pm },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	mod, err := ctx.LoadModule(&simpleExtensionConfig{moduleID: id})
	require.NoError(t, err)
	require.NotNil(t, mod)
	assert.True(t, pm.provisioned)
}

func TestLoadModule_UnknownModule(t *testing.T) {
	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := ctx.LoadModule(&simpleExtensionConfig{moduleID: "no-such-module"})
	require.Error(t, err)
}

func TestLoadModule_DuplicateReturnsError(t *testing.T) {
	id := uniqueID(t, "dup")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &stubModule{id: id} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := ctx.LoadModule(&simpleExtensionConfig{moduleID: id})
	require.NoError(t, err)

	_, err = ctx.LoadModule(&simpleExtensionConfig{moduleID: id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already been loaded")
}

func TestLoadModule_ProvisionError(t *testing.T) {
	id := uniqueID(t, "failprov")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &failingProvisionerModule{stubModule: stubModule{id: id}} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := ctx.LoadModule(&simpleExtensionConfig{moduleID: id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provision failed")
}

func TestLoadModule_UnmarshalError(t *testing.T) {
	id := uniqueID(t, "unmarshalerr")
	// Use a pointer-to-struct module so the unmarshal branch is reached.
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &stubModule{id: id} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := ctx.LoadModule(&failingUnmarshalConfig{moduleID: id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal failed")
}

func TestGetModule_AfterLoad(t *testing.T) {
	id := uniqueID(t, "get")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &stubModule{id: id} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := ctx.LoadModule(&simpleExtensionConfig{moduleID: id})
	require.NoError(t, err)

	mod, err := ctx.GetModule(id)
	require.NoError(t, err)
	assert.NotNil(t, mod)
}

func TestGetModule_NotLoaded(t *testing.T) {
	id := uniqueID(t, "notloaded")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &stubModule{id: id} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	// Never call LoadModule — GetModule should return an error.
	_, err := ctx.GetModule(id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

// appModule is a Module that also satisfies the App interface.
type appModule struct {
	stubModule

	started bool
	stopped bool
}

func (a *appModule) Start() error {
	a.started = true
	return nil
}

func (a *appModule) Stop() error {
	a.stopped = true
	return nil
}

// closableAppModule is an App that also satisfies io.Closer.
type closableAppModule struct {
	appModule

	closed bool
}

func (a *closableAppModule) Close() error {
	a.closed = true
	return nil
}

func TestLoadApp_Success(t *testing.T) {
	id := uniqueID(t, "app")
	am := &appModule{stubModule: stubModule{id: id}}

	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return am },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	app, err := ctx.LoadApp(&simpleExtensionConfig{moduleID: id})
	require.NoError(t, err)
	require.NotNil(t, app)

	got, err := ctx.GetApp(id)
	require.NoError(t, err)
	assert.Same(t, app, got)
}

func TestLoadApp_MissingAppInterface(t *testing.T) {
	id := uniqueID(t, "notapp")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &stubModule{id: id} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := ctx.LoadApp(&simpleExtensionConfig{moduleID: id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "App interface")
}

func TestLoadApp_UnknownModule(t *testing.T) {
	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := ctx.LoadApp(&simpleExtensionConfig{moduleID: "no-such-app-module"})
	require.Error(t, err)
}

func TestLoadApp_DuplicateReturnsError(t *testing.T) {
	id := uniqueID(t, "dupapp")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &appModule{stubModule: stubModule{id: id}} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := ctx.LoadApp(&simpleExtensionConfig{moduleID: id})
	require.NoError(t, err)

	_, err = ctx.LoadApp(&simpleExtensionConfig{moduleID: id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already been loaded")
}

func TestGetApp_NotLoaded(t *testing.T) {
	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := ctx.GetApp("never-loaded")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestNewContext_CancelClosesApps(t *testing.T) {
	id := uniqueID(t, "closableapp")
	cam := &closableAppModule{appModule: appModule{stubModule: stubModule{id: id}}}

	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return cam },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	_, err := ctx.LoadApp(&simpleExtensionConfig{moduleID: id})
	require.NoError(t, err)

	cancel(nil)
	assert.True(t, cam.closed, "Close() should be called on apps when context is cancelled")
}

// Ensure stubModule satisfies the Module interface at compile time.
var _ sessionmanager.Module = (*stubModule)(nil)

// Ensure provisionableModule satisfies Provisioner at compile time.
var _ sessionmanager.Provisioner = (*provisionableModule)(nil)

// Ensure closableModule satisfies io.Closer at compile time.
var _ io.Closer = (*closableModule)(nil)

// Ensure appModule satisfies App at compile time.
var _ sessionmanager.App = (*appModule)(nil)
