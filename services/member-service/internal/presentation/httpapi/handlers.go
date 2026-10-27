package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/application"
	"github.com/selamawitsh/iqub-platform/services/member-service/internal/domain/model"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// MemberUseCases is what the handlers need from the application layer.
// It is declared here, by the consumer, so handlers can be tested with a stub.
type MemberUseCases interface {
	Register(ctx context.Context, in application.RegisterInput) (*model.Member, error)
	Login(ctx context.Context, in application.LoginInput) (*application.LoginResult, error)
	GetProfile(ctx context.Context, id string) (*model.Member, error)
}

var _ MemberUseCases = (*application.MemberService)(nil)

// ReadinessChecker reports whether dependencies (the database) are reachable.
type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	members MemberUseCases
	ready   ReadinessChecker
	logger  *slog.Logger
}

func NewHandler(members MemberUseCases, ready ReadinessChecker, logger *slog.Logger) *Handler {
	return &Handler{members: members, ready: ready, logger: logger}
}

// ---- DTOs ----

type registerRequest struct {
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type loginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type memberResponse struct {
	ID        string    `json:"id"`
	FullName  string    `json:"full_name"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
}

type loginResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// toMemberResponse deliberately has no password hash field (NFR-S1).
func toMemberResponse(m *model.Member) memberResponse {
	return memberResponse{
		ID:        m.ID(),
		FullName:  m.FullName(),
		Phone:     m.Phone().String(),
		CreatedAt: m.CreatedAt(),
	}
}

// ---- Handlers ----

// Register handles POST /api/auth/register (FR-M1).
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	m, err := h.members.Register(r.Context(), application.RegisterInput{
		FullName: req.FullName,
		Phone:    req.Phone,
		Password: req.Password,
	})
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, toMemberResponse(m))
}

// Login handles POST /api/auth/login (FR-M2).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.members.Login(r.Context(), application.LoginInput{
		Phone:    req.Phone,
		Password: req.Password,
	})
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   res.ExpiresAt,
	})
}

// Me handles GET /api/members/me (FR-M3). Identity comes only from the
// X-User-Id header set by the gateway (SRS 6.1); the JWT is never parsed here.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		writeSimpleError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing user identity")
		return
	}
	m, err := h.members.GetProfile(r.Context(), userID)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toMemberResponse(m))
}

// Health handles GET /health: the process is alive (NFR-R4).
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready handles GET /ready: dependencies are reachable (NFR-R4).
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.ready.Ping(ctx); err != nil {
		h.logger.Warn("readiness check failed", slog.String("error", err.Error()))
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ---- helpers ----

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeSimpleError(w, http.StatusBadRequest, "INVALID_REQUEST", "request body is not valid JSON")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
