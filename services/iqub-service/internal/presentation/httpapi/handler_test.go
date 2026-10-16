package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/application"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/infrastructure/memory"
)

const validBody = `{"name":"Family Iqub","contribution_amount":50000,"frequency":"MONTHLY","max_members":10}`

func newTestRouter() http.Handler {
	service := application.NewIqubService(memory.NewIqubRepository())
	return NewRouter(NewHandler(service))
}

func call(router http.Handler, method, path, body, userID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID != "" {
		req.Header.Set("X-User-Id", userID)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHealthOK(t *testing.T) {
	rec := call(newTestRouter(), http.MethodGet, "/health", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHealthRejectsPost(t *testing.T) {
	rec := call(newTestRouter(), http.MethodPost, "/health", "", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestCreateWithoutIdentityIsRejected(t *testing.T) {
	rec := call(newTestRouter(), http.MethodPost, "/api/iqubs", validBody, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCreateWithInvalidAmountReturns400(t *testing.T) {
	body := `{"name":"A","contribution_amount":0,"frequency":"MONTHLY","max_members":10}`
	rec := call(newTestRouter(), http.MethodPost, "/api/iqubs", body, "m1")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateThenListShowsOnlyOwnGroups(t *testing.T) {
	router := newTestRouter()

	rec := call(router, http.MethodPost, "/api/iqubs", validBody, "m1")
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = call(router, http.MethodGet, "/api/iqubs", "", "m1")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Family Iqub") {
		t.Fatalf("m1 should see the group, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = call(router, http.MethodGet, "/api/iqubs", "", "m2")
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("m2 should see an empty list, got %s", rec.Body.String())
	}
}
