package sessionvalkey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/session-manager/internal/session"
)

type ObjectType string

const (
	objectTypeSession         ObjectType = "session"
	objectTypeState           ObjectType = "state"
	objectTypeProviderSession ObjectType = "providerSession"
	objectTypeAccessToken     ObjectType = "accessToken"
	objectTypeRefreshToken    ObjectType = "refreshToken"
	objectTypeProviderToken   ObjectType = "providerToken"
)

var (
	ErrGetSessions           = errors.New("getting sessions from store")
	ErrGetState              = errors.New("getting state from store")
	ErrStoreState            = errors.New("setting state into storage")
	ErrStoreSession          = errors.New("setting session into storage")
	ErrGetSession            = errors.New("getting session from store")
	ErrGetSessIDByProviderID = errors.New("getting session ID by provider ID from store")
	ErrGetAccessToken        = errors.New("getting access token from store")
	ErrGetRefreshToken       = errors.New("getting refresh token from store")
)

type Repository struct {
	store *store
}

var _ = session.Repository(&Repository{})

func (r *Repository) ListSessions(ctx context.Context) ([]session.Session, error) {
	var sessions []session.Session
	if err := getStoreObjects(ctx, r.store, objectTypeSession, "*", &sessions); err != nil {
		return nil, errors.Join(ErrGetSessions, err)
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
		return session.State{}, errors.Join(ErrGetState, err)
	}

	return state, nil
}

func (r *Repository) StoreState(ctx context.Context, state session.State) error {
	duration := time.Until(state.Expiry)
	if err := r.store.Set(ctx, objectTypeState, state.ID, state, duration); err != nil {
		return errors.Join(ErrStoreState, err)
	}

	return nil
}

func (r *Repository) LoadSession(ctx context.Context, sessionID string) (session.Session, error) {
	var s session.Session
	if err := r.store.Get(ctx, objectTypeSession, sessionID, &s); err != nil {
		return session.Session{}, errors.Join(ErrGetSession, err)
	}

	return s, nil
}

func (r *Repository) GetSessIDByProviderID(ctx context.Context, providerID string) (string, error) {
	var s string
	if err := r.store.Get(ctx, objectTypeProviderSession, getObjectID(objectTypeProviderSession, providerID), &s); err != nil {
		return "", errors.Join(ErrGetSessIDByProviderID, err)
	}

	return s, nil
}

func (r *Repository) GetAccessTokenForSession(ctx context.Context, sessionID string) (string, error) {
	var accessToken string
	if err := r.store.Get(ctx, objectTypeAccessToken, getObjectID(objectTypeAccessToken, sessionID), &accessToken); err != nil {
		return "", errors.Join(ErrGetAccessToken, err)
	}

	return accessToken, nil
}

func (r *Repository) GetRefreshTokenForSession(ctx context.Context, sessionID string) (string, error) {
	var refreshToken string
	if err := r.store.Get(ctx, objectTypeRefreshToken, getObjectID(objectTypeRefreshToken, sessionID), &refreshToken); err != nil {
		return "", errors.Join(ErrGetRefreshToken, err)
	}

	return refreshToken, nil
}

func (r *Repository) StoreSession(ctx context.Context, s session.Session) error {
	duration := time.Until(s.Expiry)
	var errs []error
	if err := r.store.Set(ctx, objectTypeProviderSession, getObjectID(objectTypeProviderSession, s.ProviderID), s.ID, duration); err != nil {
		errs = append(errs, err)
	}

	if err := r.store.Set(ctx, objectTypeAccessToken, getObjectID(objectTypeAccessToken, s.ID), s.AccessToken, duration); err != nil {
		errs = append(errs, err)
	}

	if err := r.store.Set(ctx, objectTypeRefreshToken, getObjectID(objectTypeRefreshToken, s.ID), s.RefreshToken, duration); err != nil {
		errs = append(errs, err)
	}

	if err := r.store.Set(ctx, objectTypeSession, s.ID, s, duration); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		err := r.DeleteSession(ctx, s)
		if err != nil {
			slogctx.Error(ctx, "couldn't delete session during rollback", "error", err)
			return err
		}
		return ErrStoreSession
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
	err = r.store.Destroy(ctx, objectTypeProviderSession, getObjectID(objectTypeProviderSession, s.ProviderID))
	if err != nil {
		return err
	}
	err = r.store.Destroy(ctx, objectTypeAccessToken, getObjectID(objectTypeAccessToken, s.ID))
	if err != nil {
		return err
	}
	err = r.store.Destroy(ctx, objectTypeRefreshToken, getObjectID(objectTypeRefreshToken, s.ID))
	if err != nil {
		return err
	}
	return nil
}

func getObjectID(prefix ObjectType, objectID string) string {
	return fmt.Sprintf("%s_%s", prefix, objectID)
}
