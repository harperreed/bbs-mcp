// ABOUTME: MCP prompt templates
// ABOUTME: Guided workflows for common tasks

package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerPrompts() {
	s.mcp.AddPrompt(&mcp.Prompt{
		Name:        "post-update",
		Description: "Post a status update to a topic",
		Arguments: []*mcp.PromptArgument{
			{Name: "topic", Description: "Topic to post to", Required: true},
			{Name: "subject", Description: "Thread subject", Required: true},
		},
	}, s.handlePostUpdatePrompt)

	s.mcp.AddPrompt(&mcp.Prompt{
		Name:        "summarize-thread",
		Description: "Summarize a thread discussion",
		Arguments: []*mcp.PromptArgument{
			{Name: "thread", Description: "Thread ID to summarize", Required: true},
		},
	}, s.handleSummarizePrompt)
}

func (s *Server) handlePostUpdatePrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	topic := req.Params.Arguments["topic"]
	subject := req.Params.Arguments["subject"]

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Post update to %s: %s", topic, subject),
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(`Post a status update to the BBS.

Topic: %s
Subject: %s

Please use the create_thread tool with your update message. Keep it concise and informative.`, topic, subject),
				},
			},
		},
	}, nil
}

func (s *Server) handleSummarizePrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	threadID := req.Params.Arguments["thread"]

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Summarize thread %s", threadID),
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(`Please summarize the discussion in thread %s.

First, use the list_messages tool to read the thread, then provide a concise summary of:
1. The main topic/question
2. Key points discussed
3. Any conclusions or action items`, threadID),
				},
			},
		},
	}, nil
}
