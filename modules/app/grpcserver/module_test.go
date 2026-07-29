package grpcserver_test

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/modules/app/grpcserver"
)

// fakeService satisfies grpcserver.Service. It records that Register was
// called on a non-nil ServiceRegistrar. The test then exercises Start/Stop
// without registering any real proto services.
type fakeService struct {
	mu         sync.Mutex
	registered bool
}

func (f *fakeService) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  "test.fake.service",
		New: func() sessionmanager.Module { return f },
	}
}

func (f *fakeService) Register(_ grpc.ServiceRegistrar) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registered = true
}

// notService is a Module that does NOT implement grpcserver.Service.
type notService struct{ id string }

func (n *notService) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  n.id,
		New: func() sessionmanager.Module { return n },
	}
}

func newCtx(t *testing.T) (*sessionmanager.Context, *config.Config) {
	t.Helper()
	cfg := &config.Config{}
	cfg.GRPC = config.GRPCServer{
		GRPCServer:      commoncfg.GRPCServer{Address: "127.0.0.1:0"},
		ShutdownTimeout: 2 * time.Second,
	}
	// Find a free port the deterministic way.
	l, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	cfg.GRPC.Address = addr

	ctx, cancel := sessionmanager.NewContext(t.Context())
	t.Cleanup(func() { cancel(nil) })
	ctx = config.WithContext(ctx, cfg)
	return ctx, cfg
}

func TestModule_StartRegistersServicesAndStops(t *testing.T) {
	ctx, _ := newCtx(t)

	fakeID := "test.fake.service." + t.Name()
	svc := &fakeService{}
	sessionmanager.RegisterModule(&customMod{id: fakeID, mod: svc})

	m := &grpcserver.Module{
		Services: []*config.ServiceCfg{newSvcCfg(fakeID)},
	}
	require.NoError(t, m.Provision(ctx))
	require.NoError(t, m.Start())

	assert.True(t, svc.registered, "service must be registered before Serve")

	require.NoError(t, m.Stop())

	// Stop is idempotent.
	require.NoError(t, m.Stop())
}

func TestModule_NonServiceUnderServicesIsRejected(t *testing.T) {
	ctx, _ := newCtx(t)

	id := "test.notservice." + t.Name()
	sessionmanager.RegisterModule(&customMod{id: id, mod: &notService{id: id}})

	m := &grpcserver.Module{
		Services: []*config.ServiceCfg{newSvcCfg(id)},
	}
	err := m.Provision(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not implement")
}

func TestModule_EmptyServicesRejected(t *testing.T) {
	ctx, _ := newCtx(t)
	m := &grpcserver.Module{}
	err := m.Provision(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one service")
}

// customMod is a tiny ExtensionConfig + module registration helper.
type customMod struct {
	id  string
	mod sessionmanager.Module
}

func (c *customMod) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  c.id,
		New: func() sessionmanager.Module { return c.mod },
	}
}

// newSvcCfg builds a config.ServiceCfg pointing at the given module ID.
// We bypass koanf entirely; the module just needs Module() to return the ID.
func newSvcCfg(modID string) *config.ServiceCfg {
	c := &config.ServiceCfg{Mod: modID}
	return c
}
