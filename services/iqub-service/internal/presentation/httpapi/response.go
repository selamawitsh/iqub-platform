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
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	ContributionAmount int64     `json:"contribution_amount"`
	Frequency          string    `json:"frequency"`
	MaxMembers         int       `json:"max_members"`
	Status             string    `json:"status"`
	AdminID            string    `json:"admin_id"`
	MemberIDs          []string  `json:"member_ids"`
	CreatedAt          time.Time `json:"created_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func toResponse(i model.Iqub) iqubResponse {
	return iqubResponse{
		ID:                 i.ID,
		Name:               i.Name,
		ContributionAmount: i.ContributionAmount,
		Frequency:          string(i.Frequency),
		MaxMembers:         i.MaxMembers,
		Status:             string(i.Status),
		AdminID:            i.AdminID,
		MemberIDs:          i.MemberIDs,
		CreatedAt:          i.CreatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Headers are already sent, so a failed write cannot be reported to the client.
	_ = json.NewEncoder(w).Encode(data)
}

// writeError translates errors from inner layers into HTTP status codes.
// This translation belongs here, in the presentation layer, and nowhere deeper.
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrValidation):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	default:
		// Log the real cause for us, but never leak internals to the client.
		log.Printf("internal error: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}
