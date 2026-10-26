package application

import "time"

const (
	EventMemberRegistered = "member.registered"
	EventVersion          = 1
)

// Event is the common envelope for every event (SRS section 5.2).
type Event struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	OccurredAt time.Time `json:"occurred_at"`
	Version    int       `json:"version"`
	Data       any       `json:"data"`
}

// MemberRegisteredData is the data of a member.registered event.
type MemberRegisteredData struct {
	MemberID string `json:"member_id"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
}
