package gamification

import (
	"context"
	"database/sql"
	"time"
)

type Stats struct {
	AdventureXP                int `json:"adventure_xp"`
	SpeciesDiscovered          int `json:"species_discovered"`
	RecordingsCollected        int `json:"recordings_collected"`
	AdventuresStarted          int `json:"adventures_started"`
	AdventuresCompleted        int `json:"adventures_completed"`
	SelfReportedOutdoorMinutes int `json:"self_reported_outdoor_minutes"`
	MeasuredOutdoorMinutes     int `json:"measured_outdoor_minutes"`
	ObservationContributions   int `json:"observation_contributions"`
	ExplorationStreakDays      int `json:"exploration_streak_days"`
}
type Badge struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Earned      bool    `json:"earned"`
	EarnedAt    *string `json:"earned_at,omitempty"`
}

func StatsFor(ctx context.Context, database *sql.DB, zone *time.Location, now time.Time) (Stats, error) {
	var out Stats
	err := database.QueryRowContext(ctx, `SELECT coalesce((SELECT sum(points) FROM xp_events),0),coalesce((SELECT count(DISTINCT p.scientific_name) FROM discoveries d JOIN predictions p ON p.id=d.prediction_id WHERE trim(p.scientific_name)!=''),0),coalesce((SELECT count(DISTINCT recording_id) FROM discoveries),0),coalesce((SELECT count(*) FROM adventure_progress WHERE started_at IS NOT NULL),0),coalesce((SELECT count(*) FROM adventure_progress WHERE status='completed'),0),coalesce((SELECT sum(self_reported_minutes) FROM adventure_progress),0),coalesce((SELECT count(*) FROM observations WHERE visibility='public'),0)`).Scan(&out.AdventureXP, &out.SpeciesDiscovered, &out.RecordingsCollected, &out.AdventuresStarted, &out.AdventuresCompleted, &out.SelfReportedOutdoorMinutes, &out.ObservationContributions)
	if err != nil {
		return out, err
	}
	days := map[string]bool{}
	rows, err := database.QueryContext(ctx, `SELECT completed_at FROM adventure_progress WHERE completed_at IS NOT NULL`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return out, err
		}
		stamp, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			continue
		}
		days[stamp.In(zone).Format("2006-01-02")] = true
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	day := now.In(zone)
	if !days[day.Format("2006-01-02")] {
		day = day.AddDate(0, 0, -1)
	}
	for days[day.Format("2006-01-02")] {
		out.ExplorationStreakDays++
		day = day.AddDate(0, 0, -1)
	}
	return out, nil
}
func Badges(ctx context.Context, database *sql.DB) ([]Badge, error) {
	rows, err := database.QueryContext(ctx, `SELECT b.id,b.name,b.description,e.earned_at FROM badges b LEFT JOIN earned_badges e ON e.badge_id=b.id ORDER BY b.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Badge{}
	for rows.Next() {
		var item Badge
		var earned sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &earned); err != nil {
			return nil, err
		}
		item.Earned = earned.Valid
		if earned.Valid {
			item.EarnedAt = &earned.String
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
