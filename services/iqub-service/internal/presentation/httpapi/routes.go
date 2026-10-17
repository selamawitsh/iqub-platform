package httpapi

import "net/http"

// NewRouter maps URLs to handlers. Go 1.22 lets us put the HTTP method in
// the pattern, and answers 405 automatically for the wrong method.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/iqubs", h.createIqub)
	mux.HandleFunc("GET /api/iqubs", h.listIqubs)

	return mux
}
