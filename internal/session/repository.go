package session

import "context"

type Repository interface {
	LoadState(ctx context.Context, tenantID, stateID string) (State, error)
	StoreState(ctx context.Context, tenantID string, state State) error
	LoadSession(ctx context.Context, tenantID, sessionID string) (Session, error)
	StoreSession(ctx context.Context, tenantID string, session Session) error
}
