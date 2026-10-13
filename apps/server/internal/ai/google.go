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

// GoogleClient communicates with Google Gemini REST endpoints.
type GoogleClient struct {
	baseURL             string
	apiKey              string
	chatModel           string
	embeddingModel      string
	embeddingDimensions int
	httpClient          *http.Client
}

// NewGoogleClient creates a Google Gemini client instance.
func NewGoogleClient(cfg *config.AIConfig) *GoogleClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	chatModel := cfg.ChatModel
	if chatModel == "" {
		chatModel = cfg.SummaryModel
	}
	if chatModel == "" {
		chatModel = "gemini-1.5-flash"
	}
	chatModel = strings.TrimPrefix(chatModel, "models/")

	embeddingModel := cfg.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "text-embedding-004"
	}
	embeddingModel = strings.TrimPrefix(embeddingModel, "models/")

	embeddingDimensions := cfg.EmbeddingDimensions
	if embeddingDimensions <= 0 {
		embeddingDimensions = 256
	}

	return &GoogleClient{
		baseURL:             baseURL,
		apiKey:              cfg.APIKey,
		chatModel:           chatModel,
		embeddingModel:      embeddingModel,
		embeddingDimensions: embeddingDimensions,
		httpClient:          &http.Client{Timeout: defaultTimeout},
	}
}

// NewGoogleEmbedder creates a Google Gemini client instance configured for embeddings.
func NewGoogleEmbedder(cfg *config.ModelEndpointConfig) *GoogleClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	model := cfg.Model
	if model == "" {
		model = "text-embedding-004"
	}
	model = strings.TrimPrefix(model, "models/")
	dims := cfg.Dimensions
	if dims <= 0 {
		dims = 256
	}
	return &GoogleClient{
		baseURL:             baseURL,
		apiKey:              cfg.APIKey,
		embeddingModel:      model,
		embeddingDimensions: dims,
		httpClient:          &http.Client{Timeout: defaultTimeout},
	}
}

// NewGoogleChatClient creates a Google Gemini client instance configured for chat and summarization.
func NewGoogleChatClient(cfg *config.ModelEndpointConfig) *GoogleClient {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	model := cfg.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}
	model = strings.TrimPrefix(model, "models/")
	return &GoogleClient{
		baseURL:    baseURL,
		apiKey:     cfg.APIKey,
		chatModel:  model,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

func (c *GoogleClient) SummarizeChapter(ctx context.Context, title, content string) (string, error) {
	prompt := FormatSummarizePrompt(title, content)
	messages := []ChatMessage{
		{Role: "system", Content: SystemSummarizePrompt},
		{Role: "user", Content: prompt},
	}
	return c.Chat(ctx, messages)
}

func (c *GoogleClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	results, err := c.GenerateBatchEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no embedding returned by google")
	}
	return results[0], nil
}

func (c *GoogleClient) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	chunks := ChunkTexts(texts, 100)
	var allEmbeddings [][]float32

	modelPath := "models/" + c.embeddingModel
	reqURL := fmt.Sprintf("%s/v1beta/%s:batchEmbedContents", c.baseURL, modelPath)

	for _, chunk := range chunks {
		type textPart struct {
			Text string `json:"text"`
		}
		type contentObj struct {
			Parts []textPart `json:"parts"`
		}
		type embedReq struct {
			Model                string     `json:"model"`
			Content              contentObj `json:"content"`
			OutputDimensionality *int       `json:"outputDimensionality,omitempty"`
		}
		type batchPayload struct {
			Requests []embedReq `json:"requests"`
		}

		reqs := make([]embedReq, len(chunk))
		dim := c.embeddingDimensions
		for i, t := range chunk {
			reqs[i] = embedReq{
				Model: modelPath,
				Content: contentObj{
					Parts: []textPart{{Text: t}},
				},
				OutputDimensionality: &dim,
			}
		}

		bodyBytes, err := json.Marshal(batchPayload{Requests: reqs})
		if err != nil {
			return nil, fmt.Errorf("marshaling google batch embeddings request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("creating google batch embeddings request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("x-goog-api-key", c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("sending google batch embeddings request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("google batch embeddings returned status %d: %s", resp.StatusCode, string(respBody))
		}

		var res struct {
			Embeddings []struct {
				Values []float32 `json:"values"`
			} `json:"embeddings"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, fmt.Errorf("decoding google batch embeddings response: %w", err)
		}

		if len(res.Embeddings) != len(chunk) {
			return nil, fmt.Errorf("expected %d embeddings from google, got %d", len(chunk), len(res.Embeddings))
		}

		for _, item := range res.Embeddings {
			normalized := NormalizeAndTruncateMRL(item.Values, c.embeddingDimensions)
			allEmbeddings = append(allEmbeddings, normalized)
		}
	}

	return allEmbeddings, nil
}

func (c *GoogleClient) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	resp, err := c.ChatWithTools(ctx, messages, nil)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (c *GoogleClient) ChatWithTools(ctx context.Context, messages []ChatMessage, tools []ToolDefinition) (*ChatResponse, error) {
	type partItem struct {
		Text             string `json:"text,omitempty"`
		FunctionCall     *struct {
			Name string         `json:"name"`
			Args map[string]any `json:"args"`
		} `json:"functionCall,omitempty"`
		FunctionResponse *struct {
			Name     string         `json:"name"`
			Response map[string]any `json:"response"`
		} `json:"functionResponse,omitempty"`
	}

	type contentItem struct {
		Role  string     `json:"role"`
		Parts []partItem `json:"parts"`
	}

	type sysInstruction struct {
		Parts []partItem `json:"parts"`
	}

	type functionDecl struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters,omitempty"`
	}

	type toolItem struct {
		FunctionDeclarations []functionDecl `json:"functionDeclarations"`
	}

	type generatePayload struct {
		SystemInstruction *sysInstruction `json:"systemInstruction,omitempty"`
		Contents          []contentItem   `json:"contents"`
		Tools             []toolItem      `json:"tools,omitempty"`
	}

	var sysParts []partItem
	var contents []contentItem

	for _, m := range messages {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "system":
			sysParts = append(sysParts, partItem{Text: m.Content})
		case "assistant":
			var parts []partItem
			if m.Content != "" {
				parts = append(parts, partItem{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var args map[string]any
				if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
					args = make(map[string]any)
				}
				parts = append(parts, partItem{
					FunctionCall: &struct {
						Name string         `json:"name"`
						Args map[string]any `json:"args"`
					}{
						Name: tc.Name,
						Args: args,
					},
				})
			}
			contents = append(contents, contentItem{
				Role:  "model",
				Parts: parts,
			})
		case "tool":
			contents = append(contents, contentItem{
				Role: "function",
				Parts: []partItem{
					{
						FunctionResponse: &struct {
							Name     string         `json:"name"`
							Response map[string]any `json:"response"`
						}{
							Name:     m.ToolCallID,
							Response: map[string]any{"output": m.Content},
						},
					},
				},
			})
		default:
			contents = append(contents, contentItem{
				Role:  "user",
				Parts: []partItem{{Text: m.Content}},
			})
		}
	}

	payload := generatePayload{Contents: contents}
	if len(sysParts) > 0 {
		payload.SystemInstruction = &sysInstruction{Parts: sysParts}
	}

	if len(tools) > 0 {
		var decls []functionDecl
		for _, t := range tools {
			decls = append(decls, functionDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			})
		}
		payload.Tools = []toolItem{{FunctionDeclarations: decls}}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling google generateContent request: %w", err)
	}

	modelPath := "models/" + c.chatModel
	reqURL := fmt.Sprintf("%s/v1beta/%s:generateContent", c.baseURL, modelPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating google generateContent request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("x-goog-api-key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending google generateContent request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google generateContent returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string         `json:"name"`
						Args map[string]any `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decoding google generateContent response: %w", err)
	}

	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no candidates returned by google generateContent")
	}

	chatResp := &ChatResponse{}
	var textBuilder strings.Builder
	for i, p := range res.Candidates[0].Content.Parts {
		if p.FunctionCall != nil {
			argsBytes, _ := json.Marshal(p.FunctionCall.Args)
			chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCall{
				ID:        fmt.Sprintf("call_%s_%d", p.FunctionCall.Name, i+1),
				Name:      p.FunctionCall.Name,
				Arguments: string(argsBytes),
			})
		}
		if p.Text != "" {
			textBuilder.WriteString(p.Text)
		}
	}
	chatResp.Content = strings.TrimSpace(textBuilder.String())

	return chatResp, nil
}
