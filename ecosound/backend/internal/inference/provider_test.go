package inference

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type providerTransport func(*http.Request) (*http.Response, error)

func (f providerTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestNatureLMDescriptionDoesNotBecomeSpeciesIdentification(t *testing.T) {
	path := t.TempDir() + "/audio.wav"
	if err := os.WriteFile(path, []byte("wav"), 0600); err != nil {
		t.Fatal(err)
	}
	p := NatureLMProvider{Endpoint: "http://naturelm.local", Enabled: true, Client: &http.Client{Transport: providerTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/infer" || r.Header.Get("X-EcoSound-Task") != "audio_description" {
			t.Fatalf("bad request: %s %s", r.URL, r.Header)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"provider":"naturelm-audio","task":"audio_description","species_identification":false,"outputs":["#0.00s - 10.00s#: Repeated, high pitched calls."]}`)), Header: http.Header{}, Request: r}, nil
	})}}
	result, err := p.Predict(context.Background(), path, "audio_description")
	if err != nil {
		t.Fatal(err)
	}
	if result.SpeciesIdentification || len(result.Predictions) > 0 || len(result.Descriptions) != 1 || result.Descriptions[0].StartTime == nil || *result.Descriptions[0].EndTime != 10 {
		t.Fatalf("NatureLM output was coerced or lost: %+v", result)
	}
}
func TestNatureLMRejectsSpeciesClaimsAndUnsupportedTasks(t *testing.T) {
	path := t.TempDir() + "/audio.wav"
	_ = os.WriteFile(path, []byte("wav"), 0600)
	p := NatureLMProvider{Endpoint: "http://naturelm.local", Enabled: true, Client: &http.Client{Transport: providerTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"provider":"naturelm-audio","task":"audio_description","species_identification":true,"outputs":["Bird"]}`)), Request: r}, nil
	})}}
	if _, err := p.Predict(context.Background(), path, "audio_description"); err == nil {
		t.Fatal("verified classification accepted")
	}
	if _, err := p.Predict(context.Background(), path, "bird_species_classification"); err == nil {
		t.Fatal("NatureLM used as BirdNET replacement")
	}
	p.Enabled = false
	if p.Available(context.Background()) {
		t.Fatal("disabled provider reports availability")
	}
}
