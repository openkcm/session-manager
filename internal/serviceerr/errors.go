package serviceerr

import "errors"

var ErrConflict = errors.New("already exists")
var ErrNotFound = errors.New("not found")
var ErrFingerprintMismatch = errors.New("fingerprint mismatch")
var ErrStateExpired = errors.New("state expired")
