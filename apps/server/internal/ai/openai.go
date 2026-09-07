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

// OpenAIClient communicates with an OpenAI-compatible endpoint.
type OpenAIClient struct {
	baseURL        string
	apiKey         string
	summaryModel   string
	embeddingModel string
	httpClient     *http.Client
}

// NewOpenAIClient creates an OpenAI-compatible client instance.
func NewOpenAIClient(cfg *config.AIConfig) *OpenAIClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	summaryModel := cfg.SummaryModel
	if summaryModel == "" {
		summaryModel = "gpt-4o-mini"
	}
	embeddingModel := cfg.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "text-embedding-3-small"
	}

	return &OpenAIClient{
		baseURL:        baseURL,
		apiKey:         cfg.APIKey,
		summaryModel:   summaryModel,
		embeddingModel: embeddingModel,
		httpClient:     &http.Client{Timeout: defaultTimeout},
	}
}

func (c *OpenAIClient) SummarizeChapter(ctx context.Context, title, content string) (string, error) {
	prompt := FormatSummarizePrompt(title, content)

	payload := map[string]interface{}{
		"model": c.summaryModel,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": SystemSummarizePrompt,
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling openai chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending openai chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai chat returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("decoding openai chat response: %w", err)
	}

	if len(res.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned by openai")
	}

	return strings.TrimSpace(res.Choices[0].Message.Content), nil
}

func (c *OpenAIClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	payload := map[string]interface{}{
		"model": c.embeddingModel,
		"input": text,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling openai embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending openai embedding request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai embedding returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decoding openai embedding response: %w", err)
	}

	if len(res.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned by openai")
	}

	return res.Data[0].Embedding, nil
}
