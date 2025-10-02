package session

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"
	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/internal/pkce"
)

type Manager struct {
	oidc     oidc.ProviderRepository
	sessions Repository
	pkce     pkce.Source
	audit    *otlpaudit.AuditLogger

	sessionDuration time.Duration
	redirectURI     string
	clientID        string
}

func NewManager(
	oidc oidc.ProviderRepository,
	sessions Repository,
	auditLogger *otlpaudit.AuditLogger,
	sessionDuration time.Duration,
	redirectURI,
	clientID string,
) *Manager {
	return &Manager{
		oidc:            oidc,
		sessions:        sessions,
		audit:           auditLogger,
		sessionDuration: sessionDuration,
		redirectURI:     redirectURI,
		clientID:        clientID,
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

	if err := m.sessions.StoreState(ctx, tenantID, state); err != nil {
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

func (m *Manager) StartTokenRefresher(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := m.RefreshTokens(ctx); err != nil {
					log.Printf("[ERROR] Failed to refresh tokens: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *Manager) RefreshTokens(ctx context.Context) error {
	sessions, err := m.sessions.GetAllSessions(ctx)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if shouldRefresh(s) {
			provider, err := m.oidc.GetForTenant(ctx, s.TenantID)
			if err != nil {
				log.Printf("[WARN] Could not get OIDC provider for tenant %s: %v", s.TenantID, err)
				continue
			}
			newToken, err := provider.RefreshToken(ctx, s.RefreshToken, m.clientID)
			if err != nil {
				log.Printf("[WARN] Could not refresh token for tenant %s, session %s: %v", s.TenantID, s.ID, err)
				continue
			}
			s.AccessToken = newToken.AccessToken
			s.RefreshToken = newToken.RefreshToken
			s.AccessTokenExpiry = newToken.ExpiresAt
			if err := m.sessions.StoreSession(ctx, s.TenantID, s); err != nil {
				log.Printf("[WARN] Could not store refreshed session for tenant %s, session %s: %v", s.TenantID, s.ID, err)
				continue
			}
		}
	}
	return nil
}

func shouldRefresh(s Session) bool {
	// refresh if token expires in less than 5 minutes
	return time.Until(s.AccessTokenExpiry) < 5*time.Minute
}
