// ABOUTME: Helper functions for queuing sync changes from CLI commands
// ABOUTME: Provides silent-fail pattern for optional sync integration

package sync

import (
	"context"
	"database/sql"

	"github.com/harperreed/sweet/vault"

	"github.com/harper/bbs/internal/config"
	"github.com/harper/bbs/internal/models"
)

// TryQueueTopicChange attempts to queue a topic change for sync.
// Returns nil if sync is not configured or if the change is queued successfully.
// Only returns an error if there's an actual sync failure after configuration.
func TryQueueTopicChange(ctx context.Context, appDB *sql.DB, topic *models.Topic, op vault.Op) error {
	cfg, err := config.Load()
	if err != nil {
		// No config or can't load - sync not set up, skip silently
		return nil
	}

	if !cfg.IsConfigured() {
		// Sync not configured - skip silently
		return nil
	}

	syncer, err := NewSyncer(appDB)
	if err != nil {
		// Can't create syncer - skip silently (might be temp issue)
		return nil
	}
	defer func() { _ = syncer.Close() }()

	return syncer.QueueTopicChange(ctx, topic, op)
}

// TryQueueThreadChange attempts to queue a thread change for sync.
// Returns nil if sync is not configured or if the change is queued successfully.
func TryQueueThreadChange(ctx context.Context, appDB *sql.DB, thread *models.Thread, op vault.Op) error {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	if !cfg.IsConfigured() {
		return nil
	}

	syncer, err := NewSyncer(appDB)
	if err != nil {
		return nil
	}
	defer func() { _ = syncer.Close() }()

	return syncer.QueueThreadChange(ctx, thread, op)
}

// TryQueueMessageChange attempts to queue a message change for sync.
// Returns nil if sync is not configured or if the change is queued successfully.
func TryQueueMessageChange(ctx context.Context, appDB *sql.DB, msg *models.Message, op vault.Op) error {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	if !cfg.IsConfigured() {
		return nil
	}

	syncer, err := NewSyncer(appDB)
	if err != nil {
		return nil
	}
	defer func() { _ = syncer.Close() }()

	return syncer.QueueMessageChange(ctx, msg, op)
}
