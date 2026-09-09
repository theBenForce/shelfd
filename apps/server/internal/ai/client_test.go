package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/config"
)

func TestOllamaClient(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate":
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req["model"] != "llama3.2:3b" {
				t.Errorf("expected model llama3.2:3b, got %v", req["model"])
			}
			prompt := req["prompt"].(string)
			if !strings.Contains(prompt, "Paul Atreides") {
				t.Errorf("expected prompt to contain chapter content, got %s", prompt)
			}
			json.NewEncoder(w).Encode(map[string]string{
				"response": "Paul Atreides prepares to depart Caladan for Arrakis. The family faces grave peril.",
			})

		case "/api/embeddings":
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req["model"] != "nomic-embed-text" {
				t.Errorf("expected model nomic-embed-text, got %v", req["model"])
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"embedding": []float32{0.1, 0.2, 0.3, 0.4},
			})

		case "/api/chat":
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req["model"] != "llama3.2:3b" {
				t.Errorf("expected model llama3.2:3b, got %v", req["model"])
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": map[string]string{
					"role":    "assistant",
					"content": "Paul was tested with the Gom Jabbar.",
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.AIConfig{
		Provider:            "ollama",
		BaseURL:             server.URL,
		SummaryModel:        "llama3.2:3b",
		EmbeddingModel:      "nomic-embed-text",
		EmbeddingDimensions: 4,
	}

	client, err := ai.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create ollama client: %v", err)
	}

	summary, err := client.SummarizeChapter(ctx, "Chapter 1", "Paul Atreides sat in his room...")
	if err != nil {
		t.Fatalf("SummarizeChapter error: %v", err)
	}
	if !strings.Contains(summary, "Paul Atreides") {
		t.Errorf("unexpected summary: %s", summary)
	}

	embedding, err := client.GenerateEmbedding(ctx, summary)
	if err != nil {
		t.Fatalf("GenerateEmbedding error: %v", err)
	}
	if len(embedding) != 4 {
		t.Errorf("expected 4 elements, got %d: %v", len(embedding), embedding)
	}
	var normSq float32
	for _, v := range embedding {
		normSq += v * v
	}
	if normSq < 0.99 || normSq > 1.01 {
		t.Errorf("expected unit L2 norm ~1.0, got %f", normSq)
	}

	reply, err := client.Chat(ctx, []ai.ChatMessage{{Role: "user", Content: "What was Paul's test?"}})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if !strings.Contains(reply, "Gom Jabbar") {
		t.Errorf("unexpected chat reply: %s", reply)
	}
}

func TestOpenAIClient(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-test-key" {
			t.Errorf("expected Authorization: Bearer sk-test-key, got %s", auth)
		}

		switch r.URL.Path {
		case "/v1/chat/completions":
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req["model"] != "gpt-4o-mini" {
				t.Errorf("expected model gpt-4o-mini, got %v", req["model"])
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]string{
							"content": "Case is hired by Armitage in Chiba City for a high-stakes cyberspace heist.",
						},
					},
				},
			})

		case "/v1/embeddings":
			var req struct {
				Model string      `json:"model"`
				Input interface{} `json:"input"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Model != "text-embedding-3-small" {
				t.Errorf("expected model text-embedding-3-small, got %v", req.Model)
			}
			if list, ok := req.Input.([]interface{}); ok {
				data := make([]map[string]interface{}, len(list))
				for i := range list {
					data[i] = map[string]interface{}{
						"embedding": []float32{0.5, 0.6, 0.7, 0.8},
						"index":     i,
					}
				}
				json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"embedding": []float32{0.5, 0.6, 0.7, 0.8},
						"index":     0,
					},
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.AIConfig{
		Provider:            "openai",
		BaseURL:             server.URL,
		APIKey:              "sk-test-key",
		SummaryModel:        "gpt-4o-mini",
		EmbeddingModel:      "text-embedding-3-small",
		EmbeddingDimensions: 4,
	}

	client, err := ai.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create openai client: %v", err)
	}

	summary, err := client.SummarizeChapter(ctx, "Chapter 1", "The sky above the port...")
	if err != nil {
		t.Fatalf("SummarizeChapter error: %v", err)
	}
	if !strings.Contains(summary, "Case is hired") {
		t.Errorf("unexpected summary: %s", summary)
	}

	embedding, err := client.GenerateEmbedding(ctx, summary)
	if err != nil {
		t.Fatalf("GenerateEmbedding error: %v", err)
	}
	if len(embedding) != 4 {
		t.Errorf("expected 4 elements, got %d: %v", len(embedding), embedding)
	}
	var normSq float32
	for _, v := range embedding {
		normSq += v * v
	}
	if normSq < 0.99 || normSq > 1.01 {
		t.Errorf("expected unit L2 norm ~1.0, got %f", normSq)
	}

	batchEmbs, err := client.GenerateBatchEmbeddings(ctx, []string{"Passage A", "Passage B"})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings error: %v", err)
	}
	if len(batchEmbs) != 2 {
		t.Errorf("expected 2 batch embeddings, got %d", len(batchEmbs))
	}

	reply, err := client.Chat(ctx, []ai.ChatMessage{{Role: "user", Content: "Who hired Case?"}})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if !strings.Contains(reply, "Case is hired") {
		t.Errorf("unexpected chat reply: %s", reply)
	}
}

func TestSSRFBlocking(t *testing.T) {
	cfg := &config.AIConfig{
		Provider: "ollama",
		BaseURL:  "http://169.254.169.254/latest/meta-data",
	}
	_, err := ai.NewClient(cfg)
	if err == nil {
		t.Fatal("expected error creating client with metadata IP, got nil")
	}
	if !strings.Contains(err.Error(), "prohibited cloud metadata address") {
		t.Errorf("expected SSRF error message, got: %v", err)
	}
}

func TestNormalizeAndTruncateMRL(t *testing.T) {
	// Truncate from 6 to 3 and normalize
	input := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0}
	res := ai.NormalizeAndTruncateMRL(input, 3)
	if len(res) != 3 {
		t.Fatalf("expected length 3, got %d", len(res))
	}
	var sumSq float32
	for _, v := range res {
		sumSq += v * v
	}
	if sumSq < 0.99 || sumSq > 1.01 {
		t.Errorf("expected unit L2 norm ~1.0, got %f", sumSq)
	}

	// Empty vector
	empty := ai.NormalizeAndTruncateMRL(nil, 4)
	if len(empty) != 0 {
		t.Errorf("expected empty vector, got %v", empty)
	}

	// All zeros
	zeros := []float32{0, 0, 0}
	zeroRes := ai.NormalizeAndTruncateMRL(zeros, 3)
	if len(zeroRes) != 3 {
		t.Errorf("expected length 3, got %d", len(zeroRes))
	}
}

func TestGoogleClient(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("x-goog-api-key")
		if apiKey != "ai-secret-key" {
			t.Errorf("expected x-goog-api-key: ai-secret-key, got %s", apiKey)
		}

		switch {
		case strings.Contains(r.URL.Path, ":batchEmbedContents"):
			var req struct {
				Requests []struct {
					Model                string `json:"model"`
					OutputDimensionality int    `json:"outputDimensionality"`
					Content              struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"requests"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(req.Requests) == 0 {
				t.Errorf("expected requests in batch, got 0")
			}
			if req.Requests[0].OutputDimensionality != 4 {
				t.Errorf("expected outputDimensionality 4, got %d", req.Requests[0].OutputDimensionality)
			}

			embeddings := make([]map[string]interface{}, len(req.Requests))
			for i := range req.Requests {
				embeddings[i] = map[string]interface{}{
					"values": []float32{0.2, 0.4, 0.6, 0.8},
				}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"embeddings": embeddings,
			})

		case strings.Contains(r.URL.Path, ":generateContent"):
			var req struct {
				SystemInstruction struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"systemInstruction"`
				Contents []struct {
					Role  string `json:"role"`
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"contents"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(req.Contents) == 0 {
				t.Errorf("expected contents in generateContent, got 0")
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"candidates": []map[string]interface{}{
					{
						"content": map[string]interface{}{
							"role": "model",
							"parts": []map[string]string{
								{"text": "The Spice Melange extends life and expands consciousness."},
							},
						},
					},
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.AIConfig{
		Provider:            "google",
		BaseURL:             server.URL,
		APIKey:              "ai-secret-key",
		ChatModel:           "gemini-1.5-flash",
		EmbeddingModel:      "text-embedding-004",
		EmbeddingDimensions: 4,
	}

	client, err := ai.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create google client: %v", err)
	}

	// 1. Test single GenerateEmbedding
	emb, err := client.GenerateEmbedding(ctx, "The Spice Melange")
	if err != nil {
		t.Fatalf("GenerateEmbedding failed: %v", err)
	}
	if len(emb) != 4 {
		t.Fatalf("expected 4 dimensions, got %d", len(emb))
	}
	var sumSq float32
	for _, v := range emb {
		sumSq += v * v
	}
	if sumSq < 0.99 || sumSq > 1.01 {
		t.Errorf("expected unit L2 norm ~1.0, got %f", sumSq)
	}

	// 2. Test GenerateBatchEmbeddings
	batchEmbs, err := client.GenerateBatchEmbeddings(ctx, []string{"Passage 1", "Passage 2"})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings failed: %v", err)
	}
	if len(batchEmbs) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(batchEmbs))
	}

	// 3. Test Chat
	reply, err := client.Chat(ctx, []ai.ChatMessage{
		{Role: "system", Content: "You are a literary assistant."},
		{Role: "user", Content: "What is the Spice?"},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if !strings.Contains(reply, "Spice Melange") {
		t.Errorf("unexpected chat reply: %s", reply)
	}

	// 4. Test SummarizeChapter
	summary, err := client.SummarizeChapter(ctx, "Chapter 1", "Arrakis Dune Desert Planet")
	if err != nil {
		t.Fatalf("SummarizeChapter failed: %v", err)
	}
	if !strings.Contains(summary, "Spice Melange") {
		t.Errorf("unexpected summary: %s", summary)
	}
}

