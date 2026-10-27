package security

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/application"
)

// JWTIssuer implements application.TokenIssuer with HS256 (ADR-006, NFR-S2).
// The member id goes in the standard "sub" claim.
type JWTIssuer struct {
	secret []byte
	ttl    time.Duration
}

var _ application.TokenIssuer = (*JWTIssuer)(nil)

func NewJWTIssuer(secret string, ttl time.Duration) *JWTIssuer {
	return &JWTIssuer{secret: []byte(secret), ttl: ttl}
}

func (i *JWTIssuer) Issue(memberID string, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(i.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   memberID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}
