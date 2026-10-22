package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
)

type iqubResponse struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	ContributionAmount int64      `json:"contribution_amount"`
	Frequency          string     `json:"frequency"`
	MaxMembers         int        `json:"max_members"`
	Status             string     `json:"status"`
	AdminID            string     `json:"admin_id"`
	MemberIDs          []string   `json:"member_ids"`
	InviteCode         string     `json:"invite_code"`
	CreatedAt          time.Time  `json:"created_at"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
}

type roundResponse struct {
	ID          string    `json:"id"`
	IqubID      string    `json:"iqub_id"`
	Number      int       `json:"number"`
	DueDate     time.Time `json:"due_date"`
	Status      string    `json:"status"`
	RecipientID string    `json:"recipient_id"`
}

type contributionResponse struct {
	ID          string    `json:"id"`
	IqubID      string    `json:"iqub_id"`
	RoundID     string    `json:"round_id"`
	MemberID    string    `json:"member_id"`
	Amount      int64     `json:"amount"`
	Status      string    `json:"status"`
	RecordedAt  time.Time `json:"recorded_at"`
	ConfirmedAt time.Time `json:"confirmed_at,omitempty"`
}

type ledgerEntryResponse struct {
	ID        string    `json:"id"`
	IqubID    string    `json:"iqub_id"`
	RoundID   string    `json:"round_id"`
	MemberID  string    `json:"member_id"`
	Type      string    `json:"type"`
	Amount    int64     `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

type confirmResponse struct {
	Contribution   contributionResponse `json:"contribution"`
	RoundCompleted bool                 `json:"round_completed"`
	RecipientID    string               `json:"recipient_id,omitempty"`
	PayoutAmount   int64                `json:"payout_amount,omitempty"`
	GroupCompleted bool                 `json:"group_completed"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func toResponse(i model.Iqub) iqubResponse {
	out := iqubResponse{
		ID:                 i.ID,
		Name:               i.Name,
		ContributionAmount: i.ContributionAmount,
		Frequency:          string(i.Frequency),
		MaxMembers:         i.MaxMembers,
		Status:             string(i.Status),
		AdminID:            i.AdminID,
		MemberIDs:          i.MemberIDs,
		InviteCode:         i.InviteCode,
		CreatedAt:          i.CreatedAt,
	}
	if !i.StartedAt.IsZero() {
		started := i.StartedAt
		out.StartedAt = &started
	}
	return out
}

func toRoundResponse(r model.Round) roundResponse {
	return roundResponse{
		ID:          r.ID,
		IqubID:      r.IqubID,
		Number:      r.Number,
		DueDate:     r.DueDate,
		Status:      string(r.Status),
		RecipientID: r.RecipientID,
	}
}

func toContributionResponse(c model.Contribution) contributionResponse {
	return contributionResponse{
		ID:          c.ID,
		IqubID:      c.IqubID,
		RoundID:     c.RoundID,
		MemberID:    c.MemberID,
		Amount:      c.Amount,
		Status:      string(c.Status),
		RecordedAt:  c.RecordedAt,
		ConfirmedAt: c.ConfirmedAt,
	}
}

func toLedgerResponse(e model.LedgerEntry) ledgerEntryResponse {
	return ledgerEntryResponse{
		ID:        e.ID,
		IqubID:    e.IqubID,
		RoundID:   e.RoundID,
		MemberID:  e.MemberID,
		Type:      string(e.Type),
		Amount:    e.Amount,
		CreatedAt: e.CreatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrValidation):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	case errors.Is(err, model.ErrForbidden):
		writeJSON(w, http.StatusForbidden, errorResponse{Error: err.Error()})
	case errors.Is(err, model.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "resource not found"})
	case errors.Is(err, model.ErrConflict):
		writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
	default:
		log.Printf("internal error: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}
