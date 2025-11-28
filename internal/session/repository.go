package session

import "context"

type Repository interface {
	LoadState(ctx context.Context, stateID string) (State, error)
	StoreState(ctx context.Context, state State) error
	LoadSession(ctx context.Context, sessionID string) (Session, error)
	StoreSession(ctx context.Context, session Session) error
	DeleteState(ctx context.Context, stateID string) error
	ListSessions(ctx context.Context) ([]Session, error)
}
