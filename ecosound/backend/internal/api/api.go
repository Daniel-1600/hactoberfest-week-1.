package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"ecosound/backend/internal/adventures"
	"ecosound/backend/internal/ai/llm"
	"ecosound/backend/internal/ai/retrieval"
	"ecosound/backend/internal/db"
	"ecosound/backend/internal/inference"
	"ecosound/backend/internal/quests"
	"ecosound/backend/internal/storage"
)

type Config struct {
	DB                           *db.Store
	Storage                      *storage.Store
	Identifier                   inference.Identifier
	QuestTimezone                *time.Location
	Origin                       string
	MaxUploadBytes               int64
	Now                          func() time.Time
	Adventures                   adventures.Service
	Qwen                         llm.Client
	QwenEnabled                  bool
	NatureLMEnabled              bool
	Education                    retrieval.Service
	PublicMapEnabled             bool
	ObservationLocationPrecision string
	NatureLM                     inference.NatureLMProvider
}

type Server struct {
	config   Config
	mux      *http.ServeMux
	analysis chan struct{}
	syncMu   sync.Mutex
}

func New(c Config) (*Server, error) {
	if c.DB == nil || c.Storage == nil || c.Identifier == nil || c.QuestTimezone == nil {
		return nil, errors.New("missing server configuration")
	}
	if c.MaxUploadBytes <= 0 {
		c.MaxUploadBytes = 15 << 20
	}
	if c.Origin == "" {
		c.Origin = "http://localhost:3000"
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.ObservationLocationPrecision == "" {
		c.ObservationLocationPrecision = "coarse"
	}
	if c.ObservationLocationPrecision != "coarse" && c.ObservationLocationPrecision != "very_coarse" {
		return nil, errors.New("observation location precision must be coarse or very_coarse")
	}
	if c.Adventures.DB == nil {
		c.Adventures.DB = c.DB
	}
	if c.Education.DB == nil {
		c.Education.DB = c.DB
	}
	if c.QwenEnabled {
		c.Adventures.Model = c.Qwen
		c.Adventures.ModelAvailable = c.Qwen.Available
		c.Education.Model = c.Qwen
		c.Education.Enabled = true
	}
	c.NatureLM.Enabled = c.NatureLMEnabled
	if c.Adventures.Now == nil {
		c.Adventures.Now = c.Now
	}
	s := &Server{config: c, mux: http.NewServeMux(), analysis: make(chan struct{}, 1)}
	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("POST /api/identify", s.identify)
	s.mux.HandleFunc("POST /api/discoveries", s.save)
	s.mux.HandleFunc("GET /api/discoveries", s.list)
	s.mux.HandleFunc("POST /api/discoveries/{id}/confirm", s.confirm)
	s.mux.HandleFunc("GET /api/quests/today", s.quest)
	s.mux.HandleFunc("GET /api/recordings/{id}/audio", s.audio)
	s.mux.HandleFunc("POST /api/recordings/{id}/naturelm", s.describeRecording)
	s.mux.HandleFunc("POST /api/adventures/generate", s.generateAdventure)
	s.mux.HandleFunc("GET /api/adventures", s.listAdventures)
	s.mux.HandleFunc("GET /api/adventures/{id}", s.getAdventure)
	s.mux.HandleFunc("POST /api/adventures/{id}/start", s.startAdventure)
	s.mux.HandleFunc("POST /api/adventures/{id}/tasks/{taskId}/complete", s.completeAdventureTask)
	s.mux.HandleFunc("POST /api/adventures/{id}/finish", s.finishAdventure)
	s.mux.HandleFunc("GET /api/inference/providers", s.providers)
	s.mux.HandleFunc("GET /api/species/{scientificName...}", s.species)
	s.mux.HandleFunc("POST /api/discoveries/{id}/explain", s.explain)
	s.mux.HandleFunc("POST /api/observations", s.createObservation)
	s.mux.HandleFunc("GET /api/observations", s.listObservations)
	s.mux.HandleFunc("GET /api/observations/{id}", s.getObservation)
	s.mux.HandleFunc("GET /api/map/observations", s.mapObservations)
	s.mux.HandleFunc("GET /api/destinations", s.listDestinations)
	s.mux.HandleFunc("GET /api/destinations/{id}", s.getDestination)
	s.mux.HandleFunc("GET /api/destinations/{id}/trails", s.destinationTrails)
	s.mux.HandleFunc("GET /api/destinations/{id}/experiences", s.destinationExperiences)
	s.mux.HandleFunc("GET /api/profile/stats", s.profileStats)
	s.mux.HandleFunc("GET /api/profile/badges", s.profileBadges)
	s.mux.HandleFunc("POST /api/sync", s.syncOperations)
	return s, nil
}

// Qwen adventures can check availability and make two generation attempts.
const QwenRequestTimeout = 3*llm.DefaultTimeout + 60*time.Second

func requestTimeout(pattern string) time.Duration {
	switch pattern {
	case "POST /api/adventures/generate", "GET /api/inference/providers", "POST /api/discoveries/{id}/explain":
		return QwenRequestTimeout
	default:
		return 240 * time.Second
	}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		origin := r.Header.Get("Origin")
		w.Header().Add("Vary", "Origin")
		if origin != "" {
			if origin != s.config.Origin {
				fail(w, 403, "origin_not_allowed", "This frontend origin is not allowed.")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Keep unknown routes/methods in the same JSON error contract.
		handler, pattern := s.mux.Handler(r)
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout(pattern))
		defer cancel()
		if pattern == "" {
			// ServeMux distinguishes method errors; encode its status explicitly.
			if r.URL.Path == "/api/identify" || r.URL.Path == "/api/discoveries" || r.URL.Path == "/api/quests/today" || r.URL.Path == "/health" {
				fail(w, 405, "method_not_allowed", "This HTTP method is not supported.")
			} else {
				fail(w, 404, "not_found", "This endpoint does not exist.")
			}
			return
		}
		// ServeMux.ServeHTTP populates route PathValue fields.
		_ = handler
		s.mux.ServeHTTP(w, r.WithContext(ctx))
	})
}

func reply(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("write response", "error", err)
	}
}

var idempotencyKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,99}$`)

func fail(w http.ResponseWriter, status int, code, message string) {
	reply(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func internal(w http.ResponseWriter, err error) {
	slog.Error("API database failure", "error", err)
	fail(w, 500, "internal_error", "Could not complete the request. Please try again.")
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.config.DB.PingContext(r.Context()); err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, map[string]string{"status": "ok", "inference": "configured"})
}

func (s *Server) identify(w http.ResponseWriter, r *http.Request) {
	select {
	case s.analysis <- struct{}{}:
		defer func() { <-s.analysis }()
	default:
		fail(w, 429, "inference_busy", "Another recording is being analyzed. Please try again shortly.")
		return
	}
	recordedAt := s.config.Now()
	// Include bounded multipart overhead, then independently enforce the audio limit.
	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadBytes+(64<<10))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			fail(w, 413, "upload_too_large", "Choose an audio file smaller than 15 MiB.")
		} else {
			fail(w, 400, "invalid_upload", "Upload one audio file using the multipart audio field.")
		}
		return
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["audio"]
	if len(files) != 1 || len(r.MultipartForm.File) != 1 {
		fail(w, 400, "invalid_upload", "Upload exactly one file in the audio field.")
		return
	}
	header := files[0]
	if header.Size > s.config.MaxUploadBytes {
		fail(w, 413, "upload_too_large", "Choose an audio file smaller than 15 MiB.")
		return
	}
	if header.Size == 0 {
		fail(w, 422, "invalid_audio", "The recording is empty. Please record again.")
		return
	}
	file, err := header.Open()
	if err != nil {
		internal(w, err)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, s.config.MaxUploadBytes+1))
	if err != nil {
		internal(w, err)
		return
	}
	if int64(len(data)) > s.config.MaxUploadBytes {
		fail(w, 413, "upload_too_large", "Choose an audio file smaller than 15 MiB.")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	hashBytes := sha256.Sum256(data)
	contentHash := hex.EncodeToString(hashBytes[:])
	if key != "" {
		if !idempotencyKey.MatchString(key) {
			fail(w, 400, "invalid_idempotency_key", "Provide a valid idempotency key.")
			return
		}
		existing, found, lookupErr := s.config.DB.IdentificationByKey(r.Context(), key, contentHash)
		if errors.Is(lookupErr, db.ErrIdempotencyConflict) {
			fail(w, 409, "idempotency_conflict", "This operation key was already used with different audio.")
			return
		}
		if lookupErr != nil {
			internal(w, lookupErr)
			return
		}
		if found {
			reply(w, 200, existing)
			return
		}
	}
	audio, err := s.config.Storage.Decode(r.Context(), bytes.NewReader(data))
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrUnsupported):
			fail(w, 415, "unsupported_audio", "Use WAV, WebM, MP4/M4A, Ogg, MP3, or FLAC audio.")
		case errors.Is(err, storage.ErrInvalid):
			fail(w, 422, "invalid_audio", "Could not decode audio. Choose a valid recording between 1 and 60 seconds.")
		case errors.Is(err, storage.ErrTools):
			fail(w, 503, "audio_decoder_unavailable", "Audio decoding is unavailable. Install FFmpeg and FFprobe on the server.")
		default:
			internal(w, err)
		}
		return
	}
	saved := false
	defer func() {
		if !saved {
			if err := os.Remove(audio.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				slog.Error("clean recording", "error", err)
			}
		}
	}()
	result, err := s.config.Identifier.Identify(r.Context(), audio.Path)
	if err != nil {
		switch {
		case errors.Is(err, inference.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
			fail(w, 504, "inference_timeout", "BirdNET analysis timed out. Try a shorter recording; pre-download the model before your first demo.")
		case errors.Is(err, inference.ErrMalformed):
			fail(w, 502, "invalid_inference_result", "BirdNET returned an unexpected result. Check the server diagnostics.")
		default:
			fail(w, 503, "inference_unavailable", "BirdNET could not analyze this recording. Check the Python environment and model downloads on the server.")
		}
		return
	}
	out, err := s.config.DB.SaveIdentificationIdempotent(r.Context(), audio.ID, audio.Path, audio.Duration, recordedAt, result, key, contentHash)
	if err != nil {
		internal(w, err)
		return
	}
	saved = true
	reply(w, 200, out)
}

func readJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		fail(w, 400, "invalid_json", "Send a valid JSON request with only the required fields.")
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		fail(w, 400, "invalid_json", "Send exactly one JSON object.")
		return false
	}
	return true
}

func (s *Server) save(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PredictionID int64 `json:"prediction_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.PredictionID <= 0 {
		fail(w, 400, "invalid_prediction", "Choose a prediction returned by BirdNET.")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key != "" && !idempotencyKey.MatchString(key) {
		fail(w, 400, "invalid_idempotency_key", "Provide a valid idempotency key.")
		return
	}
	discovery, err := s.config.DB.SaveDiscoveryIdempotent(r.Context(), body.PredictionID, s.config.Now(), key)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			fail(w, 404, "prediction_not_found", "This prediction does not exist.")
		case errors.Is(err, db.ErrDuplicate):
			fail(w, 409, "duplicate_discovery", "You already saved a discovery from this recording. Each distinct recording counts once.")
		case errors.Is(err, db.ErrIdempotencyConflict):
			fail(w, 409, "idempotency_conflict", "This operation key was already used for a different prediction.")
		default:
			internal(w, err)
		}
		return
	}
	reply(w, 201, discovery)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	discoveries, err := s.config.DB.Discoveries(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, map[string]any{"discoveries": discoveries})
}

func (s *Server) confirm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, 404, "discovery_not_found", "This discovery does not exist.")
		return
	}
	d, err := s.config.DB.Confirm(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "discovery_not_found", "This discovery does not exist.")
		} else {
			internal(w, err)
		}
		return
	}
	reply(w, 200, d)
}

func (s *Server) quest(w http.ResponseWriter, r *http.Request) {
	q, err := quests.Today(r.Context(), s.config.DB, s.config.QuestTimezone, s.config.Now())
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, q)
}

func (s *Server) audio(w http.ResponseWriter, r *http.Request) {
	var path string
	err := s.config.DB.QueryRowContext(r.Context(), "SELECT file_path FROM recordings WHERE id=?", r.PathValue("id")).Scan(&path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "recording_not_found", "This recording does not exist.")
		} else {
			internal(w, err)
		}
		return
	}
	// Database paths are generated by the server. Validate containment defensively.
	relative, err := filepath.Rel(s.config.Storage.Dir, path)
	if err != nil || relative != r.PathValue("id")+".wav" {
		internal(w, fmt.Errorf("invalid stored recording path"))
		return
	}
	file, err := os.Open(path)
	if err != nil {
		fail(w, 404, "recording_not_found", "This recording is no longer available.")
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		internal(w, err)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	http.ServeContent(w, r, "recording.wav", stat.ModTime(), file)
}
