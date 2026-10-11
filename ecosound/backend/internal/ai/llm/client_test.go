package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestModalProxyAuthorizationAndQwenThinkingConfiguration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer wk-id.ws-secret" {
			t.Fatalf("proxy token not sent by backend client: %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"chat_template_kwargs":{"enable_thinking":false}`) || !strings.Contains(string(body), `"response_format":{"type":"json_object"}`) {
			t.Fatalf("Qwen request options missing: %s", body)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`))
	}))
	defer server.Close()
	client := Client{Endpoint: server.URL + "/v1/chat/completions", Model: "qwen3-1.7b", ProxyTokenID: "wk-id", ProxyTokenKey: "ws-secret"}
	if _, err := client.Complete(context.Background(), "system", "prompt", true); err != nil {
		t.Fatal(err)
	}
}

func TestCompletionHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()
	client := Client{Endpoint: server.URL, Model: "qwen3", Timeout: 10 * time.Millisecond}
	if _, err := client.Complete(context.Background(), "system", "prompt", false); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout not returned: %v", err)
	}
}
