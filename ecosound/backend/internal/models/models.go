package models

// Confidence is the model's raw identification score, not calibrated accuracy.
type Prediction struct {
	ID             int64    `json:"id"`
	ScientificName string   `json:"scientific_name"`
	CommonName     string   `json:"common_name"`
	RawSpeciesName string   `json:"raw_species_name"`
	Confidence     float64  `json:"confidence"`
	StartTime      *float64 `json:"start_time"`
	EndTime        *float64 `json:"end_time"`
	UserConfirmed  bool     `json:"user_confirmed"`
}

type InferenceResult struct {
	Model        string       `json:"model"`
	ModelVersion string       `json:"model_version"`
	Predictions  []Prediction `json:"predictions"`
}

type Identification struct {
	RecordingID     string  `json:"recording_id"`
	AudioURL        string  `json:"audio_url"`
	DurationSeconds float64 `json:"duration_seconds"`
	InferenceResult
}

type Discovery struct {
	ID             int64   `json:"id"`
	RecordingID    string  `json:"recording_id"`
	PredictionID   int64   `json:"prediction_id"`
	ScientificName string  `json:"scientific_name"`
	CommonName     string  `json:"common_name"`
	Confidence     float64 `json:"confidence"`
	ObservedAt     string  `json:"observed_at"`
	UserConfirmed  bool    `json:"user_confirmed"`
	AudioURL       string  `json:"audio_url"`
}

type Task struct {
	Name      string `json:"name"`
	Current   int    `json:"current"`
	Target    int    `json:"target"`
	Completed bool   `json:"completed"`
}

type Quest struct {
	Name      string `json:"name"`
	Date      string `json:"date"`
	Timezone  string `json:"timezone"`
	Tasks     []Task `json:"tasks"`
	Completed bool   `json:"completed"`
}
