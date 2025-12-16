// ABOUTME: Topic database operations
// ABOUTME: CRUD functions for topics table

package db

import (
	"database/sql"
	"fmt"

	"github.com/harper/bbs/internal/models"
)

// CreateTopic inserts a new topic into the database.
func CreateTopic(db *sql.DB, topic *models.Topic) error {
	_, err := db.Exec(`
		INSERT INTO topics (id, name, description, created_at, created_by, archived)
		VALUES (?, ?, ?, ?, ?, ?)`,
		topic.ID.String(), topic.Name, topic.Description,
		topic.CreatedAt, topic.CreatedBy, topic.Archived)
	return err
}

// GetTopicByID retrieves a topic by its UUID (supports prefix matching).
func GetTopicByID(db *sql.DB, id string) (*models.Topic, error) {
	query := `SELECT id, name, description, created_at, created_by, archived
			  FROM topics WHERE id = ? OR id LIKE ?`
	row := db.QueryRow(query, id, id+"%")
	return scanTopic(row)
}

// GetTopicByName retrieves a topic by its name.
func GetTopicByName(db *sql.DB, name string) (*models.Topic, error) {
	query := `SELECT id, name, description, created_at, created_by, archived
			  FROM topics WHERE name = ?`
	row := db.QueryRow(query, name)
	return scanTopic(row)
}

// ListTopics returns all topics, optionally filtering by archived status.
func ListTopics(db *sql.DB, includeArchived bool) ([]*models.Topic, error) {
	query := `SELECT id, name, description, created_at, created_by, archived
			  FROM topics`
	if !includeArchived {
		query += " WHERE archived = FALSE"
	}
	query += " ORDER BY name"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []*models.Topic
	for rows.Next() {
		topic, err := scanTopicFromRows(rows)
		if err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	return topics, rows.Err()
}

// ArchiveTopic sets the archived status of a topic.
func ArchiveTopic(db *sql.DB, id string, archived bool) error {
	result, err := db.Exec(`UPDATE topics SET archived = ? WHERE id = ? OR id LIKE ?`,
		archived, id, id+"%")
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("topic not found: %s", id)
	}
	return nil
}

// ResolveTopicID finds a topic by ID prefix or name, returning the full ID.
func ResolveTopicID(db *sql.DB, idOrName string) (string, error) {
	// Try by name first
	topic, err := GetTopicByName(db, idOrName)
	if err == nil {
		return topic.ID.String(), nil
	}

	// Try by ID prefix
	topic, err = GetTopicByID(db, idOrName)
	if err != nil {
		return "", fmt.Errorf("topic not found: %s", idOrName)
	}
	return topic.ID.String(), nil
}

func scanTopic(row *sql.Row) (*models.Topic, error) {
	var topic models.Topic
	var id string
	err := row.Scan(&id, &topic.Name, &topic.Description,
		&topic.CreatedAt, &topic.CreatedBy, &topic.Archived)
	if err != nil {
		return nil, err
	}
	topic.ID, err = models.ParseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("invalid topic ID %q: %w", id, err)
	}
	return &topic, nil
}

func scanTopicFromRows(rows *sql.Rows) (*models.Topic, error) {
	var topic models.Topic
	var id string
	err := rows.Scan(&id, &topic.Name, &topic.Description,
		&topic.CreatedAt, &topic.CreatedBy, &topic.Archived)
	if err != nil {
		return nil, err
	}
	topic.ID, err = models.ParseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("invalid topic ID %q: %w", id, err)
	}
	return &topic, nil
}
