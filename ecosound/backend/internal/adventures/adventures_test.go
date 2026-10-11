package adventures

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"ecosound/backend/internal/db"
)

type fakeModel struct {
	answers []string
	calls   int
	err     error
	prompts []string
}

func (m *fakeModel) Complete(_ context.Context, _, prompt string, _ bool) (string, error) {
	m.calls++
	m.prompts = append(m.prompts, prompt)
	if m.err != nil {
		return "", m.err
	}
	if len(m.answers) == 0 {
		return "", errors.New("empty fake")
	}
	answer := m.answers[0]
	if len(m.answers) > 1 {
		m.answers = m.answers[1:]
	}
	return answer, nil
}

func fixture(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}
func request() Request {
	return Request{DurationMinutes: 15, Interest: "birdwatching", Environment: "park", Difficulty: "beginner"}
}
func sampleJSON(title string) string {
	return fmt.Sprintf(`{"title":%q,"description":"Listen for nature at an easy pace.","tasks":[{"type":"outdoor_observation","title":"Pause and listen","instructions":"Notice nearby sounds in a safe permitted public place.","xp":20},{"type":"audio_discovery","title":"Record a call","instructions":"Record an animal sound without approaching or disturbing it.","xp":30},{"type":"species_discovery","title":"Review a bird","instructions":"Save a real BirdNET suggestion if your recording identifies a bird.","xp":30}]}`, title)
}

func TestGenerationUsesValidatedQwenAndPersistsProgress(t *testing.T) {
	store := fixture(t)
	model := &fakeModel{answers: []string{sampleJSON("A listening adventure")}}
	now := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	service := Service{DB: store, Model: model, ModelAvailable: func(context.Context) bool { return true }, Now: func() time.Time { return now }}
	a, err := service.Generate(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if a.GenerationSource != "qwen" || model.calls != 1 {
		t.Fatalf("generation source=%s calls=%d", a.GenerationSource, model.calls)
	}
	started, err := service.Start(context.Background(), a.ID, now)
	if err != nil || started.Status != "active" {
		t.Fatalf("start=%+v %v", started, err)
	}
	if _, err = service.CompleteTask(context.Background(), a.ID, a.Tasks[0].ID, 0, true, now); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CompleteTask(context.Background(), a.ID, a.Tasks[0].ID, 0, true, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate task should conflict: %v", err)
	}
	for _, task := range a.Tasks[1:] {
		if _, err = service.CompleteTask(context.Background(), a.ID, task.ID, 1, false, now); err == nil {
			t.Fatal("must require saved discovery")
		}
	}
	if _, err = store.Exec(`INSERT INTO recordings(id,file_path,content_type,duration_seconds,recorded_at,created_at) VALUES('r','r.wav','audio/wav',3,?,?)`, db.Timestamp(now), db.Timestamp(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Exec(`INSERT INTO predictions(recording_id,model_name,model_version,raw_species_name,scientific_name,common_name,confidence,created_at) VALUES('r','birdnet','3.0','Turdus merula_Blackbird','Turdus merula','Blackbird',0.8,?)`, db.Timestamp(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Exec(`INSERT INTO discoveries(prediction_id,recording_id,observed_at,created_at) VALUES(1,'r',?,?)`, db.Timestamp(now), db.Timestamp(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CompleteTask(context.Background(), a.ID, a.Tasks[1].ID, 1, false, now); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CompleteTask(context.Background(), a.ID, a.Tasks[2].ID, 1, false, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("same discovery cannot earn XP twice: %v", err)
	}
	if _, err = service.Finish(context.Background(), a.ID, nil, "heard birds", "quiet", nil, now); err == nil {
		t.Fatal("cannot finish with uncredited species task")
	}
	if _, err = store.Exec(`INSERT INTO recordings(id,file_path,content_type,duration_seconds,recorded_at,created_at) VALUES('r2','r2.wav','audio/wav',3,?,?)`, db.Timestamp(now), db.Timestamp(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Exec(`INSERT INTO predictions(recording_id,model_name,model_version,raw_species_name,scientific_name,common_name,confidence,created_at) VALUES('r2','birdnet','3.0','Ploceus cucullatus_Village Weaver','Ploceus cucullatus','Village Weaver',0.8,?)`, db.Timestamp(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Exec(`INSERT INTO discoveries(prediction_id,recording_id,observed_at,created_at) VALUES(2,'r2',?,?)`, db.Timestamp(now), db.Timestamp(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CompleteTask(context.Background(), a.ID, a.Tasks[2].ID, 2, false, now); err != nil {
		t.Fatal(err)
	}
	minutes := 8
	finished, err := service.Finish(context.Background(), a.ID, &minutes, "heard birds", "quiet", nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != "completed" || finished.SelfReportedMinutes == nil || *finished.SelfReportedMinutes != 8 {
		t.Fatalf("finish=%+v", finished)
	}
	if _, err = service.Finish(context.Background(), a.ID, nil, "", "", nil, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate finish=%v", err)
	}
	var count int
	if err = store.QueryRow(`SELECT count(*) FROM xp_events WHERE event_key LIKE ?`, "adventure:"+a.ID+":%").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("xp events=%d, expected 3 tasks plus finish", count)
	}
}

func TestInvalidModelOutputRetriesThenUsesLabeledFallback(t *testing.T) {
	store := fixture(t)
	model := &fakeModel{answers: []string{`{"title":"go near dangerous animals","description":"Too short.","tasks":[]}`, sampleJSON("safe generated")}}
	service := Service{DB: store, Model: model, ModelAvailable: func(context.Context) bool { return true }}
	a, err := service.Generate(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if a.GenerationSource != "qwen" || model.calls != 2 {
		t.Fatalf("adventure=%+v calls=%d", a, model.calls)
	}
	model = &fakeModel{err: errors.New("offline")}
	service.Model = model
	a, err = service.Generate(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if a.GenerationSource != "fallback" || a.Title != "A Quiet Nature Quest" {
		t.Fatalf("fallback not labeled: %+v", a)
	}
}

func TestRejectsUnsupportedPreferencesAndUnsafeActivity(t *testing.T) {
	if _, err := (Service{DB: fixture(t)}).Generate(context.Background(), Request{DurationMinutes: 30, Interest: "wildlife", Environment: "park", Difficulty: "beginner"}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Service{DB: fixture(t)}).Generate(context.Background(), Request{DurationMinutes: 90, Interest: "wildlife", Environment: "park", Difficulty: "beginner"}); err == nil {
		t.Fatal("unsupported duration accepted")
	}
	value, err := decodeGenerated(sampleJSON("safe adventure"))
	if err != nil {
		t.Fatal(err)
	}
	value.Tasks[0].Instructions = "Approach wildlife to see it closely."
	if err = validate(value, request()); !errors.Is(err, ErrUnsafe) {
		t.Fatalf("unsafe instruction accepted: %v", err)
	}
	value.Tasks[0].Instructions = "Climb the tree to look into the bird nest."
	if err = validate(value, request()); !errors.Is(err, ErrUnsafe) {
		t.Fatalf("dangerous activity accepted: %v", err)
	}
}

func TestDestinationAdventureUsesOnlyVerifiedServerGuidance(t *testing.T) {
	store := fixture(t)
	now := db.Timestamp(time.Now())
	if _, err := store.Exec(`INSERT INTO destinations(id,name,description,type,region,country,status,created_at,updated_at) VALUES('verified-place','Verified place','Approved destination','park','Nairobi','Kenya','verified',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Exec(`INSERT INTO experiences(id,destination_id,title,description,duration_minutes,difficulty,experience_type,approved_content,status) VALUES('approved-exp','verified-place','Quiet listen','Use the published public path only.',15,'beginner','mindfulness','Use the marked public path; do not enter closed areas.', 'approved')`); err != nil {
		t.Fatal(err)
	}
	model := &fakeModel{answers: []string{sampleJSON("A safe destination adventure")}}
	service := Service{DB: store, Model: model, ModelAvailable: func(context.Context) bool { return true }}
	req := request()
	req.Environment = "approved_nature_destination"
	req.DestinationID = "verified-place"
	created, err := service.Generate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if created.DestinationID != "verified-place" || !strings.Contains(model.prompts[0], "Use the marked public path") {
		t.Fatalf("destination guidance missing: %+v prompt=%q", created, model.prompts)
	}
	loaded, err := service.Get(context.Background(), created.ID)
	if err != nil || loaded.DestinationID != created.DestinationID {
		t.Fatalf("destination not persistent: %+v %v", loaded, err)
	}
	req.DestinationID = "karura-concept"
	if _, err := service.Generate(context.Background(), req); err == nil {
		t.Fatal("unverified concept was accepted as an approved destination")
	}
}
