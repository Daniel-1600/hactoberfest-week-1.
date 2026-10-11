package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"ecosound/backend/internal/adventures"
)

func (s *Server) generateAdventure(w http.ResponseWriter, r *http.Request) {
	var request adventures.Request
	if !readJSON(w, r, &request) {
		return
	}
	created, err := s.config.Adventures.Generate(r.Context(), request)
	if err != nil {
		if errors.Is(err, adventures.ErrPreferences) {
			fail(w, 400, "invalid_preferences", "Choose a supported duration, interest, environment, difficulty, and verified destination when required.")
		} else {
			internal(w, err)
		}
		return
	}
	reply(w, http.StatusCreated, created)
}

func (s *Server) listAdventures(w http.ResponseWriter, r *http.Request) {
	items, err := s.config.Adventures.List(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, map[string]any{"adventures": items})
}
func (s *Server) getAdventure(w http.ResponseWriter, r *http.Request) {
	a, err := s.config.Adventures.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, adventures.ErrNotFound) {
		fail(w, 404, "adventure_not_found", "This adventure does not exist.")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, a)
}
func (s *Server) startAdventure(w http.ResponseWriter, r *http.Request) {
	a, err := s.config.Adventures.Start(r.Context(), r.PathValue("id"), s.config.Now())
	if errors.Is(err, adventures.ErrNotFound) {
		fail(w, 404, "adventure_not_found", "This adventure does not exist.")
		return
	}
	if errors.Is(err, adventures.ErrConflict) {
		fail(w, 409, "adventure_not_active", "This adventure is already finished.")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, a)
}
func (s *Server) completeAdventureTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DiscoveryID  int64 `json:"discovery_id"`
		SelfReported bool  `json:"self_reported"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	a, err := s.config.Adventures.CompleteTask(r.Context(), r.PathValue("id"), r.PathValue("taskId"), body.DiscoveryID, body.SelfReported, s.config.Now())
	if errors.Is(err, adventures.ErrNotFound) {
		fail(w, 404, "task_not_found", "This adventure task does not exist.")
		return
	}
	if errors.Is(err, adventures.ErrConflict) {
		fail(w, 409, "task_already_completed", "This task is complete or the adventure is not active.")
		return
	}
	if err != nil {
		fail(w, 422, "task_not_eligible", err.Error())
		return
	}
	reply(w, 200, a)
}
func (s *Server) finishAdventure(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SelfReportedMinutes *int   `json:"self_reported_minutes"`
		Discovered          string `json:"what_did_you_discover,omitempty"`
		Surprised           string `json:"what_surprised_you,omitempty"`
		Again               *bool  `json:"would_explore_again,omitempty"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	a, err := s.config.Adventures.Finish(r.Context(), r.PathValue("id"), body.SelfReportedMinutes, body.Discovered, body.Surprised, body.Again, s.config.Now())
	if errors.Is(err, adventures.ErrNotFound) {
		fail(w, 404, "adventure_not_found", "This adventure does not exist.")
		return
	}
	if errors.Is(err, adventures.ErrConflict) {
		fail(w, 409, "adventure_not_active", "This adventure is not active.")
		return
	}
	if err != nil {
		fail(w, 422, "adventure_incomplete", err.Error())
		return
	}
	reply(w, 200, a)
}
func (s *Server) providers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	qwenAvailable := s.config.QwenEnabled && s.config.Qwen.Available(ctx)
	reply(w, 200, map[string]any{"providers": []any{
		map[string]any{"name": "birdnet", "configured": s.config.Identifier != nil, "available": s.config.Identifier != nil, "supported_tasks": []string{"bird_species_classification", "segment_timestamps"}, "note": "Configured process; a real model call is required to verify weights and runtime."},
		map[string]any{"name": "qwen3", "configured": s.config.QwenEnabled, "available": qwenAvailable, "supported_tasks": []string{"outdoor_adventure_generation", "grounded_species_explanation"}, "model": s.config.Qwen.Model},
		map[string]any{"name": "naturelm-audio", "configured": s.config.NatureLMEnabled, "available": s.config.NatureLM.Available(ctx), "supported_tasks": s.config.NatureLM.SupportedTasks(), "note": "Text descriptions are experimental and are not verified species identifications."},
	}})
}
