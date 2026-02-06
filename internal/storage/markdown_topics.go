// ABOUTME: Topic CRUD operations for MarkdownStore
// ABOUTME: Persists topics in _topics.yaml at the data directory root

package storage

import (
	"fmt"
	"os"
	"sort"

	"github.com/google/uuid"

	"github.com/harper/bbs/internal/models"
)

// CreateTopic stores a new topic.
func (s *MarkdownStore) CreateTopic(t *models.Topic) error {
	lock, err := s.lockFile()
	if err != nil {
		return err
	}
	defer unlockFile(lock)

	entries, err := s.readTopics()
	if err != nil {
		return err
	}

	// Check for duplicate name
	for _, e := range entries {
		if e.Name == t.Name {
			return fmt.Errorf("insert topic: topic name %q already exists", t.Name)
		}
	}

	entries = append(entries, fromTopicModel(t))
	if err := s.writeTopics(entries); err != nil {
		return fmt.Errorf("write topics: %w", err)
	}

	// Create topic directory
	topicDir := s.topicDirPath(t.Name)
	if err := os.MkdirAll(topicDir, 0750); err != nil {
		return fmt.Errorf("create topic directory: %w", err)
	}

	return nil
}

// GetTopic retrieves a topic by ID.
func (s *MarkdownStore) GetTopic(id uuid.UUID) (*models.Topic, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}

	idStr := id.String()
	for _, e := range entries {
		if e.ID == idStr {
			return e.toModel(), nil
		}
	}
	return nil, fmt.Errorf("topic not found: %s", id)
}

// GetTopicByName finds a topic by its name.
func (s *MarkdownStore) GetTopicByName(name string) (*models.Topic, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.Name == name {
			return e.toModel(), nil
		}
	}
	return nil, fmt.Errorf("topic not found: %s", name)
}

// ListTopics returns all topics, optionally including archived ones.
func (s *MarkdownStore) ListTopics(includeArchived bool) ([]*models.Topic, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}

	var topics []*models.Topic
	for _, e := range entries {
		if !includeArchived && e.Archived {
			continue
		}
		topics = append(topics, e.toModel())
	}

	// Sort by name to match SQLite behavior
	sort.Slice(topics, func(i, j int) bool {
		return topics[i].Name < topics[j].Name
	})

	return topics, nil
}

// UpdateTopic updates an existing topic.
func (s *MarkdownStore) UpdateTopic(t *models.Topic) error {
	lock, err := s.lockFile()
	if err != nil {
		return err
	}
	defer unlockFile(lock)

	entries, err := s.readTopics()
	if err != nil {
		return err
	}

	idStr := t.ID.String()
	found := false
	var oldName string
	for i, e := range entries {
		if e.ID == idStr {
			oldName = e.Name
			entries[i] = fromTopicModel(t)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("topic not found: %s", t.ID)
	}

	if err := s.writeTopics(entries); err != nil {
		return fmt.Errorf("write topics: %w", err)
	}

	// Rename directory if name changed
	if oldName != t.Name {
		oldDir := s.topicDirPath(oldName)
		newDir := s.topicDirPath(t.Name)
		if _, err := os.Stat(oldDir); err == nil {
			if err := os.Rename(oldDir, newDir); err != nil {
				return fmt.Errorf("rename topic directory: %w", err)
			}
		}
	}

	return nil
}

// DeleteTopic deletes a topic and its entire directory (cascades to threads).
func (s *MarkdownStore) DeleteTopic(id uuid.UUID) error {
	lock, err := s.lockFile()
	if err != nil {
		return err
	}
	defer unlockFile(lock)

	entries, err := s.readTopics()
	if err != nil {
		return err
	}

	idStr := id.String()
	found := false
	var topicName string
	newEntries := make([]topicEntry, 0, len(entries))
	for _, e := range entries {
		if e.ID == idStr {
			found = true
			topicName = e.Name
			continue
		}
		newEntries = append(newEntries, e)
	}

	if !found {
		return fmt.Errorf("topic not found: %s", id)
	}

	if err := s.writeTopics(newEntries); err != nil {
		return fmt.Errorf("write topics: %w", err)
	}

	// Remove topic directory and all contents
	topicDir := s.topicDirPath(topicName)
	if err := os.RemoveAll(topicDir); err != nil {
		return fmt.Errorf("remove topic directory: %w", err)
	}

	return nil
}

// ArchiveTopic sets the archived status of a topic.
func (s *MarkdownStore) ArchiveTopic(id uuid.UUID, archived bool) error {
	lock, err := s.lockFile()
	if err != nil {
		return err
	}
	defer unlockFile(lock)

	entries, err := s.readTopics()
	if err != nil {
		return err
	}

	idStr := id.String()
	found := false
	for i, e := range entries {
		if e.ID == idStr {
			entries[i].Archived = archived
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("topic not found: %s", id)
	}

	return s.writeTopics(entries)
}
