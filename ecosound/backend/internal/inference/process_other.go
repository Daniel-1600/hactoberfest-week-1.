//go:build !linux && !darwin && !freebsd

package inference

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}
