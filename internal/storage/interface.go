// ABOUTME: Storage interface for BBS data backends
// ABOUTME: Defines the contract that all storage implementations must satisfy

package storage

import (
	"github.com/google/uuid"

	"github.com/harper/bbs/internal/models"
)

// Storage defines the interface for BBS data persistence.
// Implementations include SqliteStore and future backends like markdown files.
type Storage interface {
	// Topic CRUD
	CreateTopic(t *models.Topic) error
	GetTopic(id uuid.UUID) (*models.Topic, error)
	GetTopicByName(name string) (*models.Topic, error)
	ListTopics(includeArchived bool) ([]*models.Topic, error)
	UpdateTopic(t *models.Topic) error
	DeleteTopic(id uuid.UUID) error
	ArchiveTopic(id uuid.UUID, archived bool) error

	// Thread CRUD
	CreateThread(t *models.Thread) error
	GetThread(id uuid.UUID) (*models.Thread, error)
	ListThreads(topicID uuid.UUID) ([]*models.Thread, error)
	UpdateThread(t *models.Thread) error
	DeleteThread(id uuid.UUID) error
	SetThreadSticky(id uuid.UUID, sticky bool) error

	// Message CRUD
	CreateMessage(m *models.Message) error
	GetMessage(id uuid.UUID) (*models.Message, error)
	ListMessages(threadID uuid.UUID) ([]*models.Message, error)
	UpdateMessage(m *models.Message) error
	DeleteMessage(id uuid.UUID) error

	// Attachment CRUD
	CreateAttachment(a *models.Attachment) error
	GetAttachment(id uuid.UUID) (*models.Attachment, error)
	ListAttachments(messageID uuid.UUID) ([]*models.Attachment, error)
	DeleteAttachment(id uuid.UUID) error

	// Resolution helpers
	ResolveTopic(idOrName string) (*models.Topic, error)
	ResolveThread(idPrefix string) (*models.Thread, error)
	ResolveMessage(idPrefix string) (*models.Message, error)

	// Close releases resources held by the storage backend.
	Close() error
}
