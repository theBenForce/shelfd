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

// NewOllamaEmbedder creates an Ollama client instance configured for embeddings.
func NewOllamaEmbedder(cfg *config.ModelEndpointConfig) *OllamaClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	model := cfg.Model
	if model == "" {
		model = "nomic-embed-text"
	}
	dims := cfg.Dimensions
	if dims <= 0 {
		dims = 256
	}
	return &OllamaClient{
		baseURL:             baseURL,
		embeddingModel:      model,
		embeddingDimensions: dims,
		httpClient:          &http.Client{Timeout: defaultTimeout},
	}
}

// NewOllamaChatClient creates an Ollama client instance configured for chat and summarization.
func NewOllamaChatClient(cfg *config.ModelEndpointConfig) *OllamaClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	model := cfg.Model
	if model == "" {
		model = "llama3.2:3b"
	}
	return &OllamaClient{
		baseURL:      baseURL,
		summaryModel: model,
		httpClient:   &http.Client{Timeout: defaultTimeout},
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
	resp, err := c.ChatWithTools(ctx, messages, nil)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (c *OllamaClient) ChatWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition) (*ChatResponse, error) {
	var ollamaMessages []map[string]any
	for _, m := range messages {
		msg := map[string]any{
			"role":    m.Role,
			"content": m.Content,
		}
		if len(m.ToolCalls) > 0 {
			var tcList []map[string]any
			for _, tc := range m.ToolCalls {
				var argsObj any
				if err := json.Unmarshal([]byte(tc.Arguments), &argsObj); err != nil {
					argsObj = map[string]any{}
				}
				tcList = append(tcList, map[string]any{
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": argsObj,
					},
				})
			}
			msg["tool_calls"] = tcList
		}
		ollamaMessages = append(ollamaMessages, msg)
	}

	payload := map[string]any{
		"model":    c.summaryModel,
		"messages": ollamaMessages,
		"stream":   false,
	}

	if len(tools) > 0 {
		var ollamaTools []map[string]any
		for _, t := range tools {
			ollamaTools = append(ollamaTools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				},
			})
		}
		payload["tools"] = ollamaTools
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling ollama chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending ollama chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama chat returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments any    `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decoding ollama chat response: %w", err)
	}

	chatResp := &ChatResponse{
		Content: strings.TrimSpace(res.Message.Content),
	}
	for i, tc := range res.Message.ToolCalls {
		argsStr := ""
		switch a := tc.Function.Arguments.(type) {
		case string:
			argsStr = a
		default:
			if b, err := json.Marshal(a); err == nil {
				argsStr = string(b)
			}
		}
		chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCall{
			ID:        fmt.Sprintf("call_%d", i+1),
			Name:      tc.Function.Name,
			Arguments: argsStr,
		})
	}

	return chatResp, nil
}

