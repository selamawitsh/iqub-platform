package security

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/application"
)

const DefaultBcryptCost = bcrypt.DefaultCost

// BcryptHasher implements application.PasswordHasher (NFR-S1).
type BcryptHasher struct {
	cost int
}

var _ application.PasswordHasher = (*BcryptHasher)(nil)

func NewBcryptHasher(cost int) *BcryptHasher {
	return &BcryptHasher{cost: cost}
}

func (h *BcryptHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (h *BcryptHasher) Compare(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
