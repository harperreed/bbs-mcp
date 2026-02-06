// ABOUTME: SQLite storage layer for BBS data
// ABOUTME: Implements CRUD operations for topics, threads, messages, attachments

package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/harper/bbs/internal/models"
)

// SqliteStore provides SQLite-backed storage for BBS data.
type SqliteStore struct {
	db *sql.DB
}

// Compile-time check that SqliteStore implements Storage.
var _ Storage = (*SqliteStore)(nil)

// schema defines the database schema
const schema = `
CREATE TABLE IF NOT EXISTS topics (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME NOT NULL,
    created_by TEXT NOT NULL,
    archived INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS threads (
    id TEXT PRIMARY KEY,
    topic_id TEXT NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    subject TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    created_by TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sticky INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    created_by TEXT NOT NULL,
    edited_at DATETIME
);

CREATE TABLE IF NOT EXISTS attachments (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    data BLOB NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_threads_topic_id ON threads(topic_id);
CREATE INDEX IF NOT EXISTS idx_threads_updated_at ON threads(updated_at);
CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_attachments_message_id ON attachments(message_id);
`

// NewSqliteStore creates a new SQLite-backed store.
func NewSqliteStore(dbPath string) (*SqliteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable foreign keys and WAL mode
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &SqliteStore{db: db}, nil
}

// Close closes the database connection.
func (s *SqliteStore) Close() error {
	return s.db.Close()
}

// Topic CRUD

// CreateTopic stores a new topic.
func (s *SqliteStore) CreateTopic(t *models.Topic) error {
	_, err := s.db.Exec(
		`INSERT INTO topics (id, name, description, created_at, created_by, archived)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		t.ID.String(), t.Name, t.Description, t.CreatedAt.UTC(), t.CreatedBy, boolToInt(t.Archived),
	)
	if err != nil {
		return fmt.Errorf("insert topic: %w", err)
	}
	return nil
}

// GetTopic retrieves a topic by ID.
func (s *SqliteStore) GetTopic(id uuid.UUID) (*models.Topic, error) {
	var t models.Topic
	var idStr string
	var archived int
	var createdAt string

	err := s.db.QueryRow(
		`SELECT id, name, description, created_at, created_by, archived FROM topics WHERE id = ?`,
		id.String(),
	).Scan(&idStr, &t.Name, &t.Description, &createdAt, &t.CreatedBy, &archived)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("topic not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query topic: %w", err)
	}

	t.ID, _ = uuid.Parse(idStr)
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	t.Archived = archived != 0
	return &t, nil
}

// GetTopicByName finds a topic by its name.
func (s *SqliteStore) GetTopicByName(name string) (*models.Topic, error) {
	var t models.Topic
	var idStr string
	var archived int
	var createdAt string

	err := s.db.QueryRow(
		`SELECT id, name, description, created_at, created_by, archived FROM topics WHERE name = ?`,
		name,
	).Scan(&idStr, &t.Name, &t.Description, &createdAt, &t.CreatedBy, &archived)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("topic not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("query topic: %w", err)
	}

	t.ID, _ = uuid.Parse(idStr)
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	t.Archived = archived != 0
	return &t, nil
}

// UpdateTopic updates an existing topic.
func (s *SqliteStore) UpdateTopic(t *models.Topic) error {
	_, err := s.db.Exec(
		`UPDATE topics SET name = ?, description = ?, archived = ? WHERE id = ?`,
		t.Name, t.Description, boolToInt(t.Archived), t.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update topic: %w", err)
	}
	return nil
}

// DeleteTopic deletes a topic (cascades to threads).
func (s *SqliteStore) DeleteTopic(id uuid.UUID) error {
	_, err := s.db.Exec(`DELETE FROM topics WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete topic: %w", err)
	}
	return nil
}

// ListTopics returns all topics, optionally including archived ones.
func (s *SqliteStore) ListTopics(includeArchived bool) ([]*models.Topic, error) {
	query := `SELECT id, name, description, created_at, created_by, archived FROM topics`
	if !includeArchived {
		query += ` WHERE archived = 0`
	}
	query += ` ORDER BY name`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query topics: %w", err)
	}
	defer rows.Close()

	var topics []*models.Topic
	for rows.Next() {
		var t models.Topic
		var idStr string
		var archived int
		var createdAt string

		if err := rows.Scan(&idStr, &t.Name, &t.Description, &createdAt, &t.CreatedBy, &archived); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		t.ID, _ = uuid.Parse(idStr)
		t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		t.Archived = archived != 0
		topics = append(topics, &t)
	}
	return topics, rows.Err()
}

// ArchiveTopic sets the archived status of a topic.
func (s *SqliteStore) ArchiveTopic(id uuid.UUID, archived bool) error {
	_, err := s.db.Exec(`UPDATE topics SET archived = ? WHERE id = ?`, boolToInt(archived), id.String())
	if err != nil {
		return fmt.Errorf("archive topic: %w", err)
	}
	return nil
}

// Thread CRUD

// CreateThread stores a new thread.
func (s *SqliteStore) CreateThread(t *models.Thread) error {
	_, err := s.db.Exec(
		`INSERT INTO threads (id, topic_id, subject, created_at, created_by, updated_at, sticky)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID.String(), t.TopicID.String(), t.Subject, t.CreatedAt.UTC(), t.CreatedBy, t.UpdatedAt.UTC(), boolToInt(t.Sticky),
	)
	if err != nil {
		return fmt.Errorf("insert thread: %w", err)
	}
	return nil
}

// GetThread retrieves a thread by ID.
func (s *SqliteStore) GetThread(id uuid.UUID) (*models.Thread, error) {
	var t models.Thread
	var idStr, topicIDStr string
	var sticky int
	var createdAt, updatedAt string

	err := s.db.QueryRow(
		`SELECT id, topic_id, subject, created_at, created_by, updated_at, sticky FROM threads WHERE id = ?`,
		id.String(),
	).Scan(&idStr, &topicIDStr, &t.Subject, &createdAt, &t.CreatedBy, &updatedAt, &sticky)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("thread not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query thread: %w", err)
	}

	t.ID, _ = uuid.Parse(idStr)
	t.TopicID, _ = uuid.Parse(topicIDStr)
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	t.Sticky = sticky != 0
	return &t, nil
}

// UpdateThread updates an existing thread.
func (s *SqliteStore) UpdateThread(t *models.Thread) error {
	_, err := s.db.Exec(
		`UPDATE threads SET subject = ?, sticky = ?, updated_at = ? WHERE id = ?`,
		t.Subject, boolToInt(t.Sticky), time.Now().UTC(), t.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update thread: %w", err)
	}
	return nil
}

// DeleteThread deletes a thread (cascades to messages).
func (s *SqliteStore) DeleteThread(id uuid.UUID) error {
	_, err := s.db.Exec(`DELETE FROM threads WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete thread: %w", err)
	}
	return nil
}

// ListThreads returns all threads for a topic, sorted by sticky then updated_at.
func (s *SqliteStore) ListThreads(topicID uuid.UUID) ([]*models.Thread, error) {
	rows, err := s.db.Query(
		`SELECT id, topic_id, subject, created_at, created_by, updated_at, sticky
		 FROM threads WHERE topic_id = ?
		 ORDER BY sticky DESC, updated_at DESC`,
		topicID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("query threads: %w", err)
	}
	defer rows.Close()

	var threads []*models.Thread
	for rows.Next() {
		var t models.Thread
		var idStr, topicIDStr string
		var sticky int
		var createdAt, updatedAt string

		if err := rows.Scan(&idStr, &topicIDStr, &t.Subject, &createdAt, &t.CreatedBy, &updatedAt, &sticky); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		t.ID, _ = uuid.Parse(idStr)
		t.TopicID, _ = uuid.Parse(topicIDStr)
		t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		t.Sticky = sticky != 0
		threads = append(threads, &t)
	}
	return threads, rows.Err()
}

// SetThreadSticky sets the sticky status of a thread.
func (s *SqliteStore) SetThreadSticky(id uuid.UUID, sticky bool) error {
	_, err := s.db.Exec(`UPDATE threads SET sticky = ? WHERE id = ?`, boolToInt(sticky), id.String())
	if err != nil {
		return fmt.Errorf("set thread sticky: %w", err)
	}
	return nil
}

// Message CRUD

// CreateMessage stores a new message.
func (s *SqliteStore) CreateMessage(m *models.Message) error {
	var editedAt interface{}
	if m.EditedAt != nil {
		editedAt = m.EditedAt.UTC()
	}

	_, err := s.db.Exec(
		`INSERT INTO messages (id, thread_id, content, created_at, created_by, edited_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.ID.String(), m.ThreadID.String(), m.Content, m.CreatedAt.UTC(), m.CreatedBy, editedAt,
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	// Update thread's updated_at
	_, err = s.db.Exec(`UPDATE threads SET updated_at = ? WHERE id = ?`, time.Now().UTC(), m.ThreadID.String())
	if err != nil {
		return fmt.Errorf("update thread timestamp: %w", err)
	}

	return nil
}

// GetMessage retrieves a message by ID.
func (s *SqliteStore) GetMessage(id uuid.UUID) (*models.Message, error) {
	var m models.Message
	var idStr, threadIDStr string
	var createdAt string
	var editedAt sql.NullString

	err := s.db.QueryRow(
		`SELECT id, thread_id, content, created_at, created_by, edited_at FROM messages WHERE id = ?`,
		id.String(),
	).Scan(&idStr, &threadIDStr, &m.Content, &createdAt, &m.CreatedBy, &editedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query message: %w", err)
	}

	m.ID, _ = uuid.Parse(idStr)
	m.ThreadID, _ = uuid.Parse(threadIDStr)
	m.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if editedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, editedAt.String)
		m.EditedAt = &t
	}
	return &m, nil
}

// UpdateMessage updates an existing message.
func (s *SqliteStore) UpdateMessage(m *models.Message) error {
	var editedAt interface{}
	if m.EditedAt != nil {
		editedAt = m.EditedAt.UTC()
	}

	_, err := s.db.Exec(
		`UPDATE messages SET content = ?, edited_at = ? WHERE id = ?`,
		m.Content, editedAt, m.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update message: %w", err)
	}
	return nil
}

// DeleteMessage deletes a message (cascades to attachments).
func (s *SqliteStore) DeleteMessage(id uuid.UUID) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	return nil
}

// ListMessages returns all messages for a thread, sorted by created_at.
func (s *SqliteStore) ListMessages(threadID uuid.UUID) ([]*models.Message, error) {
	rows, err := s.db.Query(
		`SELECT id, thread_id, content, created_at, created_by, edited_at
		 FROM messages WHERE thread_id = ?
		 ORDER BY created_at ASC`,
		threadID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var m models.Message
		var idStr, threadIDStr string
		var createdAt string
		var editedAt sql.NullString

		if err := rows.Scan(&idStr, &threadIDStr, &m.Content, &createdAt, &m.CreatedBy, &editedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.ID, _ = uuid.Parse(idStr)
		m.ThreadID, _ = uuid.Parse(threadIDStr)
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		if editedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, editedAt.String)
			m.EditedAt = &t
		}
		messages = append(messages, &m)
	}
	return messages, rows.Err()
}

// Attachment CRUD

// CreateAttachment stores a new attachment.
func (s *SqliteStore) CreateAttachment(a *models.Attachment) error {
	_, err := s.db.Exec(
		`INSERT INTO attachments (id, message_id, filename, mime_type, data, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID.String(), a.MessageID.String(), a.Filename, a.MimeType, a.Data, a.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert attachment: %w", err)
	}
	return nil
}

// GetAttachment retrieves an attachment by ID.
func (s *SqliteStore) GetAttachment(id uuid.UUID) (*models.Attachment, error) {
	var a models.Attachment
	var idStr, messageIDStr string
	var createdAt string

	err := s.db.QueryRow(
		`SELECT id, message_id, filename, mime_type, data, created_at FROM attachments WHERE id = ?`,
		id.String(),
	).Scan(&idStr, &messageIDStr, &a.Filename, &a.MimeType, &a.Data, &createdAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("attachment not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query attachment: %w", err)
	}

	a.ID, _ = uuid.Parse(idStr)
	a.MessageID, _ = uuid.Parse(messageIDStr)
	a.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return &a, nil
}

// DeleteAttachment deletes an attachment.
func (s *SqliteStore) DeleteAttachment(id uuid.UUID) error {
	_, err := s.db.Exec(`DELETE FROM attachments WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	return nil
}

// ListAttachments returns all attachments for a message.
func (s *SqliteStore) ListAttachments(messageID uuid.UUID) ([]*models.Attachment, error) {
	rows, err := s.db.Query(
		`SELECT id, message_id, filename, mime_type, data, created_at
		 FROM attachments WHERE message_id = ?
		 ORDER BY created_at ASC`,
		messageID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("query attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*models.Attachment
	for rows.Next() {
		var a models.Attachment
		var idStr, messageIDStr string
		var createdAt string

		if err := rows.Scan(&idStr, &messageIDStr, &a.Filename, &a.MimeType, &a.Data, &createdAt); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		a.ID, _ = uuid.Parse(idStr)
		a.MessageID, _ = uuid.Parse(messageIDStr)
		a.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		attachments = append(attachments, &a)
	}
	return attachments, rows.Err()
}

// Resolution helpers

// ResolveTopic finds a topic by ID, ID prefix, or name.
func (s *SqliteStore) ResolveTopic(idOrName string) (*models.Topic, error) {
	// Try as full UUID first
	if id, err := uuid.Parse(idOrName); err == nil {
		return s.GetTopic(id)
	}

	// Try by name
	if topic, err := s.GetTopicByName(idOrName); err == nil {
		return topic, nil
	}

	// Try as ID prefix
	topics, err := s.ListTopics(true)
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
func (s *SqliteStore) ResolveThread(idPrefix string) (*models.Thread, error) {
	// Try as full UUID first
	if id, err := uuid.Parse(idPrefix); err == nil {
		return s.GetThread(id)
	}

	// Try as ID prefix - scan all threads
	rows, err := s.db.Query(
		`SELECT id, topic_id, subject, created_at, created_by, updated_at, sticky FROM threads WHERE id LIKE ?`,
		idPrefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("query threads: %w", err)
	}
	defer rows.Close()

	var matches []*models.Thread
	for rows.Next() {
		var t models.Thread
		var idStr, topicIDStr string
		var sticky int
		var createdAt, updatedAt string

		if err := rows.Scan(&idStr, &topicIDStr, &t.Subject, &createdAt, &t.CreatedBy, &updatedAt, &sticky); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		t.ID, _ = uuid.Parse(idStr)
		t.TopicID, _ = uuid.Parse(topicIDStr)
		t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		t.Sticky = sticky != 0
		matches = append(matches, &t)
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
func (s *SqliteStore) ResolveMessage(idPrefix string) (*models.Message, error) {
	// Try as full UUID first
	if id, err := uuid.Parse(idPrefix); err == nil {
		return s.GetMessage(id)
	}

	// Try as ID prefix - scan all messages
	rows, err := s.db.Query(
		`SELECT id, thread_id, content, created_at, created_by, edited_at FROM messages WHERE id LIKE ?`,
		idPrefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var matches []*models.Message
	for rows.Next() {
		var m models.Message
		var idStr, threadIDStr string
		var createdAt string
		var editedAt sql.NullString

		if err := rows.Scan(&idStr, &threadIDStr, &m.Content, &createdAt, &m.CreatedBy, &editedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.ID, _ = uuid.Parse(idStr)
		m.ThreadID, _ = uuid.Parse(threadIDStr)
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		if editedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, editedAt.String)
			m.EditedAt = &t
		}
		matches = append(matches, &m)
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

// DataDir returns the default data directory path.
func DataDir() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "bbs")
}

// DefaultDBPath returns the default database path.
func DefaultDBPath() string {
	return filepath.Join(DataDir(), "bbs.db")
}

// Helper function
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
