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
	"github.com/harper/suite/mdstore"

	"github.com/harper/bbs/internal/models"
)

// CreateTopic stores a new topic.
func (s *MarkdownStore) CreateTopic(t *models.Topic) error {
	return mdstore.WithLock(s.dataDir, func() error {
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
		if err := mdstore.EnsureDir(topicDir); err != nil {
			return fmt.Errorf("create topic directory: %w", err)
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

	idStr := id.String()
	for _, e := range entries {
		if e.ID == idStr {
			topic, err := e.toModel()
			if err != nil {
				return nil, fmt.Errorf("parse topic entry: %w", err)
			}
			return topic, nil
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
			topic, err := e.toModel()
			if err != nil {
				return nil, fmt.Errorf("parse topic entry: %w", err)
			}
			return topic, nil
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
		topic, err := e.toModel()
		if err != nil {
			// Skip malformed entries so one corrupt entry doesn't break listing
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

		// Rename directory if name changed and update thread frontmatter
		if oldName != t.Name {
			oldDir := s.topicDirPath(oldName)
			newDir := s.topicDirPath(t.Name)
			if _, err := os.Stat(oldDir); err == nil {
				if err := os.Rename(oldDir, newDir); err != nil {
					return fmt.Errorf("rename topic directory: %w", err)
				}
			}

			// Update the topic field in all thread frontmatter files
			if err := s.updateThreadTopicNames(newDir, oldName, t.Name); err != nil {
				return fmt.Errorf("update thread frontmatter after topic rename: %w", err)
			}
		}

		return nil
	})
}

// updateThreadTopicNames iterates all .md files in a topic directory and updates
// their frontmatter topic field from oldName to newName.
func (s *MarkdownStore) updateThreadTopicNames(topicDir, oldName, newName string) error {
	dirEntries, err := os.ReadDir(topicDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read topic directory: %w", err)
	}

	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		fp := filepath.Join(topicDir, de.Name())
		if err := s.updateThreadFileTopicName(fp, oldName, newName); err != nil {
			// Skip files that fail to parse rather than aborting the entire rename
			continue
		}
	}
	return nil
}

// updateThreadFileTopicName reads a single thread file, updates the topic field
// in its frontmatter, and writes it back atomically.
func (s *MarkdownStore) updateThreadFileTopicName(fp, oldName, newName string) error {
	fm, err := readThreadFrontmatter(fp)
	if err != nil {
		return err
	}
	if fm.Topic != oldName {
		return nil
	}

	data, err := os.ReadFile(fp)
	if err != nil {
		return err
	}

	messages := parseThreadMessages(string(data))

	threadID, err := uuid.Parse(fm.ID)
	if err != nil {
		return fmt.Errorf("parse thread ID: %w", err)
	}
	topicID, err := s.topicIDByName(newName)
	if err != nil {
		return fmt.Errorf("resolve topic ID: %w", err)
	}
	createdAt, _ := mdstore.ParseTime(fm.CreatedAt)

	// Compute updated_at from messages
	updatedAt := createdAt
	for _, msg := range messages {
		if msg.CreatedAt.After(updatedAt) {
			updatedAt = msg.CreatedAt
		}
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

	content := renderThread(thread, newName, messages)
	return mdstore.AtomicWrite(fp, []byte(content))
}

// DeleteTopic deletes a topic and its entire directory (cascades to threads).
func (s *MarkdownStore) DeleteTopic(id uuid.UUID) error {
	return mdstore.WithLock(s.dataDir, func() error {
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
	})
}

// ArchiveTopic sets the archived status of a topic.
func (s *MarkdownStore) ArchiveTopic(id uuid.UUID, archived bool) error {
	return mdstore.WithLock(s.dataDir, func() error {
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
	})
}
