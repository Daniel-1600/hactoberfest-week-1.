package inference

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"ecosound/backend/internal/models"
)

// Modal calls the authenticated BirdNET Modal web function with normalized WAV bytes.
// Proxy credentials are held only in this backend process.
type Modal struct {
	Endpoint, TokenID, TokenSecret string
	Timeout                        time.Duration
	Client                         *http.Client
}

func (m Modal) Identify(ctx context.Context, path string) (models.InferenceResult, error) {
	if strings.TrimSpace(m.Endpoint) == "" || strings.TrimSpace(m.TokenID) == "" || strings.TrimSpace(m.TokenSecret) == "" {
		return models.InferenceResult{}, ErrUnavailable
	}
	f, err := os.Open(path)
	if err != nil {
		return models.InferenceResult{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	if err != nil {
		return models.InferenceResult{}, err
	}
	if len(data) > 8<<20 {
		return models.InferenceResult{}, fmt.Errorf("%w: WAV exceeds Modal request limit", ErrMalformed)
	}
	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 180 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Endpoint, bytes.NewReader(data))
	if err != nil {
		return models.InferenceResult{}, fmt.Errorf("%w: invalid Modal endpoint", ErrUnavailable)
	}
	req.Header.Set("Content-Type", "audio/wav")
	req.Header.Set("Modal-Key", m.TokenID)
	req.Header.Set("Modal-Secret", m.TokenSecret)
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return models.InferenceResult{}, fmt.Errorf("%w: %v", ErrTimeout, ctx.Err())
		}
		return models.InferenceResult{}, ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return models.InferenceResult{}, fmt.Errorf("%w: Modal returned HTTP %d", ErrUnavailable, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil {
		return models.InferenceResult{}, fmt.Errorf("%w: read Modal response", ErrUnavailable)
	}
	if len(body) > 4<<20 {
		return models.InferenceResult{}, fmt.Errorf("%w: response too large", ErrMalformed)
	}
	return DecodeResult(body)
}
