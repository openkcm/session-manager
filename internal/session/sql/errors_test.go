package sessionsql

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/serviceerr"
)

type errorAssertionFunc func(t *testing.T, err error, msgAndArgs ...any) bool

func errIs(target error) errorAssertionFunc {
	return func(t *testing.T, err error, msgAndArgs ...any) bool {
		t.Helper()
		return assert.ErrorIs(t, err, target, msgAndArgs...)
	}
}

var errUnknown = errors.New("unknown error")

func Test_handlePgError(t *testing.T) {
	tests := []struct {
		name      string
		inputErr  error
		errAssert errorAssertionFunc
		wantOk    bool
	}{
		{
			name:      "23505 error",
			inputErr:  &pgconn.PgError{Code: "23505"},
			errAssert: errIs(serviceerr.ErrConflict),
			wantOk:    true,
		},
		{
			name:      "Unknown error",
			inputErr:  errUnknown,
			errAssert: errIs(errUnknown),
			wantOk:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr, ok := handlePgError(tt.inputErr)
			if !tt.errAssert(t, gotErr, fmt.Sprintf("handlePgError() error %v", gotErr)) {
				return
			}

			if !assert.Equal(t, tt.wantOk, ok, "handlePgError() OK = %v, want = %v", ok, tt.wantOk) {
				return
			}
		})
	}
}
