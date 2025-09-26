package sessionsql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openkcm/session-manager/internal/serviceerr"
	"github.com/openkcm/session-manager/internal/session"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func setTenantContext(ctx context.Context, tx pgx.Tx, tenantID string) error {
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true);`, tenantID); err != nil {
		return fmt.Errorf("setting app.tenant_id: %w", err)
	}

	return nil
}

func (r *Repository) LoadState(ctx context.Context, tenantID, stateID string) (state session.State, _ error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return session.State{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := setTenantContext(ctx, tx, tenantID); err != nil {
		return session.State{}, fmt.Errorf("setting tenant context: %w", err)
	}

	if err := tx.QueryRow(ctx, `SELECT id, tenant_id, fingerprint, verifier, request_uri, expiry
FROM pkce_state
WHERE id = $1
	AND tenant_id = current_setting('app.tenant_id');`,
		stateID,
	).
		Scan(&state.ID, &state.TenantID, &state.Fingerprint, &state.PKCEVerifier, &state.RequestURI, &state.Expiry); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return state, serviceerr.ErrNotFound
		}

		return session.State{}, fmt.Errorf("selecting from pkce_state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return session.State{}, fmt.Errorf("committing tx: %w", err)
	}

	return state, nil
}

func (r *Repository) StoreState(ctx context.Context, tenantID string, state session.State) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := setTenantContext(ctx, tx, tenantID); err != nil {
		return fmt.Errorf("setting tenant context: %w", err)
	}

	if _, err := tx.Exec(
		ctx, `INSERT INTO pkce_state (id, tenant_id, fingerprint, verifier, request_uri, expiry)
	VALUES ($1, current_setting('app.tenant_id'), $2, $3, $4, $5)
	ON CONFLICT (id)
	DO UPDATE SET (tenant_id, fingerprint, verifier, request_uri, expiry) =
		(EXCLUDED.tenant_id, EXCLUDED.fingerprint, EXCLUDED.verifier, EXCLUDED.request_uri, EXCLUDED.expiry);`,
		state.ID, state.Fingerprint, state.PKCEVerifier, state.RequestURI, state.Expiry,
	); err != nil {
		if err, ok := handlePgError(err); ok {
			return err
		}

		return fmt.Errorf("inserting into pkce_state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing tx: %w", err)
	}

	return nil
}

func (r *Repository) LoadSession(ctx context.Context, tenantID, sessionID string) (s session.Session, _ error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return session.Session{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := setTenantContext(ctx, tx, tenantID); err != nil {
		return session.Session{}, fmt.Errorf("setting tenant context: %w", err)
	}

	if err := tx.QueryRow(
		ctx, `SELECT state_id, tenant_id, fingerprint, token, expiry
FROM sessions
WHERE state_id = $1
	AND tenant_id = current_setting('app.tenant_id');`,
		sessionID,
	).
		Scan(&s.ID, &s.TenantID, &s.Fingerprint, &s.Token, &s.Expiry); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return session.Session{}, serviceerr.ErrNotFound
		}

		return session.Session{}, fmt.Errorf("selecting from sessions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return session.Session{}, fmt.Errorf("committing tx: %w", err)
	}

	return s, nil
}

func (r *Repository) StoreSession(ctx context.Context, tenantID string, session session.Session) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := setTenantContext(ctx, tx, tenantID); err != nil {
		return fmt.Errorf("setting tenant context: %w", err)
	}

	if _, err := tx.Exec(
		ctx, `INSERT INTO sessions (state_id, tenant_id, fingerprint, token, expiry)
VALUES ($1, current_setting('app.tenant_id'), $2, $3, $4)
	ON CONFLICT (state_id)
	DO UPDATE SET (tenant_id, fingerprint, token, expiry) =
		(EXCLUDED.tenant_id, EXCLUDED.fingerprint, EXCLUDED.token, EXCLUDED.expiry);`,
		session.ID, session.Fingerprint, session.Token, session.Expiry,
	); err != nil {
		if err, ok := handlePgError(err); ok {
			return err
		}

		return fmt.Errorf("inserting into sessions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing tx: %w", err)
	}

	return nil
}
