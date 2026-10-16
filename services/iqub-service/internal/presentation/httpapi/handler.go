package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/application"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
)

// iqubUseCases lists only what this handler needs. Go interfaces are defined
// by the code that uses them. *application.IqubService satisfies it.
type iqubUseCases interface {
	CreateIqub(ctx context.Context, cmd application.CreateIqubCommand) (model.Iqub, error)
	ListMyIqubs(ctx context.Context, memberID string) ([]model.Iqub, error)
}

type Handler struct {
	service iqubUseCases
}

func NewHandler(service iqubUseCases) *Handler {
	return &Handler{service: service}
}

// memberIDFrom reads the caller's identity. For now it trusts the X-User-Id
// header. This is only safe once the gateway is the sole entry point
// (Milestone 4), because until then any client can claim any identity.
func memberIDFrom(r *http.Request) (string, bool) {
	id := r.Header.Get("X-User-Id")
	return id, id != ""
}

func (h *Handler) createIqub(w http.ResponseWriter, r *http.Request) {
	memberID, ok := memberIDFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "missing X-User-Id header"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // limit body to 1 MB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createIqubRequest
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
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
	memberID, ok := memberIDFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "missing X-User-Id header"})
		return
	}

	iqubs, err := h.service.ListMyIqubs(r.Context(), memberID)
	if err != nil {
		writeError(w, err)
		return
	}

	out := make([]iqubResponse, 0, len(iqubs))
	for _, i := range iqubs {
		out = append(out, toResponse(i))
	}
	writeJSON(w, http.StatusOK, out)
}
