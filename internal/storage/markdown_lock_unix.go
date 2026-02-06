// ABOUTME: Unix-specific file locking using syscall.Flock for MarkdownStore
// ABOUTME: Provides exclusive advisory locking for concurrent write protection

//go:build !windows

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockFile acquires an exclusive file lock for write operations.
// Returns the lock file which must be unlocked by the caller.
func (s *MarkdownStore) lockFile() (*os.File, error) {
	lockPath := filepath.Join(s.dataDir, ".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	return f, nil
}

// unlockFile releases the file lock.
func unlockFile(f *os.File) {
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
}
