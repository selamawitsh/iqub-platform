package model

import "strings"

// Phone is an Ethiopian mobile number in canonical form: +251 followed by
// 9 digits, the first being 9 (Ethio Telecom) or 7 (Safaricom Ethiopia).
type Phone string

var phoneCleaner = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "")

// NewPhone accepts 09XXXXXXXX, 07XXXXXXXX, 9XXXXXXXX, 2519XXXXXXXX and
// +2519XXXXXXXX (spaces, dashes and brackets are ignored) and returns the
// canonical +251 form.
func NewPhone(raw string) (Phone, error) {
	s := phoneCleaner.Replace(strings.TrimSpace(raw))

	switch {
	case strings.HasPrefix(s, "+251"):
		s = s[4:]
	case strings.HasPrefix(s, "251"):
		s = s[3:]
	case strings.HasPrefix(s, "0"):
		s = s[1:]
	}

	if len(s) != 9 || (s[0] != '9' && s[0] != '7') {
		return "", ErrInvalidPhone
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return "", ErrInvalidPhone
		}
	}

	return Phone("+251" + s), nil
}

func (p Phone) String() string { return string(p) }
