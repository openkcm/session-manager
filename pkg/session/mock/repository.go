package sessionmock

import (
	"context"

	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/pkg/session"
)

type Repository struct {
	States   map[string]session.State
	Sessions map[string]session.Session

	loadStateErr, storeStateErr, loadSessionErr, storeSessionErr, deleteStateErr error
}

func NewInMemRepository(loadStateErr, storeStateErr, loadSessionErr, storeSessionErr, deleteStateErr error) *Repository {
	return &Repository{
		States:          make(map[string]session.State),
		Sessions:        make(map[string]session.Session),
		loadStateErr:    loadStateErr,
		storeStateErr:   storeStateErr,
		loadSessionErr:  loadSessionErr,
		storeSessionErr: storeSessionErr,
		deleteStateErr:  deleteStateErr,
	}
}

func (r *Repository) LoadState(ctx context.Context, tenantID, stateID string) (session.State, error) {
	if r.loadStateErr != nil {
		return session.State{}, r.loadStateErr
	}

	if state, ok := r.States[stateID]; ok {
		return state, nil
	}

	return session.State{}, serviceerr.ErrNotFound
}

func (r *Repository) StoreState(ctx context.Context, tenantID string, state session.State) error {
	if r.storeStateErr != nil {
		return r.storeStateErr
	}

	if _, ok := r.States[state.ID]; ok {
		return serviceerr.ErrConflict
	}

	return nil
}

func (r *Repository) LoadSession(ctx context.Context, tenantID, sessionID string) (session.Session, error) {
	if r.loadSessionErr != nil {
		return session.Session{}, r.loadSessionErr
	}

	if s, ok := r.Sessions[sessionID]; ok {
		return s, nil
	}

	return session.Session{}, serviceerr.ErrNotFound
}

func (r *Repository) StoreSession(ctx context.Context, tenantID string, session session.Session) error {
	if r.storeSessionErr != nil {
		return r.storeSessionErr
	}

	if _, ok := r.Sessions[session.ID]; ok {
		return serviceerr.ErrConflict
	}

	return nil
}

func (r *Repository) DeleteState(ctx context.Context, tenantID, stateID string) error {
	if r.deleteStateErr != nil {
		return r.deleteStateErr
	}

	if _, ok := r.States[stateID]; !ok {
		return serviceerr.ErrNotFound
	}

	delete(r.States, stateID)
	return nil
}
