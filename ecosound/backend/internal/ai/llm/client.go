// Package llm calls a local or Modal OpenAI-compatible model server.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	Endpoint      string
	Model         string
	Timeout       time.Duration
	ProxyTokenID  string
	ProxyTokenKey string
	HTTP          *http.Client
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	ResponseFmt any           `json:"response_format,omitempty"`
	ExtraBody   any           `json:"chat_template_kwargs,omitempty"`
}
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c Client) Complete(ctx context.Context, system, prompt string, jsonMode bool) (string, error) {
	if c.Endpoint == "" || c.Model == "" {
		return "", errors.New("Qwen endpoint or model is not configured")
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body := chatRequest{Model: c.Model, Messages: []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: prompt}}, Temperature: 0.3, MaxTokens: 900}
	body.ExtraBody = map[string]bool{"enable_thinking": false}
	if jsonMode {
		body.ResponseFmt = map[string]string{"type": "json_object"}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(encoded))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setProxyAuth(req)
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: timeout, CheckRedirect: rejectRedirect}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Qwen request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("Qwen returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return "", err
	}
	if len(data) > 1<<20 {
		return "", errors.New("Qwen response exceeds 1 MiB")
	}
	var result chatResponse
	if err := json.Unmarshal(data, &result); err != nil || len(result.Choices) != 1 {
		return "", errors.New("Qwen returned an invalid chat response")
	}
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("Qwen returned empty content")
	}
	return content, nil
}

func (c Client) Available(ctx context.Context) bool {
	if c.Endpoint == "" || c.Model == "" {
		return false
	}
	base := strings.TrimSuffix(c.Endpoint, "/chat/completions") + "/models"
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base, nil)
	if err != nil {
		return false
	}
	c.setProxyAuth(req)
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second, CheckRedirect: rejectRedirect}
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}
	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, (1<<20)+1)).Decode(&models) != nil {
		return false
	}
	for _, model := range models.Data {
		if model.ID == c.Model {
			return true
		}
	}
	return false
}

func (c Client) setProxyAuth(req *http.Request) {
	if c.ProxyTokenID != "" && c.ProxyTokenKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.ProxyTokenID+"."+c.ProxyTokenKey)
	}
}

func rejectRedirect(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}
