// ABOUTME: Tests for MCP server and tool handlers
// ABOUTME: Verifies server creation, tool execution, and resource handling

package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/harper/bbs/internal/models"
	"github.com/harper/bbs/internal/storage"
)

func TestNewServerRequiresStorage(t *testing.T) {
	_, err := NewServer(nil)
	if err == nil {
		t.Error("NewServer should fail with nil storage")
	}
}

func TestNewServerSuccess(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, err := NewServer(store)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	if server.store != store {
		t.Error("server store not set correctly")
	}
}

// Helper function to create a CallToolRequest with JSON arguments
func makeToolRequest(args map[string]interface{}) *mcp.CallToolRequest {
	argsJSON, _ := json.Marshal(args)
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: argsJSON,
		},
	}
}

func TestHandleListTopics(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	// Create some topics
	topic1 := models.NewTopic("General", "General discussion", "test@mcp")
	topic2 := models.NewTopic("Support", "Get help", "test@mcp")
	_ = store.CreateTopic(topic1)
	_ = store.CreateTopic(topic2)

	tests := []struct {
		name            string
		args            map[string]interface{}
		includeArchived bool
	}{
		{
			name: "list all topics",
			args: map[string]interface{}{},
		},
		{
			name:            "with archived flag",
			args:            map[string]interface{}{"include_archived": true},
			includeArchived: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeToolRequest(tt.args)
			result, err := server.handleListTopics(context.Background(), req)
			if err != nil {
				t.Fatalf("handleListTopics failed: %v", err)
			}
			if result.IsError {
				t.Errorf("handleListTopics returned error: %v", result.Content)
			}
		})
	}
}

func TestHandleCreateTopic(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "create basic topic",
			args: map[string]interface{}{
				"name":        "NewTopic",
				"description": "A new topic",
			},
			wantErr: false,
		},
		{
			name: "create topic with agent name",
			args: map[string]interface{}{
				"name":        "AgentTopic",
				"description": "Created by agent",
				"agent_name":  "claude",
			},
			wantErr: false,
		},
		{
			name: "create topic without description",
			args: map[string]interface{}{
				"name": "NoDesc",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeToolRequest(tt.args)
			result, err := server.handleCreateTopic(context.Background(), req)
			if err != nil {
				t.Fatalf("handleCreateTopic failed: %v", err)
			}
			if result.IsError != tt.wantErr {
				t.Errorf("handleCreateTopic IsError = %v, want %v", result.IsError, tt.wantErr)
			}
		})
	}
}

func TestHandleArchiveTopic(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)

	tests := []struct {
		name     string
		args     map[string]interface{}
		wantErr  bool
		archived bool
	}{
		{
			name: "archive topic",
			args: map[string]interface{}{
				"topic":    topic.Name,
				"archived": true,
			},
			wantErr:  false,
			archived: true,
		},
		{
			name: "unarchive topic",
			args: map[string]interface{}{
				"topic":    topic.Name,
				"archived": false,
			},
			wantErr:  false,
			archived: false,
		},
		{
			name: "archive nonexistent topic",
			args: map[string]interface{}{
				"topic":    "nonexistent",
				"archived": true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeToolRequest(tt.args)
			result, _ := server.handleArchiveTopic(context.Background(), req)
			if result.IsError != tt.wantErr {
				t.Errorf("handleArchiveTopic IsError = %v, want %v", result.IsError, tt.wantErr)
			}
		})
	}
}

func TestHandleListThreads(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)

	thread := models.NewThread(topic.ID, "Test Thread", "test@mcp")
	_ = store.CreateThread(thread)

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "list threads by topic name",
			args: map[string]interface{}{
				"topic": topic.Name,
			},
			wantErr: false,
		},
		{
			name: "list threads by topic ID",
			args: map[string]interface{}{
				"topic": topic.ID.String(),
			},
			wantErr: false,
		},
		{
			name: "list threads nonexistent topic",
			args: map[string]interface{}{
				"topic": "nonexistent",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeToolRequest(tt.args)
			result, _ := server.handleListThreads(context.Background(), req)
			if result.IsError != tt.wantErr {
				t.Errorf("handleListThreads IsError = %v, want %v", result.IsError, tt.wantErr)
			}
		})
	}
}

func TestHandleCreateThread(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "create thread without message",
			args: map[string]interface{}{
				"topic":   topic.Name,
				"subject": "New Thread",
			},
			wantErr: false,
		},
		{
			name: "create thread with message",
			args: map[string]interface{}{
				"topic":   topic.Name,
				"subject": "Thread With Message",
				"message": "This is the initial message",
			},
			wantErr: false,
		},
		{
			name: "create thread with agent name",
			args: map[string]interface{}{
				"topic":      topic.Name,
				"subject":    "Agent Thread",
				"agent_name": "claude",
			},
			wantErr: false,
		},
		{
			name: "create thread nonexistent topic",
			args: map[string]interface{}{
				"topic":   "nonexistent",
				"subject": "Test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeToolRequest(tt.args)
			result, _ := server.handleCreateThread(context.Background(), req)
			if result.IsError != tt.wantErr {
				t.Errorf("handleCreateThread IsError = %v, want %v", result.IsError, tt.wantErr)
			}
		})
	}
}

func TestHandleStickyThread(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@mcp")
	_ = store.CreateThread(thread)

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "pin thread",
			args: map[string]interface{}{
				"thread": thread.ID.String(),
				"sticky": true,
			},
			wantErr: false,
		},
		{
			name: "unpin thread",
			args: map[string]interface{}{
				"thread": thread.ID.String(),
				"sticky": false,
			},
			wantErr: false,
		},
		{
			name: "sticky nonexistent thread",
			args: map[string]interface{}{
				"thread": "nonexistent",
				"sticky": true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeToolRequest(tt.args)
			result, _ := server.handleStickyThread(context.Background(), req)
			if result.IsError != tt.wantErr {
				t.Errorf("handleStickyThread IsError = %v, want %v", result.IsError, tt.wantErr)
			}
		})
	}
}

func TestHandleListMessages(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@mcp")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello world", "test@mcp")
	_ = store.CreateMessage(msg)

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "list messages",
			args: map[string]interface{}{
				"thread": thread.ID.String(),
			},
			wantErr: false,
		},
		{
			name: "list messages by prefix",
			args: map[string]interface{}{
				"thread": thread.ID.String()[:8],
			},
			wantErr: false,
		},
		{
			name: "list messages nonexistent thread",
			args: map[string]interface{}{
				"thread": "nonexistent",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeToolRequest(tt.args)
			result, _ := server.handleListMessages(context.Background(), req)
			if result.IsError != tt.wantErr {
				t.Errorf("handleListMessages IsError = %v, want %v", result.IsError, tt.wantErr)
			}
		})
	}
}

func TestHandlePostMessage(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@mcp")
	_ = store.CreateThread(thread)

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "post message",
			args: map[string]interface{}{
				"thread":  thread.ID.String(),
				"content": "Hello world",
			},
			wantErr: false,
		},
		{
			name: "post message with agent name",
			args: map[string]interface{}{
				"thread":     thread.ID.String(),
				"content":    "Hello from agent",
				"agent_name": "claude",
			},
			wantErr: false,
		},
		{
			name: "post message nonexistent thread",
			args: map[string]interface{}{
				"thread":  "nonexistent",
				"content": "Hello",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeToolRequest(tt.args)
			result, _ := server.handlePostMessage(context.Background(), req)
			if result.IsError != tt.wantErr {
				t.Errorf("handlePostMessage IsError = %v, want %v", result.IsError, tt.wantErr)
			}
		})
	}
}

func TestHandleEditMessage(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@mcp")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Original content", "test@mcp")
	_ = store.CreateMessage(msg)

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "edit message",
			args: map[string]interface{}{
				"message_id": msg.ID.String(),
				"content":    "Edited content",
			},
			wantErr: false,
		},
		{
			name: "edit message by prefix",
			args: map[string]interface{}{
				"message_id": msg.ID.String()[:8],
				"content":    "Edited again",
			},
			wantErr: false,
		},
		{
			name: "edit nonexistent message",
			args: map[string]interface{}{
				"message_id": "nonexistent",
				"content":    "Hello",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeToolRequest(tt.args)
			result, _ := server.handleEditMessage(context.Background(), req)
			if result.IsError != tt.wantErr {
				t.Errorf("handleEditMessage IsError = %v, want %v", result.IsError, tt.wantErr)
			}
		})
	}
}

func TestHandleTopicsResource(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)

	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "bbs://topics",
		},
	}

	result, err := server.handleTopicsResource(context.Background(), req)
	if err != nil {
		t.Fatalf("handleTopicsResource failed: %v", err)
	}
	if len(result.Contents) == 0 {
		t.Error("expected contents in result")
	}
}

func TestHandleRecentResource(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@mcp")
	_ = store.CreateThread(thread)

	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "bbs://recent",
		},
	}

	result, err := server.handleRecentResource(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRecentResource failed: %v", err)
	}
	if len(result.Contents) == 0 {
		t.Error("expected contents in result")
	}
}

func TestHandleTopicThreadsResource(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@mcp")
	_ = store.CreateThread(thread)

	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name:    "valid topic",
			uri:     "bbs://topics/TestTopic/threads",
			wantErr: false,
		},
		{
			name:    "invalid URI format",
			uri:     "bbs://invalid",
			wantErr: true,
		},
		{
			name:    "nonexistent topic",
			uri:     "bbs://topics/nonexistent/threads",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &mcp.ReadResourceRequest{
				Params: &mcp.ReadResourceParams{
					URI: tt.uri,
				},
			}

			result, err := server.handleTopicThreadsResource(context.Background(), req)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result.Contents) == 0 {
					t.Error("expected contents in result")
				}
			}
		})
	}
}

func TestHandleThreadMessagesResource(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@mcp")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello", "test@mcp")
	_ = store.CreateMessage(msg)

	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name:    "valid thread",
			uri:     "bbs://threads/" + thread.ID.String() + "/messages",
			wantErr: false,
		},
		{
			name:    "invalid URI format",
			uri:     "bbs://invalid",
			wantErr: true,
		},
		{
			name:    "nonexistent thread",
			uri:     "bbs://threads/nonexistent/messages",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &mcp.ReadResourceRequest{
				Params: &mcp.ReadResourceParams{
					URI: tt.uri,
				},
			}

			result, err := server.handleThreadMessagesResource(context.Background(), req)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result.Contents) == 0 {
					t.Error("expected contents in result")
				}
			}
		})
	}
}

func TestHandlePostUpdatePrompt(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	req := &mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{
			Arguments: map[string]string{
				"topic":   "General",
				"subject": "Status Update",
			},
		},
	}

	result, err := server.handlePostUpdatePrompt(context.Background(), req)
	if err != nil {
		t.Fatalf("handlePostUpdatePrompt failed: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Error("expected messages in result")
	}
}

func TestHandleSummarizePrompt(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	req := &mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{
			Arguments: map[string]string{
				"thread": "abc123",
			},
		},
	}

	result, err := server.handleSummarizePrompt(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSummarizePrompt failed: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Error("expected messages in result")
	}
}

func TestInvalidJSONArguments(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	invalidJSON := json.RawMessage(`{invalid json}`)
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: invalidJSON,
		},
	}

	// Test each handler with invalid JSON
	handlers := []struct {
		name    string
		handler func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{"listTopics", server.handleListTopics},
		{"createTopic", server.handleCreateTopic},
		{"archiveTopic", server.handleArchiveTopic},
		{"listThreads", server.handleListThreads},
		{"createThread", server.handleCreateThread},
		{"stickyThread", server.handleStickyThread},
		{"listMessages", server.handleListMessages},
		{"postMessage", server.handlePostMessage},
		{"editMessage", server.handleEditMessage},
	}

	for _, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			result, err := h.handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			if !result.IsError {
				t.Error("expected IsError to be true for invalid JSON")
			}
		})
	}
}

func TestHandleRecentResourceMultipleTopics(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	// Create multiple topics with threads
	for i := 0; i < 3; i++ {
		topic := models.NewTopic("Topic"+string(rune('A'+i)), "Test", "test@mcp")
		_ = store.CreateTopic(topic)
		for j := 0; j < 4; j++ {
			thread := models.NewThread(topic.ID, "Thread"+string(rune('1'+j)), "test@mcp")
			thread.Sticky = j == 0 // First thread is sticky
			_ = store.CreateThread(thread)
		}
	}

	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "bbs://recent",
		},
	}

	result, err := server.handleRecentResource(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRecentResource failed: %v", err)
	}
	if len(result.Contents) == 0 {
		t.Error("expected contents in result")
	}
}

func TestHandleListTopicsEmpty(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	req := makeToolRequest(map[string]interface{}{})
	result, err := server.handleListTopics(context.Background(), req)
	if err != nil {
		t.Fatalf("handleListTopics failed: %v", err)
	}
	if result.IsError {
		t.Error("unexpected error in result")
	}
	// Should return empty array, not error
	content := result.Content[0].(*mcp.TextContent).Text
	if content != "[]" && content != "null" {
		t.Logf("Got content: %s", content)
	}
}

func TestHandleCreateTopicDuplicate(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	// Create first topic
	req1 := makeToolRequest(map[string]interface{}{
		"name":        "TestTopic",
		"description": "Test",
	})
	_, _ = server.handleCreateTopic(context.Background(), req1)

	// Try to create duplicate
	req2 := makeToolRequest(map[string]interface{}{
		"name":        "TestTopic",
		"description": "Duplicate",
	})
	result, _ := server.handleCreateTopic(context.Background(), req2)
	if !result.IsError {
		t.Error("expected error for duplicate topic")
	}
}

func TestHandleTopicsResourceEmpty(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "bbs://topics",
		},
	}

	result, err := server.handleTopicsResource(context.Background(), req)
	if err != nil {
		t.Fatalf("handleTopicsResource failed: %v", err)
	}
	if len(result.Contents) == 0 {
		t.Error("expected contents even with no topics")
	}
}

func TestHandleRecentResourceEmpty(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "bbs://recent",
		},
	}

	result, err := server.handleRecentResource(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRecentResource failed: %v", err)
	}
	if len(result.Contents) == 0 {
		t.Error("expected contents even with no threads")
	}
}

func TestHandleCreateThreadWithMessageAndAgentName(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)

	req := makeToolRequest(map[string]interface{}{
		"topic":      topic.Name,
		"subject":    "Thread With Message",
		"message":    "This is the initial message",
		"agent_name": "claude",
	})

	result, _ := server.handleCreateThread(context.Background(), req)
	if result.IsError {
		t.Errorf("handleCreateThread failed: %v", result.Content)
	}

	// Verify message was created
	threads, _ := store.ListThreads(topic.ID)
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	messages, _ := store.ListMessages(threads[0].ID)
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
}

func TestHandleStickyThreadPinAndUnpin(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@mcp")
	_ = store.CreateThread(thread)

	// Pin
	reqPin := makeToolRequest(map[string]interface{}{
		"thread": thread.ID.String(),
		"sticky": true,
	})
	resultPin, _ := server.handleStickyThread(context.Background(), reqPin)
	if resultPin.IsError {
		t.Errorf("pin failed: %v", resultPin.Content)
	}

	// Verify pinned
	gotThread, _ := store.GetThread(thread.ID)
	if !gotThread.Sticky {
		t.Error("expected thread to be sticky")
	}

	// Unpin
	reqUnpin := makeToolRequest(map[string]interface{}{
		"thread": thread.ID.String(),
		"sticky": false,
	})
	resultUnpin, _ := server.handleStickyThread(context.Background(), reqUnpin)
	if resultUnpin.IsError {
		t.Errorf("unpin failed: %v", resultUnpin.Content)
	}

	// Verify unpinned
	gotThread, _ = store.GetThread(thread.ID)
	if gotThread.Sticky {
		t.Error("expected thread to not be sticky")
	}
}

func TestHandlePostMessageWithAgentName(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Test Thread", "test@mcp")
	_ = store.CreateThread(thread)

	req := makeToolRequest(map[string]interface{}{
		"thread":     thread.ID.String(),
		"content":    "Hello from agent",
		"agent_name": "claude",
	})

	result, _ := server.handlePostMessage(context.Background(), req)
	if result.IsError {
		t.Errorf("handlePostMessage failed: %v", result.Content)
	}

	// Verify identity
	messages, _ := store.ListMessages(thread.ID)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].CreatedBy != "claude@mcp" {
		t.Errorf("expected createdBy 'claude@mcp', got '%s'", messages[0].CreatedBy)
	}
}

func TestHandleArchiveTopicUnarchive(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	server, _ := NewServer(store)

	topic := models.NewTopic("TestTopic", "Test", "test@mcp")
	topic.Archived = true
	_ = store.CreateTopic(topic)

	// Unarchive
	req := makeToolRequest(map[string]interface{}{
		"topic":    topic.Name,
		"archived": false,
	})
	result, _ := server.handleArchiveTopic(context.Background(), req)
	if result.IsError {
		t.Errorf("unarchive failed: %v", result.Content)
	}

	// Verify
	got, _ := store.GetTopic(topic.ID)
	if got.Archived {
		t.Error("expected topic to be unarchived")
	}
}

// Helper to create a test store
func newTestStore(t *testing.T) *storage.SqliteStore {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := storage.NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return store
}
