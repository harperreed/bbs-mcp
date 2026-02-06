// ABOUTME: Windows-specific file locking for MarkdownStore using exclusive file creation
// ABOUTME: Provides write protection via O_CREATE|O_EXCL lock file strategy

//go:build windows

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// lockFile acquires an exclusive file lock for write operations.
// On Windows, this uses an exclusive-create lock file approach since
// syscall.Flock is not available. The lock file is created with O_CREATE|O_EXCL
// which fails atomically if the file already exists.
// Returns the lock file which must be unlocked by the caller.
func (s *MarkdownStore) lockFile() (*os.File, error) {
	lockPath := filepath.Join(s.dataDir, ".lock")

	// Retry loop to wait for lock availability
	const maxRetries = 100
	const retryDelay = 50 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err == nil {
			return f, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("open lock file: %w", err)
		}
		// Lock file exists, check for staleness (older than 30 seconds)
		if info, statErr := os.Stat(lockPath); statErr == nil {
			if time.Since(info.ModTime()) > 30*time.Second {
				os.Remove(lockPath)
				continue
			}
		}
		time.Sleep(retryDelay)
	}
	return nil, fmt.Errorf("acquire lock: timed out waiting for lock file")
}

// unlockFile releases the file lock by closing and removing the lock file.
func unlockFile(f *os.File) {
	name := f.Name()
	f.Close()
	os.Remove(name)
}
