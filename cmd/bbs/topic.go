// ABOUTME: Topic CLI commands
// ABOUTME: Implements topic list, new, archive, show subcommands

package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/harper/bbs/internal/charm"
	"github.com/harper/bbs/internal/identity"
	"github.com/harper/bbs/internal/models"
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
	client, err := charm.Global()
	if err != nil {
		return err
	}

	topics, err := client.ListTopics(showArchived)
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
	client, err := charm.Global()
	if err != nil {
		return err
	}

	name := args[0]
	description := ""
	if len(args) > 1 {
		description = args[1]
	}

	id := identity.GetIdentity(identityFlag, "cli")
	topic := models.NewTopic(name, description, id)

	if err := client.CreateTopic(topic); err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	color.Green("Created topic: %s", name)
	fmt.Printf("ID: %s\n", topic.ID.String()[:8])
	return nil
}

func runTopicArchive(cmd *cobra.Command, args []string) error {
	client, err := charm.Global()
	if err != nil {
		return err
	}

	topic, err := client.ResolveTopic(args[0])
	if err != nil {
		return err
	}

	archived := !unarchive
	if err := client.ArchiveTopic(topic.ID, archived); err != nil {
		return err
	}

	if archived {
		color.Yellow("Archived topic: %s", args[0])
	} else {
		color.Green("Unarchived topic: %s", args[0])
	}
	return nil
}

func runTopicShow(cmd *cobra.Command, args []string) error {
	client, err := charm.Global()
	if err != nil {
		return err
	}

	topic, err := client.ResolveTopic(args[0])
	if err != nil {
		return fmt.Errorf("topic not found: %s", args[0])
	}

	fmt.Printf("Topic: %s\n", topic.Name)
	fmt.Printf("Description: %s\n", topic.Description)
	fmt.Printf("Created by: %s\n", topic.CreatedBy)
	fmt.Printf("Created at: %s\n", topic.CreatedAt.Format("2006-01-02 15:04"))
	if topic.Archived {
		color.Yellow("Status: Archived")
	}

	// Show recent threads
	threads, _ := client.ListThreads(topic.ID)
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
