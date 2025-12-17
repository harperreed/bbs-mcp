// ABOUTME: Post CLI command
// ABOUTME: Implements posting messages to threads

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/harper/bbs/internal/db"
	"github.com/harper/bbs/internal/identity"
	"github.com/harper/bbs/internal/models"
	"github.com/harper/bbs/internal/sync"
	"github.com/harperreed/sweet/vault"
	"github.com/spf13/cobra"
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
	thread, err := db.GetThreadByID(dbConn, args[0])
	if err != nil {
		return fmt.Errorf("thread not found: %s", args[0])
	}

	id := identity.GetIdentity(identityFlag, "cli")
	msg := models.NewMessage(thread.ID, args[1], id)

	if err := db.CreateMessage(dbConn, msg); err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}

	// Queue sync change (best-effort, won't fail if sync not configured)
	if err := sync.TryQueueMessageChange(context.Background(), dbConn, msg, vault.OpUpsert); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to queue sync: %v\n", err)
	}

	color.Green("Posted to: %s", thread.Subject)
	fmt.Printf("Message ID: %s\n", msg.ID.String()[:8])
	return nil
}

func runEdit(cmd *cobra.Command, args []string) error {
	if err := db.UpdateMessage(dbConn, args[0], args[1]); err != nil {
		return err
	}

	// Get updated message for sync
	msg, err := db.GetMessageByID(dbConn, args[0])
	if err == nil {
		// Queue sync change (best-effort, won't fail if sync not configured)
		if err := sync.TryQueueMessageChange(context.Background(), dbConn, msg, vault.OpUpsert); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to queue sync: %v\n", err)
		}
	}

	color.Green("Message updated")
	return nil
}
