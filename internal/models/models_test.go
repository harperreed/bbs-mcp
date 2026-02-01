// ABOUTME: Tests for BBS data models
// ABOUTME: Verifies model creation, validation, and UUID parsing

package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewTopic(t *testing.T) {
	tests := []struct {
		name        string
		topicName   string
		description string
		createdBy   string
	}{
		{
			name:        "basic topic",
			topicName:   "general",
			description: "General discussion",
			createdBy:   "harper@cli",
		},
		{
			name:        "empty description",
			topicName:   "support",
			description: "",
			createdBy:   "agent@mcp",
		},
		{
			name:        "special characters",
			topicName:   "test-topic_123",
			description: "Topic with special chars: !@#$%",
			createdBy:   "user@tui",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic := NewTopic(tt.topicName, tt.description, tt.createdBy)

			if topic.ID == uuid.Nil {
				t.Error("expected non-nil UUID")
			}
			if topic.Name != tt.topicName {
				t.Errorf("expected name %q, got %q", tt.topicName, topic.Name)
			}
			if topic.Description != tt.description {
				t.Errorf("expected description %q, got %q", tt.description, topic.Description)
			}
			if topic.CreatedBy != tt.createdBy {
				t.Errorf("expected createdBy %q, got %q", tt.createdBy, topic.CreatedBy)
			}
			if topic.Archived {
				t.Error("expected archived to be false")
			}
			if topic.CreatedAt.IsZero() {
				t.Error("expected CreatedAt to be set")
			}
		})
	}
}

func TestNewThread(t *testing.T) {
	tests := []struct {
		name      string
		subject   string
		createdBy string
	}{
		{
			name:      "basic thread",
			subject:   "Test subject",
			createdBy: "harper@cli",
		},
		{
			name:      "long subject",
			subject:   "This is a really long subject that might be used for detailed discussions about complex topics",
			createdBy: "claude@mcp",
		},
		{
			name:      "unicode subject",
			subject:   "Thread with emoji: 🎉 and unicode: 你好",
			createdBy: "user@tui",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topicID := uuid.New()
			thread := NewThread(topicID, tt.subject, tt.createdBy)

			if thread.ID == uuid.Nil {
				t.Error("expected non-nil UUID")
			}
			if thread.TopicID != topicID {
				t.Error("expected topicID to match")
			}
			if thread.Subject != tt.subject {
				t.Errorf("expected subject %q, got %q", tt.subject, thread.Subject)
			}
			if thread.CreatedBy != tt.createdBy {
				t.Errorf("expected createdBy %q, got %q", tt.createdBy, thread.CreatedBy)
			}
			if thread.UpdatedAt.IsZero() {
				t.Error("expected UpdatedAt to be set")
			}
			if thread.UpdatedAt != thread.CreatedAt {
				t.Error("expected UpdatedAt to equal CreatedAt on new thread")
			}
			if thread.Sticky {
				t.Error("expected Sticky to be false")
			}
		})
	}
}

func TestNewMessage(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		createdBy string
	}{
		{
			name:      "basic message",
			content:   "Hello world",
			createdBy: "claude@mcp",
		},
		{
			name:      "multiline message",
			content:   "Line 1\nLine 2\nLine 3",
			createdBy: "user@cli",
		},
		{
			name:      "empty message",
			content:   "",
			createdBy: "agent@mcp",
		},
		{
			name:      "message with code",
			content:   "```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```",
			createdBy: "dev@tui",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			threadID := uuid.New()
			msg := NewMessage(threadID, tt.content, tt.createdBy)

			if msg.ID == uuid.Nil {
				t.Error("expected non-nil UUID")
			}
			if msg.ThreadID != threadID {
				t.Error("expected threadID to match")
			}
			if msg.Content != tt.content {
				t.Errorf("expected content %q, got %q", tt.content, msg.Content)
			}
			if msg.CreatedBy != tt.createdBy {
				t.Errorf("expected createdBy %q, got %q", tt.createdBy, msg.CreatedBy)
			}
			if msg.CreatedAt.IsZero() {
				t.Error("expected CreatedAt to be set")
			}
			if msg.EditedAt != nil {
				t.Error("expected EditedAt to be nil for new message")
			}
		})
	}
}

func TestNewAttachment(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		mimeType string
		data     []byte
	}{
		{
			name:     "text file",
			filename: "test.txt",
			mimeType: "text/plain",
			data:     []byte("test content"),
		},
		{
			name:     "json file",
			filename: "data.json",
			mimeType: "application/json",
			data:     []byte(`{"key": "value"}`),
		},
		{
			name:     "binary data",
			filename: "image.png",
			mimeType: "image/png",
			data:     []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
		},
		{
			name:     "empty file",
			filename: "empty.txt",
			mimeType: "text/plain",
			data:     []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messageID := uuid.New()
			att := NewAttachment(messageID, tt.filename, tt.mimeType, tt.data)

			if att.ID == uuid.Nil {
				t.Error("expected non-nil UUID")
			}
			if att.MessageID != messageID {
				t.Error("expected messageID to match")
			}
			if att.Filename != tt.filename {
				t.Errorf("expected filename %q, got %q", tt.filename, att.Filename)
			}
			if att.MimeType != tt.mimeType {
				t.Errorf("expected mimeType %q, got %q", tt.mimeType, att.MimeType)
			}
			if string(att.Data) != string(tt.data) {
				t.Errorf("expected data %v, got %v", tt.data, att.Data)
			}
			if att.CreatedAt.IsZero() {
				t.Error("expected CreatedAt to be set")
			}
		})
	}
}

func TestParseUUID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid UUID",
			input:   "123e4567-e89b-12d3-a456-426614174000",
			wantErr: false,
		},
		{
			name:    "generated UUID",
			input:   uuid.New().String(),
			wantErr: false,
		},
		{
			name:    "uppercase UUID",
			input:   "123E4567-E89B-12D3-A456-426614174000",
			wantErr: false,
		},
		{
			name:    "invalid UUID - too short",
			input:   "123e4567-e89b-12d3",
			wantErr: true,
		},
		{
			name:    "invalid UUID - wrong format",
			input:   "not-a-uuid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseUUID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == uuid.Nil {
					t.Error("expected non-nil UUID")
				}
			}
		})
	}
}

func TestTopicFields(t *testing.T) {
	topic := NewTopic("test", "desc", "user@cli")

	// Test that fields are accessible
	if topic.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if topic.Name == "" {
		t.Error("Name should not be empty")
	}
	if topic.Description == "" {
		t.Error("Description should not be empty")
	}
	if topic.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if topic.CreatedBy == "" {
		t.Error("CreatedBy should not be empty")
	}

	// Test modification
	topic.Archived = true
	if !topic.Archived {
		t.Error("Archived should be modifiable")
	}
}

func TestThreadFields(t *testing.T) {
	topicID := uuid.New()
	thread := NewThread(topicID, "subject", "user@cli")

	// Test that fields are accessible
	if thread.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if thread.TopicID == uuid.Nil {
		t.Error("TopicID should not be nil")
	}
	if thread.Subject == "" {
		t.Error("Subject should not be empty")
	}
	if thread.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if thread.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
	if thread.CreatedBy == "" {
		t.Error("CreatedBy should not be empty")
	}

	// Test modification
	thread.Sticky = true
	if !thread.Sticky {
		t.Error("Sticky should be modifiable")
	}

	thread.UpdatedAt = time.Now().Add(time.Hour)
	if thread.UpdatedAt.Equal(thread.CreatedAt) {
		t.Error("UpdatedAt should be modifiable")
	}
}

func TestMessageFields(t *testing.T) {
	threadID := uuid.New()
	msg := NewMessage(threadID, "content", "user@cli")

	// Test that fields are accessible
	if msg.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if msg.ThreadID == uuid.Nil {
		t.Error("ThreadID should not be nil")
	}
	if msg.Content == "" {
		t.Error("Content should not be empty")
	}
	if msg.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if msg.CreatedBy == "" {
		t.Error("CreatedBy should not be empty")
	}
	if msg.EditedAt != nil {
		t.Error("EditedAt should be nil for new message")
	}

	// Test modification
	msg.Content = "updated content"
	if msg.Content != "updated content" {
		t.Error("Content should be modifiable")
	}

	now := time.Now()
	msg.EditedAt = &now
	if msg.EditedAt == nil {
		t.Error("EditedAt should be modifiable")
	}
}

func TestAttachmentFields(t *testing.T) {
	messageID := uuid.New()
	data := []byte("test data")
	att := NewAttachment(messageID, "file.txt", "text/plain", data)

	// Test that fields are accessible
	if att.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if att.MessageID == uuid.Nil {
		t.Error("MessageID should not be nil")
	}
	if att.Filename == "" {
		t.Error("Filename should not be empty")
	}
	if att.MimeType == "" {
		t.Error("MimeType should not be empty")
	}
	if len(att.Data) == 0 {
		t.Error("Data should not be empty")
	}
	if att.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Test data integrity
	if string(att.Data) != string(data) {
		t.Error("Data should match original")
	}
}

func TestUUIDType(t *testing.T) {
	// Test that UUID type alias works correctly
	var id = uuid.New()

	if id == uuid.Nil {
		t.Error("UUID should not be nil")
	}

	// Test string conversion
	str := id.String()
	if len(str) != 36 {
		t.Errorf("UUID string should be 36 chars, got %d", len(str))
	}
}

func TestTimestampPrecision(t *testing.T) {
	before := time.Now()
	topic := NewTopic("test", "desc", "user@cli")
	after := time.Now()

	if topic.CreatedAt.Before(before) {
		t.Error("CreatedAt should not be before creation")
	}
	if topic.CreatedAt.After(after) {
		t.Error("CreatedAt should not be after creation completed")
	}
}
