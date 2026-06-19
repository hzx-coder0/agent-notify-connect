//go:build windows

package teamstate

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// withFileLock executes fn while holding an exclusive file lock (LockFileEx).
// This ensures cross-process safety between Stop and TeammateIdle hooks.
func withFileLock(teamName string, fn func() error) error {
	path := lockPath(teamName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer f.Close()

	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, ol); err != nil {
		return fmt.Errorf("acquire file lock: %w", err)
	}
	defer func() { _ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol) }()

	return fn()
}
