// ABOUTME: Thread CLI commands
// ABOUTME: Implements thread list, new, show, sticky subcommands

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

var threadCmd = &cobra.Command{
	Use:   "thread",
	Short: "Manage threads",
	Long:  "Create, list, and view threads within topics.",
}

var threadListCmd = &cobra.Command{
	Use:   "list <topic>",
	Short: "List threads in a topic",
	Args:  cobra.ExactArgs(1),
	RunE:  runThreadList,
}

var threadNewCmd = &cobra.Command{
	Use:   "new <topic> <subject>",
	Short: "Create a new thread",
	Args:  cobra.ExactArgs(2),
	RunE:  runThreadNew,
}

var threadShowCmd = &cobra.Command{
	Use:   "show <thread>",
	Short: "Show thread with messages",
	Args:  cobra.ExactArgs(1),
	RunE:  runThreadShow,
}

var threadStickyCmd = &cobra.Command{
	Use:   "sticky <thread>",
	Short: "Pin/unpin a thread",
	Args:  cobra.ExactArgs(1),
	RunE:  runThreadSticky,
}

var unsticky bool

func init() {
	rootCmd.AddCommand(threadCmd)
	threadCmd.AddCommand(threadListCmd, threadNewCmd, threadShowCmd, threadStickyCmd)

	threadStickyCmd.Flags().BoolVar(&unsticky, "unpin", false, "unpin instead of pin")
}

func runThreadList(cmd *cobra.Command, args []string) error {
	client, err := charm.Global()
	if err != nil {
		return err
	}

	topic, err := client.ResolveTopic(args[0])
	if err != nil {
		return err
	}

	threads, err := client.ListThreads(topic.ID)
	if err != nil {
		return err
	}

	if len(threads) == 0 {
		fmt.Println("No threads found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SUBJECT\tCREATED BY\tDATE")
	for _, t := range threads {
		prefix := ""
		if t.Sticky {
			prefix = "📌 "
		}
		fmt.Fprintf(w, "%s%s\t%s\t%s\n", prefix, t.Subject, t.CreatedBy, t.CreatedAt.Format("Jan 02"))
	}
	return w.Flush()
}

func runThreadNew(cmd *cobra.Command, args []string) error {
	client, err := charm.Global()
	if err != nil {
		return err
	}

	topic, err := client.ResolveTopic(args[0])
	if err != nil {
		return err
	}

	id := identity.GetIdentity(identityFlag, "cli")
	thread := models.NewThread(topic.ID, args[1], id)

	if err := client.CreateThread(thread); err != nil {
		return fmt.Errorf("failed to create thread: %w", err)
	}

	color.Green("Created thread: %s", args[1])
	fmt.Printf("ID: %s\n", thread.ID.String()[:8])
	return nil
}

func runThreadShow(cmd *cobra.Command, args []string) error {
	client, err := charm.Global()
	if err != nil {
		return err
	}

	thread, err := client.ResolveThread(args[0])
	if err != nil {
		return fmt.Errorf("thread not found: %s", args[0])
	}

	if thread.Sticky {
		fmt.Print("📌 ")
	}
	fmt.Printf("%s\n", thread.Subject)
	faint := color.New(color.Faint)
	faint.Printf("by %s on %s\n\n", thread.CreatedBy, thread.CreatedAt.Format("2006-01-02 15:04"))

	messages, err := client.ListMessages(thread.ID)
	if err != nil {
		return err
	}

	for _, msg := range messages {
		fmt.Printf("─────────────────────────────────\n")
		faint.Printf("%s · %s", msg.CreatedBy, msg.CreatedAt.Format("Jan 02 15:04"))
		if msg.EditedAt != nil {
			faint.Printf(" (edited)")
		}
		fmt.Println()
		fmt.Println(msg.Content)
		fmt.Println()
	}

	if len(messages) == 0 {
		fmt.Println("No messages yet.")
	}

	return nil
}

func runThreadSticky(cmd *cobra.Command, args []string) error {
	client, err := charm.Global()
	if err != nil {
		return err
	}

	thread, err := client.ResolveThread(args[0])
	if err != nil {
		return err
	}

	sticky := !unsticky
	if err := client.SetThreadSticky(thread.ID, sticky); err != nil {
		return err
	}

	if sticky {
		color.Green("📌 Pinned thread")
	} else {
		color.Yellow("Unpinned thread")
	}
	return nil
}
