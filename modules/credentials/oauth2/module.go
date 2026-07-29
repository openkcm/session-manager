// Package oauth2 provides the credentials.module.oauth2 module: a
// credentials.Builder that produces transport credentials for OAuth2/OIDC
// client authentication. Source data lives under sessionManager.clientAuth in
// the top-level config and is read via config.FromContext.
package oauth2

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
)

const moduleID = "credentials.module.oauth2"

const (
	authMTLS             = "mtls"
	authClientSecret     = "client_secret"
	authClientSecretPost = "client_secret_post"
	authInsecure         = "insecure"
)

func init() {
	sessionmanager.RegisterModule(new(Module))
}

func newModule() sessionmanager.Module {
	return new(Module)
}

// Module is the credentials.module.oauth2 module. It exposes a
// credentials.Builder constructed from sessionManager.clientAuth.
type Module struct {
	Mod string `yaml:"module"`

	builder credentials.Builder
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

	clientAuth := cfg.SessionManager.ClientAuth
	switch clientAuth.Type {
	case authMTLS:
		tlsConfig, err := commoncfg.LoadMTLSConfig(clientAuth.MTLS)
		if err != nil {
			return fmt.Errorf("loading mTLS config: %w", err)
		}
		m.builder = func(clientID string) credentials.TransportCredentials {
			return credentials.NewTLS(clientID, tlsConfig)
		}
	case authClientSecret, authClientSecretPost:
		secret, err := commoncfg.LoadValueFromSourceRef(clientAuth.ClientSecret)
		if err != nil {
			return fmt.Errorf("loading client secret: %w", err)
		}
		m.builder = func(clientID string) credentials.TransportCredentials {
			return credentials.NewClientSecretPost(clientID, string(secret))
		}
	case authInsecure:
		slog.Warn("insecure credentials are used. Do not use this in production")
		m.builder = func(clientID string) credentials.TransportCredentials {
			return credentials.NewInsecure(clientID)
		}
	default:
		return fmt.Errorf("unknown client auth type %q", clientAuth.Type)
	}

	return nil
}

// Builder returns the credentials.Builder produced during Provision.
func (m *Module) Builder() credentials.Builder {
	return m.builder
}
