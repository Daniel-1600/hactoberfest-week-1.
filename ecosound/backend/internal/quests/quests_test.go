package quests

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ecosound/backend/internal/db"
	"ecosound/backend/internal/models"
)

func TestTodayUsesLocalObservationDayAndDistinctSavedData(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(filepath.Join(t.TempDir(), "quest.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	zone, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, zone)
	add := func(id, name string, observed time.Time) int64 {
		t.Helper()
		result := models.InferenceResult{Model: "birdnet", ModelVersion: "3.0", Predictions: []models.Prediction{{ScientificName: name, CommonName: "bird", RawSpeciesName: name + "_bird", Confidence: 0.8}}}
		identification, err := store.SaveIdentification(ctx, id, "/tmp/"+id+".wav", 4, observed, result)
		if err != nil {
			t.Fatal(err)
		}
		predictionID := identification.Predictions[0].ID
		if _, err := store.SaveDiscovery(ctx, predictionID, now); err != nil {
			t.Fatal(err)
		}
		return predictionID
	}
	// 21:00 UTC is Nairobi midnight: the start is inclusive and end exclusive.
	add("previous", "Corvus splendens", time.Date(2026, 10, 9, 20, 59, 59, 0, time.UTC))
	add("first", "Turdus merula", time.Date(2026, 10, 9, 21, 0, 0, 0, time.UTC))
	add("second", "Turdus merula", time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC))
	add("noise", "unknown", time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC))
	add("empty", "", time.Date(2026, 10, 10, 9, 30, 0, 0, time.UTC))
	add("tomorrow", "Passer domesticus", time.Date(2026, 10, 10, 21, 0, 0, 0, time.UTC))
	q, err := Today(ctx, store, zone, now)
	if err != nil {
		t.Fatal(err)
	}
	if q.Date != "2026-10-10" || q.Timezone != "Africa/Nairobi" {
		t.Fatalf("wrong local day: %+v", q)
	}
	if q.Tasks[0].Current != 4 || !q.Tasks[0].Completed {
		t.Fatalf("calls: %+v", q.Tasks[0])
	}
	if q.Tasks[1].Current != 1 || q.Tasks[1].Completed || q.Completed {
		t.Fatalf("species: %+v", q)
	}
	if _, err := store.Confirm(ctx, 2); err != nil {
		t.Fatal(err)
	}
	confirmed, err := Today(ctx, store, zone, now)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Tasks[0].Current != q.Tasks[0].Current || confirmed.Tasks[1].Current != q.Tasks[1].Current {
		t.Fatalf("confirmation changed quest: before=%+v after=%+v", q, confirmed)
	}
	add("third-species", "Passer domesticus", time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC))
	completed, err := Today(ctx, store, zone, now)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Tasks[1].Current != 2 || !completed.Completed {
		t.Fatalf("expected completed quest: %+v", completed)
	}
	next, err := Today(ctx, store, zone, now.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if next.Tasks[0].Current != 1 || next.Tasks[1].Current != 1 || next.Completed {
		t.Fatalf("local midnight boundary wrong: %+v", next)
	}
	dayAfter, err := Today(ctx, store, zone, now.AddDate(0, 0, 2))
	if err != nil {
		t.Fatal(err)
	}
	if dayAfter.Tasks[0].Current != 0 || dayAfter.Tasks[1].Current != 0 || dayAfter.Completed {
		t.Fatalf("quest did not reset: %+v", dayAfter)
	}
}
