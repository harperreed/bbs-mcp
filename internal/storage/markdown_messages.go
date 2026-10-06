// ABOUTME: Message CRUD operations for MarkdownStore
// ABOUTME: Messages are stored as sections within thread markdown files

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

// CreateMessage stores a new message by appending to the thread's markdown file.
func (s *MarkdownStore) CreateMessage(m *models.Message) error {
	return mdstore.WithLock(s.dataDir, func() error {
		locations, err := s.messageLocations(m.ID)
		if err != nil {
			return err
		}
		if len(locations) > 0 {
			return fmt.Errorf("insert message: duplicate message ID %s", m.ID)
		}

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

		// Parse existing messages before any rewrite so malformed v2 data is preserved.
		existingMessages, err := parseThreadMessages(string(data))
		if err != nil {
			return fmt.Errorf("parse thread messages: %w", err)
		}

		// Add new message
		newMsg := &parsedMessage{
			ID:        m.ID,
			CreatedBy: m.CreatedBy,
			CreatedAt: m.CreatedAt,
			EditedAt:  m.EditedAt,
			Content:   m.Content,
		}
		existingMessages = append(existingMessages, newMsg)

		// Rebuild the thread with activity from the create operation, not imported message time.
		fm, err := readThreadFrontmatter(threadFP)
		if err != nil {
			return fmt.Errorf("read frontmatter: %w", err)
		}

		createdAt, err := mdstore.ParseTime(fm.CreatedAt)
		if err != nil {
			return fmt.Errorf("parse thread created_at: %w", err)
		}
		topicID, err := s.topicIDByName(topicName)
		if err != nil {
			return fmt.Errorf("resolve topic ID: %w", err)
		}

		thread := &models.Thread{
			ID:        m.ThreadID,
			TopicID:   topicID,
			Subject:   fm.Subject,
			CreatedAt: createdAt,
			CreatedBy: fm.CreatedBy,
			UpdatedAt: time.Now().UTC(),
			Sticky:    fm.Sticky,
		}

		content, err := renderThread(thread, topicName, existingMessages)
		if err != nil {
			return fmt.Errorf("render thread: %w", err)
		}
		if err := mdstore.AtomicWrite(threadFP, []byte(content)); err != nil {
			return fmt.Errorf("write thread file: %w", err)
		}

		return nil
	})
}

// GetMessage retrieves a message by ID.
func (s *MarkdownStore) GetMessage(id uuid.UUID) (*models.Message, error) {
	location, err := s.uniqueMessageLocation(id)
	if err != nil {
		return nil, err
	}
	return location.message, nil
}

type messageLocation struct {
	threadPath string
	topicName  string
	message    *models.Message
}

func (s *MarkdownStore) messageLocations(id uuid.UUID) ([]messageLocation, error) {
	entries, err := s.readTopics()
	if err != nil {
		return nil, err
	}
	var locations []messageLocation
	for _, topic := range entries {
		matches, err := s.messageLocationsInTopic(topic.Name, id)
		if err != nil {
			return nil, err
		}
		locations = append(locations, matches...)
	}
	return locations, nil
}

func (s *MarkdownStore) messageLocationsInTopic(topicName string, id uuid.UUID) ([]messageLocation, error) {
	topicDir, err := s.safeTopicDirPath(topicName)
	if err != nil {
		return nil, err
	}
	dirEntries, err := os.ReadDir(topicDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []messageLocation{}, nil
		}
		return nil, fmt.Errorf("read topic directory: %w", err)
	}
	var locations []messageLocation
	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		threadPath, err := containedPath(s.dataDir, filepath.Join(topicDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		matches, err := messageLocationsInFile(threadPath, topicName, id)
		if err != nil {
			return nil, err
		}
		locations = append(locations, matches...)
	}
	return locations, nil
}

func messageLocationsInFile(threadPath string, topicName string, id uuid.UUID) ([]messageLocation, error) {
	data, err := os.ReadFile(threadPath)
	if err != nil {
		return nil, fmt.Errorf("read thread file %s: %w", threadPath, err)
	}
	fm, err := parseThreadFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse thread frontmatter %s: %w", threadPath, err)
	}
	threadID, err := uuid.Parse(fm.ID)
	if err != nil {
		return nil, fmt.Errorf("parse thread ID in %s: %w", threadPath, err)
	}
	messages, err := parseThreadMessages(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse thread messages in %s: %w", threadPath, err)
	}
	var locations []messageLocation
	for _, message := range messages {
		if message.ID != id {
			continue
		}
		locations = append(locations, messageLocation{
			threadPath: threadPath,
			topicName:  topicName,
			message: &models.Message{
				ID: message.ID, ThreadID: threadID, Content: message.Content,
				CreatedAt: message.CreatedAt, CreatedBy: message.CreatedBy, EditedAt: message.EditedAt,
			},
		})
	}
	return locations, nil
}

func (s *MarkdownStore) uniqueMessageLocation(id uuid.UUID) (*messageLocation, error) {
	locations, err := s.messageLocations(id)
	if err != nil {
		return nil, err
	}
	if len(locations) > 1 {
		return nil, fmt.Errorf("ambiguous message ID %s: UUID collision across %d messages", id, len(locations))
	}
	if len(locations) == 0 {
		return nil, fmt.Errorf("message not found: %s", id)
	}
	return &locations[0], nil
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

	parsed, err := parseThreadMessages(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse thread messages in %s: %w", threadFP, err)
	}

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
		location, err := s.uniqueMessageLocation(m.ID)
		if err != nil {
			return err
		}
		if location.threadPath != threadFP {
			return fmt.Errorf("message %s does not belong to thread %s", m.ID, m.ThreadID)
		}

		data, err := os.ReadFile(threadFP)
		if err != nil {
			return fmt.Errorf("read thread file: %w", err)
		}

		messages, err := parseThreadMessages(string(data))
		if err != nil {
			return fmt.Errorf("parse thread messages: %w", err)
		}

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
		createdAt, err := mdstore.ParseTime(fm.CreatedAt)
		if err != nil {
			return fmt.Errorf("parse thread created_at: %w", err)
		}
		topicID, err := s.topicIDByName(topicName)
		if err != nil {
			return fmt.Errorf("resolve topic ID: %w", err)
		}

		updatedAt, err := threadActivity(fm, messages)
		if err != nil {
			return fmt.Errorf("read thread activity: %w", err)
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

		content, err := renderThread(thread, topicName, messages)
		if err != nil {
			return fmt.Errorf("render thread: %w", err)
		}
		return mdstore.AtomicWrite(threadFP, []byte(content))
	})
}

// DeleteMessage deletes a message from its thread file.
func (s *MarkdownStore) DeleteMessage(id uuid.UUID) error {
	return mdstore.WithLock(s.dataDir, func() error {
		location, err := s.uniqueMessageLocation(id)
		if err != nil {
			return err
		}
		return s.deleteMessageFromFile(location.threadPath, id, location.topicName)
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

	messages, err := parseThreadMessages(string(data))
	if err != nil {
		return fmt.Errorf("parse thread messages: %w", err)
	}
	updatedAt, err := threadActivity(fm, messages)
	if err != nil {
		return fmt.Errorf("read thread activity: %w", err)
	}

	// Filter out the target message after attachment ownership is fully preflighted.
	found := false
	var attachmentDirs []string
	newMessages := make([]*parsedMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.ID == msgID {
			found = true
			attachmentDirs, err = s.attachmentDirsForMessage(topicName, msg.ID)
			if err != nil {
				return err
			}
			continue
		}
		newMessages = append(newMessages, msg)
	}

	if !found {
		return fmt.Errorf("message not found in file")
	}
	if err := preflightAttachmentRemoval(attachmentDirs); err != nil {
		return err
	}

	// Rebuild thread file
	createdAt, err := mdstore.ParseTime(fm.CreatedAt)
	if err != nil {
		return fmt.Errorf("parse thread created_at: %w", err)
	}
	topicID, err := s.topicIDByName(topicName)
	if err != nil {
		return fmt.Errorf("resolve topic ID: %w", err)
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

	content, err := renderThread(thread, topicName, newMessages)
	if err != nil {
		return fmt.Errorf("render thread: %w", err)
	}
	if err := mdstore.AtomicWrite(fp, []byte(content)); err != nil {
		return err
	}
	for _, attachmentDir := range attachmentDirs {
		if err := os.RemoveAll(attachmentDir); err != nil {
			return fmt.Errorf("remove message attachments: %w", err)
		}
	}
	return nil
}

// findThreadFile locates the file path and topic name for a given thread ID.
func (s *MarkdownStore) findThreadFile(threadID uuid.UUID) (string, string, error) {
	location, err := s.uniqueThreadLocation(threadID)
	if err != nil {
		return "", "", err
	}
	return location.path, location.topicName, nil
}
