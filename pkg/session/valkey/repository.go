package sessionvalkey

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/valkey-io/valkey-go"

	"github.com/openkcm/session-manager/pkg/session"
)

const objectTypeSession = "session"
const objectTypeState = "state"

type Repository struct {
	store *store
}

func (r *Repository) GetAllSessions(ctx context.Context) ([]session.Session, error) {
	var sessions []session.Session
	var cursor uint64 = 0
	pattern := r.store.prefix + ":" + objectTypeSession + ":*"

	for {
		resp := r.store.valkey.Do(ctx, r.store.valkey.B().Scan().Cursor(cursor).Match(pattern).Count(100).Build())
		if resp.Error() != nil {
			return nil, fmt.Errorf("scanning session keys: %w", resp.Error())
		}

		// Parse the SCAN response as []interface{}
		vals, err := resp.ToArray()
		if err != nil || len(vals) != 2 {
			return nil, fmt.Errorf("unexpected scan response: %v", err)
		}

		// First element is new cursor (string), second is []string of keys
		cursorStr, err := vals[0].ToString()
		if err != nil {
			return nil, fmt.Errorf("unexpected cursor type: %v", err)
		}
		cursor, err = strconv.ParseUint(cursorStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing scan cursor: %w", err)
		}

		keyArr, err := vals[1].ToArray()
		if err != nil {
			return nil, fmt.Errorf("unexpected keys type: %v", err)
		}
		for _, k := range keyArr {
			key, err := k.ToString()
			if err != nil {
				continue
			}
			parts := strings.Split(key, ":")
			if len(parts) < 4 {
				continue // Invalid key format
			}
			tenantID := parts[len(parts)-2]
			sessionID := parts[len(parts)-1]
			s, err := r.LoadSession(ctx, tenantID, sessionID)
			if err != nil {
				continue
			}
			sessions = append(sessions, s)
		}
		if cursor == 0 {
			break
		}
	}
	return sessions, nil
}

func NewRepository(valkeyClient valkey.Client, prefix string) *Repository {
	return &Repository{
		store: newStore(valkeyClient, prefix),
	}
}

func (r *Repository) LoadState(ctx context.Context, tenantID, stateID string) (state session.State, _ error) {
	if err := r.store.Get(ctx, objectTypeState, tenantID, stateID, &state); err != nil {
		return session.State{}, fmt.Errorf("getting state from store: %w", err)
	}

	return state, nil
}

func (r *Repository) StoreState(ctx context.Context, tenantID string, state session.State) error {
	if err := r.store.Set(ctx, objectTypeState, tenantID, state.ID, state); err != nil {
		return fmt.Errorf("setting state into storage: %w", err)
	}

	return nil
}

func (r *Repository) LoadSession(ctx context.Context, tenantID, sessionID string) (s session.Session, _ error) {
	if err := r.store.Get(ctx, objectTypeSession, tenantID, sessionID, &s); err != nil {
		return session.Session{}, fmt.Errorf("getting session from store: %w", err)
	}

	return s, nil
}

func (r *Repository) StoreSession(ctx context.Context, tenantID string, s session.Session) error {
	if err := r.store.Set(ctx, objectTypeSession, tenantID, s.ID, s); err != nil {
		return fmt.Errorf("setting session into storage: %w", err)
	}

	return nil
}
