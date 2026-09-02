// Package oauth2 provides the credentials.module.oauth2 module: a
// credentials.Provider that produces transport credentials for OAuth2/OIDC
// client authentication. Source data lives under sessionManager.clientAuth in
// the top-level config and is read via config.FromContext.
package oauth2

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/zitadel/oidc/pkg/client/rp"
	"github.com/zitadel/oidc/pkg/client/rs"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
)

type unknownAuthTypeError struct {
	typ string
}

func (e unknownAuthTypeError) Error() string {
	return fmt.Sprintf("unknown client auth type %q", e.typ)
}

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

// Module is the credentials.module.oauth2 module. It implements credentials.Provider internface
// that builds credentials from sessionManager.clientAuth config field.
type Module struct {
	Mod string `yaml:"module"`

	typ       string
	secret    string
	tlsConfig *tls.Config
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
	m.typ = clientAuth.Type
	switch m.typ {
	case authMTLS:
		tlsConfig, err := commoncfg.LoadMTLSConfig(clientAuth.MTLS)
		if err != nil {
			return fmt.Errorf("loading mTLS config: %w", err)
		}
		if clientAuth.AllowTLSRenegotiationOnce {
			tlsConfig.Renegotiation = tls.RenegotiateOnceAsClient
		}

		m.tlsConfig = tlsConfig
	case authClientSecret, authClientSecretPost:
		secret, err := commoncfg.LoadValueFromSourceRef(clientAuth.ClientSecret)
		if err != nil {
			return fmt.Errorf("loading client secret: %w", err)
		}

		m.secret = string(secret)
	case authInsecure:
		slog.Warn("insecure credentials are used. Do not use this in production")
	default:
		return unknownAuthTypeError{typ: m.typ}
	}

	return nil
}

// ResourceServer implements [credentials.Provider]
func (m *Module) ResourceServer(clientID, issuer string) (rs.ResourceServer, error) {
	switch m.typ {
	case authMTLS:
		return credentials.NewTLSRS(clientID, issuer, m.tlsConfig)
	case authClientSecret, authClientSecretPost:
		return credentials.NewClientSecretPostRS(issuer, clientID, m.secret)
	case authInsecure:
		return credentials.NewInsecureRS(clientID, issuer)
	default:
		return nil, unknownAuthTypeError{typ: m.typ}
	}
}

// RelyingParty implements [credentials.Provider]
func (m *Module) RelyingParty(oidc *oidcv1.OIDC, redirectURI string) (rp.RelyingParty, error) {
	switch m.typ {
	case authMTLS:
		return credentials.NewTLSRP(oidc.GetClientId(), oidc.GetIssuer(), redirectURI, m.tlsConfig)
	case authClientSecret, authClientSecretPost:
		return credentials.NewClientSecretPostRP(oidc.GetIssuer(), oidc.GetClientId(), m.secret, redirectURI)
	case authInsecure:
		return credentials.NewInsecureRP(oidc.GetClientId(), oidc.GetIssuer(), redirectURI)
	default:
		return nil, unknownAuthTypeError{typ: m.typ}
	}
}
