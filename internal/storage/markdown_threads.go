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
	"github.com/harperreed/mdstore"

	"github.com/harper/bbs/internal/models"
)

// CreateThread stores a new thread as a markdown file.
func (s *MarkdownStore) CreateThread(t *models.Thread) error {
	return mdstore.WithLock(s.dataDir, func() error {
		locations, err := s.threadLocations(t.ID)
		if err != nil {
			return err
		}
		if len(locations) > 0 {
			return fmt.Errorf("insert thread: duplicate thread ID %s", t.ID)
		}

		// Look up topic name
		topicName, err := s.topicNameByID(t.TopicID)
		if err != nil {
			return fmt.Errorf("resolve topic for thread: %w", err)
		}

		// Ensure topic directory exists
		topicDir, err := s.safeTopicDirPath(topicName)
		if err != nil {
			return err
		}
		if err := mdstore.EnsureDir(topicDir); err != nil {
			return fmt.Errorf("create topic directory: %w", err)
		}

		// Generate filename
		filename := s.threadFileName(topicName, t.Subject, t.ID)
		fp, err := safeNamedPath(s.dataDir, topicDir, filename, "thread filename")
		if err != nil {
			return err
		}

		// Render the thread file (no messages yet)
		content, err := renderThread(t, topicName, nil)
		if err != nil {
			return fmt.Errorf("render thread: %w", err)
		}
		if err := mdstore.AtomicWrite(fp, []byte(content)); err != nil {
			return fmt.Errorf("write thread file: %w", err)
		}

		return nil
	})
}

// GetThread retrieves a thread by ID.
func (s *MarkdownStore) GetThread(id uuid.UUID) (*models.Thread, error) {
	location, err := s.uniqueThreadLocation(id)
	if err != nil {
		return nil, err
	}
	return s.readThreadFromFile(location.path, id)
}

type threadLocation struct {
	path      string
	topicName string
}

func (s *MarkdownStore) threadLocations(id uuid.UUID) ([]threadLocation, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}
	var locations []threadLocation
	for _, topic := range entries {
		topicDir, err := s.safeTopicDirPath(topic.Name)
		if err != nil {
			return nil, err
		}
		dirEntries, err := os.ReadDir(topicDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read topic directory: %w", err)
		}
		for _, entry := range dirEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			path, err := containedPath(s.dataDir, filepath.Join(topicDir, entry.Name()))
			if err != nil {
				return nil, err
			}
			fm, err := readThreadFrontmatter(path)
			if err != nil {
				return nil, fmt.Errorf("read thread frontmatter %s: %w", path, err)
			}
			storedID, err := uuid.Parse(fm.ID)
			if err != nil {
				return nil, fmt.Errorf("parse thread ID in %s: %w", path, err)
			}
			if storedID == id {
				locations = append(locations, threadLocation{path: path, topicName: topic.Name})
			}
		}
	}
	return locations, nil
}

func (s *MarkdownStore) uniqueThreadLocation(id uuid.UUID) (*threadLocation, error) {
	locations, err := s.threadLocations(id)
	if err != nil {
		return nil, err
	}
	if len(locations) > 1 {
		return nil, fmt.Errorf("ambiguous thread ID %s: UUID collision across %d files", id, len(locations))
	}
	if len(locations) == 0 {
		return nil, fmt.Errorf("thread not found: %s", id)
	}
	return &locations[0], nil
}

// ListThreads returns all threads for a topic, sorted by sticky, updated_at DESC, then UUID.
func (s *MarkdownStore) ListThreads(topicID uuid.UUID) ([]*models.Thread, error) {
	topicName, err := s.topicNameByID(topicID)
	if err != nil {
		return nil, fmt.Errorf("resolve topic: %w", err)
	}

	topicDir, err := s.safeTopicDirPath(topicName)
	if err != nil {
		return nil, err
	}
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
		fp, err := containedPath(s.dataDir, filepath.Join(topicDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		thread, err := s.readThreadFromFileWithUpdatedAt(fp)
		if err != nil {
			return nil, fmt.Errorf("read thread %s: %w", fp, err)
		}
		if thread.TopicID != topicID {
			continue
		}
		threads = append(threads, thread)
	}

	// Sort: sticky first, then by updated_at DESC, then UUID.
	sort.Slice(threads, func(i, j int) bool {
		if threads[i].Sticky != threads[j].Sticky {
			return threads[i].Sticky
		}
		if threads[i].UpdatedAt.Equal(threads[j].UpdatedAt) {
			return threads[i].ID.String() < threads[j].ID.String()
		}
		return threads[j].UpdatedAt.Before(threads[i].UpdatedAt)
	})

	return threads, nil
}

// UpdateThread updates an existing thread.
func (s *MarkdownStore) UpdateThread(t *models.Thread) error {
	return mdstore.WithLock(s.dataDir, func() error {
		location, err := s.uniqueThreadLocation(t.ID)
		if err != nil {
			return err
		}
		topicName, err := s.topicNameByID(t.TopicID)
		if err != nil {
			return fmt.Errorf("resolve topic: %w", err)
		}
		if location.topicName != topicName {
			return fmt.Errorf("thread %s does not belong to topic %s", t.ID, t.TopicID)
		}
		oldPath := location.path

		// Read existing messages
		data, err := os.ReadFile(oldPath)
		if err != nil {
			return fmt.Errorf("read thread file: %w", err)
		}

		messages, err := parseThreadMessages(string(data))
		if err != nil {
			return fmt.Errorf("parse thread messages: %w", err)
		}

		// Set updated_at to now
		t.UpdatedAt = time.Now().UTC()

		// Render updated thread
		content, err := renderThread(t, topicName, messages)
		if err != nil {
			return fmt.Errorf("render thread: %w", err)
		}

		// Determine new filename (in case subject changed)
		newFilename := s.threadFileName(topicName, t.Subject, t.ID)
		topicDir, err := s.safeTopicDirPath(topicName)
		if err != nil {
			return err
		}
		newPath, err := safeNamedPath(s.dataDir, topicDir, newFilename, "thread filename")
		if err != nil {
			return err
		}

		if err := mdstore.AtomicWrite(newPath, []byte(content)); err != nil {
			return fmt.Errorf("write thread file: %w", err)
		}

		// Remove old file if path changed
		if oldPath != newPath {
			os.Remove(oldPath)
		}

		return nil
	})
}

// DeleteThread deletes a thread (removes its markdown file).
func (s *MarkdownStore) DeleteThread(id uuid.UUID) error {
	return mdstore.WithLock(s.dataDir, func() error {
		location, err := s.uniqueThreadLocation(id)
		if err != nil {
			return err
		}
		return s.deleteThreadFile(location.path, location.topicName)
	})
}

func (s *MarkdownStore) deleteThreadFile(path string, topicName string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read thread before delete: %w", err)
	}
	messages, err := parseThreadMessages(string(data))
	if err != nil {
		return fmt.Errorf("parse thread before delete: %w", err)
	}
	var attachmentDirs []string
	for _, msg := range messages {
		dirs, err := s.attachmentDirsForMessage(topicName, msg.ID)
		if err != nil {
			return err
		}
		attachmentDirs = append(attachmentDirs, dirs...)
	}
	if err := preflightAttachmentRemoval(attachmentDirs); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete thread file: %w", err)
	}
	for _, attachmentDir := range attachmentDirs {
		if err := os.RemoveAll(attachmentDir); err != nil {
			return fmt.Errorf("remove thread attachments: %w", err)
		}
	}
	return nil
}

// SetThreadSticky sets the sticky status of a thread.
// Performs the read-modify-write under a single lock hold to avoid TOCTOU races.
func (s *MarkdownStore) SetThreadSticky(id uuid.UUID, sticky bool) error {
	return mdstore.WithLock(s.dataDir, func() error {
		threadFP, topicName, err := s.findThreadFile(id)
		if err != nil {
			return err
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
		createdAt, err := mdstore.ParseTime(fm.CreatedAt)
		if err != nil {
			return fmt.Errorf("parse thread created_at: %w", err)
		}

		messages, err := parseThreadMessages(string(data))
		if err != nil {
			return fmt.Errorf("parse thread messages: %w", err)
		}

		updatedAt, err := threadActivity(fm, messages)
		if err != nil {
			return fmt.Errorf("read thread activity: %w", err)
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

		content, err := renderThread(thread, topicName, messages)
		if err != nil {
			return fmt.Errorf("render thread: %w", err)
		}
		if err := mdstore.AtomicWrite(threadFP, []byte(content)); err != nil {
			return fmt.Errorf("write thread file: %w", err)
		}

		return nil
	})
}

// topicNameByID looks up a topic name by its UUID.
func (s *MarkdownStore) topicNameByID(id uuid.UUID) (string, error) {
	entries, err := s.readTopics()
	if err != nil {
		return "", err
	}

	index, err := topicEntryIndexByID(entries, id)
	if err != nil {
		return "", err
	}
	if index < 0 {
		return "", fmt.Errorf("topic not found: %s", id)
	}
	if _, err := s.safeTopicDirPath(entries[index].Name); err != nil {
		return "", err
	}
	return entries[index].Name, nil
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

	createdAt, err := mdstore.ParseTime(fm.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse thread created_at: %w", err)
	}

	data, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("read thread file: %w", err)
	}
	messages, err := parseThreadMessages(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse thread messages in %s: %w", fp, err)
	}
	updatedAt, err := threadActivity(fm, messages)
	if err != nil {
		return nil, fmt.Errorf("read thread activity in %s: %w", fp, err)
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
