// ABOUTME: Integration tests for MCP server using MarkdownStore backend
// ABOUTME: Verifies full MCP workflow produces identical results with both storage backends

package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/harper/bbs/internal/models"
	"github.com/harper/bbs/internal/storage"
)

// newTestMarkdownStore creates a MarkdownStore in a temporary directory for testing.
func newTestMarkdownStore(t *testing.T) *storage.MarkdownStore {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := storage.NewMarkdownStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create test markdown store: %v", err)
	}
	return store
}

func TestMCPMarkdownCreateAndListTopic(t *testing.T) {
	server, _, ctx := setupMarkdownMCPTest(t)

	mcpToolMustSucceed(t, server.handleCreateTopic, ctx, map[string]interface{}{
		"name":        "mcp-test-topic",
		"description": "Created via MCP tools",
	})
	mcpToolMustSucceed(t, server.handleListTopics, ctx, map[string]interface{}{})
}

func TestMCPMarkdownThreadAndMessages(t *testing.T) {
	server, store, ctx := setupMarkdownMCPTest(t)

	mcpToolMustSucceed(t, server.handleCreateTopic, ctx, map[string]interface{}{
		"name": "msg-topic",
	})
	mcpToolMustSucceed(t, server.handleCreateThread, ctx, map[string]interface{}{
		"topic":      "msg-topic",
		"subject":    "MCP Integration Test",
		"message":    "Initial message",
		"agent_name": "test-agent",
	})

	threadID := mustGetFirstThreadID(t, store, "msg-topic")

	mcpToolMustSucceed(t, server.handlePostMessage, ctx, map[string]interface{}{
		"thread": threadID, "content": "Reply 1", "agent_name": "another-agent",
	})
	mcpToolMustSucceed(t, server.handlePostMessage, ctx, map[string]interface{}{
		"thread": threadID, "content": "Reply 2",
	})
	mcpToolMustSucceed(t, server.handleListMessages, ctx, map[string]interface{}{
		"thread": threadID,
	})

	messages := mustListMessages(t, store, threadID)
	if len(messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(messages))
	}
}

func TestMCPMarkdownStickyAndArchive(t *testing.T) {
	server, store, ctx := setupMarkdownMCPTest(t)

	mcpToolMustSucceed(t, server.handleCreateTopic, ctx, map[string]interface{}{
		"name": "sticky-topic",
	})
	mcpToolMustSucceed(t, server.handleCreateThread, ctx, map[string]interface{}{
		"topic": "sticky-topic", "subject": "Pin Me",
	})

	threadID := mustGetFirstThreadID(t, store, "sticky-topic")

	mcpToolMustSucceed(t, server.handleStickyThread, ctx, map[string]interface{}{
		"thread": threadID, "sticky": true,
	})
	mcpToolMustSucceed(t, server.handleArchiveTopic, ctx, map[string]interface{}{
		"topic": "sticky-topic", "archived": true,
	})

	topic, err := store.GetTopicByName("sticky-topic")
	if err != nil {
		t.Fatalf("GetTopicByName: %v", err)
	}
	if !topic.Archived {
		t.Error("topic should be archived")
	}
}

func TestMCPMarkdownResources(t *testing.T) {
	server, _, ctx := setupMarkdownMCPTest(t)

	mcpToolMustSucceed(t, server.handleCreateTopic, ctx, map[string]interface{}{
		"name": "res-topic",
	})
	mcpToolMustSucceed(t, server.handleCreateThread, ctx, map[string]interface{}{
		"topic": "res-topic", "subject": "Resource Test", "message": "content",
	})

	mcpResourceMustSucceed(t, server.handleTopicsResource, ctx, "bbs://topics")
	mcpResourceMustSucceed(t, server.handleRecentResource, ctx, "bbs://recent")
}

// setupMarkdownMCPTest creates a MarkdownStore-backed MCP server for testing.
func setupMarkdownMCPTest(t *testing.T) (*Server, storage.Storage, context.Context) {
	t.Helper()
	store := newTestMarkdownStore(t)
	t.Cleanup(func() { store.Close() })
	server, err := NewServer(store)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	return server, store, context.Background()
}

// mcpToolMustSucceed calls an MCP tool handler and fatals on error.
func mcpToolMustSucceed(t *testing.T, handler func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error), ctx context.Context, args map[string]interface{}) {
	t.Helper()
	result, err := handler(ctx, makeToolRequest(args))
	if err != nil {
		t.Fatalf("MCP tool handler failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("MCP tool handler returned error: %v", result.Content)
	}
}

// mcpResourceMustSucceed calls an MCP resource handler and fatals on error.
func mcpResourceMustSucceed(t *testing.T, handler func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error), ctx context.Context, uri string) {
	t.Helper()
	req := &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: uri}}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("resource handler for %s failed: %v", uri, err)
	}
	if len(result.Contents) == 0 {
		t.Errorf("expected contents from resource %s", uri)
	}
}

// mustGetFirstThreadID resolves a topic and returns the first thread's ID as a string.
func mustGetFirstThreadID(t *testing.T, store storage.Storage, topicName string) string {
	t.Helper()
	threads, err := store.ListThreads(mustResolveTopicID(t, store, topicName))
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) == 0 {
		t.Fatalf("no threads found in topic %q", topicName)
	}
	return threads[0].ID.String()
}

// mustListMessages resolves a thread ID string and returns its messages.
func mustListMessages(t *testing.T, store storage.Storage, threadIDStr string) []*models.Message {
	t.Helper()
	thread, err := store.ResolveThread(threadIDStr)
	if err != nil {
		t.Fatalf("ResolveThread failed: %v", err)
	}
	messages, err := store.ListMessages(thread.ID)
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	return messages
}

func TestMarkdownAndSqliteProduceSameResults(t *testing.T) {
	// Create identical data in both backends and verify the output matches
	mdStore := newTestMarkdownStore(t)
	defer mdStore.Close()

	sqlStore := newTestStore(t)
	defer sqlStore.Close()

	// Create identical data in both stores
	topic := models.NewTopic("test-parity", "Parity test", "test@cli")
	if err := mdStore.CreateTopic(topic); err != nil {
		t.Fatalf("markdown CreateTopic failed: %v", err)
	}
	if err := sqlStore.CreateTopic(topic); err != nil {
		t.Fatalf("sqlite CreateTopic failed: %v", err)
	}

	thread := models.NewThread(topic.ID, "Parity Thread", "test@cli")
	thread.Sticky = true
	if err := mdStore.CreateThread(thread); err != nil {
		t.Fatalf("markdown CreateThread failed: %v", err)
	}
	if err := sqlStore.CreateThread(thread); err != nil {
		t.Fatalf("sqlite CreateThread failed: %v", err)
	}

	msg := models.NewMessage(thread.ID, "Hello parity", "test@cli")
	if err := mdStore.CreateMessage(msg); err != nil {
		t.Fatalf("markdown CreateMessage failed: %v", err)
	}
	if err := sqlStore.CreateMessage(msg); err != nil {
		t.Fatalf("sqlite CreateMessage failed: %v", err)
	}

	// Create MCP servers for both
	mdServer, err := NewServer(mdStore)
	if err != nil {
		t.Fatalf("NewServer (markdown) failed: %v", err)
	}
	sqlServer, err := NewServer(sqlStore)
	if err != nil {
		t.Fatalf("NewServer (sqlite) failed: %v", err)
	}

	ctx := context.Background()

	// Compare list_topics
	mdTopicsResult, _ := mdServer.handleListTopics(ctx, makeToolRequest(map[string]interface{}{}))
	sqlTopicsResult, _ := sqlServer.handleListTopics(ctx, makeToolRequest(map[string]interface{}{}))

	mdTopicsJSON := extractTextContent(t, mdTopicsResult)
	sqlTopicsJSON := extractTextContent(t, sqlTopicsResult)

	var mdTopics, sqlTopics []map[string]interface{}
	if err := json.Unmarshal([]byte(mdTopicsJSON), &mdTopics); err != nil {
		t.Fatalf("unmarshal markdown topics: %v", err)
	}
	if err := json.Unmarshal([]byte(sqlTopicsJSON), &sqlTopics); err != nil {
		t.Fatalf("unmarshal sqlite topics: %v", err)
	}
	if len(mdTopics) != len(sqlTopics) {
		t.Errorf("topic count mismatch: markdown=%d, sqlite=%d", len(mdTopics), len(sqlTopics))
	}

	// Compare list_threads
	mdThreadsResult, _ := mdServer.handleListThreads(ctx, makeToolRequest(map[string]interface{}{"topic": "test-parity"}))
	sqlThreadsResult, _ := sqlServer.handleListThreads(ctx, makeToolRequest(map[string]interface{}{"topic": "test-parity"}))

	mdThreadsJSON := extractTextContent(t, mdThreadsResult)
	sqlThreadsJSON := extractTextContent(t, sqlThreadsResult)

	var mdThreads, sqlThreads []map[string]interface{}
	if err := json.Unmarshal([]byte(mdThreadsJSON), &mdThreads); err != nil {
		t.Fatalf("unmarshal markdown threads: %v", err)
	}
	if err := json.Unmarshal([]byte(sqlThreadsJSON), &sqlThreads); err != nil {
		t.Fatalf("unmarshal sqlite threads: %v", err)
	}
	if len(mdThreads) != len(sqlThreads) {
		t.Errorf("thread count mismatch: markdown=%d, sqlite=%d", len(mdThreads), len(sqlThreads))
	}

	// Verify key fields match (subject, sticky, etc.)
	if len(mdThreads) > 0 && len(sqlThreads) > 0 {
		if mdThreads[0]["subject"] != sqlThreads[0]["subject"] {
			t.Errorf("thread subject mismatch: markdown=%v, sqlite=%v",
				mdThreads[0]["subject"], sqlThreads[0]["subject"])
		}
		if mdThreads[0]["sticky"] != sqlThreads[0]["sticky"] {
			t.Errorf("thread sticky mismatch: markdown=%v, sqlite=%v",
				mdThreads[0]["sticky"], sqlThreads[0]["sticky"])
		}
	}

	// Compare list_messages
	mdMsgsResult, _ := mdServer.handleListMessages(ctx, makeToolRequest(map[string]interface{}{"thread": thread.ID.String()}))
	sqlMsgsResult, _ := sqlServer.handleListMessages(ctx, makeToolRequest(map[string]interface{}{"thread": thread.ID.String()}))

	mdMsgsJSON := extractTextContent(t, mdMsgsResult)
	sqlMsgsJSON := extractTextContent(t, sqlMsgsResult)

	var mdMsgs, sqlMsgs []map[string]interface{}
	if err := json.Unmarshal([]byte(mdMsgsJSON), &mdMsgs); err != nil {
		t.Fatalf("unmarshal markdown messages: %v", err)
	}
	if err := json.Unmarshal([]byte(sqlMsgsJSON), &sqlMsgs); err != nil {
		t.Fatalf("unmarshal sqlite messages: %v", err)
	}
	if len(mdMsgs) != len(sqlMsgs) {
		t.Errorf("message count mismatch: markdown=%d, sqlite=%d", len(mdMsgs), len(sqlMsgs))
	}
	if len(mdMsgs) > 0 && len(sqlMsgs) > 0 {
		if mdMsgs[0]["content"] != sqlMsgs[0]["content"] {
			t.Errorf("message content mismatch: markdown=%v, sqlite=%v",
				mdMsgs[0]["content"], sqlMsgs[0]["content"])
		}
		if mdMsgs[0]["created_by"] != sqlMsgs[0]["created_by"] {
			t.Errorf("message created_by mismatch: markdown=%v, sqlite=%v",
				mdMsgs[0]["created_by"], sqlMsgs[0]["created_by"])
		}
	}
}

// mustResolveTopicID resolves a topic name to its UUID via the store.
func mustResolveTopicID(t *testing.T, store storage.Storage, name string) models.UUID {
	t.Helper()
	topic, err := store.ResolveTopic(name)
	if err != nil {
		t.Fatalf("resolve topic %q: %v", name, err)
	}
	return topic.ID
}

// extractTextContent gets the text string from an MCP CallToolResult.
func extractTextContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

func TestMCPEditMessageWithMarkdownBackend(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	server, _ := NewServer(store)
	ctx := context.Background()

	// Setup: create topic, thread, message
	topic := models.NewTopic("edit-test", "Edit test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Edit Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Original content", "test@mcp")
	_ = store.CreateMessage(msg)

	// Edit via MCP
	editReq := makeToolRequest(map[string]interface{}{
		"message_id": msg.ID.String(),
		"content":    "Edited via MCP",
	})
	result, err := server.handleEditMessage(ctx, editReq)
	if err != nil {
		t.Fatalf("handleEditMessage failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleEditMessage returned error: %v", result.Content)
	}

	// Verify edit was persisted
	got, err := store.GetMessage(msg.ID)
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if got.Content != "Edited via MCP" {
		t.Errorf("expected content 'Edited via MCP', got %q", got.Content)
	}
	if got.EditedAt == nil {
		t.Error("expected EditedAt to be set after edit")
	}
}

func TestMCPCreateTopicDuplicateWithMarkdownBackend(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	server, _ := NewServer(store)
	ctx := context.Background()

	// Create first topic
	req := makeToolRequest(map[string]interface{}{
		"name":        "unique-topic",
		"description": "First",
	})
	result, _ := server.handleCreateTopic(ctx, req)
	if result.IsError {
		t.Fatalf("first create failed: %v", result.Content)
	}

	// Try to create duplicate
	req2 := makeToolRequest(map[string]interface{}{
		"name":        "unique-topic",
		"description": "Duplicate",
	})
	result2, _ := server.handleCreateTopic(ctx, req2)
	if !result2.IsError {
		t.Error("expected error for duplicate topic name")
	}
}

func TestMCPResourcesWithMarkdownBackend(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	server, _ := NewServer(store)
	ctx := context.Background()

	// Create data
	topic := models.NewTopic("res-test", "Resource test", "test@cli")
	_ = store.CreateTopic(topic)
	thread := models.NewThread(topic.ID, "Resource Thread", "test@cli")
	_ = store.CreateThread(thread)
	msg := models.NewMessage(thread.ID, "Hello resource", "test@cli")
	_ = store.CreateMessage(msg)

	// Test topic threads resource
	threadResReq := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "bbs://topics/res-test/threads",
		},
	}
	resResult, err := server.handleTopicThreadsResource(ctx, threadResReq)
	if err != nil {
		t.Fatalf("handleTopicThreadsResource failed: %v", err)
	}
	if len(resResult.Contents) == 0 {
		t.Error("expected contents in topic threads resource")
	}

	// Test thread messages resource
	msgResReq := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "bbs://threads/" + thread.ID.String() + "/messages",
		},
	}
	resResult, err = server.handleThreadMessagesResource(ctx, msgResReq)
	if err != nil {
		t.Fatalf("handleThreadMessagesResource failed: %v", err)
	}
	if len(resResult.Contents) == 0 {
		t.Error("expected contents in thread messages resource")
	}
}
