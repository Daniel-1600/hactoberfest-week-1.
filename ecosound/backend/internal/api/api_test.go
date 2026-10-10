package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ecosound/backend/internal/db"
	"ecosound/backend/internal/inference"
	"ecosound/backend/internal/models"
	"ecosound/backend/internal/storage"
)

type fixtureIdentifier struct {
	result models.InferenceResult
	err    error
}

func (f fixtureIdentifier) Identify(_ context.Context, path string) (models.InferenceResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return models.InferenceResult{}, err
	}
	if len(data) < 44 || string(data[:4]) != "RIFF" {
		return models.InferenceResult{}, errors.New("not decoded WAV")
	}
	return f.result, f.err
}

func testWAV() []byte {
	// One second of mono 16-bit PCM at 8 kHz.
	data := make([]byte, 44+16000)
	copy(data[:4], "RIFF")
	put32 := func(at, v int) {
		for i := 0; i < 4; i++ {
			data[at+i] = byte(v >> (8 * i))
		}
	}
	put16 := func(at, v int) { data[at], data[at+1] = byte(v), byte(v>>8) }
	put32(4, len(data)-8)
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	put32(16, 16)
	put16(20, 1)
	put16(22, 1)
	put32(24, 8000)
	put32(28, 16000)
	put16(32, 2)
	put16(34, 16)
	copy(data[36:40], "data")
	put32(40, 16000)
	return data
}

func request(t *testing.T, handler http.Handler, method, path, contentType string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func upload(t *testing.T, handler http.Handler, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("audio", "clip.wav")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return request(t, handler, "POST", "/api/identify", writer.FormDataContentType(), body.Bytes())
}

func decode[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("bad JSON %q: %v", response.Body.String(), err)
	}
	return result
}

func TestIdentifySaveRetrieveConfirmAndQuest(t *testing.T) {
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "ecosound.db")
	store, err := db.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	audio, err := storage.New(filepath.Join(directory, "audio"))
	if err != nil {
		t.Fatal(err)
	}
	zone, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC)
	start, end := 0.0, 1.0
	fake := fixtureIdentifier{result: models.InferenceResult{Model: "birdnet", ModelVersion: "3.0", Predictions: []models.Prediction{
		{ScientificName: "Turdus merula", CommonName: "Common Blackbird", RawSpeciesName: "Turdus merula_Common Blackbird", Confidence: 0.8123456789, StartTime: &start, EndTime: &end},
		{ScientificName: "Passer domesticus", CommonName: "House Sparrow", RawSpeciesName: "Passer domesticus_House Sparrow", Confidence: 0.7, StartTime: &start, EndTime: &end},
	}}}
	server, err := New(Config{DB: store, Storage: audio, Identifier: fake, QuestTimezone: zone, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.Handler()
	identified := upload(t, handler, testWAV())
	if identified.Code != 200 {
		t.Fatalf("identify %d: %s", identified.Code, identified.Body.String())
	}
	result := decode[models.Identification](t, identified)
	if result.RecordingID == "" || len(result.Predictions) != 2 || result.Predictions[0].Confidence != 0.8123456789 {
		t.Fatalf("unexpected identification: %+v", result)
	}
	if result.Predictions[0].UserConfirmed {
		t.Fatal("model candidate began confirmed")
	}
	if result.AudioURL != db.AudioURL(result.RecordingID) {
		t.Fatalf("wrong audio URL: %s", result.AudioURL)
	}
	if response := request(t, handler, "GET", result.AudioURL, "", nil); response.Code != 200 || !bytes.HasPrefix(response.Body.Bytes(), []byte("RIFF")) {
		t.Fatalf("stored audio unavailable: %d", response.Code)
	}

	before := decode[models.Quest](t, request(t, handler, "GET", "/api/quests/today", "", nil))
	if before.Tasks[0].Current != 0 {
		t.Fatalf("unsaved candidate earned quest credit: %+v", before)
	}
	savePath := "/api/discoveries"
	save := func(id int64) *httptest.ResponseRecorder {
		return request(t, handler, "POST", savePath, "application/json", []byte(fmt.Sprintf(`{"prediction_id":%d}`, id)))
	}
	saved := save(result.Predictions[0].ID)
	if saved.Code != 201 {
		t.Fatalf("save %d: %s", saved.Code, saved.Body.String())
	}
	discovery := decode[models.Discovery](t, saved)
	if discovery.ObservedAt != db.Timestamp(now) || discovery.PredictionID != result.Predictions[0].ID {
		t.Fatalf("wrong discovery: %+v", discovery)
	}
	for _, candidate := range result.Predictions {
		duplicate := save(candidate.ID)
		if duplicate.Code != 409 || !strings.Contains(duplicate.Body.String(), "duplicate_discovery") {
			t.Fatalf("second save %d: %s", duplicate.Code, duplicate.Body.String())
		}
	}
	quest := decode[models.Quest](t, request(t, handler, "GET", "/api/quests/today", "", nil))
	if quest.Tasks[0].Current != 1 || quest.Tasks[1].Current != 1 {
		t.Fatalf("wrong quest after save: %+v", quest)
	}
	confirmPath := fmt.Sprintf("/api/discoveries/%d/confirm", discovery.ID)
	for i := 0; i < 2; i++ {
		confirmed := request(t, handler, "POST", confirmPath, "", nil)
		if confirmed.Code != 200 || !decode[models.Discovery](t, confirmed).UserConfirmed {
			t.Fatalf("confirmation %d: %s", confirmed.Code, confirmed.Body.String())
		}
	}
	list := decode[struct {
		Discoveries []models.Discovery `json:"discoveries"`
	}](t, request(t, handler, "GET", savePath, "", nil))
	if len(list.Discoveries) != 1 || !list.Discoveries[0].UserConfirmed {
		t.Fatalf("wrong collection: %+v", list)
	}
	var score float64
	if err := store.QueryRow("SELECT confidence FROM predictions WHERE id=?", result.Predictions[0].ID).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != 0.8123456789 {
		t.Fatalf("confirmation changed model score: %v", score)
	}
	if _, err := store.Exec("UPDATE predictions SET confidence=0.1 WHERE id=?", result.Predictions[0].ID); err == nil {
		t.Fatal("model prediction was mutable")
	}
	if _, err := store.Exec("INSERT INTO discoveries(prediction_id,recording_id,observed_at,created_at) VALUES(999,'missing',?,?)", db.Timestamp(now), db.Timestamp(now)); err == nil {
		t.Fatal("discovery foreign key was not enforced")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := db.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, err := reopened.Discoveries(context.Background())
	if err != nil || len(persisted) != 1 || !persisted[0].UserConfirmed {
		t.Fatalf("discovery did not survive restart: %+v, %v", persisted, err)
	}
}

func TestUploadValidationAndInferenceFailure(t *testing.T) {
	directory := t.TempDir()
	store, err := db.Open(filepath.Join(directory, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	audio, err := storage.New(filepath.Join(directory, "audio"))
	if err != nil {
		t.Fatal(err)
	}
	newServer := func(identifier fixtureIdentifier, max int64) http.Handler {
		t.Helper()
		server, err := New(Config{DB: store, Storage: audio, Identifier: identifier, QuestTimezone: time.UTC, MaxUploadBytes: max})
		if err != nil {
			t.Fatal(err)
		}
		return server.Handler()
	}
	handler := newServer(fixtureIdentifier{}, 1<<20)
	if response := request(t, handler, "POST", "/api/identify", "application/json", []byte("{}")); response.Code != 400 {
		t.Fatalf("malformed multipart: %d", response.Code)
	}
	if response := upload(t, handler, []byte("not audio")); response.Code != 415 {
		t.Fatalf("unsupported upload: %d %s", response.Code, response.Body.String())
	}
	if response := upload(t, newServer(fixtureIdentifier{}, 100), testWAV()); response.Code != 413 {
		t.Fatalf("oversized upload: %d %s", response.Code, response.Body.String())
	}
	if response := request(t, handler, "POST", "/api/discoveries", "application/json", []byte(`{"prediction_id":999}`)); response.Code != 404 {
		t.Fatalf("unknown prediction: %d", response.Code)
	}
	for _, tc := range []struct {
		err    error
		status int
	}{{inference.ErrUnavailable, 503}, {inference.ErrMalformed, 502}, {inference.ErrTimeout, 504}} {
		response := upload(t, newServer(fixtureIdentifier{err: tc.err}, 1<<20), testWAV())
		if response.Code != tc.status {
			t.Fatalf("inference %v: %d %s", tc.err, response.Code, response.Body.String())
		}
	}
	entries, err := os.ReadDir(audio.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed inference leaked audio: %+v", entries)
	}
}

func TestEmptyModelResultIsSuccessfulWithoutQuestCredit(t *testing.T) {
	directory := t.TempDir()
	store, err := db.Open(filepath.Join(directory, "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	audio, err := storage.New(filepath.Join(directory, "audio"))
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(Config{DB: store, Storage: audio, Identifier: fixtureIdentifier{result: models.InferenceResult{Model: "birdnet", ModelVersion: "3.0", Predictions: []models.Prediction{}}}, QuestTimezone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	response := upload(t, server.Handler(), testWAV())
	if response.Code != 200 {
		t.Fatalf("valid no-detection response: %d %s", response.Code, response.Body.String())
	}
	result := decode[models.Identification](t, response)
	if result.Predictions == nil || len(result.Predictions) != 0 || result.RecordingID == "" {
		t.Fatalf("unexpected no-detection result: %+v", result)
	}
	quest := decode[models.Quest](t, request(t, server.Handler(), "GET", "/api/quests/today", "", nil))
	if quest.Tasks[0].Current != 0 || quest.Tasks[1].Current != 0 {
		t.Fatalf("no detection earned credit: %+v", quest)
	}
}
