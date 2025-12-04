package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/oidc"
)

// RefreshExpiringTokens refreshes access tokens that are nearing expiration.
func (m *Manager) RefreshExpiringTokens(ctx context.Context) error {
	sessions, err := m.sessions.ListSessions(ctx)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		provider, err := m.oidc.Get(ctx, s.TenantID)
		if err != nil {
			return fmt.Errorf("getting OIDC provider: %w", err)
		}

		if shouldRefresh(s) {
			if err := m.refreshExpiringToken(ctx, &s, provider); err != nil {
				slogctx.Warn(ctx, "Could not refresh token", "tenant_id", s.TenantID, "error", err)
				continue
			}

			if err := m.sessions.StoreSession(ctx, s); err != nil {
				slogctx.Warn(ctx, "Could not store refreshed session", "tenant_id", s.TenantID, "error", err)
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

// refreshExpiringToken refreshes the access token for the given session if needed.
func (m *Manager) refreshExpiringToken(ctx context.Context, s *Session, provider oidc.Provider) error {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", s.RefreshToken)
	// The client_id is already set in the manager's secureClient.
	// data.Set("client_id", m.clientID)
	for _, parameter := range m.queryParametersToken {
		value, ok := provider.Properties[parameter]
		if !ok {
			return fmt.Errorf("missing token parameter: %s", parameter)
		}
		data.Set(parameter, value)
	}

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

// CleanupIdleSessions deletes sessions that have been idle for longer than the specified timeout.
func (m *Manager) CleanupIdleSessions(ctx context.Context, timeout time.Duration) error {
	sessions, err := m.sessions.ListSessions(ctx)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if time.Since(s.LastVisited) < timeout {
			continue
		}
		if err := m.sessions.DeleteSession(ctx, s); err != nil {
			slogctx.Warn(ctx, "Could not delete idle session", "tenant_id", s.TenantID, "error", err)
			continue
		}
		slogctx.Info(ctx, "Deleted idle session", "tenant_id", s.TenantID)
		continue
	}
	return nil
}
