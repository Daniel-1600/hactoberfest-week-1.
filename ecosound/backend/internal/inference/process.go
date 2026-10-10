package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os/exec"
	"strings"
	"time"

	"ecosound/backend/internal/models"
)

var (
	ErrTimeout     = errors.New("inference timed out")
	ErrUnavailable = errors.New("BirdNET inference unavailable")
	ErrMalformed   = errors.New("invalid inference output")
)

type Identifier interface {
	Identify(context.Context, string) (models.InferenceResult, error)
}
type Process struct {
	Python, Script string
	Timeout        time.Duration
}

// cappedBuffer drains excess diagnostics without allowing unbounded allocation.
type cappedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - b.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

func (p Process) Identify(ctx context.Context, path string) (models.InferenceResult, error) {
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 180 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.Python, p.Script, path)
	configureProcess(cmd)
	stdout, stderr := &cappedBuffer{limit: 4 << 20}, &cappedBuffer{limit: 64 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return models.InferenceResult{}, fmt.Errorf("%w: %v", ErrTimeout, ctx.Err())
	}
	if err != nil {
		slog.Error("BirdNET process failed", "error", err, "diagnostic", stderr.String())
		return models.InferenceResult{}, ErrUnavailable
	}
	if stdout.overflow {
		return models.InferenceResult{}, fmt.Errorf("%w: response too large", ErrMalformed)
	}
	return DecodeResult(stdout.Bytes())
}

func DecodeResult(data []byte) (models.InferenceResult, error) {
	// Pointer score detects a missing field instead of interpreting it as zero.
	var wire struct {
		Model       string `json:"model"`
		Version     string `json:"model_version"`
		Predictions []struct {
			Scientific *string  `json:"scientific_name"`
			Common     *string  `json:"common_name"`
			Raw        *string  `json:"raw_species_name"`
			Confidence *float64 `json:"confidence"`
			Start      *float64 `json:"start_time"`
			End        *float64 `json:"end_time"`
		} `json:"predictions"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return models.InferenceResult{}, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return models.InferenceResult{}, fmt.Errorf("%w: expected exactly one object", ErrMalformed)
	}
	if wire.Model != "birdnet" || wire.Version != "3.0" || wire.Predictions == nil || len(wire.Predictions) > 200 {
		return models.InferenceResult{}, fmt.Errorf("%w: incompatible model contract", ErrMalformed)
	}
	result := models.InferenceResult{Model: wire.Model, ModelVersion: wire.Version, Predictions: []models.Prediction{}}
	for _, row := range wire.Predictions {
		if row.Scientific == nil || row.Common == nil || row.Raw == nil || row.Confidence == nil {
			return models.InferenceResult{}, fmt.Errorf("%w: missing prediction field", ErrMalformed)
		}
		sci, common, sep := strings.Cut(*row.Raw, "_")
		if !sep {
			common = ""
		}
		if sci != *row.Scientific || common != *row.Common || len(*row.Raw) > 512 || math.IsNaN(*row.Confidence) || math.IsInf(*row.Confidence, 0) || *row.Confidence < 0 || *row.Confidence > 1 {
			return models.InferenceResult{}, fmt.Errorf("%w: inconsistent label or score", ErrMalformed)
		}
		if (row.Start == nil) != (row.End == nil) {
			return models.InferenceResult{}, fmt.Errorf("%w: incomplete window", ErrMalformed)
		}
		if row.Start != nil && (math.IsNaN(*row.Start) || math.IsInf(*row.Start, 0) || math.IsNaN(*row.End) || math.IsInf(*row.End, 0) || *row.Start < 0 || *row.End < *row.Start || *row.End > 60.1) {
			return models.InferenceResult{}, fmt.Errorf("%w: invalid window", ErrMalformed)
		}
		result.Predictions = append(result.Predictions, models.Prediction{ScientificName: *row.Scientific, CommonName: *row.Common, RawSpeciesName: *row.Raw, Confidence: *row.Confidence, StartTime: row.Start, EndTime: row.End})
	}
	return result, nil
}
