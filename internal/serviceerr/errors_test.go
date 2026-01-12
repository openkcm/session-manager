package serviceerr_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/session-manager/internal/serviceerr"
)

func TestError_Error(t *testing.T) {
	tests := []struct {
		name        string
		err         *serviceerr.Error
		expectedMsg string
	}{
		{
			name:        "Error with description",
			err:         &serviceerr.Error{Err: serviceerr.CodeNotFound, Description: "resource not found"},
			expectedMsg: "not_found: resource not found",
		},
		{
			name:        "Error without description",
			err:         &serviceerr.Error{Err: serviceerr.CodeInvalidRequest, Description: ""},
			expectedMsg: "invalid_request",
		},
		{
			name:        "Predefined error - ErrUnknown",
			err:         serviceerr.ErrUnknown,
			expectedMsg: "unknown: unknown error",
		},
		{
			name:        "Predefined error - ErrNotFound",
			err:         serviceerr.ErrNotFound,
			expectedMsg: "not_found: not found",
		},
		{
			name:        "Predefined error - ErrInvalidRequest",
			err:         serviceerr.ErrInvalidRequest,
			expectedMsg: "invalid_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			assert.Equal(t, tt.expectedMsg, tt.err.Error())
		})
	}
}

func TestError_HTTPStatus(t *testing.T) {
	tests := []struct {
		name               string
		code               serviceerr.Code
		expectedHTTPStatus int
	}{
		// RFC6749 Authorization errors
		{
			name:               "CodeInvalidRequest returns BadRequest",
			code:               serviceerr.CodeInvalidRequest,
			expectedHTTPStatus: http.StatusBadRequest,
		},
		{
			name:               "CodeUnauthorizedClient returns Unauthorized",
			code:               serviceerr.CodeUnauthorizedClient,
			expectedHTTPStatus: http.StatusUnauthorized,
		},
		{
			name:               "CodeAccessDenied returns Forbidden",
			code:               serviceerr.CodeAccessDenied,
			expectedHTTPStatus: http.StatusForbidden,
		},
		{
			name:               "CodeUnsupportedResponseType returns BadRequest",
			code:               serviceerr.CodeUnsupportedResponseType,
			expectedHTTPStatus: http.StatusBadRequest,
		},
		{
			name:               "CodeInvalidScope returns BadRequest",
			code:               serviceerr.CodeInvalidScope,
			expectedHTTPStatus: http.StatusBadRequest,
		},
		{
			name:               "CodeServerError returns InternalServerError",
			code:               serviceerr.CodeServerError,
			expectedHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:               "CodeTemporarilyUnavailable returns ServiceUnavailable",
			code:               serviceerr.CodeTemporarilyUnavailable,
			expectedHTTPStatus: http.StatusServiceUnavailable,
		},

		// RFC6749 Token errors
		{
			name:               "CodeInvalidClient returns BadRequest",
			code:               serviceerr.CodeInvalidClient,
			expectedHTTPStatus: http.StatusBadRequest,
		},
		{
			name:               "CodeInvalidGrant returns BadRequest",
			code:               serviceerr.CodeInvalidGrant,
			expectedHTTPStatus: http.StatusBadRequest,
		},
		{
			name:               "CodeUnsupportedGrantType returns BadRequest",
			code:               serviceerr.CodeUnsupportedGrantType,
			expectedHTTPStatus: http.StatusBadRequest,
		},

		// Custom codes
		{
			name:               "CodeUnknown returns InternalServerError",
			code:               serviceerr.CodeUnknown,
			expectedHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:               "CodeConflict returns Conflict",
			code:               serviceerr.CodeConflict,
			expectedHTTPStatus: http.StatusConflict,
		},
		{
			name:               "CodeNotFound returns NotFound",
			code:               serviceerr.CodeNotFound,
			expectedHTTPStatus: http.StatusNotFound,
		},
		{
			name:               "CodeFingerprintMismatch returns Forbidden",
			code:               serviceerr.CodeFingerprintMismatch,
			expectedHTTPStatus: http.StatusForbidden,
		},
		{
			name:               "CodeStateExpired returns Gone",
			code:               serviceerr.CodeStateExpired,
			expectedHTTPStatus: http.StatusGone,
		},
		{
			name:               "CodeInvalidOIDCProvider returns PreconditionFailed",
			code:               serviceerr.CodeInvalidOIDCProvider,
			expectedHTTPStatus: http.StatusPreconditionFailed,
		},
		{
			name:               "CodeInvalidCSRFToken returns InternalServerError by default",
			code:               serviceerr.CodeInvalidCSRFToken,
			expectedHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:               "CodeInvalidAtHashToken returns Unauthorized",
			code:               serviceerr.CodeInvalidAtHashToken,
			expectedHTTPStatus: http.StatusUnauthorized,
		},
		{
			name:               "CodeEndSessionNotSupported returns PreconditionFailed",
			code:               serviceerr.CodeEndSessionNotSupported,
			expectedHTTPStatus: http.StatusPreconditionFailed,
		},
		{
			name:               "Unknown code returns InternalServerError",
			code:               serviceerr.Code("unknown_code"),
			expectedHTTPStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			err := serviceerr.Error{Err: tt.code}
			assert.Equal(t, tt.expectedHTTPStatus, err.HTTPStatus())
		})
	}
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         *serviceerr.Error
		expectedErr serviceerr.Code
		hasDesc     bool
	}{
		// RFC6749 Authorization errors
		{name: "ErrInvalidRequest", err: serviceerr.ErrInvalidRequest, expectedErr: serviceerr.CodeInvalidRequest, hasDesc: false},
		{name: "ErrUnauthorizedClient", err: serviceerr.ErrUnauthorizedClient, expectedErr: serviceerr.CodeUnauthorizedClient, hasDesc: false},
		{name: "ErrAccessDenied", err: serviceerr.ErrAccessDenied, expectedErr: serviceerr.CodeAccessDenied, hasDesc: false},
		{name: "ErrUnsupportedResponseType", err: serviceerr.ErrUnsupportedResponseType, expectedErr: serviceerr.CodeUnsupportedResponseType, hasDesc: false},
		{name: "ErrInvalidScope", err: serviceerr.ErrInvalidScope, expectedErr: serviceerr.CodeInvalidScope, hasDesc: false},
		{name: "ErrServerError", err: serviceerr.ErrServerError, expectedErr: serviceerr.CodeServerError, hasDesc: false},
		{name: "ErrTemporarilyUnavailable", err: serviceerr.ErrTemporarilyUnavailable, expectedErr: serviceerr.CodeTemporarilyUnavailable, hasDesc: false},

		// RFC6749 Token errors
		{name: "ErrInvalidClient", err: serviceerr.ErrInvalidClient, expectedErr: serviceerr.CodeInvalidClient, hasDesc: false},
		{name: "ErrInvalidGrant", err: serviceerr.ErrInvalidGrant, expectedErr: serviceerr.CodeInvalidGrant, hasDesc: false},
		{name: "ErrUnsupportedGrantType", err: serviceerr.ErrUnsupportedGrantType, expectedErr: serviceerr.CodeUnsupportedGrantType, hasDesc: false},

		// Custom errors
		{name: "ErrUnknown", err: serviceerr.ErrUnknown, expectedErr: serviceerr.CodeUnknown, hasDesc: true},
		{name: "ErrConflict", err: serviceerr.ErrConflict, expectedErr: serviceerr.CodeConflict, hasDesc: true},
		{name: "ErrNotFound", err: serviceerr.ErrNotFound, expectedErr: serviceerr.CodeNotFound, hasDesc: true},
		{name: "ErrFingerprintMismatch", err: serviceerr.ErrFingerprintMismatch, expectedErr: serviceerr.CodeFingerprintMismatch, hasDesc: true},
		{name: "ErrStateExpired", err: serviceerr.ErrStateExpired, expectedErr: serviceerr.CodeStateExpired, hasDesc: true},
		{name: "ErrInvalidOIDCProvider", err: serviceerr.ErrInvalidOIDCProvider, expectedErr: serviceerr.CodeInvalidOIDCProvider, hasDesc: true},
		{name: "ErrInvalidCSRFToken", err: serviceerr.ErrInvalidCSRFToken, expectedErr: serviceerr.CodeInvalidCSRFToken, hasDesc: true},
		{name: "ErrUnauthorized", err: serviceerr.ErrUnauthorized, expectedErr: serviceerr.CodeUnauthorizedClient, hasDesc: true},
		{name: "ErrInvalidAtHash", err: serviceerr.ErrInvalidAtHash, expectedErr: serviceerr.CodeInvalidAtHashToken, hasDesc: true},
		{name: "ErrEndSessionNotSupported", err: serviceerr.ErrEndSessionNotSupported, expectedErr: serviceerr.CodeEndSessionNotSupported, hasDesc: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			assert.NotNil(t, tt.err)
			assert.Equal(t, tt.expectedErr, tt.err.Err)
			if tt.hasDesc {
				assert.NotEmpty(t, tt.err.Description)
			} else {
				assert.Empty(t, tt.err.Description)
			}
			// Ensure Error() method works
			assert.NotEmpty(t, tt.err.Error())
			// Ensure HTTPStatus() returns valid status
			assert.NotZero(t, tt.err.HTTPStatus())
		})
	}
}

func TestErrorCodes(t *testing.T) {
	// Test that all code constants are defined correctly
	codes := []struct {
		name     string
		code     serviceerr.Code
		expected string
	}{
		// RFC6749 codes
		{name: "CodeInvalidRequest", code: serviceerr.CodeInvalidRequest, expected: "invalid_request"},
		{name: "CodeUnauthorizedClient", code: serviceerr.CodeUnauthorizedClient, expected: "unauthorized_client"},
		{name: "CodeAccessDenied", code: serviceerr.CodeAccessDenied, expected: "access_denied"},
		{name: "CodeUnsupportedResponseType", code: serviceerr.CodeUnsupportedResponseType, expected: "unsupported_response_type"},
		{name: "CodeInvalidScope", code: serviceerr.CodeInvalidScope, expected: "invalid_scope"},
		{name: "CodeServerError", code: serviceerr.CodeServerError, expected: "server_error"},
		{name: "CodeTemporarilyUnavailable", code: serviceerr.CodeTemporarilyUnavailable, expected: "temporarily_unavailable"},
		{name: "CodeInvalidClient", code: serviceerr.CodeInvalidClient, expected: "invalid_client"},
		{name: "CodeInvalidGrant", code: serviceerr.CodeInvalidGrant, expected: "invalid_grant"},
		{name: "CodeUnsupportedGrantType", code: serviceerr.CodeUnsupportedGrantType, expected: "unsupported_grant_type"},

		// Custom codes
		{name: "CodeUnknown", code: serviceerr.CodeUnknown, expected: "unknown"},
		{name: "CodeConflict", code: serviceerr.CodeConflict, expected: "conflict"},
		{name: "CodeNotFound", code: serviceerr.CodeNotFound, expected: "not_found"},
		{name: "CodeFingerprintMismatch", code: serviceerr.CodeFingerprintMismatch, expected: "fingerprint_mismatch"},
		{name: "CodeStateExpired", code: serviceerr.CodeStateExpired, expected: "state_expired"},
		{name: "CodeInvalidOIDCProvider", code: serviceerr.CodeInvalidOIDCProvider, expected: "invalid_oidc_provider"},
		{name: "CodeInvalidCSRFToken", code: serviceerr.CodeInvalidCSRFToken, expected: "invalid_csrf_token"},
		{name: "CodeInvalidAtHashToken", code: serviceerr.CodeInvalidAtHashToken, expected: "invalid_at_hash_token"},
		{name: "CodeEndSessionNotSupported", code: serviceerr.CodeEndSessionNotSupported, expected: "end_session_not_supported"},
	}

	for _, tc := range codes {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			assert.Equal(t, tc.expected, string(tc.code))
		})
	}
}
