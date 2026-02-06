// ABOUTME: Core MarkdownStore struct and helpers for file-based BBS storage
// ABOUTME: Provides constructor, atomic writes, slug generation, and frontmatter parsing

package storage

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/harper/bbs/internal/models"
)

// MarkdownStore provides file-based storage for BBS data using markdown files and YAML.
type MarkdownStore struct {
	dataDir string
}

// Compile-time check that MarkdownStore implements Storage.
var _ Storage = (*MarkdownStore)(nil)

// NewMarkdownStore creates a new markdown-backed store rooted at dataDir.
func NewMarkdownStore(dataDir string) (*MarkdownStore, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	return &MarkdownStore{dataDir: dataDir}, nil
}

// Close releases resources. For MarkdownStore this is a no-op.
func (s *MarkdownStore) Close() error {
	return nil
}

// topicsFilePath returns the path to the _topics.yaml file.
func (s *MarkdownStore) topicsFilePath() string {
	return filepath.Join(s.dataDir, "_topics.yaml")
}

// topicDirPath returns the directory path for a topic.
func (s *MarkdownStore) topicDirPath(topicName string) string {
	return filepath.Join(s.dataDir, topicName)
}

// threadFilePath returns the path to a thread's markdown file.
// It searches the topic directory for a file whose frontmatter matches the thread ID.
func (s *MarkdownStore) threadFilePath(topicName string, threadID uuid.UUID) (string, error) {
	topicDir := s.topicDirPath(topicName)
	entries, err := os.ReadDir(topicDir)
	if err != nil {
		return "", fmt.Errorf("read topic directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		fp := filepath.Join(topicDir, entry.Name())
		fm, err := readThreadFrontmatter(fp)
		if err != nil {
			continue
		}
		if fm.ID == threadID.String() {
			return fp, nil
		}
	}
	return "", fmt.Errorf("thread file not found: %s", threadID)
}

// attachmentDirPath returns the directory path for attachments of a specific message.
func (s *MarkdownStore) attachmentDirPath(topicName string, msgIDPrefix string) string {
	return filepath.Join(s.topicDirPath(topicName), "_attachments", msgIDPrefix)
}

// atomicWrite writes data to a file atomically by writing to a temp file then renaming.
// Files are created with 0640 permissions (owner rw, group r).
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	suffix, err := randomSuffix()
	if err != nil {
		return fmt.Errorf("generate temp suffix: %w", err)
	}
	tmpPath := path + ".tmp." + suffix

	if err := os.WriteFile(tmpPath, data, 0640); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// randomSuffix generates a short random string for temp file names.
func randomSuffix() (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 8)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result[i] = chars[n.Int64()]
	}
	return string(result), nil
}

// slugify converts a string to a URL-friendly slug.
func slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace non-alphanumeric chars with hyphens
	var result []rune
	prevHyphen := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result = append(result, r)
			prevHyphen = false
		} else if !prevHyphen && len(result) > 0 {
			result = append(result, '-')
			prevHyphen = true
		}
	}

	slug := strings.TrimRight(string(result), "-")
	if slug == "" {
		slug = "thread"
	}
	return slug
}

// threadFileName generates a unique filename for a thread.
// Uses slugified subject, adding UUID suffix on collision.
func (s *MarkdownStore) threadFileName(topicName, subject string, threadID uuid.UUID) string {
	slug := slugify(subject)
	base := slug + ".md"
	topicDir := s.topicDirPath(topicName)

	path := filepath.Join(topicDir, base)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return base
	}

	// Check if this file belongs to the same thread (e.g., during update)
	fm, err := readThreadFrontmatter(path)
	if err == nil && fm.ID == threadID.String() {
		return base
	}

	// Collision: add UUID prefix
	return slug + "-" + threadID.String()[:8] + ".md"
}

// threadFrontmatter holds the YAML frontmatter of a thread file.
type threadFrontmatter struct {
	ID        string `yaml:"id"`
	Topic     string `yaml:"topic"`
	Subject   string `yaml:"subject"`
	CreatedAt string `yaml:"created_at"`
	CreatedBy string `yaml:"created_by"`
	Sticky    bool   `yaml:"sticky"`
}

// parsedMessage holds a message parsed from a thread markdown file.
type parsedMessage struct {
	ID        uuid.UUID
	CreatedBy string
	CreatedAt time.Time
	EditedAt  *time.Time
	Content   string
}

// readThreadFrontmatter reads just the frontmatter from a thread file.
func readThreadFrontmatter(path string) (*threadFrontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseThreadFrontmatter(string(data))
}

// parseThreadFrontmatter extracts the YAML frontmatter from markdown content.
func parseThreadFrontmatter(content string) (*threadFrontmatter, error) {
	// Frontmatter is between the first two "---" lines
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("no frontmatter found")
	}

	end := strings.Index(content[4:], "\n---\n")
	if end == -1 {
		// Try with just "---" at end of file
		end = strings.Index(content[4:], "\n---")
		if end == -1 {
			return nil, fmt.Errorf("frontmatter not terminated")
		}
	}

	yamlContent := content[4 : 4+end]
	var fm threadFrontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	return &fm, nil
}

// msgIDRegexp matches the message ID comment: <!-- msg:UUID -->
var msgIDRegexp = regexp.MustCompile(`<!-- msg:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}) -->`)

// msgHeaderRegexp matches the message header: ## author — timestamp (with optional fractional seconds)
var msgHeaderRegexp = regexp.MustCompile(`^## (.+?) — (\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)$`)

// editedAtRegexp matches the edited marker: <!-- edited:timestamp --> (with optional fractional seconds)
var editedAtRegexp = regexp.MustCompile(`<!-- edited:(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z) -->`)

// parseThreadMessages parses messages from the body of a thread markdown file (after frontmatter).
// Malformed message sections are silently skipped.
func parseThreadMessages(content string) []*parsedMessage {
	// Split off frontmatter
	body := stripFrontmatter(content)
	if body == "" {
		return nil
	}

	// Split on message boundaries (## headers preceded by ---)
	// Messages are separated by "\n\n---\n\n"
	sections := splitMessages(body)

	var messages []*parsedMessage
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		msg, err := parseMessageSection(section)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	return messages
}

// stripFrontmatter removes the YAML frontmatter from markdown content.
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	end := strings.Index(content[4:], "\n---\n")
	if end == -1 {
		end = strings.Index(content[4:], "\n---")
		if end == -1 {
			return ""
		}
		return strings.TrimSpace(content[4+end+4:])
	}
	return strings.TrimSpace(content[4+end+5:])
}

// splitMessages splits the message body into individual message sections.
func splitMessages(body string) []string {
	// Messages are separated by "\n\n---\n\n" but the first message just starts with ##
	parts := strings.Split(body, "\n---\n")
	return parts
}

// parseMessageHeader extracts author and timestamp from a message header line.
func parseMessageHeader(line string) (author string, createdAt time.Time, ok bool) {
	matches := msgHeaderRegexp.FindStringSubmatch(line)
	if matches == nil {
		return "", time.Time{}, false
	}
	t, err := parseTimestamp(matches[2])
	if err != nil {
		return "", time.Time{}, false
	}
	return matches[1], t, true
}

// parseMessageID extracts a message UUID from an ID comment line.
func parseMessageID(line string) (uuid.UUID, bool) {
	matches := msgIDRegexp.FindStringSubmatch(line)
	if matches == nil {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(matches[1])
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}

// parseEditedAt extracts an edited timestamp from an edited marker line.
func parseEditedAt(line string) (*time.Time, bool) {
	matches := editedAtRegexp.FindStringSubmatch(line)
	if matches == nil {
		return nil, false
	}
	t, err := parseTimestamp(matches[1])
	if err != nil {
		return nil, false
	}
	return &t, true
}

// parseMessageSection parses a single message section.
func parseMessageSection(section string) (*parsedMessage, error) {
	lines := strings.Split(section, "\n")

	var msg parsedMessage
	headerFound := false
	idFound := false
	var contentLines []string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" && !headerFound {
			continue
		}

		if !headerFound {
			author, createdAt, ok := parseMessageHeader(trimmed)
			if ok {
				msg.CreatedBy = author
				msg.CreatedAt = createdAt
				headerFound = true
				continue
			}
		}

		if headerFound && !idFound {
			id, ok := parseMessageID(trimmed)
			if ok {
				msg.ID = id
				idFound = true
				continue
			}
		}

		if headerFound {
			if editedAt, ok := parseEditedAt(trimmed); ok {
				msg.EditedAt = editedAt
				continue
			}
		}

		if headerFound && idFound {
			contentLines = append(contentLines, lines[i])
		}
	}

	if !headerFound || !idFound {
		return nil, fmt.Errorf("incomplete message section")
	}

	msg.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
	return &msg, nil
}

// renderThread renders a complete thread file (frontmatter + messages).
func renderThread(thread *models.Thread, topicName string, messages []*parsedMessage) string {
	var b strings.Builder

	// Frontmatter
	b.WriteString("---\n")
	fm := threadFrontmatter{
		ID:        thread.ID.String(),
		Topic:     topicName,
		Subject:   thread.Subject,
		CreatedAt: thread.CreatedAt.UTC().Format(time.RFC3339Nano),
		CreatedBy: thread.CreatedBy,
		Sticky:    thread.Sticky,
	}
	fmData, _ := yaml.Marshal(&fm)
	b.Write(fmData)
	b.WriteString("---\n")

	// Messages
	for i, msg := range messages {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		b.WriteString("\n")
		b.WriteString(renderMessage(msg))
	}

	return b.String()
}

// renderMessage renders a single message section.
func renderMessage(msg *parsedMessage) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("## %s — %s\n", msg.CreatedBy, msg.CreatedAt.UTC().Format(time.RFC3339Nano)))
	b.WriteString(fmt.Sprintf("<!-- msg:%s -->\n", msg.ID.String()))

	if msg.EditedAt != nil {
		b.WriteString(fmt.Sprintf("<!-- edited:%s -->\n", msg.EditedAt.UTC().Format(time.RFC3339Nano)))
	}

	b.WriteString("\n")
	b.WriteString(msg.Content)
	b.WriteString("\n")

	return b.String()
}

// topicEntry represents a single topic in the _topics.yaml file.
type topicEntry struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	CreatedAt   string `yaml:"created_at"`
	CreatedBy   string `yaml:"created_by"`
	Archived    bool   `yaml:"archived"`
}

// toModel converts a topicEntry to a models.Topic.
// Returns an error if the entry contains malformed data (e.g., invalid UUID or timestamp).
func (e *topicEntry) toModel() (*models.Topic, error) {
	id, err := uuid.Parse(e.ID)
	if err != nil {
		return nil, fmt.Errorf("parse topic ID %q: %w", e.ID, err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, e.CreatedAt)
	if err != nil {
		createdAt, err = time.Parse(time.RFC3339, e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse topic created_at %q: %w", e.CreatedAt, err)
		}
	}
	return &models.Topic{
		ID:          id,
		Name:        e.Name,
		Description: e.Description,
		CreatedAt:   createdAt,
		CreatedBy:   e.CreatedBy,
		Archived:    e.Archived,
	}, nil
}

// fromTopicModel converts a models.Topic to a topicEntry.
func fromTopicModel(t *models.Topic) topicEntry {
	return topicEntry{
		ID:          t.ID.String(),
		Name:        t.Name,
		Description: t.Description,
		CreatedAt:   t.CreatedAt.UTC().Format(time.RFC3339Nano),
		CreatedBy:   t.CreatedBy,
		Archived:    t.Archived,
	}
}

// readTopics reads the _topics.yaml file.
func (s *MarkdownStore) readTopics() ([]topicEntry, error) {
	data, err := os.ReadFile(s.topicsFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read topics file: %w", err)
	}

	var entries []topicEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse topics file: %w", err)
	}
	return entries, nil
}

// writeTopics writes the _topics.yaml file atomically.
func (s *MarkdownStore) writeTopics(entries []topicEntry) error {
	data, err := yaml.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal topics: %w", err)
	}
	return atomicWrite(s.topicsFilePath(), data)
}

// parseTimestamp parses a timestamp string trying RFC3339Nano first, then RFC3339.
func parseTimestamp(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	return t, err
}

// attachmentMeta holds metadata for an attachment stored alongside the file.
type attachmentMeta struct {
	ID        string `yaml:"id"`
	MessageID string `yaml:"message_id"`
	Filename  string `yaml:"filename"`
	MimeType  string `yaml:"mime_type"`
	CreatedAt string `yaml:"created_at"`
}
