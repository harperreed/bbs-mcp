// ABOUTME: Message database operations
// ABOUTME: CRUD functions for messages table

package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/harper/bbs/internal/models"
)

// CreateMessage inserts a new message into the database.
func CreateMessage(db *sql.DB, msg *models.Message) error {
	_, err := db.Exec(`
		INSERT INTO messages (id, thread_id, content, created_at, created_by, edited_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		msg.ID.String(), msg.ThreadID.String(), msg.Content,
		msg.CreatedAt, msg.CreatedBy, msg.EditedAt)
	return err
}

// GetMessageByID retrieves a message by its UUID (supports prefix matching).
func GetMessageByID(db *sql.DB, id string) (*models.Message, error) {
	query := `SELECT id, thread_id, content, created_at, created_by, edited_at
			  FROM messages WHERE id = ? OR id LIKE ?`
	row := db.QueryRow(query, id, id+"%")
	return scanMessage(row)
}

// ListMessages returns all messages in a thread, ordered by created_at.
func ListMessages(db *sql.DB, threadID string) ([]*models.Message, error) {
	query := `SELECT id, thread_id, content, created_at, created_by, edited_at
			  FROM messages WHERE thread_id = ? OR thread_id LIKE ?
			  ORDER BY created_at ASC`

	rows, err := db.Query(query, threadID, threadID+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg, err := scanMessageFromRows(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// UpdateMessage updates the content of a message and sets edited_at.
func UpdateMessage(db *sql.DB, id string, content string) error {
	now := time.Now()
	result, err := db.Exec(`UPDATE messages SET content = ?, edited_at = ? WHERE id = ? OR id LIKE ?`,
		content, now, id, id+"%")
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("message not found: %s", id)
	}
	return nil
}

func scanMessage(row *sql.Row) (*models.Message, error) {
	var msg models.Message
	var id, threadID string
	var editedAt sql.NullTime
	err := row.Scan(&id, &threadID, &msg.Content,
		&msg.CreatedAt, &msg.CreatedBy, &editedAt)
	if err != nil {
		return nil, err
	}
	msg.ID, _ = models.ParseUUID(id)
	msg.ThreadID, _ = models.ParseUUID(threadID)
	if editedAt.Valid {
		msg.EditedAt = &editedAt.Time
	}
	return &msg, nil
}

func scanMessageFromRows(rows *sql.Rows) (*models.Message, error) {
	var msg models.Message
	var id, threadID string
	var editedAt sql.NullTime
	err := rows.Scan(&id, &threadID, &msg.Content,
		&msg.CreatedAt, &msg.CreatedBy, &editedAt)
	if err != nil {
		return nil, err
	}
	msg.ID, _ = models.ParseUUID(id)
	msg.ThreadID, _ = models.ParseUUID(threadID)
	if editedAt.Valid {
		msg.EditedAt = &editedAt.Time
	}
	return &msg, nil
}
