// ABOUTME: Tests for storage migration between backends
// ABOUTME: Covers sqlite-to-markdown, markdown-to-sqlite, data integrity, and error cases

package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/harper/bbs/internal/models"
)

// seedTestData populates a storage backend with a representative data set
// and returns the counts and IDs for verification.
func seedTestData(t *testing.T, src Storage) (topics []*models.Topic, threads []*models.Thread, messages []*models.Message, attachments []*models.Attachment) {
	t.Helper()

	// Create two topics, one archived
	topic1 := models.NewTopic("general", "General discussion", "admin@cli")
	topic2 := models.NewTopic("dev", "Development talk", "admin@cli")
	topic2.Archived = true
	mustNoErr(t, src.CreateTopic(topic1))
	mustNoErr(t, src.CreateTopic(topic2))
	topics = append(topics, topic1, topic2)

	// Create threads in topic1
	thread1 := models.NewThread(topic1.ID, "Welcome Thread", "admin@cli")
	thread1.Sticky = true
	thread2 := models.NewThread(topic1.ID, "Random Chat", "user@cli")
	mustNoErr(t, src.CreateThread(thread1))
	mustNoErr(t, src.CreateThread(thread2))

	// Create a thread in topic2
	thread3 := models.NewThread(topic2.ID, "Build Failures", "ci@mcp")
	mustNoErr(t, src.CreateThread(thread3))
	threads = append(threads, thread1, thread2, thread3)

	// Create messages
	msg1 := models.NewMessage(thread1.ID, "Welcome to the BBS!", "admin@cli")
	time.Sleep(time.Millisecond)
	msg2 := models.NewMessage(thread1.ID, "Thanks for the welcome.", "user@cli")
	now := time.Now()
	msg2.EditedAt = &now
	msg3 := models.NewMessage(thread2.ID, "Anyone here?", "user@cli")
	msg4 := models.NewMessage(thread3.ID, "Build is red on main.", "ci@mcp")
	mustNoErr(t, src.CreateMessage(msg1))
	mustNoErr(t, src.CreateMessage(msg2))
	mustNoErr(t, src.CreateMessage(msg3))
	mustNoErr(t, src.CreateMessage(msg4))
	messages = append(messages, msg1, msg2, msg3, msg4)

	// Create attachments
	att1 := models.NewAttachment(msg4.ID, "build.log", "text/plain", []byte("ERROR: test failure on line 42"))
	att2 := models.NewAttachment(msg1.ID, "rules.txt", "text/plain", []byte("Be kind. Have fun."))
	mustNoErr(t, src.CreateAttachment(att1))
	mustNoErr(t, src.CreateAttachment(att2))
	attachments = append(attachments, att1, att2)

	return
}

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// verifyMigratedData checks that the destination storage contains all expected data.
func verifyMigratedData(t *testing.T, dst Storage, topics []*models.Topic, threads []*models.Thread, messages []*models.Message, attachments []*models.Attachment) {
	t.Helper()
	verifyMigratedTopics(t, dst, topics)
	verifyMigratedThreads(t, dst, threads)
	verifyMigratedMessages(t, dst, messages)
	verifyMigratedAttachments(t, dst, attachments)
}

func verifyMigratedTopics(t *testing.T, dst Storage, topics []*models.Topic) {
	t.Helper()
	for _, orig := range topics {
		got, err := dst.GetTopic(orig.ID)
		if err != nil {
			t.Errorf("topic %s (%s) not found in destination: %v", orig.Name, orig.ID, err)
			continue
		}
		if got.Name != orig.Name {
			t.Errorf("topic name mismatch: want %q, got %q", orig.Name, got.Name)
		}
		if got.Description != orig.Description {
			t.Errorf("topic description mismatch: want %q, got %q", orig.Description, got.Description)
		}
		if got.CreatedBy != orig.CreatedBy {
			t.Errorf("topic createdBy mismatch: want %q, got %q", orig.CreatedBy, got.CreatedBy)
		}
		if got.Archived != orig.Archived {
			t.Errorf("topic archived mismatch: want %v, got %v", orig.Archived, got.Archived)
		}
	}
}

func verifyMigratedThreads(t *testing.T, dst Storage, threads []*models.Thread) {
	t.Helper()
	for _, orig := range threads {
		got, err := dst.GetThread(orig.ID)
		if err != nil {
			t.Errorf("thread %s (%s) not found in destination: %v", orig.Subject, orig.ID, err)
			continue
		}
		if got.Subject != orig.Subject {
			t.Errorf("thread subject mismatch: want %q, got %q", orig.Subject, got.Subject)
		}
		if got.TopicID != orig.TopicID {
			t.Errorf("thread topicID mismatch: want %s, got %s", orig.TopicID, got.TopicID)
		}
		if got.CreatedBy != orig.CreatedBy {
			t.Errorf("thread createdBy mismatch: want %q, got %q", orig.CreatedBy, got.CreatedBy)
		}
		if got.Sticky != orig.Sticky {
			t.Errorf("thread sticky mismatch: want %v, got %v", orig.Sticky, got.Sticky)
		}
	}
}

func verifyMigratedMessages(t *testing.T, dst Storage, messages []*models.Message) {
	t.Helper()
	for _, orig := range messages {
		got, err := dst.GetMessage(orig.ID)
		if err != nil {
			t.Errorf("message %s not found in destination: %v", orig.ID, err)
			continue
		}
		if got.Content != orig.Content {
			t.Errorf("message content mismatch: want %q, got %q", orig.Content, got.Content)
		}
		if got.ThreadID != orig.ThreadID {
			t.Errorf("message threadID mismatch: want %s, got %s", orig.ThreadID, got.ThreadID)
		}
		if got.CreatedBy != orig.CreatedBy {
			t.Errorf("message createdBy mismatch: want %q, got %q", orig.CreatedBy, got.CreatedBy)
		}
		if (orig.EditedAt == nil) != (got.EditedAt == nil) {
			t.Errorf("message editedAt nil mismatch: orig=%v, got=%v", orig.EditedAt, got.EditedAt)
		}
	}
}

func verifyMigratedAttachments(t *testing.T, dst Storage, attachments []*models.Attachment) {
	t.Helper()
	for _, orig := range attachments {
		got, err := dst.GetAttachment(orig.ID)
		if err != nil {
			t.Errorf("attachment %s (%s) not found in destination: %v", orig.Filename, orig.ID, err)
			continue
		}
		if got.Filename != orig.Filename {
			t.Errorf("attachment filename mismatch: want %q, got %q", orig.Filename, got.Filename)
		}
		if got.MimeType != orig.MimeType {
			t.Errorf("attachment mimetype mismatch: want %q, got %q", orig.MimeType, got.MimeType)
		}
		if string(got.Data) != string(orig.Data) {
			t.Errorf("attachment data mismatch: want %q, got %q", string(orig.Data), string(got.Data))
		}
		if got.MessageID != orig.MessageID {
			t.Errorf("attachment messageID mismatch: want %s, got %s", orig.MessageID, got.MessageID)
		}
	}
}

func TestMigrateData_SqliteToMarkdown(t *testing.T) {
	// Set up source (sqlite)
	srcDir := t.TempDir()
	src, err := NewSqliteStore(filepath.Join(srcDir, "bbs.db"))
	if err != nil {
		t.Fatalf("create source store: %v", err)
	}
	defer src.Close()

	topics, threads, messages, attachments := seedTestData(t, src)

	// Set up destination (markdown)
	dstDir := t.TempDir()
	dst, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("create destination store: %v", err)
	}
	defer dst.Close()

	// Run migration
	summary, err := MigrateData(src, dst)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	// Verify summary counts
	if summary.Topics != len(topics) {
		t.Errorf("summary topics: want %d, got %d", len(topics), summary.Topics)
	}
	if summary.Threads != len(threads) {
		t.Errorf("summary threads: want %d, got %d", len(threads), summary.Threads)
	}
	if summary.Messages != len(messages) {
		t.Errorf("summary messages: want %d, got %d", len(messages), summary.Messages)
	}
	if summary.Attachments != len(attachments) {
		t.Errorf("summary attachments: want %d, got %d", len(attachments), summary.Attachments)
	}

	// Verify all data was migrated correctly
	verifyMigratedData(t, dst, topics, threads, messages, attachments)
}

func TestMigrateData_MarkdownToSqlite(t *testing.T) {
	// Set up source (markdown)
	srcDir := t.TempDir()
	src, err := NewMarkdownStore(srcDir)
	if err != nil {
		t.Fatalf("create source store: %v", err)
	}
	defer src.Close()

	topics, threads, messages, attachments := seedTestData(t, src)

	// Set up destination (sqlite)
	dstDir := t.TempDir()
	dst, err := NewSqliteStore(filepath.Join(dstDir, "bbs.db"))
	if err != nil {
		t.Fatalf("create destination store: %v", err)
	}
	defer dst.Close()

	// Run migration
	summary, err := MigrateData(src, dst)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	// Verify summary counts
	if summary.Topics != len(topics) {
		t.Errorf("summary topics: want %d, got %d", len(topics), summary.Topics)
	}
	if summary.Threads != len(threads) {
		t.Errorf("summary threads: want %d, got %d", len(threads), summary.Threads)
	}
	if summary.Messages != len(messages) {
		t.Errorf("summary messages: want %d, got %d", len(messages), summary.Messages)
	}
	if summary.Attachments != len(attachments) {
		t.Errorf("summary attachments: want %d, got %d", len(attachments), summary.Attachments)
	}

	// Verify all data was migrated correctly
	verifyMigratedData(t, dst, topics, threads, messages, attachments)
}

func TestMigrateData_EmptySource(t *testing.T) {
	// Set up empty source (sqlite)
	srcDir := t.TempDir()
	src, err := NewSqliteStore(filepath.Join(srcDir, "bbs.db"))
	if err != nil {
		t.Fatalf("create source store: %v", err)
	}
	defer src.Close()

	// Set up destination (markdown)
	dstDir := t.TempDir()
	dst, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("create destination store: %v", err)
	}
	defer dst.Close()

	summary, err := MigrateData(src, dst)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	if summary.Topics != 0 || summary.Threads != 0 || summary.Messages != 0 || summary.Attachments != 0 {
		t.Errorf("expected all zero counts for empty source, got topics=%d threads=%d messages=%d attachments=%d",
			summary.Topics, summary.Threads, summary.Messages, summary.Attachments)
	}
}

func TestIsDirNonEmpty(t *testing.T) {
	// Empty directory
	emptyDir := t.TempDir()
	nonEmpty, err := IsDirNonEmpty(emptyDir)
	if err != nil {
		t.Fatalf("IsDirNonEmpty on empty dir: %v", err)
	}
	if nonEmpty {
		t.Error("expected empty dir to be reported as empty")
	}

	// Non-empty directory
	nonEmptyDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonEmptyDir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}
	nonEmpty, err = IsDirNonEmpty(nonEmptyDir)
	if err != nil {
		t.Fatalf("IsDirNonEmpty on non-empty dir: %v", err)
	}
	if !nonEmpty {
		t.Error("expected non-empty dir to be reported as non-empty")
	}

	// Non-existent directory
	nonEmpty, err = IsDirNonEmpty(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("IsDirNonEmpty on non-existent dir: %v", err)
	}
	if nonEmpty {
		t.Error("expected non-existent dir to be reported as empty")
	}
}

func TestMigrateData_SqliteToSqlite(t *testing.T) {
	// Test migrating between two sqlite instances (roundtrip sanity check)
	srcDir := t.TempDir()
	src, err := NewSqliteStore(filepath.Join(srcDir, "bbs.db"))
	if err != nil {
		t.Fatalf("create source store: %v", err)
	}
	defer src.Close()

	topics, threads, messages, attachments := seedTestData(t, src)

	dstDir := t.TempDir()
	dst, err := NewSqliteStore(filepath.Join(dstDir, "bbs.db"))
	if err != nil {
		t.Fatalf("create destination store: %v", err)
	}
	defer dst.Close()

	summary, err := MigrateData(src, dst)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	if summary.Topics != len(topics) {
		t.Errorf("summary topics: want %d, got %d", len(topics), summary.Topics)
	}

	verifyMigratedData(t, dst, topics, threads, messages, attachments)
}
