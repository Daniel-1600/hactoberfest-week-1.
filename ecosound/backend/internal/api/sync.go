package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"ecosound/backend/internal/adventures"
	"ecosound/backend/internal/db"
	"ecosound/backend/internal/observations"
)

type syncOperation struct {
	ID      string          `json:"operation_id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
type syncOutcome struct {
	ID     string `json:"operation_id"`
	Status string `json:"status"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (s *Server) syncOperations(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		Operations []syncOperation `json:"operations"`
	}
	if err := decoder.Decode(&body); err != nil {
		fail(w, 400, "invalid_sync", "Send a bounded list of offline operations.")
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		fail(w, 400, "invalid_sync", "Send exactly one JSON object.")
		return
	}
	if len(body.Operations) == 0 || len(body.Operations) > 50 {
		fail(w, 400, "invalid_sync", "Send between 1 and 50 offline operations.")
		return
	}
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	results := make([]syncOutcome, 0, len(body.Operations))
	for _, operation := range body.Operations {
		out := syncOutcome{ID: operation.ID, Status: "failed"}
		if !idempotencyKey.MatchString(operation.ID) || len(operation.Payload) == 0 || len(operation.Payload) > 32768 {
			out.Error = "invalid operation identifier or payload"
			results = append(results, out)
			continue
		}
		hash := sha256.Sum256(operation.Payload)
		payloadHash := hex.EncodeToString(hash[:])
		var prior, resultJSON string
		err := s.config.DB.QueryRowContext(r.Context(), `SELECT payload_hash,result_json FROM sync_operations WHERE operation_id=?`, operation.ID).Scan(&prior, &resultJSON)
		if err == nil {
			if prior != payloadHash {
				out.Error = "operation identifier reused with different data"
				results = append(results, out)
				continue
			}
			out.Status = "already_applied"
			_ = json.Unmarshal([]byte(resultJSON), &out.Result)
			results = append(results, out)
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			out.Error = "could not check operation idempotency"
			results = append(results, out)
			continue
		}
		result, applyErr := s.applySyncOperation(r.Context(), operation)
		if applyErr != nil {
			out.Error = "operation was rejected; verify its data and retry if appropriate"
			results = append(results, out)
			continue
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			out.Error = "operation result could not be saved"
			results = append(results, out)
			continue
		}
		if _, err = s.config.DB.ExecContext(r.Context(), `INSERT INTO sync_operations(operation_id,operation_type,payload_hash,result_json,created_at) VALUES(?,?,?,?,?)`, operation.ID, operation.Type, payloadHash, string(encoded), db.Timestamp(s.config.Now())); err != nil {
			out.Error = "operation result could not be saved"
			results = append(results, out)
			continue
		}
		out.Status = "applied"
		out.Result = result
		results = append(results, out)
	}
	reply(w, 200, map[string]any{"results": results})
}

func (s *Server) applySyncOperation(ctx context.Context, operation syncOperation) (any, error) {
	switch operation.Type {
	case "save_discovery":
		var payload struct {
			PredictionID int64 `json:"prediction_id"`
		}
		if err := strictPayload(operation.Payload, &payload); err != nil {
			return nil, err
		}
		if payload.PredictionID <= 0 {
			return nil, errors.New("prediction_id must be positive")
		}
		return s.config.DB.SaveDiscoveryIdempotent(ctx, payload.PredictionID, s.config.Now(), operation.ID)
	case "complete_adventure_task":
		var payload struct {
			AdventureID  string `json:"adventure_id"`
			TaskID       string `json:"task_id"`
			DiscoveryID  int64  `json:"discovery_id"`
			SelfReported bool   `json:"self_reported"`
		}
		if err := strictPayload(operation.Payload, &payload); err != nil {
			return nil, err
		}
		if payload.AdventureID == "" || payload.TaskID == "" {
			return nil, errors.New("adventure_id and task_id are required")
		}
		return s.config.Adventures.CompleteTaskIdempotent(ctx, payload.AdventureID, payload.TaskID, payload.DiscoveryID, payload.SelfReported, s.config.Now())
	case "create_observation":
		var payload observations.Input
		if err := strictPayload(operation.Payload, &payload); err != nil {
			return nil, err
		}
		payload.IdempotencyKey = operation.ID
		return observations.Create(ctx, s.config.DB, payload, s.config.Now())
	case "finish_adventure":
		var payload struct {
			AdventureID string `json:"adventure_id"`
			Minutes     *int   `json:"self_reported_minutes,omitempty"`
			Discovered  string `json:"what_did_you_discover,omitempty"`
			Surprised   string `json:"what_surprised_you,omitempty"`
			Again       *bool  `json:"would_explore_again,omitempty"`
		}
		if err := strictPayload(operation.Payload, &payload); err != nil || payload.AdventureID == "" {
			return nil, errors.New("adventure_id is required")
		}
		adventure, err := s.config.Adventures.Finish(ctx, payload.AdventureID, payload.Minutes, payload.Discovered, payload.Surprised, payload.Again, s.config.Now())
		if errors.Is(err, adventures.ErrConflict) {
			adventure, err = s.config.Adventures.Get(ctx, payload.AdventureID)
			if err == nil && adventure.Status != "completed" {
				return nil, adventures.ErrConflict
			}
		}
		return adventure, err
	default:
		return nil, errors.New("unsupported offline operation type")
	}
}
func strictPayload(data []byte, dest any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return errors.New("operation payload is invalid")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("operation payload must contain one object")
	}
	return nil
}
