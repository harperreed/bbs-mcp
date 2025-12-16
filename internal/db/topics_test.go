// ABOUTME: Tests for topic database operations
// ABOUTME: Verifies CRUD operations for topics

package db

import (
	"path/filepath"
	"testing"

	"github.com/harper/bbs/internal/models"
)

func TestCreateTopic(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	topic := models.NewTopic("general", "General discussion", "harper@cli")
	err = CreateTopic(db, topic)
	if err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}

	// Verify it was created
	got, err := GetTopicByID(db, topic.ID.String())
	if err != nil {
		t.Fatalf("GetTopicByID failed: %v", err)
	}
	if got.Name != "general" {
		t.Errorf("expected name 'general', got '%s'", got.Name)
	}
}

func TestListTopics(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Create two topics
	topic1 := models.NewTopic("general", "General", "harper@cli")
	topic2 := models.NewTopic("builds", "Build logs", "harper@cli")
	CreateTopic(db, topic1)
	CreateTopic(db, topic2)

	topics, err := ListTopics(db, false)
	if err != nil {
		t.Fatalf("ListTopics failed: %v", err)
	}
	if len(topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(topics))
	}
}

func TestArchiveTopic(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	topic := models.NewTopic("old-stuff", "Old", "harper@cli")
	CreateTopic(db, topic)

	err = ArchiveTopic(db, topic.ID.String(), true)
	if err != nil {
		t.Fatalf("ArchiveTopic failed: %v", err)
	}

	// Should not appear in non-archived list
	topics, _ := ListTopics(db, false)
	if len(topics) != 0 {
		t.Errorf("expected 0 non-archived topics, got %d", len(topics))
	}

	// Should appear in archived list
	topics, _ = ListTopics(db, true)
	if len(topics) != 1 {
		t.Errorf("expected 1 archived topic, got %d", len(topics))
	}
}
