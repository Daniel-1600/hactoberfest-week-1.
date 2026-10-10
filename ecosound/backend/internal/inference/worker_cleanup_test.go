//go:build linux || darwin || freebsd

package inference

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestTimeoutStopsChildWorkers(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "orphan-worker-marker")
	childCode := fmt.Sprintf("import time,pathlib; time.sleep(1); pathlib.Path(%q).write_text('worker survived')", marker)
	script := filepath.Join(dir, "workers.py")
	body := fmt.Sprintf("import subprocess,sys,time\nsubprocess.Popen([sys.executable, '-c', %q])\ntime.sleep(10)\n", childCode)
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = (Process{Python: python, Script: script, Timeout: 300 * time.Millisecond}).Identify(context.Background(), "unused.wav")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected timeout, got %v", err)
	}
	// A surviving worker would write the marker even though its parent was killed.
	time.Sleep(1200 * time.Millisecond)
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inference worker survived timeout: %v", err)
	}
}
