package platform

import (
	"os/exec"
	"syscall"
)

// SetDetachedProcAttr configures the command to run as a detached process on Windows.
// CREATE_NEW_PROCESS_GROUP (0x200) detaches from parent's console group.
// CREATE_NO_WINDOW (0x08000000) prevents a visible console window.
func SetDetachedProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000200 | 0x08000000,
	}
}
