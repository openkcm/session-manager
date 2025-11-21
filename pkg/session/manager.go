package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/pkce"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/pkg/csrf"
)

type Manager struct {
	oidc         oidc.ProviderRepository
	sessions     Repository
	pkce         pkce.Source
	audit        *otlpaudit.AuditLogger
	secureClient *http.Client

	sessionDuration    time.Duration
	callbackURL        *url.URL
	clientID           string
	getParametersAuth  []string
	getParametersToken []string
	authContextKeys    []string

	csrfSecret []byte
	jwsSigAlgs []jose.SignatureAlgorithm
}

func NewManager(
	cfg *config.SessionManager,
	oidc oidc.ProviderRepository,
	sessions Repository,
	auditLogger *otlpaudit.AuditLogger,
	httpClient *http.Client,
) (*Manager, error) {
	algs := make([]jose.SignatureAlgorithm, 0, len(cfg.JWSSigAlgs))
	for _, alg := range cfg.JWSSigAlgs {
		algs = append(algs, jose.SignatureAlgorithm(alg))
	}

	csrfSecret, err := commoncfg.LoadValueFromSourceRef(cfg.CSRFSecret)
	if err != nil {
		return nil, fmt.Errorf("loading csrf token from source ref: %w", err)
	}
	if len(csrfSecret) < 32 {
		return nil, errors.New("CSRF secret must be at least 32 bytes")
	}

	callbackURL, err := url.Parse(cfg.CallbackURL)
	if err != nil {
		return nil, fmt.Errorf("parsing callback URL: %w", err)
	}

	return &Manager{
		oidc:               oidc,
		sessions:           sessions,
		audit:              auditLogger,
		sessionDuration:    cfg.SessionDuration,
		getParametersAuth:  cfg.AdditionalGetParametersAuthorize,
		getParametersToken: cfg.AdditionalGetParametersToken,
		authContextKeys:    cfg.AdditionalAuthContextKeys,
		callbackURL:        callbackURL,
		clientID:           cfg.ClientAuth.ClientID,
		secureClient:       httpClient,
		csrfSecret:         csrfSecret,
		jwsSigAlgs:         algs,
	}, nil
}

// MakeAuthURI returns an OIDC authentication URI.
func (m *Manager) MakeAuthURI(ctx context.Context, tenantID, fingerprint, requestURI string) (string, error) {
	provider, err := m.oidc.GetForTenant(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("getting oidc provider: %w", err)
	}

	openidConf, err := m.getOpenIDConfig(ctx, provider)
	if err != nil {
		return "", fmt.Errorf("getting an openid config: %w", err)
	}

	stateID := m.pkce.State()
	pkce := m.pkce.PKCE()

	state := State{
		ID:           stateID,
		TenantID:     tenantID,
		Fingerprint:  fingerprint,
		PKCEVerifier: pkce.Verifier,
		RequestURI:   requestURI,
		Expiry:       time.Now().Add(m.sessionDuration),
	}

	if err := m.sessions.StoreState(ctx, state); err != nil {
		return "", fmt.Errorf("storing session: %w", err)
	}

	u, err := m.authURI(openidConf, state, pkce, provider.Properties)
	if err != nil {
		return "", fmt.Errorf("generating auth uri: %w", err)
	}

	return u, nil
}

func (m *Manager) authURI(openidConf oidc.Configuration, state State, pkce pkce.PKCE, properties map[string]string) (string, error) {
	u, err := url.Parse(openidConf.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("parsing authorisation endpoint url: %w", err)
	}

	q := u.Query()
	q.Set("scope", "openid profile email groups")
	q.Set("response_type", "code")
	q.Set("client_id", m.clientID)
	q.Set("state", state.ID)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", pkce.Method)
	q.Set("redirect_uri", m.callbackURL.String())
	for _, parameter := range m.getParametersAuth {
		value, ok := properties[parameter]
		if !ok {
			return "", fmt.Errorf("missing auth parameter: %s", parameter)
		}
		q.Set(parameter, value)
	}

	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (m *Manager) getProviderKeySet(ctx context.Context, oidcConf oidc.Configuration) (*jose.JSONWebKeySet, error) {
	var keySet jose.JSONWebKeySet
	uri := oidcConf.JwksURI
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, fmt.Errorf("creating a new HTTP request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing an http request: %w", err)
	}

	if err := json.NewDecoder(resp.Body).Decode(&keySet); err != nil {
		return nil, fmt.Errorf("decoding keyset response: %w", err)
	}

	return &keySet, nil
}

func (m *Manager) FinaliseOIDCLogin(ctx context.Context, stateID, code, fingerprint string) (OIDCSessionData, error) {
	state, err := m.sessions.LoadState(ctx, stateID)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("loading state from the storage: %w", err)
	}

	// audit log metadata
	correlationId := uuid.NewString()
	metadata, err := otlpaudit.NewEventMetadata("session manager", state.TenantID, correlationId)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("creating audit metadata: %w", err)
	}

	ctx = slogctx.With(ctx, "tenant_id", state.TenantID)

	if time.Now().After(state.Expiry) {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "state expired")
		return OIDCSessionData{}, serviceerr.ErrStateExpired
	}

	if state.Fingerprint != fingerprint {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "fingerprint mismatch")
		return OIDCSessionData{}, serviceerr.ErrFingerprintMismatch
	}

	provider, err := m.oidc.GetForTenant(ctx, state.TenantID)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get oidc provider")
		return OIDCSessionData{}, fmt.Errorf("getting oidc provider: %w", err)
	}

	openidConf, err := m.getOpenIDConfig(ctx, provider)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get openid configuration")
		return OIDCSessionData{}, fmt.Errorf("getting openid configuration: %w", err)
	}

	tokens, err := m.exchangeCode(ctx, openidConf, code, state.PKCEVerifier, provider.Properties)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to exchange code for tokens")
		return OIDCSessionData{}, fmt.Errorf("exchanging code for tokens: %w", err)
	}

	slogctx.Info(ctx, "Exchanged the auth code for tokens")

	sessionID := m.pkce.SessionID()
	csrfToken := csrf.NewToken(sessionID, m.csrfSecret)

	token, err := jwt.ParseSigned(tokens.IDToken, m.jwsSigAlgs)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to parse id token")
		return OIDCSessionData{}, fmt.Errorf("parsing id token: %w", err)
	}

	jws, err := jose.ParseSigned(tokens.IDToken, m.jwsSigAlgs)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("parsing JWS: %w", err)
	}

	keyset, err := m.getProviderKeySet(ctx, openidConf)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get jwks for provider")
		return OIDCSessionData{}, fmt.Errorf("getting jwks for a provider: %w", err)
	}

	var claims jwt.Claims
	if err := token.Claims(keyset, &claims); err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get JWT claims")
		return OIDCSessionData{}, fmt.Errorf("getting JWT claims: %w", err)
	}

	// prepare the auth context used by ExtAuthZ
	authContext := map[string]string{
		"issuer":    provider.IssuerURL,
		"client_id": m.clientID,
	}
	for _, parameter := range m.authContextKeys {
		value, ok := provider.Properties[parameter]
		if !ok {
			return OIDCSessionData{}, fmt.Errorf("missing auth context parameter: %s", parameter)
		}
		authContext[parameter] = value
	}

	session := Session{
		ID:          sessionID,
		TenantID:    state.TenantID,
		Fingerprint: fingerprint,
		CSRFToken:   csrfToken,
		Issuer:      provider.IssuerURL,
		RawClaims:   string(jws.UnsafePayloadWithoutVerification()),
		Claims: Claims{
			Subject: claims.Subject,
			Email:   "",         // TODO: extract email from claims
			Groups:  []string{}, // TODO: extract groups from claims
		},
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Expiry:       time.Now().Add(m.sessionDuration),
		AuthContext:  authContext,
	}

	if err := m.sessions.StoreSession(ctx, session); err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to store session")
		return OIDCSessionData{}, fmt.Errorf("storing session: %w", err)
	}

	if err := m.sessions.DeleteState(ctx, stateID); err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to delete state")
		return OIDCSessionData{}, fmt.Errorf("deleting state: %w", err)
	}

	// audit userLoginSuccess
	event, err := otlpaudit.NewUserLoginSuccessEvent(metadata, state.TenantID, otlpaudit.LOGINMETHOD_OPENIDCONNECT, otlpaudit.MFATYPE_NONE, otlpaudit.USERTYPE_BUSINESS, state.TenantID)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("creating audit log: %w", err)
	}
	otlpauditErr := m.audit.SendEvent(ctx, event)
	if otlpauditErr != nil {
		slogctx.Error(ctx, "Failed to send audit log for user login success", "error", otlpauditErr)
	}

	return OIDCSessionData{
		SessionID:  sessionID,
		CSRFToken:  csrfToken,
		RequestURI: state.RequestURI,
	}, nil
}

func (m *Manager) MakeCSRFCookieDomain() (string, error) {
	host := m.callbackURL.Hostname()
	// strip the first subdomain and return the rest with a leading . as cookie domain
	if _, cookieDomain, found := strings.Cut(host, "."); found {
		return "." + cookieDomain, nil
	}
	return "", fmt.Errorf("could not determine cookie domain from host: %s", host)
}

// sendUserLoginFailureAudit creates the user-login-failure audit event and sends it.
// The function logs any errors encountered while creating or sending the event but
// does not propagate them to the caller.
func (m *Manager) sendUserLoginFailureAudit(ctx context.Context, metadata otlpaudit.EventMetadata, objectID, reason string) {
	if m.audit == nil {
		slogctx.Warn(ctx, "audit logger is nil; skipping user login failure event")
		return
	}

	event, err := otlpaudit.NewUserLoginFailureEvent(metadata, objectID, otlpaudit.LOGINMETHOD_OPENIDCONNECT, otlpaudit.FailReason(reason), objectID)
	if err != nil {
		slogctx.Error(ctx, "creating audit log", "error", err)
		return
	}

	if err := m.audit.SendEvent(ctx, event); err != nil {
		slogctx.Error(ctx, "Failed to send audit log for user login failure", "error", err)
	}
}

func (m *Manager) exchangeCode(ctx context.Context, openidConf oidc.Configuration, code, codeVerifier string, properties map[string]string) (tokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", m.callbackURL.String())
	data.Set("client_id", m.clientID)
	for _, parameter := range m.getParametersToken {
		value, ok := properties[parameter]
		if !ok {
			return tokenResponse{}, fmt.Errorf("missing token parameter: %s", parameter)
		}
		data.Set(parameter, value)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openidConf.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.secureClient.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return tokenResponse{}, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var tokens tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return tokenResponse{}, fmt.Errorf("decoding response: %w", err)
	}

	return tokens, nil
}

func (m *Manager) ValidateCSRFToken(token, sessionID string) bool {
	return csrf.Validate(token, sessionID, m.csrfSecret)
}

func (m *Manager) RefreshExpiringSessions(ctx context.Context) error {
	sessions, err := m.sessions.ListSessions(ctx)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		provider, err := m.oidc.Get(ctx, s.Issuer)
		if err != nil {
			return fmt.Errorf("getting odic provider: %w", err)
		}

		if shouldRefresh(s) {
			if err := m.RefreshSession(ctx, &s, provider); err != nil {
				slogctx.Warn(ctx, "Could not refresh token", "tenant_id", s.TenantID)
				continue
			}

			if err := m.sessions.StoreSession(ctx, s); err != nil {
				slogctx.Warn(ctx, "Could not store refreshed session", "tenant_id", s.TenantID)
				continue
			}
		}
	}
	return nil
}

// RefreshSession refreshes the access token using the given refresh token for the tenant.
func (m *Manager) RefreshSession(ctx context.Context, s *Session, provider oidc.Provider) error {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", s.RefreshToken)
	data.Set("client_id", m.clientID)

	tokenEndpoint, err := url.JoinPath(provider.IssuerURL, "/token")
	if err != nil {
		return fmt.Errorf("making issuer token path: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.secureClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("token endpoint returned non-200 status")
	}

	var respData struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	s.AccessToken = respData.AccessToken
	s.RefreshToken = respData.RefreshToken
	s.AccessTokenExpiry = time.Now().Add(time.Duration(respData.ExpiresIn))

	return nil
}

func (m *Manager) getOpenIDConfig(ctx context.Context, provider oidc.Provider) (oidc.Configuration, error) {
	const wellKnownOpenIDConfigPath = "/.well-known/openid-configuration"

	u, err := url.JoinPath(provider.IssuerURL, wellKnownOpenIDConfigPath)
	if err != nil {
		return oidc.Configuration{}, fmt.Errorf("building path to the well-known openid-config endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return oidc.Configuration{}, fmt.Errorf("creating an HTTP request: %w", err)
	}

	resp, err := m.secureClient.Do(req)
	if err != nil {
		return oidc.Configuration{}, fmt.Errorf("doing an HTTP request: %w", err)
	}

	var conf oidc.Configuration
	if err := json.NewDecoder(resp.Body).Decode(&conf); err != nil {
		return oidc.Configuration{}, fmt.Errorf("decoding a well-known openid config: %w", err)
	}

	// Validate the configuration
	if conf.Issuer != provider.IssuerURL {
		return oidc.Configuration{}, serviceerr.ErrInvalidOIDCProvider
	}

	return conf, nil
}

func shouldRefresh(s Session) bool {
	// refresh if token expires in less than 5 minutes
	return time.Until(s.AccessTokenExpiry) < 5*time.Minute
}
