package session

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"uuid"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/openkcm/common-sdk/pkg/csrf"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"

	flowv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/flow/v1"
	oidcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/trust/oidc/v1"
	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	slogctx "github.com/veqryn/slog-context"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/internal/credentials"
	"github.com/openkcm/session-manager/internal/pkce"
	"github.com/openkcm/session-manager/internal/validation"
	"github.com/openkcm/session-manager/pkg/serviceerr"
)

const (
	LoginCSRFCookieName = "__Host-LoginCSRF"

	// meterName is the instrumentation scope name for OpenTelemetry metrics.
	// Per the OTEL spec, the import path of the instrumentation package is used.
	meterName = "github.com/openkcm/session-manager/internal/session"
)

type Manager struct {
	trust                 sessionmanager.Trust
	sessions              Repository
	pkce                  pkce.Source
	audit                 *otlpaudit.AuditLogger
	cProvider             credentials.Provider
	globalUserIDClaimKeys []string

	sessionDuration    time.Duration
	idleSessionTimeout time.Duration
	callbackURL        *url.URL

	sessionCookieTemplate   config.CookieTemplate
	csrfCookieTemplate      config.CookieTemplate
	loginCSRFCookieTemplate config.CookieTemplate

	csrfSecret []byte

	allowHttpScheme         bool
	allowedRedirectBaseURLs []*url.URL

	userLastSeenGauge metric.Int64Gauge
}

func NewManager(
	ctx context.Context,
	cfg *config.SessionManager,
	trust sessionmanager.Trust,
	sessionsRepo Repository,
	auditLogger *otlpaudit.AuditLogger,
	opts ...ManagerOption,
) (*Manager, error) {
	callbackURL, err := url.Parse(cfg.CallbackURL)
	if err != nil {
		return nil, fmt.Errorf("parsing callback URL: %w", err)
	}

	m := &Manager{
		trust:                   trust,
		sessions:                sessionsRepo,
		audit:                   auditLogger,
		sessionDuration:         cfg.SessionDuration,
		idleSessionTimeout:      cfg.IdleSessionTimeout,
		sessionCookieTemplate:   cfg.SessionCookieTemplate,
		csrfCookieTemplate:      cfg.CSRFCookieTemplate,
		loginCSRFCookieTemplate: cfg.LoginCSRFCookieTemplate,
		callbackURL:             callbackURL,
		cProvider:               credentials.InsecureProvider{},
		csrfSecret:              cfg.CSRFSecretParsed,
		allowedRedirectBaseURLs: parseURLs(cfg.AllowedRedirectBaseURLs),
		globalUserIDClaimKeys:   []string{"user_uuid", "sub"},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}

	var gaugeErr error
	m.userLastSeenGauge, gaugeErr = otel.Meter(meterName).Int64Gauge(
		"user.last_seen",
		metric.WithDescription("Unix timestamp of the most recent successful login per user."),
		metric.WithUnit("s"),
	)
	if gaugeErr != nil {
		return nil, fmt.Errorf("creating user.last_seen gauge: %w", gaugeErr)
	}

	return m, nil
}

// MakeAuthURI returns an OIDC authentication URI.
func (m *Manager) MakeAuthURI(ctx context.Context, tenantID, requestURI, errorURI string) (string, string, error) {
	trust, err := m.trust.Get(ctx, tenantID)
	if err != nil {
		return "", "", fmt.Errorf("getting trust: %w", err)
	}

	oidc := trust.GetOidc()

	stateID := m.pkce.State()
	pkce := m.pkce.PKCE()
	csrfToken := csrf.NewToken(stateID, m.csrfSecret)

	if !m.IsValidRequestURI(requestURI) {
		return "", "", serviceerr.ErrInvalidRequest
	}

	state := State{
		ID:             stateID,
		TenantID:       tenantID,
		PKCEVerifier:   pkce.Verifier,
		RequestURI:     requestURI,
		ErrorURI:       errorURI,
		Expiry:         time.Now().Add(m.sessionDuration),
		LoginCSRFToken: csrfToken,
	}

	err = m.sessions.StoreState(ctx, state)
	if err != nil {
		return "", "", fmt.Errorf("storing session: %w", err)
	}

	u, err := m.authURI(ctx, state, pkce, oidc)
	if err != nil {
		return "", "", fmt.Errorf("generating auth uri: %w", err)
	}

	return u, csrfToken, nil
}

func (m *Manager) LoadState(ctx context.Context, stateID string) (State, error) {
	return m.sessions.LoadState(ctx, stateID)
}

func (m *Manager) authURI(ctx context.Context, state State, pkce pkce.PKCE, oidcTrust *oidcv1.OIDC) (string, error) {
	relyingParty, err := m.relyingParty(ctx, oidcTrust)
	if err != nil {
		return "", fmt.Errorf("creating relying party: %w", err)
	}

	opts := []rp.AuthURLOpt{
		rp.WithCodeChallenge(pkce.Challenge),
		rp.AuthURLOpt(rp.WithURLParam("scope", "openid profile email groups")),
	}

	//nolint:forcetypeassert
	for _, param := range proto.GetExtension(oidcTrust, flowv1.E_AuthAttributes).([]*flowv1.Attribute) {
		opts = append(opts, rp.AuthURLOpt(rp.WithURLParam(param.GetKey(), param.GetValue())))
	}

	return rp.AuthURL(state.ID, relyingParty, opts...), nil
}

func (m *Manager) getProviderKeySet(ctx context.Context, oidcConf *oidc.DiscoveryConfiguration) (*jose.JSONWebKeySet, error) {
	jwksURI := oidcConf.JwksURI
	if err := validation.SecureScheme(jwksURI, m.allowHttpScheme); err != nil {
		return nil, err
	}

	var keySet jose.JSONWebKeySet
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("creating a new HTTP request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing an http request: %w", err)
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&keySet)
	if err != nil {
		return nil, fmt.Errorf("decoding keyset response: %w", err)
	}

	return &keySet, nil
}

func (m *Manager) FinaliseOIDCLogin(ctx context.Context, stateID, code string) (OIDCSessionData, error) {
	state, err := m.sessions.LoadState(ctx, stateID)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("loading state from the storage: %w", err)
	}

	// audit log metadata
	correlationId := uuid.New().String()
	metadata, err := otlpaudit.NewEventMetadata("unknown", state.TenantID, correlationId)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("creating audit metadata: %w", err)
	}

	ctx = slogctx.With(ctx, "tenantId", state.TenantID)

	if time.Now().After(state.Expiry) {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "login request timed out", otlpaudit.FAILREASON_SESSIONEXPIRED)
		return OIDCSessionData{}, serviceerr.ErrStateExpired
	}

	trust, err := m.trust.Get(ctx, state.TenantID)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "authentication configuration error", otlpaudit.FAILREASON_TOKENINVALID)
		return OIDCSessionData{}, fmt.Errorf("getting trust: %w", err)
	}

	oidc := trust.GetOidc()
	openidConf, err := m.getOpenIDConfig(ctx, oidc.GetIssuer())
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "authentication configuration error", otlpaudit.FAILREASON_TOKENINVALID)
		return OIDCSessionData{}, fmt.Errorf("getting openid configuration: %w", err)
	}

	tokens, err := m.exchangeCode(ctx, code, state.PKCEVerifier, oidc)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "identity provider rejected the login attempt", otlpaudit.FAILREASON_TOKENINVALID)
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
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "identity provider returned an invalid token", otlpaudit.FAILREASON_TOKENINVALID)
		return OIDCSessionData{}, fmt.Errorf("parsing id token: %w, %s", err, algs)
	}

	keyset, err := m.getProviderKeySet(ctx, openidConf)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "identity provider returned an invalid token", otlpaudit.FAILREASON_TOKENINVALID)
		return OIDCSessionData{}, fmt.Errorf("getting jwks for a provider: %w", err)
	}

	type CustomClaims struct {
		SID        string   `json:"sid"`
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
	var restClaims map[string]any
	if err := token.Claims(keyset, &standardClaims, &customClaims, &extraClaims, &restClaims); err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "identity provider returned an invalid token", otlpaudit.FAILREASON_TOKENINVALID)
		return OIDCSessionData{}, fmt.Errorf("getting JWT claims: %w", err)
	}

	globalUserID := m.resolveGlobalUserID(restClaims, standardClaims.Subject)
	metadata[otlpaudit.UserInitiatorIDKey] = globalUserID

	if err := standardClaims.Validate(jwt.Expected{
		Issuer:      oidc.GetIssuer(),
		AnyAudience: jwt.Audience{oidc.GetClientId()},
		Time:        time.Now(),
	}); err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "token verification failed", otlpaudit.FAILREASON_TOKENINVALID)
		return OIDCSessionData{}, fmt.Errorf("validating id token claims: %w", err)
	}

	if extraClaims.AtHash != "" {
		err := m.verifyAccessToken(tokens.AccessToken, extraClaims.AtHash, token)
		if err != nil {
			return OIDCSessionData{}, err
		}
	}

	// prepare the auth context used by ExtAuthZ
	authContext := map[string]string{
		"issuer":    oidc.GetIssuer(),
		"client_id": oidc.GetClientId(),
	}

	//nolint:forcetypeassert
	for _, param := range proto.GetExtension(oidc, flowv1.E_AuthContext).([]*flowv1.Attribute) {
		authContext[param.GetKey()] = param.GetValue()
	}

	session := Session{
		ID:         sessionID,
		TenantID:   state.TenantID,
		ProviderID: customClaims.SID,
		CSRFToken:  csrfToken,
		Issuer:     oidc.GetIssuer(),
		Claims: Claims{
			Subject:    standardClaims.Subject,
			UserUUID:   globalUserID,
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
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "session could not be established", "")
		return OIDCSessionData{}, fmt.Errorf("storing session: %w", err)
	}

	if err := m.sessions.BumpActive(ctx, session.ID, m.idleSessionTimeout); err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "session could not be established", "")
		return OIDCSessionData{}, fmt.Errorf("bumping session active status: %w", err)
	}

	err = m.sessions.DeleteState(ctx, stateID)
	if err != nil {
		m.sendUserLoginFailureAudit(ctx, metadata, state.TenantID, "session could not be established", "")
		return OIDCSessionData{}, fmt.Errorf("deleting state: %w", err)
	}

	// audit userLoginSuccess. A failure to create or send the audit event must
	// not fail the login: the session has already been established at this point.
	m.sendUserLoginSuccessAudit(ctx, metadata, state.TenantID)

	userHash := fmt.Sprintf("%x", sha256.Sum256([]byte(standardClaims.Subject)))
	m.userLastSeenGauge.Record(ctx, time.Now().Unix(), metric.WithAttributes(attribute.String("user.hash", userHash)))

	return OIDCSessionData{
		SessionID:  sessionID,
		TenantID:   state.TenantID,
		CSRFToken:  csrfToken,
		RequestURI: state.RequestURI,
		ErrorURI:   state.ErrorURI,
	}, nil
}

func (m *Manager) Logout(ctx context.Context, sessionID, postLogoutRedirectURL string) (string, error) {
	// The post logout redirect URL is handed back to the browser (and sent to the provider as
	// post_logout_redirect_uri), so reject a disallowed scheme up front, before touching any state:
	// it is a configuration error, and failing early lets an operator fix it and the user retry.
	if err := validation.SecureScheme(postLogoutRedirectURL, m.allowHttpScheme); err != nil {
		slogctx.Error(ctx, "the post logout redirect URL uses a disallowed scheme", "error", err)
		return "", fmt.Errorf("validating post logout redirect URL: %w", err)
	}

	session, err := m.sessions.LoadSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			// There is no such session (already logged out, expired, or a stale cookie). Logout is
			// idempotent, so redirect the user to the post logout URL; the handler still clears the
			// cookies on this path.
			slogctx.Warn(ctx, "logout attempt: session not found; redirecting the user to the post logout URL", "error", err)
			return postLogoutRedirectURL, nil
		}

		slogctx.Warn(ctx, "failed to get session by id", "error", err)
		return "", fmt.Errorf("getting session id: %w", err)
	}

	ctx = slogctx.With(ctx, "tenantId", session.TenantID)

	// Resolve where to send the user: the provider's RP-initiated logout URL when possible, or the
	// post logout URL otherwise. A configuration error is surfaced without logging the user out, so
	// it can be fixed and the logout retried.
	logoutURL, err := m.resolveLogoutURL(ctx, session, postLogoutRedirectURL)
	if err != nil {
		return "", err
	}

	// The user is being logged out: delete the local session, then redirect. A failed delete is not
	// fatal (the session expires on its own), but we fall back to the post logout URL rather than
	// bounce the user through the provider's end session endpoint with the session still present.
	if err := m.sessions.DeleteSession(ctx, session); err != nil && !errors.Is(err, serviceerr.ErrNotFound) {
		slogctx.Error(ctx, "failed to delete a session; redirecting the user to the post logout URL", "error", err)
		return postLogoutRedirectURL, nil
	}

	return logoutURL, nil
}

// resolveLogoutURL returns the URL to redirect the user to in order to complete logout: the
// provider's RP-initiated logout URL when one can be built, or postLogoutRedirectURL when
// RP-initiated logout is not possible but the user should still be logged out locally (trust
// removed, provider unreachable, no usable end session endpoint). It returns an error only for a
// configuration or infrastructure problem the caller must surface without logging the user out.
func (m *Manager) resolveLogoutURL(ctx context.Context, session Session, postLogoutRedirectURL string) (string, error) {
	trust, err := m.trust.Get(ctx, session.TenantID)
	if err != nil {
		if errors.Is(err, serviceerr.ErrNotFound) {
			slogctx.Warn(ctx, "logout attempt: trust configuration does not exist; logging out locally", "error", err)
			return postLogoutRedirectURL, nil
		}

		// A transient failure reading the trust config: keep the session so the user can retry.
		slogctx.Error(ctx, "failed to get trust for a tenant", "error", err)
		return "", fmt.Errorf("getting trust: %w", err)
	}

	oidc := trust.GetOidc()
	ctx = slogctx.With(ctx, "issuer", oidc.GetIssuer())

	// A disallowed issuer scheme is a configuration error (the same check guards login and refresh),
	// so fail early without logging the user out: once the configuration is fixed the user can retry
	// and get a complete logout, including at the provider.
	if err := validation.SecureScheme(oidc.GetIssuer(), m.allowHttpScheme); err != nil {
		slogctx.Error(ctx, "the issuer uses a disallowed scheme", "error", err)
		return "", fmt.Errorf("validating issuer: %w", err)
	}

	oidcConf, err := m.getOpenIDConfig(ctx, oidc.GetIssuer())
	if err != nil {
		// The issuer scheme is already validated, so this is a transient discovery failure: log the
		// user out locally rather than blocking logout on provider availability.
		slogctx.Warn(ctx, "failed to get oidc configuration; logging out locally", "error", err)
		return postLogoutRedirectURL, nil
	}

	if oidcConf.EndSessionEndpoint == "" {
		slogctx.Warn(ctx, "the provider does not support RP-Initiated Logout; logging out locally")
		return postLogoutRedirectURL, nil
	}

	// The end session endpoint is discovered dynamically, so its scheme can only be checked here; an
	// insecure or unparseable endpoint is unusable, so fall back to a local logout.
	if err := validation.SecureScheme(oidcConf.EndSessionEndpoint, m.allowHttpScheme); err != nil {
		slogctx.Warn(ctx, "the end session endpoint uses a disallowed scheme; logging out locally", "error", err)
		return postLogoutRedirectURL, nil
	}

	redirectURL, err := url.Parse(oidcConf.EndSessionEndpoint)
	if err != nil {
		slogctx.Warn(ctx, "failed to parse the end session endpoint; logging out locally", "error", err)
		return postLogoutRedirectURL, nil
	}

	vals := make(url.Values, 2)
	vals.Set("client_id", oidc.GetClientId())
	vals.Set("post_logout_redirect_uri", postLogoutRedirectURL)

	//nolint:forcetypeassert
	for _, param := range proto.GetExtension(oidc, flowv1.E_LogoutAttributes).([]*flowv1.Attribute) {
		vals.Set(param.GetKey(), param.GetValue())
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

	trust, err := m.trust.Get(ctx, session.TenantID)
	if err != nil {
		return fmt.Errorf("getting trust: %w", err)
	}

	oidc := trust.GetOidc()
	oidcConf, err := m.getOpenIDConfig(ctx, oidc.GetIssuer())
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

	checkCookie(ctx, csrfCookie)

	return csrfCookie, nil
}

func (m *Manager) MakeLoginCSRFCookie(ctx context.Context, value string) (*http.Cookie, error) {
	loginCSRFCookie := m.loginCSRFCookieTemplate.ToCookie(value)
	// Honor a configured cookie name (e.g. a non-"__Host-" name for local http
	// development, where the "__Host-" prefix's mandatory Secure attribute makes
	// the cookie unusable). Fall back to the hardened default when unset.
	if loginCSRFCookie.Name == "" {
		loginCSRFCookie.Name = LoginCSRFCookieName
	}
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

// sendUserLoginFailureAudit creates the user-login-failure audit event and sends it.
// The function logs any errors encountered while creating or sending the event but
// does not propagate them to the caller.
func (m *Manager) sendUserLoginFailureAudit(ctx context.Context, metadata otlpaudit.EventMetadata, objectID, payload string, reason otlpaudit.FailReason) {
	if m.audit == nil {
		slogctx.Warn(ctx, "Audit logger is not configured; skipping user login failure event")
		return
	}

	event, err := otlpaudit.NewUserLoginFailureEvent(metadata, objectID, otlpaudit.LOGINMETHOD_OPENIDCONNECT, reason, payload)
	if err != nil {
		slogctx.Error(ctx, "Failed to create user login failure event", "error", err)
		return
	}

	err = m.audit.SendEvent(ctx, event)
	if err != nil {
		slogctx.Error(ctx, "Failed to send audit log for user login failure", "error", err)
	}
	slogctx.Debug(ctx, "Audit log for user login failure has been sent")
}

// sendUserLoginSuccessAudit creates the user-login-success audit event and sends it.
// The function logs any errors encountered while creating or sending the event but
// does not propagate them to the caller: the session is already established by the
// time it is called, so a failed audit event must not fail the login.
func (m *Manager) sendUserLoginSuccessAudit(ctx context.Context, metadata otlpaudit.EventMetadata, objectID string) {
	if m.audit == nil {
		slogctx.Warn(ctx, "Audit logger is not configured; skipping user login success event")
		return
	}

	event, err := otlpaudit.NewUserLoginSuccessEvent(metadata, objectID, otlpaudit.LOGINMETHOD_OPENIDCONNECT, otlpaudit.MFATYPE_NONE, otlpaudit.USERTYPE_BUSINESS, objectID)
	if err != nil {
		slogctx.Error(ctx, "Failed to create user login success event", "error", err)
		return
	}

	if err := m.audit.SendEvent(ctx, event); err != nil {
		slogctx.Error(ctx, "Failed to send audit log for user login success", "error", err)
		return
	}
	slogctx.Debug(ctx, "Audit log for user login success has been sent")
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
	if subtle.ConstantTimeCompare([]byte(actual), []byte(atHash)) != 1 {
		return serviceerr.ErrInvalidAtHash
	}

	return nil
}

// IsValidRequestURI returns true if the URI is relative (no scheme or host),
// or if its scheme and host exactly match a configured base URL and its path
// is rooted under that base path.
func (m *Manager) IsValidRequestURI(requestURI string) bool {
	u, err := url.Parse(requestURI)
	if err != nil {
		return false
	}
	if u.Scheme == "" && u.Host == "" {
		return true
	}
	for _, b := range m.allowedRedirectBaseURLs {
		if strings.HasPrefix(requestURI, b.String()) {
			return true
		}
	}
	return false
}

func parseURLs(raw []string) []*url.URL {
	parsed := make([]*url.URL, 0, len(raw))
	for _, s := range raw {
		u, err := url.Parse(s)
		if err != nil {
			slog.Warn("skipping unparseable allowed redirect base URL", "url", s, "error", err)
			continue
		}
		parsed = append(parsed, u)
	}
	return parsed
}

func (m *Manager) exchangeCode(ctx context.Context, code, codeVerifier string, oidcTrust *oidcv1.OIDC) (*oidc.Tokens[*oidc.IDTokenClaims], error) {
	relyingParty, err := m.relyingParty(ctx, oidcTrust)
	if err != nil {
		return nil, fmt.Errorf("creating relying party: %w", err)
	}

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	}
	//nolint:forcetypeassert
	for _, param := range proto.GetExtension(oidcTrust, flowv1.E_TokenAttributes).([]*flowv1.Attribute) {
		opts = append(opts, oauth2.SetAuthURLParam(param.GetKey(), param.GetValue()))
	}

	oauthCtx := context.WithValue(ctx, oauth2.HTTPClient, relyingParty.HttpClient())
	token, err := relyingParty.OAuthConfig().Exchange(oauthCtx, code, opts...)
	if err != nil {
		return nil, fmt.Errorf("exchanging code: %w", err)
	}

	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, errors.New("missing or invalid id_token in token response")
	}

	return &oidc.Tokens[*oidc.IDTokenClaims]{Token: token, IDToken: idToken}, nil
}

func (m *Manager) ValidateCSRFToken(token, sessionID string) bool {
	return csrf.Validate(token, sessionID, m.csrfSecret)
}

func (m *Manager) resolveGlobalUserID(claims map[string]any, fallback string) string {
	for _, key := range m.globalUserIDClaimKeys {
		if claim, ok := claims[key]; ok {
			if val, ok := claim.(string); ok && len(strings.TrimSpace(val)) > 0 {
				return val
			}
		}
	}

	return fallback
}
