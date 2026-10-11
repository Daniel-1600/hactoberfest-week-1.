package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ecosound/backend/internal/db"
	"ecosound/backend/internal/models"
	"ecosound/backend/internal/storage"
)

func communityFixture(t *testing.T) (*Server, *db.Store, int64) {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	dir := t.TempDir()
	audio, err := storage.New(filepath.Join(dir, "audio"))
	if err != nil {
		t.Fatal(err)
	}
	zone, _ := time.LoadLocation("Africa/Nairobi")
	now := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	identifier := fixtureIdentifier{result: models.InferenceResult{Model: "birdnet", ModelVersion: "3.0", Predictions: []models.Prediction{{ScientificName: "Bostrychia hagedash", CommonName: "Hadada Ibis", RawSpeciesName: "Bostrychia hagedash_Hadada Ibis", Confidence: .8}}}}
	server, err := New(Config{DB: store, Storage: audio, Identifier: identifier, QuestTimezone: zone, Now: func() time.Time { return now }, PublicMapEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return server, store, 0
}

func TestObservationConsentPrivacyDestinationsAndPassport(t *testing.T) {
	server, store, _ := communityFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	if _, err := store.ExecContext(ctx, `INSERT INTO recordings(id,file_path,content_type,duration_seconds,recorded_at,created_at) VALUES('r','r.wav','audio/wav',4,?,?)`, db.Timestamp(now), db.Timestamp(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `INSERT INTO predictions(recording_id,model_name,model_version,raw_species_name,scientific_name,common_name,confidence,created_at) VALUES('r','birdnet','3.0','Bostrychia hagedash_Hadada Ibis','Bostrychia hagedash','Hadada Ibis',0.8,?)`, db.Timestamp(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `INSERT INTO discoveries(prediction_id,recording_id,observed_at,created_at) VALUES(1,'r',?,?)`, db.Timestamp(now), db.Timestamp(now)); err != nil {
		t.Fatal(err)
	}
	withoutConsent := request(t, server.Handler(), http.MethodPost, "/api/observations", "application/json", []byte(`{"discovery_id":1,"visibility":"public","location_source":"device","latitude":-1.2634,"longitude":36.8172}`))
	if withoutConsent.Code != 422 {
		t.Fatalf("consent status=%d body=%s", withoutConsent.Code, withoutConsent.Body)
	}
	withConsent := request(t, server.Handler(), http.MethodPost, "/api/observations", "application/json", []byte(`{"discovery_id":1,"visibility":"public","consent":true,"location_source":"device","latitude":-1.2634,"longitude":36.8172,"location_accuracy_meters":12,"consent_version":"ecosound-observation-v1"}`))
	if withConsent.Code != 201 {
		t.Fatalf("create status=%d body=%s", withConsent.Code, withConsent.Body)
	}
	var public struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Status    string  `json:"verification_status"`
	}
	if err := json.Unmarshal(withConsent.Body.Bytes(), &public); err != nil {
		t.Fatal(err)
	}
	if public.Latitude != -1.26 || public.Longitude != 36.82 || public.Status != "ai_predicted" {
		t.Fatalf("privacy/status=%+v", public)
	}
	mapResponse := request(t, server.Handler(), http.MethodGet, "/api/map/observations?status=ai_predicted", "", nil)
	if mapResponse.Code != 200 || strings.Contains(mapResponse.Body.String(), "-1.2634") {
		t.Fatalf("unsafe map response: %d %s", mapResponse.Code, mapResponse.Body)
	}
	confirmed := request(t, server.Handler(), http.MethodPost, "/api/discoveries/1/confirm", "application/json", []byte(`{}`))
	if confirmed.Code != 200 {
		t.Fatalf("personal confirmation failed: %d %s", confirmed.Code, confirmed.Body)
	}
	mapResponse = request(t, server.Handler(), http.MethodGet, "/api/map/observations?status=user_confirmed", "", nil)
	if mapResponse.Code != 200 || !strings.Contains(mapResponse.Body.String(), `"verification_status":"user_confirmed"`) {
		t.Fatalf("map did not show personal status: %d %s", mapResponse.Code, mapResponse.Body)
	}
	destination := request(t, server.Handler(), http.MethodGet, "/api/destinations/karura-concept", "", nil)
	if destination.Code != 200 || !strings.Contains(destination.Body.String(), "Concept demonstration — not affiliated with Karura Forest management") || strings.Contains(destination.Body.String(), "geometry_geojson") {
		t.Fatalf("route-free concept not presented clearly: %d %s", destination.Code, destination.Body)
	}
	trails := request(t, server.Handler(), http.MethodGet, "/api/destinations/karura-concept/trails", "", nil)
	if trails.Code != 200 || strings.Contains(trails.Body.String(), "geometry_geojson") || strings.Contains(trails.Body.String(), "Karura") {
		t.Fatalf("unverified trail leaked: %s", trails.Body)
	}
	stats := request(t, server.Handler(), http.MethodGet, "/api/profile/stats", "", nil)
	if stats.Code != 200 || !strings.Contains(stats.Body.String(), `"measured_outdoor_minutes":0`) {
		t.Fatalf("stats=%d %s", stats.Code, stats.Body)
	}
}

func TestAdventureFallbackAndSyncArePersistentAndIdempotent(t *testing.T) {
	server, store, _ := communityFixture(t)
	invalidDestination := request(t, server.Handler(), http.MethodPost, "/api/adventures/generate", "application/json", []byte(`{"duration_minutes":15,"interest":"birdwatching","environment":"approved_nature_destination","difficulty":"beginner","destination_id":"karura-concept"}`))
	if invalidDestination.Code != 400 {
		t.Fatalf("unverified concept accepted as an approved destination: %d %s", invalidDestination.Code, invalidDestination.Body)
	}
	generated := request(t, server.Handler(), http.MethodPost, "/api/adventures/generate", "application/json", []byte(`{"duration_minutes":15,"interest":"birdwatching","environment":"forest","difficulty":"beginner"}`))
	if generated.Code != 201 {
		t.Fatalf("generation %d %s", generated.Code, generated.Body)
	}
	var adventure struct {
		ID     string `json:"id"`
		Source string `json:"generation_source"`
		Tasks  []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(generated.Body.Bytes(), &adventure); err != nil {
		t.Fatal(err)
	}
	if adventure.Source != "fallback" || len(adventure.Tasks) != 3 {
		t.Fatalf("must label fallback: %+v", adventure)
	}
	started := request(t, server.Handler(), http.MethodPost, "/api/adventures/"+adventure.ID+"/start", "application/json", []byte(`{}`))
	if started.Code != 200 {
		t.Fatalf("start %d %s", started.Code, started.Body)
	}
	// A self-reported pause can be synchronized; task uniqueness prevents repeated XP.
	payload := `{"operations":[{"operation_id":"offline-op-0001","type":"complete_adventure_task","payload":{"adventure_id":"` + adventure.ID + `","task_id":"task-1","self_reported":true}}]}`
	first := request(t, server.Handler(), http.MethodPost, "/api/sync", "application/json", []byte(payload))
	if first.Code != 200 || !strings.Contains(first.Body.String(), `"status":"applied"`) {
		t.Fatalf("sync=%d %s", first.Code, first.Body)
	}
	second := request(t, server.Handler(), http.MethodPost, "/api/sync", "application/json", []byte(payload))
	if second.Code != 200 || !strings.Contains(second.Body.String(), `"status":"already_applied"`) {
		t.Fatalf("retry=%d %s", second.Code, second.Body)
	}
	reuse := request(t, server.Handler(), http.MethodPost, "/api/sync", "application/json", []byte(strings.Replace(payload, "\"self_reported\":true", "\"self_reported\":false", 1)))
	if !strings.Contains(reuse.Body.String(), "reused with different data") {
		t.Fatalf("changed payload under same operation id accepted: %s", reuse.Body)
	}
	var xp int
	if err := store.QueryRow(`SELECT coalesce(sum(points),0) FROM xp_events`).Scan(&xp); err != nil {
		t.Fatal(err)
	}
	if xp != 20 {
		t.Fatalf("XP=%d; expected one self-report task reward", xp)
	}
}
