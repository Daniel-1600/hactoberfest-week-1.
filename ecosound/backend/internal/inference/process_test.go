package inference

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDecodeResultRejectsBrokenContracts(t *testing.T) {
	valid := `{"model":"birdnet","model_version":"3.0","predictions":[{"scientific_name":"Turdus merula","common_name":"Common_Blackbird","raw_species_name":"Turdus merula_Common_Blackbird","confidence":0.8123456789,"start_time":0,"end_time":3}]}`
	result, err := DecodeResult([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Predictions) != 1 || result.Predictions[0].Confidence != 0.8123456789 {
		t.Fatalf("lost prediction precision: %+v", result)
	}
	for name, data := range map[string]string{
		"not json":            `BirdNET loading`,
		"extra object":        valid + ` {}`,
		"missing predictions": `{"model":"birdnet","model_version":"3.0"}`,
		"missing score":       `{"model":"birdnet","model_version":"3.0","predictions":[{"scientific_name":"A b","common_name":"c","raw_species_name":"A b_c","start_time":0,"end_time":1}]}`,
		"inconsistent label":  `{"model":"birdnet","model_version":"3.0","predictions":[{"scientific_name":"A b","common_name":"wrong","raw_species_name":"A b_c","confidence":0.5,"start_time":0,"end_time":1}]}`,
		"reversed window":     `{"model":"birdnet","model_version":"3.0","predictions":[{"scientific_name":"A b","common_name":"c","raw_species_name":"A b_c","confidence":0.5,"start_time":2,"end_time":1}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeResult([]byte(data)); !errors.Is(err, ErrMalformed) {
				t.Fatalf("wanted ErrMalformed, got %v", err)
			}
		})
	}
}

func TestProcessBoundaryDistinguishesFailureMalformedAndTimeout(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	makeScript := func(body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "fixture.py")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	for _, tc := range []struct {
		name, body string
		timeout    time.Duration
		want       error
	}{
		{"failure", "import sys\nprint('model missing', file=sys.stderr)\nsys.exit(1)\n", time.Second, ErrUnavailable},
		{"malformed", "print('not json')\n", time.Second, ErrMalformed},
		{"timeout", "import time\ntime.sleep(10)\n", 100 * time.Millisecond, ErrTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (Process{Python: python, Script: makeScript(tc.body), Timeout: tc.timeout}).Identify(context.Background(), "unused.wav")
			if !errors.Is(err, tc.want) {
				t.Fatalf("wanted %v, got %v", tc.want, err)
			}
		})
	}
}
