// ABOUTME: Tests for storage migration between backends
// ABOUTME: Covers sqlite-to-markdown, markdown-to-sqlite, data integrity, and error cases

package storage

import (
	"os"
	"path/filepath"
	"strings"
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

// --- Round-trip Tests ---

func TestMigrateRoundTrip_SqliteToMarkdownToSqlite(t *testing.T) {
	// Phase 1: Create rich data in SQLite
	srcDir := t.TempDir()
	original, err := NewSqliteStore(filepath.Join(srcDir, "original.db"))
	if err != nil {
		t.Fatalf("create original store: %v", err)
	}
	defer original.Close()

	topics, threads, messages, attachments := seedRichTestData(t, original)

	// Phase 2: Migrate SQLite -> Markdown
	mdDir := t.TempDir()
	mdStore, err := NewMarkdownStore(mdDir)
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	defer mdStore.Close()

	summary1, err := MigrateData(original, mdStore)
	if err != nil {
		t.Fatalf("MigrateData (sqlite->markdown) failed: %v", err)
	}
	if summary1.Topics != len(topics) || summary1.Threads != len(threads) ||
		summary1.Messages != len(messages) || summary1.Attachments != len(attachments) {
		t.Errorf("phase 1 summary mismatch: topics=%d/%d threads=%d/%d messages=%d/%d attachments=%d/%d",
			summary1.Topics, len(topics), summary1.Threads, len(threads),
			summary1.Messages, len(messages), summary1.Attachments, len(attachments))
	}

	// Phase 3: Migrate Markdown -> new SQLite
	dstDir := t.TempDir()
	final, err := NewSqliteStore(filepath.Join(dstDir, "final.db"))
	if err != nil {
		t.Fatalf("create final store: %v", err)
	}
	defer final.Close()

	summary2, err := MigrateData(mdStore, final)
	if err != nil {
		t.Fatalf("MigrateData (markdown->sqlite) failed: %v", err)
	}
	if summary2.Topics != len(topics) || summary2.Threads != len(threads) ||
		summary2.Messages != len(messages) || summary2.Attachments != len(attachments) {
		t.Errorf("phase 2 summary mismatch: topics=%d/%d threads=%d/%d messages=%d/%d attachments=%d/%d",
			summary2.Topics, len(topics), summary2.Threads, len(threads),
			summary2.Messages, len(messages), summary2.Attachments, len(attachments))
	}

	// Phase 4: Field-by-field verification against original data
	verifyMigratedData(t, final, topics, threads, messages, attachments)
}

func TestMigrateRoundTrip_MarkdownToSqliteToMarkdown(t *testing.T) {
	// Phase 1: Create data in Markdown
	srcDir := t.TempDir()
	original, err := NewMarkdownStore(srcDir)
	if err != nil {
		t.Fatalf("create original store: %v", err)
	}
	defer original.Close()

	topics, threads, messages, attachments := seedRichTestData(t, original)

	// Phase 2: Migrate Markdown -> SQLite
	sqlDir := t.TempDir()
	sqlStore, err := NewSqliteStore(filepath.Join(sqlDir, "mid.db"))
	if err != nil {
		t.Fatalf("create sqlite store: %v", err)
	}
	defer sqlStore.Close()

	_, err = MigrateData(original, sqlStore)
	if err != nil {
		t.Fatalf("MigrateData (markdown->sqlite) failed: %v", err)
	}

	// Phase 3: Migrate SQLite -> new Markdown
	dstDir := t.TempDir()
	final, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("create final store: %v", err)
	}
	defer final.Close()

	_, err = MigrateData(sqlStore, final)
	if err != nil {
		t.Fatalf("MigrateData (sqlite->markdown) failed: %v", err)
	}

	// Phase 4: Verify all data
	verifyMigratedData(t, final, topics, threads, messages, attachments)
}

// seedRichTestData creates a richer data set than seedTestData, including
// special characters, multiple messages with edits, and binary attachments.
func seedRichTestData(t *testing.T, src Storage) (topics []*models.Topic, threads []*models.Thread, messages []*models.Message, attachments []*models.Attachment) {
	t.Helper()

	// Topics with special characters in descriptions
	topic1 := models.NewTopic("general", "General discussion & Q/A", "admin@cli")
	topic2 := models.NewTopic("dev-ops", "CI/CD, deployments, & infrastructure", "admin@cli")
	topic2.Archived = true
	topic3 := models.NewTopic("random", "Off-topic chatter", "admin@cli")
	mustNoErr(t, src.CreateTopic(topic1))
	mustNoErr(t, src.CreateTopic(topic2))
	mustNoErr(t, src.CreateTopic(topic3))
	topics = append(topics, topic1, topic2, topic3)

	// Threads with various configurations
	thread1 := models.NewThread(topic1.ID, "Welcome Thread", "admin@cli")
	thread1.Sticky = true
	thread2 := models.NewThread(topic1.ID, "Random Chat", "user@cli")
	thread3 := models.NewThread(topic2.ID, "Build Failures", "ci@mcp")
	thread4 := models.NewThread(topic3.ID, "Favorite Foods", "user@cli")
	mustNoErr(t, src.CreateThread(thread1))
	mustNoErr(t, src.CreateThread(thread2))
	mustNoErr(t, src.CreateThread(thread3))
	mustNoErr(t, src.CreateThread(thread4))
	threads = append(threads, thread1, thread2, thread3, thread4)

	// Messages with special content
	msg1 := models.NewMessage(thread1.ID, "Welcome to the BBS! Rules:\n1. Be kind\n2. Have fun", "admin@cli")
	time.Sleep(time.Millisecond)
	msg2 := models.NewMessage(thread1.ID, "Thanks for the welcome!", "user@cli")
	now := time.Now()
	msg2.EditedAt = &now
	time.Sleep(time.Millisecond)
	msg3 := models.NewMessage(thread2.ID, "Content with `backticks` and **bold** and _italic_", "user@cli")
	time.Sleep(time.Millisecond)
	msg4 := models.NewMessage(thread3.ID, "Build is red:\n```\nERROR: test failure on line 42\n```", "ci@mcp")
	time.Sleep(time.Millisecond)
	msg5 := models.NewMessage(thread4.ID, "Pizza > everything", "user@cli")
	time.Sleep(time.Millisecond)
	msg6 := models.NewMessage(thread1.ID, "Message with special chars: <>&\"'", "user2@cli")
	mustNoErr(t, src.CreateMessage(msg1))
	mustNoErr(t, src.CreateMessage(msg2))
	mustNoErr(t, src.CreateMessage(msg3))
	mustNoErr(t, src.CreateMessage(msg4))
	mustNoErr(t, src.CreateMessage(msg5))
	mustNoErr(t, src.CreateMessage(msg6))
	messages = append(messages, msg1, msg2, msg3, msg4, msg5, msg6)

	// Attachments including binary-like data
	att1 := models.NewAttachment(msg4.ID, "build.log", "text/plain", []byte("ERROR: test failure on line 42\nFAILED: 3 tests"))
	att2 := models.NewAttachment(msg1.ID, "rules.txt", "text/plain", []byte("Be kind. Have fun."))
	att3 := models.NewAttachment(msg4.ID, "screenshot.bin", "application/octet-stream", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	mustNoErr(t, src.CreateAttachment(att1))
	mustNoErr(t, src.CreateAttachment(att2))
	mustNoErr(t, src.CreateAttachment(att3))
	attachments = append(attachments, att1, att2, att3)

	return
}

func TestMigrateData_PreservesThreadOrdering(t *testing.T) {
	// Create data in SQLite with specific sticky/non-sticky threads
	srcDir := t.TempDir()
	src, err := NewSqliteStore(filepath.Join(srcDir, "bbs.db"))
	if err != nil {
		t.Fatalf("create source store: %v", err)
	}
	defer src.Close()

	topic := models.NewTopic("test-ordering", "Test ordering", "test@cli")
	mustNoErr(t, src.CreateTopic(topic))

	// Create threads: first normal, then sticky
	normalThread := models.NewThread(topic.ID, "Normal Thread", "test@cli")
	stickyThread := models.NewThread(topic.ID, "Sticky Thread", "test@cli")
	stickyThread.Sticky = true
	mustNoErr(t, src.CreateThread(normalThread))
	mustNoErr(t, src.CreateThread(stickyThread))

	// Migrate to markdown
	dstDir := t.TempDir()
	dst, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("create destination store: %v", err)
	}
	defer dst.Close()

	_, err = MigrateData(src, dst)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	// Verify ordering: sticky should come first
	threads, err := dst.ListThreads(topic.ID)
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}
	if !threads[0].Sticky {
		t.Error("sticky thread should be first in listing")
	}
	if threads[1].Sticky {
		t.Error("normal thread should not be sticky")
	}
}

func TestMigrateData_PreservesMessageOrdering(t *testing.T) {
	// Verify messages arrive in created_at order after migration
	srcDir := t.TempDir()
	src, err := NewSqliteStore(filepath.Join(srcDir, "bbs.db"))
	if err != nil {
		t.Fatalf("create source store: %v", err)
	}
	defer src.Close()

	topic := models.NewTopic("msg-order", "Message order test", "test@cli")
	mustNoErr(t, src.CreateTopic(topic))
	thread := models.NewThread(topic.ID, "Ordered Thread", "test@cli")
	mustNoErr(t, src.CreateThread(thread))

	// Create messages with defined order
	for i := 0; i < 5; i++ {
		time.Sleep(time.Millisecond)
		msg := models.NewMessage(thread.ID, strings.Repeat("x", i+1), "test@cli")
		mustNoErr(t, src.CreateMessage(msg))
	}

	// Migrate to markdown
	dstDir := t.TempDir()
	dst, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("create destination store: %v", err)
	}
	defer dst.Close()

	_, err = MigrateData(src, dst)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	// Verify ordering
	messages, err := dst.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(messages) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(messages))
	}
	for i := 0; i < len(messages)-1; i++ {
		if !messages[i].CreatedAt.Before(messages[i+1].CreatedAt) {
			t.Errorf("messages not in order: msg[%d].CreatedAt=%v >= msg[%d].CreatedAt=%v",
				i, messages[i].CreatedAt, i+1, messages[i+1].CreatedAt)
		}
	}
}
