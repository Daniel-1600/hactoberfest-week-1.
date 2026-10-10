CREATE TABLE recordings (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    content_type TEXT NOT NULL,
    duration_seconds REAL NOT NULL CHECK (duration_seconds > 0),
    recorded_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE predictions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    recording_id TEXT NOT NULL REFERENCES recordings(id),
    model_name TEXT NOT NULL,
    model_version TEXT NOT NULL,
    raw_species_name TEXT NOT NULL,
    scientific_name TEXT NOT NULL,
    common_name TEXT NOT NULL,
    confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    start_time REAL,
    end_time REAL,
    created_at TEXT NOT NULL,
    UNIQUE(id, recording_id)
);
CREATE TABLE discoveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    prediction_id INTEGER NOT NULL UNIQUE,
    recording_id TEXT NOT NULL UNIQUE REFERENCES recordings(id),
    observed_at TEXT NOT NULL,
    user_confirmed INTEGER NOT NULL DEFAULT 0 CHECK (user_confirmed IN (0,1)),
    created_at TEXT NOT NULL,
    FOREIGN KEY(prediction_id, recording_id) REFERENCES predictions(id, recording_id)
);
CREATE INDEX discoveries_observed_at ON discoveries(observed_at);
CREATE INDEX predictions_recording_id ON predictions(recording_id);
-- Predictions remain immutable, including when a person confirms a discovery.
CREATE TRIGGER predictions_immutable BEFORE UPDATE ON predictions
BEGIN SELECT RAISE(ABORT, 'model predictions are immutable'); END;
