package inference

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestModalPostsWAVWithProxyAuthenticationAndDecodesExistingContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audio.wav")
	if err := os.WriteFile(path, []byte("normalized wav bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	want := `{"model":"birdnet","model_version":"3.0","predictions":[]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "audio/wav" || r.Header.Get("Modal-Key") != "wk-test" || r.Header.Get("Modal-Secret") != "ws-test" {
			t.Fatalf("unexpected request: %s %v", r.Method, r.Header)
		}
		body := make([]byte, 64)
		n, _ := r.Body.Read(body)
		if string(body[:n]) != "normalized wav bytes" {
			t.Fatalf("unexpected body: %q", body[:n])
		}
		_, _ = w.Write([]byte(want))
	}))
	defer server.Close()
	result, err := (Modal{Endpoint: server.URL, TokenID: "wk-test", TokenSecret: "ws-test"}).Identify(context.Background(), path)
	if err != nil || result.ModelVersion != "3.0" || result.Predictions == nil || len(result.Predictions) != 0 {
		t.Fatalf("invalid result: %+v, %v", result, err)
	}
}

func TestModalRejectsInvalidContractsAndResponseSize(t *testing.T) {
	for _, response := range []string{
		`{"model":"other","model_version":"3.0","predictions":[]}`,
		`{"model":"birdnet","model_version":"3.0","predictions":[]}{}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(response)) }))
		_, err := (Modal{Endpoint: server.URL, TokenID: "id", TokenSecret: "secret"}).Identify(context.Background(), writeAudio(t))
		server.Close()
		if !errors.Is(err, ErrMalformed) {
			t.Fatalf("bad contract accepted: %v", err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", (4<<20)+1))) }))
	defer server.Close()
	_, err := (Modal{Endpoint: server.URL, TokenID: "id", TokenSecret: "secret"}).Identify(context.Background(), writeAudio(t))
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("oversized response accepted: %v", err)
	}
}

func TestModalTimeoutAndMissingCredentials(t *testing.T) {
	path := writeAudio(t)
	if _, err := (Modal{Endpoint: "http://example.invalid"}).Identify(context.Background(), path); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing credentials should fail closed: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	_, err := (Modal{Endpoint: server.URL, TokenID: "id", TokenSecret: "secret", Timeout: 5 * time.Millisecond}).Identify(context.Background(), path)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("deadline was not mapped to ErrTimeout: %v", err)
	}
}

func writeAudio(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audio.wav")
	if err := os.WriteFile(path, []byte("wav"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
