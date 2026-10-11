package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"ecosound/backend/internal/models"
)

// InferenceProvider represents independent model outputs without coercing audio descriptions into taxonomy.
type InferenceProvider interface {
	Name() string
	SupportedTasks() []string
	Predict(context.Context, string, string) (ProviderResult, error)
	Available(context.Context) bool
}
type ProviderResult struct {
	Provider              string               `json:"provider"`
	Task                  string               `json:"task"`
	SpeciesIdentification bool                 `json:"species_identification"`
	Predictions           []models.Prediction  `json:"predictions,omitempty"`
	Descriptions          []DescriptionSegment `json:"descriptions,omitempty"`
	Disclaimer            string               `json:"disclaimer"`
}
type DescriptionSegment struct {
	StartTime *float64 `json:"start_time,omitempty"`
	EndTime   *float64 `json:"end_time,omitempty"`
	Text      string   `json:"text"`
}

type BirdNETProvider struct{ Model Identifier }

func (BirdNETProvider) Name() string { return "birdnet" }
func (BirdNETProvider) SupportedTasks() []string {
	return []string{"bird_species_classification", "segment_timestamps"}
}
func (p BirdNETProvider) Available(context.Context) bool { return p.Model != nil }
func (p BirdNETProvider) Predict(ctx context.Context, path, task string) (ProviderResult, error) {
	if task != "bird_species_classification" {
		return ProviderResult{}, errors.New("unsupported BirdNET task")
	}
	if p.Model == nil {
		return ProviderResult{}, ErrUnavailable
	}
	result, err := p.Model.Identify(ctx, path)
	if err != nil {
		return ProviderResult{}, err
	}
	return ProviderResult{Provider: "birdnet", Task: task, SpeciesIdentification: true, Predictions: result.Predictions, Disclaimer: "AI prediction; not independently scientifically verified."}, nil
}

type NatureLMProvider struct {
	Endpoint string
	Enabled  bool
	Timeout  time.Duration
	Client   *http.Client
}

func (NatureLMProvider) Name() string { return "naturelm-audio" }
func (NatureLMProvider) SupportedTasks() []string {
	return []string{"audio_description", "vocalization_characteristics"}
}

var segmentPattern = regexp.MustCompile(`^#([0-9]+(?:\.[0-9]+)?)s\s*-\s*([0-9]+(?:\.[0-9]+)?)s#:\s*(.*)$`)

func (p NatureLMProvider) Available(ctx context.Context) bool {
	if !p.Enabled || p.Endpoint == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(p.Endpoint, "/")+"/health", nil)
	if err != nil {
		return false
	}
	c := p.Client
	if c == nil {
		c = &http.Client{Timeout: 2 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}
func (p NatureLMProvider) Predict(ctx context.Context, path, task string) (ProviderResult, error) {
	if !p.Enabled {
		return ProviderResult{}, ErrUnavailable
	}
	if task != "audio_description" && task != "vocalization_characteristics" {
		return ProviderResult{}, errors.New("unsupported NatureLM task")
	}
	file, err := os.Open(path)
	if err != nil {
		return ProviderResult{}, err
	}
	defer file.Close()
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(p.Endpoint, "/")+"/infer", file)
	if err != nil {
		return ProviderResult{}, err
	}
	req.Header.Set("Content-Type", "audio/wav")
	req.Header.Set("X-EcoSound-Task", task)
	c := p.Client
	if c == nil {
		c = &http.Client{Timeout: timeout}
	}
	resp, err := c.Do(req)
	if err != nil {
		return ProviderResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ProviderResult{}, fmt.Errorf("NatureLM returned HTTP %d", resp.StatusCode)
	}
	var wire struct {
		Provider              string   `json:"provider"`
		Task                  string   `json:"task"`
		Outputs               []string `json:"outputs"`
		SpeciesIdentification bool     `json:"species_identification"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&wire); err != nil {
		return ProviderResult{}, err
	}
	if wire.Provider != "naturelm-audio" || wire.Task != task || wire.SpeciesIdentification || len(wire.Outputs) == 0 || len(wire.Outputs) > 30 {
		return ProviderResult{}, errors.New("invalid NatureLM response contract")
	}
	result := ProviderResult{Provider: wire.Provider, Task: task, SpeciesIdentification: false, Descriptions: []DescriptionSegment{}, Disclaimer: "Experimental model-generated audio description; not a verified species identification."}
	for _, output := range wire.Outputs {
		text := strings.TrimSpace(output)
		if text == "" || len(text) > 2000 {
			return ProviderResult{}, errors.New("invalid NatureLM description")
		}
		segment := DescriptionSegment{Text: text}
		if match := segmentPattern.FindStringSubmatch(text); match != nil {
			start, _ := time.ParseDuration(match[1] + "s")
			end, _ := time.ParseDuration(match[2] + "s")
			a, b := start.Seconds(), end.Seconds()
			segment.StartTime = &a
			segment.EndTime = &b
			segment.Text = strings.TrimSpace(match[3])
			if b < a || segment.Text == "" {
				return ProviderResult{}, errors.New("invalid NatureLM segment")
			}
		}
		result.Descriptions = append(result.Descriptions, segment)
	}
	return result, nil
}
