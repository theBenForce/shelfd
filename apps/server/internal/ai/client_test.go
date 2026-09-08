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
	if len(embedding) != 4 || embedding[0] != 0.1 {
		t.Errorf("unexpected embedding: %v", embedding)
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
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req["model"] != "text-embedding-3-small" {
				t.Errorf("expected model text-embedding-3-small, got %v", req["model"])
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"embedding": []float32{0.5, 0.6, 0.7, 0.8},
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
	if len(embedding) != 4 || embedding[0] != 0.5 {
		t.Errorf("unexpected embedding: %v", embedding)
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
