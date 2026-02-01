// ABOUTME: MCP server command implementation
// ABOUTME: Starts BBS MCP server in stdio mode

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/harper/bbs/internal/mcp"
	"github.com/harper/bbs/internal/storage"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdio mode)",
	Long: `Start the Model Context Protocol server for AI agent integration.

The MCP server communicates via stdio, allowing AI agents like Claude
to interact with BBS through a standardized protocol.`,
	RunE: runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	store, err := storage.NewStore(storage.DefaultDBPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	server, err := mcp.NewServer(store)
	if err != nil {
		return err
	}

	return server.Serve(ctx)
}
