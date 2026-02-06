// ABOUTME: MCP server initialization and configuration
// ABOUTME: Sets up server with tools, resources, and prompts

package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/harper/bbs/internal/storage"
)

// Server wraps MCP server with storage.
type Server struct {
	mcp   *mcp.Server
	store storage.Storage
}

// NewServer creates MCP server with all capabilities.
func NewServer(store storage.Storage) (*Server, error) {
	if store == nil {
		return nil, fmt.Errorf("storage is required")
	}

	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "bbs",
			Version: "1.0.0",
		},
		nil,
	)

	s := &Server{
		mcp:   mcpServer,
		store: store,
	}

	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	return s, nil
}

// Serve starts the MCP server in stdio mode.
func (s *Server) Serve(ctx context.Context) error {
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}
