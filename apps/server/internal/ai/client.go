package ai

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/shelfd/shelfd/internal/config"
)

// NormalizeAndTruncateMRL truncates an embedding vector to targetDim and L2-normalizes it.
func NormalizeAndTruncateMRL(vec []float32, targetDim int) []float32 {
	if len(vec) == 0 {
		return vec
	}
	if targetDim > 0 && len(vec) > targetDim {
		vec = vec[:targetDim]
	}

	var sumSq float64
	for _, v := range vec {
		sumSq += float64(v * v)
	}
	if sumSq == 0 {
		return vec
	}

	norm := float32(1.0 / math.Sqrt(sumSq))
	res := make([]float32, len(vec))
	for i, v := range vec {
		res[i] = v * norm
	}
	return res
}

// ChatMessage represents an individual conversational message.
type ChatMessage struct {
	Role       string     `json:"role"` // "system", "user", "assistant", "tool"
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ChatResponse represents the outcome of a chat completion step.
type ChatResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Embedder defines the interface for batch and single vector embedding generation.
type Embedder interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
}

// ChatClient defines the interface for LLM chat and chapter summarization.
type ChatClient interface {
	SummarizeChapter(ctx context.Context, title, content string) (string, error)
	Chat(ctx context.Context, messages []ChatMessage) (string, error)
	ChatWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition) (*ChatResponse, error)
}

// Client defines the universal composite interface for embeddings, chat, and summarization.
type Client interface {
	Embedder
	ChatClient
}

// CompositeClient wraps separate Embedder and ChatClient instances into a single Client.
type CompositeClient struct {
	Embedder
	ChatClient
}

// ChunkTexts partitions a slice of strings into chunks of at most maxBatchSize.
func ChunkTexts(texts []string, maxBatchSize int) [][]string {
	if len(texts) == 0 {
		return nil
	}
	if maxBatchSize <= 0 || len(texts) <= maxBatchSize {
		return [][]string{texts}
	}
	var chunks [][]string
	for i := 0; i < len(texts); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		chunks = append(chunks, texts[i:end])
	}
	return chunks
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

// SanitizeURL strips credentials from a URL for safe logging.
func SanitizeURL(rawURL string) string {
	if strings.TrimSpace(rawURL) == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsed.User != nil {
		parsed.User = url.UserPassword(parsed.User.Username(), "REDACTED")
	}
	return parsed.String()
}

// NewEmbedder instantiates an embedding client based on endpoint configuration.
func NewEmbedder(cfg *config.ModelEndpointConfig) (Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedder endpoint config cannot be nil")
	}
	if err := config.ValidateSSRF(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("embedder base_url contains %w", err)
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "ollama":
		return NewOllamaEmbedder(cfg), nil
	case "openai":
		return NewOpenAIEmbedder(cfg), nil
	case "google", "gemini":
		return NewGoogleEmbedder(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported ai embedder provider: %s", cfg.Provider)
	}
}

// NewChatClient instantiates a chat/summarization client based on endpoint configuration.
func NewChatClient(cfg *config.ModelEndpointConfig) (ChatClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("chat endpoint config cannot be nil")
	}
	if err := config.ValidateSSRF(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("chat base_url contains %w", err)
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "ollama":
		return NewOllamaChatClient(cfg), nil
	case "openai":
		return NewOpenAIChatClient(cfg), nil
	case "google", "gemini":
		return NewGoogleChatClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported ai chat provider: %s", cfg.Provider)
	}
}

// NewClient instantiates a composite AI client, decoupling embedder and chat endpoints.
func NewClient(cfg *config.AIConfig) (Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("ai config cannot be nil")
	}

	// Resolve Embedder config with fallback to top-level legacy fields
	embedderCfg := cfg.Embedder
	if embedderCfg.Provider == "" {
		embedderCfg.Provider = cfg.Provider
	}
	if embedderCfg.BaseURL == "" {
		embedderCfg.BaseURL = cfg.BaseURL
	}
	if embedderCfg.APIKey == "" {
		embedderCfg.APIKey = cfg.APIKey
	}
	if embedderCfg.Model == "" {
		embedderCfg.Model = cfg.EmbeddingModel
	}
	if embedderCfg.Dimensions <= 0 {
		embedderCfg.Dimensions = cfg.EmbeddingDimensions
	}

	// Resolve Chat config with fallback to top-level legacy fields
	chatCfg := cfg.Chat
	if chatCfg.Provider == "" {
		chatCfg.Provider = cfg.Provider
	}
	if chatCfg.BaseURL == "" {
		chatCfg.BaseURL = cfg.BaseURL
	}
	if chatCfg.APIKey == "" {
		chatCfg.APIKey = cfg.APIKey
	}
	if chatCfg.Model == "" {
		if cfg.ChatModel != "" {
			chatCfg.Model = cfg.ChatModel
		} else {
			chatCfg.Model = cfg.SummaryModel
		}
	}

	embedder, err := NewEmbedder(&embedderCfg)
	if err != nil {
		return nil, fmt.Errorf("initializing embedder: %w", err)
	}

	chatClient, err := NewChatClient(&chatCfg)
	if err != nil {
		return nil, fmt.Errorf("initializing chat client: %w", err)
	}

	return &CompositeClient{
		Embedder:   embedder,
		ChatClient: chatClient,
	}, nil
}

const defaultTimeout = 60 * time.Second
