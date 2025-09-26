package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/pkce"
	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/pkg/csrf"
)

type Manager struct {
	oidc     oidc.ProviderRepository
	sessions Repository
	pkce     pkce.Source
	audit    *otlpaudit.AuditLogger

	sessionDuration time.Duration
	redirectURI     string
	clientID        string

	csrfSecret []byte
}

func NewManager(
	oidc oidc.ProviderRepository,
	sessions Repository,
	auditLogger *otlpaudit.AuditLogger,
	sessionDuration time.Duration,
	redirectURI,
	clientID string,
	csrfHMACSecret string,
) *Manager {
	return &Manager{
		oidc:            oidc,
		sessions:        sessions,
		audit:           auditLogger,
		sessionDuration: sessionDuration,
		redirectURI:     redirectURI,
		clientID:        clientID,
		csrfSecret:      []byte(csrfHMACSecret),
	}
}

// Auth returns an OIDC authorise URI.
func (m *Manager) Auth(ctx context.Context, tenantID, fingerprint, requestURI string) (string, error) {
	provider, err := m.oidc.GetForTenant(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("getting oidc provider: %w", err)
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

	u, err := m.authURI(provider, state, pkce)
	if err != nil {
		return "", fmt.Errorf("generating auth uri: %w", err)
	}

	return u, nil
}

func (m *Manager) authURI(provider oidc.Provider, state State, pkce pkce.PKCE) (string, error) {
	u, err := url.Parse(provider.IssuerURL)
	if err != nil {
		return "", fmt.Errorf("parsing issuer url: %w", err)
	}

	q := u.Query()
	q["scope"] = append(q["scope"], "openid", "profile", "email", "groups")
	q.Set("response_type", "code")
	q.Set("client_id", m.clientID)
	q.Set("state", state.ID)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", pkce.Method)
	q.Set("redirect_uri", m.redirectURI)

	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (m *Manager) FinaliseOIDCLogin(ctx context.Context, stateID, code, fingerprint string) (OIDCSessionData, error) {
	state, err := m.sessions.LoadState(ctx, stateID)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("loading state from the storage: %w", err)
	}

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

	tokenSet, err := m.exchangeCode(ctx, provider, code, state.PKCEVerifier)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("exchanging code for tokens: %w", err)
	}

	sessionID := m.pkce.SessionID()
	csrfToken := csrf.NewToken(sessionID, m.csrfSecret)

	// TODO: which claims should we use?
	claimsJSON, err := json.Marshal(tokenSet.IDToken)
	if err != nil {
		return OIDCSessionData{}, fmt.Errorf("marshaling claims: %w", err)
	}

	session := Session{
		ID:           sessionID,
		TenantID:     state.TenantID,
		Fingerprint:  fingerprint,
		CSRFToken:    csrfToken,
		Issuer:       provider.IssuerURL,
		Claims:       string(claimsJSON),
		AccessToken:  tokenSet.AccessToken,
		RefreshToken: tokenSet.RefreshToken,
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

func (m *Manager) exchangeCode(ctx context.Context, provider oidc.Provider, code, codeVerifier string) (tokenSet, error) {
	tokenEndpoint := provider.IssuerURL + "/token"

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", m.redirectURI)
	data.Set("client_id", m.clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return tokenSet{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenSet{}, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return tokenSet{}, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var tokens tokenSet
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return tokenSet{}, fmt.Errorf("decoding response: %w", err)
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
				slogctx.Warn(ctx, "Could not refresh token", "tenant_id", s.TenantID, "session_id", s.ID)
				continue
			}

			if err := m.sessions.StoreSession(ctx, s); err != nil {
				slogctx.Warn(ctx, "Could not store refreshed session", "tenant_id", s.TenantID, "session_id", s.ID)
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

	tokenEndpoint := path.Join(provider.IssuerURL, "/token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
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

func shouldRefresh(s Session) bool {
	// refresh if token expires in less than 5 minutes
	return time.Until(s.AccessTokenExpiry) < 5*time.Minute
}
