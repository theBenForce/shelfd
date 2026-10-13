package ai_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/ai"
)

type mockChatClientWithTools struct {
	turnCount     int
	chatWithTools func(ctx context.Context, messages []ai.ChatMessage, tools []ai.ToolDefinition) (*ai.ChatResponse, error)
}

func (m *mockChatClientWithTools) SummarizeChapter(ctx context.Context, title, content string) (string, error) {
	return "Summary", nil
}

func (m *mockChatClientWithTools) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	resp, err := m.ChatWithTools(ctx, messages, nil)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (m *mockChatClientWithTools) ChatWithTools(ctx context.Context, messages []ai.ChatMessage, tools []ai.ToolDefinition) (*ai.ChatResponse, error) {
	m.turnCount++
	if m.chatWithTools != nil {
		return m.chatWithTools(ctx, messages, tools)
	}
	return &ai.ChatResponse{Content: "Default reply"}, nil
}

type mockToolInvoker struct {
	lastExecutedTool string
	lastArgs         map[string]any
	definitions      []ai.ToolDefinition
	executeFunc      func(ctx context.Context, name string, args map[string]any) (*ai.ToolExecutionResult, error)
}

func (m *mockToolInvoker) AvailableToolDefinitions() []ai.ToolDefinition {
	return m.definitions
}

func (m *mockToolInvoker) ExecuteInProcess(ctx context.Context, name string, args map[string]any) (*ai.ToolExecutionResult, error) {
	m.lastExecutedTool = name
	m.lastArgs = args
	if m.executeFunc != nil {
		return m.executeFunc(ctx, name, args)
	}
	return &ai.ToolExecutionResult{
		Content: "Mock tool output",
	}, nil
}

func TestChatAgent_DirectAnswer(t *testing.T) {
	ctx := context.Background()

	client := &mockChatClientWithTools{
		chatWithTools: func(ctx context.Context, messages []ai.ChatMessage, tools []ai.ToolDefinition) (*ai.ChatResponse, error) {
			return &ai.ChatResponse{
				Content: "Paul Atreides is the son of Duke Leto.",
			}, nil
		},
	}

	invoker := &mockToolInvoker{
		definitions: []ai.ToolDefinition{{Name: "search_library"}},
	}

	agent := ai.NewChatAgent(client, invoker)
	res, err := agent.Execute(ctx, "book_123", []ai.ChatMessage{
		{Role: "user", Content: "Who is Paul Atreides?"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Reply != "Paul Atreides is the son of Duke Leto." {
		t.Errorf("expected reply, got %s", res.Reply)
	}
	if len(res.Citations) != 0 {
		t.Errorf("expected 0 citations for direct answer, got %d", len(res.Citations))
	}
	if client.turnCount != 1 {
		t.Errorf("expected 1 turn, got %d", client.turnCount)
	}
}

func TestChatAgent_ToolCallingAndBookScoping(t *testing.T) {
	ctx := context.Background()

	invoker := &mockToolInvoker{
		definitions: []ai.ToolDefinition{{Name: "search_library"}},
		executeFunc: func(ctx context.Context, name string, args map[string]any) (*ai.ToolExecutionResult, error) {
			if args["book_id"] != "book_target" {
				t.Errorf("expected book_id to be forced to 'book_target', got %v", args["book_id"])
			}
			return &ai.ToolExecutionResult{
				Content: "Found passage about the gom jabbar.",
				Citations: []ai.BookCitation{
					{
						BookID:         "book_target",
						BookTitle:      "Dune",
						ChapterIndex:   1,
						StartParagraph: 5,
						EndParagraph:   8,
						Excerpt:        "What's in the box? Pain.",
					},
				},
			}, nil
		},
	}

	callCount := 0
	client := &mockChatClientWithTools{
		chatWithTools: func(ctx context.Context, messages []ai.ChatMessage, tools []ai.ToolDefinition) (*ai.ChatResponse, error) {
			callCount++
			if callCount == 1 {
				// Turn 1: Model requests tool call with spoofed book_id
				return &ai.ChatResponse{
					ToolCalls: []ai.ToolCall{
						{
							ID:        "call_1",
							Name:      "search_library",
							Arguments: `{"query":"box of pain","book_id":"spoofed_book"}`,
						},
					},
				}, nil
			}

			// Turn 2: Verify tool result was delivered in messages
			foundToolResult := false
			for _, m := range messages {
				if m.Role == "tool" && strings.Contains(m.Content, "<retrieved_passage>") {
					foundToolResult = true
					break
				}
			}
			if !foundToolResult {
				t.Errorf("expected tool result wrapped in <retrieved_passage> in messages")
			}

			return &ai.ChatResponse{
				Content: "Paul survived the test of the gom jabbar.",
			}, nil
		},
	}

	agent := ai.NewChatAgent(client, invoker)
	res, err := agent.Execute(ctx, "book_target", []ai.ChatMessage{
		{Role: "user", Content: "Tell me about the box of pain."},
	})
	if err != nil {
		t.Fatalf("agent execute error: %v", err)
	}

	if res.Reply != "Paul survived the test of the gom jabbar." {
		t.Errorf("expected reply, got %s", res.Reply)
	}
	if len(res.Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(res.Citations))
	}
	if res.Citations[0].BookID != "book_target" {
		t.Errorf("expected citation book_id 'book_target', got %s", res.Citations[0].BookID)
	}
	if callCount != 2 {
		t.Errorf("expected 2 turns, got %d", callCount)
	}
}

func TestChatAgent_TurnCeilingGuaranteedSynthesis(t *testing.T) {
	ctx := context.Background()

	invoker := &mockToolInvoker{
		definitions: []ai.ToolDefinition{{Name: "search_library"}},
		executeFunc: func(ctx context.Context, name string, args map[string]any) (*ai.ToolExecutionResult, error) {
			return &ai.ToolExecutionResult{
				Content: fmt.Sprintf("Passage from search on query %v", args["query"]),
				Citations: []ai.BookCitation{
					{
						BookID:       "book_1",
						ChapterIndex: 1,
						Excerpt:      "Sample excerpt",
					},
				},
			}, nil
		},
	}

	client := &mockChatClientWithTools{
		chatWithTools: func(ctx context.Context, messages []ai.ChatMessage, tools []ai.ToolDefinition) (*ai.ChatResponse, error) {
			// At turn 4 (turn ceiling), tools must be nil
			if len(tools) == 0 {
				return &ai.ChatResponse{
					Content: "Synthesized final answer after 4 turns.",
				}, nil
			}
			// Keep requesting tool calls
			return &ai.ChatResponse{
				ToolCalls: []ai.ToolCall{
					{
						ID:        "call_loop",
						Name:      "search_library",
						Arguments: `{"query":"more details"}`,
					},
				},
			}, nil
		},
	}

	agent := ai.NewChatAgent(client, invoker)
	res, err := agent.Execute(ctx, "book_1", []ai.ChatMessage{
		{Role: "user", Content: "Complex research query"},
	})
	if err != nil {
		t.Fatalf("agent execute error: %v", err)
	}

	if res.Reply != "Synthesized final answer after 4 turns." {
		t.Errorf("expected synthesized reply on ceiling, got: %s", res.Reply)
	}
	if client.turnCount != 4 {
		t.Errorf("expected exactly 4 turns, got %d", client.turnCount)
	}
	if len(res.Citations) != 3 {
		t.Errorf("expected 3 accumulated citations from earlier turns, got %d", len(res.Citations))
	}
}
