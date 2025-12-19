// ABOUTME: Post CLI command
// ABOUTME: Implements posting messages to threads

package main

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/harper/bbs/internal/charm"
	"github.com/harper/bbs/internal/identity"
	"github.com/harper/bbs/internal/models"
)

var postCmd = &cobra.Command{
	Use:   "post <thread> <message>",
	Short: "Post a message to a thread",
	Args:  cobra.ExactArgs(2),
	RunE:  runPost,
}

var editCmd = &cobra.Command{
	Use:   "edit <message-id> <new-content>",
	Short: "Edit a message",
	Args:  cobra.ExactArgs(2),
	RunE:  runEdit,
}

func init() {
	rootCmd.AddCommand(postCmd)
	rootCmd.AddCommand(editCmd)
}

func runPost(cmd *cobra.Command, args []string) error {
	client, err := charm.Global()
	if err != nil {
		return err
	}

	thread, err := client.ResolveThread(args[0])
	if err != nil {
		return fmt.Errorf("thread not found: %s", args[0])
	}

	id := identity.GetIdentity(identityFlag, "cli")
	msg := models.NewMessage(thread.ID, args[1], id)

	if err := client.CreateMessage(msg); err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}

	color.Green("Posted to: %s", thread.Subject)
	fmt.Printf("Message ID: %s\n", msg.ID.String()[:8])
	return nil
}

func runEdit(cmd *cobra.Command, args []string) error {
	client, err := charm.Global()
	if err != nil {
		return err
	}

	msg, err := client.ResolveMessage(args[0])
	if err != nil {
		return fmt.Errorf("message not found: %s", args[0])
	}

	msg.Content = args[1]
	now := time.Now()
	msg.EditedAt = &now

	if err := client.UpdateMessage(msg); err != nil {
		return err
	}

	color.Green("Message updated")
	return nil
}
