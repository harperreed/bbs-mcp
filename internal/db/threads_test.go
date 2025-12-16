// ABOUTME: Tests for thread database operations
// ABOUTME: Verifies CRUD operations for threads

package db

import (
	"path/filepath"
	"testing"

	"github.com/harper/bbs/internal/models"
)

func TestCreateThread(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Create topic first
	topic := models.NewTopic("general", "General", "harper@cli")
	CreateTopic(db, topic)

	thread := models.NewThread(topic.ID, "Hello World", "harper@cli")
	err = CreateThread(db, thread)
	if err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}

	got, err := GetThreadByID(db, thread.ID.String())
	if err != nil {
		t.Fatalf("GetThreadByID failed: %v", err)
	}
	if got.Subject != "Hello World" {
		t.Errorf("expected subject 'Hello World', got '%s'", got.Subject)
	}
}

func TestListThreads(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	topic := models.NewTopic("general", "General", "harper@cli")
	CreateTopic(db, topic)

	thread1 := models.NewThread(topic.ID, "First", "harper@cli")
	thread2 := models.NewThread(topic.ID, "Second", "harper@cli")
	CreateThread(db, thread1)
	CreateThread(db, thread2)

	threads, err := ListThreads(db, topic.ID.String())
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) != 2 {
		t.Errorf("expected 2 threads, got %d", len(threads))
	}
}

func TestStickyThread(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	topic := models.NewTopic("general", "General", "harper@cli")
	CreateTopic(db, topic)

	thread := models.NewThread(topic.ID, "Important", "harper@cli")
	CreateThread(db, thread)

	err = SetThreadSticky(db, thread.ID.String(), true)
	if err != nil {
		t.Fatalf("SetThreadSticky failed: %v", err)
	}

	got, _ := GetThreadByID(db, thread.ID.String())
	if !got.Sticky {
		t.Error("expected thread to be sticky")
	}
}
