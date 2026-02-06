// ABOUTME: Export and import commands for BBS data
// ABOUTME: Supports markdown, yaml, and json formats

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/harper/bbs/internal/config"
	"github.com/harper/bbs/internal/models"
	"github.com/harper/bbs/internal/storage"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export BBS data",
	Long: `Export BBS data in various formats.

Formats:
  markdown  - Human-readable markdown
  yaml      - Structured YAML backup
  json      - JSON backup`,
}

var exportMarkdownCmd = &cobra.Command{
	Use:   "markdown [path]",
	Short: "Export as markdown",
	Long:  "Export all BBS data as human-readable markdown.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runExportMarkdown,
}

var exportYAMLCmd = &cobra.Command{
	Use:   "yaml [path]",
	Short: "Export as YAML",
	Long:  "Export all BBS data as structured YAML.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runExportYAML,
}

var exportJSONCmd = &cobra.Command{
	Use:   "json [path]",
	Short: "Export as JSON",
	Long:  "Export all BBS data as JSON.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runExportJSON,
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import BBS data",
	Long:  "Import BBS data from various formats.",
}

var importYAMLCmd = &cobra.Command{
	Use:   "yaml <path>",
	Short: "Import from YAML",
	Long:  "Import BBS data from a YAML backup file.",
	Args:  cobra.ExactArgs(1),
	RunE:  runImportYAML,
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.AddCommand(exportMarkdownCmd, exportYAMLCmd, exportJSONCmd)

	rootCmd.AddCommand(importCmd)
	importCmd.AddCommand(importYAMLCmd)
}

// Export data structures
type ExportData struct {
	Version    string        `json:"version" yaml:"version"`
	ExportedAt time.Time     `json:"exported_at" yaml:"exported_at"`
	Tool       string        `json:"tool" yaml:"tool"`
	Topics     []ExportTopic `json:"topics" yaml:"topics"`
}

type ExportTopic struct {
	ID          string         `json:"id" yaml:"id"`
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at" yaml:"created_at"`
	CreatedBy   string         `json:"created_by" yaml:"created_by"`
	Archived    bool           `json:"archived,omitempty" yaml:"archived,omitempty"`
	Threads     []ExportThread `json:"threads" yaml:"threads"`
}

type ExportThread struct {
	ID        string          `json:"id" yaml:"id"`
	Subject   string          `json:"subject" yaml:"subject"`
	CreatedAt time.Time       `json:"created_at" yaml:"created_at"`
	CreatedBy string          `json:"created_by" yaml:"created_by"`
	UpdatedAt time.Time       `json:"updated_at" yaml:"updated_at"`
	Sticky    bool            `json:"sticky,omitempty" yaml:"sticky,omitempty"`
	Messages  []ExportMessage `json:"messages" yaml:"messages"`
}

type ExportMessage struct {
	ID          string             `json:"id" yaml:"id"`
	Content     string             `json:"content" yaml:"content"`
	CreatedAt   time.Time          `json:"created_at" yaml:"created_at"`
	CreatedBy   string             `json:"created_by" yaml:"created_by"`
	EditedAt    *time.Time         `json:"edited_at,omitempty" yaml:"edited_at,omitempty"`
	Attachments []ExportAttachment `json:"attachments,omitempty" yaml:"attachments,omitempty"`
}

type ExportAttachment struct {
	ID        string    `json:"id" yaml:"id"`
	Filename  string    `json:"filename" yaml:"filename"`
	MimeType  string    `json:"mime_type" yaml:"mime_type"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// Data is base64 encoded in JSON, raw bytes in YAML
	Data []byte `json:"data" yaml:"data"`
}

func buildExportData(store storage.Storage) (*ExportData, error) {
	topics, err := store.ListTopics(true)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}

	// Sort topics by name
	sort.Slice(topics, func(i, j int) bool {
		return topics[i].Name < topics[j].Name
	})

	exportTopics := make([]ExportTopic, 0, len(topics))
	for _, topic := range topics {
		threads, err := store.ListThreads(topic.ID)
		if err != nil {
			return nil, fmt.Errorf("list threads for topic %s: %w", topic.Name, err)
		}

		exportThreads := make([]ExportThread, 0, len(threads))
		for _, thread := range threads {
			messages, err := store.ListMessages(thread.ID)
			if err != nil {
				return nil, fmt.Errorf("list messages for thread %s: %w", thread.Subject, err)
			}

			exportMessages := make([]ExportMessage, 0, len(messages))
			for _, msg := range messages {
				attachments, err := store.ListAttachments(msg.ID)
				if err != nil {
					return nil, fmt.Errorf("list attachments for message %s: %w", msg.ID, err)
				}

				exportAttachments := make([]ExportAttachment, 0, len(attachments))
				for _, att := range attachments {
					exportAttachments = append(exportAttachments, ExportAttachment{
						ID:        att.ID.String(),
						Filename:  att.Filename,
						MimeType:  att.MimeType,
						CreatedAt: att.CreatedAt,
						Data:      att.Data,
					})
				}

				exportMessages = append(exportMessages, ExportMessage{
					ID:          msg.ID.String(),
					Content:     msg.Content,
					CreatedAt:   msg.CreatedAt,
					CreatedBy:   msg.CreatedBy,
					EditedAt:    msg.EditedAt,
					Attachments: exportAttachments,
				})
			}

			exportThreads = append(exportThreads, ExportThread{
				ID:        thread.ID.String(),
				Subject:   thread.Subject,
				CreatedAt: thread.CreatedAt,
				CreatedBy: thread.CreatedBy,
				UpdatedAt: thread.UpdatedAt,
				Sticky:    thread.Sticky,
				Messages:  exportMessages,
			})
		}

		exportTopics = append(exportTopics, ExportTopic{
			ID:          topic.ID.String(),
			Name:        topic.Name,
			Description: topic.Description,
			CreatedAt:   topic.CreatedAt,
			CreatedBy:   topic.CreatedBy,
			Archived:    topic.Archived,
			Threads:     exportThreads,
		})
	}

	return &ExportData{
		Version:    "1.0",
		ExportedAt: time.Now().UTC(),
		Tool:       "bbs",
		Topics:     exportTopics,
	}, nil
}

func runExportMarkdown(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	store, err := cfg.OpenStorage()
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	data, err := buildExportData(store)
	if err != nil {
		return err
	}

	var output string
	output += fmt.Sprintf("# BBS Export - %s\n\n", time.Now().Format("2006-01-02"))
	output += fmt.Sprintf("Generated: %s\n\n", data.ExportedAt.Format(time.RFC3339))

	for _, topic := range data.Topics {
		output += fmt.Sprintf("# Topic: %s\n", topic.Name)
		if topic.Description != "" {
			output += fmt.Sprintf("%s\n", topic.Description)
		}
		if topic.Archived {
			output += "*Archived*\n"
		}
		output += "\n"

		for _, thread := range topic.Threads {
			prefix := ""
			if thread.Sticky {
				prefix = "[PINNED] "
			}
			output += fmt.Sprintf("## %s%s\n", prefix, thread.Subject)
			output += fmt.Sprintf("*by %s on %s*\n\n", thread.CreatedBy, thread.CreatedAt.Format("2006-01-02"))

			for _, msg := range thread.Messages {
				output += fmt.Sprintf("### %s - %s\n", msg.CreatedBy, msg.CreatedAt.Format("2006-01-02 15:04"))
				if msg.EditedAt != nil {
					output += fmt.Sprintf("*(edited %s)*\n", msg.EditedAt.Format("2006-01-02 15:04"))
				}
				output += fmt.Sprintf("\n%s\n\n", msg.Content)

				if len(msg.Attachments) > 0 {
					output += "Attachments:\n"
					for _, att := range msg.Attachments {
						output += fmt.Sprintf("- %s (%s)\n", att.Filename, att.MimeType)
					}
					output += "\n"
				}
			}
		}
		output += "\n---\n\n"
	}

	return writeOutput(args, output, "bbs-export.md")
}

func runExportYAML(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	store, err := cfg.OpenStorage()
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	data, err := buildExportData(store)
	if err != nil {
		return err
	}

	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	return writeOutput(args, string(yamlData), "bbs-export.yaml")
}

func runExportJSON(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	store, err := cfg.OpenStorage()
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	data, err := buildExportData(store)
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	return writeOutput(args, string(jsonData), "bbs-export.json")
}

func writeOutput(args []string, content, defaultName string) error {
	if len(args) == 0 {
		fmt.Print(content)
		return nil
	}

	path := args[0]
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		path = filepath.Join(path, defaultName)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	color.Green("Exported to: %s", path)
	return nil
}

//nolint:gocognit // Complex but clear import logic with nested data structures
func runImportYAML(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var exportData ExportData
	if err := yaml.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if exportData.Tool != "bbs" {
		return fmt.Errorf("invalid export file: tool is %q, expected %q", exportData.Tool, "bbs")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	store, err := cfg.OpenStorage()
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	var topicsImported, threadsImported, messagesImported int

	for _, topic := range exportData.Topics {
		topicID, err := models.ParseUUID(topic.ID)
		if err != nil {
			return fmt.Errorf("parse topic ID %s: %w", topic.ID, err)
		}

		t := &models.Topic{
			ID:          topicID,
			Name:        topic.Name,
			Description: topic.Description,
			CreatedAt:   topic.CreatedAt,
			CreatedBy:   topic.CreatedBy,
			Archived:    topic.Archived,
		}

		if err := store.CreateTopic(t); err != nil {
			// Topic might already exist, try to continue
			fmt.Printf("Warning: could not create topic %s: %v\n", topic.Name, err)
			continue
		}
		topicsImported++

		for _, thread := range topic.Threads {
			threadID, err := models.ParseUUID(thread.ID)
			if err != nil {
				return fmt.Errorf("parse thread ID %s: %w", thread.ID, err)
			}

			th := &models.Thread{
				ID:        threadID,
				TopicID:   topicID,
				Subject:   thread.Subject,
				CreatedAt: thread.CreatedAt,
				CreatedBy: thread.CreatedBy,
				UpdatedAt: thread.UpdatedAt,
				Sticky:    thread.Sticky,
			}

			if err := store.CreateThread(th); err != nil {
				fmt.Printf("Warning: could not create thread %s: %v\n", thread.Subject, err)
				continue
			}
			threadsImported++

			for _, msg := range thread.Messages {
				msgID, err := models.ParseUUID(msg.ID)
				if err != nil {
					return fmt.Errorf("parse message ID %s: %w", msg.ID, err)
				}

				m := &models.Message{
					ID:        msgID,
					ThreadID:  threadID,
					Content:   msg.Content,
					CreatedAt: msg.CreatedAt,
					CreatedBy: msg.CreatedBy,
					EditedAt:  msg.EditedAt,
				}

				if err := store.CreateMessage(m); err != nil {
					fmt.Printf("Warning: could not create message: %v\n", err)
					continue
				}
				messagesImported++

				for _, att := range msg.Attachments {
					attID, err := models.ParseUUID(att.ID)
					if err != nil {
						return fmt.Errorf("parse attachment ID %s: %w", att.ID, err)
					}

					a := &models.Attachment{
						ID:        attID,
						MessageID: msgID,
						Filename:  att.Filename,
						MimeType:  att.MimeType,
						Data:      att.Data,
						CreatedAt: att.CreatedAt,
					}

					if err := store.CreateAttachment(a); err != nil {
						fmt.Printf("Warning: could not create attachment %s: %v\n", att.Filename, err)
					}
				}
			}
		}
	}

	color.Green("Imported: %d topics, %d threads, %d messages", topicsImported, threadsImported, messagesImported)
	return nil
}
