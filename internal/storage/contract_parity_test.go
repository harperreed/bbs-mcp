// ABOUTME: Cross-backend contract tests for missing entities and parent ownership.
// ABOUTME: Exercises real SQLite and Markdown stores without mocks or fakes.

package storage

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/harper/bbs/internal/models"
)

type contractStoreFactory struct {
	name string
	open func(t *testing.T) Storage
}

func contractStoreFactories() []contractStoreFactory {
	return []contractStoreFactory{
		{
			name: "sqlite",
			open: func(t *testing.T) Storage {
				t.Helper()
				store := newTestStore(t)
				t.Cleanup(func() {
					if err := store.Close(); err != nil {
						t.Errorf("close SQLite store: %v", err)
					}
				})
				return store
			},
		},
		{
			name: "markdown",
			open: func(t *testing.T) Storage {
				t.Helper()
				store := newTestMarkdownStore(t)
				t.Cleanup(func() {
					if err := store.Close(); err != nil {
						t.Errorf("close Markdown store: %v", err)
					}
				})
				return store
			},
		},
	}
}

func TestStorageContractThreadOrderingUsesUUIDToBreakActivityTies(t *testing.T) {
	updatedAt := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	threadIDs := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		uuid.MustParse("00000000-0000-0000-0000-000000000004"),
	}

	for _, factory := range contractStoreFactories() {
		t.Run(factory.name, func(t *testing.T) {
			exerciseThreadOrderingContract(t, factory.open(t), updatedAt, threadIDs)
		})
	}
}

func exerciseThreadOrderingContract(t *testing.T, store Storage, updatedAt time.Time, threadIDs []uuid.UUID) {
	t.Helper()
	topic := createContractTopic(t, store, "thread-order")
	threads := []*models.Thread{
		{ID: threadIDs[1], TopicID: topic.ID, Subject: "alpha non-sticky", CreatedAt: updatedAt, CreatedBy: "contract@test", UpdatedAt: updatedAt},
		{ID: threadIDs[0], TopicID: topic.ID, Subject: "zulu non-sticky", CreatedAt: updatedAt, CreatedBy: "contract@test", UpdatedAt: updatedAt},
		{ID: threadIDs[3], TopicID: topic.ID, Subject: "bravo sticky", CreatedAt: updatedAt, CreatedBy: "contract@test", UpdatedAt: updatedAt},
		{ID: threadIDs[2], TopicID: topic.ID, Subject: "yankee sticky", CreatedAt: updatedAt, CreatedBy: "contract@test", UpdatedAt: updatedAt},
	}
	for _, thread := range threads {
		if err := store.CreateThread(thread); err != nil {
			t.Fatalf("create thread %s: %v", thread.ID, err)
		}
	}
	for _, thread := range threads[2:] {
		setStickyAndAssertActivity(t, store, thread.ID, updatedAt)
	}

	listed, err := store.ListThreads(topic.ID)
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	assertThreadOrder(t, listed, []uuid.UUID{threadIDs[2], threadIDs[3], threadIDs[0], threadIDs[1]})
}

func setStickyAndAssertActivity(t *testing.T, store Storage, threadID uuid.UUID, updatedAt time.Time) {
	t.Helper()
	if err := store.SetThreadSticky(threadID, true); err != nil {
		t.Fatalf("set thread %s sticky: %v", threadID, err)
	}
	stored, err := store.GetThread(threadID)
	if err != nil {
		t.Fatalf("get sticky thread %s: %v", threadID, err)
	}
	if !stored.UpdatedAt.Equal(updatedAt) {
		t.Errorf("sticky toggle changed thread %s activity: got %s, want %s", threadID, stored.UpdatedAt, updatedAt)
	}
}

func assertThreadOrder(t *testing.T, listed []*models.Thread, want []uuid.UUID) {
	t.Helper()
	if len(listed) != len(want) {
		t.Fatalf("listed %d threads, want %d", len(listed), len(want))
	}
	for i, wantID := range want {
		if listed[i].ID != wantID {
			t.Errorf("thread %d = %s, want %s", i, listed[i].ID, wantID)
		}
	}
}

func TestStorageContractRejectsMissingEntities(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, store Storage) error
	}{
		{
			name: "archive topic",
			run: func(_ *testing.T, store Storage) error {
				return store.ArchiveTopic(uuid.New(), true)
			},
		},
		{
			name: "update topic",
			run: func(_ *testing.T, store Storage) error {
				return store.UpdateTopic(models.NewTopic("missing-"+uuid.NewString(), "missing", "contract@test"))
			},
		},
		{
			name: "delete topic",
			run: func(_ *testing.T, store Storage) error {
				return store.DeleteTopic(uuid.New())
			},
		},
		{
			name: "update thread",
			run: func(t *testing.T, store Storage) error {
				topic := createContractTopic(t, store, "missing-thread")
				return store.UpdateThread(models.NewThread(topic.ID, "missing", "contract@test"))
			},
		},
		{
			name: "delete thread",
			run: func(_ *testing.T, store Storage) error {
				return store.DeleteThread(uuid.New())
			},
		},
		{
			name: "list threads for missing topic",
			run: func(_ *testing.T, store Storage) error {
				_, err := store.ListThreads(uuid.New())
				return err
			},
		},
		{
			name: "set missing thread sticky",
			run: func(_ *testing.T, store Storage) error {
				return store.SetThreadSticky(uuid.New(), true)
			},
		},
		{
			name: "update message",
			run: func(t *testing.T, store Storage) error {
				thread := createContractThread(t, store, "missing-message")
				return store.UpdateMessage(models.NewMessage(thread.ID, "missing", "contract@test"))
			},
		},
		{
			name: "delete message",
			run: func(_ *testing.T, store Storage) error {
				return store.DeleteMessage(uuid.New())
			},
		},
		{
			name: "list messages for missing thread",
			run: func(_ *testing.T, store Storage) error {
				_, err := store.ListMessages(uuid.New())
				return err
			},
		},
		{
			name: "delete attachment",
			run:  deleteMissingAttachmentWithoutMutation,
		},
	}

	for _, factory := range contractStoreFactories() {
		t.Run(factory.name, func(t *testing.T) {
			store := factory.open(t)
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					err := test.run(t, store)
					if err == nil {
						t.Fatal("expected a non-nil error for a missing entity")
					}
					if strings.TrimSpace(err.Error()) == "" {
						t.Fatal("expected an actionable, non-empty error")
					}
				})
			}
		})
	}
}

func deleteMissingAttachmentWithoutMutation(t *testing.T, store Storage) error {
	t.Helper()
	thread := createContractThread(t, store, "missing-attachment")
	message := models.NewMessage(thread.ID, "attachment owner", "contract@test")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("create attachment owner: %v", err)
	}
	attachment := models.NewAttachment(message.ID, "preserved.txt", "text/plain", []byte("preserved"))
	if err := store.CreateAttachment(attachment); err != nil {
		t.Fatalf("create preserved attachment: %v", err)
	}

	err := store.DeleteAttachment(uuid.New())
	stored, getErr := store.GetAttachment(attachment.ID)
	if getErr != nil {
		t.Fatalf("get attachment after rejected delete: %v", getErr)
	}
	if stored.ID != attachment.ID || stored.MessageID != attachment.MessageID ||
		stored.Filename != attachment.Filename || stored.MimeType != attachment.MimeType ||
		!bytes.Equal(stored.Data, attachment.Data) || !stored.CreatedAt.Equal(attachment.CreatedAt) {
		t.Fatalf("rejected delete changed attachment: got %#v, want %#v", stored, attachment)
	}
	listed, listErr := store.ListAttachments(message.ID)
	if listErr != nil {
		t.Fatalf("list attachments after rejected delete: %v", listErr)
	}
	if len(listed) != 1 || listed[0].ID != attachment.ID {
		t.Fatalf("attachments after rejected delete = %#v, want only %s", listed, attachment.ID)
	}
	return err
}

func TestStorageContractUpdateTopicRejectsDuplicateNameAtomically(t *testing.T) {
	sourceCases := []struct {
		name     string
		populate bool
	}{
		{name: "empty source"},
		{name: "populated source", populate: true},
	}
	for _, factory := range contractStoreFactories() {
		for _, sourceCase := range sourceCases {
			for _, populateTarget := range []bool{false, true} {
				targetCase := "empty target"
				if populateTarget {
					targetCase = "populated target"
				}
				t.Run(factory.name+"/"+sourceCase.name+"/"+targetCase, func(t *testing.T) {
					exerciseDuplicateTopicNameUpdateContract(t, factory.open(t), sourceCase.populate, populateTarget)
				})
			}
		}
	}
}

func exerciseDuplicateTopicNameUpdateContract(t *testing.T, store Storage, populateSource bool, populateTarget bool) {
	t.Helper()
	source := createContractTopic(t, store, "rename-source")
	target := createContractTopic(t, store, "rename-target")
	var sourceThread *models.Thread
	var sourceMessage *models.Message
	if populateSource {
		sourceThread = models.NewThread(source.ID, "source thread", "contract@test")
		if err := store.CreateThread(sourceThread); err != nil {
			t.Fatalf("populate source topic: %v", err)
		}
		sourceMessage = models.NewMessage(sourceThread.ID, "source content", "contract@test")
		if err := store.CreateMessage(sourceMessage); err != nil {
			t.Fatalf("populate source thread: %v", err)
		}
	}
	var targetThread *models.Thread
	if populateTarget {
		targetThread = models.NewThread(target.ID, "target thread", "contract@test")
		if err := store.CreateThread(targetThread); err != nil {
			t.Fatalf("populate target topic: %v", err)
		}
	}

	source.Description = "same-name update"
	if err := store.UpdateTopic(source); err != nil {
		t.Fatalf("valid same-name update: %v", err)
	}

	update := *source
	update.Name = target.Name
	update.Description = "must not persist"
	if err := store.UpdateTopic(&update); err == nil {
		t.Fatal("expected rename to an existing topic name to fail")
	}

	storedSource, err := store.GetTopic(source.ID)
	if err != nil {
		t.Fatalf("get source after rejected rename: %v", err)
	}
	if storedSource.Name != source.Name || storedSource.Description != source.Description {
		t.Fatalf("rejected rename changed source: got %#v, want name %q description %q", storedSource, source.Name, source.Description)
	}
	storedTarget, err := store.GetTopic(target.ID)
	if err != nil {
		t.Fatalf("get target after rejected rename: %v", err)
	}
	if storedTarget.Name != target.Name || storedTarget.Description != target.Description {
		t.Fatalf("rejected rename changed target: got %#v, want %#v", storedTarget, target)
	}
	if sourceThread == nil {
		threads, err := store.ListThreads(source.ID)
		if err != nil || len(threads) != 0 {
			t.Fatalf("rejected rename changed empty source: threads=%#v err=%v", threads, err)
		}
	} else {
		storedSourceThread, err := store.GetThread(sourceThread.ID)
		if err != nil {
			t.Fatalf("get populated source after rejected rename: %v", err)
		}
		if storedSourceThread.TopicID != source.ID || storedSourceThread.Subject != sourceThread.Subject {
			t.Fatalf("rejected rename changed source thread: got %#v, want %#v", storedSourceThread, sourceThread)
		}
		storedSourceMessage, err := store.GetMessage(sourceMessage.ID)
		if err != nil {
			t.Fatalf("get source message after rejected rename: %v", err)
		}
		if storedSourceMessage.ThreadID != sourceThread.ID || storedSourceMessage.Content != sourceMessage.Content {
			t.Fatalf("rejected rename changed source message: got %#v, want %#v", storedSourceMessage, sourceMessage)
		}
	}
	if targetThread != nil {
		if _, err := store.GetThread(targetThread.ID); err != nil {
			t.Fatalf("rejected rename changed populated target: %v", err)
		}
	}
}

func TestStorageContractUpdateThreadRequiresOwningTopic(t *testing.T) {
	for _, factory := range contractStoreFactories() {
		t.Run(factory.name, func(t *testing.T) {
			store := factory.open(t)
			thread := createContractThread(t, store, "owned-thread")
			otherTopic := createContractTopic(t, store, "other-topic")

			update := *thread
			update.TopicID = otherTopic.ID
			update.Subject = "wrong-parent update"
			if err := store.UpdateThread(&update); err == nil {
				t.Fatal("expected a parent-mismatch error")
			}

			stored, err := store.GetThread(thread.ID)
			if err != nil {
				t.Fatalf("get thread after rejected update: %v", err)
			}
			if stored.TopicID != thread.TopicID {
				t.Fatalf("thread parent changed: got %s, want %s", stored.TopicID, thread.TopicID)
			}
			if stored.Subject != thread.Subject {
				t.Fatalf("thread subject changed: got %q, want %q", stored.Subject, thread.Subject)
			}
		})
	}
}

func TestStorageContractUpdateMessageRequiresOwningThread(t *testing.T) {
	for _, factory := range contractStoreFactories() {
		t.Run(factory.name, func(t *testing.T) {
			store := factory.open(t)
			originalThread := createContractThread(t, store, "original")
			otherThread := createContractThread(t, store, "other")
			message := models.NewMessage(originalThread.ID, "original content", "contract@test")
			if err := store.CreateMessage(message); err != nil {
				t.Fatalf("create message: %v", err)
			}

			update := *message
			update.ThreadID = otherThread.ID
			update.Content = "wrong-parent update"
			if err := store.UpdateMessage(&update); err == nil {
				t.Fatal("expected a parent-mismatch error")
			}

			stored, err := store.GetMessage(message.ID)
			if err != nil {
				t.Fatalf("get message after rejected update: %v", err)
			}
			if stored.ThreadID != originalThread.ID {
				t.Fatalf("message parent changed: got %s, want %s", stored.ThreadID, originalThread.ID)
			}
			if stored.Content != message.Content {
				t.Fatalf("message content changed: got %q, want %q", stored.Content, message.Content)
			}
		})
	}
}

func TestStorageContractCreateMessageRejectsMissingThreadAtomically(t *testing.T) {
	for _, factory := range contractStoreFactories() {
		t.Run(factory.name, func(t *testing.T) {
			store := factory.open(t)
			message := models.NewMessage(uuid.New(), "orphan", "contract@test")
			if err := store.CreateMessage(message); err == nil {
				t.Fatal("expected create message to reject a missing thread")
			}
			if _, err := store.GetMessage(message.ID); err == nil {
				t.Fatal("rejected message was partially persisted")
			}
		})
	}
}

func TestStorageContractListAttachmentsRequiresMessageAndUsesCanonicalOrder(t *testing.T) {
	createdAt := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	attachmentIDs := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		uuid.MustParse("00000000-0000-0000-0000-000000000003"),
	}

	for _, factory := range contractStoreFactories() {
		t.Run(factory.name, func(t *testing.T) {
			store := factory.open(t)
			t.Run("missing message", func(t *testing.T) {
				assertListAttachmentsRejectsMissingMessage(t, store)
			})
			t.Run("existing message", func(t *testing.T) {
				exerciseExistingMessageAttachmentList(t, store, createdAt, attachmentIDs)
			})
		})
	}
}

func assertListAttachmentsRejectsMissingMessage(t *testing.T, store Storage) {
	t.Helper()
	if _, err := store.ListAttachments(uuid.New()); err == nil {
		t.Fatal("expected list attachments to reject a missing message")
	}
}

func exerciseExistingMessageAttachmentList(t *testing.T, store Storage, createdAt time.Time, attachmentIDs []uuid.UUID) {
	t.Helper()
	thread := createContractThread(t, store, "attachment-list")
	message := models.NewMessage(thread.ID, "attachment owner", "contract@test")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("create message: %v", err)
	}

	listed, err := store.ListAttachments(message.ID)
	if err != nil {
		t.Fatalf("list empty attachments: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("listed %d attachments for empty message, want 0", len(listed))
	}

	attachments := []*models.Attachment{
		{ID: attachmentIDs[2], MessageID: message.ID, Filename: "third.txt", MimeType: "text/plain", Data: []byte("third"), CreatedAt: createdAt},
		{ID: attachmentIDs[1], MessageID: message.ID, Filename: "second.txt", MimeType: "text/plain", Data: []byte("second"), CreatedAt: createdAt.Add(time.Second)},
		{ID: attachmentIDs[0], MessageID: message.ID, Filename: "first.txt", MimeType: "text/plain", Data: []byte("first"), CreatedAt: createdAt},
	}
	for _, attachment := range attachments {
		if err := store.CreateAttachment(attachment); err != nil {
			t.Fatalf("create attachment %s: %v", attachment.ID, err)
		}
	}

	listed, err = store.ListAttachments(message.ID)
	if err != nil {
		t.Fatalf("list attachments: %v", err)
	}
	want := []*models.Attachment{attachments[2], attachments[0], attachments[1]}
	if len(listed) != len(want) {
		t.Fatalf("listed %d attachments, want %d", len(listed), len(want))
	}
	for i, wantAttachment := range want {
		if listed[i].ID != wantAttachment.ID {
			t.Errorf("attachment %d ID = %s, want %s", i, listed[i].ID, wantAttachment.ID)
		}
		if listed[i].MessageID != wantAttachment.MessageID {
			t.Errorf("attachment %d owner = %s, want fixture owner %s", i, listed[i].MessageID, wantAttachment.MessageID)
		}
		if listed[i].Filename != wantAttachment.Filename {
			t.Errorf("attachment %d filename = %q, want %q", i, listed[i].Filename, wantAttachment.Filename)
		}
		if listed[i].MimeType != wantAttachment.MimeType {
			t.Errorf("attachment %d MIME type = %q, want %q", i, listed[i].MimeType, wantAttachment.MimeType)
		}
		if !bytes.Equal(listed[i].Data, wantAttachment.Data) {
			t.Errorf("attachment %d data = %q, want %q", i, listed[i].Data, wantAttachment.Data)
		}
		if !listed[i].CreatedAt.Equal(wantAttachment.CreatedAt) {
			t.Errorf("attachment %d created at = %s, want %s", i, listed[i].CreatedAt, wantAttachment.CreatedAt)
		}
	}
}

func TestStorageContractValidMutationsAndCascade(t *testing.T) {
	for _, factory := range contractStoreFactories() {
		t.Run(factory.name, func(t *testing.T) {
			exerciseValidMutationsAndCascade(t, factory.open(t))
		})
	}
}

func exerciseValidMutationsAndCascade(t *testing.T, store Storage) {
	t.Helper()
	topic := createContractTopic(t, store, "valid")
	thread := models.NewThread(topic.ID, "before", "contract@test")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("create thread: %v", err)
	}
	message := models.NewMessage(thread.ID, "before", "contract@test")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("create message: %v", err)
	}

	topic.Description = "updated topic"
	if err := store.UpdateTopic(topic); err != nil {
		t.Fatalf("update topic: %v", err)
	}
	thread.Subject = "updated thread"
	if err := store.UpdateThread(thread); err != nil {
		t.Fatalf("update thread: %v", err)
	}
	if err := store.SetThreadSticky(thread.ID, true); err != nil {
		t.Fatalf("set thread sticky: %v", err)
	}
	message.Content = "updated message"
	if err := store.UpdateMessage(message); err != nil {
		t.Fatalf("update message: %v", err)
	}

	threads, err := store.ListThreads(topic.ID)
	if err != nil || len(threads) != 1 || threads[0].Subject != thread.Subject || !threads[0].Sticky {
		t.Fatalf("list updated thread: threads=%#v err=%v", threads, err)
	}
	messages, err := store.ListMessages(thread.ID)
	if err != nil || len(messages) != 1 || messages[0].Content != message.Content {
		t.Fatalf("list updated message: messages=%#v err=%v", messages, err)
	}

	if err := store.DeleteTopic(topic.ID); err != nil {
		t.Fatalf("delete topic: %v", err)
	}
	if _, err := store.GetThread(thread.ID); err == nil {
		t.Fatal("topic deletion did not cascade to thread")
	}
	if _, err := store.GetMessage(message.ID); err == nil {
		t.Fatal("topic deletion did not cascade to message")
	}
}

func TestSqliteCreateMessageRollsBackWhenActivityUpdateFails(t *testing.T) {
	store, err := NewSqliteStore(filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatalf("open SQLite store: %v", err)
	}
	defer store.Close()

	thread := createContractThread(t, store, "rollback")
	if _, err := store.db.Exec(`CREATE TRIGGER reject_activity
		BEFORE UPDATE OF updated_at ON threads
		BEGIN SELECT RAISE(ABORT, 'activity update rejected'); END`); err != nil {
		t.Fatalf("create rejecting trigger: %v", err)
	}
	message := models.NewMessage(thread.ID, "must roll back", "contract@test")
	if err := store.CreateMessage(message); err == nil {
		t.Fatal("expected activity update failure")
	}
	if _, err := store.GetMessage(message.ID); err == nil {
		t.Fatal("message insert survived failed activity update")
	}
}

func TestSqliteForeignKeysApplyToEveryPooledConnection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "board # pooled?.db")
	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("open SQLite store: %v", err)
	}
	defer store.Close()
	store.db.SetMaxOpenConns(4)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conns := make([]*sql.Conn, 0, 4)
	for range 4 {
		conn, err := store.db.Conn(ctx)
		if err != nil {
			t.Fatalf("acquire pooled connection: %v", err)
		}
		conns = append(conns, conn)
	}
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	for i, conn := range conns {
		var enabled int
		if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&enabled); err != nil {
			t.Fatalf("connection %d foreign_keys: %v", i, err)
		}
		if enabled != 1 {
			t.Errorf("connection %d foreign_keys = %d, want 1", i, enabled)
		}

		orphanThreadID := uuid.NewString()
		_, err := conn.ExecContext(ctx, `INSERT INTO threads
			(id, topic_id, subject, created_at, created_by, updated_at, sticky)
			VALUES (?, ?, ?, ?, ?, ?, 0)`,
			orphanThreadID, uuid.NewString(), "orphan", time.Now().UTC(), "contract@test", time.Now().UTC())
		if err == nil {
			t.Errorf("connection %d accepted an orphan thread", i)
		}
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("database path with URI characters was not preserved: %v", err)
	}
}

func createContractTopic(t *testing.T, store Storage, label string) *models.Topic {
	t.Helper()
	topic := models.NewTopic(label+"-"+uuid.NewString(), "contract topic", "contract@test")
	if err := store.CreateTopic(topic); err != nil {
		t.Fatalf("create topic: %v", err)
	}
	return topic
}

func createContractThread(t *testing.T, store Storage, label string) *models.Thread {
	t.Helper()
	topic := createContractTopic(t, store, label)
	thread := models.NewThread(topic.ID, label, "contract@test")
	if err := store.CreateThread(thread); err != nil {
		t.Fatalf("create thread: %v", err)
	}
	return thread
}
