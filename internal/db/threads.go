// ABOUTME: Thread database operations
// ABOUTME: CRUD functions for threads table

package db

import (
	"database/sql"
	"fmt"

	"github.com/harper/bbs/internal/models"
)

// CreateThread inserts a new thread into the database.
func CreateThread(db *sql.DB, thread *models.Thread) error {
	_, err := db.Exec(`
		INSERT INTO threads (id, topic_id, subject, created_at, created_by, sticky)
		VALUES (?, ?, ?, ?, ?, ?)`,
		thread.ID.String(), thread.TopicID.String(), thread.Subject,
		thread.CreatedAt, thread.CreatedBy, thread.Sticky)
	return err
}

// GetThreadByID retrieves a thread by its UUID (supports prefix matching).
func GetThreadByID(db *sql.DB, id string) (*models.Thread, error) {
	query := `SELECT id, topic_id, subject, created_at, created_by, sticky
			  FROM threads WHERE id = ? OR id LIKE ?`
	row := db.QueryRow(query, id, id+"%")
	return scanThread(row)
}

// ListThreads returns all threads in a topic, ordered by sticky then created_at.
func ListThreads(db *sql.DB, topicID string) ([]*models.Thread, error) {
	query := `SELECT id, topic_id, subject, created_at, created_by, sticky
			  FROM threads WHERE topic_id = ? OR topic_id LIKE ?
			  ORDER BY sticky DESC, created_at DESC`

	rows, err := db.Query(query, topicID, topicID+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []*models.Thread
	for rows.Next() {
		thread, err := scanThreadFromRows(rows)
		if err != nil {
			return nil, err
		}
		threads = append(threads, thread)
	}
	return threads, rows.Err()
}

// SetThreadSticky sets the sticky status of a thread.
func SetThreadSticky(db *sql.DB, id string, sticky bool) error {
	result, err := db.Exec(`UPDATE threads SET sticky = ? WHERE id = ? OR id LIKE ?`,
		sticky, id, id+"%")
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("thread not found: %s", id)
	}
	return nil
}

func scanThread(row *sql.Row) (*models.Thread, error) {
	var thread models.Thread
	var id, topicID string
	err := row.Scan(&id, &topicID, &thread.Subject,
		&thread.CreatedAt, &thread.CreatedBy, &thread.Sticky)
	if err != nil {
		return nil, err
	}
	thread.ID, err = models.ParseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("invalid thread ID %q: %w", id, err)
	}
	thread.TopicID, err = models.ParseUUID(topicID)
	if err != nil {
		return nil, fmt.Errorf("invalid topic ID %q: %w", topicID, err)
	}
	return &thread, nil
}

func scanThreadFromRows(rows *sql.Rows) (*models.Thread, error) {
	var thread models.Thread
	var id, topicID string
	err := rows.Scan(&id, &topicID, &thread.Subject,
		&thread.CreatedAt, &thread.CreatedBy, &thread.Sticky)
	if err != nil {
		return nil, err
	}
	thread.ID, err = models.ParseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("invalid thread ID %q: %w", id, err)
	}
	thread.TopicID, err = models.ParseUUID(topicID)
	if err != nil {
		return nil, fmt.Errorf("invalid topic ID %q: %w", topicID, err)
	}
	return &thread, nil
}
