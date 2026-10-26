package clock

import (
	"time"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/application"
)

// SystemClock implements application.Clock.
type SystemClock struct{}

var _ application.Clock = SystemClock{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
