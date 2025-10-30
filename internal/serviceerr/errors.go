package serviceerr

import "net/http"

type Code int

const (
	CodeUnknown Code = iota
	CodeConflict
	CodeNotFound
	CodeFingerprintMismatch
	CodeStateExpired
)

var ErrUnknown = newErr("unknown error", CodeUnknown)
var ErrConflict = newErr("already exists", CodeConflict)
var ErrNotFound = newErr("not found", CodeNotFound)
var ErrFingerprintMismatch = newErr("fingerprint mismatch", CodeFingerprintMismatch)
var ErrStateExpired = newErr("state expired", CodeStateExpired)

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
	default:
		return http.StatusInternalServerError
	}
}
