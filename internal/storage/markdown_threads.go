// ABOUTME: Thread CRUD operations for MarkdownStore
// ABOUTME: Persists threads as markdown files with YAML frontmatter in topic directories

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/harper/bbs/internal/models"
)

// CreateThread stores a new thread as a markdown file.
func (s *MarkdownStore) CreateThread(t *models.Thread) error {
	lock, err := s.lockFile()
	if err != nil {
		return err
	}
	defer unlockFile(lock)

	// Look up topic name
	topicName, err := s.topicNameByID(t.TopicID)
	if err != nil {
		return fmt.Errorf("resolve topic for thread: %w", err)
	}

	// Ensure topic directory exists
	topicDir := s.topicDirPath(topicName)
	if err := os.MkdirAll(topicDir, 0750); err != nil {
		return fmt.Errorf("create topic directory: %w", err)
	}

	// Generate filename
	filename := s.threadFileName(topicName, t.Subject, t.ID)
	fp := filepath.Join(topicDir, filename)

	// Render the thread file (no messages yet)
	content := renderThread(t, topicName, nil)
	if err := atomicWrite(fp, []byte(content)); err != nil {
		return fmt.Errorf("write thread file: %w", err)
	}

	return nil
}

// GetThread retrieves a thread by ID.
func (s *MarkdownStore) GetThread(id uuid.UUID) (*models.Thread, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		fp, err := s.threadFilePath(e.Name, id)
		if err != nil {
			continue
		}
		thread, err := s.readThreadFromFile(fp, id)
		if err != nil {
			continue
		}
		return thread, nil
	}
	return nil, fmt.Errorf("thread not found: %s", id)
}

// ListThreads returns all threads for a topic, sorted by sticky then updated_at DESC.
func (s *MarkdownStore) ListThreads(topicID uuid.UUID) ([]*models.Thread, error) {
	topicName, err := s.topicNameByID(topicID)
	if err != nil {
		return nil, fmt.Errorf("resolve topic: %w", err)
	}

	topicDir := s.topicDirPath(topicName)
	entries, err := os.ReadDir(topicDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read topic directory: %w", err)
	}

	var threads []*models.Thread
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		fp := filepath.Join(topicDir, entry.Name())
		thread, err := s.readThreadFromFileWithUpdatedAt(fp)
		if err != nil {
			continue
		}
		if thread.TopicID != topicID {
			continue
		}
		threads = append(threads, thread)
	}

	// Sort: sticky first, then by updated_at DESC
	sort.Slice(threads, func(i, j int) bool {
		if threads[i].Sticky != threads[j].Sticky {
			return threads[i].Sticky
		}
		return threads[j].UpdatedAt.Before(threads[i].UpdatedAt)
	})

	return threads, nil
}

// UpdateThread updates an existing thread.
func (s *MarkdownStore) UpdateThread(t *models.Thread) error {
	lock, err := s.lockFile()
	if err != nil {
		return err
	}
	defer unlockFile(lock)

	topicName, err := s.topicNameByID(t.TopicID)
	if err != nil {
		return fmt.Errorf("resolve topic: %w", err)
	}

	oldPath, err := s.threadFilePath(topicName, t.ID)
	if err != nil {
		return fmt.Errorf("find thread file: %w", err)
	}

	// Read existing messages
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return fmt.Errorf("read thread file: %w", err)
	}

	messages := parseThreadMessages(string(data))

	// Set updated_at to now
	t.UpdatedAt = time.Now().UTC()

	// Render updated thread
	content := renderThread(t, topicName, messages)

	// Determine new filename (in case subject changed)
	newFilename := s.threadFileName(topicName, t.Subject, t.ID)
	newPath := filepath.Join(s.topicDirPath(topicName), newFilename)

	if err := atomicWrite(newPath, []byte(content)); err != nil {
		return fmt.Errorf("write thread file: %w", err)
	}

	// Remove old file if path changed
	if oldPath != newPath {
		os.Remove(oldPath)
	}

	return nil
}

// DeleteThread deletes a thread (removes its markdown file).
func (s *MarkdownStore) DeleteThread(id uuid.UUID) error {
	lock, err := s.lockFile()
	if err != nil {
		return err
	}
	defer unlockFile(lock)

	entries, err := s.readTopics()
	if err != nil {
		return err
	}

	for _, e := range entries {
		fp, err := s.threadFilePath(e.Name, id)
		if err != nil {
			continue
		}

		// Also clean up any attachments for messages in this thread
		data, err := os.ReadFile(fp)
		if err == nil {
			messages := parseThreadMessages(string(data))
			for _, msg := range messages {
				prefix := msg.ID.String()[:8]
				attDir := s.attachmentDirPath(e.Name, prefix)
				os.RemoveAll(attDir)
			}
		}

		if err := os.Remove(fp); err != nil {
			return fmt.Errorf("delete thread file: %w", err)
		}
		return nil
	}
	return fmt.Errorf("thread not found: %s", id)
}

// SetThreadSticky sets the sticky status of a thread.
// Performs the read-modify-write under a single lock hold to avoid TOCTOU races.
func (s *MarkdownStore) SetThreadSticky(id uuid.UUID, sticky bool) error {
	lock, err := s.lockFile()
	if err != nil {
		return err
	}
	defer unlockFile(lock)

	// Find the thread file while holding the lock
	entries, err := s.readTopics()
	if err != nil {
		return err
	}

	var threadFP string
	var topicName string
	for _, e := range entries {
		fp, fpErr := s.threadFilePath(e.Name, id)
		if fpErr != nil {
			continue
		}
		threadFP = fp
		topicName = e.Name
		break
	}
	if threadFP == "" {
		return fmt.Errorf("thread not found: %s", id)
	}

	// Read thread data under lock
	data, err := os.ReadFile(threadFP)
	if err != nil {
		return fmt.Errorf("read thread file: %w", err)
	}

	fm, err := readThreadFrontmatter(threadFP)
	if err != nil {
		return fmt.Errorf("read frontmatter: %w", err)
	}

	threadID, err := uuid.Parse(fm.ID)
	if err != nil {
		return fmt.Errorf("parse thread ID: %w", err)
	}
	if threadID != id {
		return fmt.Errorf("thread ID mismatch")
	}

	topicID, err := s.topicIDByName(topicName)
	if err != nil {
		return fmt.Errorf("resolve topic ID: %w", err)
	}
	createdAt, _ := parseTimestamp(fm.CreatedAt)

	messages := parseThreadMessages(string(data))

	// Compute updated_at from messages
	updatedAt := createdAt
	for _, msg := range messages {
		if msg.CreatedAt.After(updatedAt) {
			updatedAt = msg.CreatedAt
		}
	}

	// Modify sticky and write back, all under the same lock
	thread := &models.Thread{
		ID:        id,
		TopicID:   topicID,
		Subject:   fm.Subject,
		CreatedAt: createdAt,
		CreatedBy: fm.CreatedBy,
		UpdatedAt: updatedAt,
		Sticky:    sticky,
	}

	content := renderThread(thread, topicName, messages)
	if err := atomicWrite(threadFP, []byte(content)); err != nil {
		return fmt.Errorf("write thread file: %w", err)
	}

	return nil
}

// topicNameByID looks up a topic name by its UUID.
func (s *MarkdownStore) topicNameByID(id uuid.UUID) (string, error) {
	entries, err := s.readTopics()
	if err != nil {
		return "", err
	}

	idStr := id.String()
	for _, e := range entries {
		if e.ID == idStr {
			return e.Name, nil
		}
	}
	return "", fmt.Errorf("topic not found: %s", id)
}

// readThreadFromFile reads a thread from a markdown file and computes updated_at.
func (s *MarkdownStore) readThreadFromFile(fp string, expectedID uuid.UUID) (*models.Thread, error) {
	fm, err := readThreadFrontmatter(fp)
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(fm.ID)
	if err != nil {
		return nil, fmt.Errorf("parse thread ID: %w", err)
	}

	if id != expectedID {
		return nil, fmt.Errorf("thread ID mismatch")
	}

	topicID, err := s.topicIDByName(fm.Topic)
	if err != nil {
		return nil, fmt.Errorf("resolve topic ID: %w", err)
	}

	createdAt, _ := parseTimestamp(fm.CreatedAt)

	// Compute updated_at from messages
	updatedAt := createdAt
	data, err := os.ReadFile(fp)
	if err == nil {
		messages := parseThreadMessages(string(data))
		for _, msg := range messages {
			if msg.CreatedAt.After(updatedAt) {
				updatedAt = msg.CreatedAt
			}
		}
	}

	return &models.Thread{
		ID:        id,
		TopicID:   topicID,
		Subject:   fm.Subject,
		CreatedAt: createdAt,
		CreatedBy: fm.CreatedBy,
		UpdatedAt: updatedAt,
		Sticky:    fm.Sticky,
	}, nil
}

// readThreadFromFileWithUpdatedAt reads a thread from a file without requiring an expected ID.
func (s *MarkdownStore) readThreadFromFileWithUpdatedAt(fp string) (*models.Thread, error) {
	fm, err := readThreadFrontmatter(fp)
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(fm.ID)
	if err != nil {
		return nil, fmt.Errorf("parse thread ID: %w", err)
	}

	return s.readThreadFromFile(fp, id)
}

// topicIDByName looks up a topic ID by its name.
func (s *MarkdownStore) topicIDByName(name string) (uuid.UUID, error) {
	entries, err := s.readTopics()
	if err != nil {
		return uuid.UUID{}, err
	}

	for _, e := range entries {
		if e.Name == name {
			id, err := uuid.Parse(e.ID)
			if err != nil {
				return uuid.UUID{}, err
			}
			return id, nil
		}
	}
	return uuid.UUID{}, fmt.Errorf("topic not found: %s", name)
}
