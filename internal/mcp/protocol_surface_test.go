// ABOUTME: Exercises MCP resource and prompt discovery through the real SDK protocol boundary.
// ABOUTME: Verifies protocol metadata and rendered content against a temporary SQLite store.

package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/harper/bbs/internal/models"
	"github.com/harper/bbs/internal/storage"
)

func TestProtocolResourceDiscovery(t *testing.T) {
	client, _ := newMetadataClient(t)
	ctx := context.Background()

	resources, err := client.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}
	wantResources := []resourceMetadata{
		{
			URI:         "bbs://topics",
			Name:        "All Topics",
			Description: "List of all active topics",
			MIMEType:    "application/json",
		},
		{
			URI:         "bbs://recent",
			Name:        "Recent Activity",
			Description: "Up to three threads per active topic, ordered sticky first then by most recent update; message content is not included",
			MIMEType:    "text/markdown",
		},
	}
	gotResources := make([]resourceMetadata, 0, len(resources.Resources))
	for _, resource := range resources.Resources {
		gotResources = append(gotResources, resourceMetadata{
			URI:         resource.URI,
			Name:        resource.Name,
			Description: resource.Description,
			MIMEType:    resource.MIMEType,
		})
	}
	slices.SortFunc(gotResources, func(a, b resourceMetadata) int { return strings.Compare(a.URI, b.URI) })
	slices.SortFunc(wantResources, func(a, b resourceMetadata) int { return strings.Compare(a.URI, b.URI) })
	if !reflect.DeepEqual(gotResources, wantResources) {
		t.Errorf("resource metadata = %#v, want %#v", gotResources, wantResources)
	}

	templates, err := client.ListResourceTemplates(ctx, nil)
	if err != nil {
		t.Fatalf("ListResourceTemplates failed: %v", err)
	}
	wantTemplates := []resourceTemplateMetadata{
		{
			URITemplate: "bbs://topics/{topic}/threads",
			Name:        "Topic Threads",
			Description: "Threads in a specific topic",
			MIMEType:    "application/json",
		},
		{
			URITemplate: "bbs://threads/{thread}/messages",
			Name:        "Thread Messages",
			Description: "Messages in a specific thread",
			MIMEType:    "text/markdown",
		},
	}
	gotTemplates := make([]resourceTemplateMetadata, 0, len(templates.ResourceTemplates))
	for _, template := range templates.ResourceTemplates {
		gotTemplates = append(gotTemplates, resourceTemplateMetadata{
			URITemplate: template.URITemplate,
			Name:        template.Name,
			Description: template.Description,
			MIMEType:    template.MIMEType,
		})
	}
	slices.SortFunc(gotTemplates, func(a, b resourceTemplateMetadata) int {
		return strings.Compare(a.URITemplate, b.URITemplate)
	})
	slices.SortFunc(wantTemplates, func(a, b resourceTemplateMetadata) int {
		return strings.Compare(a.URITemplate, b.URITemplate)
	})
	if !reflect.DeepEqual(gotTemplates, wantTemplates) {
		t.Errorf("resource template metadata = %#v, want %#v", gotTemplates, wantTemplates)
	}
}

func TestProtocolResourceReads(t *testing.T) {
	client, store := newMetadataClient(t)
	fixture := seedProtocolSurface(t, store)

	topics := readProtocolResource(t, client, "bbs://topics", "application/json")
	var decodedTopics []*models.Topic
	if err := json.Unmarshal([]byte(topics.Text), &decodedTopics); err != nil {
		t.Fatalf("decode bbs://topics content: %v", err)
	}
	if want := []*models.Topic{fixture.activeTopic}; !reflect.DeepEqual(decodedTopics, want) {
		t.Errorf("bbs://topics data = %#v, want %#v", decodedTopics, want)
	}

	recent := readProtocolResource(t, client, "bbs://recent", "text/markdown")
	for _, want := range []string{"## protocol-alpha", "**Protocol Thread**", "thread-author@mcp"} {
		if !strings.Contains(recent.Text, want) {
			t.Errorf("bbs://recent is missing %q:\n%s", want, recent.Text)
		}
	}
	for _, excluded := range []string{"protocol-archived", fixture.message.Content} {
		if strings.Contains(recent.Text, excluded) {
			t.Errorf("bbs://recent unexpectedly contains %q:\n%s", excluded, recent.Text)
		}
	}

	threadURI := "bbs://topics/protocol-alpha/threads"
	threads := readProtocolResource(t, client, threadURI, "application/json")
	var decodedThreads []*models.Thread
	if err := json.Unmarshal([]byte(threads.Text), &decodedThreads); err != nil {
		t.Fatalf("decode %s content: %v", threadURI, err)
	}
	storedThread, err := store.GetThread(fixture.thread.ID)
	if err != nil {
		t.Fatalf("GetThread for expected resource data: %v", err)
	}
	if want := []*models.Thread{storedThread}; !reflect.DeepEqual(decodedThreads, want) {
		t.Errorf("%s data = %#v, want %#v", threadURI, decodedThreads, want)
	}

	messageURI := "bbs://threads/" + fixture.thread.ID.String() + "/messages"
	messages := readProtocolResource(t, client, messageURI, "text/markdown")
	wantMessages := "# Protocol Thread\n\n" +
		"*Started by thread-author@mcp on 2026-01-02*\n\n" +
		"---\n\n" +
		"**message-author@mcp** · Jul 10 12:34\n\n" +
		"Golden message body\n\n" +
		"---\n\n"
	if messages.Text != wantMessages {
		t.Errorf("%s content = %q, want %q", messageURI, messages.Text, wantMessages)
	}
}

func TestProtocolPromptDiscoveryAndRendering(t *testing.T) {
	client, _ := newMetadataClient(t)
	ctx := context.Background()

	result, err := client.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}
	wantMetadata := []promptMetadata{
		{
			Name:        "post-update",
			Description: "Post a status update to a topic",
			Arguments: []promptArgumentMetadata{
				{Name: "topic", Description: "Topic to post to", Required: true},
				{Name: "subject", Description: "Thread subject", Required: true},
			},
		},
		{
			Name:        "summarize-thread",
			Description: "Summarize a thread discussion",
			Arguments: []promptArgumentMetadata{
				{Name: "thread", Description: "Thread ID to summarize", Required: true},
			},
		},
	}
	gotMetadata := make([]promptMetadata, 0, len(result.Prompts))
	for _, prompt := range result.Prompts {
		metadata := promptMetadata{Name: prompt.Name, Description: prompt.Description}
		for _, argument := range prompt.Arguments {
			metadata.Arguments = append(metadata.Arguments, promptArgumentMetadata{
				Name: argument.Name, Description: argument.Description, Required: argument.Required,
			})
		}
		gotMetadata = append(gotMetadata, metadata)
	}
	slices.SortFunc(gotMetadata, func(a, b promptMetadata) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(wantMetadata, func(a, b promptMetadata) int { return strings.Compare(a.Name, b.Name) })
	if !reflect.DeepEqual(gotMetadata, wantMetadata) {
		t.Errorf("prompt metadata = %#v, want %#v", gotMetadata, wantMetadata)
	}

	tests := []struct {
		name        string
		arguments   map[string]string
		description string
		text        string
	}{
		{
			name:        "post-update",
			arguments:   map[string]string{"topic": "protocol-alpha", "subject": "Release status"},
			description: "Post update to protocol-alpha: Release status",
			text: "Post a status update to the BBS.\n\n" +
				"Topic: protocol-alpha\nSubject: Release status\n\n" +
				"Please use the create_thread tool with your update message. Keep it concise and informative.",
		},
		{
			name:        "summarize-thread",
			arguments:   map[string]string{"thread": "11111111"},
			description: "Summarize thread 11111111",
			text: "Please summarize the discussion in thread 11111111.\n\n" +
				"First, use the list_messages tool to read the thread, then provide a concise summary of:\n" +
				"1. The main topic/question\n2. Key points discussed\n3. Any conclusions or action items",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := client.GetPrompt(ctx, &sdk.GetPromptParams{Name: tt.name, Arguments: tt.arguments})
			if err != nil {
				t.Fatalf("GetPrompt failed: %v", err)
			}
			if prompt.Description != tt.description {
				t.Errorf("description = %q, want %q", prompt.Description, tt.description)
			}
			if len(prompt.Messages) != 1 {
				t.Fatalf("message count = %d, want 1", len(prompt.Messages))
			}
			message := prompt.Messages[0]
			if message.Role != sdk.Role("user") {
				t.Errorf("message role = %q, want user", message.Role)
			}
			content, ok := message.Content.(*sdk.TextContent)
			if !ok {
				t.Fatalf("message content type = %T, want *mcp.TextContent", message.Content)
			}
			if content.Text != tt.text {
				t.Errorf("message text = %q, want %q", content.Text, tt.text)
			}
		})
	}
}

type resourceMetadata struct {
	URI         string
	Name        string
	Description string
	MIMEType    string
}

type resourceTemplateMetadata struct {
	URITemplate string
	Name        string
	Description string
	MIMEType    string
}

type promptMetadata struct {
	Name        string
	Description string
	Arguments   []promptArgumentMetadata
}

type promptArgumentMetadata struct {
	Name        string
	Description string
	Required    bool
}

type protocolSurfaceFixture struct {
	activeTopic *models.Topic
	thread      *models.Thread
	message     *models.Message
}

func seedProtocolSurface(t *testing.T, store *storage.SqliteStore) protocolSurfaceFixture {
	t.Helper()

	active := &models.Topic{
		ID:          mustProtocolID(t, "00000000-0000-0000-0000-000000000001"),
		Name:        "protocol-alpha",
		Description: "Active protocol topic",
		CreatedAt:   time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		CreatedBy:   "topic-author@mcp",
	}
	archived := &models.Topic{
		ID:          mustProtocolID(t, "00000000-0000-0000-0000-000000000002"),
		Name:        "protocol-archived",
		Description: "Archived protocol topic",
		CreatedAt:   time.Date(2026, time.January, 3, 3, 4, 5, 0, time.UTC),
		CreatedBy:   "topic-author@mcp",
		Archived:    true,
	}
	thread := &models.Thread{
		ID:        mustProtocolID(t, "00000000-0000-0000-0000-000000000003"),
		TopicID:   active.ID,
		Subject:   "Protocol Thread",
		CreatedAt: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		CreatedBy: "thread-author@mcp",
		UpdatedAt: time.Date(2026, time.July, 9, 11, 22, 33, 0, time.UTC),
		Sticky:    true,
	}
	message := &models.Message{
		ID:        mustProtocolID(t, "00000000-0000-0000-0000-000000000004"),
		ThreadID:  thread.ID,
		Content:   "Golden message body",
		CreatedAt: time.Date(2026, time.July, 10, 12, 34, 56, 0, time.UTC),
		CreatedBy: "message-author@mcp",
	}

	creates := []struct {
		label string
		call  func() error
	}{
		{label: "active topic", call: func() error { return store.CreateTopic(active) }},
		{label: "archived topic", call: func() error { return store.CreateTopic(archived) }},
		{label: "thread", call: func() error { return store.CreateThread(thread) }},
		{label: "message", call: func() error { return store.CreateMessage(message) }},
	}
	for _, create := range creates {
		if err := create.call(); err != nil {
			t.Fatalf("create %s: %v", create.label, err)
		}
	}

	return protocolSurfaceFixture{activeTopic: active, thread: thread, message: message}
}

func readProtocolResource(
	t *testing.T,
	client *sdk.ClientSession,
	uri string,
	wantMIMEType string,
) *sdk.ResourceContents {
	t.Helper()

	result, err := client.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("ReadResource(%q) failed: %v", uri, err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("ReadResource(%q) returned %d contents, want 1", uri, len(result.Contents))
	}
	content := result.Contents[0]
	if content.URI != uri {
		t.Errorf("ReadResource(%q) content URI = %q", uri, content.URI)
	}
	if content.MIMEType != wantMIMEType {
		t.Errorf("ReadResource(%q) MIME type = %q, want %q", uri, content.MIMEType, wantMIMEType)
	}
	return content
}

func mustProtocolID(t *testing.T, value string) models.UUID {
	t.Helper()

	id, err := models.ParseUUID(value)
	if err != nil {
		t.Fatalf("parse protocol fixture UUID %q: %v", value, err)
	}
	return id
}
