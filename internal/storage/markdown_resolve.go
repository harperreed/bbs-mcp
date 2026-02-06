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
		topicDir := s.topicDirPath(e.Name)
		dirEntries, err := os.ReadDir(topicDir)
		if err != nil {
			continue
		}

		for _, de := range dirEntries {
			if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
				continue
			}
			fp := filepath.Join(topicDir, de.Name())
			fm, err := readThreadFrontmatter(fp)
			if err != nil {
				continue
			}
			if strings.HasPrefix(fm.ID, idPrefix) {
				threadID, err := uuid.Parse(fm.ID)
				if err != nil {
					continue
				}
				thread, err := s.readThreadFromFile(fp, threadID)
				if err != nil {
					continue
				}
				matches = append(matches, thread)
			}
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

	var matches []*models.Message
	for _, e := range entries {
		topicDir := s.topicDirPath(e.Name)
		dirEntries, err := os.ReadDir(topicDir)
		if err != nil {
			continue
		}

		for _, de := range dirEntries {
			if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
				continue
			}
			found := s.findMessagesInFile(filepath.Join(topicDir, de.Name()), idPrefix)
			matches = append(matches, found...)
		}
	}
	return matches, nil
}

// findMessagesInFile searches a single thread file for messages matching an ID prefix.
func (s *MarkdownStore) findMessagesInFile(fp string, idPrefix string) []*models.Message {
	data, err := os.ReadFile(fp)
	if err != nil {
		return nil
	}

	fm, err := parseThreadFrontmatter(string(data))
	if err != nil {
		return nil
	}
	threadID, err := uuid.Parse(fm.ID)
	if err != nil {
		return nil
	}

	messages := parseThreadMessages(string(data))

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
	return matches
}
