package application

import (
	"context"
	"time"
)

// PasswordHasher hashes and verifies passwords (bcrypt in infrastructure).
type PasswordHasher interface {
	Hash(plain string) (string, error)
	// Compare returns nil when plain matches hash, and an error otherwise.
	Compare(hash, plain string) error
}

// TokenIssuer creates signed access tokens (JWT HS256 in infrastructure).
type TokenIssuer interface {
	Issue(memberID string, now time.Time) (token string, expiresAt time.Time, err error)
}

// IDGenerator produces unique ids (UUIDs in infrastructure).
type IDGenerator interface {
	NewID() string
}

// Clock returns the current time.
type Clock interface {
	Now() time.Time
}

// EventPublisher publishes domain events (RabbitMQ in infrastructure).
type EventPublisher interface {
	Publish(ctx context.Context, e Event) error
}
