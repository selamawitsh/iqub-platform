package model

const (
	MinPasswordLen = 8
	// bcrypt only uses the first 72 bytes, so longer passwords are rejected
	// instead of being silently truncated.
	MaxPasswordBytes = 72
)

// ValidatePassword checks the plaintext password against the policy.
// It runs before hashing; the Member entity never sees the plaintext.
func ValidatePassword(plain string) error {
	if len(plain) < MinPasswordLen || len(plain) > MaxPasswordBytes {
		return ErrWeakPassword
	}
	return nil
}
