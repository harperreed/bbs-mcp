// ABOUTME: Data migration between BBS storage backends
// ABOUTME: Copies topics, threads, messages, and attachments from source to destination

package storage

import (
	"fmt"
	"os"

	"github.com/harper/bbs/internal/models"
)

// MigrateSummary holds counts of migrated entities.
type MigrateSummary struct {
	Topics      int
	Threads     int
	Messages    int
	Attachments int
}

type migrationData struct {
	topics []migrationTopic
}

type migrationTopic struct {
	topic   *models.Topic
	threads []migrationThread
}

type migrationThread struct {
	thread   *models.Thread
	messages []migrationMessage
}

type migrationMessage struct {
	message     *models.Message
	attachments []*models.Attachment
}

// MigrateData copies all data from src to dst storage. It reads and validates the
// complete source graph before the first destination mutation. Destination writes
// still stop on the first error and are not rolled back. The destination should be
// empty before calling this function.
func MigrateData(src, dst Storage) (*MigrateSummary, error) {
	data, err := collectMigrationData(src)
	if err != nil {
		return nil, err
	}
	return writeMigrationData(dst, data)
}

func collectMigrationData(src Storage) (*migrationData, error) {
	topics, err := src.ListTopics(true)
	if err != nil {
		return nil, fmt.Errorf("list source topics: %w", err)
	}

	data := &migrationData{topics: make([]migrationTopic, 0, len(topics))}
	for _, topic := range topics {
		threads, err := src.ListThreads(topic.ID)
		if err != nil {
			return nil, fmt.Errorf("list threads for topic %q: %w", topic.Name, err)
		}

		collectedTopic := migrationTopic{
			topic:   topic,
			threads: make([]migrationThread, 0, len(threads)),
		}
		for _, thread := range threads {
			messages, err := src.ListMessages(thread.ID)
			if err != nil {
				return nil, fmt.Errorf("list messages for thread %q: %w", thread.Subject, err)
			}

			collectedThread := migrationThread{
				thread:   thread,
				messages: make([]migrationMessage, 0, len(messages)),
			}
			for _, message := range messages {
				attachments, err := src.ListAttachments(message.ID)
				if err != nil {
					return nil, fmt.Errorf("list attachments for message %s: %w", message.ID, err)
				}
				collectedThread.messages = append(collectedThread.messages, migrationMessage{
					message:     message,
					attachments: attachments,
				})
			}
			collectedTopic.threads = append(collectedTopic.threads, collectedThread)
		}
		data.topics = append(data.topics, collectedTopic)
	}

	return data, nil
}

func writeMigrationData(dst Storage, data *migrationData) (*MigrateSummary, error) {
	summary := &MigrateSummary{}

	for _, collectedTopic := range data.topics {
		if err := dst.CreateTopic(collectedTopic.topic); err != nil {
			return nil, fmt.Errorf("create topic %q: %w", collectedTopic.topic.Name, err)
		}
		summary.Topics++

		for _, collectedThread := range collectedTopic.threads {
			if err := dst.CreateThread(collectedThread.thread); err != nil {
				return nil, fmt.Errorf("create thread %q in topic %q: %w", collectedThread.thread.Subject, collectedTopic.topic.Name, err)
			}
			summary.Threads++

			for _, collectedMessage := range collectedThread.messages {
				if err := dst.CreateMessage(collectedMessage.message); err != nil {
					return nil, fmt.Errorf("create message %s in thread %q: %w", collectedMessage.message.ID, collectedThread.thread.Subject, err)
				}
				summary.Messages++

				for _, attachment := range collectedMessage.attachments {
					if err := dst.CreateAttachment(attachment); err != nil {
						return nil, fmt.Errorf("create attachment %q for message %s: %w", attachment.Filename, collectedMessage.message.ID, err)
					}
					summary.Attachments++
				}
			}
		}
	}

	return summary, nil
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
