// ABOUTME: Topic CRUD operations for MarkdownStore
// ABOUTME: Persists topics in _topics.yaml at the data directory root

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/harperreed/mdstore"

	"github.com/harper/bbs/internal/models"
)

// CreateTopic stores a new topic.
func (s *MarkdownStore) CreateTopic(t *models.Topic) error {
	return mdstore.WithLock(s.dataDir, func() error {
		entries, err := s.readTopics()
		if err != nil {
			return err
		}
		if index, err := topicEntryIndexByID(entries, t.ID); err != nil {
			return err
		} else if index >= 0 {
			return fmt.Errorf("insert topic: duplicate topic ID %s", t.ID)
		}

		// Check for duplicate name
		for _, e := range entries {
			if e.Name == t.Name {
				return fmt.Errorf("insert topic: topic name %q already exists", t.Name)
			}
		}

		topicDir, err := s.safeTopicDirPath(t.Name)
		if err != nil {
			return err
		}
		_, statErr := os.Stat(topicDir)
		dirCreated := os.IsNotExist(statErr)
		// Create the validated directory before publishing its metadata.
		if err := mdstore.EnsureDir(topicDir); err != nil {
			return fmt.Errorf("create topic directory: %w", err)
		}

		entries = append(entries, fromTopicModel(t))
		if err := s.writeTopics(entries); err != nil {
			if dirCreated {
				_ = os.Remove(topicDir)
			}
			return fmt.Errorf("write topics: %w", err)
		}

		return nil
	})
}

// GetTopic retrieves a topic by ID.
func (s *MarkdownStore) GetTopic(id uuid.UUID) (*models.Topic, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}

	index, err := topicEntryIndexByID(entries, id)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return nil, fmt.Errorf("topic not found: %s", id)
	}
	topic, err := entries[index].toModel()
	if err != nil {
		return nil, fmt.Errorf("parse topic entry: %w", err)
	}
	return topic, nil
}

func topicEntryIndexByID(entries []topicEntry, id uuid.UUID) (int, error) {
	match := -1
	for i, entry := range entries {
		storedID, err := uuid.Parse(entry.ID)
		if err != nil {
			return -1, fmt.Errorf("parse topic ID %q: %w", entry.ID, err)
		}
		if storedID != id {
			continue
		}
		if match >= 0 {
			return -1, fmt.Errorf("ambiguous topic ID %s: UUID collision", id)
		}
		match = i
	}
	return match, nil
}

// GetTopicByName finds a topic by its name.
func (s *MarkdownStore) GetTopicByName(name string) (*models.Topic, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.Name == name {
			topic, err := e.toModel()
			if err != nil {
				return nil, fmt.Errorf("parse topic entry: %w", err)
			}
			if _, err := topicEntryIndexByID(entries, topic.ID); err != nil {
				return nil, err
			}
			return topic, nil
		}
	}
	return nil, fmt.Errorf("topic not found: %s", name)
}

// ListTopics returns active topics when includeArchived is false and all topics when it is true.
func (s *MarkdownStore) ListTopics(includeArchived bool) ([]*models.Topic, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}

	converted := make([]*models.Topic, len(entries))
	for i := range entries {
		topic, err := entries[i].toModel()
		if err != nil {
			return nil, fmt.Errorf(
				"parse topic registry %q entry %d (%q): %w",
				s.topicsFilePath(), i+1, entries[i].Name, err,
			)
		}
		converted[i] = topic
	}

	type topicOccurrence struct {
		index int
		name  string
	}
	seen := make(map[uuid.UUID]topicOccurrence, len(converted))
	for i, topic := range converted {
		if first, ok := seen[topic.ID]; ok {
			return nil, fmt.Errorf(
				"validate topic registry %q entry %d (%q): ambiguous topic ID %s: UUID collision; "+
					"first occurrence entry %d (%q)",
				s.topicsFilePath(), i+1, entries[i].Name, topic.ID, first.index+1, first.name,
			)
		}
		seen[topic.ID] = topicOccurrence{index: i, name: entries[i].Name}
	}

	var topics []*models.Topic
	for _, topic := range converted {
		if !includeArchived && topic.Archived {
			continue
		}
		topics = append(topics, topic)
	}

	// Sort by name to match SQLite behavior
	sort.Slice(topics, func(i, j int) bool {
		return topics[i].Name < topics[j].Name
	})

	return topics, nil
}

// UpdateTopic updates an existing topic.
func (s *MarkdownStore) UpdateTopic(t *models.Topic) error {
	return mdstore.WithLock(s.dataDir, func() error {
		entries, err := s.readTopics()
		if err != nil {
			return err
		}

		index, err := topicEntryIndexByID(entries, t.ID)
		if err != nil {
			return err
		}
		if index < 0 {
			return fmt.Errorf("topic not found: %s", t.ID)
		}
		oldName := entries[index].Name
		for otherIndex, entry := range entries {
			if otherIndex != index && entry.Name == t.Name {
				return fmt.Errorf("update topic: topic name %q already exists", t.Name)
			}
		}
		entries[index] = fromTopicModel(t)
		oldDir, err := s.safeTopicDirPath(oldName)
		if err != nil {
			return err
		}
		if _, err := s.safeTopicDirPath(t.Name); err != nil {
			return err
		}

		var threadUpdates []threadTopicRename
		if oldName != t.Name {
			threadUpdates, err = prepareThreadTopicRename(s.dataDir, oldDir, oldName, t.Name, t.ID)
			if err != nil {
				return fmt.Errorf("prepare thread files before topic rename: %w", err)
			}
		}

		if oldName == t.Name {
			if err := s.writeTopics(entries); err != nil {
				return fmt.Errorf("write topics: %w", err)
			}
			return nil
		}

		return s.commitTopicRename(entries, oldName, t.Name, threadUpdates)
	})
}

func (s *MarkdownStore) commitTopicRename(
	entries []topicEntry,
	oldName string,
	newName string,
	threadUpdates []threadTopicRename,
) error {
	oldDir, err := s.safeTopicDirPath(oldName)
	if err != nil {
		return err
	}
	newDir, err := s.safeTopicDirPath(newName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(oldDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect topic directory: %w", err)
		}
		if err := s.writeTopics(entries); err != nil {
			return fmt.Errorf("write topics: %w", err)
		}
		return nil
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return fmt.Errorf("rename topic directory: %w", err)
	}

	written, err := writeThreadTopicRename(newDir, threadUpdates)
	if err != nil {
		rollbackErr := rollbackThreadTopicRename(oldDir, newDir, threadUpdates, written)
		return topicRenameCommitError("update thread frontmatter", err, rollbackErr)
	}
	if err := s.writeTopics(entries); err != nil {
		rollbackErr := rollbackThreadTopicRename(oldDir, newDir, threadUpdates, len(threadUpdates))
		return topicRenameCommitError("write topics", err, rollbackErr)
	}

	return nil
}

type threadTopicRename struct {
	filename string
	original []byte
	content  []byte
}

// prepareThreadTopicRename validates and renders every thread before persistent rename state changes.
func prepareThreadTopicRename(
	boardRoot string,
	topicDir string,
	oldName string,
	newName string,
	topicID uuid.UUID,
) ([]threadTopicRename, error) {
	dirEntries, err := os.ReadDir(topicDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read topic directory: %w", err)
	}

	updates := make([]threadTopicRename, 0, len(dirEntries))
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		fp, err := containedPath(boardRoot, filepath.Join(topicDir, de.Name()))
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(fp)
		if err != nil {
			return nil, fmt.Errorf("read thread file %s: %w", de.Name(), err)
		}
		fm, err := parseThreadFrontmatter(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse thread frontmatter %s: %w", de.Name(), err)
		}
		if fm.Topic != oldName {
			continue
		}
		messages, err := parseThreadMessages(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse thread messages %s: %w", de.Name(), err)
		}
		threadID, err := uuid.Parse(fm.ID)
		if err != nil {
			return nil, fmt.Errorf("parse thread ID %s: %w", de.Name(), err)
		}
		createdAt, err := mdstore.ParseTime(fm.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse thread created_at %s: %w", de.Name(), err)
		}
		updatedAt, err := threadActivity(fm, messages)
		if err != nil {
			return nil, fmt.Errorf("read thread activity %s: %w", de.Name(), err)
		}
		thread := &models.Thread{
			ID:        threadID,
			TopicID:   topicID,
			Subject:   fm.Subject,
			CreatedAt: createdAt,
			CreatedBy: fm.CreatedBy,
			UpdatedAt: updatedAt,
			Sticky:    fm.Sticky,
		}
		content, err := renderThread(thread, newName, messages)
		if err != nil {
			return nil, fmt.Errorf("render thread %s: %w", de.Name(), err)
		}
		updates = append(updates, threadTopicRename{
			filename: de.Name(),
			original: append([]byte(nil), data...),
			content:  []byte(content),
		})
	}
	return updates, nil
}

// writeThreadTopicRename commits prepared thread contents after the directory rename.
func writeThreadTopicRename(topicDir string, updates []threadTopicRename) (int, error) {
	for i, update := range updates {
		if err := mdstore.AtomicWrite(filepath.Join(topicDir, update.filename), update.content); err != nil {
			return i, fmt.Errorf("write thread file %s: %w", update.filename, err)
		}
	}
	return len(updates), nil
}

// rollbackThreadTopicRename restores rewritten files before moving the directory back.
func rollbackThreadTopicRename(
	oldDir string,
	newDir string,
	updates []threadTopicRename,
	written int,
) error {
	var rollbackErr error
	for i := 0; i < written; i++ {
		update := updates[i]
		if err := mdstore.AtomicWrite(filepath.Join(newDir, update.filename), update.original); err != nil && rollbackErr == nil {
			rollbackErr = fmt.Errorf("restore thread file %s: %w", update.filename, err)
		}
	}
	if err := os.Rename(newDir, oldDir); err != nil && rollbackErr == nil {
		rollbackErr = fmt.Errorf("restore topic directory: %w", err)
	}
	return rollbackErr
}

func topicRenameCommitError(operation string, commitErr error, rollbackErr error) error {
	if rollbackErr != nil {
		return fmt.Errorf("%s: %w; rollback failed: %w", operation, commitErr, rollbackErr)
	}
	return fmt.Errorf("%s: %w", operation, commitErr)
}

// DeleteTopic deletes a topic and its entire directory (cascades to threads).
func (s *MarkdownStore) DeleteTopic(id uuid.UUID) error {
	return mdstore.WithLock(s.dataDir, func() error {
		entries, err := s.readTopics()
		if err != nil {
			return err
		}

		index, err := topicEntryIndexByID(entries, id)
		if err != nil {
			return err
		}
		if index < 0 {
			return fmt.Errorf("topic not found: %s", id)
		}
		topicName := entries[index].Name
		newEntries := append([]topicEntry(nil), entries[:index]...)
		newEntries = append(newEntries, entries[index+1:]...)
		topicDir, err := s.safeTopicDirPath(topicName)
		if err != nil {
			return err
		}

		if err := s.writeTopics(newEntries); err != nil {
			return fmt.Errorf("write topics: %w", err)
		}

		// Remove topic directory and all contents
		if err := os.RemoveAll(topicDir); err != nil {
			return fmt.Errorf("remove topic directory: %w", err)
		}

		return nil
	})
}

// ArchiveTopic sets the archived status of a topic.
func (s *MarkdownStore) ArchiveTopic(id uuid.UUID, archived bool) error {
	return mdstore.WithLock(s.dataDir, func() error {
		entries, err := s.readTopics()
		if err != nil {
			return err
		}

		index, err := topicEntryIndexByID(entries, id)
		if err != nil {
			return err
		}
		if index < 0 {
			return fmt.Errorf("topic not found: %s", id)
		}
		entries[index].Archived = archived

		return s.writeTopics(entries)
	})
}
