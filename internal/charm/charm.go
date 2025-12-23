// ABOUTME: Charm KV client wrapper using transactional Do API
// ABOUTME: Short-lived connections to avoid lock contention with other MCP servers

package charm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/charm/client"
	"github.com/charmbracelet/charm/kv"
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

// DBName is the name of the BBS key-value store
const DBName = "bbs"

// Client holds configuration for KV operations.
// Unlike the previous implementation, it does NOT hold a persistent connection.
// Each operation opens the database, performs the operation, and closes it.
type Client struct {
	dbName   string
	autoSync bool
}

// Option configures a Client.
type Option func(*Client)

// WithDBName sets the database name.
func WithDBName(name string) Option {
	return func(c *Client) {
		c.dbName = name
	}
}

// WithAutoSync enables or disables auto-sync after writes.
func WithAutoSync(enabled bool) Option {
	return func(c *Client) {
		c.autoSync = enabled
	}
}

// NewClient creates a new client with the given options.
func NewClient(opts ...Option) (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	// Set charm host if configured
	if cfg.CharmHost != "" {
		if err := os.Setenv("CHARM_HOST", cfg.CharmHost); err != nil {
			return nil, err
		}
	}

	c := &Client{
		dbName:   DBName,
		autoSync: cfg.AutoSync,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// DoReadOnly executes a function with read-only database access.
// Use this for batch read operations that need multiple Gets.
func (c *Client) DoReadOnly(fn func(k *kv.KV) error) error {
	return kv.DoReadOnly(c.dbName, fn)
}

// Do executes a function with write access to the database.
// Use this for batch write operations.
func (c *Client) Do(fn func(k *kv.KV) error) error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		if err := fn(k); err != nil {
			return err
		}
		if c.autoSync {
			return k.Sync()
		}
		return nil
	})
}

// --- Legacy compatibility layer ---
// These functions maintain backwards compatibility with existing code.

// InitGlobal is a no-op for backwards compatibility.
// With Do API, connections are automatically managed.
func InitGlobal() error {
	// Set CHARM_HOST if not already set
	if os.Getenv("CHARM_HOST") == "" {
		os.Setenv("CHARM_HOST", "charm.2389.dev")
	}
	return nil
}

// Global returns a new client instance.
func Global() (*Client, error) {
	return NewClient()
}

// Close is a no-op for backwards compatibility.
// With Do API, connections are automatically closed after each operation.
func (c *Client) Close() error {
	return nil
}

// IsLinked returns true if the user has linked their account.
func (c *Client) IsLinked() bool {
	_, err := c.ID()
	return err == nil
}

// ID returns the current user's Charm ID.
func (c *Client) ID() (string, error) {
	cc, err := client.NewClientWithDefaults()
	if err != nil {
		return "", err
	}
	return cc.ID()
}

// CharmClient returns a new charm client for auth operations.
func (c *Client) CharmClient() (*client.Client, error) {
	return client.NewClientWithDefaults()
}

// Sync triggers a manual sync with the charm server.
func (c *Client) Sync() error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		return k.Sync()
	})
}

// Reset clears all data (nuclear option).
func (c *Client) Reset() error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		return k.Reset()
	})
}

// Config returns the current configuration.
func (c *Client) Config() *Config {
	cfg, _ := LoadConfig()
	return cfg
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
	return c.Do(func(k *kv.KV) error {
		return k.Set(topicKey(t.ID), data)
	})
}

// GetTopic retrieves a topic by ID.
func (c *Client) GetTopic(id uuid.UUID) (*models.Topic, error) {
	var topic models.Topic
	err := c.DoReadOnly(func(k *kv.KV) error {
		data, err := k.Get(topicKey(id))
		if err != nil {
			if errors.Is(err, kv.ErrMissingKey) {
				return fmt.Errorf("topic not found: %s", id)
			}
			return err
		}
		return json.Unmarshal(data, &topic)
	})
	if err != nil {
		return nil, err
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
	return c.Do(func(k *kv.KV) error {
		return k.Delete(topicKey(id))
	})
}

// ListTopics returns all topics, optionally including archived ones.
func (c *Client) ListTopics(includeArchived bool) ([]*models.Topic, error) {
	var topics []*models.Topic
	prefix := []byte(TopicPrefix)

	err := c.DoReadOnly(func(k *kv.KV) error {
		// Get all keys from the database
		keys, err := k.Keys()
		if err != nil {
			return err
		}

		// Filter keys by prefix and unmarshal values
		for _, key := range keys {
			if !bytes.HasPrefix(key, prefix) {
				continue
			}

			data, err := k.Get(key)
			if err != nil {
				if errors.Is(err, kv.ErrMissingKey) {
					continue // Key was deleted between Keys() and Get()
				}
				return err
			}

			var topic models.Topic
			if err := json.Unmarshal(data, &topic); err != nil {
				return err
			}

			if includeArchived || !topic.Archived {
				topics = append(topics, &topic)
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
	return c.Do(func(k *kv.KV) error {
		return k.Set(threadKey(t.ID), data)
	})
}

// GetThread retrieves a thread by ID.
func (c *Client) GetThread(id uuid.UUID) (*models.Thread, error) {
	var thread models.Thread
	err := c.DoReadOnly(func(k *kv.KV) error {
		data, err := k.Get(threadKey(id))
		if err != nil {
			if errors.Is(err, kv.ErrMissingKey) {
				return fmt.Errorf("thread not found: %s", id)
			}
			return err
		}
		return json.Unmarshal(data, &thread)
	})
	if err != nil {
		return nil, err
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
	return c.Do(func(k *kv.KV) error {
		return k.Delete(threadKey(id))
	})
}

// ListThreads returns all threads for a topic.
func (c *Client) ListThreads(topicID uuid.UUID) ([]*models.Thread, error) {
	var threads []*models.Thread
	prefix := []byte(ThreadPrefix)

	err := c.DoReadOnly(func(k *kv.KV) error {
		// Get all keys from the database
		keys, err := k.Keys()
		if err != nil {
			return err
		}

		// Filter keys by prefix and unmarshal values
		for _, key := range keys {
			if !bytes.HasPrefix(key, prefix) {
				continue
			}

			data, err := k.Get(key)
			if err != nil {
				if errors.Is(err, kv.ErrMissingKey) {
					continue // Key was deleted between Keys() and Get()
				}
				return err
			}

			var thread models.Thread
			if err := json.Unmarshal(data, &thread); err != nil {
				return err
			}

			if thread.TopicID == topicID {
				threads = append(threads, &thread)
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
	return c.Do(func(k *kv.KV) error {
		return k.Set(messageKey(m.ID), data)
	})
}

// GetMessage retrieves a message by ID.
func (c *Client) GetMessage(id uuid.UUID) (*models.Message, error) {
	var msg models.Message
	err := c.DoReadOnly(func(k *kv.KV) error {
		data, err := k.Get(messageKey(id))
		if err != nil {
			if errors.Is(err, kv.ErrMissingKey) {
				return fmt.Errorf("message not found: %s", id)
			}
			return err
		}
		return json.Unmarshal(data, &msg)
	})
	if err != nil {
		return nil, err
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
	return c.Do(func(k *kv.KV) error {
		return k.Delete(messageKey(id))
	})
}

// ListMessages returns all messages for a thread.
func (c *Client) ListMessages(threadID uuid.UUID) ([]*models.Message, error) {
	var messages []*models.Message
	prefix := []byte(MessagePrefix)

	err := c.DoReadOnly(func(k *kv.KV) error {
		// Get all keys from the database
		keys, err := k.Keys()
		if err != nil {
			return err
		}

		// Filter keys by prefix and unmarshal values
		for _, key := range keys {
			if !bytes.HasPrefix(key, prefix) {
				continue
			}

			data, err := k.Get(key)
			if err != nil {
				if errors.Is(err, kv.ErrMissingKey) {
					continue // Key was deleted between Keys() and Get()
				}
				return err
			}

			var msg models.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				return err
			}

			if msg.ThreadID == threadID {
				messages = append(messages, &msg)
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
	return c.Do(func(k *kv.KV) error {
		return k.Set(attachmentKey(a.ID), data)
	})
}

// GetAttachment retrieves an attachment by ID.
func (c *Client) GetAttachment(id uuid.UUID) (*models.Attachment, error) {
	var att models.Attachment
	err := c.DoReadOnly(func(k *kv.KV) error {
		data, err := k.Get(attachmentKey(id))
		if err != nil {
			if errors.Is(err, kv.ErrMissingKey) {
				return fmt.Errorf("attachment not found: %s", id)
			}
			return err
		}
		return json.Unmarshal(data, &att)
	})
	if err != nil {
		return nil, err
	}
	return &att, nil
}

// DeleteAttachment deletes an attachment.
func (c *Client) DeleteAttachment(id uuid.UUID) error {
	return c.Do(func(k *kv.KV) error {
		return k.Delete(attachmentKey(id))
	})
}

// ListAttachments returns all attachments for a message.
func (c *Client) ListAttachments(messageID uuid.UUID) ([]*models.Attachment, error) {
	var attachments []*models.Attachment
	prefix := []byte(AttachmentPrefix)

	err := c.DoReadOnly(func(k *kv.KV) error {
		// Get all keys from the database
		keys, err := k.Keys()
		if err != nil {
			return err
		}

		// Filter keys by prefix and unmarshal values
		for _, key := range keys {
			if !bytes.HasPrefix(key, prefix) {
				continue
			}

			data, err := k.Get(key)
			if err != nil {
				if errors.Is(err, kv.ErrMissingKey) {
					continue // Key was deleted between Keys() and Get()
				}
				return err
			}

			var att models.Attachment
			if err := json.Unmarshal(data, &att); err != nil {
				return err
			}

			if att.MessageID == messageID {
				attachments = append(attachments, &att)
			}
		}
		return nil
	})

	return attachments, err
}
