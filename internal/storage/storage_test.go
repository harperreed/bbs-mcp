// ABOUTME: Tests for SQLite storage layer
// ABOUTME: Covers CRUD operations for topics, threads, messages, attachments

package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/harper/bbs/internal/models"
)

func TestNewSqliteStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSqliteStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("NewSqliteStore returned nil")
	}

	// Verify database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("Database file was not created")
	}
}

func TestTopicCRUD(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create
	topic := models.NewTopic("General", "General discussion", "test@cli")
	err := store.CreateTopic(topic)
	if err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}

	// Read
	got, err := store.GetTopic(topic.ID)
	if err != nil {
		t.Fatalf("GetTopic failed: %v", err)
	}
	if got.Name != topic.Name {
		t.Errorf("expected name %q, got %q", topic.Name, got.Name)
	}
	if got.Description != topic.Description {
		t.Errorf("expected description %q, got %q", topic.Description, got.Description)
	}
	if got.CreatedBy != topic.CreatedBy {
		t.Errorf("expected createdBy %q, got %q", topic.CreatedBy, got.CreatedBy)
	}

	// Update
	topic.Description = "Updated description"
	topic.Archived = true
	err = store.UpdateTopic(topic)
	if err != nil {
		t.Fatalf("UpdateTopic failed: %v", err)
	}
	got, _ = store.GetTopic(topic.ID)
	if got.Description != "Updated description" {
		t.Errorf("expected updated description, got %q", got.Description)
	}
	if !got.Archived {
		t.Error("expected topic to be archived")
	}

	// List
	topics, err := store.ListTopics(false)
	if err != nil {
		t.Fatalf("ListTopics failed: %v", err)
	}
	if len(topics) != 0 {
		t.Errorf("expected 0 non-archived topics, got %d", len(topics))
	}

	topics, err = store.ListTopics(true)
	if err != nil {
		t.Fatalf("ListTopics (with archived) failed: %v", err)
	}
	if len(topics) != 1 {
		t.Errorf("expected 1 topic, got %d", len(topics))
	}

	// Delete
	err = store.DeleteTopic(topic.ID)
	if err != nil {
		t.Fatalf("DeleteTopic failed: %v", err)
	}
	_, err = store.GetTopic(topic.ID)
	if err == nil {
		t.Error("expected error getting deleted topic")
	}
}

func TestGetTopicByName(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("TestTopic", "A test topic", "test@cli")
	_ = store.CreateTopic(topic)

	got, err := store.GetTopicByName("TestTopic")
	if err != nil {
		t.Fatalf("GetTopicByName failed: %v", err)
	}
	if got.ID != topic.ID {
		t.Errorf("expected ID %s, got %s", topic.ID, got.ID)
	}

	_, err = store.GetTopicByName("NonExistent")
	if err == nil {
		t.Error("expected error for non-existent topic name")
	}
}

func TestThreadCRUD(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create topic first
	topic := models.NewTopic("Test", "Test topic", "test@cli")
	_ = store.CreateTopic(topic)

	// Create thread
	thread := models.NewThread(topic.ID, "Hello World", "test@cli")
	err := store.CreateThread(thread)
	if err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}

	// Read
	got, err := store.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread failed: %v", err)
	}
	if got.Subject != thread.Subject {
		t.Errorf("expected subject %q, got %q", thread.Subject, got.Subject)
	}
	if got.TopicID != topic.ID {
		t.Errorf("expected topicID %s, got %s", topic.ID, got.TopicID)
	}

	// Update
	thread.Sticky = true
	err = store.UpdateThread(thread)
	if err != nil {
		t.Fatalf("UpdateThread failed: %v", err)
	}
	got, _ = store.GetThread(thread.ID)
	if !got.Sticky {
		t.Error("expected thread to be sticky")
	}

	// List
	threads, err := store.ListThreads(topic.ID)
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) != 1 {
		t.Errorf("expected 1 thread, got %d", len(threads))
	}

	// Delete
	err = store.DeleteThread(thread.ID)
	if err != nil {
		t.Fatalf("DeleteThread failed: %v", err)
	}
	_, err = store.GetThread(thread.ID)
	if err == nil {
		t.Error("expected error getting deleted thread")
	}
}

func TestThreadUpdatedAt(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)

	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

	// Get initial updated_at
	got, _ := store.GetThread(thread.ID)
	initialUpdatedAt := got.UpdatedAt

	// Wait a bit and update
	time.Sleep(10 * time.Millisecond)
	thread.Subject = "Updated Subject"
	_ = store.UpdateThread(thread)

	got, _ = store.GetThread(thread.ID)
	if !got.UpdatedAt.After(initialUpdatedAt) {
		t.Error("expected updated_at to be updated")
	}
}

func TestMessageCRUD(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create topic and thread first
	topic := models.NewTopic("Test", "Test topic", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

	// Create message
	msg := models.NewMessage(thread.ID, "Hello, world!", "test@cli")
	err := store.CreateMessage(msg)
	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	// Read
	got, err := store.GetMessage(msg.ID)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if got.Content != msg.Content {
		t.Errorf("expected content %q, got %q", msg.Content, got.Content)
	}
	if got.ThreadID != thread.ID {
		t.Errorf("expected threadID %s, got %s", thread.ID, got.ThreadID)
	}

	// Update
	now := time.Now()
	msg.Content = "Edited message"
	msg.EditedAt = &now
	err = store.UpdateMessage(msg)
	if err != nil {
		t.Fatalf("UpdateMessage failed: %v", err)
	}
	got, _ = store.GetMessage(msg.ID)
	if got.Content != "Edited message" {
		t.Errorf("expected edited content, got %q", got.Content)
	}
	if got.EditedAt == nil {
		t.Error("expected editedAt to be set")
	}

	// List
	messages, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}

	// Delete
	err = store.DeleteMessage(msg.ID)
	if err != nil {
		t.Fatalf("DeleteMessage failed: %v", err)
	}
	_, err = store.GetMessage(msg.ID)
	if err == nil {
		t.Error("expected error getting deleted message")
	}
}

func TestAttachmentCRUD(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create topic, thread, message first
	topic := models.NewTopic("Test", "Test topic", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	_ = store.CreateMessage(msg)

	// Create attachment
	att := models.NewAttachment(msg.ID, "test.txt", "text/plain", []byte("test content"))
	err := store.CreateAttachment(att)
	if err != nil {
		t.Fatalf("CreateAttachment failed: %v", err)
	}

	// Read
	got, err := store.GetAttachment(att.ID)
	if err != nil {
		t.Fatalf("GetAttachment failed: %v", err)
	}
	if got.Filename != att.Filename {
		t.Errorf("expected filename %q, got %q", att.Filename, got.Filename)
	}
	if string(got.Data) != "test content" {
		t.Errorf("expected data %q, got %q", "test content", string(got.Data))
	}

	// List
	attachments, err := store.ListAttachments(msg.ID)
	if err != nil {
		t.Fatalf("ListAttachments failed: %v", err)
	}
	if len(attachments) != 1 {
		t.Errorf("expected 1 attachment, got %d", len(attachments))
	}

	// Delete
	err = store.DeleteAttachment(att.ID)
	if err != nil {
		t.Fatalf("DeleteAttachment failed: %v", err)
	}
	_, err = store.GetAttachment(att.ID)
	if err == nil {
		t.Error("expected error getting deleted attachment")
	}
}

func TestCascadeDelete(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create full hierarchy
	topic := models.NewTopic("Test", "Test topic", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	_ = store.CreateMessage(msg)
	att := models.NewAttachment(msg.ID, "test.txt", "text/plain", []byte("test"))
	_ = store.CreateAttachment(att)

	// Delete topic should cascade
	err := store.DeleteTopic(topic.ID)
	if err != nil {
		t.Fatalf("DeleteTopic failed: %v", err)
	}

	// All children should be gone
	_, err = store.GetThread(thread.ID)
	if err == nil {
		t.Error("thread should be deleted by cascade")
	}
	_, err = store.GetMessage(msg.ID)
	if err == nil {
		t.Error("message should be deleted by cascade")
	}
	_, err = store.GetAttachment(att.ID)
	if err == nil {
		t.Error("attachment should be deleted by cascade")
	}
}

func TestResolveTopic(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("General", "General discussion", "test@cli")
	_ = store.CreateTopic(topic)

	// By full ID
	got, err := store.ResolveTopic(topic.ID.String())
	if err != nil {
		t.Fatalf("ResolveTopic by ID failed: %v", err)
	}
	if got.ID != topic.ID {
		t.Error("wrong topic returned")
	}

	// By name
	got, err = store.ResolveTopic("General")
	if err != nil {
		t.Fatalf("ResolveTopic by name failed: %v", err)
	}
	if got.ID != topic.ID {
		t.Error("wrong topic returned")
	}

	// By ID prefix (first 8 chars)
	prefix := topic.ID.String()[:8]
	got, err = store.ResolveTopic(prefix)
	if err != nil {
		t.Fatalf("ResolveTopic by prefix failed: %v", err)
	}
	if got.ID != topic.ID {
		t.Error("wrong topic returned")
	}

	// Not found
	_, err = store.ResolveTopic("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent topic")
	}
}

func TestResolveThread(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

	// By full ID
	got, err := store.ResolveThread(thread.ID.String())
	if err != nil {
		t.Fatalf("ResolveThread by ID failed: %v", err)
	}
	if got.ID != thread.ID {
		t.Error("wrong thread returned")
	}

	// By ID prefix
	prefix := thread.ID.String()[:8]
	got, err = store.ResolveThread(prefix)
	if err != nil {
		t.Fatalf("ResolveThread by prefix failed: %v", err)
	}
	if got.ID != thread.ID {
		t.Error("wrong thread returned")
	}
}

func TestResolveMessage(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	_ = store.CreateMessage(msg)

	// By full ID
	got, err := store.ResolveMessage(msg.ID.String())
	if err != nil {
		t.Fatalf("ResolveMessage by ID failed: %v", err)
	}
	if got.ID != msg.ID {
		t.Error("wrong message returned")
	}

	// By ID prefix
	prefix := msg.ID.String()[:8]
	got, err = store.ResolveMessage(prefix)
	if err != nil {
		t.Fatalf("ResolveMessage by prefix failed: %v", err)
	}
	if got.ID != msg.ID {
		t.Error("wrong message returned")
	}

	// Not found
	_, err = store.ResolveMessage("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent message")
	}
}

func TestResolveTopicAmbiguous(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create topics that could have similar ID prefixes
	topic1 := models.NewTopic("Topic1", "First", "test@cli")
	topic2 := models.NewTopic("Topic2", "Second", "test@cli")
	_ = store.CreateTopic(topic1)
	_ = store.CreateTopic(topic2)

	// Test ambiguous prefix (using very short prefix that might match multiple)
	// This is hard to test deterministically since UUIDs are random
	// But we can test the nonexistent case
	_, err := store.ResolveTopic("zzz")
	if err == nil {
		t.Error("expected error for non-matching prefix")
	}
}

func TestResolveThreadNotFound(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	_, err := store.ResolveThread("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent thread")
	}
}

func TestCreateMessageWithEditedAt(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	now := msg.CreatedAt
	msg.EditedAt = &now
	err := store.CreateMessage(msg)
	if err != nil {
		t.Fatalf("CreateMessage with EditedAt failed: %v", err)
	}

	// Verify the message was stored with EditedAt
	got, err := store.GetMessage(msg.ID)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if got.EditedAt == nil {
		t.Error("expected EditedAt to be set")
	}
}

func TestUpdateMessageWithoutEditedAt(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	_ = store.CreateMessage(msg)

	// Update without setting EditedAt
	msg.Content = "Updated content"
	err := store.UpdateMessage(msg)
	if err != nil {
		t.Fatalf("UpdateMessage failed: %v", err)
	}

	got, _ := store.GetMessage(msg.ID)
	if got.Content != "Updated content" {
		t.Error("content was not updated")
	}
}

func TestListTopicsWithRowsErr(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create multiple topics to iterate through
	for i := 0; i < 3; i++ {
		topic := models.NewTopic("Topic"+string(rune('A'+i)), "Desc", "test@cli")
		_ = store.CreateTopic(topic)
	}

	topics, err := store.ListTopics(true)
	if err != nil {
		t.Fatalf("ListTopics failed: %v", err)
	}
	if len(topics) != 3 {
		t.Errorf("expected 3 topics, got %d", len(topics))
	}
}

func TestListThreadsOrdering(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)

	// Create threads with different sticky states
	thread1 := models.NewThread(topic.ID, "Normal Thread", "test@cli")
	thread2 := models.NewThread(topic.ID, "Sticky Thread", "test@cli")
	thread2.Sticky = true
	_ = store.CreateThread(thread1)
	_ = store.CreateThread(thread2)

	threads, err := store.ListThreads(topic.ID)
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}
	// Sticky thread should be first
	if !threads[0].Sticky {
		t.Error("sticky thread should be first")
	}
}

func TestListMessagesWithEditedAt(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

	// Create a message with EditedAt
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	now := msg.CreatedAt
	msg.EditedAt = &now
	_ = store.CreateMessage(msg)

	messages, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].EditedAt == nil {
		t.Error("expected EditedAt to be set")
	}
}

func TestListAttachmentsOrdering(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	_ = store.CreateMessage(msg)

	// Create multiple attachments
	att1 := models.NewAttachment(msg.ID, "file1.txt", "text/plain", []byte("content1"))
	att2 := models.NewAttachment(msg.ID, "file2.txt", "text/plain", []byte("content2"))
	_ = store.CreateAttachment(att1)
	_ = store.CreateAttachment(att2)

	attachments, err := store.ListAttachments(msg.ID)
	if err != nil {
		t.Fatalf("ListAttachments failed: %v", err)
	}
	if len(attachments) != 2 {
		t.Errorf("expected 2 attachments, got %d", len(attachments))
	}
}

func TestDataDir(t *testing.T) {
	// Test with XDG_DATA_HOME set
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	dir := DataDir()
	if dir == "" {
		t.Error("DataDir returned empty string")
	}
	if !strings.HasPrefix(dir, tmpDir) {
		t.Errorf("DataDir should use XDG_DATA_HOME, got %s", dir)
	}
}

func TestDataDirWithoutXDG(t *testing.T) {
	// Test without XDG_DATA_HOME
	t.Setenv("XDG_DATA_HOME", "")

	dir := DataDir()
	if dir == "" {
		t.Error("DataDir returned empty string")
	}
	// Should fall back to ~/.local/share
	if !strings.Contains(dir, ".local/share") {
		t.Errorf("DataDir should use .local/share fallback, got %s", dir)
	}
}

func TestDefaultDBPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	path := DefaultDBPath()
	if path == "" {
		t.Error("DefaultDBPath returned empty string")
	}
	if !strings.HasSuffix(path, "bbs.db") {
		t.Errorf("DefaultDBPath should end with bbs.db, got %s", path)
	}
}

func TestResolveMessageWithEditedAt(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	now := msg.CreatedAt
	msg.EditedAt = &now
	_ = store.CreateMessage(msg)

	// Resolve by prefix
	prefix := msg.ID.String()[:8]
	got, err := store.ResolveMessage(prefix)
	if err != nil {
		t.Fatalf("ResolveMessage failed: %v", err)
	}
	if got.EditedAt == nil {
		t.Error("expected EditedAt to be set on resolved message")
	}
}

func TestArchiveTopic(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)

	// Archive
	err := store.ArchiveTopic(topic.ID, true)
	if err != nil {
		t.Fatalf("ArchiveTopic failed: %v", err)
	}
	got, _ := store.GetTopic(topic.ID)
	if !got.Archived {
		t.Error("topic should be archived")
	}

	// Unarchive
	err = store.ArchiveTopic(topic.ID, false)
	if err != nil {
		t.Fatalf("Unarchive failed: %v", err)
	}
	got, _ = store.GetTopic(topic.ID)
	if got.Archived {
		t.Error("topic should not be archived")
	}
}

func TestSetThreadSticky(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

	// Set sticky
	err := store.SetThreadSticky(thread.ID, true)
	if err != nil {
		t.Fatalf("SetThreadSticky failed: %v", err)
	}
	got, _ := store.GetThread(thread.ID)
	if !got.Sticky {
		t.Error("thread should be sticky")
	}

	// Unset sticky
	err = store.SetThreadSticky(thread.ID, false)
	if err != nil {
		t.Fatalf("Unset sticky failed: %v", err)
	}
	got, _ = store.GetThread(thread.ID)
	if got.Sticky {
		t.Error("thread should not be sticky")
	}
}

func TestGetNotFound(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	randomID := uuid.New()

	_, err := store.GetTopic(randomID)
	if err == nil {
		t.Error("expected error for non-existent topic")
	}

	_, err = store.GetThread(randomID)
	if err == nil {
		t.Error("expected error for non-existent thread")
	}

	_, err = store.GetMessage(randomID)
	if err == nil {
		t.Error("expected error for non-existent message")
	}

	_, err = store.GetAttachment(randomID)
	if err == nil {
		t.Error("expected error for non-existent attachment")
	}
}

// Helper to create a test store
func newTestStore(t *testing.T) *SqliteStore {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return store
}
