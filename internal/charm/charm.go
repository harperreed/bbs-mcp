// ABOUTME: Charm KV client wrapper for cloud-synced storage
// ABOUTME: Provides automatic sync via SSH keys using Charm Cloud

package charm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/charm/client"
	"github.com/charmbracelet/charm/kv"
	badger "github.com/dgraph-io/badger/v3"
	"github.com/google/uuid"

	"github.com/harper/bbs/internal/models"
)

// Key prefixes for entity types
const (
	TopicPrefix      = "topic:"
	ThreadPrefix     = "thread:"
	MessagePrefix    = "message:"
	AttachmentPrefix = "attachment:"
)

// DefaultCharmHost is the default Charm server
const DefaultCharmHost = "charm.2389.dev"

// DBName is the name of the BBS key-value store
const DBName = "bbs"

var (
	globalKV   *kv.KV
	globalOnce sync.Once
	initErr    error
)

// Client wraps charm/kv.KV with BBS-specific operations
type Client struct {
	kv *kv.KV
}

// InitGlobal initializes the global Charm KV client.
// This is thread-safe and will only initialize once.
// Automatically falls back to read-only mode if another process holds the lock.
func InitGlobal() error {
	globalOnce.Do(func() {
		// Set CHARM_HOST if not already set
		if os.Getenv("CHARM_HOST") == "" {
			os.Setenv("CHARM_HOST", DefaultCharmHost)
		}

		globalKV, initErr = kv.OpenWithDefaultsFallback(DBName)
		if initErr != nil {
			return
		}

		// Sync on startup to pull remote changes (skip in read-only mode)
		if !globalKV.IsReadOnly() {
			initErr = globalKV.Sync()
		}
	})
	return initErr
}

// Global returns the global Charm client.
// Must call InitGlobal first.
func Global() (*Client, error) {
	if globalKV == nil {
		return nil, fmt.Errorf("charm not initialized - call InitGlobal first")
	}
	return &Client{kv: globalKV}, nil
}

// NewClient creates a new Charm client with the given KV store.
func NewClient(db *kv.KV) *Client {
	return &Client{kv: db}
}

// Close closes the underlying KV store.
func (c *Client) Close() error {
	if c.kv != nil {
		return c.kv.Close()
	}
	return nil
}

// ID returns the current user's Charm ID.
func (c *Client) ID() (string, error) {
	cc := c.kv.Client()
	return cc.ID()
}

// IsLinked returns true if the user has linked their account.
func (c *Client) IsLinked() bool {
	_, err := c.ID()
	return err == nil
}

// IsReadOnly returns true if the database is open in read-only mode.
// This happens when another process (like an MCP server) holds the lock.
func (c *Client) IsReadOnly() bool {
	return c.kv.IsReadOnly()
}

// Sync pulls remote changes from Charm Cloud.
func (c *Client) Sync() error {
	return c.kv.Sync()
}

// Reset wipes local data and re-syncs from Charm Cloud.
func (c *Client) Reset() error {
	return c.kv.Reset()
}

// KV returns the underlying kv.KV instance for direct access.
func (c *Client) KV() *kv.KV {
	return c.kv
}

// CharmClient returns the underlying charm client for auth operations.
func (c *Client) CharmClient() *client.Client {
	return c.kv.Client()
}

// key helpers

func topicKey(id uuid.UUID) []byte {
	return []byte(TopicPrefix + id.String())
}

func threadKey(id uuid.UUID) []byte {
	return []byte(ThreadPrefix + id.String())
}

func messageKey(id uuid.UUID) []byte {
	return []byte(MessagePrefix + id.String())
}

func attachmentKey(id uuid.UUID) []byte {
	return []byte(AttachmentPrefix + id.String())
}

// Topic CRUD

// CreateTopic stores a new topic.
func (c *Client) CreateTopic(t *models.Topic) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal topic: %w", err)
	}
	if err := c.kv.Set(topicKey(t.ID), data); err != nil {
		return err
	}
	// Push changes to cloud
	return c.kv.Sync()
}

// GetTopic retrieves a topic by ID.
func (c *Client) GetTopic(id uuid.UUID) (*models.Topic, error) {
	data, err := c.kv.Get(topicKey(id))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, fmt.Errorf("topic not found: %s", id)
		}
		return nil, err
	}
	var topic models.Topic
	if err := json.Unmarshal(data, &topic); err != nil {
		return nil, fmt.Errorf("unmarshal topic: %w", err)
	}
	return &topic, nil
}

// UpdateTopic updates an existing topic.
func (c *Client) UpdateTopic(t *models.Topic) error {
	return c.CreateTopic(t) // Same operation - overwrite
}

// DeleteTopic deletes a topic and all its threads (cascade).
func (c *Client) DeleteTopic(id uuid.UUID) error {
	// Cascade delete threads first
	threads, err := c.ListThreads(id)
	if err != nil {
		return fmt.Errorf("list threads for cascade delete: %w", err)
	}
	for _, thread := range threads {
		if err := c.DeleteThread(thread.ID); err != nil {
			return fmt.Errorf("cascade delete thread %s: %w", thread.ID, err)
		}
	}
	return c.kv.Delete(topicKey(id))
}

// ListTopics returns all topics, optionally including archived ones.
func (c *Client) ListTopics(includeArchived bool) ([]*models.Topic, error) {
	var topics []*models.Topic
	prefix := []byte(TopicPrefix)

	err := c.kv.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var topic models.Topic
				if err := json.Unmarshal(val, &topic); err != nil {
					return err
				}
				if includeArchived || !topic.Archived {
					topics = append(topics, &topic)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return topics, err
}

// GetTopicByName finds a topic by its name.
func (c *Client) GetTopicByName(name string) (*models.Topic, error) {
	topics, err := c.ListTopics(true) // Include archived
	if err != nil {
		return nil, err
	}
	for _, t := range topics {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, fmt.Errorf("topic not found: %s", name)
}

// Thread CRUD

// CreateThread stores a new thread.
func (c *Client) CreateThread(t *models.Thread) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal thread: %w", err)
	}
	if err := c.kv.Set(threadKey(t.ID), data); err != nil {
		return err
	}
	return c.kv.Sync()
}

// GetThread retrieves a thread by ID.
func (c *Client) GetThread(id uuid.UUID) (*models.Thread, error) {
	data, err := c.kv.Get(threadKey(id))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, fmt.Errorf("thread not found: %s", id)
		}
		return nil, err
	}
	var thread models.Thread
	if err := json.Unmarshal(data, &thread); err != nil {
		return nil, fmt.Errorf("unmarshal thread: %w", err)
	}
	return &thread, nil
}

// UpdateThread updates an existing thread.
func (c *Client) UpdateThread(t *models.Thread) error {
	return c.CreateThread(t)
}

// DeleteThread deletes a thread and all its messages (cascade).
func (c *Client) DeleteThread(id uuid.UUID) error {
	// Cascade delete messages first
	messages, err := c.ListMessages(id)
	if err != nil {
		return fmt.Errorf("list messages for cascade delete: %w", err)
	}
	for _, msg := range messages {
		if err := c.DeleteMessage(msg.ID); err != nil {
			return fmt.Errorf("cascade delete message %s: %w", msg.ID, err)
		}
	}
	return c.kv.Delete(threadKey(id))
}

// ListThreads returns all threads for a topic.
func (c *Client) ListThreads(topicID uuid.UUID) ([]*models.Thread, error) {
	var threads []*models.Thread
	prefix := []byte(ThreadPrefix)

	err := c.kv.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var thread models.Thread
				if err := json.Unmarshal(val, &thread); err != nil {
					return err
				}
				if thread.TopicID == topicID {
					threads = append(threads, &thread)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return threads, err
}

// Message CRUD

// CreateMessage stores a new message.
func (c *Client) CreateMessage(m *models.Message) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	if err := c.kv.Set(messageKey(m.ID), data); err != nil {
		return err
	}
	return c.kv.Sync()
}

// GetMessage retrieves a message by ID.
func (c *Client) GetMessage(id uuid.UUID) (*models.Message, error) {
	data, err := c.kv.Get(messageKey(id))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, fmt.Errorf("message not found: %s", id)
		}
		return nil, err
	}
	var msg models.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal message: %w", err)
	}
	return &msg, nil
}

// UpdateMessage updates an existing message.
func (c *Client) UpdateMessage(m *models.Message) error {
	return c.CreateMessage(m)
}

// DeleteMessage deletes a message and all its attachments (cascade).
func (c *Client) DeleteMessage(id uuid.UUID) error {
	// Cascade delete attachments first
	attachments, err := c.ListAttachments(id)
	if err != nil {
		return fmt.Errorf("list attachments for cascade delete: %w", err)
	}
	for _, att := range attachments {
		if err := c.DeleteAttachment(att.ID); err != nil {
			return fmt.Errorf("cascade delete attachment %s: %w", att.ID, err)
		}
	}
	return c.kv.Delete(messageKey(id))
}

// ListMessages returns all messages for a thread.
func (c *Client) ListMessages(threadID uuid.UUID) ([]*models.Message, error) {
	var messages []*models.Message
	prefix := []byte(MessagePrefix)

	err := c.kv.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var msg models.Message
				if err := json.Unmarshal(val, &msg); err != nil {
					return err
				}
				if msg.ThreadID == threadID {
					messages = append(messages, &msg)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return messages, err
}

// Attachment CRUD

// CreateAttachment stores a new attachment.
func (c *Client) CreateAttachment(a *models.Attachment) error {
	data, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshal attachment: %w", err)
	}
	return c.kv.Set(attachmentKey(a.ID), data)
}

// GetAttachment retrieves an attachment by ID.
func (c *Client) GetAttachment(id uuid.UUID) (*models.Attachment, error) {
	data, err := c.kv.Get(attachmentKey(id))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, fmt.Errorf("attachment not found: %s", id)
		}
		return nil, err
	}
	var att models.Attachment
	if err := json.Unmarshal(data, &att); err != nil {
		return nil, fmt.Errorf("unmarshal attachment: %w", err)
	}
	return &att, nil
}

// DeleteAttachment deletes an attachment.
func (c *Client) DeleteAttachment(id uuid.UUID) error {
	return c.kv.Delete(attachmentKey(id))
}

// ListAttachments returns all attachments for a message.
func (c *Client) ListAttachments(messageID uuid.UUID) ([]*models.Attachment, error) {
	var attachments []*models.Attachment
	prefix := []byte(AttachmentPrefix)

	err := c.kv.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var att models.Attachment
				if err := json.Unmarshal(val, &att); err != nil {
					return err
				}
				if att.MessageID == messageID {
					attachments = append(attachments, &att)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return attachments, err
}
