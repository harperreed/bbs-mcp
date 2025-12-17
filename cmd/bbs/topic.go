// ABOUTME: Topic CLI commands
// ABOUTME: Implements topic list, new, archive, show subcommands

package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/harper/bbs/internal/db"
	"github.com/harper/bbs/internal/identity"
	"github.com/harper/bbs/internal/models"
	"github.com/harper/bbs/internal/sync"
	"github.com/harperreed/sweet/vault"
	"github.com/spf13/cobra"
)

var topicCmd = &cobra.Command{
	Use:   "topic",
	Short: "Manage topics",
	Long:  "Create, list, archive, and view topics on the board.",
}

var topicListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all topics",
	RunE:  runTopicList,
}

var topicNewCmd = &cobra.Command{
	Use:   "new <name> [description]",
	Short: "Create a new topic",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runTopicNew,
}

var topicArchiveCmd = &cobra.Command{
	Use:   "archive <topic>",
	Short: "Archive a topic",
	Args:  cobra.ExactArgs(1),
	RunE:  runTopicArchive,
}

var topicShowCmd = &cobra.Command{
	Use:   "show <topic>",
	Short: "Show topic details",
	Args:  cobra.ExactArgs(1),
	RunE:  runTopicShow,
}

var (
	showArchived bool
	unarchive    bool
)

func init() {
	rootCmd.AddCommand(topicCmd)
	topicCmd.AddCommand(topicListCmd, topicNewCmd, topicArchiveCmd, topicShowCmd)

	topicListCmd.Flags().BoolVar(&showArchived, "archived", false, "show archived topics")
	topicArchiveCmd.Flags().BoolVar(&unarchive, "unarchive", false, "unarchive instead of archive")
}

func runTopicList(cmd *cobra.Command, args []string) error {
	topics, err := db.ListTopics(dbConn, showArchived)
	if err != nil {
		return err
	}

	if len(topics) == 0 {
		fmt.Println("No topics found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tCREATED BY")
	for _, t := range topics {
		fmt.Fprintf(w, "%s\t%s\t%s\n", t.Name, t.Description, t.CreatedBy)
	}
	return w.Flush()
}

func runTopicNew(cmd *cobra.Command, args []string) error {
	name := args[0]
	description := ""
	if len(args) > 1 {
		description = args[1]
	}

	id := identity.GetIdentity(identityFlag, "cli")
	topic := models.NewTopic(name, description, id)

	if err := db.CreateTopic(dbConn, topic); err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	// Queue sync change (best-effort, won't fail if sync not configured)
	if err := sync.TryQueueTopicChange(context.Background(), dbConn, topic, vault.OpUpsert); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to queue sync: %v\n", err)
	}

	color.Green("Created topic: %s", name)
	fmt.Printf("ID: %s\n", topic.ID.String()[:8])
	return nil
}

func runTopicArchive(cmd *cobra.Command, args []string) error {
	topicID, err := db.ResolveTopicID(dbConn, args[0])
	if err != nil {
		return err
	}

	archived := !unarchive
	if err := db.ArchiveTopic(dbConn, topicID, archived); err != nil {
		return err
	}

	// Get updated topic for sync
	topic, err := db.GetTopicByID(dbConn, topicID)
	if err == nil {
		// Queue sync change (best-effort, won't fail if sync not configured)
		if err := sync.TryQueueTopicChange(context.Background(), dbConn, topic, vault.OpUpsert); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to queue sync: %v\n", err)
		}
	}

	if archived {
		color.Yellow("Archived topic: %s", args[0])
	} else {
		color.Green("Unarchived topic: %s", args[0])
	}
	return nil
}

func runTopicShow(cmd *cobra.Command, args []string) error {
	topic, err := db.GetTopicByName(dbConn, args[0])
	if err != nil {
		topic, err = db.GetTopicByID(dbConn, args[0])
		if err != nil {
			return fmt.Errorf("topic not found: %s", args[0])
		}
	}

	fmt.Printf("Topic: %s\n", topic.Name)
	fmt.Printf("Description: %s\n", topic.Description)
	fmt.Printf("Created by: %s\n", topic.CreatedBy)
	fmt.Printf("Created at: %s\n", topic.CreatedAt.Format("2006-01-02 15:04"))
	if topic.Archived {
		color.Yellow("Status: Archived")
	}

	// Show recent threads
	threads, _ := db.ListThreads(dbConn, topic.ID.String())
	if len(threads) > 0 {
		fmt.Printf("\nRecent threads (%d):\n", len(threads))
		for i, t := range threads {
			if i >= 5 {
				fmt.Printf("  ... and %d more\n", len(threads)-5)
				break
			}
			prefix := "  "
			if t.Sticky {
				prefix = "📌"
			}
			fmt.Printf("%s %s (%s)\n", prefix, t.Subject, t.CreatedBy)
		}
	}

	return nil
}
