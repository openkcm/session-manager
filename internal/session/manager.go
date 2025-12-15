package session

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/csrf"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/pkce"
	"github.com/openkcm/session-manager/internal/serviceerr"
)

type Manager struct {
	oidc         oidc.ProviderRepository
	sessions     Repository
	pkce         pkce.Source
	audit        *otlpaudit.AuditLogger
	secureClient *http.Client

	sessionDuration      time.Duration
	callbackURL          *url.URL
	clientID             string
	queryParametersAuth  []string
	queryParametersToken []string
	authContextKeys      []string

	sessionCookieTemplate config.CookieTemplate
	csrfCookieTemplate    config.CookieTemplate

	csrfSecret []byte
}

func NewManager(
	cfg *config.SessionManager,
	oidc oidc.ProviderRepository,
	sessions Repository,
	auditLogger *otlpaudit.AuditLogger,
	httpClient *http.Client,
) (*Manager, error) {
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
		oidc:                  oidc,
		sessions:              sessions,
		audit:                 auditLogger,
		sessionDuration:       cfg.SessionDuration,
		queryParametersAuth:   cfg.AdditionalQueryParametersAuthorize,
		queryParametersToken:  cfg.AdditionalQueryParametersToken,
		authContextKeys:       cfg.AdditionalAuthContextKeys,
		sessionCookieTemplate: cfg.SessionCookieTemplate,
		csrfCookieTemplate:    cfg.CSRFCookieTemplate,
		callbackURL:           callbackURL,
		clientID:              cfg.ClientAuth.ClientID,
		secureClient:          httpClient,
		csrfSecret:            csrfSecret,
	}, nil
}

// MakeAuthURI returns an OIDC authentication URI.
func (m *Manager) MakeAuthURI(ctx context.Context, tenantID, fingerprint, requestURI string) (string, error) {
	provider, err := m.oidc.Get(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("getting oidc provider: %w", err)
	}

	openidConf, err := provider.GetOpenIDConfig(ctx, http.DefaultClient)
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

	provider, err := m.oidc.Get(ctx, state.TenantID)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get oidc provider")
		return OIDCSessionData{}, fmt.Errorf("getting oidc provider: %w", err)
	}

	openidConf, err := provider.GetOpenIDConfig(ctx, http.DefaultClient)
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
	if err := token.Claims(keyset, &standardClaims, &customClaims, &extraClaims); err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "failed to get JWT claims")
		return OIDCSessionData{}, fmt.Errorf("getting JWT claims: %w", err)
	}

	if extraClaims.AtHash != "" {
		if err := m.verifyAccessToken(tokens.AccessToken, extraClaims.AtHash, token); err != nil {
			return OIDCSessionData{}, err
		}
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
		ProviderID:  customClaims.SID,
		Fingerprint: fingerprint,
		CSRFToken:   csrfToken,
		Issuer:      provider.IssuerURL,
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
		LastVisited:  time.Now(),
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

	slogctx.Debug(ctx, "sent audit log for user login success")

	return OIDCSessionData{
		SessionID:  sessionID,
		CSRFToken:  csrfToken,
		RequestURI: state.RequestURI,
	}, nil
}

func (m *Manager) MakeSessionCookie(ctx context.Context, value string) (*http.Cookie, error) {
	sessionCookie := m.sessionCookieTemplate.ToCookie(value)

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

func (m *Manager) MakeCSRFCookie(ctx context.Context, value string) (*http.Cookie, error) {
	csrfCookie := m.csrfCookieTemplate.ToCookie(value)

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

	if err := m.audit.SendEvent(ctx, event); err != nil {
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

func (m *Manager) exchangeCode(ctx context.Context, openidConf oidc.Configuration, code, codeVerifier string, properties map[string]string) (tokenResponse, error) {
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
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return tokenResponse{}, fmt.Errorf("decoding response: %w", err)
	}

	return tokens, nil
}

func (m *Manager) ValidateCSRFToken(token, sessionID string) bool {
	return csrf.Validate(token, sessionID, m.csrfSecret)
}
