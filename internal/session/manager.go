package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/csrf"
	"github.com/patrickmn/go-cache"
	"github.com/zitadel/oidc/v3/pkg/client/rp"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	slogctx "github.com/veqryn/slog-context"
	zitadeloidc "github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
	"github.com/openkcm/session-manager/internal/debugtools"
	"github.com/openkcm/session-manager/internal/pkce"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/trust"
)

var debugSettingSMDumpTransport = debugtools.NewSetting("smdumptransport")

const (
	LoginCSRFCookieName = "LoginCSRF"
)

// AppIDTokenClaims extends IDTokenClaims with application-specific claims (user_uuid and groups).
type AppIDTokenClaims struct {
	zitadeloidc.IDTokenClaims

	UserUUID string   `json:"user_uuid,omitempty"`
	Groups   []string `json:"groups,omitempty"`
}

// UnmarshalJSON delegates to IDTokenClaims and extracts user_uuid and groups from the claims map.
func (c *AppIDTokenClaims) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &c.IDTokenClaims); err != nil {
		return err
	}
	if v, ok := c.Claims["user_uuid"]; ok {
		if s, ok := v.(string); ok {
			c.UserUUID = s
		}
	}
	if v, ok := c.Claims["groups"]; ok {
		if arr, ok := v.([]any); ok {
			groups := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					groups = append(groups, s)
				}
			}
			c.Groups = groups
		}
	}
	return nil
}

type Manager struct {
	trustRepo trust.OIDCMappingRepository
	sessions  Repository
	pkce      pkce.Source
	audit     *otlpaudit.AuditLogger
	newCreds  credentials.Builder

	sessionDuration       time.Duration
	idleSessionTimeout    time.Duration
	callbackURL           *url.URL
	clientID              string
	queryParametersAuth   []string
	queryParametersToken  []string
	authContextKeys       []string
	queryParametersLogout []string
	postLogoutRedirectURL string

	sessionCookieTemplate   config.CookieTemplate
	csrfCookieTemplate      config.CookieTemplate
	loginCSRFCookieTemplate config.CookieTemplate

	csrfSecret []byte

	cache *cache.Cache

	allowHttpScheme bool
}

func NewManager(
	cfg *config.SessionManager,
	trustRepo trust.OIDCMappingRepository,
	sessionsRepo Repository,
	auditLogger *otlpaudit.AuditLogger,
	opts ...ManagerOption,
) (*Manager, error) {
	callbackURL, err := url.Parse(cfg.CallbackURL)
	if err != nil {
		return nil, fmt.Errorf("parsing callback URL: %w", err)
	}

	m := &Manager{
		trustRepo:               trustRepo,
		sessions:                sessionsRepo,
		audit:                   auditLogger,
		sessionDuration:         cfg.SessionDuration,
		idleSessionTimeout:      cfg.IdleSessionTimeout,
		queryParametersAuth:     cfg.AdditionalQueryParametersAuthorize,
		queryParametersToken:    cfg.AdditionalQueryParametersToken,
		authContextKeys:         cfg.AdditionalAuthContextKeys,
		queryParametersLogout:   cfg.AdditionalQueryParametersLogout,
		postLogoutRedirectURL:   cfg.PostLogoutRedirectURL,
		sessionCookieTemplate:   cfg.SessionCookieTemplate,
		csrfCookieTemplate:      cfg.CSRFCookieTemplate,
		loginCSRFCookieTemplate: cfg.LoginCSRFCookieTemplate,
		callbackURL:             callbackURL,
		clientID:                cfg.ClientAuth.ClientID,
		newCreds:                func(clientID string) credentials.TransportCredentials { return credentials.NewInsecure(clientID) },
		csrfSecret:              cfg.CSRFSecretParsed,
		cache:                   cache.New(2*time.Minute, 10*time.Minute),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}

	return m, nil
}

// MakeAuthURI returns an OIDC authentication URI.
func (m *Manager) MakeAuthURI(ctx context.Context, tenantID, fingerprint, requestURI string) (string, string, error) {
	mapping, err := m.trustRepo.Get(ctx, tenantID)
	if err != nil {
		return "", "", fmt.Errorf("getting trust mapping: %w", err)
	}

	openidConf, err := m.getOpenIDConfig(ctx, mapping.IssuerURL, mapping)
	if err != nil {
		return "", "", fmt.Errorf("getting an openid config: %w", err)
	}

	stateID := m.pkce.State()
	pkce := m.pkce.PKCE()
	csrfToken := csrf.NewToken(stateID, m.csrfSecret)

	state := State{
		ID:             stateID,
		TenantID:       tenantID,
		Fingerprint:    fingerprint,
		PKCEVerifier:   pkce.Verifier,
		RequestURI:     requestURI,
		Expiry:         time.Now().Add(m.sessionDuration),
		LoginCSRFToken: csrfToken,
	}

	err = m.sessions.StoreState(ctx, state)
	if err != nil {
		return "", "", fmt.Errorf("storing session: %w", err)
	}

	u, err := m.authURI(openidConf, state, pkce, mapping)
	if err != nil {
		return "", "", fmt.Errorf("generating auth uri: %w", err)
	}

	return u, csrfToken, nil
}

func (m *Manager) LoadState(ctx context.Context, stateID string) (State, error) {
	return m.sessions.LoadState(ctx, stateID)
}

func (m *Manager) authURI(openidConf *zitadeloidc.DiscoveryConfiguration, state State, pkce pkce.PKCE, mapping trust.OIDCMapping) (string, error) {
	u, err := url.Parse(openidConf.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("parsing authorisation endpoint url: %w", err)
	}

	q := u.Query()
	q.Set("scope", "openid profile email groups")
	q.Set("response_type", "code")
	q.Set("client_id", m.getClientID(mapping))
	q.Set("state", state.ID)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", pkce.Method)
	q.Set("redirect_uri", m.callbackURL.String())
	for _, parameter := range m.queryParametersAuth {
		value, ok := mapping.Properties[parameter]
		if ok {
			q.Set(parameter, value)
		}
	}

	u.RawQuery = q.Encode()

	return u.String(), nil
}

// newRemoteKeySet creates a remote key set for the given JWKS URI.
func (m *Manager) newRemoteKeySet(oidcConf *zitadeloidc.DiscoveryConfiguration, mapping trust.OIDCMapping) zitadeloidc.KeySet {
	return rp.NewRemoteKeySet(m.httpClient(mapping), oidcConf.JwksURI)
}

func (m *Manager) FinaliseOIDCLogin(ctx context.Context, stateID, code, fingerprint string) (OIDCSessionData, error) {
	state, err := m.sessions.LoadState(ctx, stateID)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("loading state from the storage: %w", err)
	}

	correlationId := uuid.NewString()
	metadata, err := otlpaudit.NewEventMetadata("session manager", state.TenantID, correlationId)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("creating audit metadata: %w", err)
	}

	ctx = slogctx.With(ctx, "tenantId", state.TenantID)

	if time.Now().After(state.Expiry) {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "state expired")
		return OIDCSessionData{}, serviceerr.ErrStateExpired
	}

	if state.Fingerprint != fingerprint {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "fingerprint mismatch")
		return OIDCSessionData{}, serviceerr.ErrFingerprintMismatch
	}

	mapping, err := m.trustRepo.Get(ctx, state.TenantID)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get trust mapping")
		return OIDCSessionData{}, fmt.Errorf("getting trust mapping: %w", err)
	}

	openidConf, err := m.getOpenIDConfig(ctx, mapping.IssuerURL, mapping)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get openid configuration")
		return OIDCSessionData{}, fmt.Errorf("getting openid configuration: %w", err)
	}

	tokens, err := m.exchangeCode(ctx, openidConf, code, state.PKCEVerifier, mapping)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to exchange code for tokens")
		return OIDCSessionData{}, fmt.Errorf("exchanging code for tokens: %w", err)
	}

	slogctx.Info(ctx, "Exchanged the auth code for tokens")

	sessionID := m.pkce.SessionID()
	csrfToken := csrf.NewToken(sessionID, m.csrfSecret)

	keySet := m.newRemoteKeySet(openidConf, mapping)
	verifier := rp.NewIDTokenVerifier(
		mapping.IssuerURL,
		m.getClientID(mapping),
		keySet,
		rp.WithSupportedSigningAlgorithms(openidConf.IDTokenSigningAlgValuesSupported...),
	)

	idTokenClaims, err := rp.VerifyTokens[*AppIDTokenClaims](ctx, tokens.AccessToken, tokens.IDToken, verifier)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "id token verification failed")
		return OIDCSessionData{}, fmt.Errorf("verifying id token: %w", err)
	}

	authContext := map[string]string{
		"issuer":    mapping.IssuerURL,
		"client_id": m.getClientID(mapping),
	}
	for _, parameter := range m.authContextKeys {
		value, ok := mapping.Properties[parameter]
		if ok {
			authContext[parameter] = value
		}
	}

	session := Session{
		ID:          sessionID,
		TenantID:    state.TenantID,
		ProviderID:  idTokenClaims.SessionID,
		Fingerprint: fingerprint,
		CSRFToken:   csrfToken,
		Issuer:      mapping.IssuerURL,
		Claims: Claims{
			Subject:    idTokenClaims.GetSubject(),
			UserUUID:   idTokenClaims.UserUUID,
			GivenName:  idTokenClaims.GivenName,
			FamilyName: idTokenClaims.FamilyName,
			Email:      idTokenClaims.Email,
			Groups:     idTokenClaims.Groups,
		},
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Expiry:       time.Now().Add(m.sessionDuration),
		AuthContext:  authContext,
	}

	err = m.sessions.StoreSession(ctx, session)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to store session")
		return OIDCSessionData{}, fmt.Errorf("storing session: %w", err)
	}

	if err := m.sessions.BumpActive(ctx, session.ID, m.idleSessionTimeout); err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to bump the session active status")
		return OIDCSessionData{}, fmt.Errorf("bumping session active status: %w", err)
	}

	err = m.sessions.DeleteState(ctx, stateID)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to delete state")
		return OIDCSessionData{}, fmt.Errorf("deleting state: %w", err)
	}

	event, err := otlpaudit.NewUserLoginSuccessEvent(metadata, state.TenantID, otlpaudit.LOGINMETHOD_OPENIDCONNECT, otlpaudit.MFATYPE_NONE, otlpaudit.USERTYPE_BUSINESS, state.TenantID)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("creating audit log: %w", err)
	}
	otlpauditErr := m.audit.SendEvent(ctx, event)
	if otlpauditErr != nil {
		slogctx.Error(ctx, "Failed to send audit log for user login success", "error", otlpauditErr)
	}

	slogctx.Debug(ctx, "sent audit log for user login success")

	return OIDCSessionData{
		SessionID:  sessionID,
		TenantID:   state.TenantID,
		CSRFToken:  csrfToken,
		RequestURI: state.RequestURI,
	}, nil
}

func (m *Manager) Logout(ctx context.Context, sessionID string) (string, error) {
	session, err := m.sessions.LoadSession(ctx, sessionID)
	if err != nil {
		slogctx.Warn(ctx, "failed to get session by id", "error", err)
		return "", fmt.Errorf("getting session id: %w", err)
	}

	ctx = slogctx.With(ctx, "tenantId", session.TenantID)

	mapping, err := m.trustRepo.Get(ctx, session.TenantID)
	if err != nil {
		slogctx.Error(ctx, "failed to get trust mapping for a tenant", "error", err)
		return "", fmt.Errorf("getting trust mapping: %w", err)
	}

	ctx = slogctx.With(ctx, "issuerUrl", mapping.IssuerURL)

	oidcConf, err := m.getOpenIDConfig(ctx, mapping.IssuerURL, mapping)
	if err != nil {
		slogctx.Warn(ctx, "failed to get oidc configuration", "error", err)
		return "", fmt.Errorf("getting oidc configuration: %w", err)
	}

	if err := m.sessions.DeleteSession(ctx, session); err != nil {
		slogctx.Error(ctx, "failed to delete a session", "error", err)
		return "", fmt.Errorf("deleting session: %w", err)
	}

	if oidcConf.EndSessionEndpoint == "" {
		slogctx.Warn(ctx, "the provider does not support RP-Initiated Logout")

		if m.postLogoutRedirectURL != "" {
			return m.postLogoutRedirectURL, nil
		}

		return "", serviceerr.ErrEndSessionNotSupported
	}

	redirectURL, err := url.Parse(oidcConf.EndSessionEndpoint)
	if err != nil {
		slogctx.Warn(ctx, "failed to parse oidc session endpont", "error", err)
		return "", serviceerr.ErrInvalidOIDCProvider
	}

	vals := make(url.Values, 2)
	vals.Set("client_id", m.getClientID(mapping))
	if m.postLogoutRedirectURL != "" {
		vals.Set("post_logout_redirect_uri", m.postLogoutRedirectURL)
	}

	for _, parameter := range m.queryParametersLogout {
		value, ok := mapping.Properties[parameter]
		if ok {
			vals.Set(parameter, value)
		}
	}

	redirectURL.RawQuery = vals.Encode()

	return redirectURL.String(), nil
}

func (m *Manager) BCLogout(ctx context.Context, logoutJWT string) error {
	token, err := jwt.ParseSigned(logoutJWT, []jose.SignatureAlgorithm{
		jose.EdDSA,
		jose.HS256,
		jose.HS384,
		jose.HS512,
		jose.RS256,
		jose.RS384,
		jose.RS512,
		jose.ES256,
		jose.ES384,
		jose.ES512,
		jose.PS256,
		jose.PS384,
		jose.PS512,
	})
	if err != nil {
		return fmt.Errorf("parsing jwt: %w", err)
	}

	type logoutTokenClaims struct {
		jwt.Claims

		Events    map[string]json.RawMessage `json:"events,omitempty"`
		SessionID string                     `json:"sid,omitempty"`
	}

	var unsafeClaims logoutTokenClaims
	if err := token.UnsafeClaimsWithoutVerification(&unsafeClaims); err != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "failed to parse claims", "error", err)
		return fmt.Errorf("parsing claims unsafe: %w", err)
	}

	if _, ok := unsafeClaims.Events["http://schemas.openid.net/event/backchannel-logout"]; !ok {
		slogctx.FromCtx(ctx).WarnContext(ctx, "backchannel-logout: JWT token is not a logout token")
		return serviceerr.ErrInvalidRequest
	}

	if unsafeClaims.SessionID == "" {
		slogctx.FromCtx(ctx).WarnContext(ctx, "missing session id in the claims")
		return serviceerr.ErrInvalidRequest
	}

	session, err := m.sessions.LoadSessionByProviderID(ctx, unsafeClaims.SessionID)
	if err != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "backchannel-logout: session is not open")
		return nil
	}

	mapping, err := m.trustRepo.Get(ctx, session.TenantID)
	if err != nil {
		return fmt.Errorf("getting trust mapping: %w", err)
	}

	oidcConf, err := m.getOpenIDConfig(ctx, mapping.IssuerURL, mapping)
	if err != nil {
		return fmt.Errorf("getting oidc config: %w", err)
	}

	keyset := m.newRemoteKeySet(oidcConf, mapping)
	if _, err := rp.VerifyIDToken[*zitadeloidc.IDTokenClaims](ctx, logoutJWT, rp.NewIDTokenVerifier(
		mapping.IssuerURL,
		m.getClientID(mapping),
		keyset,
	)); err != nil {
		return fmt.Errorf("verifying logout token: %w", err)
	}

	if err := m.sessions.DeleteSession(ctx, session); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	return nil
}

func (m *Manager) MakeSessionCookie(ctx context.Context, tenantID, value string) (*http.Cookie, error) {
	sessionCookie := m.sessionCookieTemplate.ToCookie(value)
	if tenantID != "" {
		sessionCookie.Name = sessionCookie.Name + "-" + tenantID
	}

	err := sessionCookie.Valid()
	if err != nil {
		return nil, fmt.Errorf("invalid CSRF cookie: %w", err)
	}

	if !strings.HasPrefix(sessionCookie.Name, "__Host-Http-") {
		slogctx.Warn(ctx, "Session cookie name does not start with __Host-Http-; this is not recommended in production environments")
	}
	if !sessionCookie.Secure {
		slogctx.Warn(ctx, "Session cookie is not marked as Secure; this is not recommended in production environments")
	}
	if !sessionCookie.HttpOnly {
		slogctx.Warn(ctx, "Session cookie is not marked as HttpOnly; this is not recommended in production environments")
	}

	return sessionCookie, nil
}

func (m *Manager) MakeCSRFCookie(ctx context.Context, tenantID, value string) (*http.Cookie, error) {
	csrfCookie := m.csrfCookieTemplate.ToCookie(value)
	if tenantID != "" {
		csrfCookie.Name = csrfCookie.Name + "-" + tenantID
	}

	err := csrfCookie.Valid()
	if err != nil {
		return nil, fmt.Errorf("invalid CSRF cookie: %w", err)
	}

	checkCookie(ctx, csrfCookie)

	return csrfCookie, nil
}

func (m *Manager) MakeLoginCSRFCookie(ctx context.Context, value string) (*http.Cookie, error) {
	loginCSRFCookie := m.loginCSRFCookieTemplate.ToCookie(value)
	loginCSRFCookie.Name = LoginCSRFCookieName
	err := loginCSRFCookie.Valid()
	if err != nil {
		return nil, fmt.Errorf("invalid CSRF cookie: %w", err)
	}

	checkCookie(ctx, loginCSRFCookie)

	return loginCSRFCookie, nil
}

func checkCookie(ctx context.Context, csrfCookie *http.Cookie) {
	if !csrfCookie.Secure {
		slogctx.Warn(ctx, "CSRF cookie is not marked as Secure; this is not recommended in production environments")
	}
	if csrfCookie.HttpOnly {
		slogctx.Warn(ctx, "CSRF cookie is marked as HttpOnly; this is not recommended as the CSRF token needs to be accessible from JavaScript")
	}
	if csrfCookie.SameSite != http.SameSiteStrictMode {
		slogctx.Warn(ctx, "CSRF cookie is not marked as SameSite=Strict; this is not recommended in production environments")
	}
}

// sendUserLoginFailureAudit sends a user-login-failure audit event, logging any errors without propagating them.
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

	err = m.audit.SendEvent(ctx, event)
	if err != nil {
		slogctx.Error(ctx, "Failed to send audit log for user login failure", "error", err)
	}
	slogctx.Debug(ctx, "sent audit log for user login failure")
}

func (m *Manager) httpClient(mapping trust.OIDCMapping) *http.Client {
	creds := m.newCreds(m.getClientID(mapping))
	transport := creds.Transport()
	if debugSettingSMDumpTransport.Value() == "1" {
		transport = debugtools.NewTransport(transport)
	}

	return &http.Client{
		Transport: transport,
	}
}

func (m *Manager) getClientID(mapping trust.OIDCMapping) string {
	if mapping.ClientID != "" {
		return mapping.ClientID
	}

	return m.clientID
}

func (m *Manager) exchangeCode(ctx context.Context, openidConf *zitadeloidc.DiscoveryConfiguration, code, codeVerifier string, mapping trust.OIDCMapping) (tokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", m.callbackURL.String())
	for _, parameter := range m.queryParametersToken {
		value, ok := mapping.Properties[parameter]
		if ok {
			data.Set(parameter, value)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openidConf.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := m.httpClient(mapping)
	resp, err := client.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return tokenResponse{}, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var tokens tokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tokens)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("decoding response: %w", err)
	}

	return tokens, nil
}

func (m *Manager) ValidateCSRFToken(token, sessionID string) bool {
	return csrf.Validate(token, sessionID, m.csrfSecret)
}
