package session

import (
	"context"
	"time"
)

type Repository interface {
	// State operations
	LoadState(ctx context.Context, stateID string) (State, error)
	StoreState(ctx context.Context, state State) error
	DeleteState(ctx context.Context, stateID string) error

	// Session operations
	ListSessions(ctx context.Context) ([]Session, error)
	LoadSession(ctx context.Context, sessionID string) (Session, error)
	LoadSessionByProviderID(ctx context.Context, providerID string) (Session, error)
	StoreSession(ctx context.Context, session Session) error
	DeleteSession(ctx context.Context, session Session) error
	IsActive(ctx context.Context, sessionID string) (bool, error)
	BumpActive(ctx context.Context, sessionID string, timeout time.Duration) error
}
