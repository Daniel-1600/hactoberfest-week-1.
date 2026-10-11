package api

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"

	"ecosound/backend/internal/ai/retrieval"
)

func (s *Server) species(w http.ResponseWriter, r *http.Request) {
	name, err := url.PathUnescape(r.PathValue("scientificName"))
	if err != nil || len(name) > 120 {
		fail(w, 400, "invalid_species_name", "Provide a valid scientific name.")
		return
	}
	item, err := s.config.Education.Lookup(r.Context(), name)
	if errors.Is(err, retrieval.ErrNotFound) {
		reply(w, 404, map[string]any{"available": false, "message": "No curated species information is available for this identification yet."})
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, item)
}

func (s *Server) explain(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, 404, "discovery_not_found", "This discovery does not exist.")
		return
	}
	result, err := s.config.Education.Explain(r.Context(), id)
	if errors.Is(err, retrieval.ErrNotFound) {
		reply(w, 404, map[string]any{"available": false, "message": "No trusted species information is available for this identification yet."})
		return
	}
	if errors.Is(err, retrieval.ErrModelUnavailable) {
		fail(w, 503, "education_unavailable", "Curated facts are available, but the local Qwen model is not ready to write an explanation.")
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "discovery_not_found", "This discovery does not exist.")
		return
	}
	if err != nil {
		fail(w, 502, "invalid_explanation", "The local model returned an invalid explanation.")
		return
	}
	reply(w, 200, result)
}

func (s *Server) describeRecording(w http.ResponseWriter, r *http.Request) {
	if !s.config.NatureLM.Available(r.Context()) {
		fail(w, 503, "naturelm_unavailable", "NatureLM-audio is not installed or its optional local model server is unavailable.")
		return
	}
	var path string
	if err := s.config.DB.QueryRowContext(r.Context(), "SELECT file_path FROM recordings WHERE id=?", r.PathValue("id")).Scan(&path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "recording_not_found", "This recording does not exist.")
		} else {
			internal(w, err)
		}
		return
	}
	relative, err := filepath.Rel(s.config.Storage.Dir, path)
	if err != nil || relative != r.PathValue("id")+".wav" {
		internal(w, errors.New("invalid stored recording path"))
		return
	}
	var body struct {
		Task string `json:"task"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	result, err := s.config.NatureLM.Predict(r.Context(), path, body.Task)
	if err != nil {
		fail(w, 502, "naturelm_failed", "NatureLM-audio could not describe this recording.")
		return
	}
	reply(w, 200, result)
}
