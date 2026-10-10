package inference

import (
	"errors"
	"strings"
	"testing"
)

func TestWindowCandidatesArePreservedAtDurationBoundary(t *testing.T) {
	// A 60-second browser recording can have a little codec padding and 21 windows.
	// Keep every top-five candidate instead of rejecting the final padded window.
	row := `{"scientific_name":"Turdus merula","common_name":"Blackbird","raw_species_name":"Turdus merula_Blackbird","confidence":0.5,"start_time":0,"end_time":3}`
	makeResult := func(n int) []byte {
		return []byte(`{"model":"birdnet","model_version":"3.0","predictions":[` + strings.TrimSuffix(strings.Repeat(row+",", n), ",") + `]}`)
	}
	result, err := DecodeResult(makeResult(105))
	if err != nil || len(result.Predictions) != 105 {
		t.Fatalf("lost valid temporal candidates: %v, %d", err, len(result.Predictions))
	}
	if _, err = DecodeResult(makeResult(201)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("unbounded result accepted: %v", err)
	}
}
