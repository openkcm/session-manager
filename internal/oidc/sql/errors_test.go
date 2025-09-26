package oidcsql

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/serviceerr"
)

var errUnknown = errors.New("unknown error")

func Test_handlePgError(t *testing.T) {
	tests := []struct {
		name      string
		inputErr  error
		errTarget error
		wantOk    bool
	}{
		{
			name:      "23505 error",
			inputErr:  &pgconn.PgError{Code: "23505"},
			errTarget: serviceerr.ErrConflict,
			wantOk:    true,
		},
		{
			name:      "Unknown error",
			inputErr:  errUnknown,
			errTarget: errUnknown,
			wantOk:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr, ok := handlePgError(tt.inputErr)
			if !assert.ErrorIsf(t, gotErr, tt.errTarget, "handlePgError() error %v", gotErr) {
				return
			}

			if !assert.Equalf(t, tt.wantOk, ok, "handlePgError() OK = %v, want = %v", ok, tt.wantOk) {
				return
			}
		})
	}
}
