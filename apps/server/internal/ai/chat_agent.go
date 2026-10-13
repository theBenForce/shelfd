package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolInvoker defines an in-process tool executor interface.
type ToolInvoker interface {
	ExecuteInProcess(ctx context.Context, name string, args map[string]any) (*ToolExecutionResult, error)
	AvailableToolDefinitions() []ToolDefinition
}

// ChatAgent coordinates agentic multi-turn conversations with tool calling.
type ChatAgent struct {
	client   ChatClient
	invoker  ToolInvoker
	maxTurns int
}

// NewChatAgent creates a new ChatAgent instance.
func NewChatAgent(client ChatClient, invoker ToolInvoker) *ChatAgent {
	return &ChatAgent{
		client:   client,
		invoker:  invoker,
		maxTurns: 4,
	}
}

// ChatAgentResult contains the assistant reply and accumulated citations.
type ChatAgentResult struct {
	Reply     string
	Citations []BookCitation
}

// Execute runs the agentic conversation loop for a specific book.
func (a *ChatAgent) Execute(ctx context.Context, bookID string, initialMessages []ChatMessage) (*ChatAgentResult, error) {
	messages := make([]ChatMessage, len(initialMessages))
	copy(messages, initialMessages)

	var tools []ToolDefinition
	if a.invoker != nil {
		tools = a.invoker.AvailableToolDefinitions()
	}

	var allCitations []BookCitation

	for turn := 1; turn <= a.maxTurns; turn++ {
		// At turn ceiling (turn == maxTurns), force model to synthesize text without tools
		activeTools := tools
		if turn == a.maxTurns {
			activeTools = nil
		}

		resp, err := a.client.ChatWithTools(ctx, messages, activeTools)
		if err != nil {
			return nil, fmt.Errorf("llm chat error at turn %d: %w", turn, err)
		}

		// If the model produced a final text response without requesting tool calls
		if len(resp.ToolCalls) == 0 || activeTools == nil {
			return &ChatAgentResult{
				Reply:     strings.TrimSpace(resp.Content),
				Citations: allCitations,
			}, nil
		}

		// Add assistant message with tool calls to history
		messages = append(messages, ChatMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute all requested tool calls
		for _, tc := range resp.ToolCalls {
			var args map[string]any
			if strings.TrimSpace(tc.Arguments) != "" {
				_ = json.Unmarshal([]byte(tc.Arguments), &args)
			}
			if args == nil {
				args = make(map[string]any)
			}

			// Security guard: Enforce book_id scoping strictly to current book
			if tc.Name == "search_library" || tc.Name == "get_book_metadata" || tc.Name == "read_chapter_content" {
				args["book_id"] = bookID
			}

			toolRes, err := a.invoker.ExecuteInProcess(ctx, tc.Name, args)
			resultContent := ""
			if err != nil {
				resultContent = fmt.Sprintf("Error executing tool %s: %v", tc.Name, err)
			} else if toolRes.IsError {
				resultContent = fmt.Sprintf("Tool error: %s", toolRes.Content)
			} else {
				// Wrap retrieved passage in XML tags to defend against prompt injection
				resultContent = fmt.Sprintf("<retrieved_passage>\n%s\n</retrieved_passage>", toolRes.Content)
				allCitations = append(allCitations, toolRes.Citations...)
			}

			messages = append(messages, ChatMessage{
				Role:       "tool",
				Content:    resultContent,
				ToolCallID: tc.ID,
			})
		}
	}

	return &ChatAgentResult{
		Reply:     "I was unable to complete the answer within the maximum allowed steps.",
		Citations: allCitations,
	}, nil
}
