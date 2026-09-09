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
	baseURL             string
	apiKey              string
	summaryModel        string
	embeddingModel      string
	embeddingDimensions int
	httpClient          *http.Client
}

// NewOpenAIClient creates an OpenAI-compatible client instance.
func NewOpenAIClient(cfg *config.AIConfig) *OpenAIClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	chatModel := cfg.ChatModel
	if chatModel == "" {
		chatModel = cfg.SummaryModel
	}
	if chatModel == "" {
		chatModel = "gpt-4o-mini"
	}
	embeddingModel := cfg.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "text-embedding-3-small"
	}
	embeddingDimensions := cfg.EmbeddingDimensions
	if embeddingDimensions <= 0 {
		embeddingDimensions = 256
	}

	return &OpenAIClient{
		baseURL:             baseURL,
		apiKey:              cfg.APIKey,
		summaryModel:        chatModel,
		embeddingModel:      embeddingModel,
		embeddingDimensions: embeddingDimensions,
		httpClient:          &http.Client{Timeout: defaultTimeout},
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
	results, err := c.GenerateBatchEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no embedding returned by openai")
	}
	return results[0], nil
}

func (c *OpenAIClient) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	chunks := ChunkTexts(texts, 100)
	var allEmbeddings [][]float32

	for _, chunk := range chunks {
		payload := map[string]interface{}{
			"model": c.embeddingModel,
			"input": chunk,
		}
		if c.embeddingDimensions > 0 {
			payload["dimensions"] = c.embeddingDimensions
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshaling openai batch embedding request: %w", err)
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
			return nil, fmt.Errorf("sending openai batch embedding request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("openai batch embedding returned status %d: %s", resp.StatusCode, string(respBody))
		}

		var res struct {
			Data []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, fmt.Errorf("decoding openai batch embedding response: %w", err)
		}

		if len(res.Data) != len(chunk) {
			return nil, fmt.Errorf("expected %d embeddings from openai, got %d", len(chunk), len(res.Data))
		}

		ordered := make([][]float32, len(chunk))
		for _, d := range res.Data {
			if d.Index >= 0 && d.Index < len(chunk) {
				ordered[d.Index] = NormalizeAndTruncateMRL(d.Embedding, c.embeddingDimensions)
			}
		}
		allEmbeddings = append(allEmbeddings, ordered...)
	}

	return allEmbeddings, nil
}

func (c *OpenAIClient) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	openAIMessages := make([]map[string]string, len(messages))
	for i, m := range messages {
		openAIMessages[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}

	payload := map[string]interface{}{
		"model":    c.summaryModel,
		"messages": openAIMessages,
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

