package trustsql

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/openkcm/session-manager/internal/serviceerr"
)

func handlePgError(err error) (error, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return serviceerr.ErrConflict, true
	}

	return err, false
}
