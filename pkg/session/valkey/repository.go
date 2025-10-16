package sessionvalkey

import (
	"context"
	"fmt"

	"github.com/valkey-io/valkey-go"

	"github.com/openkcm/session-manager/pkg/session"
)

const objectTypeSession = "session"
const objectTypeState = "state"

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

func (r *Repository) LoadState(ctx context.Context, stateID string) (state session.State, _ error) {
	if err := r.store.Get(ctx, objectTypeState, stateID, &state); err != nil {
		return session.State{}, fmt.Errorf("getting state from store: %w", err)
	}

	return state, nil
}

func (r *Repository) StoreState(ctx context.Context, state session.State) error {
	if err := r.store.Set(ctx, objectTypeState, state.ID, state); err != nil {
		return fmt.Errorf("setting state into storage: %w", err)
	}

	return nil
}

func (r *Repository) LoadSession(ctx context.Context, sessionID string) (s session.Session, _ error) {
	if err := r.store.Get(ctx, objectTypeSession, sessionID, &s); err != nil {
		return session.Session{}, fmt.Errorf("getting session from store: %w", err)
	}

	return s, nil
}

func (r *Repository) StoreSession(ctx context.Context, s session.Session) error {
	if err := r.store.Set(ctx, objectTypeSession, s.ID, s); err != nil {
		return fmt.Errorf("setting session into storage: %w", err)
	}

	return nil
}

func (r *Repository) DeleteState(ctx context.Context, stateID string) error {
	if err := r.store.Destroy(ctx, objectTypeState, stateID); err != nil {
		return fmt.Errorf("deleting state from store: %w", err)
	}

	return nil
}
