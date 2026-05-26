// Package session provides the service.module.grpc.session module: a gRPC
// service module that registers the kms.api.cmk.sessionmanager.session.v1.Service
// proto onto a grpc.ServiceRegistrar supplied by app.module.grpcserver.
package session

import (
	"errors"
	"fmt"

	"google.golang.org/grpc"

	sessionv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/session/v1"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
	internalsession "github.com/openkcm/session-manager/internal/session"
)

const moduleID = "service.module.grpc.session"

func init() {
	sessionmanager.RegisterModule(new(Module))
}

func newModule() sessionmanager.Module {
	return new(Module)
}

// credentialsBuilder is the interface satisfied by a credentials module
// (e.g. credentials.module.oauth2).
type credentialsBuilder interface {
	Builder() credentials.Builder
}

// Module is the service.module.grpc.session module. It wires its three
// dependencies (trust, session store, credentials) by ID via ctx.GetModule
// and owns a *Server that implements the proto.
type Module struct {
	Mod          string `yaml:"module"`
	Trust        string `yaml:"trust"        default:"trust.module.oidc"`
	SessionStore string `yaml:"sessionStore" default:"sessionstore.module.valkey"`
	Credentials  string `yaml:"credentials"  default:"credentials.module.oauth2"`

	AllowHttpScheme           bool     `yaml:"allowHttpScheme"`
	QueryParametersIntrospect []string `yaml:"queryParametersIntrospect"`

	server *Server
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

	trustMod, err := ctx.GetModule(m.Trust)
	if err != nil {
		return fmt.Errorf("getting trust module %q: %w", m.Trust, err)
	}
	trust, ok := trustMod.(sessionmanager.Trust)
	if !ok {
		return fmt.Errorf("module %q does not implement sessionmanager.Trust", m.Trust)
	}

	storeMod, err := ctx.GetModule(m.SessionStore)
	if err != nil {
		return fmt.Errorf("getting session-store module %q: %w", m.SessionStore, err)
	}
	repo, ok := storeMod.(internalsession.Repository)
	if !ok {
		return fmt.Errorf("module %q does not implement session.Repository", m.SessionStore)
	}

	credsMod, err := ctx.GetModule(m.Credentials)
	if err != nil {
		return fmt.Errorf("getting credentials module %q: %w", m.Credentials, err)
	}
	creds, ok := credsMod.(credentialsBuilder)
	if !ok {
		return fmt.Errorf("module %q does not expose Builder()", m.Credentials)
	}

	opts := []Option{
		WithTransportCredentials(creds.Builder()),
		WithAllowHttpScheme(m.AllowHttpScheme),
	}
	if m.QueryParametersIntrospect != nil {
		opts = append(opts, WithQueryParametersIntrospect(m.QueryParametersIntrospect))
	}

	m.server = NewServer(
		ctx,
		repo,
		trust,
		cfg.SessionManager.IdleSessionTimeout,
		cfg.SessionManager.ClientAuth.ClientID,
		opts...,
	)

	return nil
}

func (m *Module) Register(s grpc.ServiceRegistrar) {
	sessionv1.RegisterServiceServer(s, m.server)
}
