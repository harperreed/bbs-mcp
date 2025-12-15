// ABOUTME: Core data models for topics, threads, messages, attachments
// ABOUTME: Provides constructor functions for each model type

package models

import (
	"time"

	"github.com/google/uuid"
)

// Topic represents a message board category.
type Topic struct {
	ID          uuid.UUID
	Name        string
	Description string
	CreatedAt   time.Time
	CreatedBy   string
	Archived    bool
}

// Thread represents a discussion within a topic.
type Thread struct {
	ID        uuid.UUID
	TopicID   uuid.UUID
	Subject   string
	CreatedAt time.Time
	CreatedBy string
	Sticky    bool
}

// Message represents a post within a thread.
type Message struct {
	ID        uuid.UUID
	ThreadID  uuid.UUID
	Content   string
	CreatedAt time.Time
	CreatedBy string
	EditedAt  *time.Time
}

// Attachment represents a file attached to a message.
type Attachment struct {
	ID        uuid.UUID
	MessageID uuid.UUID
	Filename  string
	MimeType  string
	Data      []byte
	CreatedAt time.Time
}

// NewTopic creates a new topic with generated UUID and timestamp.
func NewTopic(name, description, createdBy string) *Topic {
	return &Topic{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		CreatedBy:   createdBy,
		Archived:    false,
	}
}

// NewThread creates a new thread with generated UUID and timestamp.
func NewThread(topicID uuid.UUID, subject, createdBy string) *Thread {
	return &Thread{
		ID:        uuid.New(),
		TopicID:   topicID,
		Subject:   subject,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
		Sticky:    false,
	}
}

// NewMessage creates a new message with generated UUID and timestamp.
func NewMessage(threadID uuid.UUID, content, createdBy string) *Message {
	return &Message{
		ID:        uuid.New(),
		ThreadID:  threadID,
		Content:   content,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
	}
}

// NewAttachment creates a new attachment with generated UUID and timestamp.
func NewAttachment(messageID uuid.UUID, filename, mimeType string, data []byte) *Attachment {
	return &Attachment{
		ID:        uuid.New(),
		MessageID: messageID,
		Filename:  filename,
		MimeType:  mimeType,
		Data:      data,
		CreatedAt: time.Now(),
	}
}
