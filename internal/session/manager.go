package session

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/csrf"
	"github.com/openkcm/common-sdk/pkg/oidc"
	"github.com/patrickmn/go-cache"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/pkce"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/trust"
)

type Manager struct {
	trustRepo    trust.OIDCMappingRepository
	sessions     Repository
	pkce         pkce.Source
	audit        *otlpaudit.AuditLogger
	secureClient *http.Client

	sessionDuration       time.Duration
	idleSessionTimeout    time.Duration
	callbackURL           *url.URL
	clientID              string
	queryParametersAuth   []string
	queryParametersToken  []string
	authContextKeys       []string
	queryParametersLogout []string
	postLogoutRedirectURL string

	sessionCookieTemplate config.CookieTemplate
	csrfCookieTemplate    config.CookieTemplate

	csrfSecret []byte

	cache *cache.Cache

	allowHttpScheme bool
}

func NewManager(
	cfg *config.SessionManager,
	oidc trust.OIDCMappingRepository,
	sessions Repository,
	auditLogger *otlpaudit.AuditLogger,
	httpClient *http.Client,
) (*Manager, error) {
	callbackURL, err := url.Parse(cfg.CallbackURL)
	if err != nil {
		return nil, fmt.Errorf("parsing callback URL: %w", err)
	}

	return &Manager{
		trustRepo:             oidc,
		sessions:              sessions,
		audit:                 auditLogger,
		sessionDuration:       cfg.SessionDuration,
		idleSessionTimeout:    cfg.IdleSessionTimeout,
		queryParametersAuth:   cfg.AdditionalQueryParametersAuthorize,
		queryParametersToken:  cfg.AdditionalQueryParametersToken,
		authContextKeys:       cfg.AdditionalAuthContextKeys,
		queryParametersLogout: cfg.AdditionalQueryParametersLogout,
		postLogoutRedirectURL: cfg.PostLogoutRedirectURL,
		sessionCookieTemplate: cfg.SessionCookieTemplate,
		csrfCookieTemplate:    cfg.CSRFCookieTemplate,
		callbackURL:           callbackURL,
		clientID:              cfg.ClientAuth.ClientID,
		secureClient:          httpClient,
		csrfSecret:            cfg.CSRFSecretParsed,
		cache:                 cache.New(2*time.Minute, 10*time.Minute),
	}, nil
}

// MakeAuthURI returns an OIDC authentication URI.
func (m *Manager) MakeAuthURI(ctx context.Context, tenantID, fingerprint, requestURI string) (string, error) {
	mapping, err := m.trustRepo.Get(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("getting trust mapping: %w", err)
	}

	openidConf, err := m.getOpenIDConfig(ctx, mapping.IssuerURL)
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

	err = m.sessions.StoreState(ctx, state)
	if err != nil {
		return "", fmt.Errorf("storing session: %w", err)
	}

	u, err := m.authURI(openidConf, state, pkce, mapping.Properties)
	if err != nil {
		return "", fmt.Errorf("generating auth uri: %w", err)
	}

	return u, nil
}

func (m *Manager) authURI(openidConf *oidc.Configuration, state State, pkce pkce.PKCE, properties map[string]string) (string, error) {
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
	for _, parameter := range m.queryParametersAuth {
		value, ok := properties[parameter]
		if !ok {
			return "", fmt.Errorf("missing auth parameter: %s", parameter)
		}
		q.Set(parameter, value)
	}

	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (m *Manager) getProviderKeySet(ctx context.Context, oidcConf *oidc.Configuration) (*jose.JSONWebKeySet, error) {
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

	err = json.NewDecoder(resp.Body).Decode(&keySet)
	if err != nil {
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

	mapping, err := m.trustRepo.Get(ctx, state.TenantID)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get trust mapping")
		return OIDCSessionData{}, fmt.Errorf("getting trust mapping: %w", err)
	}

	openidConf, err := m.getOpenIDConfig(ctx, mapping.IssuerURL)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get openid configuration")
		return OIDCSessionData{}, fmt.Errorf("getting openid configuration: %w", err)
	}

	tokens, err := m.exchangeCode(ctx, openidConf, code, state.PKCEVerifier, mapping.Properties)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to exchange code for tokens")
		return OIDCSessionData{}, fmt.Errorf("exchanging code for tokens: %w", err)
	}

	slogctx.Info(ctx, "Exchanged the auth code for tokens")

	sessionID := m.pkce.SessionID()
	csrfToken := csrf.NewToken(sessionID, m.csrfSecret)
	algs := make([]jose.SignatureAlgorithm, 0, len(openidConf.IDTokenSigningAlgValuesSupported))
	for _, alg := range openidConf.IDTokenSigningAlgValuesSupported {
		algs = append(algs, jose.SignatureAlgorithm(alg))
	}
	token, err := jwt.ParseSigned(tokens.IDToken, algs)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to parse id token")
		return OIDCSessionData{}, fmt.Errorf("parsing id token: %w, %s", err, algs)
	}

	keyset, err := m.getProviderKeySet(ctx, openidConf)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get jwks for provider")
		return OIDCSessionData{}, fmt.Errorf("getting jwks for a provider: %w", err)
	}

	type CustomClaims struct {
		SID        string   `json:"sid"`
		UserUUID   string   `json:"user_uuid"`
		GivenName  string   `json:"given_name"`
		FamilyName string   `json:"family_name"`
		Email      string   `json:"email"`
		Groups     []string `json:"groups"`
	}

	type ExtraClaims struct {
		AtHash string `json:"at_hash,omitempty"`
	}

	var standardClaims jwt.Claims
	var customClaims CustomClaims
	var extraClaims ExtraClaims
	err = token.Claims(keyset, &standardClaims, &customClaims, &extraClaims)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get JWT claims")
		return OIDCSessionData{}, fmt.Errorf("getting JWT claims: %w", err)
	}

	if extraClaims.AtHash != "" {
		err := m.verifyAccessToken(tokens.AccessToken, extraClaims.AtHash, token)
		if err != nil {
			return OIDCSessionData{}, err
		}
	}

	// prepare the auth context used by ExtAuthZ
	authContext := map[string]string{
		"issuer":    mapping.IssuerURL,
		"client_id": m.clientID,
	}
	for _, parameter := range m.authContextKeys {
		value, ok := mapping.Properties[parameter]
		if !ok {
			return OIDCSessionData{}, fmt.Errorf("missing auth context parameter: %s", parameter)
		}
		authContext[parameter] = value
	}

	session := Session{
		ID:          sessionID,
		TenantID:    state.TenantID,
		ProviderID:  customClaims.SID,
		Fingerprint: fingerprint,
		CSRFToken:   csrfToken,
		Issuer:      mapping.IssuerURL,
		Claims: Claims{
			Subject:    standardClaims.Subject,
			UserUUID:   customClaims.UserUUID,
			GivenName:  customClaims.GivenName,
			FamilyName: customClaims.FamilyName,
			Email:      customClaims.Email,
			Groups:     customClaims.Groups,
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

	// audit userLoginSuccess
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

	ctx = slogctx.With(ctx, "tenant_id", session.TenantID)

	mapping, err := m.trustRepo.Get(ctx, session.TenantID)
	if err != nil {
		slogctx.Error(ctx, "failed to get trust mapping for a tenant", "error", err)
		return "", fmt.Errorf("getting trust mapping: %w", err)
	}

	ctx = slogctx.With(ctx, "issuer_url", mapping.IssuerURL)

	oidcConf, err := m.getOpenIDConfig(ctx, mapping.IssuerURL)
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

		// Redirect to the landing page if possible
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

	vals := make(url.Values)
	vals.Set("client_id", m.clientID)
	if m.postLogoutRedirectURL != "" {
		vals.Set("post_logout_redirect_uri", m.postLogoutRedirectURL)
	}

	for _, p := range m.queryParametersLogout {
		v, ok := mapping.Properties[p]
		if !ok {
			return "", fmt.Errorf("missing auth parameter: %s", p)
		}

		vals.Set(p, v)
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

	// Logout token must contain either a sub or a sid Claim, and may contain both.
	type logoutTokenClaims struct {
		jwt.Claims

		// Events is always "http://schemas.openid.net/event/backchannel-logout": {}
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

	oidcConf, err := m.getOpenIDConfig(ctx, mapping.IssuerURL)
	if err != nil {
		return fmt.Errorf("getting oidc config: %w", err)
	}

	keyset, err := m.getProviderKeySet(ctx, oidcConf)
	if err != nil {
		return fmt.Errorf("getting jwks for a provider: %w", err)
	}

	var claims logoutTokenClaims
	if err := token.Claims(keyset, &claims); err != nil {
		return fmt.Errorf("parsing claims: %w", err)
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

	if !csrfCookie.Secure {
		slogctx.Warn(ctx, "CSRF cookie is not marked as Secure; this is not recommended in production environments")
	}
	if csrfCookie.HttpOnly {
		slogctx.Warn(ctx, "CSRF cookie is marked as HttpOnly; this is not recommended as the CSRF token needs to be accessible from JavaScript")
	}
	if csrfCookie.SameSite != http.SameSiteStrictMode {
		slogctx.Warn(ctx, "CSRF cookie is not marked as SameSite=Strict; this is not recommended in production environments")
	}

	return csrfCookie, nil
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

	err = m.audit.SendEvent(ctx, event)
	if err != nil {
		slogctx.Error(ctx, "Failed to send audit log for user login failure", "error", err)
	}
	slogctx.Debug(ctx, "sent audit log for user login failure")
}

func (m *Manager) verifyAccessToken(accessToken, atHash string, idToken *jwt.JSONWebToken) error {
	var h hash.Hash
	switch alg := idToken.Headers[0].Algorithm; alg {
	case "RS256", "ES256", "PS256":
		h = sha256.New()
	case "RS384", "ES384", "PS384":
		h = sha512.New384()
	case "RS512", "ES512", "PS512", "EdDSA":
		h = sha512.New()
	default:
		return fmt.Errorf("oidc: unsupported signing algorithm %q", alg)
	}

	h.Write([]byte(accessToken)) // NOSONAR
	sum := h.Sum(nil)[:h.Size()/2]
	actual := base64.RawURLEncoding.EncodeToString(sum)
	if actual != atHash {
		return serviceerr.ErrInvalidAtHash
	}

	return nil
}

func (m *Manager) exchangeCode(ctx context.Context, openidConf *oidc.Configuration, code, codeVerifier string, properties map[string]string) (tokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", m.callbackURL.String())
	data.Set("client_id", m.clientID)
	for _, parameter := range m.queryParametersToken {
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
	err = json.NewDecoder(resp.Body).Decode(&tokens)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("decoding response: %w", err)
	}

	return tokens, nil
}

func (m *Manager) ValidateCSRFToken(token, sessionID string) bool {
	return csrf.Validate(token, sessionID, m.csrfSecret)
}
