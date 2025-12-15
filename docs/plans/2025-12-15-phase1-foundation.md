# Phase 1: Foundation - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Set up Go project, data models, database layer, and CLI skeleton.

**Architecture:** Standard Go CLI with Cobra, SQLite via modernc.org/sqlite, XDG paths.

**Tech Stack:** Go 1.24+, Cobra, modernc.org/sqlite, google/uuid

---

## Task 1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `cmd/bbs/main.go`

**Step 1: Initialize module**

Run: `cd /Users/harper/Public/src/personal/suite/bbs && go mod init github.com/harper/bbs`

**Step 2: Create main.go**

```go
// ABOUTME: CLI entry point for bbs
// ABOUTME: Initializes and executes root command

package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 3: Verify it compiles (will fail - rootCmd not defined yet)**

Run: `go build ./cmd/bbs`
Expected: Error about rootCmd undefined (this is fine)

**Step 4: Commit**

```bash
git add go.mod cmd/
git commit -m "feat: initialize go module and main entry point"
```

---

## Task 2: Create Root Command

**Files:**
- Create: `cmd/bbs/root.go`

**Step 1: Create root.go**

```go
// ABOUTME: Root Cobra command and global flags
// ABOUTME: Sets up CLI structure and database connection

package main

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	dbPath   string
	dbConn   *sql.DB
	identity string
)

var rootCmd = &cobra.Command{
	Use:   "bbs",
	Short: "A lightweight message board for humans and agents",
	Long: `
██████╗ ██████╗ ███████╗
██╔══██╗██╔══██╗██╔════╝
██████╔╝██████╔╝███████╗
██╔══██╗██╔══██╗╚════██║
██████╔╝██████╔╝███████║
╚═════╝ ╚═════╝ ╚══════╝

   THUNDERBOARD 3000

A message board for humans and agents to communicate.
Topics → Threads → Messages`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Database init will be added in Task 4
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if dbConn != nil {
			return dbConn.Close()
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "database file path")
	rootCmd.PersistentFlags().StringVar(&identity, "as", "", "identity override (username)")
}
```

**Step 2: Add cobra dependency**

Run: `go get github.com/spf13/cobra`

**Step 3: Verify it compiles**

Run: `go build ./cmd/bbs`
Expected: Success

**Step 4: Run help**

Run: `go run ./cmd/bbs --help`
Expected: Shows banner and help text

**Step 5: Commit**

```bash
git add cmd/bbs/root.go go.mod go.sum
git commit -m "feat: add root cobra command with CLI skeleton"
```

---

## Task 3: Create Data Models

**Files:**
- Create: `internal/models/models.go`
- Create: `internal/models/models_test.go`

**Step 1: Write failing test**

```go
// ABOUTME: Tests for BBS data models
// ABOUTME: Verifies model creation and validation

package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewTopic(t *testing.T) {
	topic := NewTopic("general", "General discussion", "harper@cli")

	if topic.ID == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
	if topic.Name != "general" {
		t.Errorf("expected name 'general', got '%s'", topic.Name)
	}
	if topic.CreatedBy != "harper@cli" {
		t.Errorf("expected createdBy 'harper@cli', got '%s'", topic.CreatedBy)
	}
	if topic.Archived {
		t.Error("expected archived to be false")
	}
}

func TestNewThread(t *testing.T) {
	topicID := uuid.New()
	thread := NewThread(topicID, "Test subject", "harper@cli")

	if thread.ID == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
	if thread.TopicID != topicID {
		t.Error("expected topicID to match")
	}
	if thread.Subject != "Test subject" {
		t.Errorf("expected subject 'Test subject', got '%s'", thread.Subject)
	}
}

func TestNewMessage(t *testing.T) {
	threadID := uuid.New()
	msg := NewMessage(threadID, "Hello world", "claude@mcp")

	if msg.ID == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
	if msg.Content != "Hello world" {
		t.Errorf("expected content 'Hello world', got '%s'", msg.Content)
	}
	if msg.CreatedBy != "claude@mcp" {
		t.Errorf("expected createdBy 'claude@mcp', got '%s'", msg.CreatedBy)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go get github.com/google/uuid && go test ./internal/models/...`
Expected: FAIL (models.go doesn't exist)

**Step 3: Write models implementation**

```go
// ABOUTME: Core data models for topics, threads, messages, attachments
// ABOUTME: Provides constructor functions for each model type

package models

import (
	"time"

	"github.com/google/uuid"
)

// Topic represents a message board category.
type Topic struct {
	ID          uuid.UUID
	Name        string
	Description string
	CreatedAt   time.Time
	CreatedBy   string
	Archived    bool
}

// Thread represents a discussion within a topic.
type Thread struct {
	ID        uuid.UUID
	TopicID   uuid.UUID
	Subject   string
	CreatedAt time.Time
	CreatedBy string
	Sticky    bool
}

// Message represents a post within a thread.
type Message struct {
	ID        uuid.UUID
	ThreadID  uuid.UUID
	Content   string
	CreatedAt time.Time
	CreatedBy string
	EditedAt  *time.Time
}

// Attachment represents a file attached to a message.
type Attachment struct {
	ID        uuid.UUID
	MessageID uuid.UUID
	Filename  string
	MimeType  string
	Data      []byte
	CreatedAt time.Time
}

// NewTopic creates a new topic with generated UUID and timestamp.
func NewTopic(name, description, createdBy string) *Topic {
	return &Topic{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		CreatedBy:   createdBy,
		Archived:    false,
	}
}

// NewThread creates a new thread with generated UUID and timestamp.
func NewThread(topicID uuid.UUID, subject, createdBy string) *Thread {
	return &Thread{
		ID:        uuid.New(),
		TopicID:   topicID,
		Subject:   subject,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
		Sticky:    false,
	}
}

// NewMessage creates a new message with generated UUID and timestamp.
func NewMessage(threadID uuid.UUID, content, createdBy string) *Message {
	return &Message{
		ID:        uuid.New(),
		ThreadID:  threadID,
		Content:   content,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
	}
}

// NewAttachment creates a new attachment with generated UUID and timestamp.
func NewAttachment(messageID uuid.UUID, filename, mimeType string, data []byte) *Attachment {
	return &Attachment{
		ID:        uuid.New(),
		MessageID: messageID,
		Filename:  filename,
		MimeType:  mimeType,
		Data:      data,
		CreatedAt: time.Now(),
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/models/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/models/
git commit -m "feat: add data models for topic, thread, message, attachment"
```

---

## Task 4: Create Database Layer - Connection

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/db_test.go`

**Step 1: Write failing test**

```go
// ABOUTME: Tests for database initialization
// ABOUTME: Verifies connection and migration execution

package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDB(t *testing.T) {
	// Use temp directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	tables := []string{"topics", "threads", "messages", "attachments"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestGetDefaultDBPath(t *testing.T) {
	path := GetDefaultDBPath()
	if path == "" {
		t.Error("expected non-empty path")
	}
	if filepath.Base(path) != "bbs.db" {
		t.Errorf("expected bbs.db, got %s", filepath.Base(path))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go get modernc.org/sqlite && go test ./internal/db/...`
Expected: FAIL (db.go doesn't exist)

**Step 3: Write db.go**

```go
// ABOUTME: Database connection management and initialization
// ABOUTME: Handles SQLite connection and schema creation

package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// InitDB initializes the database connection and creates schema.
func InitDB(dbPath string) (*sql.DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create schema
	if err := createSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return db, nil
}

// GetDefaultDBPath returns the default database path following XDG standards.
func GetDefaultDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		dataDir = filepath.Join(homeDir, ".local", "share")
	}

	return filepath.Join(dataDir, "bbs", "bbs.db")
}

func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS topics (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		description TEXT DEFAULT '',
		created_at DATETIME NOT NULL,
		created_by TEXT NOT NULL,
		archived BOOLEAN DEFAULT FALSE
	);

	CREATE TABLE IF NOT EXISTS threads (
		id TEXT PRIMARY KEY,
		topic_id TEXT NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
		subject TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		created_by TEXT NOT NULL,
		sticky BOOLEAN DEFAULT FALSE
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

	CREATE INDEX IF NOT EXISTS idx_threads_topic ON threads(topic_id);
	CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(thread_id);
	CREATE INDEX IF NOT EXISTS idx_attachments_message ON attachments(message_id);
	`

	_, err := db.Exec(schema)
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/db/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/
git commit -m "feat: add database layer with schema creation"
```

---

## Task 5: Wire Database to Root Command

**Files:**
- Modify: `cmd/bbs/root.go`

**Step 1: Update root.go to use database**

Replace the PersistentPreRunE in `cmd/bbs/root.go`:

```go
// ABOUTME: Root Cobra command and global flags
// ABOUTME: Sets up CLI structure and database connection

package main

import (
	"database/sql"
	"fmt"

	"github.com/harper/bbs/internal/db"
	"github.com/spf13/cobra"
)

var (
	dbPath   string
	dbConn   *sql.DB
	identity string
)

var rootCmd = &cobra.Command{
	Use:   "bbs",
	Short: "A lightweight message board for humans and agents",
	Long: `
██████╗ ██████╗ ███████╗
██╔══██╗██╔══██╗██╔════╝
██████╔╝██████╔╝███████╗
██╔══██╗██╔══██╗╚════██║
██████╔╝██████╔╝███████║
╚═════╝ ╚═════╝ ╚══════╝

   THUNDERBOARD 3000

A message board for humans and agents to communicate.
Topics → Threads → Messages`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB init for help commands
		if cmd.Name() == "help" || cmd.Name() == "version" {
			return nil
		}

		// Use default path if not specified
		path := dbPath
		if path == "" {
			path = db.GetDefaultDBPath()
		}

		var err error
		dbConn, err = db.InitDB(path)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if dbConn != nil {
			return dbConn.Close()
		}
		return nil
	},
}

func init() {
	defaultPath := db.GetDefaultDBPath()
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultPath, "database file path")
	rootCmd.PersistentFlags().StringVar(&identity, "as", "", "identity override (username)")
}
```

**Step 2: Build and verify**

Run: `go build ./cmd/bbs && ./cmd/bbs/bbs --help`
Expected: Shows help (no errors)

**Step 3: Commit**

```bash
git add cmd/bbs/root.go
git commit -m "feat: wire database initialization to root command"
```

---

## Task 6: Add Identity Helper

**Files:**
- Create: `internal/identity/identity.go`
- Create: `internal/identity/identity_test.go`

**Step 1: Write failing test**

```go
// ABOUTME: Tests for identity resolution
// ABOUTME: Verifies username@source format handling

package identity

import (
	"os"
	"testing"
)

func TestGetIdentity(t *testing.T) {
	tests := []struct {
		name     string
		override string
		source   string
		want     string
	}{
		{"with override", "mybot", "cli", "mybot@cli"},
		{"without override", "", "mcp", os.Getenv("USER") + "@mcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetIdentity(tt.override, tt.source)
			if got != tt.want {
				t.Errorf("GetIdentity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseIdentity(t *testing.T) {
	user, source := ParseIdentity("harper@cli")
	if user != "harper" {
		t.Errorf("expected user 'harper', got '%s'", user)
	}
	if source != "cli" {
		t.Errorf("expected source 'cli', got '%s'", source)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/...`
Expected: FAIL

**Step 3: Write identity.go**

```go
// ABOUTME: Identity resolution for BBS users
// ABOUTME: Handles username@source format

package identity

import (
	"os"
	"strings"
)

// GetIdentity returns the identity string for a user.
// If override is provided, uses that as username.
// Otherwise uses $USER or $BBS_USER environment variable.
func GetIdentity(override, source string) string {
	username := override
	if username == "" {
		username = os.Getenv("BBS_USER")
	}
	if username == "" {
		username = os.Getenv("USER")
	}
	if username == "" {
		username = "anonymous"
	}
	return username + "@" + source
}

// ParseIdentity splits an identity string into username and source.
func ParseIdentity(id string) (username, source string) {
	parts := strings.SplitN(id, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return id, "unknown"
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/identity/
git commit -m "feat: add identity helper for username@source format"
```

---

## Phase 1 Complete Checklist

After completing all tasks:

- [ ] `go build ./cmd/bbs` succeeds
- [ ] `go test ./...` passes
- [ ] `./cmd/bbs/bbs --help` shows banner
- [ ] Database created at XDG path on first run

---

## Next Phase

Phase 2 will add:
- Topic CRUD (db layer + CLI commands)
- Thread CRUD (db layer + CLI commands)
- Message CRUD (db layer + CLI commands)
