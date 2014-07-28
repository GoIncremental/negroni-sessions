package sessions

import (
	"errors"
)

var (
	ErrInvalidId       = errors.New("session: invalid session id")
	ErrInvalidModified = errors.New("mongostore: invalid modified value")
)
