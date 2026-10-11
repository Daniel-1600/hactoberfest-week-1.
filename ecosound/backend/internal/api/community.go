package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"ecosound/backend/internal/destinations"
	"ecosound/backend/internal/gamification"
	"ecosound/backend/internal/observations"
)

func (s *Server) createObservation(w http.ResponseWriter, r *http.Request) {
	var input observations.Input
	if !readJSON(w, r, &input) {
		return
	}
	input.IdempotencyKey = r.Header.Get("Idempotency-Key")
	if input.IdempotencyKey != "" && !idempotencyKey.MatchString(input.IdempotencyKey) {
		fail(w, 400, "invalid_idempotency_key", "Use a valid idempotency key.")
		return
	}
	precision := 2
	if s.config.ObservationLocationPrecision == "very_coarse" {
		precision = 1
	}
	item, err := observations.CreateWithPrecision(r.Context(), s.config.DB, input, s.config.Now(), precision)
	if errors.Is(err, observations.ErrInvalid) {
		fail(w, 422, "invalid_observation", "Confirm the sharing consent and provide valid optional location details.")
		return
	}
	if errors.Is(err, observations.ErrNotFound) {
		fail(w, 404, "discovery_not_found", "The saved discovery or destination was not found.")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 201, item)
}

func (s *Server) observationFilter(r *http.Request) (observations.Filter, error) {
	q := r.URL.Query()
	filter := observations.Filter{Species: strings.TrimSpace(q.Get("species")), Status: q.Get("status"), Destination: q.Get("destination"), Since: q.Get("since"), Until: q.Get("until")}
	if len(filter.Species) > 120 || len(filter.Destination) > 80 {
		return filter, errors.New("filter too long")
	}
	return filter, observations.ValidateFilters(filter)
}
func (s *Server) listObservations(w http.ResponseWriter, r *http.Request) {
	filter, err := s.observationFilter(r)
	if err != nil {
		fail(w, 400, "invalid_filter", "Use a valid species, status, destination, and RFC3339 date filter.")
		return
	}
	items, err := observations.List(r.Context(), s.config.DB, filter, false)
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, map[string]any{"observations": items})
}
func (s *Server) getObservation(w http.ResponseWriter, r *http.Request) {
	item, err := observations.Get(r.Context(), s.config.DB, r.PathValue("id"))
	if errors.Is(err, observations.ErrNotFound) {
		fail(w, 404, "observation_not_found", "This observation does not exist.")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, item)
}
func (s *Server) mapObservations(w http.ResponseWriter, r *http.Request) {
	if !s.config.PublicMapEnabled {
		fail(w, 503, "map_disabled", "Public observations are disabled by the server.")
		return
	}
	filter, err := s.observationFilter(r)
	if err != nil {
		fail(w, 400, "invalid_filter", "Use a valid map filter.")
		return
	}
	items, err := observations.List(r.Context(), s.config.DB, filter, true)
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, map[string]any{"observations": items, "location_precision": s.config.ObservationLocationPrecision, "note": "Approximate public locations only. Personal confirmation is not scientific verification."})
}

func (s *Server) listDestinations(w http.ResponseWriter, r *http.Request) {
	items, err := destinations.List(r.Context(), s.config.DB.DB)
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, map[string]any{"destinations": items})
}
func (s *Server) getDestination(w http.ResponseWriter, r *http.Request) {
	item, err := destinations.Get(r.Context(), s.config.DB.DB, r.PathValue("id"))
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "destination_not_found", "This destination does not exist.")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, item)
}
func (s *Server) destinationTrails(w http.ResponseWriter, r *http.Request) {
	items, err := destinations.Trails(r.Context(), s.config.DB.DB, r.PathValue("id"))
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, map[string]any{"trails": items})
}
func (s *Server) destinationExperiences(w http.ResponseWriter, r *http.Request) {
	items, err := destinations.Experiences(r.Context(), s.config.DB.DB, r.PathValue("id"))
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, map[string]any{"experiences": items})
}

func (s *Server) profileStats(w http.ResponseWriter, r *http.Request) {
	stats, err := gamification.StatsFor(r.Context(), s.config.DB.DB, s.config.QuestTimezone, s.config.Now())
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, stats)
}
func (s *Server) profileBadges(w http.ResponseWriter, r *http.Request) {
	badges, err := gamification.Badges(r.Context(), s.config.DB.DB)
	if err != nil {
		internal(w, err)
		return
	}
	reply(w, 200, map[string]any{"badges": badges})
}
