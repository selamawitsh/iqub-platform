package model

import (
	"errors"
	"fmt"
)

// The four kinds of business error. Outer layers use errors.Is to decide
// the HTTP status: validation 400, forbidden 403, not found 404, conflict 409.
var (
	ErrValidation = errors.New("validation error")
	ErrConflict   = errors.New("conflict")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
)

func validation(msg string) error {
	return fmt.Errorf("%w: %s", ErrValidation, msg)
}

func conflict(msg string) error {
	return fmt.Errorf("%w: %s", ErrConflict, msg)
}

func forbidden(msg string) error {
	return fmt.Errorf("%w: %s", ErrForbidden, msg)
}
