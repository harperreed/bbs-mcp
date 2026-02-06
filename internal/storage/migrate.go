// ABOUTME: Data migration between BBS storage backends
// ABOUTME: Copies topics, threads, messages, and attachments from source to destination

package storage

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/harper/bbs/internal/models"
)

// MigrateSummary holds counts of migrated entities.
type MigrateSummary struct {
	Topics      int
	Threads     int
	Messages    int
	Attachments int
}

// MigrateData copies all data from src to dst storage.
// It iterates through topics, threads, messages, and attachments in order,
// creating each entity in the destination. The destination should be empty
// before calling this function.
func MigrateData(src, dst Storage) (*MigrateSummary, error) {
	summary := &MigrateSummary{}

	// List all topics (including archived)
	topics, err := src.ListTopics(true)
	if err != nil {
		return nil, fmt.Errorf("list source topics: %w", err)
	}

	for _, topic := range topics {
		if err := dst.CreateTopic(topic); err != nil {
			return nil, fmt.Errorf("create topic %q: %w", topic.Name, err)
		}
		summary.Topics++

		if err := migrateTopic(src, dst, topic, summary); err != nil {
			return nil, err
		}
	}

	return summary, nil
}

// migrateTopic copies all threads (and their messages/attachments) for a single topic.
func migrateTopic(src, dst Storage, topic *models.Topic, summary *MigrateSummary) error {
	threads, err := src.ListThreads(topic.ID)
	if err != nil {
		return fmt.Errorf("list threads for topic %q: %w", topic.Name, err)
	}

	for _, thread := range threads {
		if err := dst.CreateThread(thread); err != nil {
			return fmt.Errorf("create thread %q in topic %q: %w", thread.Subject, topic.Name, err)
		}
		summary.Threads++

		if err := migrateThread(src, dst, thread, summary); err != nil {
			return err
		}
	}
	return nil
}

// migrateThread copies all messages (and their attachments) for a single thread.
func migrateThread(src, dst Storage, thread *models.Thread, summary *MigrateSummary) error {
	messages, err := src.ListMessages(thread.ID)
	if err != nil {
		return fmt.Errorf("list messages for thread %q: %w", thread.Subject, err)
	}

	for _, msg := range messages {
		if err := dst.CreateMessage(msg); err != nil {
			return fmt.Errorf("create message %s in thread %q: %w", msg.ID, thread.Subject, err)
		}
		summary.Messages++

		if err := migrateAttachments(src, dst, msg.ID, summary); err != nil {
			return err
		}
	}
	return nil
}

// migrateAttachments copies all attachments for a single message.
func migrateAttachments(src, dst Storage, msgID uuid.UUID, summary *MigrateSummary) error {
	attachments, err := src.ListAttachments(msgID)
	if err != nil {
		return fmt.Errorf("list attachments for message %s: %w", msgID, err)
	}

	for _, att := range attachments {
		if err := dst.CreateAttachment(att); err != nil {
			return fmt.Errorf("create attachment %q for message %s: %w", att.Filename, msgID, err)
		}
		summary.Attachments++
	}
	return nil
}

// IsDirNonEmpty checks whether a directory exists and contains any files or subdirectories.
// Returns false if the directory does not exist or is empty.
func IsDirNonEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read directory %q: %w", path, err)
	}
	return len(entries) > 0, nil
}
