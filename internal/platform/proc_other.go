//go:build !windows

package platform

import (
	"os/exec"
	"syscall"
)

// SetDetachedProcAttr configures the command to run as a detached process on Unix.
// Setpgid creates a new process group so the child survives parent exit.
func SetDetachedProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
