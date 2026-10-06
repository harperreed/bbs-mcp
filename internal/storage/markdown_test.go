// ABOUTME: Tests for MarkdownStore file-based storage backend
// ABOUTME: Covers CRUD for topics, threads, messages, attachments, resolution, and edge cases

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/mdstore"

	"github.com/harper/bbs/internal/models"
)

// newTestMarkdownStore creates a MarkdownStore in a temporary directory for testing.
func newTestMarkdownStore(t *testing.T) *MarkdownStore {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := NewMarkdownStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create test markdown store: %v", err)
	}
	return store
}

func TestNewMarkdownStore(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "bbs-data")

	store, err := NewMarkdownStore(dataDir)
	if err != nil {
		t.Fatalf("NewMarkdownStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("NewMarkdownStore returned nil")
	}

	// Verify data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Fatal("Data directory was not created")
	}
}

func TestMarkdownTopicCRUD(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownGetTopicByName(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownThreadCRUD(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownThreadUpdatedAt(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)

	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

	// Get initial updated_at (should equal created_at since no messages yet)
	got, _ := store.GetThread(thread.ID)
	initialUpdatedAt := got.UpdatedAt

	// Wait a bit and add a message - this should advance updated_at
	time.Sleep(10 * time.Millisecond)
	msg := models.NewMessage(thread.ID, "Bump", "test@cli")
	_ = store.CreateMessage(msg)

	got, _ = store.GetThread(thread.ID)
	if !got.UpdatedAt.After(initialUpdatedAt) {
		t.Error("expected updated_at to advance when a message is added")
	}
}

func TestMarkdownMessageCRUD(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownAttachmentCRUD(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownCascadeDelete(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownResolveTopic(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownResolveThread(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownResolveMessage(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownResolveTopicAmbiguous(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic1 := models.NewTopic("Topic1", "First", "test@cli")
	topic2 := models.NewTopic("Topic2", "Second", "test@cli")
	_ = store.CreateTopic(topic1)
	_ = store.CreateTopic(topic2)

	// Test nonexistent prefix
	_, err := store.ResolveTopic("zzz")
	if err == nil {
		t.Error("expected error for non-matching prefix")
	}
}

func TestMarkdownResolveThreadNotFound(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	_, err := store.ResolveThread("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent thread")
	}
}

func TestMarkdownCreateMessageWithEditedAt(t *testing.T) {
	store := newTestMarkdownStore(t)
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

	got, err := store.GetMessage(msg.ID)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if got.EditedAt == nil {
		t.Error("expected EditedAt to be set")
	}
}

func TestMarkdownUpdateMessageWithoutEditedAt(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	_ = store.CreateMessage(msg)

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

func TestMarkdownListTopicsMultiple(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

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

func TestMarkdownListThreadsOrdering(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownListMessagesWithEditedAt(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

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

func TestMarkdownListAttachmentsOrdering(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	_ = store.CreateMessage(msg)

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

func TestMarkdownArchiveTopic(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownSetThreadSticky(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownGetNotFound(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownResolveMessageWithEditedAt(t *testing.T) {
	store := newTestMarkdownStore(t)
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

func TestMarkdownSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"BBS Health Check", "bbs-health-check"},
		{"  multiple   spaces  ", "multiple-spaces"},
		{"Special! @chars# $here", "special-chars-here"},
		{"", "untitled"},
		{"ALLCAPS", "allcaps"},
		{"with-existing-hyphens", "with-existing-hyphens"},
		{"123-numbers", "123-numbers"},
	}

	for _, tt := range tests {
		got := mdstore.Slugify(tt.input)
		if got != tt.expected {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestMarkdownFrontmatterParsing(t *testing.T) {
	content := `---
id: 5681e681-3603-4dbf-b289-08ae47819163
topic: general
subject: BBS Health Check
created_at: "2026-02-02T02:39:08Z"
created_by: Claude@mcp
sticky: false
---

## Claude@mcp — 2026-02-02T02:39:08Z
<!-- msg:49528f10-0ec9-49fa-903e-e078a45c08fc -->

Testing that the BBS is operational.
`
	fm, err := parseThreadFrontmatter(content)
	if err != nil {
		t.Fatalf("parseThreadFrontmatter failed: %v", err)
	}
	if fm.ID != "5681e681-3603-4dbf-b289-08ae47819163" {
		t.Errorf("expected ID 5681e681-3603-4dbf-b289-08ae47819163, got %s", fm.ID)
	}
	if fm.Topic != "general" {
		t.Errorf("expected topic general, got %s", fm.Topic)
	}
	if fm.Subject != "BBS Health Check" {
		t.Errorf("expected subject 'BBS Health Check', got %s", fm.Subject)
	}
}

func TestMarkdownMessageParsing(t *testing.T) {
	threadID := uuid.New()
	content := `---
id: ` + threadID.String() + `
topic: general
subject: Test Thread
created_at: "2026-02-02T02:39:08Z"
created_by: test@cli
sticky: false
---

## Claude@mcp — 2026-02-02T02:39:08Z
<!-- msg:49528f10-0ec9-49fa-903e-e078a45c08fc -->

Testing that the BBS is operational. All systems nominal.

<!-- message-separator -->

## claude_opus@mcp — 2026-02-02T02:42:30Z
<!-- msg:762ee7bf-1234-5678-abcd-ef0123456789 -->

Reply test - confirming message posting works correctly.
`
	messages, err := parseThreadMessages(content)
	if err != nil {
		t.Fatalf("parseThreadMessages failed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	if messages[0].ID.String() != "49528f10-0ec9-49fa-903e-e078a45c08fc" {
		t.Errorf("expected first message ID 49528f10-0ec9-49fa-903e-e078a45c08fc, got %s", messages[0].ID)
	}
	if messages[0].CreatedBy != "Claude@mcp" {
		t.Errorf("expected first message author Claude@mcp, got %s", messages[0].CreatedBy)
	}
	if !strings.Contains(messages[0].Content, "Testing that the BBS is operational") {
		t.Errorf("unexpected first message content: %s", messages[0].Content)
	}

	if messages[1].ID.String() != "762ee7bf-1234-5678-abcd-ef0123456789" {
		t.Errorf("expected second message ID 762ee7bf-1234-5678-abcd-ef0123456789, got %s", messages[1].ID)
	}
}

func TestMarkdownMessageWithEditedMarker(t *testing.T) {
	threadID := uuid.New()
	content := `---
id: ` + threadID.String() + `
topic: general
subject: Test
created_at: "2026-02-02T02:39:08Z"
created_by: test@cli
sticky: false
---

## test@cli — 2026-02-02T02:39:08Z
<!-- msg:49528f10-0ec9-49fa-903e-e078a45c08fc -->
<!-- edited:2026-02-02T03:00:00Z -->

This was edited.
`
	messages, err := parseThreadMessages(content)
	if err != nil {
		t.Fatalf("parseThreadMessages failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].EditedAt == nil {
		t.Fatal("expected EditedAt to be set")
	}
	expected, _ := time.Parse(time.RFC3339, "2026-02-02T03:00:00Z")
	if !messages[0].EditedAt.Equal(expected) {
		t.Errorf("expected EditedAt %v, got %v", expected, *messages[0].EditedAt)
	}
}

func TestMarkdownThreadFileCreation(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("general", "General discussion", "test@cli")
	_ = store.CreateTopic(topic)

	thread := models.NewThread(topic.ID, "BBS Health Check", "test@cli")
	_ = store.CreateThread(thread)

	// Verify the file was created with the expected name
	expectedPath := filepath.Join(store.dataDir, "general", "bbs-health-check.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected thread file at %s", expectedPath)
	}
}

func TestMarkdownThreadDeleteCleansUpAttachments(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test topic", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	_ = store.CreateMessage(msg)
	att := models.NewAttachment(msg.ID, "test.txt", "text/plain", []byte("test"))
	_ = store.CreateAttachment(att)

	// Verify attachment exists
	_, err := store.GetAttachment(att.ID)
	if err != nil {
		t.Fatalf("attachment should exist before delete: %v", err)
	}

	// Delete thread
	err = store.DeleteThread(thread.ID)
	if err != nil {
		t.Fatalf("DeleteThread failed: %v", err)
	}

	// Attachment should be gone too
	_, err = store.GetAttachment(att.ID)
	if err == nil {
		t.Error("attachment should be deleted when thread is deleted")
	}
}

func TestMarkdownMultipleMessages(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

	// Create multiple messages
	msg1 := models.NewMessage(thread.ID, "First message", "user1@cli")
	time.Sleep(time.Millisecond)
	msg2 := models.NewMessage(thread.ID, "Second message", "user2@cli")
	time.Sleep(time.Millisecond)
	msg3 := models.NewMessage(thread.ID, "Third message", "user3@cli")

	_ = store.CreateMessage(msg1)
	_ = store.CreateMessage(msg2)
	_ = store.CreateMessage(msg3)

	messages, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}

	// Verify order is by created_at ASC
	if messages[0].Content != "First message" {
		t.Errorf("expected first message content 'First message', got %q", messages[0].Content)
	}
	if messages[1].Content != "Second message" {
		t.Errorf("expected second message content 'Second message', got %q", messages[1].Content)
	}
	if messages[2].Content != "Third message" {
		t.Errorf("expected third message content 'Third message', got %q", messages[2].Content)
	}
}

func TestMarkdownClose(t *testing.T) {
	store := newTestMarkdownStore(t)
	err := store.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestMarkdownDuplicateTopicName(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic1 := models.NewTopic("General", "First", "test@cli")
	err := store.CreateTopic(topic1)
	if err != nil {
		t.Fatalf("First CreateTopic failed: %v", err)
	}

	topic2 := models.NewTopic("General", "Second", "test@cli")
	err = store.CreateTopic(topic2)
	if err == nil {
		t.Error("expected error creating topic with duplicate name")
	}
}

func TestMarkdownTopicDirectoryCreated(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("my-topic", "A topic", "test@cli")
	_ = store.CreateTopic(topic)

	topicDir := filepath.Join(store.dataDir, "my-topic")
	if _, err := os.Stat(topicDir); os.IsNotExist(err) {
		t.Error("topic directory should be created")
	}
}

func TestMarkdownTopicDirectoryRemovedOnDelete(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("my-topic", "A topic", "test@cli")
	_ = store.CreateTopic(topic)

	topicDir := filepath.Join(store.dataDir, "my-topic")
	if _, err := os.Stat(topicDir); os.IsNotExist(err) {
		t.Fatal("topic directory should exist")
	}

	_ = store.DeleteTopic(topic.ID)

	if _, err := os.Stat(topicDir); !os.IsNotExist(err) {
		t.Error("topic directory should be removed on delete")
	}
}

func TestMarkdownUpdateTopicRenameUpdatesThreadFrontmatter(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("old-name", "A topic", "test@cli")
	_ = store.CreateTopic(topic)

	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

	msg := models.NewMessage(thread.ID, "Hello from old topic", "test@cli")
	_ = store.CreateMessage(msg)

	// Rename the topic
	topic.Name = "new-name"
	err := store.UpdateTopic(topic)
	if err != nil {
		t.Fatalf("UpdateTopic (rename) failed: %v", err)
	}

	// Verify old directory is gone and new directory exists
	oldDir := filepath.Join(store.dataDir, "old-name")
	newDir := filepath.Join(store.dataDir, "new-name")
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("old topic directory should not exist after rename")
	}
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Error("new topic directory should exist after rename")
	}

	// Verify the thread is still retrievable
	got, err := store.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread after rename failed: %v", err)
	}
	if got.Subject != "Test Thread" {
		t.Errorf("expected subject 'Test Thread', got %q", got.Subject)
	}

	// Verify thread frontmatter was updated by reading the file directly
	entries, _ := os.ReadDir(newDir)
	foundThread := false
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		fp := filepath.Join(newDir, de.Name())
		fm, fmErr := readThreadFrontmatter(fp)
		if fmErr != nil {
			continue
		}
		if fm.ID == thread.ID.String() {
			foundThread = true
			if fm.Topic != "new-name" {
				t.Errorf("expected thread frontmatter topic to be 'new-name', got %q", fm.Topic)
			}
		}
	}
	if !foundThread {
		t.Error("thread file not found in renamed topic directory")
	}

	// Verify messages survived the rename
	messages, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages after rename failed: %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("expected 1 message after rename, got %d", len(messages))
	}
}

func TestMarkdownAttachmentFilenameCollision(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test topic", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@cli")
	_ = store.CreateMessage(msg)

	// Create first attachment with a filename
	att1 := models.NewAttachment(msg.ID, "report.txt", "text/plain", []byte("first report"))
	err := store.CreateAttachment(att1)
	if err != nil {
		t.Fatalf("CreateAttachment (first) failed: %v", err)
	}

	// Create second attachment with the same filename
	att2 := models.NewAttachment(msg.ID, "report.txt", "text/plain", []byte("second report"))
	err = store.CreateAttachment(att2)
	if err != nil {
		t.Fatalf("CreateAttachment (collision) failed: %v", err)
	}

	// Both attachments should be retrievable with correct data
	got1, err := store.GetAttachment(att1.ID)
	if err != nil {
		t.Fatalf("GetAttachment (first) failed: %v", err)
	}
	if string(got1.Data) != "first report" {
		t.Errorf("expected first attachment data 'first report', got %q", string(got1.Data))
	}

	got2, err := store.GetAttachment(att2.ID)
	if err != nil {
		t.Fatalf("GetAttachment (second) failed: %v", err)
	}
	if string(got2.Data) != "second report" {
		t.Errorf("expected second attachment data 'second report', got %q", string(got2.Data))
	}
	// The second attachment had a collision, so its filename should still be the original
	if got2.Filename != "report.txt" {
		t.Errorf("expected second attachment filename 'report.txt' (original preserved), got %q", got2.Filename)
	}

	// Both should appear in list
	attachments, err := store.ListAttachments(msg.ID)
	if err != nil {
		t.Fatalf("ListAttachments failed: %v", err)
	}
	if len(attachments) != 2 {
		t.Errorf("expected 2 attachments, got %d", len(attachments))
	}
}

func TestMarkdownToModelErrorPropagation(t *testing.T) {
	// Test that toModel properly returns an error for invalid data
	entry := topicEntry{
		ID:        "not-a-uuid",
		Name:      "test",
		CreatedAt: "2026-01-01T00:00:00Z",
	}
	_, err := entry.toModel()
	if err == nil {
		t.Error("expected error for invalid UUID in toModel")
	}

	entry2 := topicEntry{
		ID:        "5681e681-3603-4dbf-b289-08ae47819163",
		Name:      "test",
		CreatedAt: "not-a-timestamp",
	}
	_, err = entry2.toModel()
	if err == nil {
		t.Error("expected error for invalid timestamp in toModel")
	}

	// Valid entry should succeed
	entry3 := topicEntry{
		ID:        "5681e681-3603-4dbf-b289-08ae47819163",
		Name:      "test",
		CreatedAt: "2026-01-01T00:00:00Z",
		CreatedBy: "test@cli",
	}
	topic, err := entry3.toModel()
	if err != nil {
		t.Fatalf("expected no error for valid entry, got: %v", err)
	}
	if topic.Name != "test" {
		t.Errorf("expected name 'test', got %q", topic.Name)
	}
}

func TestMarkdownSetThreadStickyAtomicity(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("Test", "Test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@cli")
	_ = store.CreateThread(thread)

	// Add a message to the thread
	msg := models.NewMessage(thread.ID, "Important content", "test@cli")
	_ = store.CreateMessage(msg)

	// Set sticky
	err := store.SetThreadSticky(thread.ID, true)
	if err != nil {
		t.Fatalf("SetThreadSticky failed: %v", err)
	}

	// Verify sticky is set and message content is preserved
	got, err := store.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread after SetThreadSticky failed: %v", err)
	}
	if !got.Sticky {
		t.Error("thread should be sticky")
	}

	messages, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages after SetThreadSticky failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "Important content" {
		t.Errorf("expected message content 'Important content', got %q", messages[0].Content)
	}
}

// --- Concurrency Tests ---

func TestMarkdownConcurrentTopicCreation(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	const numGoroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			topic := models.NewTopic(fmt.Sprintf("topic-%d", idx), fmt.Sprintf("Description %d", idx), "test@cli")
			if err := store.CreateTopic(topic); err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// Verify all topics are readable
	topics, err := store.ListTopics(true)
	if err != nil {
		t.Fatalf("ListTopics failed: %v", err)
	}
	if len(topics) != numGoroutines {
		t.Errorf("expected %d topics, got %d", numGoroutines, len(topics))
	}

	// Verify each topic is individually retrievable
	for _, topic := range topics {
		got, err := store.GetTopic(topic.ID)
		if err != nil {
			t.Errorf("GetTopic(%s) failed: %v", topic.ID, err)
			continue
		}
		if got.Name != topic.Name {
			t.Errorf("topic name mismatch: want %q, got %q", topic.Name, got.Name)
		}
	}
}

func TestMarkdownConcurrentMessagePosting(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("concurrent-topic", "Concurrency test", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Concurrent Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}

	const numGoroutines = 15
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			// Small stagger so messages get slightly different timestamps
			time.Sleep(time.Duration(idx) * time.Millisecond)
			msg := models.NewMessage(thread.ID, fmt.Sprintf("Concurrent message %d", idx), fmt.Sprintf("user%d@cli", idx))
			if err := store.CreateMessage(msg); err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// Verify all messages are readable
	messages, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(messages) != numGoroutines {
		t.Errorf("expected %d messages, got %d", numGoroutines, len(messages))
	}

	// Verify each message is individually retrievable
	for _, msg := range messages {
		got, err := store.GetMessage(msg.ID)
		if err != nil {
			t.Errorf("GetMessage(%s) failed: %v", msg.ID, err)
			continue
		}
		if got.Content != msg.Content {
			t.Errorf("message content mismatch: want %q, got %q", msg.Content, got.Content)
		}
	}
}

func TestMarkdownConcurrentMixedOperations(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	// Seed some topics and threads
	topic := models.NewTopic("mixed-ops", "Mixed operations", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}

	const numThreads = 10
	threadIDs := make([]uuid.UUID, numThreads)
	for i := 0; i < numThreads; i++ {
		thread := models.NewThread(topic.ID, fmt.Sprintf("Thread %d", i), "test@cli")
		if err := store.CreateThread(thread); err != nil {
			t.Fatalf("CreateThread %d failed: %v", i, err)
		}
		threadIDs[i] = thread.ID
	}

	// Concurrently: post messages to different threads, read threads, list threads
	const numGoroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines*3)

	// Writers: post a message to each thread
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			threadIdx := idx % numThreads
			msg := models.NewMessage(threadIDs[threadIdx], fmt.Sprintf("msg %d", idx), "test@cli")
			if err := store.CreateMessage(msg); err != nil {
				errs <- fmt.Errorf("write goroutine %d: %w", idx, err)
			}
		}(i)
	}

	// Readers: list threads and messages concurrently with writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := store.ListThreads(topic.ID)
			if err != nil {
				errs <- fmt.Errorf("list threads goroutine %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// Verify all threads exist and all messages are accounted for
	threads, err := store.ListThreads(topic.ID)
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) != numThreads {
		t.Errorf("expected %d threads, got %d", numThreads, len(threads))
	}

	totalMessages := 0
	for _, thread := range threads {
		msgs, err := store.ListMessages(thread.ID)
		if err != nil {
			t.Errorf("ListMessages(%s) failed: %v", thread.ID, err)
			continue
		}
		totalMessages += len(msgs)
	}
	if totalMessages != numGoroutines {
		t.Errorf("expected %d total messages, got %d", numGoroutines, totalMessages)
	}
}

// --- Edge Case Tests ---

func TestMarkdownThreadFilenameCollision(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("collision-test", "Test filename collision", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}

	// Create two threads whose subjects slugify identically
	thread1 := models.NewThread(topic.ID, "Hello World!", "test@cli")
	if err := store.CreateThread(thread1); err != nil {
		t.Fatalf("CreateThread (1) failed: %v", err)
	}
	thread2 := models.NewThread(topic.ID, "Hello World?", "test@cli")
	if err := store.CreateThread(thread2); err != nil {
		t.Fatalf("CreateThread (2) failed: %v", err)
	}

	// Both threads should be independently retrievable
	got1, err := store.GetThread(thread1.ID)
	if err != nil {
		t.Fatalf("GetThread (1) failed: %v", err)
	}
	if got1.Subject != "Hello World!" {
		t.Errorf("thread 1 subject mismatch: want %q, got %q", "Hello World!", got1.Subject)
	}

	got2, err := store.GetThread(thread2.ID)
	if err != nil {
		t.Fatalf("GetThread (2) failed: %v", err)
	}
	if got2.Subject != "Hello World?" {
		t.Errorf("thread 2 subject mismatch: want %q, got %q", "Hello World?", got2.Subject)
	}

	// ListThreads should return both
	threads, err := store.ListThreads(topic.ID)
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) != 2 {
		t.Errorf("expected 2 threads, got %d", len(threads))
	}

	// Post messages to both and verify they don't interfere
	msg1 := models.NewMessage(thread1.ID, "Message in thread 1", "test@cli")
	msg2 := models.NewMessage(thread2.ID, "Message in thread 2", "test@cli")
	if err := store.CreateMessage(msg1); err != nil {
		t.Fatalf("CreateMessage (1) failed: %v", err)
	}
	if err := store.CreateMessage(msg2); err != nil {
		t.Fatalf("CreateMessage (2) failed: %v", err)
	}

	msgs1, _ := store.ListMessages(thread1.ID)
	msgs2, _ := store.ListMessages(thread2.ID)
	if len(msgs1) != 1 || msgs1[0].Content != "Message in thread 1" {
		t.Errorf("thread 1 messages wrong: got %v", msgs1)
	}
	if len(msgs2) != 1 || msgs2[0].Content != "Message in thread 2" {
		t.Errorf("thread 2 messages wrong: got %v", msgs2)
	}
}

func TestMarkdownMessageContainingMarkdownSeparator(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("separator-test", "Test markdown separators", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Separator Test", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}

	// Post a message containing "---" (the separator used between messages)
	contentWithSeparator := "Here is some content\n---\nThis looks like a separator\n---\nBut it's all one message"
	msg := models.NewMessage(thread.ID, contentWithSeparator, "test@cli")
	if err := store.CreateMessage(msg); err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	// Post a second message after it
	msg2 := models.NewMessage(thread.ID, "Second message", "test@cli")
	time.Sleep(time.Millisecond) // ensure distinct timestamp
	if err := store.CreateMessage(msg2); err != nil {
		t.Fatalf("CreateMessage (2) failed: %v", err)
	}

	// Retrieve and verify both messages
	messages, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	// The HTML comment separator ensures "---" in content does not corrupt parsing.
	if len(messages) != 2 {
		t.Errorf("expected exactly 2 messages, got %d", len(messages))
	}

	// Verify the first message content is fully preserved including "---"
	if messages[0].Content != contentWithSeparator {
		t.Errorf("first message content was corrupted:\nwant: %q\ngot:  %q", contentWithSeparator, messages[0].Content)
	}

	// Verify the second message is intact
	if messages[1].Content != "Second message" {
		t.Errorf("expected second message content 'Second message', got %q", messages[1].Content)
	}
}

func TestMarkdownEmptyThread(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("empty-test", "Test empty thread", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Empty Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}

	// Thread with zero messages should be retrievable
	got, err := store.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread failed: %v", err)
	}
	if got.Subject != "Empty Thread" {
		t.Errorf("expected subject 'Empty Thread', got %q", got.Subject)
	}

	// ListMessages should return empty slice, not error
	messages, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages for empty thread failed: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}

	// ListThreads should include the empty thread
	threads, err := store.ListThreads(topic.ID)
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) != 1 {
		t.Errorf("expected 1 thread, got %d", len(threads))
	}
}

func TestMarkdownTopicWithSpecialCharacters(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	tests := []struct {
		name string
		desc string
	}{
		{"topic with spaces", "Has spaces in name"},
		{"topic-with-hyphens", "Has hyphens in name"},
		{"UPPERCASE", "Uppercase topic"},
		{"123-numbers", "Starts with numbers"},
	}

	for _, tt := range tests {
		topic := models.NewTopic(tt.name, tt.desc, "test@cli")
		if err := store.CreateTopic(topic); err != nil {
			t.Errorf("CreateTopic(%q) failed: %v", tt.name, err)
			continue
		}

		// Verify it can be retrieved by ID
		got, err := store.GetTopic(topic.ID)
		if err != nil {
			t.Errorf("GetTopic(%q) failed: %v", tt.name, err)
			continue
		}
		if got.Name != tt.name {
			t.Errorf("topic name mismatch: want %q, got %q", tt.name, got.Name)
		}

		// Verify it can be retrieved by name
		gotByName, err := store.GetTopicByName(tt.name)
		if err != nil {
			t.Errorf("GetTopicByName(%q) failed: %v", tt.name, err)
			continue
		}
		if gotByName.ID != topic.ID {
			t.Errorf("GetTopicByName(%q) returned wrong ID", tt.name)
		}

		// Verify directory was created
		topicDir := filepath.Join(store.dataDir, tt.name)
		if _, err := os.Stat(topicDir); os.IsNotExist(err) {
			t.Errorf("topic directory not created for %q", tt.name)
		}
	}

	// Verify all topics are listed
	topics, err := store.ListTopics(true)
	if err != nil {
		t.Fatalf("ListTopics failed: %v", err)
	}
	if len(topics) != len(tests) {
		t.Errorf("expected %d topics, got %d", len(tests), len(topics))
	}
}

func TestMarkdownMalformedTopicsYaml(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	// Write garbage to the _topics.yaml file
	topicsPath := filepath.Join(store.dataDir, "_topics.yaml")
	if err := os.WriteFile(topicsPath, []byte("this is not: [valid: yaml: {{{}"), 0640); err != nil {
		t.Fatalf("failed to write malformed yaml: %v", err)
	}

	// Operations that read topics should return an error, not panic
	_, err := store.ListTopics(true)
	if err == nil {
		t.Error("expected error from ListTopics with malformed _topics.yaml")
	}

	_, err = store.GetTopic(uuid.New())
	if err == nil {
		t.Error("expected error from GetTopic with malformed _topics.yaml")
	}

	_, err = store.GetTopicByName("anything")
	if err == nil {
		t.Error("expected error from GetTopicByName with malformed _topics.yaml")
	}
}

func TestMarkdownTopicsYamlWithInvalidEntriesFailsClosed(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	// Create a valid topic first
	validTopic := models.NewTopic("valid-topic", "This one is fine", "test@cli")
	if err := store.CreateTopic(validTopic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}

	// Now corrupt the _topics.yaml by appending an entry with invalid UUID
	topicsPath := filepath.Join(store.dataDir, "_topics.yaml")
	data, err := os.ReadFile(topicsPath)
	if err != nil {
		t.Fatalf("read topics file: %v", err)
	}
	corrupted := string(data) + "\n- id: not-a-valid-uuid\n  name: corrupt-topic\n  created_at: not-a-date\n  created_by: test@cli\n"
	if err := os.WriteFile(topicsPath, []byte(corrupted), 0640); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}

	// Invalid identifiers make repository cardinality unknowable and must fail closed.
	if _, err := store.ListTopics(true); err == nil || !strings.Contains(err.Error(), "not-a-valid-uuid") {
		t.Fatalf("ListTopics invalid topic ID error = %v", err)
	}
	assertFileBytes(t, topicsPath, []byte(corrupted))
}

func TestMarkdownListTopicsFailsClosedOnInvalidTopicEntries(t *testing.T) {
	tests := []struct {
		name      string
		corrupt   func(*topicEntry)
		wantCause string
	}{
		{
			name: "invalid created_at",
			corrupt: func(entry *topicEntry) {
				entry.CreatedAt = "not-a-date"
			},
			wantCause: "parse topic created_at",
		},
		{
			name: "invalid UUID",
			corrupt: func(entry *topicEntry) {
				entry.ID = "not-a-valid-uuid"
			},
			wantCause: "parse topic ID",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, includeArchived := range []bool{false, true} {
				t.Run(fmt.Sprintf("include archived %t", includeArchived), func(t *testing.T) {
					store := newTestMarkdownStore(t)
					valid := models.NewTopic("alpha-valid", "Valid topic", "test@cli")
					corrupt := fromTopicModel(models.NewTopic("zeta-corrupt", "Corrupt topic", "test@cli"))
					corrupt.Archived = true
					test.corrupt(&corrupt)
					if err := store.writeTopics([]topicEntry{fromTopicModel(valid), corrupt}); err != nil {
						t.Fatalf("write topic registry fixture: %v", err)
					}

					topics, err := store.ListTopics(includeArchived)
					if err == nil {
						t.Fatalf("ListTopics returned partial topics %#v for corrupt registry", topics)
					}
					if topics != nil {
						t.Errorf("ListTopics topics = %#v, want nil on corrupt registry", topics)
					}
					for _, want := range []string{store.topicsFilePath(), "entry 2", `"zeta-corrupt"`, test.wantCause} {
						if !strings.Contains(err.Error(), want) {
							t.Errorf("ListTopics error %q lacks context %q", err, want)
						}
					}
				})
			}
		})
	}
}

func TestMarkdownListTopicsFailsClosedOnDuplicateTopicIDs(t *testing.T) {
	for _, firstArchived := range []bool{false, true} {
		for _, duplicateArchived := range []bool{false, true} {
			for _, includeArchived := range []bool{false, true} {
				t.Run(fmt.Sprintf(
					"first archived %t duplicate archived %t include archived %t",
					firstArchived, duplicateArchived, includeArchived,
				), func(t *testing.T) {
					assertListTopicsDuplicateFailsClosed(t, firstArchived, duplicateArchived, includeArchived)
				})
			}
		}
	}
}

func assertListTopicsDuplicateFailsClosed(t *testing.T, firstArchived, duplicateArchived, includeArchived bool) {
	t.Helper()
	store := newTestMarkdownStore(t)
	first := models.NewTopic("alpha-valid", "First occurrence", "test@cli")
	first.Archived = firstArchived
	duplicate := models.NewTopic("zeta-duplicate", "Duplicate occurrence", "test@cli")
	duplicate.ID = first.ID
	duplicate.Archived = duplicateArchived
	if err := store.writeTopics([]topicEntry{fromTopicModel(first), fromTopicModel(duplicate)}); err != nil {
		t.Fatalf("write duplicate topic registry: %v", err)
	}

	topics, err := store.ListTopics(includeArchived)
	if err == nil {
		t.Fatalf("ListTopics returned duplicate topics %#v", topics)
	}
	if topics != nil {
		t.Errorf("ListTopics topics = %#v, want nil on duplicate registry", topics)
	}
	for _, want := range []string{
		store.topicsFilePath(),
		`entry 2 ("zeta-duplicate")`,
		`first occurrence entry 1 ("alpha-valid")`,
		first.ID.String(),
		"ambiguous topic ID",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ListTopics error %q lacks duplicate context %q", err, want)
		}
	}
}

func TestMigrateDataInvalidMarkdownTopicRegistryLeavesDestinationUnchanged(t *testing.T) {
	corruptions := []struct {
		name    string
		corrupt func([]topicEntry)
		want    []string
	}{
		{
			name: "invalid created_at",
			corrupt: func(entries []topicEntry) {
				entries[1].CreatedAt = "not-a-date"
			},
			want: []string{`entry 2 ("zeta-corrupt")`, "parse topic created_at"},
		},
		{
			name: "duplicate UUID",
			corrupt: func(entries []topicEntry) {
				entries[1].ID = entries[0].ID
			},
			want: []string{
				`entry 2 ("zeta-corrupt")`,
				`first occurrence entry 1 ("alpha-valid")`,
				"ambiguous topic ID",
			},
		},
	}

	for _, corruption := range corruptions {
		t.Run(corruption.name, func(t *testing.T) {
			assertInvalidTopicRegistryMigrationFailsClosed(t, corruption.corrupt, corruption.want)
		})
	}
}

func assertInvalidTopicRegistryMigrationFailsClosed(t *testing.T, corrupt func([]topicEntry), want []string) {
	t.Helper()
	src := newTestMarkdownStore(t)
	defer src.Close()
	entries := []topicEntry{
		fromTopicModel(models.NewTopic("alpha-valid", "Valid topic", "test@cli")),
		fromTopicModel(models.NewTopic("zeta-corrupt", "Corrupt topic", "test@cli")),
	}
	corrupt(entries)
	if err := src.writeTopics(entries); err != nil {
		t.Fatalf("write source topic registry fixture: %v", err)
	}

	for _, destination := range []string{"sqlite", "markdown"} {
		t.Run(destination, func(t *testing.T) {
			assertInvalidTopicRegistryDestinationUnchanged(t, src, destination, want)
		})
	}
}

func assertInvalidTopicRegistryDestinationUnchanged(t *testing.T, src *MarkdownStore, destination string, want []string) {
	t.Helper()
	dst := openMigrationDestination(t, destination)
	defer dst.Close()
	seedTestData(t, dst)
	beforeData, err := collectMigrationData(dst)
	if err != nil {
		t.Fatalf("snapshot seeded destination: %v", err)
	}
	var beforeMarkdown markdownTreeSnapshot
	if markdown, ok := dst.(*MarkdownStore); ok {
		beforeMarkdown = snapshotMarkdownTree(t, markdown)
	}

	summary, err := MigrateData(src, dst)
	if err == nil {
		t.Fatalf("MigrateData returned summary %#v for corrupt source registry", summary)
	}
	if summary != nil {
		t.Errorf("MigrateData summary = %#v, want nil", summary)
	}
	for _, context := range append([]string{"list source topics", src.topicsFilePath()}, want...) {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("MigrateData error %q lacks context %q", err, context)
		}
	}
	afterData, snapshotErr := collectMigrationData(dst)
	if snapshotErr != nil {
		t.Fatalf("snapshot destination after source preflight failure: %v", snapshotErr)
	}
	if !reflect.DeepEqual(afterData, beforeData) {
		t.Errorf("destination graph changed after source preflight failure:\nbefore: %#v\n after: %#v", beforeData, afterData)
	}
	if markdown, ok := dst.(*MarkdownStore); ok {
		assertMarkdownTreeUnchanged(t, markdown, beforeMarkdown)
	}
}

func TestMarkdownThreadWithLongSubject(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("long-subject", "Test long subjects", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}

	// A moderately long subject (under filesystem limits) should work fine
	moderateSubject := strings.Repeat("long ", 30)
	thread1 := models.NewThread(topic.ID, moderateSubject, "test@cli")
	if err := store.CreateThread(thread1); err != nil {
		t.Fatalf("CreateThread with moderate subject failed: %v", err)
	}

	got, err := store.GetThread(thread1.ID)
	if err != nil {
		t.Fatalf("GetThread failed: %v", err)
	}
	if got.Subject != moderateSubject {
		t.Errorf("subject was altered: got %d chars, want %d chars", len(got.Subject), len(moderateSubject))
	}

	// Verify messages can be posted and retrieved on the long-subject thread
	msg := models.NewMessage(thread1.ID, "Message in long-subject thread", "test@cli")
	if err := store.CreateMessage(msg); err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}
	messages, err := store.ListMessages(thread1.ID)
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}

	// An extremely long subject (300+ chars) that exceeds filesystem filename limits
	// should return an error rather than panic
	extremeSubject := strings.Repeat("This is a very long subject ", 15)
	thread2 := models.NewThread(topic.ID, extremeSubject, "test@cli")
	err = store.CreateThread(thread2)
	if err == nil {
		// If it succeeds (e.g. on a filesystem with large name limits), that's ok too.
		// Verify the data round-trips.
		got2, getErr := store.GetThread(thread2.ID)
		if getErr != nil {
			t.Fatalf("GetThread (extreme) failed: %v", getErr)
		}
		if got2.Subject != extremeSubject {
			t.Errorf("extreme subject was altered")
		}
	}
	// If err != nil, that's the expected graceful failure for long filenames
}

func TestMarkdownMessageWithMultilineContent(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	topic := models.NewTopic("multiline", "Test multiline content", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Multiline Test", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}

	// Message with various markdown formatting
	content := "# Heading\n\nSome paragraph.\n\n```go\nfunc main() {\n\tprintln(\"hello\")\n}\n```\n\n- list item 1\n- list item 2\n\n> Blockquote here"
	msg := models.NewMessage(thread.ID, content, "test@cli")
	if err := store.CreateMessage(msg); err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	got, err := store.GetMessage(msg.ID)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if got.Content != content {
		t.Errorf("multiline content mismatch:\nwant: %q\ngot:  %q", content, got.Content)
	}
}

func TestMarkdownDeleteNonexistentEntities(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	fakeID := uuid.New()

	err := store.DeleteTopic(fakeID)
	if err == nil {
		t.Error("expected error deleting nonexistent topic")
	}

	err = store.DeleteThread(fakeID)
	if err == nil {
		t.Error("expected error deleting nonexistent thread")
	}

	err = store.DeleteMessage(fakeID)
	if err == nil {
		t.Error("expected error deleting nonexistent message")
	}

	err = store.DeleteAttachment(fakeID)
	if err == nil {
		t.Error("expected error deleting nonexistent attachment")
	}
}

func TestMarkdownArchiveNonexistentTopic(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	err := store.ArchiveTopic(uuid.New(), true)
	if err == nil {
		t.Error("expected error archiving nonexistent topic")
	}
}

func TestMarkdownSetStickyNonexistentThread(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	err := store.SetThreadSticky(uuid.New(), true)
	if err == nil {
		t.Error("expected error setting sticky on nonexistent thread")
	}
}

func TestMarkdownV2MessageContentRoundTripsExactly(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("lossless", "Lossless message bodies", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Exact bytes", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}

	content := "  leading spaces\n<!-- message-separator -->\nUnicode: café 🦬\ntrailing spaces  \n"
	msg := models.NewMessage(thread.ID, content, "test@cli")
	if err := store.CreateMessage(msg); err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	threadPath, err := store.threadFilePath(topic.Name, thread.ID)
	if err != nil {
		t.Fatalf("threadFilePath failed: %v", err)
	}
	data, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("read thread file: %v", err)
	}
	for _, marker := range []string{
		"format_version: 2",
		"updated_at:",
		fmt.Sprintf("<!-- content-bytes:%d -->", len(content)),
	} {
		if !strings.Contains(string(data), marker) {
			t.Errorf("thread file missing %q:\n%s", marker, data)
		}
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	reopened, err := NewMarkdownStore(store.dataDir)
	if err != nil {
		t.Fatalf("reopen MarkdownStore: %v", err)
	}
	defer reopened.Close()
	got, err := reopened.GetMessage(msg.ID)
	if err != nil {
		t.Fatalf("GetMessage after reopen failed: %v", err)
	}
	if got.Content != content {
		t.Errorf("message bytes changed across reopen:\nwant: %q\n got: %q", content, got.Content)
	}
}

func TestMarkdownLegacyThreadReadDoesNotRewrite(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	topic := models.NewTopic("legacy", "Legacy format", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}

	threadID := uuid.New()
	messageID := uuid.New()
	legacy := fmt.Sprintf(`---
id: %s
topic: legacy
subject: Existing thread
created_at: "2025-01-01T00:00:00Z"
created_by: legacy@cli
sticky: false
---

## legacy@cli — 2025-02-01T00:00:00Z
<!-- msg:%s -->

Existing content.
`, threadID, messageID)
	threadPath := filepath.Join(store.dataDir, topic.Name, "existing-thread.md")
	if err := os.WriteFile(threadPath, []byte(legacy), 0640); err != nil {
		t.Fatalf("write legacy thread: %v", err)
	}

	gotThread, err := store.GetThread(threadID)
	if err != nil {
		t.Fatalf("GetThread legacy file failed: %v", err)
	}
	wantUpdatedAt, _ := time.Parse(time.RFC3339, "2025-02-01T00:00:00Z")
	if !gotThread.UpdatedAt.Equal(wantUpdatedAt) {
		t.Errorf("legacy UpdatedAt = %s, want %s", gotThread.UpdatedAt, wantUpdatedAt)
	}
	messages, err := store.ListMessages(threadID)
	if err != nil {
		t.Fatalf("ListMessages legacy file failed: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "Existing content." {
		t.Fatalf("legacy messages = %#v, want one intact message", messages)
	}

	after, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("read legacy thread after reads: %v", err)
	}
	if string(after) != legacy {
		t.Errorf("read-only operations rewrote legacy file:\nwant: %q\n got: %q", legacy, after)
	}
}

func TestMarkdownThreadUpdatePersistsActivityAndOrdering(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("activity", "Activity ordering", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}

	first := models.NewThread(topic.ID, "First", "test@cli")
	if err := store.CreateThread(first); err != nil {
		t.Fatalf("CreateThread first failed: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	second := models.NewThread(topic.ID, "Second", "test@cli")
	if err := store.CreateThread(second); err != nil {
		t.Fatalf("CreateThread second failed: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	beforeUpdate := time.Now().UTC()
	first.Subject = "First updated"
	if err := store.UpdateThread(first); err != nil {
		t.Fatalf("UpdateThread failed: %v", err)
	}
	afterUpdate := time.Now().UTC()
	if first.UpdatedAt.Before(beforeUpdate) || first.UpdatedAt.After(afterUpdate) {
		t.Errorf("UpdateThread activity = %s, want within [%s, %s]", first.UpdatedAt, beforeUpdate, afterUpdate)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	reopened, err := NewMarkdownStore(store.dataDir)
	if err != nil {
		t.Fatalf("reopen MarkdownStore: %v", err)
	}
	defer reopened.Close()
	got, err := reopened.GetThread(first.ID)
	if err != nil {
		t.Fatalf("GetThread after reopen failed: %v", err)
	}
	if !got.UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("persisted activity = %s, want %s", got.UpdatedAt, first.UpdatedAt)
	}
	threads, err := reopened.ListThreads(topic.ID)
	if err != nil {
		t.Fatalf("ListThreads after reopen failed: %v", err)
	}
	if len(threads) != 2 || threads[0].ID != first.ID {
		t.Errorf("thread ordering = %#v, want updated thread first", threads)
	}
}

func TestMarkdownCreateMessageUsesOperationTimeForActivity(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	topic := models.NewTopic("message-activity", "Message activity", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}

	msg := models.NewMessage(thread.ID, "Imported old message", "test@cli")
	msg.CreatedAt = time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)
	beforeCreate := time.Now().UTC()
	if err := store.CreateMessage(msg); err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}
	afterCreate := time.Now().UTC()
	got, err := store.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread failed: %v", err)
	}
	if got.UpdatedAt.Before(beforeCreate) || got.UpdatedAt.After(afterCreate) {
		t.Errorf("message activity = %s, want within [%s, %s]", got.UpdatedAt, beforeCreate, afterCreate)
	}
}

func TestMarkdownNonActivityMutationsPreserveActivity(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("preserve", "Preserve activity", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	msg := models.NewMessage(thread.ID, "Message", "test@cli")
	if err := store.CreateMessage(msg); err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}
	active, err := store.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread failed: %v", err)
	}
	wantActivity := active.UpdatedAt
	assertActivity := func(operation string) {
		t.Helper()
		got, getErr := store.GetThread(thread.ID)
		if getErr != nil {
			t.Fatalf("GetThread after %s failed: %v", operation, getErr)
		}
		if !got.UpdatedAt.Equal(wantActivity) {
			t.Errorf("activity after %s = %s, want %s", operation, got.UpdatedAt, wantActivity)
		}
	}

	if err := store.SetThreadSticky(thread.ID, true); err != nil {
		t.Fatalf("SetThreadSticky failed: %v", err)
	}
	assertActivity("sticky toggle")

	topic.Name = "preserved"
	if err := store.UpdateTopic(topic); err != nil {
		t.Fatalf("UpdateTopic rename failed: %v", err)
	}
	assertActivity("topic rename")

	editedAt := time.Now().UTC()
	msg.Content = "Edited"
	msg.EditedAt = &editedAt
	if err := store.UpdateMessage(msg); err != nil {
		t.Fatalf("UpdateMessage failed: %v", err)
	}
	assertActivity("message edit")

	if err := store.DeleteMessage(msg.ID); err != nil {
		t.Fatalf("DeleteMessage failed: %v", err)
	}
	assertActivity("message delete")

	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	reopened, err := NewMarkdownStore(store.dataDir)
	if err != nil {
		t.Fatalf("reopen MarkdownStore: %v", err)
	}
	defer reopened.Close()
	got, err := reopened.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread after reopen failed: %v", err)
	}
	if !got.UpdatedAt.Equal(wantActivity) {
		t.Errorf("activity after reopen = %s, want %s", got.UpdatedAt, wantActivity)
	}
}

type malformedV2Fixture struct {
	store      *MarkdownStore
	topic      *models.Topic
	thread     *models.Thread
	messages   []*models.Message
	threadPath string
	content    string
}

func newMalformedV2Fixture(t *testing.T) *malformedV2Fixture {
	t.Helper()
	store := newTestMarkdownStore(t)
	t.Cleanup(func() { _ = store.Close() })
	topic := models.NewTopic("corrupt", "Corrupt v2", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	messages := []*models.Message{
		models.NewMessage(thread.ID, "First", "test@cli"),
		models.NewMessage(thread.ID, "Second message with unique length", "test@cli"),
		models.NewMessage(thread.ID, "Third", "test@cli"),
	}
	for _, message := range messages {
		if err := store.CreateMessage(message); err != nil {
			t.Fatalf("CreateMessage failed: %v", err)
		}
	}

	threadPath, err := store.threadFilePath(topic.Name, thread.ID)
	if err != nil {
		t.Fatalf("threadFilePath failed: %v", err)
	}
	data, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("read thread file: %v", err)
	}
	second := messages[1]
	validMetadata := fmt.Sprintf(
		"<!-- msg:%s -->\n<!-- content-bytes:%d -->",
		second.ID,
		len(second.Content),
	)
	invalidMetadata := fmt.Sprintf(
		"<!-- msg:%s -->\n<!-- content-bytes:999999 -->",
		second.ID,
	)
	corrupted := strings.Replace(string(data), validMetadata, invalidMetadata, 1)
	if corrupted == string(data) {
		t.Fatal("failed to corrupt the second message metadata")
	}
	if err := os.WriteFile(threadPath, []byte(corrupted), 0640); err != nil {
		t.Fatalf("write corrupt thread file: %v", err)
	}

	return &malformedV2Fixture{
		store:      store,
		topic:      topic,
		thread:     thread,
		messages:   messages,
		threadPath: threadPath,
		content:    corrupted,
	}
}

func TestMarkdownMalformedV2MutationLeavesFileUnchanged(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*MarkdownStore, *models.Topic, *models.Thread, []*models.Message) error
	}{
		{
			name: "thread update",
			mutate: func(store *MarkdownStore, _ *models.Topic, thread *models.Thread, _ []*models.Message) error {
				thread.Subject = "Changed"
				return store.UpdateThread(thread)
			},
		},
		{
			name: "sticky toggle",
			mutate: func(store *MarkdownStore, _ *models.Topic, thread *models.Thread, _ []*models.Message) error {
				return store.SetThreadSticky(thread.ID, true)
			},
		},
		{
			name: "topic rename",
			mutate: func(store *MarkdownStore, topic *models.Topic, _ *models.Thread, _ []*models.Message) error {
				topic.Name = "renamed"
				return store.UpdateTopic(topic)
			},
		},
		{
			name: "message create",
			mutate: func(store *MarkdownStore, _ *models.Topic, thread *models.Thread, _ []*models.Message) error {
				return store.CreateMessage(models.NewMessage(thread.ID, "Fourth", "test@cli"))
			},
		},
		{
			name: "message edit",
			mutate: func(store *MarkdownStore, _ *models.Topic, _ *models.Thread, messages []*models.Message) error {
				messages[0].Content = "Changed"
				return store.UpdateMessage(messages[0])
			},
		},
		{
			name: "message delete",
			mutate: func(store *MarkdownStore, _ *models.Topic, _ *models.Thread, messages []*models.Message) error {
				return store.DeleteMessage(messages[0].ID)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newMalformedV2Fixture(t)
			if err := test.mutate(fixture.store, fixture.topic, fixture.thread, fixture.messages); err == nil {
				t.Error("mutation succeeded with malformed v2 input")
			}
			after, err := os.ReadFile(fixture.threadPath)
			if err != nil {
				t.Fatalf("read original thread path after rejected mutation: %v", err)
			}
			if string(after) != fixture.content {
				t.Errorf("rejected mutation changed malformed file:\nwant: %q\n got: %q", fixture.content, after)
			}
		})
	}
}

func TestMarkdownTopicRenameRejectsMalformedFrontmatterBeforeChangingState(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	topic := models.NewTopic("before", "Rename preflight", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	message := models.NewMessage(thread.ID, "Message", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	threadPath, err := store.threadFilePath(topic.Name, thread.ID)
	if err != nil {
		t.Fatalf("threadFilePath failed: %v", err)
	}
	threadData, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("read thread file: %v", err)
	}
	fm, err := parseThreadFrontmatter(string(threadData))
	if err != nil {
		t.Fatalf("parse thread frontmatter: %v", err)
	}
	validUpdatedAt := fmt.Sprintf("updated_at: %q", fm.UpdatedAt)
	corrupted := strings.Replace(string(threadData), validUpdatedAt, `updated_at: "not-a-time"`, 1)
	if corrupted == string(threadData) {
		t.Fatal("failed to corrupt updated_at frontmatter")
	}
	if err := os.WriteFile(threadPath, []byte(corrupted), 0640); err != nil {
		t.Fatalf("write corrupt thread file: %v", err)
	}
	topicsBefore, err := os.ReadFile(store.topicsFilePath())
	if err != nil {
		t.Fatalf("read topics before rename: %v", err)
	}
	oldDir := store.topicDirPath(topic.Name)
	newDir := store.topicDirPath("after")

	topic.Name = "after"
	if err := store.UpdateTopic(topic); err == nil {
		t.Fatal("UpdateTopic succeeded with malformed thread frontmatter")
	}
	topicsAfter, err := os.ReadFile(store.topicsFilePath())
	if err != nil {
		t.Fatalf("read topics after rejected rename: %v", err)
	}
	if string(topicsAfter) != string(topicsBefore) {
		t.Errorf("rejected rename changed _topics.yaml:\nwant: %q\n got: %q", topicsBefore, topicsAfter)
	}
	if _, err := os.Stat(oldDir); err != nil {
		t.Errorf("original topic directory changed after rejected rename: %v", err)
	}
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		t.Errorf("new topic directory exists after rejected rename: %v", err)
	}
	after, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("read thread after rejected rename: %v", err)
	}
	if string(after) != corrupted {
		t.Errorf("rejected rename changed thread bytes:\nwant: %q\n got: %q", corrupted, after)
	}
}

type v2ValidationFixture struct {
	store         *MarkdownStore
	topic         *models.Topic
	thread        *models.Thread
	messages      []*models.Message
	threadPath    string
	content       string
	topicsContent string
	topicName     string
}

func newV2ValidationFixture(t *testing.T) *v2ValidationFixture {
	t.Helper()
	store := newTestMarkdownStore(t)
	t.Cleanup(func() { _ = store.Close() })
	topic := models.NewTopic("guarded", "Strict v2 validation", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	messages := []*models.Message{
		models.NewMessage(thread.ID, "First", "test@cli"),
		models.NewMessage(thread.ID, "Second", "test@cli"),
		models.NewMessage(thread.ID, "Third", "test@cli"),
	}
	editedAt := time.Now().UTC()
	messages[0].EditedAt = &editedAt
	for _, message := range messages {
		if err := store.CreateMessage(message); err != nil {
			t.Fatalf("CreateMessage failed: %v", err)
		}
	}
	threadPath, err := store.threadFilePath(topic.Name, thread.ID)
	if err != nil {
		t.Fatalf("threadFilePath failed: %v", err)
	}
	content, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("read thread file: %v", err)
	}
	topicsContent, err := os.ReadFile(store.topicsFilePath())
	if err != nil {
		t.Fatalf("read topics file: %v", err)
	}
	return &v2ValidationFixture{
		store:         store,
		topic:         topic,
		thread:        thread,
		messages:      messages,
		threadPath:    threadPath,
		content:       string(content),
		topicsContent: string(topicsContent),
		topicName:     topic.Name,
	}
}

func (f *v2ValidationFixture) corrupt(t *testing.T, transform func(string, *v2ValidationFixture) string) {
	t.Helper()
	corrupted := transform(f.content, f)
	if corrupted == f.content {
		t.Fatal("corruption did not change thread content")
	}
	if err := os.WriteFile(f.threadPath, []byte(corrupted), 0640); err != nil {
		t.Fatalf("write corrupt thread file: %v", err)
	}
	f.content = corrupted
}

func v2RewriteMutations() []struct {
	name   string
	mutate func(*v2ValidationFixture) error
} {
	return []struct {
		name   string
		mutate func(*v2ValidationFixture) error
	}{
		{
			name: "thread update",
			mutate: func(f *v2ValidationFixture) error {
				f.thread.Subject = "Changed"
				return f.store.UpdateThread(f.thread)
			},
		},
		{
			name: "sticky toggle",
			mutate: func(f *v2ValidationFixture) error {
				return f.store.SetThreadSticky(f.thread.ID, true)
			},
		},
		{
			name: "topic rename",
			mutate: func(f *v2ValidationFixture) error {
				f.topic.Name = "renamed"
				return f.store.UpdateTopic(f.topic)
			},
		},
		{
			name: "message create",
			mutate: func(f *v2ValidationFixture) error {
				return f.store.CreateMessage(models.NewMessage(f.thread.ID, "Fourth", "test@cli"))
			},
		},
		{
			name: "message edit",
			mutate: func(f *v2ValidationFixture) error {
				f.messages[0].Content = "Changed"
				return f.store.UpdateMessage(f.messages[0])
			},
		},
		{
			name: "message delete",
			mutate: func(f *v2ValidationFixture) error {
				return f.store.DeleteMessage(f.messages[0].ID)
			},
		},
	}
}

func assertV2FixtureUnchanged(t *testing.T, fixture *v2ValidationFixture) {
	t.Helper()
	topics, err := os.ReadFile(fixture.store.topicsFilePath())
	if err != nil {
		t.Fatalf("read topics after rejected mutation: %v", err)
	}
	if string(topics) != fixture.topicsContent {
		t.Errorf("rejected mutation changed topics:\nwant: %q\n got: %q", fixture.topicsContent, topics)
	}
	if _, err := os.Stat(fixture.store.topicDirPath(fixture.topicName)); err != nil {
		t.Errorf("rejected mutation changed original topic directory: %v", err)
	}
	thread, err := os.ReadFile(fixture.threadPath)
	if err != nil {
		t.Fatalf("read thread after rejected mutation: %v", err)
	}
	if string(thread) != fixture.content {
		t.Errorf("rejected mutation changed thread:\nwant: %q\n got: %q", fixture.content, thread)
	}
	if fixture.topic.Name != fixture.topicName {
		if _, err := os.Stat(fixture.store.topicDirPath(fixture.topic.Name)); !os.IsNotExist(err) {
			t.Errorf("rejected mutation created target topic directory: %v", err)
		}
	}
}

func TestMarkdownV2RequiresValidActivityForEveryRewrite(t *testing.T) {
	corruptions := []struct {
		name      string
		transform func(string, *v2ValidationFixture) string
	}{
		{
			name: "missing updated_at",
			transform: func(content string, _ *v2ValidationFixture) string {
				fm, _ := parseThreadFrontmatter(content)
				return strings.Replace(content, fmt.Sprintf("updated_at: %q\n", fm.UpdatedAt), "", 1)
			},
		},
		{
			name: "invalid updated_at",
			transform: func(content string, _ *v2ValidationFixture) string {
				fm, _ := parseThreadFrontmatter(content)
				return strings.Replace(content, fmt.Sprintf("updated_at: %q", fm.UpdatedAt), `updated_at: "invalid"`, 1)
			},
		},
	}

	for _, corruption := range corruptions {
		for _, mutation := range v2RewriteMutations() {
			t.Run(corruption.name+"/"+mutation.name, func(t *testing.T) {
				fixture := newV2ValidationFixture(t)
				fixture.corrupt(t, corruption.transform)
				if err := mutation.mutate(fixture); err == nil {
					t.Error("mutation accepted invalid v2 activity metadata")
				}
				assertV2FixtureUnchanged(t, fixture)
			})
		}
	}
}

func TestMarkdownV2RejectsMalformedFramingForEveryRewrite(t *testing.T) {
	corruptions := []struct {
		name      string
		transform func(string, *v2ValidationFixture) string
	}{
		{
			name: "missing separator",
			transform: func(content string, _ *v2ValidationFixture) string {
				return strings.Replace(content, messageSeparator, "", 1)
			},
		},
		{
			name: "leading separator",
			transform: func(content string, _ *v2ValidationFixture) string {
				body, _ := exactThreadBody(content)
				return content[:len(content)-len(body)] + messageSeparator + body
			},
		},
		{
			name: "junk message id",
			transform: func(content string, fixture *v2ValidationFixture) string {
				marker := fmt.Sprintf("<!-- msg:%s -->", fixture.messages[1].ID)
				return strings.Replace(content, marker, "junk "+marker, 1)
			},
		},
		{
			name: "junk edited timestamp",
			transform: func(content string, fixture *v2ValidationFixture) string {
				marker := fmt.Sprintf("<!-- edited:%s -->", mdstore.FormatTime(fixture.messages[0].EditedAt.UTC()))
				return strings.Replace(content, marker, marker+" junk", 1)
			},
		},
	}

	for _, corruption := range corruptions {
		for _, mutation := range v2RewriteMutations() {
			t.Run(corruption.name+"/"+mutation.name, func(t *testing.T) {
				fixture := newV2ValidationFixture(t)
				fixture.corrupt(t, corruption.transform)
				if err := mutation.mutate(fixture); err == nil {
					t.Error("mutation accepted malformed v2 framing")
				}
				assertV2FixtureUnchanged(t, fixture)
			})
		}
	}
}

func TestMarkdownTopicRenameFailureDoesNotSplitState(t *testing.T) {
	t.Run("destination collision", func(t *testing.T) {
		fixture := newV2ValidationFixture(t)
		targetDir := fixture.store.topicDirPath("occupied")
		if err := os.Mkdir(targetDir, 0750); err != nil {
			t.Fatalf("create occupied target: %v", err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, "keep"), []byte("occupied"), 0640); err != nil {
			t.Fatalf("seed occupied target: %v", err)
		}
		fixture.topic.Name = "occupied"
		if err := fixture.store.UpdateTopic(fixture.topic); err == nil {
			t.Fatal("rename unexpectedly succeeded over occupied directory")
		}
		fixture.topic.Name = fixture.topicName
		assertV2FixtureUnchanged(t, fixture)
	})

	t.Run("thread write permission", func(t *testing.T) {
		fixture := newV2ValidationFixture(t)
		oldDir := fixture.store.topicDirPath(fixture.topicName)
		newDir := fixture.store.topicDirPath("read-only")
		if err := os.Chmod(oldDir, 0550); err != nil {
			t.Fatalf("make topic directory read-only: %v", err)
		}
		defer func() {
			_ = os.Chmod(oldDir, 0750)
			_ = os.Chmod(newDir, 0750)
		}()
		fixture.topic.Name = "read-only"
		if err := fixture.store.UpdateTopic(fixture.topic); err == nil {
			t.Fatal("rename unexpectedly succeeded with unwritable thread directory")
		}
		fixture.topic.Name = fixture.topicName
		assertV2FixtureUnchanged(t, fixture)
		if _, err := os.Stat(newDir); !os.IsNotExist(err) {
			t.Errorf("failed rename left target directory behind: %v", err)
		}
	})
}

type markdownTopicSourceState string

const (
	markdownTopicSourceEmpty     markdownTopicSourceState = "empty source"
	markdownTopicSourcePopulated markdownTopicSourceState = "populated source"
	markdownTopicSourceMissing   markdownTopicSourceState = "missing source"
)

func TestMarkdownTopicRenameToExistingNameIsAtomic(t *testing.T) {
	sourceStates := []markdownTopicSourceState{
		markdownTopicSourceEmpty,
		markdownTopicSourcePopulated,
		markdownTopicSourceMissing,
	}
	for _, sourceState := range sourceStates {
		for _, populateTarget := range []bool{false, true} {
			targetCase := "empty target"
			if populateTarget {
				targetCase = "populated target"
			}
			t.Run(string(sourceState)+"/"+targetCase, func(t *testing.T) {
				exerciseMarkdownDuplicateTopicNameRejection(t, sourceState, populateTarget)
			})
		}
	}
}

func exerciseMarkdownDuplicateTopicNameRejection(
	t *testing.T,
	sourceState markdownTopicSourceState,
	populateTarget bool,
) {
	t.Helper()
	store := newTestMarkdownStore(t)
	source := models.NewTopic("source", "Source topic", "test@cli")
	target := models.NewTopic("target", "Target topic", "test@cli")
	if err := store.CreateTopic(source); err != nil {
		t.Fatalf("CreateTopic source: %v", err)
	}
	if err := store.CreateTopic(target); err != nil {
		t.Fatalf("CreateTopic target: %v", err)
	}

	sourceSnapshot := prepareMarkdownTopicSourceSnapshot(t, store, source, sourceState)

	if populateTarget {
		targetThread := models.NewThread(target.ID, "Target thread", "test@cli")
		if err := store.CreateThread(targetThread); err != nil {
			t.Fatalf("CreateThread target: %v", err)
		}
		if err := store.CreateMessage(models.NewMessage(targetThread.ID, "target bytes", "test@cli")); err != nil {
			t.Fatalf("CreateMessage target: %v", err)
		}
	}

	registryBefore, err := os.ReadFile(store.topicsFilePath())
	if err != nil {
		t.Fatalf("read topic registry before rejected rename: %v", err)
	}
	treeBefore := snapshotMarkdownTree(t, store)
	update := *source
	update.Name = target.Name
	update.Description = "must not persist"
	if err := store.UpdateTopic(&update); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("UpdateTopic existing-name error = %v, want explicit duplicate-name rejection", err)
	}
	assertMarkdownTreeUnchanged(t, store, treeBefore)
	registryAfter, err := os.ReadFile(store.topicsFilePath())
	if err != nil {
		t.Fatalf("read topic registry after rejected rename: %v", err)
	}
	if string(registryAfter) != string(registryBefore) {
		t.Fatalf("rejected rename changed topic registry bytes:\nwant: %q\n got: %q", registryBefore, registryAfter)
	}
	assertMarkdownTopicSourceUnchanged(t, sourceSnapshot)
}

type markdownTopicSourceSnapshot struct {
	state      markdownTopicSourceState
	dir        string
	mode       os.FileMode
	threadPath string
	thread     []byte
}

func prepareMarkdownTopicSourceSnapshot(
	t *testing.T,
	store *MarkdownStore,
	source *models.Topic,
	state markdownTopicSourceState,
) markdownTopicSourceSnapshot {
	t.Helper()
	snapshot := markdownTopicSourceSnapshot{state: state, dir: store.topicDirPath(source.Name)}
	if state == markdownTopicSourceMissing {
		if err := os.Remove(snapshot.dir); err != nil {
			t.Fatalf("remove source directory to exercise missing-source rename: %v", err)
		}
		return snapshot
	}
	if state == markdownTopicSourcePopulated {
		sourceThread := models.NewThread(source.ID, "Source thread", "test@cli")
		if err := store.CreateThread(sourceThread); err != nil {
			t.Fatalf("CreateThread source: %v", err)
		}
		if err := store.CreateMessage(models.NewMessage(sourceThread.ID, "source bytes", "test@cli")); err != nil {
			t.Fatalf("CreateMessage source: %v", err)
		}
		var err error
		snapshot.threadPath, err = store.threadFilePath(source.Name, sourceThread.ID)
		if err != nil {
			t.Fatalf("threadFilePath source: %v", err)
		}
		snapshot.thread, err = os.ReadFile(snapshot.threadPath)
		if err != nil {
			t.Fatalf("read source thread before rejected rename: %v", err)
		}
	} else if state != markdownTopicSourceEmpty {
		t.Fatalf("unsupported Markdown topic source state %q", state)
	}
	info, err := os.Stat(snapshot.dir)
	if err != nil {
		t.Fatalf("stat source directory before rejected rename: %v", err)
	}
	snapshot.mode = info.Mode()
	return snapshot
}

func assertMarkdownTopicSourceUnchanged(t *testing.T, snapshot markdownTopicSourceSnapshot) {
	t.Helper()
	if snapshot.state == markdownTopicSourceMissing {
		if _, err := os.Stat(snapshot.dir); !os.IsNotExist(err) {
			t.Fatalf("rejected rename recreated missing source directory: %v", err)
		}
		return
	}
	sourceInfo, err := os.Stat(snapshot.dir)
	if err != nil {
		t.Fatalf("stat source directory after rejected rename: %v", err)
	}
	if sourceInfo.Mode() != snapshot.mode {
		t.Fatalf("rejected rename changed source directory mode: got %v, want %v", sourceInfo.Mode(), snapshot.mode)
	}
	if snapshot.state == markdownTopicSourceEmpty {
		entries, err := os.ReadDir(snapshot.dir)
		if err != nil {
			t.Fatalf("read empty source directory after rejected rename: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("rejected rename populated empty source directory: %#v", entries)
		}
		return
	}
	sourceThreadAfter, err := os.ReadFile(snapshot.threadPath)
	if err != nil {
		t.Fatalf("read source thread after rejected rename: %v", err)
	}
	if string(sourceThreadAfter) != string(snapshot.thread) {
		t.Fatalf("rejected rename changed source thread bytes:\nwant: %q\n got: %q", snapshot.thread, sourceThreadAfter)
	}
}

func TestMarkdownTopicSameNameUpdatePreservesTopicFiles(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("same-name", "Before", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if err := store.CreateMessage(models.NewMessage(thread.ID, "preserve bytes", "test@cli")); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	topicDir := store.topicDirPath(topic.Name)
	threadPath, err := store.threadFilePath(topic.Name, thread.ID)
	if err != nil {
		t.Fatalf("threadFilePath: %v", err)
	}
	threadBefore, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("read thread before update: %v", err)
	}

	topic.Description = "After"
	if err := store.UpdateTopic(topic); err != nil {
		t.Fatalf("same-name UpdateTopic: %v", err)
	}
	stored, err := store.GetTopic(topic.ID)
	if err != nil {
		t.Fatalf("GetTopic after same-name update: %v", err)
	}
	if stored.Name != topic.Name || stored.Description != topic.Description {
		t.Fatalf("same-name update stored %#v, want name %q description %q", stored, topic.Name, topic.Description)
	}
	if _, err := os.Stat(topicDir); err != nil {
		t.Fatalf("same-name update changed topic path: %v", err)
	}
	threadAfter, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("read thread after update: %v", err)
	}
	if string(threadAfter) != string(threadBefore) {
		t.Fatalf("same-name update changed thread bytes:\nwant: %q\n got: %q", threadBefore, threadAfter)
	}
}

func TestMarkdownAttachmentFullUUIDIsolationAndOrdering(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("attachments", "Attachment identity", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Shared prefix", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	firstMessage := models.NewMessage(thread.ID, "First", "test@cli")
	firstMessage.ID = uuid.MustParse("12345678-0000-4000-8000-000000000001")
	secondMessage := models.NewMessage(thread.ID, "Second", "test@cli")
	secondMessage.ID = uuid.MustParse("12345678-0000-4000-8000-000000000002")
	for _, message := range []*models.Message{firstMessage, secondMessage} {
		if err := store.CreateMessage(message); err != nil {
			t.Fatalf("CreateMessage failed: %v", err)
		}
	}

	tie := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	firstAttachment := models.NewAttachment(firstMessage.ID, "same.txt", "text/plain", []byte("first"))
	firstAttachment.ID = uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000002")
	firstAttachment.CreatedAt = tie
	secondAttachment := models.NewAttachment(firstMessage.ID, "later.txt", "text/plain", []byte("later"))
	secondAttachment.ID = uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000003")
	secondAttachment.CreatedAt = tie.Add(time.Second)
	tieAttachment := models.NewAttachment(firstMessage.ID, "tie.txt", "text/plain", []byte("tie"))
	tieAttachment.ID = uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000001")
	tieAttachment.CreatedAt = tie
	isolatedAttachment := models.NewAttachment(secondMessage.ID, "same.txt", "text/plain", []byte("second"))
	for _, attachment := range []*models.Attachment{firstAttachment, secondAttachment, tieAttachment, isolatedAttachment} {
		if err := store.CreateAttachment(attachment); err != nil {
			t.Fatalf("CreateAttachment failed: %v", err)
		}
	}

	for _, messageID := range []uuid.UUID{firstMessage.ID, secondMessage.ID} {
		fullDir := filepath.Join(store.dataDir, topic.Name, "_attachments", messageID.String())
		if _, err := os.Stat(fullDir); err != nil {
			t.Errorf("full UUID attachment directory %q missing: %v", fullDir, err)
		}
	}
	if _, err := os.Stat(filepath.Join(store.dataDir, topic.Name, "_attachments", "12345678")); !os.IsNotExist(err) {
		t.Errorf("new attachment used legacy prefix directory: %v", err)
	}

	attachments, err := store.ListAttachments(firstMessage.ID)
	if err != nil {
		t.Fatalf("ListAttachments failed: %v", err)
	}
	wantIDs := []uuid.UUID{tieAttachment.ID, firstAttachment.ID, secondAttachment.ID}
	if len(attachments) != len(wantIDs) {
		t.Fatalf("ListAttachments returned %d attachments, want %d", len(attachments), len(wantIDs))
	}
	for i, wantID := range wantIDs {
		if attachments[i].ID != wantID {
			t.Errorf("attachment %d ID = %s, want %s", i, attachments[i].ID, wantID)
		}
	}
	isolated, err := store.ListAttachments(secondMessage.ID)
	if err != nil {
		t.Fatalf("ListAttachments second message failed: %v", err)
	}
	if len(isolated) != 1 || string(isolated[0].Data) != "second" {
		t.Errorf("second message attachments = %#v, want isolated data", isolated)
	}
}

func newLegacyAttachmentFixture(t *testing.T, ambiguous bool) (*MarkdownStore, *models.Thread, *models.Message, *models.Attachment, string) {
	t.Helper()
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("legacy-attachments", "Legacy attachments", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Legacy", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	message := models.NewMessage(thread.ID, "Owner", "test@cli")
	message.ID = uuid.MustParse("87654321-0000-4000-8000-000000000001")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}
	if ambiguous {
		other := models.NewMessage(thread.ID, "Collision", "test@cli")
		other.ID = uuid.MustParse("87654321-0000-4000-8000-000000000002")
		if err := store.CreateMessage(other); err != nil {
			t.Fatalf("CreateMessage collision failed: %v", err)
		}
	}
	attachment := models.NewAttachment(message.ID, "legacy.txt", "text/plain", []byte("legacy data"))
	legacyDir := filepath.Join(store.dataDir, topic.Name, "_attachments", "87654321")
	if err := os.MkdirAll(legacyDir, 0750); err != nil {
		t.Fatalf("create legacy directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, attachment.Filename), attachment.Data, 0640); err != nil {
		t.Fatalf("write legacy attachment: %v", err)
	}
	meta := attachmentMeta{ID: attachment.ID.String(), MessageID: message.ID.String(), Filename: attachment.Filename, MimeType: attachment.MimeType, CreatedAt: mdstore.FormatTime(attachment.CreatedAt.UTC())}
	if err := mdstore.WriteYAML(filepath.Join(legacyDir, attachment.Filename+".meta.yaml"), &meta); err != nil {
		t.Fatalf("write legacy metadata: %v", err)
	}
	return store, thread, message, attachment, legacyDir
}

func TestMarkdownAttachmentLegacyOwnership(t *testing.T) {
	t.Run("unambiguous read", func(t *testing.T) {
		store, _, message, attachment, legacyDir := newLegacyAttachmentFixture(t, false)
		before, err := os.ReadFile(filepath.Join(legacyDir, attachment.Filename))
		if err != nil {
			t.Fatalf("read legacy bytes: %v", err)
		}
		attachments, err := store.ListAttachments(message.ID)
		if err != nil {
			t.Fatalf("ListAttachments legacy failed: %v", err)
		}
		if len(attachments) != 1 || string(attachments[0].Data) != "legacy data" {
			t.Errorf("legacy attachments = %#v, want one intact attachment", attachments)
		}
		after, err := os.ReadFile(filepath.Join(legacyDir, attachment.Filename))
		if err != nil || string(after) != string(before) {
			t.Errorf("legacy read changed data: bytes=%q err=%v", after, err)
		}
	})

	for _, operation := range []string{"list", "get", "delete attachment", "delete message", "delete thread"} {
		t.Run("ambiguous "+operation+" is atomic", func(t *testing.T) {
			assertAmbiguousLegacyOperationAtomic(t, operation)
		})
	}
}

func assertAmbiguousLegacyOperationAtomic(t *testing.T, operation string) {
	t.Helper()
	store, thread, message, attachment, legacyDir := newLegacyAttachmentFixture(t, true)
	threadPath, err := store.threadFilePath("legacy-attachments", thread.ID)
	if err != nil {
		t.Fatalf("threadFilePath failed: %v", err)
	}
	threadBefore, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("read thread before operation: %v", err)
	}
	dataPath := filepath.Join(legacyDir, attachment.Filename)
	dataBefore, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("read attachment before operation: %v", err)
	}
	var operationErr error
	switch operation {
	case "list":
		_, operationErr = store.ListAttachments(message.ID)
	case "get":
		_, operationErr = store.GetAttachment(attachment.ID)
	case "delete attachment":
		operationErr = store.DeleteAttachment(attachment.ID)
	case "delete message":
		operationErr = store.DeleteMessage(message.ID)
	case "delete thread":
		operationErr = store.DeleteThread(thread.ID)
	}
	if operationErr == nil || !strings.Contains(operationErr.Error(), "collision") {
		t.Fatalf("ambiguous %s error = %v, want collision error", operation, operationErr)
	}
	assertFileBytes(t, threadPath, threadBefore)
	assertFileBytes(t, dataPath, dataBefore)
}

func TestMarkdownAttachmentAndTopicPathContainment(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("safe topic", "Safe", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Paths", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	message := models.NewMessage(thread.ID, "Paths", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	t.Run("missing message", func(t *testing.T) {
		if _, err := store.ListAttachments(uuid.New()); err == nil || !strings.Contains(err.Error(), "message not found") {
			t.Errorf("ListAttachments missing message error = %v", err)
		}
	})

	t.Run("allowed Unicode and punctuation", func(t *testing.T) {
		attachment := models.NewAttachment(message.ID, "résumé 2026 (final)! #1.txt", "text/plain", []byte("safe"))
		if err := store.CreateAttachment(attachment); err != nil {
			t.Fatalf("CreateAttachment safe filename failed: %v", err)
		}
		got, err := store.GetAttachment(attachment.ID)
		if err != nil {
			t.Fatalf("GetAttachment safe filename failed: %v", err)
		}
		if got.Filename != attachment.Filename {
			t.Errorf("safe filename = %q, want %q", got.Filename, attachment.Filename)
		}
	})

	invalidNames := []string{".", "..", "../escape", `..\escape`, "nested/name", `nested\name`, filepath.Join(store.dataDir, "absolute")}
	for _, name := range invalidNames {
		t.Run("invalid attachment "+strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			attachmentDir := filepath.Join(store.dataDir, topic.Name, "_attachments", message.ID.String())
			before, _ := os.ReadDir(attachmentDir)
			attachment := models.NewAttachment(message.ID, name, "text/plain", []byte("unsafe"))
			if err := store.CreateAttachment(attachment); err == nil {
				t.Fatalf("CreateAttachment accepted unsafe filename %q", name)
			}
			after, _ := os.ReadDir(attachmentDir)
			if len(after) != len(before) {
				t.Errorf("rejected attachment %q changed directory entries: before=%d after=%d", name, len(before), len(after))
			}
		})
	}
}

func TestMarkdownAttachmentAndTopicSymlinkContainment(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("safe topic", "Safe", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	thread := models.NewThread(topic.ID, "Paths", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	message := models.NewMessage(thread.ID, "Paths", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}
	t.Run("attachment symlink escape", func(t *testing.T) {
		outside := t.TempDir()
		for _, dirName := range []string{message.ID.String(), message.ID.String()[:8]} {
			attachmentBase := filepath.Join(store.dataDir, topic.Name, "_attachments")
			if err := os.MkdirAll(attachmentBase, 0750); err != nil {
				t.Fatalf("create attachment base: %v", err)
			}
			path := filepath.Join(attachmentBase, dirName)
			if err := os.RemoveAll(path); err != nil {
				t.Fatalf("remove attachment path: %v", err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		}
		attachment := models.NewAttachment(message.ID, "escaped.txt", "text/plain", []byte("escape"))
		if err := store.CreateAttachment(attachment); err == nil {
			t.Fatal("CreateAttachment followed attachment-directory symlink outside board root")
		}
		if _, err := os.Stat(filepath.Join(outside, attachment.Filename)); !os.IsNotExist(err) {
			t.Errorf("rejected symlink escape wrote outside board root: %v", err)
		}
	})
}

func TestMarkdownTopicNameAndSymlinkContainment(t *testing.T) {
	store := newTestMarkdownStore(t)
	if err := store.CreateTopic(models.NewTopic("safe topic", "Safe", "test@cli")); err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	invalidNames := []string{".", "..", "../escape", `..\escape`, "nested/name", `nested\name`, filepath.Join(store.dataDir, "absolute")}
	t.Run("topic names are validated before metadata changes", func(t *testing.T) {
		topicsBefore, err := os.ReadFile(store.topicsFilePath())
		if err != nil {
			t.Fatalf("read topics before invalid creates: %v", err)
		}
		for _, name := range invalidNames {
			if err := store.CreateTopic(models.NewTopic(name, "Unsafe", "test@cli")); err == nil {
				t.Errorf("CreateTopic accepted unsafe name %q", name)
			}
		}
		topicsAfter, err := os.ReadFile(store.topicsFilePath())
		if err != nil || string(topicsAfter) != string(topicsBefore) {
			t.Errorf("invalid topic create changed metadata: bytes=%q err=%v", topicsAfter, err)
		}
	})

	t.Run("topic symlink escape is atomic", func(t *testing.T) {
		outside := t.TempDir()
		link := filepath.Join(store.dataDir, "linked topic")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		topicsBefore, err := os.ReadFile(store.topicsFilePath())
		if err != nil {
			t.Fatalf("read topics before symlink create: %v", err)
		}
		if err := store.CreateTopic(models.NewTopic("linked topic", "Unsafe", "test@cli")); err == nil {
			t.Fatal("CreateTopic accepted a directory symlink outside board root")
		}
		topicsAfter, err := os.ReadFile(store.topicsFilePath())
		if err != nil || string(topicsAfter) != string(topicsBefore) {
			t.Errorf("symlink topic create changed metadata: bytes=%q err=%v", topicsAfter, err)
		}
	})
}

func writeTopicRegistryEntry(t *testing.T, store *MarkdownStore, topic *models.Topic) {
	t.Helper()
	if err := store.writeTopics([]topicEntry{fromTopicModel(topic)}); err != nil {
		t.Fatalf("write topic registry: %v", err)
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s bytes = %q, want %q", path, got, want)
	}
}

func TestMarkdownRejectsUnsafeTopicRegistryConsumers(t *testing.T) {
	for _, registry := range []struct {
		name  string
		setup func(*testing.T, *MarkdownStore) (string, string)
	}{
		{
			name: "traversal",
			setup: func(t *testing.T, store *MarkdownStore) (string, string) {
				outside := filepath.Join(filepath.Dir(store.dataDir), "outside-topic")
				if err := os.MkdirAll(outside, 0750); err != nil {
					t.Fatalf("create outside topic: %v", err)
				}
				return "../outside-topic", outside
			},
		},
		{
			name: "symlink",
			setup: func(t *testing.T, store *MarkdownStore) (string, string) {
				outside := t.TempDir()
				if err := os.Symlink(outside, filepath.Join(store.dataDir, "linked-topic")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return "linked-topic", outside
			},
		},
	} {
		for _, operation := range []string{"create thread", "list threads", "get message"} {
			t.Run(registry.name+"/"+operation, func(t *testing.T) {
				assertUnsafeTopicRegistryConsumer(t, registry.setup, operation)
			})
		}
	}
}

func assertUnsafeTopicRegistryConsumer(t *testing.T, setup func(*testing.T, *MarkdownStore) (string, string), operation string) {
	t.Helper()
	store := newTestMarkdownStore(t)
	topicName, outside := setup(t, store)
	topic := models.NewTopic(topicName, "unsafe registry", "test@cli")
	writeTopicRegistryEntry(t, store, topic)
	sentinelPath := filepath.Join(outside, "sentinel")
	sentinel := []byte("outside must stay unchanged")
	if err := os.WriteFile(sentinelPath, sentinel, 0640); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	var err error
	wantEntries := 1
	switch operation {
	case "create thread":
		err = store.CreateThread(models.NewThread(topic.ID, "escaped", "test@cli"))
	case "list threads":
		_, err = store.ListThreads(topic.ID)
	case "get message":
		wantEntries = 2
		outsideThread := models.NewThread(topic.ID, "Outside", "test@cli")
		outsideMessage := models.NewMessage(outsideThread.ID, "Outside message", "test@cli")
		content, renderErr := renderThread(outsideThread, topicName, []*parsedMessage{{ID: outsideMessage.ID, CreatedBy: outsideMessage.CreatedBy, CreatedAt: outsideMessage.CreatedAt, Content: outsideMessage.Content}})
		if renderErr != nil {
			t.Fatalf("render outside thread: %v", renderErr)
		}
		if writeErr := os.WriteFile(filepath.Join(outside, "outside.md"), []byte(content), 0640); writeErr != nil {
			t.Fatalf("write outside thread: %v", writeErr)
		}
		_, err = store.GetMessage(outsideMessage.ID)
	}
	if err == nil {
		t.Fatalf("%s accepted unsafe registry topic %q", operation, topicName)
	}
	assertFileBytes(t, sentinelPath, sentinel)
	entries, readErr := os.ReadDir(outside)
	if readErr != nil {
		t.Fatalf("read outside directory: %v", readErr)
	}
	if len(entries) != wantEntries {
		t.Errorf("%s changed outside directory entries: %#v", operation, entries)
	}
}

func TestMarkdownAttachmentCollisionUsesFullIdentity(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("collision-identity", "Collision identity", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	message := models.NewMessage(thread.ID, "Message", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	seed := models.NewAttachment(message.ID, "same.txt", "text/plain", []byte("seed"))
	first := models.NewAttachment(message.ID, "same.txt", "text/plain", []byte("first"))
	first.ID = uuid.MustParse("abcdef12-0000-4000-8000-000000000001")
	second := models.NewAttachment(message.ID, "same.txt", "text/plain", []byte("second"))
	second.ID = uuid.MustParse("abcdef12-0000-4000-8000-000000000002")
	for _, attachment := range []*models.Attachment{seed, first, second} {
		if err := store.CreateAttachment(attachment); err != nil {
			t.Fatalf("CreateAttachment: %v", err)
		}
	}
	for _, want := range []*models.Attachment{first, second} {
		got, err := store.GetAttachment(want.ID)
		if err != nil {
			t.Fatalf("GetAttachment(%s): %v", want.ID, err)
		}
		if string(got.Data) != string(want.Data) || got.Filename != "same.txt" {
			t.Errorf("GetAttachment(%s) = filename %q data %q", want.ID, got.Filename, got.Data)
		}
	}
}

func TestMarkdownAttachmentSearchSkipsTopicsWithoutAttachments(t *testing.T) {
	store := newTestMarkdownStore(t)
	empty := models.NewTopic("empty-attachments", "Empty", "test@cli")
	if err := store.CreateTopic(empty); err != nil {
		t.Fatalf("CreateTopic empty: %v", err)
	}
	topic := models.NewTopic("with-attachments", "With", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic target: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	message := models.NewMessage(thread.ID, "Message", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	attachment := models.NewAttachment(message.ID, "file.txt", "text/plain", []byte("data"))
	if err := store.CreateAttachment(attachment); err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}
	if _, err := store.GetAttachment(attachment.ID); err != nil {
		t.Fatalf("GetAttachment after empty topic: %v", err)
	}
	if err := store.DeleteAttachment(attachment.ID); err != nil {
		t.Fatalf("DeleteAttachment after empty topic: %v", err)
	}
}

func TestMarkdownAttachmentMetadataOwnership(t *testing.T) {
	for _, directoryKind := range []string{"full", "legacy"} {
		for _, operation := range []string{"get", "delete"} {
			t.Run(directoryKind+"/"+operation+" rejects mismatched message", func(t *testing.T) {
				assertMismatchedAttachmentOwnerRejected(t, directoryKind, operation)
			})
		}
	}
}

func assertMismatchedAttachmentOwnerRejected(t *testing.T, directoryKind string, operation string) {
	t.Helper()
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("ownership", "Ownership", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	owner := models.NewMessage(thread.ID, "Owner", "test@cli")
	other := models.NewMessage(thread.ID, "Other", "test@cli")
	for _, message := range []*models.Message{owner, other} {
		if err := store.CreateMessage(message); err != nil {
			t.Fatalf("CreateMessage: %v", err)
		}
	}
	attachment := models.NewAttachment(owner.ID, "owned.txt", "text/plain", []byte("owned"))
	dirName := owner.ID.String()
	if directoryKind == "legacy" {
		dirName = owner.ID.String()[:8]
	}
	dir := filepath.Join(store.dataDir, topic.Name, "_attachments", dirName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatalf("create attachment dir: %v", err)
	}
	dataPath := filepath.Join(dir, attachment.Filename)
	if err := os.WriteFile(dataPath, attachment.Data, 0640); err != nil {
		t.Fatalf("write data: %v", err)
	}
	metaPath := dataPath + ".meta.yaml"
	meta := attachmentMeta{ID: attachment.ID.String(), MessageID: other.ID.String(), Filename: attachment.Filename, MimeType: attachment.MimeType, CreatedAt: mdstore.FormatTime(attachment.CreatedAt.UTC())}
	if err := mdstore.WriteYAML(metaPath, &meta); err != nil {
		t.Fatalf("write meta: %v", err)
	}
	var err error
	if operation == "get" {
		_, err = store.GetAttachment(attachment.ID)
	} else {
		err = store.DeleteAttachment(attachment.ID)
	}
	if err == nil {
		t.Fatalf("%s accepted mismatched metadata ownership", operation)
	}
	assertFileBytes(t, dataPath, attachment.Data)
	if _, err := os.Stat(metaPath); err != nil {
		t.Errorf("metadata changed after rejected %s: %v", operation, err)
	}
}

func TestMarkdownFullAndLegacyAttachmentDuplicateIsAmbiguous(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("precedence", "Precedence", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	owner := models.NewMessage(thread.ID, "Owner", "test@cli")
	owner.ID = uuid.MustParse("feedface-0000-4000-8000-000000000001")
	other := models.NewMessage(thread.ID, "Other", "test@cli")
	other.ID = uuid.MustParse("feedface-0000-4000-8000-000000000002")
	for _, message := range []*models.Message{owner, other} {
		if err := store.CreateMessage(message); err != nil {
			t.Fatalf("CreateMessage: %v", err)
		}
	}
	attachment := models.NewAttachment(owner.ID, "full.txt", "text/plain", []byte("full"))
	if err := store.CreateAttachment(attachment); err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}
	legacyDir := filepath.Join(store.dataDir, topic.Name, "_attachments", "feedface")
	if err := os.MkdirAll(legacyDir, 0750); err != nil {
		t.Fatalf("create legacy dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "junk"), []byte("junk"), 0640); err != nil {
		t.Fatalf("write legacy junk: %v", err)
	}
	legacyMeta := attachmentMeta{ID: attachment.ID.String(), MessageID: owner.ID.String(), Filename: "junk", MimeType: attachment.MimeType, CreatedAt: mdstore.FormatTime(attachment.CreatedAt.UTC())}
	if err := mdstore.WriteYAML(filepath.Join(legacyDir, "junk.meta.yaml"), &legacyMeta); err != nil {
		t.Fatalf("write legacy meta: %v", err)
	}
	before := snapshotMarkdownTree(t, store)
	if _, err := store.GetAttachment(attachment.ID); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("GetAttachment duplicate full and legacy ID error = %v, want ambiguity", err)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

func TestMarkdownTopicRenameRejectsThreadSymlinkEscape(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("rename-source", "Rename", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	outsideBytes := []byte("outside thread bytes")
	if err := os.WriteFile(outside, outsideBytes, 0640); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(store.dataDir, topic.Name, "escape.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	topicsBefore, _ := os.ReadFile(store.topicsFilePath())
	topic.Name = "rename-target"
	if err := store.UpdateTopic(topic); err == nil {
		t.Fatal("UpdateTopic followed symlinked markdown file")
	}
	assertFileBytes(t, store.topicsFilePath(), topicsBefore)
	assertFileBytes(t, outside, outsideBytes)
	if _, err := os.Stat(filepath.Join(store.dataDir, "rename-source")); err != nil {
		t.Errorf("source topic changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.dataDir, "rename-target")); !os.IsNotExist(err) {
		t.Errorf("target topic created: %v", err)
	}
}

func TestMarkdownCreateAttachmentMetadataFailureRollsBack(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("attachment-rollback", "Rollback", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	_ = store.CreateThread(thread)
	message := models.NewMessage(thread.ID, "Message", "test@cli")
	_ = store.CreateMessage(message)
	attachment := models.NewAttachment(message.ID, "file.txt", "text/plain", []byte("replacement"))
	attachmentDir := filepath.Join(store.dataDir, topic.Name, "_attachments", message.ID.String())
	if err := os.MkdirAll(attachmentDir, 0750); err != nil {
		t.Fatalf("create attachment dir: %v", err)
	}
	dataPath := filepath.Join(attachmentDir, attachment.Filename)
	original := []byte("original")
	if err := os.WriteFile(dataPath, original, 0640); err != nil {
		t.Fatalf("write original: %v", err)
	}
	stored := attachment.ID.String() + "-" + attachment.Filename
	storedPath := filepath.Join(attachmentDir, stored)
	if err := os.WriteFile(storedPath, original, 0640); err != nil {
		t.Fatalf("write stored original: %v", err)
	}
	metaPath := storedPath + ".meta.yaml"
	if err := os.Mkdir(metaPath, 0750); err != nil {
		t.Fatalf("create blocking metadata directory: %v", err)
	}
	if err := store.CreateAttachment(attachment); err == nil {
		t.Fatal("CreateAttachment unexpectedly succeeded with metadata path directory")
	}
	assertFileBytes(t, dataPath, original)
	assertFileBytes(t, storedPath, original)
}

func TestMarkdownCreateTopicRegistryFailureRemovesDirectory(t *testing.T) {
	store := newTestMarkdownStore(t)
	if err := os.Mkdir(store.topicsFilePath(), 0750); err != nil {
		t.Fatalf("create blocking topics directory: %v", err)
	}
	topic := models.NewTopic("not-published", "Rollback", "test@cli")
	if err := store.CreateTopic(topic); err == nil {
		t.Fatal("CreateTopic unexpectedly succeeded with unwritable registry path")
	}
	if _, err := os.Stat(filepath.Join(store.dataDir, topic.Name)); !os.IsNotExist(err) {
		t.Errorf("failed CreateTopic left directory: %v", err)
	}
}

func newAttachmentRemovalFailureFixture(t *testing.T) (*MarkdownStore, *models.Thread, *models.Message, *models.Attachment, string) {
	t.Helper()
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("removal-failure", "Removal failure", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	message := models.NewMessage(thread.ID, "Message", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	attachment := models.NewAttachment(message.ID, "file.txt", "text/plain", []byte("attachment"))
	if err := store.CreateAttachment(attachment); err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}
	attachmentDir := filepath.Join(store.dataDir, topic.Name, "_attachments", message.ID.String())
	return store, thread, message, attachment, attachmentDir
}

func TestMarkdownDeleteAttachmentMetadataRemovalFailureLeavesDataUntouched(t *testing.T) {
	store, _, _, attachment, attachmentDir := newAttachmentRemovalFailureFixture(t)
	dataPath := filepath.Join(attachmentDir, attachment.Filename)
	if err := os.Chmod(attachmentDir, 0550); err != nil {
		t.Fatalf("make attachment directory read-only: %v", err)
	}
	defer func() { _ = os.Chmod(attachmentDir, 0750) }()

	err := store.DeleteAttachment(attachment.ID)
	if err == nil || !strings.Contains(err.Error(), "remove attachment metadata") {
		if err == nil {
			t.Skip("platform permits removal from a read-only directory")
		}
		t.Fatalf("delete with non-removable metadata error = %v", err)
	}
	assertFileBytes(t, dataPath, attachment.Data)
}

func TestMarkdownDeleteAttachmentDataRemovalFailureRestoresMetadataAndRemainsRetryable(t *testing.T) {
	store, _, _, attachment, attachmentDir := newAttachmentRemovalFailureFixture(t)
	dataPath := filepath.Join(attachmentDir, attachment.Filename)
	metaPath := dataPath + ".meta.yaml"
	if err := os.Chmod(metaPath, 0600); err != nil {
		t.Fatalf("set metadata fixture mode: %v", err)
	}
	metadataBefore, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata fixture: %v", err)
	}
	metadataInfoBefore, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("stat metadata fixture: %v", err)
	}
	if err := os.Remove(dataPath); err != nil {
		t.Fatalf("remove attachment data fixture: %v", err)
	}
	if err := os.Mkdir(dataPath, 0750); err != nil {
		t.Fatalf("create blocking attachment data directory: %v", err)
	}
	blockerPath := filepath.Join(dataPath, "keep")
	if err := os.WriteFile(blockerPath, []byte("occupied"), 0640); err != nil {
		t.Fatalf("occupy blocking attachment data directory: %v", err)
	}

	err = store.DeleteAttachment(attachment.ID)
	if err == nil || !strings.Contains(err.Error(), "remove attachment data") {
		t.Fatalf("delete with non-removable data error = %v", err)
	}
	metadataAfter, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read restored metadata: %v", err)
	}
	if !reflect.DeepEqual(metadataAfter, metadataBefore) {
		t.Fatalf("restored metadata bytes = %q, want %q", metadataAfter, metadataBefore)
	}
	metadataInfoAfter, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("stat restored metadata: %v", err)
	}
	if metadataInfoAfter.Mode() != metadataInfoBefore.Mode() {
		t.Fatalf("restored metadata mode = %v, want %v", metadataInfoAfter.Mode(), metadataInfoBefore.Mode())
	}

	if err := os.Remove(blockerPath); err != nil {
		t.Fatalf("remove data blocker before retry: %v", err)
	}
	if err := store.DeleteAttachment(attachment.ID); err != nil {
		t.Fatalf("retry DeleteAttachment: %v", err)
	}
	for _, path := range []string{dataPath, metaPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("successful retry left %s: %v", path, err)
		}
	}
	if err := store.DeleteAttachment(attachment.ID); err == nil || !strings.Contains(err.Error(), "attachment not found") {
		t.Fatalf("DeleteAttachment missing error = %v", err)
	}
}

func TestMarkdownDeleteAttachmentRemovalFailureIsAtomic(t *testing.T) {
	for _, operation := range []string{"message", "thread"} {
		t.Run(operation, func(t *testing.T) {
			store, thread, message, attachment, attachmentDir := newAttachmentRemovalFailureFixture(t)
			attachmentBase := filepath.Dir(attachmentDir)
			if err := os.Chmod(attachmentBase, 0550); err != nil {
				t.Fatalf("make attachment base read-only: %v", err)
			}
			defer func() { _ = os.Chmod(attachmentBase, 0750) }()
			var err error
			if operation == "message" {
				err = store.DeleteMessage(message.ID)
			} else {
				err = store.DeleteThread(thread.ID)
			}
			if err == nil {
				t.Fatalf("Delete %s unexpectedly succeeded with non-removable attachments", operation)
			}
			if _, err := store.GetMessage(message.ID); err != nil {
				t.Errorf("failed Delete %s removed message/thread: %v", operation, err)
			}
			got, err := store.GetAttachment(attachment.ID)
			if err != nil || string(got.Data) != "attachment" {
				t.Errorf("failed Delete %s changed attachment: %#v, %v", operation, got, err)
			}
		})
	}
}

func TestMarkdownCreateAttachmentRejectsMalformedV2Strictly(t *testing.T) {
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("strict-attachment", "Strict", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	thread := models.NewThread(topic.ID, "Thread", "test@cli")
	_ = store.CreateThread(thread)
	message := models.NewMessage(thread.ID, "Message", "test@cli")
	_ = store.CreateMessage(message)
	threadPath, _ := store.threadFilePath(topic.Name, thread.ID)
	before, _ := os.ReadFile(threadPath)
	corrupt := strings.Replace(string(before), fmt.Sprintf("<!-- content-bytes:%d -->", len(message.Content)), "<!-- content-bytes:999999 -->", 1)
	if err := os.WriteFile(threadPath, []byte(corrupt), 0640); err != nil {
		t.Fatalf("write malformed v2: %v", err)
	}
	attachment := models.NewAttachment(message.ID, "file.txt", "text/plain", []byte("data"))
	err := store.CreateAttachment(attachment)
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("CreateAttachment malformed v2 error = %v, want strict parse error", err)
	}
	if _, err := os.Stat(filepath.Join(store.dataDir, topic.Name, "_attachments")); !os.IsNotExist(err) {
		t.Errorf("rejected CreateAttachment changed filesystem: %v", err)
	}
}

type markdownTreeSnapshot map[string]string

func snapshotMarkdownTree(t *testing.T, store *MarkdownStore) markdownTreeSnapshot {
	t.Helper()
	snapshot := make(markdownTreeSnapshot)
	err := filepath.WalkDir(store.dataDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(store.dataDir, path)
		if err != nil {
			return err
		}
		if relative == ".lock" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			snapshot[relative] = fmt.Sprintf("directory:%o", info.Mode().Perm())
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			snapshot[relative] = "symlink:" + target
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[relative] = fmt.Sprintf("file:%o:%s", info.Mode().Perm(), data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot markdown tree: %v", err)
	}
	return snapshot
}

func assertMarkdownTreeUnchanged(t *testing.T, store *MarkdownStore, want markdownTreeSnapshot) {
	t.Helper()
	got := snapshotMarkdownTree(t, store)
	if len(got) != len(want) {
		t.Errorf("markdown tree path count = %d, want %d\ngot: %#v\nwant: %#v", len(got), len(want), got, want)
		return
	}
	for path, wantValue := range want {
		if gotValue, ok := got[path]; !ok || gotValue != wantValue {
			t.Errorf("markdown tree entry %q = %q (present %t), want %q", path, gotValue, ok, wantValue)
		}
	}
}

func TestMarkdownDuplicateCreateIsAtomic(t *testing.T) {
	t.Run("topic", func(t *testing.T) {
		store := newTestMarkdownStore(t)
		existing := models.NewTopic("existing", "Existing", "test@cli")
		if err := store.CreateTopic(existing); err != nil {
			t.Fatalf("CreateTopic seed: %v", err)
		}
		duplicate := models.NewTopic("duplicate", "Duplicate", "test@cli")
		duplicate.ID = existing.ID
		before := snapshotMarkdownTree(t, store)
		if err := store.CreateTopic(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("CreateTopic duplicate ID error = %v", err)
		}
		assertMarkdownTreeUnchanged(t, store, before)
	})

	t.Run("thread across topics", func(t *testing.T) {
		store := newTestMarkdownStore(t)
		firstTopic := models.NewTopic("first", "First", "test@cli")
		secondTopic := models.NewTopic("second", "Second", "test@cli")
		for _, topic := range []*models.Topic{firstTopic, secondTopic} {
			if err := store.CreateTopic(topic); err != nil {
				t.Fatalf("CreateTopic seed: %v", err)
			}
		}
		existing := models.NewThread(firstTopic.ID, "Existing", "test@cli")
		if err := store.CreateThread(existing); err != nil {
			t.Fatalf("CreateThread seed: %v", err)
		}
		duplicate := models.NewThread(secondTopic.ID, "Duplicate", "test@cli")
		duplicate.ID = existing.ID
		before := snapshotMarkdownTree(t, store)
		if err := store.CreateThread(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("CreateThread duplicate ID error = %v", err)
		}
		assertMarkdownTreeUnchanged(t, store, before)
	})

	t.Run("message across threads", func(t *testing.T) {
		store, _, threads := newDuplicateFixtureHierarchy(t)
		existing := models.NewMessage(threads[0].ID, "Existing", "test@cli")
		if err := store.CreateMessage(existing); err != nil {
			t.Fatalf("CreateMessage seed: %v", err)
		}
		duplicate := models.NewMessage(threads[1].ID, "Duplicate", "test@cli")
		duplicate.ID = existing.ID
		before := snapshotMarkdownTree(t, store)
		if err := store.CreateMessage(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("CreateMessage duplicate ID error = %v", err)
		}
		assertMarkdownTreeUnchanged(t, store, before)
	})

	t.Run("attachment across messages", func(t *testing.T) {
		store, _, threads := newDuplicateFixtureHierarchy(t)
		messages := []*models.Message{
			models.NewMessage(threads[0].ID, "First", "test@cli"),
			models.NewMessage(threads[1].ID, "Second", "test@cli"),
		}
		for _, message := range messages {
			if err := store.CreateMessage(message); err != nil {
				t.Fatalf("CreateMessage seed: %v", err)
			}
		}
		existing := models.NewAttachment(messages[0].ID, "existing.txt", "text/plain", []byte("existing"))
		if err := store.CreateAttachment(existing); err != nil {
			t.Fatalf("CreateAttachment seed: %v", err)
		}
		duplicate := models.NewAttachment(messages[1].ID, "duplicate.txt", "text/plain", []byte("duplicate"))
		duplicate.ID = existing.ID
		before := snapshotMarkdownTree(t, store)
		if err := store.CreateAttachment(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("CreateAttachment duplicate ID error = %v", err)
		}
		assertMarkdownTreeUnchanged(t, store, before)
	})
}

func newDuplicateFixtureHierarchy(t *testing.T) (*MarkdownStore, []*models.Topic, []*models.Thread) {
	t.Helper()
	store := newTestMarkdownStore(t)
	topics := []*models.Topic{
		models.NewTopic("first", "First", "test@cli"),
		models.NewTopic("second", "Second", "test@cli"),
	}
	threads := make([]*models.Thread, 0, len(topics))
	for i, topic := range topics {
		if err := store.CreateTopic(topic); err != nil {
			t.Fatalf("CreateTopic seed: %v", err)
		}
		thread := models.NewThread(topic.ID, fmt.Sprintf("Thread %d", i), "test@cli")
		if err := store.CreateThread(thread); err != nil {
			t.Fatalf("CreateThread seed: %v", err)
		}
		threads = append(threads, thread)
	}
	return store, topics, threads
}

func TestMarkdownSameUUIDAcrossEntityTypesIsAllowed(t *testing.T) {
	store := newTestMarkdownStore(t)
	sharedID := uuid.New()
	topic := models.NewTopic("shared", "Shared", "test@cli")
	topic.ID = sharedID
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic shared ID: %v", err)
	}
	thread := models.NewThread(topic.ID, "Shared", "test@cli")
	thread.ID = sharedID
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread cross-type shared ID: %v", err)
	}
	message := models.NewMessage(thread.ID, "Shared", "test@cli")
	message.ID = sharedID
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage cross-type shared ID: %v", err)
	}
	attachment := models.NewAttachment(message.ID, "shared.txt", "text/plain", []byte("shared"))
	attachment.ID = sharedID
	if err := store.CreateAttachment(attachment); err != nil {
		t.Fatalf("CreateAttachment cross-type shared ID: %v", err)
	}
}

func TestMarkdownPreexistingDuplicateTopicIDIsAmbiguousAndAtomic(t *testing.T) {
	operations := []struct {
		name string
		run  func(*MarkdownStore, *models.Topic) error
	}{
		{name: "get", run: func(store *MarkdownStore, topic *models.Topic) error { _, err := store.GetTopic(topic.ID); return err }},
		{name: "get by name", run: func(store *MarkdownStore, topic *models.Topic) error {
			_, err := store.GetTopicByName(topic.Name)
			return err
		}},
		{name: "resolve", run: func(store *MarkdownStore, topic *models.Topic) error {
			_, err := store.ResolveTopic(topic.ID.String())
			return err
		}},
		{name: "resolve by name", run: func(store *MarkdownStore, topic *models.Topic) error {
			_, err := store.ResolveTopic(topic.Name)
			return err
		}},
		{name: "update", run: func(store *MarkdownStore, topic *models.Topic) error {
			topic.Description = "changed"
			return store.UpdateTopic(topic)
		}},
		{name: "delete", run: func(store *MarkdownStore, topic *models.Topic) error { return store.DeleteTopic(topic.ID) }},
		{name: "archive", run: func(store *MarkdownStore, topic *models.Topic) error { return store.ArchiveTopic(topic.ID, true) }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			store := newTestMarkdownStore(t)
			first := models.NewTopic("first", "First", "test@cli")
			second := models.NewTopic("second", "Second", "test@cli")
			second.ID = first.ID
			if err := store.writeTopics([]topicEntry{fromTopicModel(first), fromTopicModel(second)}); err != nil {
				t.Fatalf("write duplicate topics: %v", err)
			}
			for _, topic := range []*models.Topic{first, second} {
				if err := os.Mkdir(store.topicDirPath(topic.Name), 0750); err != nil {
					t.Fatalf("create topic directory: %v", err)
				}
			}
			before := snapshotMarkdownTree(t, store)
			if err := operation.run(store, first); err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("%s duplicate topic ID error = %v, want ambiguity", operation.name, err)
			}
			assertMarkdownTreeUnchanged(t, store, before)
		})
	}
}

func duplicateThreadFixture(t *testing.T) (*MarkdownStore, *models.Thread) {
	t.Helper()
	store, topics, threads := newDuplicateFixtureHierarchy(t)
	secondPath, err := store.threadFilePath(topics[1].Name, threads[1].ID)
	if err != nil {
		t.Fatalf("find second thread: %v", err)
	}
	data, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second thread: %v", err)
	}
	duplicate := strings.Replace(string(data), threads[1].ID.String(), threads[0].ID.String(), 1)
	if err := os.WriteFile(secondPath, []byte(duplicate), 0640); err != nil {
		t.Fatalf("write duplicate thread ID: %v", err)
	}
	return store, threads[0]
}

func TestMarkdownPreexistingDuplicateThreadIDIsAmbiguousAndAtomic(t *testing.T) {
	operations := []struct {
		name string
		run  func(*MarkdownStore, *models.Thread) error
	}{
		{name: "get", run: func(store *MarkdownStore, thread *models.Thread) error {
			_, err := store.GetThread(thread.ID)
			return err
		}},
		{name: "resolve", run: func(store *MarkdownStore, thread *models.Thread) error {
			_, err := store.ResolveThread(thread.ID.String())
			return err
		}},
		{name: "update", run: func(store *MarkdownStore, thread *models.Thread) error {
			thread.Subject = "changed"
			return store.UpdateThread(thread)
		}},
		{name: "delete", run: func(store *MarkdownStore, thread *models.Thread) error { return store.DeleteThread(thread.ID) }},
		{name: "sticky", run: func(store *MarkdownStore, thread *models.Thread) error { return store.SetThreadSticky(thread.ID, true) }},
		{name: "list messages", run: func(store *MarkdownStore, thread *models.Thread) error {
			_, err := store.ListMessages(thread.ID)
			return err
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			store, thread := duplicateThreadFixture(t)
			before := snapshotMarkdownTree(t, store)
			if err := operation.run(store, thread); err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("%s duplicate thread ID error = %v, want ambiguity", operation.name, err)
			}
			assertMarkdownTreeUnchanged(t, store, before)
		})
	}
}

func duplicateMessageFixture(t *testing.T) (*MarkdownStore, *models.Message) {
	t.Helper()
	store, topics, threads := newDuplicateFixtureHierarchy(t)
	messages := []*models.Message{
		models.NewMessage(threads[0].ID, "First", "test@cli"),
		models.NewMessage(threads[1].ID, "Second", "test@cli"),
	}
	for _, message := range messages {
		if err := store.CreateMessage(message); err != nil {
			t.Fatalf("CreateMessage seed: %v", err)
		}
	}
	secondPath, err := store.threadFilePath(topics[1].Name, threads[1].ID)
	if err != nil {
		t.Fatalf("find second thread: %v", err)
	}
	data, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second thread: %v", err)
	}
	duplicate := strings.Replace(string(data), messages[1].ID.String(), messages[0].ID.String(), 1)
	if err := os.WriteFile(secondPath, []byte(duplicate), 0640); err != nil {
		t.Fatalf("write duplicate message ID: %v", err)
	}
	return store, messages[0]
}

func TestMarkdownPreexistingDuplicateMessageIDIsAmbiguousAndAtomic(t *testing.T) {
	operations := []struct {
		name string
		run  func(*MarkdownStore, *models.Message) error
	}{
		{name: "get", run: func(store *MarkdownStore, message *models.Message) error {
			_, err := store.GetMessage(message.ID)
			return err
		}},
		{name: "resolve", run: func(store *MarkdownStore, message *models.Message) error {
			_, err := store.ResolveMessage(message.ID.String())
			return err
		}},
		{name: "update", run: func(store *MarkdownStore, message *models.Message) error {
			message.Content = "changed"
			return store.UpdateMessage(message)
		}},
		{name: "delete", run: func(store *MarkdownStore, message *models.Message) error { return store.DeleteMessage(message.ID) }},
		{name: "list attachments", run: func(store *MarkdownStore, message *models.Message) error {
			_, err := store.ListAttachments(message.ID)
			return err
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			store, message := duplicateMessageFixture(t)
			before := snapshotMarkdownTree(t, store)
			if err := operation.run(store, message); err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("%s duplicate message ID error = %v, want ambiguity", operation.name, err)
			}
			assertMarkdownTreeUnchanged(t, store, before)
		})
	}
}

func duplicateAttachmentFixture(t *testing.T) (*MarkdownStore, *models.Attachment) {
	t.Helper()
	store, _, threads := newDuplicateFixtureHierarchy(t)
	messages := []*models.Message{
		models.NewMessage(threads[0].ID, "First", "test@cli"),
		models.NewMessage(threads[1].ID, "Second", "test@cli"),
	}
	attachments := make([]*models.Attachment, 0, len(messages))
	for i, message := range messages {
		if err := store.CreateMessage(message); err != nil {
			t.Fatalf("CreateMessage seed: %v", err)
		}
		attachment := models.NewAttachment(message.ID, fmt.Sprintf("file-%d.txt", i), "text/plain", []byte(fmt.Sprintf("data-%d", i)))
		if err := store.CreateAttachment(attachment); err != nil {
			t.Fatalf("CreateAttachment seed: %v", err)
		}
		attachments = append(attachments, attachment)
	}
	secondMetaPath := filepath.Join(store.dataDir, "second", "_attachments", messages[1].ID.String(), attachments[1].Filename+".meta.yaml")
	meta, err := readAttachmentMeta(secondMetaPath)
	if err != nil {
		t.Fatalf("read second attachment metadata: %v", err)
	}
	meta.ID = attachments[0].ID.String()
	if err := mdstore.WriteYAML(secondMetaPath, meta); err != nil {
		t.Fatalf("write duplicate attachment ID: %v", err)
	}
	return store, attachments[0]
}

func TestMarkdownPreexistingDuplicateAttachmentIDIsAmbiguousAndAtomic(t *testing.T) {
	operations := []struct {
		name string
		run  func(*MarkdownStore, *models.Attachment) error
	}{
		{name: "get", run: func(store *MarkdownStore, attachment *models.Attachment) error {
			_, err := store.GetAttachment(attachment.ID)
			return err
		}},
		{name: "delete", run: func(store *MarkdownStore, attachment *models.Attachment) error {
			return store.DeleteAttachment(attachment.ID)
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			store, attachment := duplicateAttachmentFixture(t)
			before := snapshotMarkdownTree(t, store)
			if err := operation.run(store, attachment); err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("%s duplicate attachment ID error = %v, want ambiguity", operation.name, err)
			}
			assertMarkdownTreeUnchanged(t, store, before)
		})
	}
}

func noncanonicalUUIDForms(id uuid.UUID) map[string]string {
	return map[string]string{
		"uppercase": strings.ToUpper(id.String()),
		"urn":       "urn:uuid:" + id.String(),
	}
}

func TestMarkdownTopicUUIDComparisonUsesSemanticIdentity(t *testing.T) {
	for formName, storedID := range noncanonicalUUIDForms(uuid.MustParse("12345678-1234-4234-8234-123456789abc")) {
		t.Run(formName+" duplicate create", func(t *testing.T) {
			assertTopicSemanticDuplicateCreate(t, storedID)
		})

		for _, operation := range []string{"get", "delete"} {
			t.Run(formName+" ambiguous "+operation, func(t *testing.T) {
				assertTopicSemanticAmbiguity(t, storedID, operation)
			})
		}
	}
}

func assertTopicSemanticDuplicateCreate(t *testing.T, storedID string) {
	t.Helper()
	store := newTestMarkdownStore(t)
	id := uuid.MustParse("12345678-1234-4234-8234-123456789abc")
	existing := models.NewTopic("existing", "Existing", "test@cli")
	existing.ID = id
	entry := fromTopicModel(existing)
	entry.ID = storedID
	if err := store.writeTopics([]topicEntry{entry}); err != nil {
		t.Fatalf("write noncanonical topic: %v", err)
	}
	if err := os.Mkdir(store.topicDirPath(existing.Name), 0750); err != nil {
		t.Fatalf("create topic directory: %v", err)
	}
	duplicate := models.NewTopic("duplicate", "Duplicate", "test@cli")
	duplicate.ID = id
	before := snapshotMarkdownTree(t, store)
	if err := store.CreateTopic(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("CreateTopic semantic duplicate error = %v", err)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

func assertTopicSemanticAmbiguity(t *testing.T, storedID string, operation string) {
	t.Helper()
	store := newTestMarkdownStore(t)
	id := uuid.MustParse("12345678-1234-4234-8234-123456789abc")
	first := models.NewTopic("first", "First", "test@cli")
	first.ID = id
	second := models.NewTopic("second", "Second", "test@cli")
	second.ID = id
	firstEntry := fromTopicModel(first)
	secondEntry := fromTopicModel(second)
	secondEntry.ID = storedID
	if err := store.writeTopics([]topicEntry{firstEntry, secondEntry}); err != nil {
		t.Fatalf("write semantic duplicate topics: %v", err)
	}
	for _, topic := range []*models.Topic{first, second} {
		if err := os.Mkdir(store.topicDirPath(topic.Name), 0750); err != nil {
			t.Fatalf("create topic directory: %v", err)
		}
	}
	before := snapshotMarkdownTree(t, store)
	var err error
	if operation == "get" {
		_, err = store.GetTopic(id)
	} else {
		err = store.DeleteTopic(id)
	}
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("%s semantic duplicate topic error = %v", operation, err)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

func rewriteStoredUUID(t *testing.T, path string, oldID uuid.UUID, storedID string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read UUID fixture %s: %v", path, err)
	}
	rewritten := strings.Replace(string(data), oldID.String(), storedID, 1)
	if rewritten == string(data) {
		t.Fatalf("UUID fixture %s did not contain %s", path, oldID)
	}
	if err := os.WriteFile(path, []byte(rewritten), 0640); err != nil {
		t.Fatalf("write UUID fixture %s: %v", path, err)
	}
}

func TestMarkdownThreadUUIDComparisonUsesSemanticIdentity(t *testing.T) {
	semanticID := uuid.MustParse("22345678-1234-4234-8234-123456789abc")
	for formName, storedID := range noncanonicalUUIDForms(semanticID) {
		t.Run(formName+" duplicate create", func(t *testing.T) {
			store, topics, threads := newDuplicateFixtureHierarchy(t)
			path, err := store.threadFilePath(topics[0].Name, threads[0].ID)
			if err != nil {
				t.Fatalf("find thread fixture: %v", err)
			}
			originalID := threads[0].ID
			rewriteStoredUUID(t, path, originalID, storedID)
			duplicate := models.NewThread(topics[1].ID, "Duplicate", "test@cli")
			duplicate.ID = semanticID
			before := snapshotMarkdownTree(t, store)
			if err := store.CreateThread(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("CreateThread semantic duplicate error = %v", err)
			}
			assertMarkdownTreeUnchanged(t, store, before)
		})

		t.Run(formName+" ambiguity", func(t *testing.T) {
			store, topics, threads := newDuplicateFixtureHierarchy(t)
			for i, thread := range threads {
				path, err := store.threadFilePath(topics[i].Name, thread.ID)
				if err != nil {
					t.Fatalf("find thread fixture: %v", err)
				}
				representation := semanticID.String()
				if i == 1 {
					representation = storedID
				}
				rewriteStoredUUID(t, path, thread.ID, representation)
			}
			before := snapshotMarkdownTree(t, store)
			if _, err := store.GetThread(semanticID); err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("GetThread semantic ambiguity error = %v", err)
			}
			if err := store.DeleteThread(semanticID); err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("DeleteThread semantic ambiguity error = %v", err)
			}
			assertMarkdownTreeUnchanged(t, store, before)
		})
	}
}

func TestMarkdownAttachmentUUIDComparisonUsesSemanticIdentity(t *testing.T) {
	semanticID := uuid.MustParse("32345678-1234-4234-8234-123456789abc")
	for formName, storedID := range noncanonicalUUIDForms(semanticID) {
		t.Run(formName+" duplicate create", func(t *testing.T) {
			assertAttachmentSemanticDuplicateCreate(t, semanticID, storedID)
		})

		t.Run(formName+" ambiguity", func(t *testing.T) {
			assertAttachmentSemanticAmbiguity(t, semanticID, storedID)
		})
	}
}

func assertAttachmentSemanticDuplicateCreate(t *testing.T, semanticID uuid.UUID, storedID string) {
	t.Helper()
	store, _, threads := newDuplicateFixtureHierarchy(t)
	message := models.NewMessage(threads[0].ID, "Message", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	attachment := models.NewAttachment(message.ID, "existing.txt", "text/plain", []byte("existing"))
	attachment.ID = semanticID
	if err := store.CreateAttachment(attachment); err != nil {
		t.Fatalf("CreateAttachment seed: %v", err)
	}
	metaPath := filepath.Join(store.dataDir, "first", "_attachments", message.ID.String(), attachment.Filename+".meta.yaml")
	meta, err := readAttachmentMeta(metaPath)
	if err != nil {
		t.Fatalf("read attachment metadata: %v", err)
	}
	meta.ID = storedID
	if err := mdstore.WriteYAML(metaPath, meta); err != nil {
		t.Fatalf("write noncanonical attachment ID: %v", err)
	}
	duplicate := models.NewAttachment(message.ID, "duplicate.txt", "text/plain", []byte("duplicate"))
	duplicate.ID = semanticID
	before := snapshotMarkdownTree(t, store)
	if err := store.CreateAttachment(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("CreateAttachment semantic duplicate error = %v", err)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

func assertAttachmentSemanticAmbiguity(t *testing.T, semanticID uuid.UUID, storedID string) {
	t.Helper()
	store, _, threads := newDuplicateFixtureHierarchy(t)
	messages := []*models.Message{models.NewMessage(threads[0].ID, "First", "test@cli"), models.NewMessage(threads[1].ID, "Second", "test@cli")}
	for i, message := range messages {
		if err := store.CreateMessage(message); err != nil {
			t.Fatalf("CreateMessage: %v", err)
		}
		attachment := models.NewAttachment(message.ID, fmt.Sprintf("file-%d.txt", i), "text/plain", []byte("data"))
		attachment.ID = uuid.New()
		if err := store.CreateAttachment(attachment); err != nil {
			t.Fatalf("CreateAttachment: %v", err)
		}
		metaPath := filepath.Join(store.dataDir, []string{"first", "second"}[i], "_attachments", message.ID.String(), attachment.Filename+".meta.yaml")
		meta, err := readAttachmentMeta(metaPath)
		if err != nil {
			t.Fatalf("read attachment metadata: %v", err)
		}
		meta.ID = semanticID.String()
		if i == 1 {
			meta.ID = storedID
		}
		if err := mdstore.WriteYAML(metaPath, meta); err != nil {
			t.Fatalf("write semantic attachment ID: %v", err)
		}
	}
	before := snapshotMarkdownTree(t, store)
	if _, err := store.GetAttachment(semanticID); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("GetAttachment semantic ambiguity error = %v", err)
	}
	if err := store.DeleteAttachment(semanticID); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("DeleteAttachment semantic ambiguity error = %v", err)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

func TestMarkdownCollisionScansFailClosedOnMalformedCandidates(t *testing.T) {
	t.Run("topic invalid UUID", assertMalformedTopicCollisionScan)
	t.Run("thread invalid frontmatter UUID", assertMalformedThreadCollisionScan)

	for _, operation := range []string{"create", "get", "delete"} {
		t.Run("message malformed "+operation, func(t *testing.T) {
			assertMalformedMessageCollisionScan(t, operation)
		})
		t.Run("attachment malformed "+operation, func(t *testing.T) {
			assertMalformedAttachmentCollisionScan(t, operation)
		})
	}
}

func assertMalformedTopicCollisionScan(t *testing.T) {
	t.Helper()
	store := newTestMarkdownStore(t)
	valid := models.NewTopic("valid", "Valid", "test@cli")
	invalid := fromTopicModel(models.NewTopic("invalid", "Invalid", "test@cli"))
	invalid.ID = "not-a-uuid"
	if err := store.writeTopics([]topicEntry{fromTopicModel(valid), invalid}); err != nil {
		t.Fatalf("write invalid topic registry: %v", err)
	}
	for _, name := range []string{"valid", "invalid"} {
		if err := os.Mkdir(store.topicDirPath(name), 0750); err != nil {
			t.Fatalf("create topic directory: %v", err)
		}
	}
	before := snapshotMarkdownTree(t, store)
	if err := store.CreateTopic(models.NewTopic("candidate", "Candidate", "test@cli")); err == nil || !strings.Contains(err.Error(), "not-a-uuid") {
		t.Fatalf("CreateTopic invalid registry UUID error = %v", err)
	}
	if _, err := store.GetTopic(valid.ID); err == nil || !strings.Contains(err.Error(), "not-a-uuid") {
		t.Fatalf("GetTopic invalid registry UUID error = %v", err)
	}
	if err := store.DeleteTopic(valid.ID); err == nil || !strings.Contains(err.Error(), "not-a-uuid") {
		t.Fatalf("DeleteTopic invalid registry UUID error = %v", err)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

func assertMalformedThreadCollisionScan(t *testing.T) {
	t.Helper()
	store, topics, threads := newDuplicateFixtureHierarchy(t)
	invalidPath, err := store.threadFilePath(topics[1].Name, threads[1].ID)
	if err != nil {
		t.Fatalf("find invalid thread fixture: %v", err)
	}
	rewriteStoredUUID(t, invalidPath, threads[1].ID, "not-a-uuid")
	before := snapshotMarkdownTree(t, store)
	if err := store.CreateThread(models.NewThread(topics[0].ID, "Candidate", "test@cli")); err == nil || !strings.Contains(err.Error(), filepath.Base(invalidPath)) {
		t.Fatalf("CreateThread invalid candidate error = %v", err)
	}
	if _, err := store.GetThread(threads[0].ID); err == nil || !strings.Contains(err.Error(), filepath.Base(invalidPath)) {
		t.Fatalf("GetThread invalid candidate error = %v", err)
	}
	if err := store.DeleteThread(threads[0].ID); err == nil || !strings.Contains(err.Error(), filepath.Base(invalidPath)) {
		t.Fatalf("DeleteThread invalid candidate error = %v", err)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

func assertMalformedMessageCollisionScan(t *testing.T, operation string) {
	t.Helper()
	fixture := newMalformedV2Fixture(t)
	before := snapshotMarkdownTree(t, fixture.store)
	var err error
	switch operation {
	case "create":
		duplicate := models.NewMessage(fixture.thread.ID, "Duplicate", "test@cli")
		duplicate.ID = fixture.messages[0].ID
		err = fixture.store.CreateMessage(duplicate)
	case "get":
		_, err = fixture.store.GetMessage(fixture.messages[0].ID)
	case "delete":
		err = fixture.store.DeleteMessage(fixture.messages[0].ID)
	}
	if err == nil || !strings.Contains(err.Error(), filepath.Base(fixture.threadPath)) {
		t.Fatalf("message %s malformed candidate error = %v", operation, err)
	}
	assertMarkdownTreeUnchanged(t, fixture.store, before)
}

func assertMalformedAttachmentCollisionScan(t *testing.T, operation string) {
	t.Helper()
	store, _, message, attachment, attachmentDir := newAttachmentRemovalFailureFixture(t)
	malformedPath := filepath.Join(attachmentDir, "broken.meta.yaml")
	if err := os.WriteFile(malformedPath, []byte("id: ["), 0640); err != nil {
		t.Fatalf("write malformed attachment metadata: %v", err)
	}
	before := snapshotMarkdownTree(t, store)
	var err error
	switch operation {
	case "create":
		err = store.CreateAttachment(models.NewAttachment(message.ID, "new.txt", "text/plain", []byte("new")))
	case "get":
		_, err = store.GetAttachment(attachment.ID)
	case "delete":
		err = store.DeleteAttachment(attachment.ID)
	}
	if err == nil || !strings.Contains(err.Error(), filepath.Base(malformedPath)) {
		t.Fatalf("attachment %s malformed metadata error = %v", operation, err)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

func requireUnreadableFile(t *testing.T, path string) func() {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat unreadable fixture: %v", err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Skipf("chmod cannot create unreadable fixture: %v", err)
	}
	restore := func() {
		if err := os.Chmod(path, info.Mode().Perm()); err != nil {
			t.Fatalf("restore unreadable fixture: %v", err)
		}
	}
	if _, err := os.ReadFile(path); err == nil {
		restore()
		t.Skip("platform permits reading mode-000 files")
	}
	return restore
}

func TestMarkdownCollisionScansFailClosedOnUnreadableCandidates(t *testing.T) {
	for _, operation := range []string{"create", "get", "delete"} {
		t.Run("thread "+operation, func(t *testing.T) {
			assertUnreadableThreadCollisionScan(t, operation)
		})

		t.Run("message "+operation, func(t *testing.T) {
			assertUnreadableMessageCollisionScan(t, operation)
		})

		t.Run("attachment metadata "+operation, func(t *testing.T) {
			assertUnreadableAttachmentCollisionScan(t, operation)
		})
	}
}

func assertUnreadableThreadCollisionScan(t *testing.T, operation string) {
	t.Helper()
	store, topics, threads := newDuplicateFixtureHierarchy(t)
	path, err := store.threadFilePath(topics[1].Name, threads[1].ID)
	if err != nil {
		t.Fatalf("find unreadable thread fixture: %v", err)
	}
	before := snapshotMarkdownTree(t, store)
	restore := requireUnreadableFile(t, path)
	var operationErr error
	switch operation {
	case "create":
		duplicate := models.NewThread(topics[0].ID, "Duplicate", "test@cli")
		duplicate.ID = threads[0].ID
		operationErr = store.CreateThread(duplicate)
	case "get":
		_, operationErr = store.GetThread(threads[0].ID)
	case "delete":
		operationErr = store.DeleteThread(threads[0].ID)
	}
	restore()
	if operationErr == nil || !strings.Contains(operationErr.Error(), filepath.Base(path)) {
		t.Fatalf("thread %s unreadable candidate error = %v", operation, operationErr)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

func assertUnreadableMessageCollisionScan(t *testing.T, operation string) {
	t.Helper()
	store, topics, threads := newDuplicateFixtureHierarchy(t)
	message := models.NewMessage(threads[0].ID, "Existing", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage seed: %v", err)
	}
	path, err := store.threadFilePath(topics[1].Name, threads[1].ID)
	if err != nil {
		t.Fatalf("find unreadable message candidate: %v", err)
	}
	before := snapshotMarkdownTree(t, store)
	restore := requireUnreadableFile(t, path)
	var operationErr error
	switch operation {
	case "create":
		duplicate := models.NewMessage(threads[0].ID, "Duplicate", "test@cli")
		duplicate.ID = message.ID
		operationErr = store.CreateMessage(duplicate)
	case "get":
		_, operationErr = store.GetMessage(message.ID)
	case "delete":
		operationErr = store.DeleteMessage(message.ID)
	}
	restore()
	if operationErr == nil || !strings.Contains(operationErr.Error(), filepath.Base(path)) {
		t.Fatalf("message %s unreadable candidate error = %v", operation, operationErr)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

func assertUnreadableAttachmentCollisionScan(t *testing.T, operation string) {
	t.Helper()
	store, _, message, attachment, attachmentDir := newAttachmentRemovalFailureFixture(t)
	metaPath := filepath.Join(attachmentDir, "file.txt.meta.yaml")
	before := snapshotMarkdownTree(t, store)
	restore := requireUnreadableFile(t, metaPath)
	var operationErr error
	switch operation {
	case "create":
		duplicate := models.NewAttachment(message.ID, "new.txt", "text/plain", []byte("new"))
		duplicate.ID = attachment.ID
		operationErr = store.CreateAttachment(duplicate)
	case "get":
		_, operationErr = store.GetAttachment(attachment.ID)
	case "delete":
		operationErr = store.DeleteAttachment(attachment.ID)
	}
	restore()
	if operationErr == nil || !strings.Contains(operationErr.Error(), filepath.Base(metaPath)) {
		t.Fatalf("attachment %s unreadable metadata error = %v", operation, operationErr)
	}
	assertMarkdownTreeUnchanged(t, store, before)
}

type malformedV2ThreadFixture struct {
	store      *MarkdownStore
	topic      *models.Topic
	thread     *models.Thread
	message    *models.Message
	threadPath string
}

func newMalformedV2ThreadFixture(t *testing.T, corrupt func(string) string) malformedV2ThreadFixture {
	t.Helper()
	store := newTestMarkdownStore(t)
	topic := models.NewTopic("malformed-v2", "Malformed v2 fixtures", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic fixture: %v", err)
	}
	thread := models.NewThread(topic.ID, "Malformed thread", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread fixture: %v", err)
	}
	message := models.NewMessage(thread.ID, "first message", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage fixture: %v", err)
	}
	second := models.NewMessage(thread.ID, "second message", "test@cli")
	if err := store.CreateMessage(second); err != nil {
		t.Fatalf("CreateMessage second fixture: %v", err)
	}
	threadPath, err := store.threadFilePath(topic.Name, thread.ID)
	if err != nil {
		t.Fatalf("threadFilePath fixture: %v", err)
	}
	data, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("read valid v2 fixture: %v", err)
	}
	corrupted := corrupt(string(data))
	if corrupted == string(data) {
		t.Fatal("fixture corruption did not change thread bytes")
	}
	if err := os.WriteFile(threadPath, []byte(corrupted), 0640); err != nil {
		t.Fatalf("write malformed v2 fixture: %v", err)
	}
	return malformedV2ThreadFixture{
		store: store, topic: topic, thread: thread, message: message, threadPath: threadPath,
	}
}

func malformedV2Corruptions() []struct {
	name    string
	want    string
	corrupt func(string) string
} {
	return []struct {
		name    string
		want    string
		corrupt func(string) string
	}{
		{
			name: "content bytes",
			want: "message content is shorter than declared byte count",
			corrupt: func(content string) string {
				start := strings.Index(content, "<!-- content-bytes:")
				end := strings.Index(content[start:], " -->")
				return content[:start] + "<!-- content-bytes:999999 -->" + content[start+end+len(" -->"):]
			},
		},
		{
			name: "message framing",
			want: "missing message separator",
			corrupt: func(content string) string {
				return strings.Replace(content, messageSeparator, "\n<!-- broken-separator -->\n", 1)
			},
		},
		{
			name: "v2 frontmatter",
			want: "thread updated_at is required",
			corrupt: func(content string) string {
				start := strings.Index(content, "updated_at:")
				end := strings.IndexByte(content[start:], '\n')
				return content[:start] + content[start+end+1:]
			},
		},
		{
			name: "unparseable v2 frontmatter",
			want: "parse frontmatter",
			corrupt: func(content string) string {
				return strings.Replace(content, "format_version: 2", "format_version: [2", 1)
			},
		},
	}
}

func TestParseThreadMessagesRejectsUnparseableFrontmatter(t *testing.T) {
	content := `---
format_version: [2
id: 5681e681-3603-4dbf-b289-08ae47819163
---
`

	messages, err := parseThreadMessages(content)
	if err == nil {
		t.Fatalf("parseThreadMessages returned legacy result %#v for unparseable frontmatter", messages)
	}
	if !strings.Contains(err.Error(), "parse thread frontmatter") || !strings.Contains(err.Error(), "yaml") {
		t.Fatalf("parseThreadMessages malformed frontmatter error = %v", err)
	}
}

func TestMarkdownMalformedV2ReadsFailClosed(t *testing.T) {
	for _, corruption := range malformedV2Corruptions() {
		t.Run(corruption.name, func(t *testing.T) {
			fixture := newMalformedV2ThreadFixture(t, corruption.corrupt)
			defer fixture.store.Close()
			operations := []struct {
				name string
				run  func() error
			}{
				{name: "list messages", run: func() error {
					_, err := fixture.store.ListMessages(fixture.thread.ID)
					return err
				}},
				{name: "get message", run: func() error {
					_, err := fixture.store.GetMessage(fixture.message.ID)
					return err
				}},
				{name: "resolve message", run: func() error {
					_, err := fixture.store.ResolveMessage(fixture.message.ID.String()[:8])
					return err
				}},
				{name: "get thread", run: func() error {
					_, err := fixture.store.GetThread(fixture.thread.ID)
					return err
				}},
				{name: "resolve thread", run: func() error {
					_, err := fixture.store.ResolveThread(fixture.thread.ID.String()[:8])
					return err
				}},
				{name: "list threads", run: func() error {
					_, err := fixture.store.ListThreads(fixture.topic.ID)
					return err
				}},
			}
			for _, operation := range operations {
				t.Run(operation.name, func(t *testing.T) {
					err := operation.run()
					if err == nil {
						t.Fatalf("%s returned success for malformed populated v2 thread", operation.name)
					}
					if !strings.Contains(err.Error(), filepath.Base(fixture.threadPath)) {
						t.Errorf("%s error %q does not identify path %q", operation.name, err, filepath.Base(fixture.threadPath))
					}
					if !strings.Contains(err.Error(), corruption.want) {
						t.Errorf("%s error %q does not preserve root cause %q", operation.name, err, corruption.want)
					}
				})
			}
		})
	}
}

func TestMigrateDataMalformedMarkdownReturnsError(t *testing.T) {
	fixture := newMalformedV2ThreadFixture(t, malformedV2Corruptions()[0].corrupt)
	defer fixture.store.Close()
	destination, err := NewSqliteStore(filepath.Join(t.TempDir(), "destination.db"))
	if err != nil {
		t.Fatalf("NewSqliteStore destination: %v", err)
	}
	defer destination.Close()

	if _, err := MigrateData(fixture.store, destination); err == nil ||
		!strings.Contains(err.Error(), "message content is shorter than declared byte count") {
		t.Fatalf("MigrateData malformed Markdown error = %v", err)
	}
}

func TestMarkdownResolveThreadPrefixFailsClosedOnMalformedCandidates(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "missing ID",
			content: `---
topic: resolve-corrupt
subject: Missing ID
created_at: "2026-01-01T00:00:00Z"
created_by: test@cli
---
`,
			want: "parse thread ID",
		},
		{
			name: "invalid ID",
			content: `---
id: definitely-not-a-uuid
topic: resolve-corrupt
subject: Invalid ID
created_at: "2026-01-01T00:00:00Z"
created_by: test@cli
---
`,
			want: "invalid UUID",
		},
		{
			name: "malformed YAML",
			content: `---
id: [unterminated
topic: resolve-corrupt
---
`,
			want: "parse frontmatter",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newTestMarkdownStore(t)
			defer store.Close()
			topic := models.NewTopic("resolve-corrupt", "Resolve corruption", "test@cli")
			if err := store.CreateTopic(topic); err != nil {
				t.Fatalf("CreateTopic fixture: %v", err)
			}
			candidatePath := filepath.Join(store.topicDirPath(topic.Name), "malformed-candidate.md")
			if err := os.WriteFile(candidatePath, []byte(test.content), 0640); err != nil {
				t.Fatalf("write malformed candidate: %v", err)
			}

			_, err := store.ResolveThread("deadbeef")
			if err == nil {
				t.Fatal("ResolveThread returned success for malformed candidate")
			}
			if !strings.Contains(err.Error(), filepath.Base(candidatePath)) {
				t.Errorf("ResolveThread error %q does not identify path %q", err, filepath.Base(candidatePath))
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("ResolveThread error %q does not preserve root cause %q", err, test.want)
			}
		})
	}
}

func TestMarkdownResolveThreadPrefixRejectsPartialMatchBeforeMalformedCandidate(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	topic := models.NewTopic("resolve-partial", "Resolve partial match", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic fixture: %v", err)
	}
	thread := models.NewThread(topic.ID, "Valid match", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread fixture: %v", err)
	}
	message := models.NewMessage(thread.ID, "valid message", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage fixture: %v", err)
	}
	validPath, err := store.threadFilePath(topic.Name, thread.ID)
	if err != nil {
		t.Fatalf("threadFilePath fixture: %v", err)
	}
	data, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatalf("read valid matching thread: %v", err)
	}
	corruptPath := filepath.Join(store.topicDirPath(topic.Name), "zz-malformed-matching-candidate.md")
	corrupt := malformedV2Corruptions()[0].corrupt(string(data))
	if err := os.WriteFile(corruptPath, []byte(corrupt), 0640); err != nil {
		t.Fatalf("write malformed matching candidate: %v", err)
	}

	got, err := store.ResolveThread(thread.ID.String()[:8])
	if err == nil {
		t.Fatalf("ResolveThread returned partial match %#v instead of malformed candidate error", got)
	}
	if !strings.Contains(err.Error(), filepath.Base(corruptPath)) {
		t.Errorf("ResolveThread error %q does not identify path %q", err, filepath.Base(corruptPath))
	}
	if !strings.Contains(err.Error(), "message content is shorter than declared byte count") {
		t.Errorf("ResolveThread error %q does not preserve malformed candidate root cause", err)
	}
}

func TestMarkdownResolveThreadPrefixFailsOnMissingRegisteredTopicDirectory(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	topic := models.NewTopic("resolve-missing-dir", "Missing registered directory", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic fixture: %v", err)
	}
	topicDir := store.topicDirPath(topic.Name)
	if err := os.Remove(topicDir); err != nil {
		t.Fatalf("remove registered topic directory: %v", err)
	}

	_, err := store.ResolveThread("deadbeef")
	if err == nil {
		t.Fatal("ResolveThread returned not-found success path for missing registered topic directory")
	}
	if !strings.Contains(err.Error(), topic.Name) || !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("ResolveThread missing registered directory error = %v", err)
	}
}

func TestMarkdownResolveMessagePrefixRejectsPartialMatchBeforeMalformedCrossTopicCandidate(t *testing.T) {
	store, topics, threads := newDuplicateFixtureHierarchy(t)
	defer store.Close()
	matchingMessage := models.NewMessage(threads[0].ID, "matching message", "test@cli")
	if err := store.CreateMessage(matchingMessage); err != nil {
		t.Fatalf("CreateMessage matching fixture: %v", err)
	}
	corruptPath, err := store.threadFilePath(topics[1].Name, threads[1].ID)
	if err != nil {
		t.Fatalf("threadFilePath corrupt fixture: %v", err)
	}
	data, err := os.ReadFile(corruptPath)
	if err != nil {
		t.Fatalf("read corrupt fixture source: %v", err)
	}
	corrupt := strings.Replace(string(data), "format_version: 2", "format_version: [2", 1)
	if err := os.WriteFile(corruptPath, []byte(corrupt), 0640); err != nil {
		t.Fatalf("write malformed v2 frontmatter: %v", err)
	}

	got, err := store.ResolveMessage(matchingMessage.ID.String()[:8])
	if err == nil {
		t.Fatalf("ResolveMessage returned partial match %#v instead of malformed candidate error", got)
	}
	if !strings.Contains(err.Error(), corruptPath) {
		t.Errorf("ResolveMessage error %q does not identify path %q", err, corruptPath)
	}
	if !strings.Contains(err.Error(), "parse thread frontmatter") {
		t.Errorf("ResolveMessage error %q does not preserve malformed YAML root cause", err)
	}
}

func TestMarkdownResolveMessagePrefixFailsOnMissingRegisteredTopicDirectory(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	topic := models.NewTopic("resolve-message-missing-dir", "Missing registered directory", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic fixture: %v", err)
	}
	topicDir := store.topicDirPath(topic.Name)
	if err := os.Remove(topicDir); err != nil {
		t.Fatalf("remove registered topic directory: %v", err)
	}

	_, err := store.ResolveMessage("deadbeef")
	if err == nil {
		t.Fatal("ResolveMessage returned not-found path for missing registered topic directory")
	}
	if !strings.Contains(err.Error(), topicDir) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ResolveMessage missing registered directory error = %v", err)
	}
}

func TestMarkdownResolveMessagePrefixFailsOnUnreadableRegisteredTopicDirectory(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	topic := models.NewTopic("resolve-message-unreadable-dir", "Unreadable registered directory", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic fixture: %v", err)
	}
	topicDir := store.topicDirPath(topic.Name)
	if err := os.Chmod(topicDir, 0000); err != nil {
		t.Skipf("chmod cannot create unreadable directory fixture: %v", err)
	}
	defer func() {
		if err := os.Chmod(topicDir, 0750); err != nil {
			t.Fatalf("restore registered topic directory permissions: %v", err)
		}
	}()
	if _, err := os.ReadDir(topicDir); err == nil {
		t.Skip("platform permits reading mode-000 directories")
	}

	_, err := store.ResolveMessage("deadbeef")
	if err == nil {
		t.Fatal("ResolveMessage returned not-found path for unreadable registered topic directory")
	}
	if !strings.Contains(err.Error(), topicDir) || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("ResolveMessage unreadable registered directory error = %v", err)
	}
}

func TestMarkdownResolveMessagePrefixRejectsSymlinkedCandidateOutsideBoardRoot(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	topic := models.NewTopic("resolve-message-contained", "Contained resolution", "test@cli")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("CreateTopic fixture: %v", err)
	}
	thread := models.NewThread(topic.ID, "Valid match", "test@cli")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread fixture: %v", err)
	}
	message := models.NewMessage(thread.ID, "valid message", "test@cli")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("CreateMessage fixture: %v", err)
	}
	validPath, err := store.threadFilePath(topic.Name, thread.ID)
	if err != nil {
		t.Fatalf("threadFilePath fixture: %v", err)
	}
	data, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatalf("read valid fixture: %v", err)
	}
	outsidePath := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outsidePath, data, 0640); err != nil {
		t.Fatalf("write outside thread candidate: %v", err)
	}
	linkedPath := filepath.Join(store.topicDirPath(topic.Name), "zz-outside.md")
	if err := os.Symlink(outsidePath, linkedPath); err != nil {
		t.Skipf("symlinks unavailable for containment fixture: %v", err)
	}

	got, err := store.ResolveMessage(message.ID.String()[:8])
	if err == nil {
		t.Fatalf("ResolveMessage returned partial match %#v instead of rejecting outside candidate", got)
	}
	if !strings.Contains(err.Error(), outsidePath) || !strings.Contains(err.Error(), "outside board root") {
		t.Fatalf("ResolveMessage outside candidate error = %v", err)
	}
}

func TestMarkdownExplicitUnsupportedThreadVersionsFailClosed(t *testing.T) {
	for _, version := range []int{-1, 0, 1, 3} {
		t.Run(fmt.Sprintf("version %d", version), func(t *testing.T) {
			fixture := newMalformedV2ThreadFixture(t, func(content string) string {
				return strings.Replace(content, "format_version: 2", fmt.Sprintf("format_version: %d", version), 1)
			})
			defer fixture.store.Close()
			want := fmt.Sprintf("unsupported thread format version: %d", version)
			operations := []struct {
				name string
				run  func() error
			}{
				{name: "list messages", run: func() error {
					_, err := fixture.store.ListMessages(fixture.thread.ID)
					return err
				}},
				{name: "read thread", run: func() error {
					_, err := fixture.store.GetThread(fixture.thread.ID)
					return err
				}},
				{name: "resolve message", run: func() error {
					_, err := fixture.store.ResolveMessage(fixture.message.ID.String()[:8])
					return err
				}},
			}
			for _, operation := range operations {
				t.Run(operation.name, func(t *testing.T) {
					err := operation.run()
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Fatalf("%s explicit version error = %v, want %q", operation.name, err, want)
					}
					if !strings.Contains(err.Error(), fixture.threadPath) {
						t.Fatalf("%s explicit version error %q lacks path %q", operation.name, err, fixture.threadPath)
					}
				})
			}

			before := snapshotMarkdownTree(t, fixture.store)
			fixture.message.Content = "must not be written"
			err := fixture.store.UpdateMessage(fixture.message)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("UpdateMessage explicit version error = %v, want %q", err, want)
			}
			assertMarkdownTreeUnchanged(t, fixture.store, before)
		})
	}
}

func TestMarkdownResolveMessagePrefixValidatesTopicRegistryBeforeReturningMatch(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func([]topicEntry)
		want    string
	}{
		{
			name: "invalid ID",
			corrupt: func(entries []topicEntry) {
				entries[1].ID = "not-a-topic-uuid"
			},
			want: "not-a-topic-uuid",
		},
		{
			name: "invalid created_at",
			corrupt: func(entries []topicEntry) {
				entries[1].CreatedAt = "not-a-topic-timestamp"
			},
			want: "not-a-topic-timestamp",
		},
		{
			name: "duplicate ID cardinality",
			corrupt: func(entries []topicEntry) {
				entries[1].ID = entries[0].ID
			},
			want: "ambiguous topic ID",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, _, threads := newDuplicateFixtureHierarchy(t)
			defer store.Close()
			message := models.NewMessage(threads[0].ID, "valid matching message", "test@cli")
			if err := store.CreateMessage(message); err != nil {
				t.Fatalf("CreateMessage matching fixture: %v", err)
			}
			entries, err := store.readTopics()
			if err != nil {
				t.Fatalf("read topic registry fixture: %v", err)
			}
			test.corrupt(entries)
			if err := store.writeTopics(entries); err != nil {
				t.Fatalf("write malformed topic registry fixture: %v", err)
			}

			got, err := store.ResolveMessage(message.ID.String()[:8])
			if err == nil {
				t.Fatalf("ResolveMessage returned partial match %#v for malformed topic registry", got)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ResolveMessage malformed topic registry error = %v, want %q", err, test.want)
			}
		})
	}
}
