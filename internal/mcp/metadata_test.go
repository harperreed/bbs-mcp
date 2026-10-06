// ABOUTME: Tests MCP discovery metadata exposed to protocol clients.
// ABOUTME: Verifies tool schemas and resource descriptions match handler behavior.

package mcp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/harper/bbs/internal/models"
	"github.com/harper/bbs/internal/storage"
)

func TestCreateTopicMetadataAdvertisesOptionalAgentName(t *testing.T) {
	client, _ := newMetadataClient(t)

	result, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	var createTopic *sdk.Tool
	for _, tool := range result.Tools {
		if tool.Name == "create_topic" {
			createTopic = tool
			break
		}
	}
	if createTopic == nil {
		t.Fatal("create_topic not advertised")
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	schemaJSON, err := json.Marshal(createTopic.InputSchema)
	if err != nil {
		t.Fatalf("marshal create_topic input schema: %v", err)
	}
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		t.Fatalf("decode create_topic input schema: %v", err)
	}

	agentNameSchema, ok := schema.Properties["agent_name"]
	if !ok {
		t.Fatal("create_topic input schema does not advertise agent_name")
	}
	var agentName struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(agentNameSchema, &agentName); err != nil {
		t.Fatalf("decode agent_name schema: %v", err)
	}
	if agentName.Type != "string" {
		t.Errorf("create_topic agent_name type = %q, want string", agentName.Type)
	}
	if slices.Contains(schema.Required, "agent_name") {
		t.Error("create_topic input schema advertises agent_name as required")
	}
}

func TestListTopicsMetadataDescribesArchiveScope(t *testing.T) {
	client, _ := newMetadataClient(t)

	result, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	const want = "List active topics by default; optionally include archived topics"
	for _, tool := range result.Tools {
		if tool.Name == "list_topics" {
			if tool.Description != want {
				t.Errorf("list_topics description = %q, want %q", tool.Description, want)
			}

			var schema struct {
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
			}
			schemaJSON, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatalf("marshal list_topics input schema: %v", err)
			}
			if err := json.Unmarshal(schemaJSON, &schema); err != nil {
				t.Fatalf("decode list_topics input schema: %v", err)
			}

			includeArchivedSchema, ok := schema.Properties["include_archived"]
			if !ok {
				t.Fatal("list_topics input schema does not advertise include_archived")
			}
			var includeArchived struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(includeArchivedSchema, &includeArchived); err != nil {
				t.Fatalf("decode include_archived schema: %v", err)
			}
			if includeArchived.Type != "boolean" {
				t.Errorf("list_topics include_archived type = %q, want boolean", includeArchived.Type)
			}
			if slices.Contains(schema.Required, "include_archived") {
				t.Error("list_topics input schema advertises include_archived as required")
			}
			return
		}
	}

	t.Fatal("list_topics not advertised")
}

func TestListTopicsArchiveScopeMatchesMetadata(t *testing.T) {
	client, store := newMetadataClient(t)

	active := models.NewTopic("Active Alpha", "Included by default", "test@mcp")
	if err := store.CreateTopic(active); err != nil {
		t.Fatalf("create active topic: %v", err)
	}
	archived := models.NewTopic("Archived Omega", "Included only when requested", "test@mcp")
	archived.Archived = true
	if err := store.CreateTopic(archived); err != nil {
		t.Fatalf("create archived topic: %v", err)
	}

	tests := []struct {
		name      string
		arguments any
		wantNames []string
	}{
		{name: "omitted", wantNames: []string{"Active Alpha"}},
		{name: "explicit false", arguments: map[string]any{"include_archived": false}, wantNames: []string{"Active Alpha"}},
		{name: "explicit true", arguments: map[string]any{"include_archived": true}, wantNames: []string{"Active Alpha", "Archived Omega"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.CallTool(context.Background(), &sdk.CallToolParams{
				Name:      "list_topics",
				Arguments: tt.arguments,
			})
			if err != nil {
				t.Fatalf("CallTool list_topics failed: %v", err)
			}
			if result.IsError {
				t.Fatalf("list_topics returned a tool error: %#v", result.Content)
			}
			if len(result.Content) != 1 {
				t.Fatalf("list_topics returned %d content items, want 1: %#v", len(result.Content), result.Content)
			}
			text, ok := result.Content[0].(*sdk.TextContent)
			if !ok {
				t.Fatalf("list_topics content type = %T, want *mcp.TextContent", result.Content[0])
			}

			var topics []struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(text.Text), &topics); err != nil {
				t.Fatalf("decode list_topics text %q: %v", text.Text, err)
			}
			gotNames := make([]string, 0, len(topics))
			for _, topic := range topics {
				gotNames = append(gotNames, topic.Name)
			}
			if !slices.Equal(gotNames, tt.wantNames) {
				t.Errorf("list_topics returned names %q, want %q; text = %s", gotNames, tt.wantNames, text.Text)
			}
		})
	}
}

func TestRecentResourceMetadataDescribesThreadOnlyOutput(t *testing.T) {
	client, _ := newMetadataClient(t)

	result, err := client.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}

	const want = "Up to three threads per active topic, ordered sticky first then by most recent update; message content is not included"
	for _, resource := range result.Resources {
		if resource.URI == "bbs://recent" {
			if resource.Description != want {
				t.Errorf("bbs://recent description = %q, want %q", resource.Description, want)
			}
			return
		}
	}

	t.Fatal("bbs://recent not advertised")
}

func TestRecentResourceReturnsThreeOrderedThreadsPerActiveTopicWithoutMessages(t *testing.T) {
	client, store := newMetadataClient(t)

	active := models.NewTopic("Active Topic", "Included", "test@mcp")
	if err := store.CreateTopic(active); err != nil {
		t.Fatalf("create active topic: %v", err)
	}
	archived := models.NewTopic("Archived Topic", "Excluded", "test@mcp")
	archived.Archived = true
	if err := store.CreateTopic(archived); err != nil {
		t.Fatalf("create archived topic: %v", err)
	}

	base := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	threadSpecs := []struct {
		subject   string
		updatedAt time.Time
		sticky    bool
	}{
		{subject: "Older Sticky", updatedAt: base.Add(time.Minute), sticky: true},
		{subject: "Newest Update", updatedAt: base.Add(5 * time.Minute)},
		{subject: "Second Update", updatedAt: base.Add(4 * time.Minute)},
		{subject: "Third Update", updatedAt: base.Add(3 * time.Minute)},
		{subject: "Oldest Update", updatedAt: base.Add(2 * time.Minute)},
	}
	var messageThread *models.Thread
	for _, spec := range threadSpecs {
		thread := models.NewThread(active.ID, spec.subject, "test@mcp")
		thread.UpdatedAt = spec.updatedAt
		thread.Sticky = spec.sticky
		if err := store.CreateThread(thread); err != nil {
			t.Fatalf("create thread %q: %v", spec.subject, err)
		}
		if spec.subject == "Newest Update" {
			messageThread = thread
		}
	}

	archivedThread := models.NewThread(archived.ID, "Archived Thread", "test@mcp")
	if err := store.CreateThread(archivedThread); err != nil {
		t.Fatalf("create archived thread: %v", err)
	}
	message := models.NewMessage(messageThread.ID, "private message content", "test@mcp")
	if err := store.CreateMessage(message); err != nil {
		t.Fatalf("create message: %v", err)
	}

	result, err := client.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "bbs://recent"})
	if err != nil {
		t.Fatalf("ReadResource failed: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("bbs://recent returned %d contents, want 1", len(result.Contents))
	}
	text := result.Contents[0].Text

	if !strings.Contains(text, "## Active Topic") {
		t.Errorf("active topic missing from recent resource:\n%s", text)
	}
	for _, excluded := range []string{"Archived Topic", "Archived Thread", "Third Update", "Oldest Update", "private message content"} {
		if strings.Contains(text, excluded) {
			t.Errorf("recent resource unexpectedly contains %q:\n%s", excluded, text)
		}
	}

	stickyIndex := strings.Index(text, "Older Sticky")
	newestIndex := strings.Index(text, "Newest Update")
	secondIndex := strings.Index(text, "Second Update")
	if stickyIndex < 0 || newestIndex < 0 || secondIndex < 0 {
		t.Fatalf("recent resource is missing an expected thread:\n%s", text)
	}
	if stickyIndex >= newestIndex || newestIndex >= secondIndex {
		t.Errorf("thread order is not sticky then most recent update:\n%s", text)
	}

	threadCount := 0
	for line := range strings.Lines(text) {
		if strings.HasPrefix(line, "- ") {
			threadCount++
		}
	}
	if threadCount != 3 {
		t.Errorf("recent resource contains %d threads, want 3:\n%s", threadCount, text)
	}
}

func newMetadataClient(t *testing.T) (*sdk.ClientSession, *storage.SqliteStore) {
	t.Helper()

	store := newTestStore(t)
	server, err := NewServer(store)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	serverSession, err := server.mcp.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "metadata-test", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		t.Fatalf("connect client: %v", err)
	}

	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
		_ = store.Close()
	})
	return clientSession, store
}
