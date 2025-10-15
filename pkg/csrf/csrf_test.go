package csrf_test

import (
	"testing"

	"github.com/openkcm/session-manager/pkg/csrf"
	"github.com/stretchr/testify/assert"
)

func TestCSRF(t *testing.T) {
	tests := []struct {
		name              string
		genKey            string // Key used to generate the CSRF token
		genSessionID      string // Session ID used to generate the CSRF token
		validateKey       string // Key used to validate the token
		validateSessionID string // Session ID used to validate the token
		wantValid         bool
	}{
		{
			name:              "Validate a token successfuly",
			genKey:            "my-super-secret-key",
			genSessionID:      "some-session-id",
			validateKey:       "my-super-secret-key",
			validateSessionID: "some-session-id",
			wantValid:         true,
		},
		{
			name:              "Mismatched Session ID. Token is invalid",
			genKey:            "my-super-secret-key",
			genSessionID:      "some-session-id",
			validateKey:       "my-super-secret-key",
			validateSessionID: "mismatched-session-id",
			wantValid:         false,
		},
		{
			name:              "Mismatched key. Token is invalid",
			genKey:            "my-super-secret-key",
			genSessionID:      "some-session-id",
			validateKey:       "mismatched-key",
			validateSessionID: "some-session-id",
			wantValid:         false,
		},
		{
			name:              "Mismatched Session ID and key. Token is invalid",
			genKey:            "my-super-secret-key",
			genSessionID:      "some-session-id",
			validateKey:       "mismatched-key",
			validateSessionID: "mismatched-session-id",
			wantValid:         false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token := csrf.NewToken(tc.genSessionID, []byte(tc.genKey))
			valid := csrf.Validate(token, tc.validateSessionID, []byte(tc.validateKey))
			assert.Equal(t, tc.wantValid, valid, "Failed to validate the CSRF token")
		})
	}

}
