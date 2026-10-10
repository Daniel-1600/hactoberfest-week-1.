package quests

import (
	"context"
	"ecosound/backend/internal/db"
	"ecosound/backend/internal/models"
	"regexp"
	"time"
)

// Reject noise/unknown labels without changing the original prediction string.
var scientificName = regexp.MustCompile(`^[A-Z][a-z]+ [a-z][a-z-]+(?: [a-z][a-z-]+)?$`)

func Today(ctx context.Context, store *db.Store, zone *time.Location, now time.Time) (models.Quest, error) {
	local := now.In(zone)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone)
	end := start.AddDate(0, 0, 1)
	rows, err := store.QueryContext(ctx, `SELECT DISTINCT d.recording_id,p.scientific_name FROM discoveries d JOIN predictions p ON p.id=d.prediction_id WHERE d.observed_at>=? AND d.observed_at<?`, db.Timestamp(start), db.Timestamp(end))
	if err != nil {
		return models.Quest{}, err
	}
	defer rows.Close()
	recordings, species := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return models.Quest{}, err
		}
		recordings[id] = true
		if scientificName.MatchString(name) {
			species[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		return models.Quest{}, err
	}
	q := models.Quest{Name: "Morning Bird Quest", Date: local.Format("2006-01-02"), Timezone: zone.String(), Tasks: []models.Task{
		{Name: "Record three distinct calls", Current: len(recordings), Target: 3, Completed: len(recordings) >= 3},
		{Name: "Discover two distinct species", Current: len(species), Target: 2, Completed: len(species) >= 2},
	}}
	q.Completed = q.Tasks[0].Completed && q.Tasks[1].Completed
	return q, nil
}
