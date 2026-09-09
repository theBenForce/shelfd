package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shelfd/shelfd/internal/config"
)

// OllamaClient communicates with a local or remote Ollama instance.
type OllamaClient struct {
	baseURL             string
	summaryModel        string
	embeddingModel      string
	embeddingDimensions int
	httpClient          *http.Client
}

// NewOllamaClient creates an Ollama client instance.
func NewOllamaClient(cfg *config.AIConfig) *OllamaClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	chatModel := cfg.ChatModel
	if chatModel == "" {
		chatModel = cfg.SummaryModel
	}
	if chatModel == "" {
		chatModel = "llama3.2:3b"
	}
	embeddingModel := cfg.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "nomic-embed-text"
	}
	embeddingDimensions := cfg.EmbeddingDimensions
	if embeddingDimensions <= 0 {
		embeddingDimensions = 256
	}

	return &OllamaClient{
		baseURL:             baseURL,
		summaryModel:        chatModel,
		embeddingModel:      embeddingModel,
		embeddingDimensions: embeddingDimensions,
		httpClient:          &http.Client{Timeout: defaultTimeout},
	}
}

func (c *OllamaClient) SummarizeChapter(ctx context.Context, title, content string) (string, error) {
	prompt := FormatSummarizePrompt(title, content)

	payload := map[string]interface{}{
		"model":  c.summaryModel,
		"prompt": prompt,
		"stream": false,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling ollama generate request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending ollama generate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama generate returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("decoding ollama generate response: %w", err)
	}

	return strings.TrimSpace(res.Response), nil
}

func (c *OllamaClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	payload := map[string]interface{}{
		"model":  c.embeddingModel,
		"prompt": text,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling ollama embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending ollama embedding request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embedding returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decoding ollama embedding response: %w", err)
	}

	return NormalizeAndTruncateMRL(res.Embedding, c.embeddingDimensions), nil
}

func (c *OllamaClient) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	chunks := ChunkTexts(texts, 50)
	var allEmbeddings [][]float32

	for _, chunk := range chunks {
		payload := map[string]interface{}{
			"model": c.embeddingModel,
			"input": chunk,
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshaling ollama batch embedding request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("sending ollama batch embedding request: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			var res struct {
				Embeddings [][]float32 `json:"embeddings"`
			}
			err := json.NewDecoder(resp.Body).Decode(&res)
			resp.Body.Close()
			if err == nil && len(res.Embeddings) == len(chunk) {
				for _, emb := range res.Embeddings {
					allEmbeddings = append(allEmbeddings, NormalizeAndTruncateMRL(emb, c.embeddingDimensions))
				}
				continue
			}
		} else {
			resp.Body.Close()
		}

		// Fallback for older Ollama daemon without /api/embed endpoint
		for _, t := range chunk {
			emb, err := c.GenerateEmbedding(ctx, t)
			if err != nil {
				return nil, err
			}
			allEmbeddings = append(allEmbeddings, emb)
		}
	}

	return allEmbeddings, nil
}

func (c *OllamaClient) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	ollamaMessages := make([]map[string]string, len(messages))
	for i, m := range messages {
		ollamaMessages[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}

	payload := map[string]interface{}{
		"model":    c.summaryModel,
		"messages": ollamaMessages,
		"stream":   false,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling ollama chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending ollama chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama chat returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("decoding ollama chat response: %w", err)
	}

	return strings.TrimSpace(res.Message.Content), nil
}

