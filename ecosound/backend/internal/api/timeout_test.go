package api

import (
	"ecosound/backend/internal/ai/llm"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQwenRequestTimeouts(t *testing.T) {
	for _, pattern := range []string{"POST /api/adventures/generate", "GET /api/inference/providers", "POST /api/discoveries/{id}/explain"} {
		if requestTimeout(pattern) < 3*llm.DefaultTimeout {
			t.Errorf("%s cannot support availability plus two completions", pattern)
		}
	}
	if requestTimeout("POST /api/identify") != 240*time.Second {
		t.Fatal("BirdNET deadline changed")
	}
}

func TestHandlerAppliesModelDeadline(t *testing.T) {
	for _, tc := range []struct {
		pattern, method, path string
		budget                time.Duration
	}{
		{"POST /api/adventures/generate", "POST", "/api/adventures/generate", QwenRequestTimeout},
		{"GET /api/inference/providers", "GET", "/api/inference/providers", QwenRequestTimeout},
		{"POST /api/discoveries/{id}/explain", "POST", "/api/discoveries/1/explain", QwenRequestTimeout},
		{"POST /api/identify", "POST", "/api/identify", 240 * time.Second},
	} {
		t.Run(tc.path, func(t *testing.T) {
			s := &Server{mux: http.NewServeMux()}
			s.mux.HandleFunc(tc.pattern, func(w http.ResponseWriter, r *http.Request) {
				deadline, ok := r.Context().Deadline()
				remaining := time.Until(deadline)
				if !ok || remaining > tc.budget || remaining < tc.budget-time.Second {
					t.Errorf("handler deadline %v, expected %v", remaining, tc.budget)
				}
			})
			s.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.path, nil))
		})
	}
}
