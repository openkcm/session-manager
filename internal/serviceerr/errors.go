package serviceerr

import "net/http"

type Code int

const (
	CodeUnknown Code = iota
	CodeConflict
	CodeNotFound
	CodeFingerprintMismatch
	CodeStateExpired
	CodeInvalidOIDCProvider
)

var ErrUnknown = newErr("unknown error", CodeUnknown)
var ErrConflict = newErr("already exists", CodeConflict)
var ErrNotFound = newErr("not found", CodeNotFound)
var ErrFingerprintMismatch = newErr("fingerprint mismatch", CodeFingerprintMismatch)
var ErrStateExpired = newErr("state expired", CodeStateExpired)
var ErrInvalidOIDCProvider = newErr("invalid OIDC provider", CodeInvalidOIDCProvider)

//nolint:recvcheck
type Error struct {
	Message string
	Code    Code
}

func newErr(msg string, code Code) *Error {
	return &Error{
		Message: msg,
		Code:    code,
	}
}

func (e *Error) Error() string {
	return e.Message
}

func (e Error) HTTPStatus() int {
	switch e.Code {
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
	default:
		return http.StatusInternalServerError
	}
}
