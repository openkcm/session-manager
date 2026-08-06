// Package trustmapping provides the service.module.grpc.trustmapping module:
// a gRPC service module that registers the
// kms.api.cmk.sessionmanager.trustmapping.v1.Service proto onto a
// grpc.ServiceRegistrar supplied by app.module.grpcserver.
package trustmapping

import (
	"fmt"

	"google.golang.org/grpc"

	trustmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/trustmapping/v1"

	sessionmanager "github.com/openkcm/session-manager"
)

const moduleID = "service.module.grpc.trustmapping"

func init() {
	sessionmanager.RegisterModule(new(Module))
}

func newModule() sessionmanager.Module {
	return new(Module)
}

// Module is the service.module.grpc.trustmapping module. It owns a Server
// that implements the trustmapping proto and resolves its single dependency
// (a sessionmanager.Trust implementation) by ID via ctx.GetModule.
type Module struct {
	Mod   string `yaml:"module"`
	Trust string `yaml:"trust" default:"trust.module.oidc" dep:"sessionmanager.Trust"`

	server *Server
}

func (m *Module) Module() sessionmanager.ModuleInfo {
	return sessionmanager.ModuleInfo{
		ID:  moduleID,
		New: newModule,
	}
}

func (m *Module) Provision(ctx *sessionmanager.Context) error {
	trust, err := sessionmanager.GetModuleAs[sessionmanager.Trust](ctx, m.Trust)
	if err != nil {
		return fmt.Errorf("getting trust module %q: %w", m.Trust, err)
	}

	m.server = NewServer(trust)
	return nil
}

func (m *Module) Register(s grpc.ServiceRegistrar) {
	trustmappingv1.RegisterServiceServer(s, m.server)
}
