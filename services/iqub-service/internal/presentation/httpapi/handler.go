package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/application"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
)

type iqubUseCases interface {
	CreateIqub(context.Context, application.CreateIqubCommand) (model.Iqub, error)
	ListMyIqubs(context.Context, string) ([]model.Iqub, error)
	GetIqub(context.Context, string, string) (model.Iqub, error)
	JoinIqub(context.Context, application.JoinIqubCommand) (model.Iqub, error)
	StartIqub(context.Context, application.StartIqubCommand) (model.Iqub, []model.Round, error)
	GetSchedule(context.Context, string, string) ([]model.Round, error)
	GetLedger(context.Context, string, string) ([]model.LedgerEntry, error)
}

type contributionUseCases interface {
	RecordContribution(context.Context, application.RecordContributionCommand) (model.Contribution, error)
	ConfirmContribution(context.Context, application.ConfirmContributionCommand) (application.ConfirmResult, error)
}

type Handler struct {
	service       iqubUseCases
	contributions contributionUseCases
}

func NewHandler(service iqubUseCases, contributions contributionUseCases) *Handler {
	return &Handler{
		service:       service,
		contributions: contributions,
	}
}

// For now, identity comes from X-User-Id. This is not authentication:
// clients can choose this header unless a trusted gateway protects the service.
func memberIDFrom(r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.Header.Get("X-User-Id"))
	return id, id != ""
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return false
	}
	return true
}

func requireMember(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := memberIDFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "missing X-User-Id header"})
	}
	return id, ok
}

func (h *Handler) createIqub(w http.ResponseWriter, r *http.Request) {
	memberID, ok := requireMember(w, r)
	if !ok {
		return
	}

	var req createIqubRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	iqub, err := h.service.CreateIqub(r.Context(), application.CreateIqubCommand{
		AdminID:            memberID,
		Name:               req.Name,
		ContributionAmount: req.ContributionAmount,
		Frequency:          model.Frequency(req.Frequency),
		MaxMembers:         req.MaxMembers,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toResponse(iqub))
}

func (h *Handler) listIqubs(w http.ResponseWriter, r *http.Request) {
	memberID, ok := requireMember(w, r)
	if !ok {
		return
	}

	iqubs, err := h.service.ListMyIqubs(r.Context(), memberID)
	if err != nil {
		writeError(w, err)
		return
	}

	out := make([]iqubResponse, 0, len(iqubs))
	for _, iqub := range iqubs {
		out = append(out, toResponse(iqub))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getIqub(w http.ResponseWriter, r *http.Request) {
	memberID, ok := requireMember(w, r)
	if !ok {
		return
	}

	iqub, err := h.service.GetIqub(r.Context(), r.PathValue("id"), memberID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(iqub))
}

func (h *Handler) joinIqub(w http.ResponseWriter, r *http.Request) {
	memberID, ok := requireMember(w, r)
	if !ok {
		return
	}

	var req joinIqubRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	iqub, err := h.service.JoinIqub(r.Context(), application.JoinIqubCommand{
		MemberID:   memberID,
		InviteCode: req.InviteCode,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(iqub))
}

func (h *Handler) startIqub(w http.ResponseWriter, r *http.Request) {
	memberID, ok := requireMember(w, r)
	if !ok {
		return
	}

	iqub, rounds, err := h.service.StartIqub(r.Context(), application.StartIqubCommand{
		IqubID:   r.PathValue("id"),
		CallerID: memberID,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	out := make([]roundResponse, 0, len(rounds))
	for _, round := range rounds {
		out = append(out, toRoundResponse(round))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"iqub":   toResponse(iqub),
		"rounds": out,
	})
}

func (h *Handler) getSchedule(w http.ResponseWriter, r *http.Request) {
	memberID, ok := requireMember(w, r)
	if !ok {
		return
	}

	rounds, err := h.service.GetSchedule(r.Context(), r.PathValue("id"), memberID)
	if err != nil {
		writeError(w, err)
		return
	}

	out := make([]roundResponse, 0, len(rounds))
	for _, round := range rounds {
		out = append(out, toRoundResponse(round))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) recordContribution(w http.ResponseWriter, r *http.Request) {
	memberID, ok := requireMember(w, r)
	if !ok {
		return
	}

	contribution, err := h.contributions.RecordContribution(r.Context(), application.RecordContributionCommand{
		IqubID:   r.PathValue("id"),
		MemberID: memberID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toContributionResponse(contribution))
}

func (h *Handler) confirmContribution(w http.ResponseWriter, r *http.Request) {
	memberID, ok := requireMember(w, r)
	if !ok {
		return
	}

	result, err := h.contributions.ConfirmContribution(r.Context(), application.ConfirmContributionCommand{
		IqubID:         r.PathValue("id"),
		ContributionID: r.PathValue("cid"),
		CallerID:       memberID,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, confirmResponse{
		Contribution:   toContributionResponse(result.Contribution),
		RoundCompleted: result.RoundCompleted,
		RecipientID:    result.RecipientID,
		PayoutAmount:   result.PayoutAmount,
		GroupCompleted: result.GroupCompleted,
	})
}

func (h *Handler) getLedger(w http.ResponseWriter, r *http.Request) {
	memberID, ok := requireMember(w, r)
	if !ok {
		return
	}

	entries, err := h.service.GetLedger(r.Context(), r.PathValue("id"), memberID)
	if err != nil {
		writeError(w, err)
		return
	}

	out := make([]ledgerEntryResponse, 0, len(entries))
	for _, entry := range entries {
		out = append(out, toLedgerResponse(entry))
	}
	writeJSON(w, http.StatusOK, out)
}
