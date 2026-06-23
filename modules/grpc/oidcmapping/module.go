// Package oidcmapping provides the service.module.grpc.oidcmapping module:
// a gRPC service module that registers the legacy
// kms.api.cmk.sessionmanager.oidcmapping.v1.Service proto onto a
// grpc.ServiceRegistrar supplied by app.module.grpcserver. It exists for
// backward compatibility with clients that have not migrated to the
// kms.api.cmk.sessionmanager.trustmapping.v1.Service proto.
package oidcmapping

import (
	"fmt"

	"google.golang.org/grpc"

	oidcmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	sessionmanager "github.com/openkcm/session-manager"
)

const moduleID = "service.module.grpc.oidcmapping"

func init() {
	sessionmanager.RegisterModule(new(Module))
}

func newModule() sessionmanager.Module {
	return new(Module)
}

// Module is the service.module.grpc.oidcmapping module. It owns a Server that
// adapts the legacy oidcmapping proto onto sessionmanager.Trust and resolves
// its single dependency by ID via ctx.GetModule.
type Module struct {
	Mod   string `yaml:"module"`
	Trust string `yaml:"trust" default:"trust.module.oidc"`

	server *Server
}

func (m *Module) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  moduleID,
		New: newModule,
	}
}

func (m *Module) Provision(ctx *sessionmanager.Context) error {
	trustMod, err := ctx.GetModule(m.Trust)
	if err != nil {
		return fmt.Errorf("getting trust module %q: %w", m.Trust, err)
	}
	trust, ok := trustMod.(sessionmanager.Trust)
	if !ok {
		return fmt.Errorf("module %q does not implement sessionmanager.Trust", m.Trust)
	}

	m.server = NewServer(trust)
	return nil
}

func (m *Module) Register(s grpc.ServiceRegistrar) {
	oidcmappingv1.RegisterServiceServer(s, m.server)
}
