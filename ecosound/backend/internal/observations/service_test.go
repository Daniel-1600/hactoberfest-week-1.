package observations

import (
	"context"
	"testing"
	"time"

	"ecosound/backend/internal/db"
)

func observationFixture(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	now := db.Timestamp(time.Now())
	if _, err = store.Exec(`INSERT INTO recordings(id,file_path,content_type,duration_seconds,recorded_at,created_at) VALUES('rec','rec.wav','audio/wav',5,?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Exec(`INSERT INTO predictions(recording_id,model_name,model_version,raw_species_name,scientific_name,common_name,confidence,created_at) VALUES('rec','birdnet','3.0','Bostrychia hagedash_Hadada Ibis','Bostrychia hagedash','Hadada Ibis',0.8,?)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Exec(`INSERT INTO discoveries(prediction_id,recording_id,observed_at,created_at) VALUES(1,'rec',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	return store
}
func TestPublicCoordinatesAreCoarseAndSensitiveSpeciesHidden(t *testing.T) {
	ctx := context.Background()
	store := observationFixture(t)
	now := time.Now()
	lat, lon, accuracy := -1.2634, 36.8172, 12.0
	item, err := Create(ctx, store, Input{DiscoveryID: 1, Consent: true, ConsentVersion: ConsentVersion, Visibility: "public", Latitude: &lat, Longitude: &lon, AccuracyMeters: &accuracy, LocationSource: "device"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if item.Latitude == nil || *item.Latitude != -1.26 || *item.Longitude != 36.82 || !item.LocationGeneralized {
		t.Fatalf("not generalized: %+v", item)
	}
	var privateLat, pubLat float64
	if err = store.QueryRow(`SELECT latitude,public_latitude FROM observations WHERE id=?`, item.ID).Scan(&privateLat, &pubLat); err != nil {
		t.Fatal(err)
	}
	if privateLat != lat || pubLat != *item.Latitude {
		t.Fatalf("private=%f public=%f", privateLat, pubLat)
	}
	public, err := List(ctx, store, Filter{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(public) != 1 || public[0].Latitude == nil {
		t.Fatalf("public observations=%+v", public)
	}
	if _, err = store.Exec(`UPDATE species_knowledge SET sensitive=1 WHERE scientific_name='Bostrychia hagedash'`); err != nil {
		t.Fatal(err)
	}
	public, err = List(ctx, store, Filter{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(public) != 0 {
		t.Fatalf("sensitive species leaked: %+v", public)
	}
}
func TestConsentInvalidCoordinatesPrivateVisibilityAndVerification(t *testing.T) {
	ctx := context.Background()
	store := observationFixture(t)
	now := time.Now()
	lat, lon := 91.0, 181.0
	if _, err := Create(ctx, store, Input{DiscoveryID: 1, Visibility: "public", Latitude: &lat, Longitude: &lon, LocationSource: "device"}, now); err != ErrInvalid {
		t.Fatalf("consent not enforced: %v", err)
	}
	if _, err := Create(ctx, store, Input{DiscoveryID: 1, Consent: true, ConsentVersion: ConsentVersion, Visibility: "public", Latitude: &lat, Longitude: &lon, LocationSource: "device"}, now); err != ErrInvalid {
		t.Fatalf("invalid location accepted: %v", err)
	}
	lat, lon = -1.2, 36.8
	if _, err := store.Exec(`UPDATE discoveries SET user_confirmed=1 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	item, err := Create(ctx, store, Input{DiscoveryID: 1, Consent: true, ConsentVersion: ConsentVersion, Visibility: "private", Latitude: &lat, Longitude: &lon, LocationSource: "manual"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if item.VerificationStatus != "user_confirmed" || item.Latitude != nil {
		t.Fatalf("private location or verification wrong: %+v", item)
	}
	rows, err := List(ctx, store, Filter{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("private item included in map: %+v", rows)
	}
}

func TestVeryCoarseCoordinatesRoundMoreForPublicMap(t *testing.T) {
	ctx := context.Background()
	store := observationFixture(t)
	lat, lon := -1.2634, 36.8172
	input := Input{DiscoveryID: 1, Consent: true, ConsentVersion: ConsentVersion, Visibility: "public", Latitude: &lat, Longitude: &lon, LocationSource: "device", IdempotencyKey: "offline-observation-0001"}
	item, err := CreateWithPrecision(ctx, store, input, time.Now(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if item.Latitude == nil || *item.Latitude != -1.3 || *item.Longitude != 36.8 {
		t.Fatalf("unexpected public coordinates: %+v", item)
	}
	var privateLat float64
	if err := store.QueryRow(`SELECT latitude FROM observations WHERE id=?`, item.ID).Scan(&privateLat); err != nil {
		t.Fatal(err)
	}
	if privateLat != lat {
		t.Fatalf("private coordinate was changed: %f", privateLat)
	}
	duplicate, err := CreateWithPrecision(ctx, store, input, time.Now().Add(time.Minute), 1)
	if err != nil || duplicate.ID != item.ID {
		t.Fatalf("repeated request did not return existing observation: %+v %v", duplicate, err)
	}
	var count int
	if err := store.QueryRow(`SELECT count(*) FROM observations`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("observations=%d error=%v", count, err)
	}
}
