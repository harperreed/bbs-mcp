// ABOUTME: Entity resolution helpers for Charm KV
// ABOUTME: Supports resolving entities by ID prefix or name

package charm

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/harper/bbs/internal/models"
)

// ResolveTopic finds a topic by ID, ID prefix, or name.
func (c *Client) ResolveTopic(idOrName string) (*models.Topic, error) {
	// Try as full UUID first
	if id, err := uuid.Parse(idOrName); err == nil {
		return c.GetTopic(id)
	}

	// Try by name
	if topic, err := c.GetTopicByName(idOrName); err == nil {
		return topic, nil
	}

	// Try as ID prefix
	topics, err := c.ListTopics(true)
	if err != nil {
		return nil, err
	}

	var matches []*models.Topic
	for _, t := range topics {
		if strings.HasPrefix(t.ID.String(), idOrName) {
			matches = append(matches, t)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("topic not found: %s", idOrName)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous topic ID prefix '%s' matches %d topics", idOrName, len(matches))
	}
}

// ResolveThread finds a thread by ID or ID prefix.
func (c *Client) ResolveThread(idPrefix string) (*models.Thread, error) {
	// Try as full UUID first
	if id, err := uuid.Parse(idPrefix); err == nil {
		return c.GetThread(id)
	}

	// Try as ID prefix - need to scan all threads
	threads, err := c.listAllThreads()
	if err != nil {
		return nil, err
	}

	var matches []*models.Thread
	for _, t := range threads {
		if strings.HasPrefix(t.ID.String(), idPrefix) {
			matches = append(matches, t)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("thread not found: %s", idPrefix)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous thread ID prefix '%s' matches %d threads", idPrefix, len(matches))
	}
}

// ResolveMessage finds a message by ID or ID prefix.
func (c *Client) ResolveMessage(idPrefix string) (*models.Message, error) {
	// Try as full UUID first
	if id, err := uuid.Parse(idPrefix); err == nil {
		return c.GetMessage(id)
	}

	// Try as ID prefix - need to scan all messages
	messages, err := c.listAllMessages()
	if err != nil {
		return nil, err
	}

	var matches []*models.Message
	for _, m := range messages {
		if strings.HasPrefix(m.ID.String(), idPrefix) {
			matches = append(matches, m)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("message not found: %s", idPrefix)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous message ID prefix '%s' matches %d messages", idPrefix, len(matches))
	}
}

// listAllThreads returns all threads (for prefix matching).
func (c *Client) listAllThreads() ([]*models.Thread, error) {
	topics, err := c.ListTopics(true)
	if err != nil {
		return nil, err
	}

	var allThreads []*models.Thread
	for _, topic := range topics {
		threads, err := c.ListThreads(topic.ID)
		if err != nil {
			return nil, err
		}
		allThreads = append(allThreads, threads...)
	}
	return allThreads, nil
}

// listAllMessages returns all messages (for prefix matching).
func (c *Client) listAllMessages() ([]*models.Message, error) {
	threads, err := c.listAllThreads()
	if err != nil {
		return nil, err
	}

	var allMessages []*models.Message
	for _, thread := range threads {
		messages, err := c.ListMessages(thread.ID)
		if err != nil {
			return nil, err
		}
		allMessages = append(allMessages, messages...)
	}
	return allMessages, nil
}

// ArchiveTopic sets the archived status of a topic.
func (c *Client) ArchiveTopic(id uuid.UUID, archived bool) error {
	topic, err := c.GetTopic(id)
	if err != nil {
		return err
	}
	topic.Archived = archived
	return c.UpdateTopic(topic)
}

// SetThreadSticky sets the sticky status of a thread.
func (c *Client) SetThreadSticky(id uuid.UUID, sticky bool) error {
	thread, err := c.GetThread(id)
	if err != nil {
		return err
	}
	thread.Sticky = sticky
	return c.UpdateThread(thread)
}
