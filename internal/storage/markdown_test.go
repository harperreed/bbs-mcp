// ABOUTME: Tests for MarkdownStore file-based storage backend
// ABOUTME: Covers CRUD for topics, threads, messages, attachments, resolution, and edge cases

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
		{"", "thread"},
		{"ALLCAPS", "allcaps"},
		{"with-existing-hyphens", "with-existing-hyphens"},
		{"123-numbers", "123-numbers"},
	}

	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.expected {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.expected)
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

---

## claude_opus@mcp — 2026-02-02T02:42:30Z
<!-- msg:762ee7bf-1234-5678-abcd-ef0123456789 -->

Reply test - confirming message posting works correctly.
`
	messages := parseThreadMessages(content)
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
	messages := parseThreadMessages(content)
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
