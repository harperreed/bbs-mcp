// ABOUTME: MCP tool implementations
// ABOUTME: CRUD operations exposed as MCP tools

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/harper/bbs/internal/db"
	"github.com/harper/bbs/internal/identity"
	"github.com/harper/bbs/internal/models"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerTools() {
	// Topic tools
	s.mcp.AddTool(&mcp.Tool{
		Name:        "list_topics",
		Description: "List all topics on the board",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"include_archived":{"type":"boolean","description":"Include archived topics"}}}`),
	}, s.handleListTopics)

	s.mcp.AddTool(&mcp.Tool{
		Name:        "create_topic",
		Description: "Create a new topic",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"description":{"type":"string"}},"required":["name"]}`),
	}, s.handleCreateTopic)

	s.mcp.AddTool(&mcp.Tool{
		Name:        "archive_topic",
		Description: "Archive or unarchive a topic",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"topic":{"type":"string"},"archived":{"type":"boolean"}},"required":["topic","archived"]}`),
	}, s.handleArchiveTopic)

	// Thread tools
	s.mcp.AddTool(&mcp.Tool{
		Name:        "list_threads",
		Description: "List threads in a topic",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"topic":{"type":"string"}},"required":["topic"]}`),
	}, s.handleListThreads)

	s.mcp.AddTool(&mcp.Tool{
		Name:        "create_thread",
		Description: "Create a new thread with initial message",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"topic":{"type":"string"},"subject":{"type":"string"},"message":{"type":"string"},"agent_name":{"type":"string"}},"required":["topic","subject"]}`),
	}, s.handleCreateThread)

	s.mcp.AddTool(&mcp.Tool{
		Name:        "sticky_thread",
		Description: "Pin or unpin a thread",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"thread":{"type":"string"},"sticky":{"type":"boolean"}},"required":["thread","sticky"]}`),
	}, s.handleStickyThread)

	// Message tools
	s.mcp.AddTool(&mcp.Tool{
		Name:        "list_messages",
		Description: "List messages in a thread",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"thread":{"type":"string"}},"required":["thread"]}`),
	}, s.handleListMessages)

	s.mcp.AddTool(&mcp.Tool{
		Name:        "post_message",
		Description: "Post a message to a thread",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"thread":{"type":"string"},"content":{"type":"string"},"agent_name":{"type":"string"}},"required":["thread","content"]}`),
	}, s.handlePostMessage)

	s.mcp.AddTool(&mcp.Tool{
		Name:        "edit_message",
		Description: "Edit an existing message",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"message_id":{"type":"string"},"content":{"type":"string"}},"required":["message_id","content"]}`),
	}, s.handleEditMessage)
}

func (s *Server) handleListTopics(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		IncludeArchived bool `json:"include_archived"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
			IsError: true,
		}, nil
	}

	topics, err := db.ListTopics(s.db, args.IncludeArchived)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	result, err := json.Marshal(topics)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to marshal response: %v", err)}},
			IsError: true,
		}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(result)}},
	}, nil
}

func (s *Server) handleCreateTopic(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		AgentName   string `json:"agent_name"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
			IsError: true,
		}, nil
	}

	id := identity.GetIdentity(args.AgentName, "mcp")
	topic := models.NewTopic(args.Name, args.Description, id)

	if err := db.CreateTopic(s.db, topic); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Created topic: %s (ID: %s)", args.Name, topic.ID.String()[:8])}},
	}, nil
}

func (s *Server) handleArchiveTopic(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Topic    string `json:"topic"`
		Archived bool   `json:"archived"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
			IsError: true,
		}, nil
	}

	topicID, err := db.ResolveTopicID(s.db, args.Topic)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	if err := db.ArchiveTopic(s.db, topicID, args.Archived); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	status := "archived"
	if !args.Archived {
		status = "unarchived"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Topic %s", status)}},
	}, nil
}

func (s *Server) handleListThreads(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Topic string `json:"topic"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
			IsError: true,
		}, nil
	}

	topicID, err := db.ResolveTopicID(s.db, args.Topic)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	threads, err := db.ListThreads(s.db, topicID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	result, err := json.Marshal(threads)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to marshal response: %v", err)}},
			IsError: true,
		}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(result)}},
	}, nil
}

func (s *Server) handleCreateThread(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Topic     string `json:"topic"`
		Subject   string `json:"subject"`
		Message   string `json:"message"`
		AgentName string `json:"agent_name"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
			IsError: true,
		}, nil
	}

	topicID, err := db.ResolveTopicID(s.db, args.Topic)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	topicUUID, err := models.ParseUUID(topicID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid topic ID: %v", err)}},
			IsError: true,
		}, nil
	}
	id := identity.GetIdentity(args.AgentName, "mcp")
	thread := models.NewThread(topicUUID, args.Subject, id)

	if err := db.CreateThread(s.db, thread); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	// Post initial message if provided
	if args.Message != "" {
		msg := models.NewMessage(thread.ID, args.Message, id)
		if err := db.CreateMessage(s.db, msg); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("thread created but failed to post message: %v", err)}},
				IsError: true,
			}, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Created thread: %s (ID: %s)", args.Subject, thread.ID.String()[:8])}},
	}, nil
}

func (s *Server) handleStickyThread(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Thread string `json:"thread"`
		Sticky bool   `json:"sticky"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
			IsError: true,
		}, nil
	}

	if err := db.SetThreadSticky(s.db, args.Thread, args.Sticky); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	status := "pinned"
	if !args.Sticky {
		status = "unpinned"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Thread %s", status)}},
	}, nil
}

func (s *Server) handleListMessages(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Thread string `json:"thread"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
			IsError: true,
		}, nil
	}

	messages, err := db.ListMessages(s.db, args.Thread)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	result, err := json.Marshal(messages)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to marshal response: %v", err)}},
			IsError: true,
		}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(result)}},
	}, nil
}

func (s *Server) handlePostMessage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Thread    string `json:"thread"`
		Content   string `json:"content"`
		AgentName string `json:"agent_name"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
			IsError: true,
		}, nil
	}

	thread, err := db.GetThreadByID(s.db, args.Thread)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	id := identity.GetIdentity(args.AgentName, "mcp")
	msg := models.NewMessage(thread.ID, args.Content, id)

	if err := db.CreateMessage(s.db, msg); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Posted message (ID: %s)", msg.ID.String()[:8])}},
	}, nil
}

func (s *Server) handleEditMessage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		MessageID string `json:"message_id"`
		Content   string `json:"content"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
			IsError: true,
		}, nil
	}

	if err := db.UpdateMessage(s.db, args.MessageID, args.Content); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "Message updated"}},
	}, nil
}
