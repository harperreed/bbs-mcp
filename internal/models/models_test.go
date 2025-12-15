// ABOUTME: Tests for BBS data models
// ABOUTME: Verifies model creation and validation

package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewTopic(t *testing.T) {
	topic := NewTopic("general", "General discussion", "harper@cli")

	if topic.ID == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
	if topic.Name != "general" {
		t.Errorf("expected name 'general', got '%s'", topic.Name)
	}
	if topic.CreatedBy != "harper@cli" {
		t.Errorf("expected createdBy 'harper@cli', got '%s'", topic.CreatedBy)
	}
	if topic.Archived {
		t.Error("expected archived to be false")
	}
}

func TestNewThread(t *testing.T) {
	topicID := uuid.New()
	thread := NewThread(topicID, "Test subject", "harper@cli")

	if thread.ID == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
	if thread.TopicID != topicID {
		t.Error("expected topicID to match")
	}
	if thread.Subject != "Test subject" {
		t.Errorf("expected subject 'Test subject', got '%s'", thread.Subject)
	}
}

func TestNewMessage(t *testing.T) {
	threadID := uuid.New()
	msg := NewMessage(threadID, "Hello world", "claude@mcp")

	if msg.ID == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
	if msg.Content != "Hello world" {
		t.Errorf("expected content 'Hello world', got '%s'", msg.Content)
	}
	if msg.CreatedBy != "claude@mcp" {
		t.Errorf("expected createdBy 'claude@mcp', got '%s'", msg.CreatedBy)
	}
}
