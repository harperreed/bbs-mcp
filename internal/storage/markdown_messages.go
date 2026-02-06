// ABOUTME: Message CRUD operations for MarkdownStore
// ABOUTME: Messages are stored as sections within thread markdown files

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

// CreateMessage stores a new message by appending to the thread's markdown file.
func (s *MarkdownStore) CreateMessage(m *models.Message) error {
	return mdstore.WithLock(s.dataDir, func() error {
		// Find the thread file
		threadFP, topicName, err := s.findThreadFile(m.ThreadID)
		if err != nil {
			return fmt.Errorf("find thread for message: %w", err)
		}

		// Read existing file
		data, err := os.ReadFile(threadFP)
		if err != nil {
			return fmt.Errorf("read thread file: %w", err)
		}

		// Parse existing messages
		existingMessages := parseThreadMessages(string(data))

		// Add new message
		newMsg := &parsedMessage{
			ID:        m.ID,
			CreatedBy: m.CreatedBy,
			CreatedAt: m.CreatedAt,
			EditedAt:  m.EditedAt,
			Content:   m.Content,
		}
		existingMessages = append(existingMessages, newMsg)

		// Rebuild thread with updated_at from latest message
		fm, err := readThreadFrontmatter(threadFP)
		if err != nil {
			return fmt.Errorf("read frontmatter: %w", err)
		}

		createdAt, _ := mdstore.ParseTime(fm.CreatedAt)
		topicID, _ := s.topicIDByName(topicName)

		thread := &models.Thread{
			ID:        m.ThreadID,
			TopicID:   topicID,
			Subject:   fm.Subject,
			CreatedAt: createdAt,
			CreatedBy: fm.CreatedBy,
			UpdatedAt: m.CreatedAt,
			Sticky:    fm.Sticky,
		}

		content := renderThread(thread, topicName, existingMessages)
		if err := mdstore.AtomicWrite(threadFP, []byte(content)); err != nil {
			return fmt.Errorf("write thread file: %w", err)
		}

		return nil
	})
}

// GetMessage retrieves a message by ID.
func (s *MarkdownStore) GetMessage(id uuid.UUID) (*models.Message, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}

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
			data, err := os.ReadFile(fp)
			if err != nil {
				continue
			}

			fm, err := parseThreadFrontmatter(string(data))
			if err != nil {
				continue
			}
			threadID, err := uuid.Parse(fm.ID)
			if err != nil {
				continue
			}

			messages := parseThreadMessages(string(data))
			for _, msg := range messages {
				if msg.ID == id {
					return &models.Message{
						ID:        msg.ID,
						ThreadID:  threadID,
						Content:   msg.Content,
						CreatedAt: msg.CreatedAt,
						CreatedBy: msg.CreatedBy,
						EditedAt:  msg.EditedAt,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("message not found: %s", id)
}

// ListMessages returns all messages for a thread, sorted by created_at ASC.
func (s *MarkdownStore) ListMessages(threadID uuid.UUID) ([]*models.Message, error) {
	threadFP, _, err := s.findThreadFile(threadID)
	if err != nil {
		return nil, fmt.Errorf("find thread: %w", err)
	}

	data, err := os.ReadFile(threadFP)
	if err != nil {
		return nil, fmt.Errorf("read thread file: %w", err)
	}

	parsed := parseThreadMessages(string(data))

	// Convert parsed messages to model messages
	var messages []*models.Message
	for _, msg := range parsed {
		messages = append(messages, &models.Message{
			ID:        msg.ID,
			ThreadID:  threadID,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
			CreatedBy: msg.CreatedBy,
			EditedAt:  msg.EditedAt,
		})
	}

	// Explicitly sort by created_at ASC to match SqliteStore behavior
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})

	return messages, nil
}

// UpdateMessage updates an existing message in its thread file.
func (s *MarkdownStore) UpdateMessage(m *models.Message) error {
	return mdstore.WithLock(s.dataDir, func() error {
		threadFP, topicName, err := s.findThreadFile(m.ThreadID)
		if err != nil {
			return fmt.Errorf("find thread: %w", err)
		}

		data, err := os.ReadFile(threadFP)
		if err != nil {
			return fmt.Errorf("read thread file: %w", err)
		}

		messages := parseThreadMessages(string(data))

		// Find and update the message
		found := false
		for i, msg := range messages {
			if msg.ID == m.ID {
				messages[i].Content = m.Content
				messages[i].EditedAt = m.EditedAt
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("message not found: %s", m.ID)
		}

		// Rebuild thread file
		fm, err := readThreadFrontmatter(threadFP)
		if err != nil {
			return fmt.Errorf("read frontmatter: %w", err)
		}
		createdAt, _ := mdstore.ParseTime(fm.CreatedAt)
		topicID, _ := s.topicIDByName(topicName)

		// Compute updated_at from latest message
		updatedAt := createdAt
		for _, msg := range messages {
			if msg.CreatedAt.After(updatedAt) {
				updatedAt = msg.CreatedAt
			}
		}

		thread := &models.Thread{
			ID:        m.ThreadID,
			TopicID:   topicID,
			Subject:   fm.Subject,
			CreatedAt: createdAt,
			CreatedBy: fm.CreatedBy,
			UpdatedAt: updatedAt,
			Sticky:    fm.Sticky,
		}

		content := renderThread(thread, topicName, messages)
		return mdstore.AtomicWrite(threadFP, []byte(content))
	})
}

// DeleteMessage deletes a message from its thread file.
func (s *MarkdownStore) DeleteMessage(id uuid.UUID) error {
	return mdstore.WithLock(s.dataDir, func() error {
		// Find the message across all threads
		entries, err := s.readTopics()
		if err != nil {
			return err
		}

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
				if err := s.deleteMessageFromFile(fp, id, e.Name); err == nil {
					return nil
				}
			}
		}

		return fmt.Errorf("message not found: %s", id)
	})
}

// deleteMessageFromFile removes a message from a specific thread file.
// Returns nil if the message was found and removed, error otherwise.
func (s *MarkdownStore) deleteMessageFromFile(fp string, msgID uuid.UUID, topicName string) error {
	data, err := os.ReadFile(fp)
	if err != nil {
		return err
	}

	fm, err := parseThreadFrontmatter(string(data))
	if err != nil {
		return err
	}
	threadID, err := uuid.Parse(fm.ID)
	if err != nil {
		return err
	}

	messages := parseThreadMessages(string(data))

	// Filter out the target message
	found := false
	newMessages := make([]*parsedMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.ID == msgID {
			found = true
			// Remove attachments for this message
			prefix := msg.ID.String()[:8]
			attDir := s.attachmentDirPath(topicName, prefix)
			os.RemoveAll(attDir)
			continue
		}
		newMessages = append(newMessages, msg)
	}

	if !found {
		return fmt.Errorf("message not found in file")
	}

	// Rebuild thread file
	createdAt, _ := mdstore.ParseTime(fm.CreatedAt)
	topicID, _ := s.topicIDByName(topicName)

	updatedAt := createdAt
	for _, msg := range newMessages {
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

	content := renderThread(thread, topicName, newMessages)
	return mdstore.AtomicWrite(fp, []byte(content))
}

// findThreadFile locates the file path and topic name for a given thread ID.
func (s *MarkdownStore) findThreadFile(threadID uuid.UUID) (string, string, error) {
	entries, err := s.readTopics()
	if err != nil {
		return "", "", err
	}

	for _, e := range entries {
		fp, err := s.threadFilePath(e.Name, threadID)
		if err != nil {
			continue
		}
		return fp, e.Name, nil
	}
	return "", "", fmt.Errorf("thread not found: %s", threadID)
}
