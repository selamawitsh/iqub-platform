package httpapi

import "net/http"

func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/iqubs", h.createIqub)
	mux.HandleFunc("GET /api/iqubs", h.listIqubs)
	mux.HandleFunc("POST /api/iqubs/join", h.joinIqub)

	mux.HandleFunc("GET /api/iqubs/{id}", h.getIqub)
	mux.HandleFunc("POST /api/iqubs/{id}/start", h.startIqub)
	mux.HandleFunc("GET /api/iqubs/{id}/schedule", h.getSchedule)
	mux.HandleFunc("POST /api/iqubs/{id}/contributions", h.recordContribution)
	mux.HandleFunc("POST /api/iqubs/{id}/contributions/{cid}/confirm", h.confirmContribution)
	mux.HandleFunc("GET /api/iqubs/{id}/ledger", h.getLedger)

	return mux
}
