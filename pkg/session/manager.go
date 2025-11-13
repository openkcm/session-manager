package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	slogctx "github.com/veqryn/slog-context"

	oidcprovider "github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/pkg/csrf"
)

type Manager struct {
	oidc     oidcprovider.ProviderRepository
	sessions Repository
	audit    *otlpaudit.AuditLogger

	sessionDuration    time.Duration
	redirectURI        string
	clientID           string
	clientSecret       string
	secureClient       *http.Client
	getParametersAuth  []string
	getParametersToken []string
	authContextKeys    []string
	csrfSecret         []byte
	relyingParty       map[string]rp.RelyingParty
}

func NewManager(
	oidc oidcprovider.ProviderRepository,
	sessions Repository,
	auditLogger *otlpaudit.AuditLogger,
	sessionDuration time.Duration,
	getParametersAuth []string,
	getParametersToken []string,
	authContextKeys []string,
	redirectURI string,
	clientID string,
	clientSecret string,
	httpClient *http.Client,
	csrfHMACSecret string,
) *Manager {
	return &Manager{
		oidc:               oidc,
		sessions:           sessions,
		audit:              auditLogger,
		sessionDuration:    sessionDuration,
		getParametersAuth:  getParametersAuth,
		getParametersToken: getParametersToken,
		authContextKeys:    authContextKeys,
		redirectURI:        redirectURI,
		clientID:           clientID,
		clientSecret:       clientSecret,
		secureClient:       httpClient,
		csrfSecret:         []byte(csrfHMACSecret),
		relyingParty:       make(map[string]rp.RelyingParty),
	}
}

var (
	codeVerifierStore = make(map[string]string)
	codeVerifierMu    sync.Mutex
)

// getRelyingParty creates or returns cached Zitadel OIDC client
func (m *Manager) getRelyingParty(ctx context.Context, provider oidcprovider.Provider) (rp.RelyingParty, error) {
	if rpInst, exists := m.relyingParty[provider.IssuerURL]; exists {
		return rpInst, nil
	}

	scopes := []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail, "groups"}
	relyingParty, err := rp.NewRelyingPartyOIDC(
		ctx,
		provider.IssuerURL,
		m.clientID,
		m.clientSecret,
		m.redirectURI,
		scopes,
	)
	if err != nil {
		return nil, fmt.Errorf("creating relying party: %w", err)
	}

	m.relyingParty[provider.IssuerURL] = relyingParty
	return relyingParty, nil
}

// MakeAuthURI returns an OIDC authentication URI.
func (m *Manager) MakeAuthURI(ctx context.Context, tenantID, fingerprint, requestURI string) (string, error) {
	provider, err := m.oidc.GetForTenant(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("getting oidc provider: %w", err)
	}

	relyingParty, err := m.getRelyingParty(ctx, provider)
	if err != nil {
		return "", fmt.Errorf("getting an openid config: %w", err)
	}

	stateID := generateStateID()
	state := State{
		ID:          stateID,
		TenantID:    tenantID,
		Fingerprint: fingerprint,
		RequestURI:  requestURI,
		Expiry:      time.Now().Add(m.sessionDuration),
	}

	if err := m.sessions.StoreState(ctx, state); err != nil {
		return "", fmt.Errorf("storing session: %w", err)
	}

	// Generate code verifier and challenge for PKCE
	codeVerifier := generateCodeVerifier()
	codeChallenge := generateS256Challenge(codeVerifier)
	storeCodeVerifier(stateID, codeVerifier)

	authURL := rp.AuthURL(stateID, relyingParty, rp.WithCodeChallenge(codeChallenge))
	return authURL, nil
}

func (m *Manager) FinaliseOIDCLogin(ctx context.Context, stateID, code, fingerprint string) (OIDCSessionData, error) {
	state, err := m.sessions.LoadState(ctx, stateID)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("loading state from the storage: %w", err)
	}

	ctx = slogctx.With(ctx, "tenant_id", state.TenantID)

	if time.Now().After(state.Expiry) {
		return OIDCSessionData{}, serviceerr.ErrStateExpired
	}

	if state.Fingerprint != fingerprint {
		return OIDCSessionData{}, serviceerr.ErrFingerprintMismatch
	}

	provider, err := m.oidc.GetForTenant(ctx, state.TenantID)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("getting oidc provider: %w", err)
	}

	relyingParty, err := m.getRelyingParty(ctx, provider)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("getting relying party: %w", err)
	}

	codeVerifier := getCodeVerifier(stateID)
	tokens, err := rp.CodeExchange[*oidc.IDTokenClaims](ctx, code, relyingParty, rp.WithCodeVerifier(codeVerifier))
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("exchanging code for tokens: %w", err)
	}

	claims, err := rp.VerifyTokens[*oidc.IDTokenClaims](ctx, tokens.AccessToken, tokens.IDToken, relyingParty.IDTokenVerifier())
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("verifying tokens: %w", err)
	}

	sessionID := generateSessionID()
	csrfToken := csrf.NewToken(sessionID, m.csrfSecret)

	userInfo, err := rp.Userinfo[*oidc.UserInfo](ctx, tokens.AccessToken, tokens.TokenType, claims.GetSubject(), relyingParty)
	if err != nil {
		slogctx.Warn(ctx, "Failed to get user info", "error", err)
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
		RawClaims:   tokens.IDToken,
		Claims: Claims{
			Subject: claims.GetSubject(),
			Email:   getEmailFromUserInfo(userInfo),
			Groups:  getGroupsFromUserInfo(userInfo),
		},
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		AuthContext:  authContext,
		Expiry:       time.Now().Add(m.sessionDuration),
	}

	if err := m.sessions.StoreSession(ctx, session); err != nil {
		return OIDCSessionData{}, fmt.Errorf("storing session: %w", err)
	}

	if err := m.sessions.DeleteState(ctx, stateID); err != nil {
		return OIDCSessionData{}, fmt.Errorf("deleting state: %w", err)
	}

	return OIDCSessionData{
		SessionID:  sessionID,
		CSRFToken:  csrfToken,
		RequestURI: state.RequestURI,
	}, nil
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

// RefreshSession using Zitadel library
func (m *Manager) RefreshSession(ctx context.Context, s *Session, provider oidcprovider.Provider) error {
	relyingParty, err := m.getRelyingParty(ctx, provider)
	if err != nil {
		return fmt.Errorf("getting relying party: %w", err)
	}

	newTokens, err := rp.RefreshTokens[*oidc.IDTokenClaims](ctx, relyingParty, s.RefreshToken, "", "")
	if err != nil {
		return fmt.Errorf("refreshing tokens: %w", err)
	}

	s.AccessToken = newTokens.AccessToken
	s.RefreshToken = newTokens.RefreshToken
	s.AccessTokenExpiry = newTokens.Expiry

	return nil
}

func shouldRefresh(s Session) bool {
	return time.Until(s.AccessTokenExpiry) < 5*time.Minute
}

// Helper functions to implement
func generateStateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("state-%d", time.Now().UnixNano())
	}
	return "state-" + hex.EncodeToString(b)
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return "session-" + hex.EncodeToString(b)
}

func generateCodeVerifier() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "dummy_verifier"
	}
	return hex.EncodeToString(b)
}

func storeCodeVerifier(stateID, codeVerifier string) {
	codeVerifierMu.Lock()
	defer codeVerifierMu.Unlock()
	codeVerifierStore[stateID] = codeVerifier
}

func getCodeVerifier(stateID string) string {
	codeVerifierMu.Lock()
	defer codeVerifierMu.Unlock()
	return codeVerifierStore[stateID]
}

func getEmailFromUserInfo(userInfo *oidc.UserInfo) string {
	if userInfo != nil && userInfo.Email != "" {
		return userInfo.Email
	}
	return ""
}

func generateS256Challenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return strings.TrimRight(base64.URLEncoding.EncodeToString(hash[:]), "=")
}

func getGroupsFromUserInfo(userInfo *oidc.UserInfo) []string {
	if userInfo == nil {
		return nil
	}
	// Marshal userInfo to JSON, then unmarshal to map to access custom claims
	data, err := json.Marshal(userInfo)
	if err != nil {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	if groups, ok := raw["groups"].([]interface{}); ok {
		result := make([]string, 0, len(groups))
		for _, g := range groups {
			if s, ok := g.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}
