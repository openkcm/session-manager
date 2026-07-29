package business

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
)

// fakeApp is an App whose Start/Stop record their relative order across all
// fakeApp instances sharing a counter, so tests can assert ordering.
type fakeApp struct {
	id          string
	counter     *atomic.Int64
	startErr    error
	stopErr     error
	startOrder  int64
	stopOrder   int64
	startCalled bool
	stopCalled  bool
}

func (a *fakeApp) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  a.id,
		New: func() sessionmanager.Module { return a },
	}
}

func (a *fakeApp) Start() error {
	a.startCalled = true
	a.startOrder = a.counter.Add(1)
	return a.startErr
}

func (a *fakeApp) Stop() error {
	a.stopCalled = true
	a.stopOrder = a.counter.Add(1)
	return a.stopErr
}

// registerFakeApps registers the given apps under unique module IDs scoped to
// the test name and returns a config.Config wired to load them in the order
// supplied. The returned cfg.AppsOrder pins the start order so tests don't
// rely on Go's map iteration order.
func registerFakeApps(t *testing.T, apps ...*fakeApp) *config.Config {
	t.Helper()

	cfg := &config.Config{
		Apps:      make(map[string]*config.App, len(apps)),
		AppsOrder: make([]string, 0, len(apps)),
	}

	for i, app := range apps {
		modID := fmt.Sprintf("test.app.%s.%d", t.Name(), i)
		app.id = modID

		// Capture for closure — RegisterModule's New() must hand back this
		// instance so the test can assert against it.
		captured := app
		sessionmanager.RegisterModule(&moduleInfoStub{
			id:  modID,
			new: func() sessionmanager.Module { return captured },
		})

		name := fmt.Sprintf("app-%d", i)
		cfg.Apps[name] = &config.App{Mod: modID}
		cfg.AppsOrder = append(cfg.AppsOrder, name)
	}

	return cfg
}

// moduleInfoStub adapts a (id, new) pair into the Module + ModuleInfo
// registration surface required by sessionmanager.RegisterModule.
type moduleInfoStub struct {
	id  string
	new func() sessionmanager.Module
}

func (s *moduleInfoStub) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{ID: s.id, New: s.new}
}

func TestStartApps_OrderAndReverseStop(t *testing.T) {
	var counter atomic.Int64
	a := &fakeApp{counter: &counter}
	b := &fakeApp{counter: &counter}

	cfg := registerFakeApps(t, a, b)

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	stop, err := startApps(ctx, cfg)
	require.NoError(t, err)
	require.True(t, a.startCalled)
	require.True(t, b.startCalled)
	assert.Less(t, a.startOrder, b.startOrder, "A.Start must be called before B.Start")

	require.NoError(t, stop())
	assert.Less(t, b.stopOrder, a.stopOrder, "B.Stop must be called before A.Stop")
}

func TestStartApps_StartFailureRollsBack(t *testing.T) {
	var counter atomic.Int64
	wantErr := errors.New("boom")
	a := &fakeApp{counter: &counter}
	b := &fakeApp{counter: &counter, startErr: wantErr}

	cfg := registerFakeApps(t, a, b)

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	stop, err := startApps(ctx, cfg)
	require.Error(t, err)
	require.ErrorIs(t, err, wantErr)
	assert.Nil(t, stop)

	assert.True(t, a.startCalled)
	assert.True(t, a.stopCalled, "successfully-started app must be rolled back on Start failure")
	assert.True(t, b.startCalled)
	assert.False(t, b.stopCalled, "failed-to-start app must not have Stop called")
}

func TestStartApps_FirstAppFailsNoStop(t *testing.T) {
	var counter atomic.Int64
	wantErr := errors.New("boom")
	a := &fakeApp{counter: &counter, startErr: wantErr}
	b := &fakeApp{counter: &counter}

	cfg := registerFakeApps(t, a, b)

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	_, err := startApps(ctx, cfg)
	require.ErrorIs(t, err, wantErr)
	assert.False(t, a.stopCalled)
	assert.False(t, b.startCalled)
	assert.False(t, b.stopCalled)
}

func TestStartApps_StopErrorsAggregated(t *testing.T) {
	var counter atomic.Int64
	stopErrA := errors.New("stop-a")
	stopErrB := errors.New("stop-b")
	a := &fakeApp{counter: &counter, stopErr: stopErrA}
	b := &fakeApp{counter: &counter, stopErr: stopErrB}

	cfg := registerFakeApps(t, a, b)

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)

	stop, err := startApps(ctx, cfg)
	require.NoError(t, err)

	err = stop()
	require.Error(t, err)
	assert.ErrorIs(t, err, stopErrA)
	assert.ErrorIs(t, err, stopErrB)
}

func TestAppsStartOrder_AppsOrderTakesPrecedence(t *testing.T) {
	cfg := &config.Config{
		Apps: map[string]*config.App{
			"a": {}, "b": {}, "c": {},
		},
		AppsOrder: []string{"c", "a"},
	}

	got, err := appsStartOrder(cfg)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "c", got[0])
	assert.Equal(t, "a", got[1])
	// "b" wasn't listed, so it appears last in map iteration order.
	assert.Equal(t, "b", got[2])
}

func TestAppsStartOrder_RejectsUnknownAppName(t *testing.T) {
	cfg := &config.Config{
		Apps:      map[string]*config.App{"a": {}},
		AppsOrder: []string{"missing"},
	}

	_, err := appsStartOrder(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}
