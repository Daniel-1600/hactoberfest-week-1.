package retrieval

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"ecosound/backend/internal/ai/llm"
	"ecosound/backend/internal/db"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCuratedLookupAndGroundedExplanation(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := Service{DB: store}
	item, err := service.Lookup(context.Background(), "Bostrychia hagedash")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(item.SourceURL, "iucn.org") && !strings.Contains(item.SourceURL, "doi.org") {
		t.Fatalf("missing source attribution: %+v", item)
	}
	if _, err = service.Lookup(context.Background(), "Unknown species"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown species=%v", err)
	}
	if _, err = store.Exec(`INSERT INTO recordings(id,file_path,content_type,duration_seconds,recorded_at,created_at) VALUES('r','r.wav','audio/wav',5,'2026-01-01','2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Exec(`INSERT INTO predictions(recording_id,model_name,model_version,raw_species_name,scientific_name,common_name,confidence,created_at) VALUES('r','birdnet','3.0','Bostrychia hagedash_Hadada Ibis','Bostrychia hagedash','Hadada Ibis',0.8,'2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Exec(`INSERT INTO discoveries(prediction_id,recording_id,observed_at,created_at) VALUES(1,'r','2026-01-01','2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Explain(context.Background(), 1); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("disabled model=%v", err)
	}
	service = Service{DB: store, Enabled: true, Model: llm.Client{Endpoint: "http://qwen.local/v1/chat/completions", Model: "qwen3", HTTP: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "wooded streams") || !strings.Contains(string(body), "curated bird facts") || !strings.Contains(string(body), "birdnet_prediction") {
			t.Fatalf("explanation was not grounded: %s", body)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"explanation\":\"The Hadada Ibis is a BirdNET suggestion. The cited record describes foraging in wet green spaces for invertebrates.\",\"outdoor_activities\":[\"Listen quietly near a wet green space.\",\"Look for signs of invertebrates without disturbing habitat.\",\"Note the plants and water around a nearby path.\"]}"}}]}`)), Request: r}, nil
	})}}}
	answer, err := service.Explain(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(answer.Explanation, "invertebrates") || answer.SourceURL == "" || answer.Disclaimer == "" || len(answer.OutdoorActivities) != 3 {
		t.Fatalf("bad grounded explanation: %+v", answer)
	}
	unsafe := Service{DB: store, Enabled: true, Model: llm.Client{Endpoint: "http://qwen.local/v1/chat/completions", Model: "qwen3", HTTP: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"explanation\":\"This recording has confirmed a Hadada Ibis.\",\"outdoor_activities\":[\"Activity suggestion one.\",\"Activity suggestion two.\",\"Activity suggestion three.\"]}"}}]}`)), Request: r}, nil
	})}}}
	if _, err = unsafe.Explain(context.Background(), 1); err == nil {
		t.Fatal("unverified identification was presented as confirmed")
	}
	calls := 0
	service.Model.HTTP = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "900 UTF-8 bytes") || !strings.Contains(string(body), "180 UTF-8 bytes") {
			t.Error("missing explicit output limits")
		}
		content := `{"explanation":"The Hadada Ibis is a BirdNET suggestion, not independently verified.","outdoor_activities":["Listen quietly from a permitted path.","Observe nearby plants without disturbing them.","Record notes about the surrounding habitat."]}`
		if calls == 1 {
			content = `{"explanation":"` + strings.Repeat("x", 901) + `","outdoor_activities":[]}`
		} else if !strings.Contains(string(body), "previous response was rejected") || !strings.Contains(string(body), "curated_species_record") {
			t.Error("retry missing validation feedback or grounded facts")
		}
		encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(encoded))), Request: r}, nil
	})}
	if _, err := service.Explain(context.Background(), 1); err != nil || calls != 2 {
		t.Fatalf("invalid output did not recover with one retry: calls=%d error=%v", calls, err)
	}
	calls = 0
	service.Model.HTTP = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{}"}}]}`)), Request: r}, nil
	})}
	if _, err := service.Explain(context.Background(), 1); err == nil || calls != 2 {
		t.Fatalf("invalid responses must stop after two attempts: calls=%d error=%v", calls, err)
	}
	calls = 0
	service.Model.HTTP = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, context.DeadlineExceeded
	})}
	if _, err := service.Explain(context.Background(), 1); !errors.Is(err, ErrModelUnavailable) || calls != 1 {
		t.Fatalf("model failure should not retry: calls=%d error=%v", calls, err)
	}
}

func TestExplanationValidationLimits(t *testing.T) {
	for _, tc := range []struct {
		name, explanation string
		activities        []string
		extra             bool
	}{
		{"too_long", strings.Repeat("x", 901), []string{"Listen quietly.", "Observe plants.", "Record notes."}, false},
		{"too_few_activities", "A BirdNET candidate.", []string{"Listen quietly."}, false},
		{"activity_too_long", "A BirdNET candidate.", []string{strings.Repeat("x", 181), "Observe plants.", "Record notes."}, false},
		{"unsafe_claim", "This is a confirmed identification.", []string{"Listen quietly.", "Observe plants.", "Record notes."}, false},
		{"unknown_field", "A BirdNET candidate.", []string{"Listen quietly.", "Observe plants.", "Record notes."}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := map[string]any{"explanation": tc.explanation, "outdoor_activities": tc.activities}
			if tc.extra {
				value["extra"] = "not allowed"
			}
			encoded, _ := json.Marshal(value)
			if _, err := parseExplanation(string(encoded)); err == nil {
				t.Fatal("accepted invalid output")
			}
		})
	}
}
