package serviceerr

import "errors"

var ErrConflict = errors.New("already exists")
var ErrNotFound = errors.New("not found")
