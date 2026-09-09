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
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// Client defines the universal interface for LLM chat, chapter summarization, and batch vector embedding generation.
type Client interface {
	SummarizeChapter(ctx context.Context, title, content string) (string, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
	Chat(ctx context.Context, messages []ChatMessage) (string, error)
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
	case "google", "gemini":
		return NewGoogleClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported ai provider: %s", cfg.Provider)
	}
}

const defaultTimeout = 60 * time.Second
