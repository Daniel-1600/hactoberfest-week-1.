package retrieval

import (
	"context"
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
}
