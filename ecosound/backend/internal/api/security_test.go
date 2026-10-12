package api

import (
	"ecosound/backend/internal/db"
	"ecosound/backend/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCORSAndJSONValidation(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	audio, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(Config{DB: store, Storage: audio, Identifier: fixtureIdentifier{}, QuestTimezone: time.UTC, Origin: "http://localhost:3000"})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.Handler()
	for _, tc := range []struct {
		origin, method string
		status         int
	}{{"http://localhost:3000", "GET", 200}, {"http://localhost:3000", "OPTIONS", 204}, {"https://untrusted.example", "POST", 403}} {
		req := httptest.NewRequest(tc.method, "/api/discoveries", nil)
		req.Header.Set("Origin", tc.origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != tc.status {
			t.Fatalf("origin %s: got %d", tc.origin, response.Code)
		}
		if tc.status != 403 && response.Header().Get("Access-Control-Allow-Origin") != tc.origin {
			t.Fatal("missing allowed CORS origin")
		}
		if tc.status == 403 && response.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("untrusted origin granted access")
		}
		if tc.method == "OPTIONS" && tc.status == 204 && !strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "Idempotency-Key") {
			t.Fatal("browser upload preflight does not allow idempotency header")
		}
	}
	for _, body := range []string{`{"prediction_id":1,"scientific_name":"Fake species"}`, `{"prediction_id":1} {"prediction_id":2}`, `{"prediction_id":0}`, `{"prediction_id":"1"}`} {
		response := request(t, handler, "POST", "/api/discoveries", "application/json", []byte(body))
		if response.Code != 400 {
			t.Fatalf("accepted invalid client fields %s: %d", body, response.Code)
		}
	}
	response := request(t, handler, "GET", "/api/identify", "", nil)
	if response.Code != http.StatusMethodNotAllowed || !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("method error: %d %s", response.Code, response.Body.String())
	}
}
