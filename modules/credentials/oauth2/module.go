// Package oauth2 provides the credentials.module.oauth2 module: a
// credentials.Provider that produces transport credentials for OAuth2/OIDC
// client authentication. Source data lives under sessionManager.clientAuth in
// the top-level config and is read via config.FromContext.
package oauth2

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/client/rs"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	httphelper "github.com/zitadel/oidc/v3/pkg/http"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
	"github.com/openkcm/session-manager/internal/debugtools"
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

const cacheTTL = 30 * time.Minute

func init() {
	sessionmanager.RegisterModule(new(Module))
}

func newModule() sessionmanager.Module {
	return new(Module)
}

// Module is the credentials.module.oauth2 module. It implements credentials.Provider
// that builds credentials from sessionManager.clientAuth config field.
// OIDC discovery results are cached for cacheTTL; RS and RP are built from the
// cached config on every call.
type Module struct {
	Mod string `yaml:"module"`

	typ       string
	secret    string
	tlsConfig *tls.Config

	discoveryCache *ttlcache.Cache[string, *oidc.DiscoveryConfiguration]
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

	m.discoveryCache = ttlcache.New(
		ttlcache.WithTTL[string, *oidc.DiscoveryConfiguration](cacheTTL),
		ttlcache.WithDisableTouchOnHit[string, *oidc.DiscoveryConfiguration](),
	)
	go m.discoveryCache.Start()
	context.AfterFunc(ctx, m.discoveryCache.Stop)

	return nil
}

// ResourceServer implements [credentials.Provider].
// Fetches (or returns cached) discovery config then delegates to the appropriate
// credentials constructor, which skips re-running OIDC discovery.
func (m *Module) ResourceServer(ctx context.Context, clientID, issuer string) (rs.ResourceServer, error) {
	disc, err := m.GetDiscoveryConfig(ctx, issuer)
	if err != nil {
		return nil, err
	}

	opt := credentials.WithDiscoveryConfig(disc)
	switch m.typ {
	case authMTLS:
		return credentials.NewTLSRS(ctx, issuer, clientID, m.tlsConfig, opt)
	case authClientSecret, authClientSecretPost:
		return credentials.NewClientSecretPostRS(ctx, issuer, clientID, m.secret, opt)
	case authInsecure:
		return credentials.NewInsecureRS(ctx, issuer, clientID, opt)
	default:
		return nil, unknownAuthTypeError{typ: m.typ}
	}
}

// RelyingParty implements [credentials.Provider].
// Fetches (or returns cached) discovery config then delegates to the appropriate
// credentials constructor, which skips re-running OIDC discovery.
func (m *Module) RelyingParty(ctx context.Context, oidcCfg *oidcv1.OIDC, redirectURI string) (rp.RelyingParty, error) {
	disc, err := m.GetDiscoveryConfig(ctx, oidcCfg.GetIssuer())
	if err != nil {
		return nil, err
	}

	opt := credentials.WithDiscoveryConfig(disc)
	switch m.typ {
	case authMTLS:
		return credentials.NewTLSRP(ctx, oidcCfg.GetIssuer(), oidcCfg.GetClientId(), redirectURI, m.tlsConfig, opt)
	case authClientSecret, authClientSecretPost:
		return credentials.NewClientSecretPostRP(ctx, oidcCfg.GetIssuer(), oidcCfg.GetClientId(), m.secret, redirectURI, opt)
	case authInsecure:
		return credentials.NewInsecureRP(ctx, oidcCfg.GetIssuer(), oidcCfg.GetClientId(), redirectURI, opt)
	default:
		return nil, unknownAuthTypeError{typ: m.typ}
	}
}

// GetDiscoveryConfig implements [credentials.Provider].
func (m *Module) GetDiscoveryConfig(ctx context.Context, issuer string) (*oidc.DiscoveryConfiguration, error) {
	if m.discoveryCache == nil {
		return client.Discover(ctx, issuer, m.discoverHTTPClient())
	}
	key := cacheKey(issuer)
	if item := m.discoveryCache.Get(key); item != nil {
		return item.Value(), nil
	}
	conf, err := client.Discover(ctx, issuer, m.discoverHTTPClient())
	if err != nil {
		return nil, err
	}
	m.discoveryCache.Set(key, conf, ttlcache.DefaultTTL)
	return conf, nil
}

// discoverHTTPClient returns an HTTP client for OIDC discovery calls.
func (m *Module) discoverHTTPClient() *http.Client {
	if m.typ == authMTLS {
		return &http.Client{
			Transport: debugtools.DebugTransport(&http.Transport{
				TLSClientConfig: m.tlsConfig,
			}),
		}
	}
	c := *httphelper.DefaultHTTPClient
	c.Transport = debugtools.DebugTransport(c.Transport)
	return &c
}

// cacheKey returns a stable, opaque cache key for the given parts.
func cacheKey(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte("|"))
		}
		h.Write([]byte(p))
	}
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
