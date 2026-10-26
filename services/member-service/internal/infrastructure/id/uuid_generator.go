package id

import (
	"github.com/google/uuid"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/application"
)

// UUIDGenerator implements application.IDGenerator.
type UUIDGenerator struct{}

var _ application.IDGenerator = UUIDGenerator{}

func (UUIDGenerator) NewID() string { return uuid.NewString() }
