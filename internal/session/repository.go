package session

import "context"

type Repository interface {
	// State operations
	LoadState(ctx context.Context, stateID string) (State, error)
	StoreState(ctx context.Context, state State) error
	DeleteState(ctx context.Context, stateID string) error
	// Session operations
	LoadSession(ctx context.Context, sessionID string) (Session, error)
	StoreSession(ctx context.Context, session Session) error
	ListSessions(ctx context.Context) ([]Session, error)
	DeleteSession(ctx context.Context, session Session) error
}
