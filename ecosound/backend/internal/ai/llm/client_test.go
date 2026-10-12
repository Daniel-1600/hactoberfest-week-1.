package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestModalColdStart303(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer wk-id.ws-secret" {
			t.Error("lost proxy authentication")
		}
		if !strings.HasPrefix(r.URL.Path, "/ready/") {
			http.Redirect(w, r, "/ready"+r.URL.Path, http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("303 did not retrieve queued response with GET: %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) != 0 {
			t.Error("303 replayed prompt body")
		}
		if strings.HasSuffix(r.URL.Path, "/models") {
			io.WriteString(w, `{"data":[{"id":"qwen3"}]}`)
		} else {
			io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
		}
	}))
	defer server.Close()
	custom := server.Client()
	custom.Timeout = time.Second
	custom.CheckRedirect = func(*http.Request, []*http.Request) error { t.Error("unsafe caller policy used"); return nil }
	c := Client{Endpoint: server.URL + "/v1/chat/completions", Model: "qwen3", ProxyTokenID: "wk-id", ProxyTokenKey: "ws-secret", HTTP: custom}
	if !c.Available(context.Background()) {
		t.Fatal("cold-start model check failed")
	}
	if answer, err := c.Complete(context.Background(), "system", "prompt", false); err != nil || answer != "ok" {
		t.Fatalf("cold-start completion: %q %v", answer, err)
	}
	if custom.Timeout != time.Second {
		t.Fatal("mutated caller's HTTP client")
	}
}

func TestRedirectCannotReachAnotherOrigin(t *testing.T) {
	for _, tls := range []bool{false, true} {
		t.Run(map[bool]string{false: "downgrade", true: "different_port"}[tls], func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("contacted untrusted redirect target") })
			var target *httptest.Server
			if tls {
				target = httptest.NewTLSServer(handler)
			} else {
				target = httptest.NewServer(handler)
			}
			defer target.Close()
			origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL, http.StatusSeeOther)
			}))
			defer origin.Close()
			c := Client{Endpoint: origin.URL, Model: "qwen3", ProxyTokenID: "wk-id", ProxyTokenKey: "ws-secret", HTTP: origin.Client()}
			if _, err := c.Complete(context.Background(), "", "", false); err == nil {
				t.Fatal("accepted untrusted redirect")
			}
		})
	}
}

func TestRedirectPolicy(t *testing.T) {
	original, _ := url.Parse("https://qwen.modal.run/v1/models")
	for _, tc := range []struct {
		name, target string
		status, hops int
		allowed      bool
	}{
		{"same_origin", "https://qwen.modal.run/result", 303, 1, true},
		{"other_host", "https://evil.example/result", 303, 1, false},
		{"subdomain", "https://evil.qwen.modal.run/result", 303, 1, false},
		{"downgrade", "http://qwen.modal.run/result", 303, 1, false},
		{"userinfo", "https://user@qwen.modal.run/result", 303, 1, false},
		{"loop", "https://qwen.modal.run/result", 303, 10, false},
		{"307", "https://qwen.modal.run/result", 307, 1, false},
		{"302", "https://qwen.modal.run/result", 302, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, _ := url.Parse(tc.target)
			via := make([]*http.Request, tc.hops)
			for i := range via {
				via[i] = &http.Request{URL: original}
			}
			err := modalRedirect(&http.Request{URL: target, Response: &http.Response{StatusCode: tc.status}}, via)
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v, error=%v", tc.allowed, err)
			}
		})
	}
}

func TestTimeoutBudgetsAndParentCancellation(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) < 299*time.Second || time.Until(deadline) > 300*time.Second {
			t.Errorf("model request lacks 300-second budget")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"qwen3"}],"choices":[{"message":{"content":"ok"}}]}`)), Header: make(http.Header)}, nil
	})
	c := Client{Endpoint: "https://qwen.modal.run/v1/chat/completions", Model: "qwen3", HTTP: &http.Client{Transport: transport}}
	if !c.Available(context.Background()) {
		t.Fatal("availability failed")
	}
	if _, err := c.Complete(context.Background(), "", "", false); err != nil {
		t.Fatal(err)
	}
	if c.httpClient().Timeout != 300*time.Second {
		t.Fatal("HTTP timeout does not support cold starts")
	}
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.Complete(ctx, "", "", false); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("parent deadline ignored: %v", err)
	}
	if c.Available(ctx) {
		t.Fatal("availability ignored expired parent deadline")
	}
	c.Timeout = 10 * time.Millisecond
	if c.Available(context.Background()) {
		t.Fatal("availability ignored configured timeout")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func TestTimeoutIncludesColdStartRedirectChain(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(30 * time.Millisecond):
		}
		if r.URL.Path != "/ready" {
			http.Redirect(w, r, "/ready", http.StatusSeeOther)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer server.Close()
	c := Client{Endpoint: server.URL, Model: "qwen3", Timeout: 45 * time.Millisecond, HTTP: server.Client()}
	if _, err := c.Complete(context.Background(), "", "", false); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("redirect chain reset timeout: %v", err)
	}
}

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAvailabilityUsesModalProxyAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer wk-id.ws-secret" {
			t.Fatalf("bad models request: %s %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen3-1.7b"}]}`))
	}))
	defer server.Close()
	client := Client{Endpoint: server.URL + "/v1/chat/completions", Model: "qwen3-1.7b", ProxyTokenID: "wk-id", ProxyTokenKey: "ws-secret"}
	if !client.Available(context.Background()) {
		t.Fatal("authenticated Modal Qwen model reported unavailable")
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

func TestCompletionRejectsRedirectAndOversize(t *testing.T) {
	for _, mode := range []string{"redirect", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/redirected" {
					t.Error("followed credential-bearing redirect")
				}
				if mode == "redirect" {
					http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
					return
				}
				_, _ = io.WriteString(w, strings.Repeat(" ", (1<<20)+1))
			}))
			defer server.Close()
			client := Client{Endpoint: server.URL, Model: "qwen3", ProxyTokenID: "wk-test", ProxyTokenKey: "ws-test"}
			if _, err := client.Complete(context.Background(), "system", "prompt", false); err == nil {
				t.Fatal("expected response rejection")
			}
		})
	}
}
