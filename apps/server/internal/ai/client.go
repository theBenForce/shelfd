package ai

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/shelfd/shelfd/internal/config"
)

// ChatMessage represents an individual conversational message.
type ChatMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// Client defines the universal interface for LLM summarization, vector embedding generation, and chat.
type Client interface {
	SummarizeChapter(ctx context.Context, title, content string) (string, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	Chat(ctx context.Context, messages []ChatMessage) (string, error)
}

// SystemSummarizePrompt defines the prompt constraints for chapter summarization.
const SystemSummarizePrompt = `You are a literary archivist. Provide a dense, precise 2 to 4 sentence summary of the following chapter text. Focus strictly on key character actions, critical decisions, overarching thematic concepts, and essential plot progression. Do NOT use introductory or filler phrases such as 'In this chapter', 'This chapter explores', or 'The narrative continues'.`

// FormatSummarizePrompt constructs the prompt given chapter title and body text.
func FormatSummarizePrompt(title, content string) string {
	var sb strings.Builder
	sb.WriteString(SystemSummarizePrompt)
	sb.WriteString("\n\n")
	if strings.TrimSpace(title) != "" {
		sb.WriteString("Chapter Title: ")
		sb.WriteString(title)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Chapter Text:\n")

	// Limit text passed to LLM to ~20,000 characters to prevent context window overflow
	if len(content) > 20000 {
		sb.WriteString(content[:20000])
		sb.WriteString("\n[...text truncated for summarization...]")
	} else {
		sb.WriteString(content)
	}

	return sb.String()
}

// NewClient instantiates an AI client based on configuration with SSRF safeguards.
func NewClient(cfg *config.AIConfig) (Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("ai config cannot be nil")
	}

	if cfg.BaseURL != "" {
		parsed, err := url.Parse(cfg.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid ai.base_url: %w", err)
		}
		hostname := parsed.Hostname()
		if hostname == "169.254.169.254" || strings.HasPrefix(hostname, "169.254.") {
			return nil, fmt.Errorf("ai.base_url contains prohibited cloud metadata address: %s", hostname)
		}
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "ollama":
		return NewOllamaClient(cfg), nil
	case "openai":
		return NewOpenAIClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported ai provider: %s", cfg.Provider)
	}
}

const defaultTimeout = 60 * time.Second
