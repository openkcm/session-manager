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

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: id}}})
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
	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: id}}})
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

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: id}}})
	require.NoError(t, err)
	assert.True(t, pm.provisioned)
}

func TestLoadModule_UnknownModule(t *testing.T) {
	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: "no-such-module"}}})
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

	// The same module ID listed twice in one load set is rejected.
	err := ctx.LoadAll([]sessionmanager.LoadSpec{
		{Cfg: &simpleExtensionConfig{moduleID: id}},
		{Cfg: &simpleExtensionConfig{moduleID: id}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
}

func TestLoadModule_ProvisionError(t *testing.T) {
	id := uniqueID(t, "failprov")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &failingProvisionerModule{stubModule: stubModule{id: id}} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: id}}})
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

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &failingUnmarshalConfig{moduleID: id}}})
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

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: id}}})
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

func TestGetModuleAs_Success(t *testing.T) {
	id := uniqueID(t, "as-ok")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &appModule{stubModule: stubModule{id: id}} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: id}}})
	require.NoError(t, err)

	// appModule satisfies sessionmanager.App, so the typed lookup succeeds.
	app, err := sessionmanager.GetModuleAs[sessionmanager.App](ctx, id)
	require.NoError(t, err)
	assert.NotNil(t, app)
}

func TestGetModuleAs_WrongType(t *testing.T) {
	id := uniqueID(t, "as-wrong")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &stubModule{id: id} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: id}}})
	require.NoError(t, err)

	// stubModule does NOT implement App; the assertion must fail with a
	// descriptive error naming the module ID and expected interface rather
	// than panicking.
	_, err = sessionmanager.GetModuleAs[sessionmanager.App](ctx, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), id)
	assert.Contains(t, err.Error(), "does not implement")
}

func TestGetModuleAs_NotLoaded(t *testing.T) {
	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	// Nothing loaded — the underlying GetModule error propagates.
	_, err := sessionmanager.GetModuleAs[sessionmanager.Trust](ctx, "module-that-is-not-loaded")
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

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: id}, IsApp: true}})
	require.NoError(t, err)

	got, err := ctx.GetApp(id)
	require.NoError(t, err)
	assert.Same(t, am, got)
}

func TestLoadApp_MissingAppInterface(t *testing.T) {
	id := uniqueID(t, "notapp")
	sessionmanager.RegisterModule(&customNewModule{
		id:    id,
		newFn: func() sessionmanager.Module { return &stubModule{id: id} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: id}, IsApp: true}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "App interface")
}

func TestLoadApp_UnknownModule(t *testing.T) {
	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: "no-such-app-module"}, IsApp: true}})
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

	// The same app ID listed twice in one load set is rejected.
	err := ctx.LoadAll([]sessionmanager.LoadSpec{
		{Cfg: &simpleExtensionConfig{moduleID: id}, IsApp: true},
		{Cfg: &simpleExtensionConfig{moduleID: id}, IsApp: true},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
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
	err := ctx.LoadAll([]sessionmanager.LoadSpec{{Cfg: &simpleExtensionConfig{moduleID: id}, IsApp: true}})
	require.NoError(t, err)

	cancel(nil)
	assert.True(t, cam.closed, "Close() should be called on apps when context is cancelled")
}

// orderRecorder is shared across closableOrderModule instances so tests can
// observe Close ordering across the framework's reverse-load-order shutdown.
type orderRecorder struct {
	closes []string
}

// closableOrderModule appends its ID to the recorder when Close is called.
type closableOrderModule struct {
	stubModule

	rec *orderRecorder
}

func (m *closableOrderModule) Close() error {
	m.rec.closes = append(m.rec.closes, m.id)
	return nil
}

func TestLoadAll_ProvisionFailureRollsBackEarlierSiblings(t *testing.T) {
	okID := uniqueID(t, "ok")
	failID := uniqueID(t, "fail")

	// The first module is a Closer that loads cleanly; the second fails during
	// Provision. LoadAll must roll back (Close + deregister) the first before
	// returning the error.
	c1 := &closableModule{stubModule: stubModule{id: okID}}
	sessionmanager.RegisterModule(&customNewModule{
		id:    okID,
		newFn: func() sessionmanager.Module { return c1 },
	})
	sessionmanager.RegisterModule(&customNewModule{
		id:    failID,
		newFn: func() sessionmanager.Module { return &failingProvisionerModule{stubModule: stubModule{id: failID}} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	err := ctx.LoadAll([]sessionmanager.LoadSpec{
		{Cfg: &simpleExtensionConfig{moduleID: okID}},
		{Cfg: &simpleExtensionConfig{moduleID: failID}},
	})
	require.Error(t, err)
	assert.True(t, c1.closed, "earlier sibling must be closed during rollback")

	// The earlier sibling must be removed from the registry.
	_, err = ctx.GetModule(okID)
	require.Error(t, err)

	// The failing module was never registered.
	_, err = ctx.GetModule(failID)
	require.Error(t, err)
}

func TestLoadAll_NonCloserSiblingIsRemovedOnRollback(t *testing.T) {
	plainID := uniqueID(t, "plain")
	failID := uniqueID(t, "fail-after")

	sessionmanager.RegisterModule(&customNewModule{
		id:    plainID,
		newFn: func() sessionmanager.Module { return &stubModule{id: plainID} },
	})
	sessionmanager.RegisterModule(&customNewModule{
		id:    failID,
		newFn: func() sessionmanager.Module { return &failingProvisionerModule{stubModule: stubModule{id: failID}} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	err := ctx.LoadAll([]sessionmanager.LoadSpec{
		{Cfg: &simpleExtensionConfig{moduleID: plainID}},
		{Cfg: &simpleExtensionConfig{moduleID: failID}},
	})
	require.Error(t, err)

	// The non-closer sibling must still be removed from the registry.
	_, err = ctx.GetModule(plainID)
	require.Error(t, err)
}

func TestNewContext_CloseInReverseLoadOrder(t *testing.T) {
	rec := &orderRecorder{}

	idA := uniqueID(t, "ord-A")
	idB := uniqueID(t, "ord-B")
	idC := uniqueID(t, "ord-C")

	for _, id := range []string{idA, idB, idC} {
		sessionmanager.RegisterModule(&customNewModule{
			id:    id,
			newFn: func() sessionmanager.Module { return &closableOrderModule{stubModule: stubModule{id: id}, rec: rec} },
		})
	}

	ctx, cancel := sessionmanager.NewContext(t.Context())

	err := ctx.LoadAll([]sessionmanager.LoadSpec{
		{Cfg: &simpleExtensionConfig{moduleID: idA}},
		{Cfg: &simpleExtensionConfig{moduleID: idB}},
		{Cfg: &simpleExtensionConfig{moduleID: idC}},
	})
	require.NoError(t, err)

	cancel(nil)

	require.Equal(t, []string{idC, idB, idA}, rec.closes,
		"modules must be closed in reverse load order")
}

func TestNewContext_CloseSkipsNonClosersInReverseOrder(t *testing.T) {
	rec := &orderRecorder{}

	idA := uniqueID(t, "mix-A") // closer
	idB := uniqueID(t, "mix-B") // non-closer
	idC := uniqueID(t, "mix-C") // closer

	sessionmanager.RegisterModule(&customNewModule{
		id:    idA,
		newFn: func() sessionmanager.Module { return &closableOrderModule{stubModule: stubModule{id: idA}, rec: rec} },
	})
	sessionmanager.RegisterModule(&customNewModule{
		id:    idB,
		newFn: func() sessionmanager.Module { return &stubModule{id: idB} },
	})
	sessionmanager.RegisterModule(&customNewModule{
		id:    idC,
		newFn: func() sessionmanager.Module { return &closableOrderModule{stubModule: stubModule{id: idC}, rec: rec} },
	})

	ctx, cancel := sessionmanager.NewContext(t.Context())
	err := ctx.LoadAll([]sessionmanager.LoadSpec{
		{Cfg: &simpleExtensionConfig{moduleID: idA}},
		{Cfg: &simpleExtensionConfig{moduleID: idB}},
		{Cfg: &simpleExtensionConfig{moduleID: idC}},
	})
	require.NoError(t, err)

	cancel(nil)
	require.Equal(t, []string{idC, idA}, rec.closes,
		"only Closer modules must be closed, in reverse load order")
}

// Ensure stubModule satisfies the Module interface at compile time.
var _ sessionmanager.Module = (*stubModule)(nil)

// Ensure provisionableModule satisfies Provisioner at compile time.
var _ sessionmanager.Provisioner = (*provisionableModule)(nil)

// Ensure closableModule satisfies io.Closer at compile time.
var _ io.Closer = (*closableModule)(nil)

// Ensure appModule satisfies App at compile time.
var _ sessionmanager.App = (*appModule)(nil)
