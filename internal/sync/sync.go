// ABOUTME: Sync engine for vault synchronization
// ABOUTME: Handles push/pull of changes to/from vault

package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/harper/bbs/internal/config"
	"github.com/harper/bbs/internal/models"
	"github.com/oklog/ulid/v2"
)

// ChangeType represents the type of change
type ChangeType string

const (
	ChangeUpsert ChangeType = "upsert"
	ChangeDelete ChangeType = "delete"
)

// EntityType represents the entity being changed
type EntityType string

const (
	EntityTopic      EntityType = "topic"
	EntityThread     EntityType = "thread"
	EntityMessage    EntityType = "message"
	EntityAttachment EntityType = "attachment"
)

// Change represents a queued change
type Change struct {
	ID        string     `json:"id"`
	Entity    EntityType `json:"entity"`
	EntityID  string     `json:"entity_id"`
	Op        ChangeType `json:"op"`
	Payload   string     `json:"payload"`
	CreatedAt time.Time  `json:"created_at"`
}

// Syncer handles synchronization
type Syncer struct {
	appDB   *sql.DB
	config  *config.Config
	enabled bool
}

// NewSyncer creates a new syncer
func NewSyncer(appDB *sql.DB) (*Syncer, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	return &Syncer{
		appDB:   appDB,
		config:  cfg,
		enabled: cfg.IsConfigured() && cfg.AutoSync,
	}, nil
}

// QueueTopicChange queues a topic change for sync
func (s *Syncer) QueueTopicChange(topic *models.Topic, op ChangeType) error {
	if !s.enabled {
		return nil
	}

	payload, _ := json.Marshal(topic)
	return s.queueChange(EntityTopic, topic.ID.String(), op, string(payload))
}

// QueueThreadChange queues a thread change for sync
func (s *Syncer) QueueThreadChange(thread *models.Thread, op ChangeType) error {
	if !s.enabled {
		return nil
	}

	payload, _ := json.Marshal(thread)
	return s.queueChange(EntityThread, thread.ID.String(), op, string(payload))
}

// QueueMessageChange queues a message change for sync
func (s *Syncer) QueueMessageChange(msg *models.Message, op ChangeType) error {
	if !s.enabled {
		return nil
	}

	payload, _ := json.Marshal(msg)
	return s.queueChange(EntityMessage, msg.ID.String(), op, string(payload))
}

func (s *Syncer) queueChange(entity EntityType, entityID string, op ChangeType, payload string) error {
	// For now, just log the change. Full vault integration would queue to vault.db
	change := Change{
		ID:        ulid.Make().String(),
		Entity:    entity,
		EntityID:  entityID,
		Op:        op,
		Payload:   payload,
		CreatedAt: time.Now(),
	}

	// TODO: Queue to vault database when fully integrated
	_ = change
	return nil
}

// Sync performs a full sync (push local changes, pull remote)
func (s *Syncer) Sync(ctx context.Context) error {
	if !s.config.IsConfigured() {
		return fmt.Errorf("sync not configured - run 'bbs sync login' first")
	}

	// TODO: Implement full vault sync when integrated
	return nil
}

// ApplyChange applies a remote change to local database
func (s *Syncer) ApplyChange(change *Change) error {
	switch change.Entity {
	case EntityTopic:
		return s.applyTopicChange(change)
	case EntityThread:
		return s.applyThreadChange(change)
	case EntityMessage:
		return s.applyMessageChange(change)
	}
	return nil
}

func (s *Syncer) applyTopicChange(change *Change) error {
	if change.Op == ChangeDelete {
		// Delete topic
		_, err := s.appDB.Exec("DELETE FROM topics WHERE id = ?", change.EntityID)
		return err
	}

	var topic models.Topic
	if err := json.Unmarshal([]byte(change.Payload), &topic); err != nil {
		return err
	}

	// Upsert topic
	_, err := s.appDB.Exec(`
		INSERT INTO topics (id, name, description, created_at, created_by, archived)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			archived = excluded.archived`,
		topic.ID.String(), topic.Name, topic.Description,
		topic.CreatedAt, topic.CreatedBy, topic.Archived)
	return err
}

func (s *Syncer) applyThreadChange(change *Change) error {
	if change.Op == ChangeDelete {
		_, err := s.appDB.Exec("DELETE FROM threads WHERE id = ?", change.EntityID)
		return err
	}

	var thread models.Thread
	if err := json.Unmarshal([]byte(change.Payload), &thread); err != nil {
		return err
	}

	_, err := s.appDB.Exec(`
		INSERT INTO threads (id, topic_id, subject, created_at, created_by, sticky)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			subject = excluded.subject,
			sticky = excluded.sticky`,
		thread.ID.String(), thread.TopicID.String(), thread.Subject,
		thread.CreatedAt, thread.CreatedBy, thread.Sticky)
	return err
}

func (s *Syncer) applyMessageChange(change *Change) error {
	if change.Op == ChangeDelete {
		_, err := s.appDB.Exec("DELETE FROM messages WHERE id = ?", change.EntityID)
		return err
	}

	var msg models.Message
	if err := json.Unmarshal([]byte(change.Payload), &msg); err != nil {
		return err
	}

	_, err := s.appDB.Exec(`
		INSERT INTO messages (id, thread_id, content, created_at, created_by, edited_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content = excluded.content,
			edited_at = excluded.edited_at`,
		msg.ID.String(), msg.ThreadID.String(), msg.Content,
		msg.CreatedAt, msg.CreatedBy, msg.EditedAt)
	return err
}

// IsEnabled returns true if sync is enabled
func (s *Syncer) IsEnabled() bool {
	return s.enabled
}

// GetPendingCount returns the number of pending changes
func (s *Syncer) GetPendingCount() int {
	// TODO: Query vault.db for pending changes
	return 0
}
