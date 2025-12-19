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

	"github.com/harper/bbs/internal/charm"
	"github.com/harper/bbs/internal/mcp"
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

	client, err := charm.Global()
	if err != nil {
		return fmt.Errorf("charm client not initialized: %w", err)
	}

	server, err := mcp.NewServer(client)
	if err != nil {
		return err
	}

	return server.Serve(ctx)
}
