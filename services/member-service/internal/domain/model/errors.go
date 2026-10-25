package model

import "errors"

var (
	ErrInvalidID           = errors.New("invalid member id")
	ErrInvalidFullName     = errors.New("full name must be 2 to 100 characters")
	ErrInvalidPhone        = errors.New("invalid Ethiopian mobile phone number")
	ErrInvalidPasswordHash = errors.New("password hash must not be empty")
	ErrWeakPassword        = errors.New("password must be 8 to 72 bytes long")

	ErrPhoneTaken         = errors.New("phone number already registered")
	ErrMemberNotFound     = errors.New("member not found")
	ErrInvalidCredentials = errors.New("invalid phone or password")
)
