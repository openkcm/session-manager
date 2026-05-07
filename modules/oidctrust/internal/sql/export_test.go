package sqltrust

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// PgTextOrNull exposes pgTextOrNull for testing.
func PgTextOrNull(s string) pgtype.Text {
	return pgTextOrNull(s)
}

// HandlePgError exposes handlePgError for testing.
func HandlePgError(err error) (error, bool) {
	return handlePgError(err)
}
