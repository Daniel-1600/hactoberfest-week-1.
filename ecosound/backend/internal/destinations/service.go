package destinations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type Destination struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        string       `json:"type"`
	Region      string       `json:"region"`
	Country     string       `json:"country"`
	Website     string       `json:"website,omitempty"`
	Status      string       `json:"status"`
	Experiences []Experience `json:"experiences,omitempty"`
	Trails      []Trail      `json:"trails,omitempty"`
}
type Experience struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	DurationMinutes int     `json:"duration_minutes"`
	Difficulty      string  `json:"difficulty"`
	Type            string  `json:"type"`
	Status          string  `json:"status"`
	TrailID         *string `json:"trail_id,omitempty"`
}
type Trail struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	DistanceKM         *float64        `json:"distance_km,omitempty"`
	DurationMinutes    *int            `json:"estimated_duration_minutes,omitempty"`
	Difficulty         string          `json:"difficulty"`
	Geometry           json.RawMessage `json:"geometry_geojson,omitempty"`
	AccessibilityNotes string          `json:"accessibility_notes"`
}

func List(ctx context.Context, database *sql.DB) ([]Destination, error) {
	rows, err := database.QueryContext(ctx, `SELECT id,name,description,type,region,country,website,status FROM destinations WHERE status IN ('verified','concept') ORDER BY CASE status WHEN 'verified' THEN 0 ELSE 1 END,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Destination{}
	for rows.Next() {
		var item Destination
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Type, &item.Region, &item.Country, &item.Website, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func Get(ctx context.Context, database *sql.DB, id string) (Destination, error) {
	var item Destination
	err := database.QueryRowContext(ctx, `SELECT id,name,description,type,region,country,website,status FROM destinations WHERE id=? AND status IN ('verified','concept')`, id).Scan(&item.ID, &item.Name, &item.Description, &item.Type, &item.Region, &item.Country, &item.Website, &item.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return item, sql.ErrNoRows
	}
	if err != nil {
		return item, err
	}
	item.Experiences, err = Experiences(ctx, database, id)
	if err != nil {
		return item, err
	}
	item.Trails, err = Trails(ctx, database, id)
	return item, err
}
func Experiences(ctx context.Context, database *sql.DB, id string) ([]Experience, error) {
	rows, err := database.QueryContext(ctx, `SELECT id,title,description,duration_minutes,difficulty,experience_type,status,trail_id FROM experiences WHERE destination_id=? AND status IN ('approved','concept') AND (trail_id IS NULL OR EXISTS(SELECT 1 FROM trails t WHERE t.id=experiences.trail_id AND t.status='verified')) ORDER BY title`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Experience{}
	for rows.Next() {
		var item Experience
		var trail sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.DurationMinutes, &item.Difficulty, &item.Type, &item.Status, &trail); err != nil {
			return nil, err
		}
		if trail.Valid {
			item.TrailID = &trail.String
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func Trails(ctx context.Context, database *sql.DB, id string) ([]Trail, error) {
	rows, err := database.QueryContext(ctx, `SELECT id,name,description,distance_km,estimated_duration_minutes,difficulty,geometry_geojson,accessibility_notes FROM trails WHERE destination_id=? AND status='verified' ORDER BY name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Trail{}
	for rows.Next() {
		var item Trail
		var distance sql.NullFloat64
		var duration sql.NullInt64
		var geometry sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &distance, &duration, &item.Difficulty, &geometry, &item.AccessibilityNotes); err != nil {
			return nil, err
		}
		if distance.Valid {
			item.DistanceKM = &distance.Float64
		}
		if duration.Valid {
			n := int(duration.Int64)
			item.DurationMinutes = &n
		}
		if geometry.Valid && json.Valid([]byte(geometry.String)) {
			item.Geometry = json.RawMessage(geometry.String)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
