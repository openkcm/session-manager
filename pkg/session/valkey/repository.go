package sessionvalkey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/openkcm/session-manager/pkg/session"
)

const (
	objectTypeSession         = "session"
	objectTypeState           = "state"
	objectTypeProviderSession = "providerSession"
	objectTypeAccessToken     = "accessToken"
	objectTypeRefreshToken    = "refreshToken"
	refreshTokenPrefix        = "refreshToken_"
	accessTokenPrefix         = "accessToken_"
	providerPrefix            = "providerToken_"
)

type Repository struct {
	store *store
}

func (r *Repository) ListSessions(ctx context.Context) ([]session.Session, error) {
	var sessions []session.Session
	if err := getStoreObjects(ctx, r.store, objectTypeSession, "*", &sessions); err != nil {
		return nil, fmt.Errorf("getting sessions from store: %w", err)
	}

	return sessions, nil
}

func NewRepository(valkeyClient valkey.Client, prefix string) *Repository {
	return &Repository{
		store: newStore(valkeyClient, prefix),
	}
}

func (r *Repository) LoadState(ctx context.Context, stateID string) (session.State, error) {
	var state session.State
	if err := r.store.Get(ctx, objectTypeState, stateID, &state); err != nil {
		return session.State{}, fmt.Errorf("getting state from store: %w", err)
	}

	return state, nil
}

func (r *Repository) StoreState(ctx context.Context, state session.State) error {
	duration := time.Until(state.Expiry)
	if err := r.store.Set(ctx, objectTypeState, state.ID, state, duration); err != nil {
		return fmt.Errorf("setting state into storage: %w", err)
	}

	return nil
}

func (r *Repository) LoadSession(ctx context.Context, sessionID string) (session.Session, error) {
	var s session.Session
	if err := r.store.Get(ctx, objectTypeSession, sessionID, &s); err != nil {
		return session.Session{}, fmt.Errorf("getting session from store: %w", err)
	}

	return s, nil
}

func (r *Repository) GetSessIDByProviderID(ctx context.Context, providerID string) (string, error) {
	var s string
	if err := r.store.Get(ctx, objectTypeProviderSession, providerPrefix+providerID, &s); err != nil {
		return "", fmt.Errorf("getting session from store: %w", err)
	}

	return s, nil
}

func (r *Repository) GetAccessTokenForSession(ctx context.Context, sessionID string) (string, error) {
	var accessToken string
	if err := r.store.Get(ctx, objectTypeAccessToken, accessTokenPrefix+sessionID, &accessToken); err != nil {
		return "", fmt.Errorf("getting accessToken from store: %w", err)
	}

	return accessToken, nil
}

func (r *Repository) GetRefreshTokenForSession(ctx context.Context, sessionID string) (string, error) {
	var refreshToken string
	if err := r.store.Get(ctx, objectTypeRefreshToken, refreshTokenPrefix+sessionID, &refreshToken); err != nil {
		return "", fmt.Errorf("getting refreshToken from store: %w", err)
	}

	return refreshToken, nil
}

func (r *Repository) StoreSession(ctx context.Context, s session.Session) error {
	duration := time.Until(s.Expiry)
	var errs []error
	if err := r.store.Set(ctx, objectTypeProviderSession, providerPrefix+s.ProviderID, s.ID, duration); err != nil {
		errs = append(errs, err)
	}

	if err := r.store.Set(ctx, objectTypeAccessToken, accessTokenPrefix+s.ID, s.AccessToken, duration); err != nil {
		errs = append(errs, err)
	}

	if err := r.store.Set(ctx, objectTypeRefreshToken, refreshTokenPrefix+s.ID, s.RefreshToken, duration); err != nil {
		errs = append(errs, err)
	}

	if err := r.store.Set(ctx, objectTypeSession, s.ID, s, duration); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		err := r.DeleteSession(ctx, s)
		if err != nil {
			return err
		}
		return errors.New("storing session in store")
	}

	return nil
}

func (r *Repository) DeleteState(ctx context.Context, stateID string) error {
	if err := r.store.Destroy(ctx, objectTypeState, stateID); err != nil {
		return fmt.Errorf("deleting state from store: %w", err)
	}

	return nil
}

func (r *Repository) DeleteSession(ctx context.Context, s session.Session) error {
	err := r.store.Destroy(ctx, objectTypeSession, s.ID)
	if err != nil {
		return err
	}
	err = r.store.Destroy(ctx, objectTypeProviderSession, providerPrefix+s.ProviderID)
	if err != nil {
		return err
	}
	err = r.store.Destroy(ctx, objectTypeAccessToken, accessTokenPrefix+s.AccessToken)
	if err != nil {
		return err
	}
	err = r.store.Destroy(ctx, objectTypeRefreshToken, refreshTokenPrefix+s.RefreshToken)
	if err != nil {
		return err
	}
	return nil
}
