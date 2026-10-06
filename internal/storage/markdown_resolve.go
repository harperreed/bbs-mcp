// ABOUTME: Resolution helpers for MarkdownStore
// ABOUTME: Implements fuzzy lookup by full UUID, name, or UUID prefix for topics, threads, and messages

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/harper/bbs/internal/models"
)

// ResolveTopic finds a topic by ID, ID prefix, or name.
func (s *MarkdownStore) ResolveTopic(idOrName string) (*models.Topic, error) {
	// Try as full UUID first
	if id, err := uuid.Parse(idOrName); err == nil {
		return s.GetTopic(id)
	}

	// Try by name
	if topic, err := s.GetTopicByName(idOrName); err == nil {
		return topic, nil
	}

	// Try as ID prefix
	topics, err := s.ListTopics(true)
	if err != nil {
		return nil, err
	}

	var matches []*models.Topic
	for _, t := range topics {
		if strings.HasPrefix(t.ID.String(), idOrName) {
			matches = append(matches, t)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("topic not found: %s", idOrName)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous topic ID prefix '%s' matches %d topics", idOrName, len(matches))
	}
}

// ResolveThread finds a thread by ID or ID prefix.
func (s *MarkdownStore) ResolveThread(idPrefix string) (*models.Thread, error) {
	// Try as full UUID first
	if id, err := uuid.Parse(idPrefix); err == nil {
		return s.GetThread(id)
	}

	// Try as ID prefix - scan all threads across all topics
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}

	var matches []*models.Thread
	for _, e := range entries {
		topicDir, err := s.safeTopicDirPath(e.Name)
		if err != nil {
			return nil, err
		}
		dirEntries, err := os.ReadDir(topicDir)
		if err != nil {
			return nil, fmt.Errorf("read registered topic directory %s: %w", topicDir, err)
		}

		for _, de := range dirEntries {
			if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
				continue
			}
			fp, err := containedPath(s.dataDir, filepath.Join(topicDir, de.Name()))
			if err != nil {
				return nil, err
			}
			fm, err := readThreadFrontmatter(fp)
			if err != nil {
				return nil, fmt.Errorf("read thread frontmatter %s: %w", fp, err)
			}
			threadID, err := uuid.Parse(fm.ID)
			if err != nil {
				return nil, fmt.Errorf("parse thread ID in %s: %w", fp, err)
			}
			if !strings.HasPrefix(fm.ID, idPrefix) {
				continue
			}
			thread, err := s.readThreadFromFile(fp, threadID)
			if err != nil {
				return nil, fmt.Errorf("read thread %s: %w", fp, err)
			}
			matches = append(matches, thread)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("thread not found: %s", idPrefix)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous thread ID prefix '%s' matches %d threads", idPrefix, len(matches))
	}
}

// ResolveMessage finds a message by ID or ID prefix.
func (s *MarkdownStore) ResolveMessage(idPrefix string) (*models.Message, error) {
	// Try as full UUID first
	if id, err := uuid.Parse(idPrefix); err == nil {
		return s.GetMessage(id)
	}

	// Try as ID prefix - scan all messages across all topics and threads
	matches, err := s.findMessagesByPrefix(idPrefix)
	if err != nil {
		return nil, err
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("message not found: %s", idPrefix)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous message ID prefix '%s' matches %d messages", idPrefix, len(matches))
	}
}

// findMessagesByPrefix scans all thread files for messages matching an ID prefix.
func (s *MarkdownStore) findMessagesByPrefix(idPrefix string) ([]*models.Message, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}
	topics := make([]*models.Topic, 0, len(entries))
	for i := range entries {
		topic, err := entries[i].toModel()
		if err != nil {
			return nil, fmt.Errorf("parse registered topic %q: %w", entries[i].Name, err)
		}
		if _, err := topicEntryIndexByID(entries, topic.ID); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}

	var matches []*models.Message
	for _, topic := range topics {
		topicDir, err := s.safeTopicDirPath(topic.Name)
		if err != nil {
			return nil, err
		}
		dirEntries, err := os.ReadDir(topicDir)
		if err != nil {
			return nil, fmt.Errorf("read registered topic directory %s: %w", topicDir, err)
		}

		for _, de := range dirEntries {
			if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
				continue
			}
			fp, err := containedPath(s.dataDir, filepath.Join(topicDir, de.Name()))
			if err != nil {
				return nil, err
			}
			found, err := s.findMessagesInFile(fp, idPrefix)
			if err != nil {
				return nil, fmt.Errorf("read messages from %s: %w", fp, err)
			}
			matches = append(matches, found...)
		}
	}
	return matches, nil
}

// findMessagesInFile searches a single thread file for messages matching an ID prefix.
func (s *MarkdownStore) findMessagesInFile(fp string, idPrefix string) ([]*models.Message, error) {
	data, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("read thread file: %w", err)
	}

	fm, err := parseThreadFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse thread frontmatter: %w", err)
	}
	threadID, err := uuid.Parse(fm.ID)
	if err != nil {
		return nil, fmt.Errorf("parse thread ID: %w", err)
	}

	messages, err := parseThreadMessages(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse thread messages: %w", err)
	}

	var matches []*models.Message
	for _, msg := range messages {
		if strings.HasPrefix(msg.ID.String(), idPrefix) {
			matches = append(matches, &models.Message{
				ID:        msg.ID,
				ThreadID:  threadID,
				Content:   msg.Content,
				CreatedAt: msg.CreatedAt,
				CreatedBy: msg.CreatedBy,
				EditedAt:  msg.EditedAt,
			})
		}
	}
	return matches, nil
}
