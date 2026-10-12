package main

import (
	"ecosound/backend/internal/api"
	"net/http"
	"testing"
	"time"
)

func TestServerSupportsQwenRequestBudget(t *testing.T) {
	s := newHTTPServer("127.0.0.1:8080", http.NotFoundHandler())
	if s.WriteTimeout < api.QwenRequestTimeout+s.ReadTimeout {
		t.Fatal("server can cut off Qwen response")
	}
	if s.ReadTimeout != 45*time.Second || s.ReadHeaderTimeout != 10*time.Second || s.MaxHeaderBytes != 1<<20 {
		t.Fatal("request input protections changed")
	}
}
