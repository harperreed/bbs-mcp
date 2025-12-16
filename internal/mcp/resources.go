// ABOUTME: MCP resource implementations
// ABOUTME: Read-only data access via MCP resources

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/harper/bbs/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	s.mcp.AddResource(&mcp.Resource{
		URI:         "bbs://topics",
		Name:        "All Topics",
		Description: "List of all active topics",
		MIMEType:    "application/json",
	}, s.handleTopicsResource)

	s.mcp.AddResource(&mcp.Resource{
		URI:         "bbs://recent",
		Name:        "Recent Activity",
		Description: "Recent threads and messages across all topics",
		MIMEType:    "text/markdown",
	}, s.handleRecentResource)

	// Dynamic resources for threads and messages
	s.mcp.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "bbs://topics/{topic}/threads",
		Name:        "Topic Threads",
		Description: "Threads in a specific topic",
		MIMEType:    "application/json",
	}, s.handleTopicThreadsResource)

	s.mcp.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "bbs://threads/{thread}/messages",
		Name:        "Thread Messages",
		Description: "Messages in a specific thread",
		MIMEType:    "text/markdown",
	}, s.handleThreadMessagesResource)
}

func (s *Server) handleTopicsResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	topics, err := db.ListTopics(s.db, false)
	if err != nil {
		return nil, err
	}

	data, _ := json.MarshalIndent(topics, "", "  ")
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      "bbs://topics",
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}

func (s *Server) handleRecentResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	topics, _ := db.ListTopics(s.db, false)

	var sb strings.Builder
	sb.WriteString("# Recent Activity\n\n")

	for _, topic := range topics {
		threads, _ := db.ListThreads(s.db, topic.ID.String())
		if len(threads) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n\n", topic.Name))
		for i, thread := range threads {
			if i >= 3 {
				break
			}
			prefix := ""
			if thread.Sticky {
				prefix = "📌 "
			}
			sb.WriteString(fmt.Sprintf("- %s**%s** by %s\n", prefix, thread.Subject, thread.CreatedBy))
		}
		sb.WriteString("\n")
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      "bbs://recent",
			MIMEType: "text/markdown",
			Text:     sb.String(),
		}},
	}, nil
}

func (s *Server) handleTopicThreadsResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Extract topic from URI
	parts := strings.Split(req.Params.URI, "/")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid URI")
	}
	topicName := parts[3]

	topicID, err := db.ResolveTopicID(s.db, topicName)
	if err != nil {
		return nil, err
	}

	threads, err := db.ListThreads(s.db, topicID)
	if err != nil {
		return nil, err
	}

	data, _ := json.MarshalIndent(threads, "", "  ")
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}

func (s *Server) handleThreadMessagesResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Extract thread from URI
	parts := strings.Split(req.Params.URI, "/")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid URI")
	}
	threadID := parts[3]

	thread, err := db.GetThreadByID(s.db, threadID)
	if err != nil {
		return nil, err
	}

	messages, err := db.ListMessages(s.db, thread.ID.String())
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", thread.Subject))
	sb.WriteString(fmt.Sprintf("*Started by %s on %s*\n\n", thread.CreatedBy, thread.CreatedAt.Format("2006-01-02")))
	sb.WriteString("---\n\n")

	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("**%s** · %s\n\n", msg.CreatedBy, msg.CreatedAt.Format("Jan 02 15:04")))
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n---\n\n")
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "text/markdown",
			Text:     sb.String(),
		}},
	}, nil
}
