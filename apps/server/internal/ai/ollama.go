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
	baseURL        string
	summaryModel   string
	embeddingModel string
	httpClient     *http.Client
}

// NewOllamaClient creates an Ollama client instance.
func NewOllamaClient(cfg *config.AIConfig) *OllamaClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	summaryModel := cfg.SummaryModel
	if summaryModel == "" {
		summaryModel = "llama3.2:3b"
	}
	embeddingModel := cfg.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "nomic-embed-text"
	}

	return &OllamaClient{
		baseURL:        baseURL,
		summaryModel:   summaryModel,
		embeddingModel: embeddingModel,
		httpClient:     &http.Client{Timeout: defaultTimeout},
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

	return res.Embedding, nil
}
