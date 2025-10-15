package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	otlpaudit "github.com/openkcm/common-sdk/pkg/otlp/audit"

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

type CallbackResult struct {
	SessionID   string
	CSRFToken   string
	RedirectURI string
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
	if err := m.createOperationInitiatedEvent(ctx, tenantID, "auth"); err != nil {
		return "", fmt.Errorf("creating auth initiated logging event failed: %w", err)
	}

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
		if auditErr := m.createOperationFailedEvent(ctx, tenantID, "already exists", "auth"); auditErr != nil {
			return "", fmt.Errorf("creating auth failed logging event failed: %w (original error: %w)", auditErr, err)
		}
		return "", fmt.Errorf("storing session: %w", err)
	}

	u, err := m.authURI(provider, state, pkce)
	if err != nil {
		return "", fmt.Errorf("generating auth uri: %w", err)
	}

	if auditErr := m.createOperationSuccessEvent(ctx, tenantID, stateID, "auth"); auditErr != nil {
		return "", fmt.Errorf("creating auth success logging event failed: %w", auditErr)
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

func (m *Manager) Callback(ctx context.Context, stateID, code, currentFingerprint string) (*CallbackResult, error) {
	state, err := m.sessions.LoadState(ctx, "", stateID)
	if err != nil {
		if auditErr := m.createOperationFailedEvent(ctx, "", "state load failed", "callback"); auditErr != nil {
			return nil, fmt.Errorf("creating callback failed logging event failed: %w (original error: %w)", auditErr, err)
		}
		return nil, serviceerr.ErrStateLoadFailed
	}

	if err := m.createOperationInitiatedEvent(ctx, state.TenantID, "callback"); err != nil {
		return nil, fmt.Errorf("creating callback initiated logging event failed: %w", err)
	}

	if time.Now().After(state.Expiry) {
		if auditErr := m.createOperationFailedEvent(ctx, state.TenantID, "state expired", "callback"); auditErr != nil {
			return nil, fmt.Errorf("creating callback failed logging event failed: %w (original error: %w)", auditErr, err)
		}
		return nil, serviceerr.ErrStateExpired
	}

	if state.Fingerprint != currentFingerprint {
		if auditErr := m.createOperationFailedEvent(ctx, state.TenantID, "fingerprint mismatch", "callback"); auditErr != nil {
			return nil, fmt.Errorf("creating callback failed logging event failed: %w (original error: %w)", auditErr, err)
		}
		return nil, serviceerr.ErrFingerprintMismatch
	}

	provider, err := m.oidc.GetForTenant(ctx, state.TenantID)
	if err != nil {
		return nil, fmt.Errorf("getting oidc provider: %w", err)
	}

	tokenSet, err := m.ExchangeCode(ctx, provider, code, state.PKCEVerifier)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for tokens: %w", err)
	}

	sessionID := m.pkce.SessionID()

	csrfToken := csrf.NewToken(sessionID, m.csrfSecret)

	claimsJSON, err := json.Marshal(tokenSet.IDToken)
	if err != nil {
		return nil, fmt.Errorf("marshaling claims: %w", err)
	}

	session := Session{
		ID:           sessionID,
		TenantID:     state.TenantID,
		Fingerprint:  currentFingerprint,
		CSRFToken:    csrfToken,
		Issuer:       provider.IssuerURL,
		Claims:       string(claimsJSON),
		AccessToken:  tokenSet.AccessToken,
		RefreshToken: tokenSet.RefreshToken,
		Expiry:       time.Now().Add(m.sessionDuration),
	}

	if err := m.sessions.StoreSession(ctx, state.TenantID, session); err != nil {
		return nil, fmt.Errorf("storing session: %w", err)
	}

	if err := m.sessions.DeleteState(ctx, state.TenantID, stateID); err != nil {
		return nil, fmt.Errorf("deleting state: %w", err)
	}

	if auditErr := m.createOperationSuccessEvent(ctx, state.TenantID, session.ID, "callback"); auditErr != nil {
		return nil, fmt.Errorf("creating callback success logging event failed: %w", auditErr)
	}
	return &CallbackResult{
		SessionID:   sessionID,
		CSRFToken:   csrfToken,
		RedirectURI: state.RequestURI,
	}, nil
}

func (m *Manager) ExchangeCode(ctx context.Context, provider oidc.Provider, code, codeVerifier string) (*oidc.TokenSet, error) {
	return provider.Exchange(ctx, code, codeVerifier, m.redirectURI, m.clientID)
}

func (m *Manager) ValidateCSRFToken(token, sessionID string) bool {
	return csrf.Validate(token, sessionID, m.csrfSecret)
}

func (m *Manager) createOperationInitiatedEvent(ctx context.Context, tenantID, operation string) error {
	eventMetadata, err := otlpaudit.NewEventMetadata(operation+"_user", tenantID, fmt.Sprintf("%s_initiated_%d", operation, time.Now().UnixNano()))
	if err != nil {
		return err
	}

	event, err := otlpaudit.NewUnauthenticatedRequestEvent(eventMetadata)
	if err != nil {
		return err
	}

	return m.audit.SendEvent(ctx, event)
}

func (m *Manager) createOperationSuccessEvent(ctx context.Context, tenantID, sessionID, operation string) error {
	eventMetadata, err := otlpaudit.NewEventMetadata(sessionID, tenantID, fmt.Sprintf("%s_success_%d", operation, time.Now().UnixNano()))
	if err != nil {
		return err
	}

	event, err := otlpaudit.NewUserLoginSuccessEvent(
		eventMetadata,
		sessionID,
		otlpaudit.LOGINMETHOD_OPENIDCONNECT,
		otlpaudit.MFATYPE_NONE,
		otlpaudit.USERTYPE_BUSINESS,
		operation,
	)
	if err != nil {
		return err
	}

	return m.audit.SendEvent(ctx, event)
}

func (m *Manager) createOperationFailedEvent(ctx context.Context, tenantID, reason, operation string) error {
	correlationID := fmt.Sprintf("%s_failed_%d", operation, time.Now().UnixNano())
	userID := "unknown"

	eventMetadata, err := otlpaudit.NewEventMetadata(userID, tenantID, correlationID)
	if err != nil {
		return err
	}

	var failReason otlpaudit.FailReason
	switch reason {
	case "state expired":
		failReason = otlpaudit.FAILREASON_SESSIONEXPIRED
	case "fingerprint mismatch":
		failReason = otlpaudit.FAILREASON_SESSIONREVOKED
	case "state load failed":
		failReason = otlpaudit.FAILREASON_SESSIONEXPIRED
	default:
		failReason = otlpaudit.FailReason(otlpaudit.UNSPECIFIED)
	}

	event, err := otlpaudit.NewUserLoginFailureEvent(
		eventMetadata,
		userID,
		otlpaudit.LOGINMETHOD_OPENIDCONNECT,
		failReason,
		operation,
	)
	if err != nil {
		return err
	}

	return m.audit.SendEvent(ctx, event)
}
