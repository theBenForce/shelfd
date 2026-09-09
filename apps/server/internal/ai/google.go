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
	type textPart struct {
		Text string `json:"text"`
	}
	type contentItem struct {
		Role  string     `json:"role"`
		Parts []textPart `json:"parts"`
	}
	type sysInstruction struct {
		Parts []textPart `json:"parts"`
	}
	type generatePayload struct {
		SystemInstruction *sysInstruction `json:"systemInstruction,omitempty"`
		Contents          []contentItem   `json:"contents"`
	}

	var sysParts []textPart
	var contents []contentItem

	for _, m := range messages {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "system":
			sysParts = append(sysParts, textPart{Text: m.Content})
		case "assistant":
			contents = append(contents, contentItem{
				Role:  "model",
				Parts: []textPart{{Text: m.Content}},
			})
		default:
			contents = append(contents, contentItem{
				Role:  "user",
				Parts: []textPart{{Text: m.Content}},
			})
		}
	}

	payload := generatePayload{Contents: contents}
	if len(sysParts) > 0 {
		payload.SystemInstruction = &sysInstruction{Parts: sysParts}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling google generateContent request: %w", err)
	}

	modelPath := "models/" + c.chatModel
	reqURL := fmt.Sprintf("%s/v1beta/%s:generateContent", c.baseURL, modelPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("creating google generateContent request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("x-goog-api-key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending google generateContent request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("google generateContent returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("decoding google generateContent response: %w", err)
	}

	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no candidates returned by google generateContent")
	}

	var sb strings.Builder
	for _, p := range res.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}

	return strings.TrimSpace(sb.String()), nil
}
