package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/domain/model"
)

// errorBody is the single error shape from SRS section 6.1.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// domainErrors maps domain errors to HTTP status codes and stable error codes.
var domainErrors = []struct {
	target error
	status int
	code   string
}{
	{model.ErrInvalidPhone, http.StatusBadRequest, "INVALID_PHONE"},
	{model.ErrInvalidFullName, http.StatusBadRequest, "INVALID_FULL_NAME"},
	{model.ErrWeakPassword, http.StatusBadRequest, "WEAK_PASSWORD"},
	{model.ErrInvalidCredentials, http.StatusUnauthorized, "INVALID_CREDENTIALS"},
	{model.ErrMemberNotFound, http.StatusNotFound, "MEMBER_NOT_FOUND"},
	{model.ErrPhoneTaken, http.StatusConflict, "PHONE_TAKEN"},
}

func writeError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	for _, e := range domainErrors {
		if errors.Is(err, e.target) {
			writeJSON(w, e.status, errorBody{errorDetail{Code: e.code, Message: err.Error()}})
			return
		}
	}

	// Unknown error: log the detail, never leak it to the client.
	logger.Error("unhandled error",
		slog.String("request_id", RequestIDFrom(r.Context())),
		slog.String("error", err.Error()),
	)
	writeJSON(w, http.StatusInternalServerError,
		errorBody{errorDetail{Code: "INTERNAL_ERROR", Message: "internal server error"}})
}

func writeSimpleError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{errorDetail{Code: code, Message: message}})
}
