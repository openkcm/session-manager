// Package session provides the service.module.grpc.session module: a gRPC
// service module that registers the kms.api.cmk.sessionmanager.session.v1.Service
// proto onto a grpc.ServiceRegistrar supplied by app.module.grpcserver.
package session

import (
	"errors"
	"fmt"
	"reflect"

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
	sessionmanager.RegisterDepInterface("session.Repository", reflect.TypeFor[internalsession.Repository]())
	sessionmanager.RegisterDepInterface("credentials.Provider", reflect.TypeFor[credentials.Provider]())
}

func newModule() sessionmanager.Module {
	return new(Module)
}

// Module is the service.module.grpc.session module. It wires its three
// dependencies (trust, session store, credentials) by ID via ctx.GetModule
// and owns a *Server that implements the proto.
type Module struct {
	Mod             string `yaml:"module"`
	Trust           string `yaml:"trust"        default:"trust.module.oidc"        dep:"sessionmanager.Trust"`
	SessionStore    string `yaml:"sessionStore" default:"sessionstore.module.valkey" dep:"session.Repository"`
	Credentials     string `yaml:"credentials"  default:"credentials.module.oauth2"  dep:"credentials.Provider"`
	AllowHttpScheme bool   `yaml:"allowHttpScheme"`

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

	trust, err := sessionmanager.GetModuleAs[sessionmanager.Trust](ctx, m.Trust)
	if err != nil {
		return fmt.Errorf("getting trust module %q: %w", m.Trust, err)
	}

	repo, err := sessionmanager.GetModuleAs[internalsession.Repository](ctx, m.SessionStore)
	if err != nil {
		return fmt.Errorf("getting session-store module %q: %w", m.SessionStore, err)
	}

	creds, err := sessionmanager.GetModuleAs[credentials.Provider](ctx, m.Credentials)
	if err != nil {
		return fmt.Errorf("getting credentials module %q: %w", m.Credentials, err)
	}

	opts := []Option{
		WithCredentialsProvider(creds),
		WithAllowHttpScheme(m.AllowHttpScheme),
	}

	m.server = NewServer(
		ctx,
		repo,
		trust,
		cfg.SessionManager.IdleSessionTimeout,
		opts...,
	)

	return nil
}

func (m *Module) Register(s grpc.ServiceRegistrar) {
	sessionv1.RegisterServiceServer(s, m.server)
}
