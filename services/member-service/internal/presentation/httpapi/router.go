package httpapi

import (
	"log/slog"
	"net/http"
)

// NewRouter wires routes (SRS section 6) and middleware.
// Go 1.22 method+path patterns are used, so no router library is needed.
func NewRouter(h *Handler, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("GET /api/members/me", h.Me)
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /ready", h.Ready)

	return RequestID(Logging(logger)(mux))
}
