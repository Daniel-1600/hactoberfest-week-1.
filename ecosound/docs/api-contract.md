# EcoSound shared API contract

Base: `http://localhost:8080`. JSON UTF-8. No authentication: a single local collection.
Errors: `{ "error": { "code": "duplicate_discovery", "message": "..." } }`.
No mock inference or seeded wildlife in the running application.

## Identify
`POST /api/identify`, multipart field `audio`. Max file 15 MiB; decoded duration 1–60 seconds.
Accepted containers are WAV, WebM, MP4/M4A, Ogg, MP3, FLAC. FFprobe validates actual content; FFmpeg converts to mono PCM WAV before inference. Server generates UUIDs and owns timestamps.

200 response: `{ "recording_id": "uuid", "audio_url": "/api/recordings/uuid/audio", "duration_seconds": 12, "model": "birdnet", "model_version": "3.0", "predictions": [...] }`.
Each prediction: `{ "id": 12, "scientific_name": "...", "common_name": "...", "raw_species_name": "scientific_common", "confidence": 0.87, "start_time": 0, "end_time": 3, "user_confirmed": false }`.
Empty predictions is a successful no-detection result. Scores are AI identification scores, not guaranteed accuracy. All temporal candidate rows are preserved. UI may group by scientific name, choosing the highest score for display only.
Errors: 400 malformed request, 413 too large, 415 unsupported format, 422 invalid/too short/too long audio, 503 model/process unavailable, 502 invalid inference contract, 504 analysis timeout, 429 inference busy.

## Discoveries
`POST /api/discoveries` JSON `{ "prediction_id": 12 }` → 201 discovery object.
404 unknown prediction, 409 duplicate prediction **or another saved prediction for the same recording**. MVP permits one saved discovery per recording; choosing a different candidate does not earn extra quest credit.
`GET /api/discoveries` → `{ "discoveries": [...] }`, newest first.
Discovery object: `{ "id": 1, "recording_id": "uuid", "prediction_id": 12, "scientific_name": "...", "common_name": "...", "confidence": 0.87, "observed_at": "2026-10-10T06:00:00Z", "user_confirmed": false, "audio_url": "/api/recordings/uuid/audio" }`.
`POST /api/discoveries/{id}/confirm` → 200 updated discovery; 404 missing. Idempotent.
Exact UI copy: **Personally confirmed — not independently scientifically verified.**
`GET /api/recordings/{id}/audio` → normalized WAV; no server file paths exposed.

## Quest
`GET /api/quests/today` → `{ "name": "Morning Bird Quest", "date": "2026-10-10", "timezone": "Africa/Nairobi", "tasks": [{ "name": "Record three distinct calls", "current": 1, "target": 3, "completed": false }, { "name": "Discover two distinct species", "current": 1, "target": 2, "completed": false }], "completed": false }`.
Counts use saved discovery observation times (recording's server receipt time), local-day boundaries, distinct recording IDs and distinct recognized nonempty scientific names. Confirmation has no effect. UI refreshes quest + collection after saving, no optimistic increments.

`GET /health` → `{ "status": "ok", "inference": "configured" }` means server/database healthy, not that model download has been verified.

## Python boundary
`python inference/identify.py decoded.wav` outputs exactly one JSON object with model, model_version, predictions; diagnostics on stderr; exit 1 on failure, 2 on argument error. Normalization is based on official BirdNET 1.1.1 source: `AcousticPredictionResultBase.to_structured_array()` supplies numeric seconds, original `species_name` labels, and scores; labels split at the first underscore without taxonomic rewriting. `unprocessable_inputs` is checked so failed decoding is never treated as no detection. Threshold 0.25, top 5 per window, single inference worker. Every returned row is retained without rounding.

## Feasibility status at initial delegation
Python 3.14.7 and FFmpeg 8.1.2 are available. `pip install birdnet` and downloading the official soundscape failed with DNS errors from the sandbox (`Name or service not known` / `Could not resolve host`). BirdNET was not installed, so no real model output or bird identification was observed. The exact structured-array schema and 3.0 method signature were inspected through the official GitHub source, not inferred from illustrative JSON. Runtime remains real inference only; tests may inject fixtures.
Sources: https://github.com/birdnet-team/birdnet/blob/main/src/birdnet/acoustic/inference/core/prediction/prediction_result.py and https://github.com/birdnet-team/birdnet/blob/main/src/birdnet/acoustic/models/v3_0/model.py .
