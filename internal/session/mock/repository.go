package sessionmock

import (
	"context"

	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/session"
)

type RepositoryOption func(*Repository)

type Repository struct {
	states          map[string]session.State
	sessions        map[string]session.Session
	providerSession map[string]session.Session

	loadStateErr, storeStateErr, deleteStateErr       error
	loadSessionErr, storeSessionErr, deleteSessionErr error
}

func WithState(state session.State) RepositoryOption {
	return func(r *Repository) { r.states[state.ID] = state }
}
func WithSession(sess session.Session) RepositoryOption {
	return func(r *Repository) { r.sessions[sess.ID] = sess }
}
func WithLoadStateError(err error) RepositoryOption {
	return func(r *Repository) { r.loadStateErr = err }
}
func WithStoreStateError(err error) RepositoryOption {
	return func(r *Repository) { r.storeStateErr = err }
}
func WithDeleteStateError(err error) RepositoryOption {
	return func(r *Repository) { r.deleteStateErr = err }
}
func WithLoadSessionError(err error) RepositoryOption {
	return func(r *Repository) { r.loadSessionErr = err }
}
func WithStoreSessionError(err error) RepositoryOption {
	return func(r *Repository) { r.storeSessionErr = err }
}
func WithDeleteSessionError(err error) RepositoryOption {
	return func(r *Repository) { r.deleteSessionErr = err }
}

var _ = session.Repository(&Repository{})

func (r *Repository) ListSessions(ctx context.Context) ([]session.Session, error) {
	sessions := make([]session.Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func NewInMemRepository(opts ...RepositoryOption) *Repository {
	r := &Repository{
		states:          make(map[string]session.State),
		sessions:        make(map[string]session.Session),
		providerSession: make(map[string]session.Session),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

func (r *Repository) LoadState(_ context.Context, stateID string) (session.State, error) {
	if r.loadStateErr != nil {
		return session.State{}, r.loadStateErr
	}
	if state, ok := r.states[stateID]; ok {
		return state, nil
	}
	return session.State{}, serviceerr.ErrNotFound
}

func (r *Repository) StoreState(_ context.Context, state session.State) error {
	if r.storeStateErr != nil {
		return r.storeStateErr
	}
	if _, ok := r.states[state.ID]; ok {
		return serviceerr.ErrConflict
	}
	return nil
}

func (r *Repository) DeleteState(_ context.Context, stateID string) error {
	if r.deleteStateErr != nil {
		return r.deleteStateErr
	}
	if _, ok := r.states[stateID]; !ok {
		return serviceerr.ErrNotFound
	}
	delete(r.states, stateID)
	return nil
}

func (r *Repository) LoadSession(_ context.Context, sessionID string) (session.Session, error) {
	if r.loadSessionErr != nil {
		return session.Session{}, r.loadSessionErr
	}
	if s, ok := r.sessions[sessionID]; ok {
		return s, nil
	}
	return session.Session{}, serviceerr.ErrNotFound
}

func (r *Repository) LoadSessionByProviderID(_ context.Context, providerID string) (session.Session, error) {
	if r.loadSessionErr != nil {
		return session.Session{}, r.loadSessionErr
	}
	if s, ok := r.providerSession[providerID]; ok {
		return s, nil
	}
	return session.Session{}, serviceerr.ErrNotFound
}

func (r *Repository) StoreSession(_ context.Context, sess session.Session) error {
	if r.storeSessionErr != nil {
		return r.storeSessionErr
	}
	if _, ok := r.sessions[sess.ID]; ok {
		return serviceerr.ErrConflict
	}
	r.sessions[sess.ID] = sess
	r.providerSession[sess.ProviderID] = sess
	return nil
}

func (r *Repository) DeleteSession(_ context.Context, sess session.Session) error {
	if r.deleteSessionErr != nil {
		return r.deleteSessionErr
	}
	if _, ok := r.sessions[sess.ID]; !ok {
		return serviceerr.ErrNotFound
	}
	delete(r.sessions, sess.ID)
	delete(r.providerSession, sess.ProviderID)
	return nil
}
