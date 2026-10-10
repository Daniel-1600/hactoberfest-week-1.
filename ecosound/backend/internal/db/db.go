package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ecosound/backend/internal/models"
	"ecosound/backend/migrations"
	"github.com/mattn/go-sqlite3"
)

var ErrDuplicate = errors.New("a discovery is already saved for this prediction or recording")

type Store struct{ *sql.DB }

func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
	}
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	// A single connection makes the PRAGMA apply to every query and serializes writes.
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	s := &Store{database}
	if _, err = database.Exec("PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000; PRAGMA journal_mode = WAL;"); err != nil {
		database.Close()
		return nil, err
	}
	if err = s.migrate(); err != nil {
		database.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	if _, err := s.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)"); err != nil {
		return err
	}
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		var n int
		if err := s.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version=?", entry.Name()).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		script, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			return err
		}
		tx, err := s.Begin()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(string(script)); err == nil {
			_, err = tx.Exec("INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)", entry.Name(), Timestamp(time.Now()))
		}
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Fixed-width UTC values sort chronologically, including day boundaries.
func Timestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000000000Z") }

func (s *Store) SaveIdentification(ctx context.Context, id, path string, duration float64, recordedAt time.Time, result models.InferenceResult) (models.Identification, error) {
	out := models.Identification{RecordingID: id, AudioURL: AudioURL(id), DurationSeconds: duration, InferenceResult: result}
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	now := Timestamp(recordedAt)
	_, err = tx.ExecContext(ctx, "INSERT INTO recordings(id,file_path,content_type,duration_seconds,recorded_at,created_at) VALUES(?,?,?,?,?,?)", id, path, "audio/wav", duration, now, now)
	if err != nil {
		return out, err
	}
	for i := range out.Predictions {
		p := &out.Predictions[i]
		res, err := tx.ExecContext(ctx, `INSERT INTO predictions(recording_id,model_name,model_version,raw_species_name,scientific_name,common_name,confidence,start_time,end_time,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, result.Model, result.ModelVersion, p.RawSpeciesName, p.ScientificName, p.CommonName, p.Confidence, p.StartTime, p.EndTime, now)
		if err != nil {
			return out, err
		}
		p.ID, err = res.LastInsertId()
		if err != nil {
			return out, err
		}
	}
	return out, tx.Commit()
}

func AudioURL(id string) string { return "/api/recordings/" + id + "/audio" }

func (s *Store) SaveDiscovery(ctx context.Context, predictionID int64, now time.Time) (models.Discovery, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return models.Discovery{}, err
	}
	defer tx.Rollback()
	var recordingID, observedAt string
	err = tx.QueryRowContext(ctx, `SELECT p.recording_id,r.recorded_at FROM predictions p JOIN recordings r ON r.id=p.recording_id WHERE p.id=?`, predictionID).Scan(&recordingID, &observedAt)
	if err != nil {
		return models.Discovery{}, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO discoveries(prediction_id,recording_id,observed_at,created_at) VALUES(?,?,?,?)`, predictionID, recordingID, observedAt, Timestamp(now))
	if err != nil {
		var se sqlite3.Error
		if errors.As(err, &se) && se.ExtendedCode == sqlite3.ErrConstraintUnique {
			return models.Discovery{}, ErrDuplicate
		}
		return models.Discovery{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.Discovery{}, err
	}
	if err = tx.Commit(); err != nil {
		return models.Discovery{}, err
	}
	return s.Discovery(ctx, id)
}

const discoverySelect = `SELECT d.id,d.recording_id,d.prediction_id,p.scientific_name,p.common_name,p.confidence,d.observed_at,d.user_confirmed FROM discoveries d JOIN predictions p ON p.id=d.prediction_id`

type scanner interface{ Scan(...any) error }

func scanDiscovery(row scanner) (models.Discovery, error) {
	var d models.Discovery
	err := row.Scan(&d.ID, &d.RecordingID, &d.PredictionID, &d.ScientificName, &d.CommonName, &d.Confidence, &d.ObservedAt, &d.UserConfirmed)
	d.AudioURL = AudioURL(d.RecordingID)
	return d, err
}

func (s *Store) Discovery(ctx context.Context, id int64) (models.Discovery, error) {
	return scanDiscovery(s.QueryRowContext(ctx, discoverySelect+" WHERE d.id=?", id))
}

func (s *Store) Discoveries(ctx context.Context) ([]models.Discovery, error) {
	rows, err := s.QueryContext(ctx, discoverySelect+" ORDER BY d.observed_at DESC, d.id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Discovery{}
	for rows.Next() {
		d, err := scanDiscovery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) Confirm(ctx context.Context, id int64) (models.Discovery, error) {
	res, err := s.ExecContext(ctx, "UPDATE discoveries SET user_confirmed=1 WHERE id=?", id)
	if err != nil {
		return models.Discovery{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return models.Discovery{}, err
	}
	if n == 0 {
		return models.Discovery{}, sql.ErrNoRows
	}
	return s.Discovery(ctx, id)
}
