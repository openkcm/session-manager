// Package grpcserver provides the app.module.grpcserver app module: a
// long-running gRPC server that hosts service modules registered through its
// services: config block. The Service interface is exported here so that
// service modules can satisfy it without importing google.golang.org/grpc
// from the top-level sessionmanager package.
package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"google.golang.org/grpc"

	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
)

const moduleID = "app.module.grpcserver"

// Service is the interface that every gRPC service module loaded under an
// app.module.grpcserver entry must satisfy. The grpc app calls Register on
// each child service, in declaration order, before invoking Serve.
type Service interface {
	Register(s grpc.ServiceRegistrar)
}

func init() {
	sessionmanager.RegisterModule(new(Module))
}

func newModule() sessionmanager.Module {
	return new(Module)
}

// Module is the gRPC server app. Its lifecycle:
//
// - Provision: resolve every service module listed under services: the
// services are provisioned before this app by the loader's topological
// order (the app depends on its services), so they are looked up by ID
// rather than loaded here; collect them in declaration order.
//
// - Start: build the underlying *grpc.Server via commongrpc.NewServer using
// the top-level cfg.GRPC block, register every collected service onto it,
// listen on cfg.GRPC.Address, and begin Serve in a goroutine.
//
// - Stop: GracefulStop bounded by cfg.GRPC.ShutdownTimeout; if the timeout
// fires, fall back to a forceful Stop.
type Module struct {
	Mod      string               `yaml:"module"`
	Services []*config.ServiceCfg `yaml:"services"`

	ctx      context.Context //nolint:containedctx
	cfg      *config.Config
	services []Service

	server   *grpc.Server
	listener net.Listener

	stopOnce sync.Once
	stopErr  error

	serveDone chan struct{}
}

func (m *Module) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  moduleID,
		New: newModule,
	}
}

func (m *Module) Provision(ctx *sessionmanager.Context) error {
	cfg, ok := config.FromContext(ctx)
	if !ok {
		return errors.New("config not found in context")
	}
	m.ctx = ctx
	m.cfg = cfg

	if len(m.Services) == 0 {
		return errors.New("app.module.grpcserver requires at least one service under services")
	}

	// Services are provisioned ahead of this app by the loader (the app declares
	// each service as a structural dependency), so resolve them by ID in
	// declaration order rather than loading them here.
	m.services = make([]Service, 0, len(m.Services))
	for i, svcCfg := range m.Services {
		svc, err := sessionmanager.GetModuleAs[Service](ctx, svcCfg.Module())
		if err != nil {
			return fmt.Errorf("resolving service[%d] %q: %w", i, svcCfg.Module(), err)
		}
		m.services = append(m.services, svc)
	}

	return nil
}

func (m *Module) Start() error {
	m.server = commongrpc.NewServer(m.ctx, &m.cfg.GRPC.GRPCServer)

	for _, svc := range m.services {
		svc.Register(m.server)
	}

	listener, err := new(net.ListenConfig).Listen(m.ctx, "tcp", m.cfg.GRPC.Address)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", m.cfg.GRPC.Address, err)
	}
	m.listener = listener

	slogctx.Info(m.ctx, "Starting a gRPC listener", "address", listener.Addr().String())

	m.serveDone = make(chan struct{})
	go func() {
		defer close(m.serveDone)
		if err := m.server.Serve(listener); err != nil {
			slogctx.Error(m.ctx, "gRPC server stopped with error", "error", err)
		}
	}()

	return nil
}

func (m *Module) Stop() error {
	m.stopOnce.Do(func() {
		if m.server == nil {
			return
		}

		gracefulDone := make(chan struct{})
		go func() {
			m.server.GracefulStop()
			close(gracefulDone)
		}()

		timeout := m.cfg.GRPC.ShutdownTimeout
		if timeout <= 0 {
			<-gracefulDone
		} else {
			tctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			select {
			case <-gracefulDone:
			case <-tctx.Done():
				slogctx.Warn(m.ctx, "gRPC graceful stop exceeded timeout; forcing Stop", "timeout", timeout)
				m.server.Stop()
				<-gracefulDone
			}
		}

		if m.serveDone != nil {
			<-m.serveDone
		}
	})
	return m.stopErr
}
