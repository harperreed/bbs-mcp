// ABOUTME: Sync engine for vault synchronization
// ABOUTME: Handles push/pull of changes to/from vault using suitesync

package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/harperreed/sweet/vault"

	"github.com/harper/bbs/internal/config"
	"github.com/harper/bbs/internal/models"
)

// AppID is the unique identifier for BBS in the vault sync system.
// This UUID namespaces all BBS entities to prevent collisions with other apps.
const AppID = "dd3a2aae-cf61-490d-89a4-b8d91486dd37"

// Entity types for sync
const (
	EntityTopic      = "topic"
	EntityThread     = "thread"
	EntityMessage    = "message"
	EntityAttachment = "attachment"
)

// Syncer handles synchronization with vault
type Syncer struct {
	config      *config.Config
	store       *vault.Store
	keys        vault.Keys
	client      *vault.Client
	vaultSyncer *vault.Syncer
	appDB       *sql.DB
}

// NewSyncer creates a new syncer from config
func NewSyncer(appDB *sql.DB) (*Syncer, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// If not configured, return a disabled syncer
	if !cfg.IsConfigured() {
		return &Syncer{
			config: cfg,
			appDB:  appDB,
		}, nil
	}

	// Parse derived key (stored as hex-encoded seed)
	seed, err := vault.ParseSeedPhrase(cfg.DerivedKey)
	if err != nil {
		return nil, fmt.Errorf("invalid derived key: %w", err)
	}

	keys, err := vault.DeriveKeys(seed, "", vault.DefaultKDFParams())
	if err != nil {
		return nil, fmt.Errorf("derive keys: %w", err)
	}

	store, err := vault.OpenStore(cfg.VaultDB)
	if err != nil {
		return nil, fmt.Errorf("open vault store: %w", err)
	}

	var tokenExpires time.Time
	if cfg.TokenExpires != "" {
		tokenExpires, _ = time.Parse(time.RFC3339, cfg.TokenExpires)
	}

	client := vault.NewClient(vault.SyncConfig{
		BaseURL:      cfg.Server,
		AppID:        AppID,
		DeviceID:     cfg.DeviceID,
		AuthToken:    cfg.Token,
		RefreshToken: cfg.RefreshToken,
		TokenExpires: tokenExpires,
		OnTokenRefresh: func(token, refreshToken string, expires time.Time) {
			cfg.Token = token
			cfg.RefreshToken = refreshToken
			cfg.TokenExpires = expires.Format(time.RFC3339)
			if err := cfg.Save(); err != nil {
				fmt.Printf("warning: failed to save refreshed token: %v\n", err)
			}
		},
	})

	return &Syncer{
		config:      cfg,
		store:       store,
		keys:        keys,
		client:      client,
		vaultSyncer: vault.NewSyncer(store, client, keys, cfg.UserID),
		appDB:       appDB,
	}, nil
}

// Close releases syncer resources
func (s *Syncer) Close() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

// IsEnabled returns true if sync is enabled
func (s *Syncer) IsEnabled() bool {
	return s.config.IsConfigured() && s.store != nil && s.vaultSyncer != nil
}

// canSync returns true if we have all requirements to sync
func (s *Syncer) canSync() bool {
	return s.config.Server != "" && s.config.Token != "" && s.config.UserID != "" && s.store != nil && s.vaultSyncer != nil
}

// QueueTopicChange queues a topic change for sync
func (s *Syncer) QueueTopicChange(ctx context.Context, topic *models.Topic, op vault.Op) error {
	if s.vaultSyncer == nil {
		return nil
	}

	var payload map[string]any
	if op != vault.OpDelete {
		payload = map[string]any{
			"id":          topic.ID.String(),
			"name":        topic.Name,
			"description": topic.Description,
			"created_at":  topic.CreatedAt.UTC().Unix(),
			"created_by":  topic.CreatedBy,
			"archived":    topic.Archived,
		}
	}

	return s.queueChange(ctx, EntityTopic, topic.ID.String(), op, payload)
}

// QueueThreadChange queues a thread change for sync
func (s *Syncer) QueueThreadChange(ctx context.Context, thread *models.Thread, op vault.Op) error {
	if s.vaultSyncer == nil {
		return nil
	}

	var payload map[string]any
	if op != vault.OpDelete {
		payload = map[string]any{
			"id":         thread.ID.String(),
			"topic_id":   thread.TopicID.String(),
			"subject":    thread.Subject,
			"created_at": thread.CreatedAt.UTC().Unix(),
			"created_by": thread.CreatedBy,
			"sticky":     thread.Sticky,
		}
	}

	return s.queueChange(ctx, EntityThread, thread.ID.String(), op, payload)
}

// QueueMessageChange queues a message change for sync
func (s *Syncer) QueueMessageChange(ctx context.Context, msg *models.Message, op vault.Op) error {
	if s.vaultSyncer == nil {
		return nil
	}

	var payload map[string]any
	if op != vault.OpDelete {
		payload = map[string]any{
			"id":         msg.ID.String(),
			"thread_id":  msg.ThreadID.String(),
			"content":    msg.Content,
			"created_at": msg.CreatedAt.UTC().Unix(),
			"created_by": msg.CreatedBy,
		}
		if msg.EditedAt != nil {
			payload["edited_at"] = msg.EditedAt.UTC().Unix()
		}
	}

	return s.queueChange(ctx, EntityMessage, msg.ID.String(), op, payload)
}

func (s *Syncer) queueChange(ctx context.Context, entity, entityID string, op vault.Op, payload map[string]any) error {
	if s.vaultSyncer == nil {
		return nil
	}

	if _, err := s.vaultSyncer.QueueChange(ctx, entity, entityID, op, payload); err != nil {
		return fmt.Errorf("queue change: %w", err)
	}

	// Auto-sync if enabled
	if s.config.AutoSync && s.canSync() {
		return s.Sync(ctx)
	}

	return nil
}

// Sync pushes local changes and pulls remote changes
func (s *Syncer) Sync(ctx context.Context) error {
	return s.SyncWithEvents(ctx, nil)
}

// SyncWithEvents pushes local changes and pulls remote changes with progress callbacks
func (s *Syncer) SyncWithEvents(ctx context.Context, events *vault.SyncEvents) error {
	if !s.canSync() {
		return errors.New("sync not configured - run 'bbs sync login' first")
	}

	err := vault.Sync(ctx, s.store, s.client, s.keys, s.config.UserID, s.applyChange, events)
	if err != nil {
		// Check for device-related 403 errors (v0.3.0 device validation)
		errStr := err.Error()
		if strings.Contains(errStr, "403") ||
			strings.Contains(errStr, "device") ||
			strings.Contains(errStr, "not registered") {
			return fmt.Errorf("device not registered - please run 'bbs sync login' again: %w", err)
		}
	}
	return err
}

// applyChange applies a remote change to the local database
func (s *Syncer) applyChange(ctx context.Context, c vault.Change) error {
	switch c.Entity {
	case EntityTopic:
		return s.applyTopicChange(ctx, c)
	case EntityThread:
		return s.applyThreadChange(ctx, c)
	case EntityMessage:
		return s.applyMessageChange(ctx, c)
	default:
		// Ignore unknown entities
		return nil
	}
}

func (s *Syncer) applyTopicChange(ctx context.Context, c vault.Change) error {
	if c.Op == vault.OpDelete || c.Deleted {
		_, err := s.appDB.ExecContext(ctx, "DELETE FROM topics WHERE id = ?", c.EntityID)
		return err
	}

	var payload struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		CreatedAt   int64  `json:"created_at"`
		CreatedBy   string `json:"created_by"`
		Archived    bool   `json:"archived"`
	}
	if err := json.Unmarshal(c.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal topic payload: %w", err)
	}

	createdAt := time.Unix(payload.CreatedAt, 0)
	_, err := s.appDB.ExecContext(ctx, `
		INSERT INTO topics (id, name, description, created_at, created_by, archived)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			archived = excluded.archived`,
		payload.ID, payload.Name, payload.Description,
		createdAt, payload.CreatedBy, payload.Archived)
	return err
}

func (s *Syncer) applyThreadChange(ctx context.Context, c vault.Change) error {
	if c.Op == vault.OpDelete || c.Deleted {
		_, err := s.appDB.ExecContext(ctx, "DELETE FROM threads WHERE id = ?", c.EntityID)
		return err
	}

	var payload struct {
		ID        string `json:"id"`
		TopicID   string `json:"topic_id"`
		Subject   string `json:"subject"`
		CreatedAt int64  `json:"created_at"`
		CreatedBy string `json:"created_by"`
		Sticky    bool   `json:"sticky"`
	}
	if err := json.Unmarshal(c.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal thread payload: %w", err)
	}

	createdAt := time.Unix(payload.CreatedAt, 0)
	_, err := s.appDB.ExecContext(ctx, `
		INSERT INTO threads (id, topic_id, subject, created_at, created_by, sticky)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			subject = excluded.subject,
			sticky = excluded.sticky`,
		payload.ID, payload.TopicID, payload.Subject,
		createdAt, payload.CreatedBy, payload.Sticky)
	return err
}

func (s *Syncer) applyMessageChange(ctx context.Context, c vault.Change) error {
	if c.Op == vault.OpDelete || c.Deleted {
		_, err := s.appDB.ExecContext(ctx, "DELETE FROM messages WHERE id = ?", c.EntityID)
		return err
	}

	var payload struct {
		ID        string `json:"id"`
		ThreadID  string `json:"thread_id"`
		Content   string `json:"content"`
		CreatedAt int64  `json:"created_at"`
		CreatedBy string `json:"created_by"`
		EditedAt  *int64 `json:"edited_at,omitempty"`
	}
	if err := json.Unmarshal(c.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal message payload: %w", err)
	}

	createdAt := time.Unix(payload.CreatedAt, 0)
	var editedAt *time.Time
	if payload.EditedAt != nil {
		t := time.Unix(*payload.EditedAt, 0)
		editedAt = &t
	}

	_, err := s.appDB.ExecContext(ctx, `
		INSERT INTO messages (id, thread_id, content, created_at, created_by, edited_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content = excluded.content,
			edited_at = excluded.edited_at`,
		payload.ID, payload.ThreadID, payload.Content,
		createdAt, payload.CreatedBy, editedAt)
	return err
}

// GetPendingCount returns the number of pending changes
func (s *Syncer) GetPendingCount(ctx context.Context) (int, error) {
	if s.store == nil {
		return 0, nil
	}
	batch, err := s.store.DequeueBatch(ctx, 1000)
	if err != nil {
		return 0, err
	}
	return len(batch), nil
}

// PendingItem represents a change waiting to be synced
type PendingItem struct {
	ChangeID string
	Entity   string
	TS       time.Time
}

// PendingChanges returns details of changes waiting to be synced
func (s *Syncer) PendingChanges(ctx context.Context) ([]PendingItem, error) {
	if s.store == nil {
		return nil, nil
	}
	batch, err := s.store.DequeueBatch(ctx, 100)
	if err != nil {
		return nil, err
	}

	items := make([]PendingItem, len(batch))
	for i, b := range batch {
		items[i] = PendingItem{
			ChangeID: b.ChangeID,
			Entity:   b.Entity,
			TS:       time.Unix(b.TS, 0),
		}
	}
	return items, nil
}

// LastSyncedSeq returns the last pulled sequence number
func (s *Syncer) LastSyncedSeq(ctx context.Context) (string, error) {
	if s.store == nil {
		return "0", nil
	}
	return s.store.GetState(ctx, "last_pulled_seq", "0")
}
