// ABOUTME: Tests for message database operations
// ABOUTME: Verifies CRUD operations for messages

package db

import (
	"path/filepath"
	"testing"

	"github.com/harper/bbs/internal/models"
)

func TestCreateMessage(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Create topic and thread first
	topic := models.NewTopic("general", "General", "harper@cli")
	CreateTopic(db, topic)
	thread := models.NewThread(topic.ID, "Hello", "harper@cli")
	CreateThread(db, thread)

	msg := models.NewMessage(thread.ID, "Hello world!", "harper@cli")
	err = CreateMessage(db, msg)
	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	got, err := GetMessageByID(db, msg.ID.String())
	if err != nil {
		t.Fatalf("GetMessageByID failed: %v", err)
	}
	if got.Content != "Hello world!" {
		t.Errorf("expected content 'Hello world!', got '%s'", got.Content)
	}
}

func TestListMessages(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	topic := models.NewTopic("general", "General", "harper@cli")
	CreateTopic(db, topic)
	thread := models.NewThread(topic.ID, "Hello", "harper@cli")
	CreateThread(db, thread)

	msg1 := models.NewMessage(thread.ID, "First", "harper@cli")
	msg2 := models.NewMessage(thread.ID, "Second", "claude@mcp")
	CreateMessage(db, msg1)
	CreateMessage(db, msg2)

	messages, err := ListMessages(db, thread.ID.String())
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestUpdateMessage(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	topic := models.NewTopic("general", "General", "harper@cli")
	CreateTopic(db, topic)
	thread := models.NewThread(topic.ID, "Hello", "harper@cli")
	CreateThread(db, thread)

	msg := models.NewMessage(thread.ID, "Original", "harper@cli")
	CreateMessage(db, msg)

	err = UpdateMessage(db, msg.ID.String(), "Edited content")
	if err != nil {
		t.Fatalf("UpdateMessage failed: %v", err)
	}

	got, _ := GetMessageByID(db, msg.ID.String())
	if got.Content != "Edited content" {
		t.Errorf("expected 'Edited content', got '%s'", got.Content)
	}
	if got.EditedAt == nil {
		t.Error("expected EditedAt to be set")
	}
}
