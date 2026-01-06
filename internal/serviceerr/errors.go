package serviceerr

import (
	"fmt"
	"net/http"
)

type Code string

// Defined by RFC6749
// https://www.rfc-editor.org/rfc/rfc6749.html#section-4.1.2.1
// https://www.rfc-editor.org/rfc/rfc6749.html#section-4.2.2.1
const (
	CodeInvalidRequest          Code = "invalid_request"
	CodeUnauthorizedClient      Code = "unauthorized_client"
	CodeAccessDenied            Code = "access_denied"
	CodeUnsupportedResponseType Code = "unsupported_response_type"
	CodeInvalidScope            Code = "invalid_scope"
	CodeServerError             Code = "server_error"
	CodeTemporarilyUnavailable  Code = "temporarily_unavailable"
)

// Defined by RFC6749
// https://www.rfc-editor.org/rfc/rfc6749.html#section-5.2
const (
	CodeInvalidClient        Code = "invalid_client"
	CodeInvalidGrant         Code = "invalid_grant"
	CodeUnsupportedGrantType Code = "unsupported_grant_type"
)

// Custom defined
const (
	CodeUnknown                Code = "unknown"
	CodeConflict               Code = "conflict"
	CodeNotFound               Code = "not_found"
	CodeFingerprintMismatch    Code = "fingerprint_mismatch"
	CodeStateExpired           Code = "state_expired"
	CodeInvalidOIDCProvider    Code = "invalid_oidc_provider"
	CodeInvalidCSRFToken       Code = "invalid_csrf_token"
	CodeInvalidAtHashToken     Code = "invalid_at_hash_token"
	CodeEndSessionNotSupported Code = "end_session_not_supported"
)

// Defined by RFC6749
// https://www.rfc-editor.org/rfc/rfc6749.html#section-4.1.2.1
// https://www.rfc-editor.org/rfc/rfc6749.html#section-4.2.2.1
var (
	ErrInvalidRequest          = newErr("", CodeInvalidRequest)
	ErrUnauthorizedClient      = newErr("", CodeUnauthorizedClient)
	ErrAccessDenied            = newErr("", CodeAccessDenied)
	ErrUnsupportedResponseType = newErr("", CodeUnsupportedResponseType)
	ErrInvalidScope            = newErr("", CodeInvalidScope)
	ErrServerError             = newErr("", CodeServerError)
	ErrTemporarilyUnavailable  = newErr("", CodeTemporarilyUnavailable)
)

// Defined by RFC6749
// https://www.rfc-editor.org/rfc/rfc6749.html#section-5.2
var (
	ErrInvalidClient        = newErr("", CodeInvalidClient)
	ErrInvalidGrant         = newErr("", CodeInvalidGrant)
	ErrUnsupportedGrantType = newErr("", CodeUnsupportedGrantType)
)

// Custom defined
var (
	ErrUnknown                = newErr("unknown error", CodeUnknown)
	ErrConflict               = newErr("already exists", CodeConflict)
	ErrNotFound               = newErr("not found", CodeNotFound)
	ErrFingerprintMismatch    = newErr("fingerprint mismatch", CodeFingerprintMismatch)
	ErrStateExpired           = newErr("state expired", CodeStateExpired)
	ErrInvalidOIDCProvider    = newErr("invalid OIDC provider", CodeInvalidOIDCProvider)
	ErrInvalidCSRFToken       = newErr("invalid CSRF token", CodeInvalidCSRFToken)
	ErrUnauthorized           = newErr("unauthorized", CodeUnauthorizedClient)
	ErrInvalidAtHash          = newErr("invalid atHash token", CodeInvalidAtHashToken)
	ErrEndSessionNotSupported = newErr("the provider does not support end session", CodeEndSessionNotSupported)
)

//nolint:recvcheck
type Error struct {
	Err         Code   `json:"error"`
	Description string `json:"error_description,omitempty"` // Optional
}

func newErr(description string, c Code) *Error {
	return &Error{
		Err:         c,
		Description: description,
	}
}

func (e *Error) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("%s: %s", e.Err, e.Description)
	}

	return string(e.Err)
}

func (e Error) HTTPStatus() int {
	switch e.Err {
	// RFC6749
	case CodeInvalidRequest:
		return http.StatusBadRequest
	case CodeUnauthorizedClient:
		return http.StatusUnauthorized
	case CodeAccessDenied:
		return http.StatusForbidden
	case CodeUnsupportedResponseType:
		return http.StatusBadRequest
	case CodeInvalidScope:
		return http.StatusBadRequest
	case CodeServerError:
		return http.StatusInternalServerError
	case CodeTemporarilyUnavailable:
		return http.StatusServiceUnavailable
	case CodeInvalidClient:
		return http.StatusBadRequest
	case CodeInvalidGrant:
		return http.StatusBadRequest
	case CodeUnsupportedGrantType:
		return http.StatusBadRequest

	// Custom
	case CodeUnknown:
		return http.StatusInternalServerError
	case CodeConflict:
		return http.StatusConflict
	case CodeNotFound:
		return http.StatusNotFound
	case CodeFingerprintMismatch:
		return http.StatusForbidden
	case CodeStateExpired:
		return http.StatusGone
	case CodeInvalidOIDCProvider:
		return http.StatusPreconditionFailed
	case CodeInvalidAtHashToken:
		return http.StatusUnauthorized
	case CodeEndSessionNotSupported:
		return http.StatusPreconditionFailed
	default:
		return http.StatusInternalServerError
	}
}
