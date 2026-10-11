package observations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"ecosound/backend/internal/db"
)

var ErrNotFound = errors.New("observation or discovery not found")
var ErrInvalid = errors.New("invalid observation consent or location")

const ConsentVersion = "ecosound-observation-v1"

type Input struct {
	DiscoveryID    int64    `json:"discovery_id"`
	Consent        bool     `json:"consent"`
	Visibility     string   `json:"visibility"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	AccuracyMeters *float64 `json:"location_accuracy_meters,omitempty"`
	LocationSource string   `json:"location_source"`
	DestinationID  string   `json:"destination_id,omitempty"`
	ConsentVersion string   `json:"consent_version"`
	IdempotencyKey string   `json:"-"`
}
type Observation struct {
	ID                  string   `json:"id"`
	DiscoveryID         int64    `json:"discovery_id"`
	ScientificName      string   `json:"scientific_name"`
	CommonName          string   `json:"common_name"`
	ObservedAt          string   `json:"observed_at"`
	Latitude            *float64 `json:"latitude,omitempty"`
	Longitude           *float64 `json:"longitude,omitempty"`
	VerificationStatus  string   `json:"verification_status"`
	Visibility          string   `json:"visibility"`
	LocationGeneralized bool     `json:"location_generalized"`
	DestinationID       string   `json:"destination_id,omitempty"`
}

func Create(ctx context.Context, database *db.Store, input Input, now time.Time) (Observation, error) {
	return CreateWithPrecision(ctx, database, input, now, 2)
}

func CreateWithPrecision(ctx context.Context, database *db.Store, input Input, now time.Time, precisionDigits int) (Observation, error) {
	if precisionDigits < 0 || precisionDigits > 2 {
		return Observation{}, ErrInvalid
	}
	if !input.Consent || input.ConsentVersion != ConsentVersion || input.DiscoveryID <= 0 || input.Visibility != "public" && input.Visibility != "private" {
		return Observation{}, ErrInvalid
	}
	if (input.Latitude == nil) != (input.Longitude == nil) {
		return Observation{}, ErrInvalid
	}
	if input.Latitude != nil && (!finite(*input.Latitude) || !finite(*input.Longitude) || *input.Latitude < -90 || *input.Latitude > 90 || *input.Longitude < -180 || *input.Longitude > 180) {
		return Observation{}, ErrInvalid
	}
	if input.AccuracyMeters != nil && (!finite(*input.AccuracyMeters) || *input.AccuracyMeters < 0 || *input.AccuracyMeters > 100000) {
		return Observation{}, ErrInvalid
	}
	if input.Latitude == nil && input.LocationSource != "none" || input.Latitude != nil && input.LocationSource != "device" && input.LocationSource != "manual" {
		return Observation{}, ErrInvalid
	}
	if input.DestinationID != "" {
		var status string
		if err := database.QueryRowContext(ctx, `SELECT status FROM destinations WHERE id=?`, input.DestinationID).Scan(&status); err != nil {
			return Observation{}, ErrNotFound
		}
	}
	if input.IdempotencyKey != "" {
		if existing, err := getByKey(ctx, database, input.IdempotencyKey); err == nil {
			return existing, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return Observation{}, err
		}
	}
	if existing, err := getByDiscovery(ctx, database, input.DiscoveryID); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Observation{}, err
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return Observation{}, err
	}
	defer tx.Rollback()
	var out Observation
	out.ID = db.NewID()
	out.DiscoveryID = input.DiscoveryID
	out.Visibility = input.Visibility
	out.DestinationID = input.DestinationID
	var confirmed bool
	err = tx.QueryRowContext(ctx, `SELECT p.scientific_name,p.common_name,d.observed_at,d.user_confirmed FROM discoveries d JOIN predictions p ON p.id=d.prediction_id WHERE d.id=?`, input.DiscoveryID).Scan(&out.ScientificName, &out.CommonName, &out.ObservedAt, &confirmed)
	if errors.Is(err, sql.ErrNoRows) {
		return Observation{}, ErrNotFound
	}
	if err != nil {
		return Observation{}, err
	}
	out.VerificationStatus = "ai_predicted"
	if confirmed {
		out.VerificationStatus = "user_confirmed"
	}
	var sensitive int
	_ = tx.QueryRowContext(ctx, `SELECT sensitive FROM species_knowledge WHERE scientific_name=?`, out.ScientificName).Scan(&sensitive)
	var lat, lon, pubLat, pubLon, accuracy any
	if input.Latitude != nil {
		lat = *input.Latitude
		lon = *input.Longitude
		if input.AccuracyMeters != nil {
			accuracy = *input.AccuracyMeters
		}
	}
	if input.Visibility == "public" && input.Latitude != nil && sensitive == 0 {
		factor := math.Pow10(precisionDigits)
		pubLat = math.Round(*input.Latitude*factor) / factor
		pubLon = math.Round(*input.Longitude*factor) / factor
		l1, l2 := pubLat.(float64), pubLon.(float64)
		out.Latitude = &l1
		out.Longitude = &l2
		out.LocationGeneralized = true
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO observations(id,discovery_id,observed_at,latitude,longitude,public_latitude,public_longitude,location_accuracy_meters,location_source,verification_status,visibility,consent_version,created_at,destination_id,idempotency_key) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, out.ID, input.DiscoveryID, out.ObservedAt, lat, lon, pubLat, pubLon, accuracy, source(input), out.VerificationStatus, input.Visibility, ConsentVersion, db.Timestamp(now), nullable(input.DestinationID), nullable(input.IdempotencyKey))
	if err != nil {
		return Observation{}, err
	}
	return out, tx.Commit()
}

func getByKey(ctx context.Context, database *db.Store, key string) (Observation, error) {
	var id string
	if err := database.QueryRowContext(ctx, "SELECT id FROM observations WHERE idempotency_key=?", key).Scan(&id); err != nil {
		return Observation{}, err
	}
	return getByID(ctx, database, id)
}
func getByDiscovery(ctx context.Context, database *db.Store, discovery int64) (Observation, error) {
	var id string
	if err := database.QueryRowContext(ctx, "SELECT id FROM observations WHERE discovery_id=?", discovery).Scan(&id); err != nil {
		return Observation{}, err
	}
	return getByID(ctx, database, id)
}
func getByID(ctx context.Context, database *db.Store, id string) (Observation, error) {
	var item Observation
	var lat, lon sql.NullFloat64
	var destination sql.NullString
	err := database.QueryRowContext(ctx, `SELECT o.id,o.discovery_id,p.scientific_name,p.common_name,o.observed_at,o.public_latitude,o.public_longitude,o.verification_status,o.visibility,o.destination_id FROM observations o JOIN discoveries d ON d.id=o.discovery_id JOIN predictions p ON p.id=d.prediction_id WHERE o.id=?`, id).Scan(&item.ID, &item.DiscoveryID, &item.ScientificName, &item.CommonName, &item.ObservedAt, &lat, &lon, &item.VerificationStatus, &item.Visibility, &destination)
	if err != nil {
		return item, err
	}
	if lat.Valid && lon.Valid {
		item.Latitude = &lat.Float64
		item.Longitude = &lon.Float64
		item.LocationGeneralized = true
	}
	if destination.Valid {
		item.DestinationID = destination.String
	}
	return item, nil
}

type Filter struct{ Species, Status, Destination, Since, Until string }

func List(ctx context.Context, database *db.Store, filter Filter, publicOnly bool) ([]Observation, error) {
	query := `SELECT o.id,o.discovery_id,p.scientific_name,p.common_name,o.observed_at,o.public_latitude,o.public_longitude,o.verification_status,o.visibility,o.destination_id FROM observations o JOIN discoveries d ON d.id=o.discovery_id JOIN predictions p ON p.id=d.prediction_id LEFT JOIN species_knowledge sk ON sk.scientific_name=p.scientific_name WHERE 1=1`
	args := []any{}
	if publicOnly {
		query += ` AND o.visibility='public' AND o.public_latitude IS NOT NULL AND o.public_longitude IS NOT NULL AND coalesce(sk.sensitive,1)=0 AND o.verification_status!='rejected'`
	}
	if filter.Species != "" {
		query += ` AND p.scientific_name=?`
		args = append(args, filter.Species)
	}
	if filter.Status != "" {
		query += ` AND o.verification_status=?`
		args = append(args, filter.Status)
	}
	if filter.Destination != "" {
		query += ` AND o.destination_id=?`
		args = append(args, filter.Destination)
	}
	if filter.Since != "" {
		query += ` AND o.observed_at>=?`
		args = append(args, filter.Since)
	}
	if filter.Until != "" {
		query += ` AND o.observed_at<?`
		args = append(args, filter.Until)
	}
	query += ` ORDER BY o.observed_at DESC LIMIT 500`
	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Observation{}
	for rows.Next() {
		var item Observation
		var lat, lon sql.NullFloat64
		var destination sql.NullString
		if err := rows.Scan(&item.ID, &item.DiscoveryID, &item.ScientificName, &item.CommonName, &item.ObservedAt, &lat, &lon, &item.VerificationStatus, &item.Visibility, &destination); err != nil {
			return nil, err
		}
		if lat.Valid && lon.Valid {
			item.Latitude = &lat.Float64
			item.Longitude = &lon.Float64
			item.LocationGeneralized = true
		}
		if destination.Valid {
			item.DestinationID = destination.String
		}
		if publicOnly {
			item.Visibility = "public"
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func Get(ctx context.Context, database *db.Store, id string) (Observation, error) {
	items, err := List(ctx, database, Filter{}, false)
	if err != nil {
		return Observation{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return Observation{}, ErrNotFound
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func source(input Input) string {
	if input.Latitude == nil {
		return "none"
	}
	return input.LocationSource
}
func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
func ValidateFilters(filter Filter) error {
	if filter.Status != "" && filter.Status != "ai_predicted" && filter.Status != "user_confirmed" && filter.Status != "expert_verified" && filter.Status != "rejected" {
		return fmt.Errorf("invalid observation status")
	}
	if filter.Since != "" {
		if _, err := time.Parse(time.RFC3339, filter.Since); err != nil {
			return err
		}
	}
	if filter.Until != "" {
		if _, err := time.Parse(time.RFC3339, filter.Until); err != nil {
			return err
		}
	}
	return nil
}
