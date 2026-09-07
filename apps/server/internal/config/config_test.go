package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shelfd/shelfd/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Database.Type != "sqlite" {
		t.Errorf("expected sqlite, got %s", cfg.Database.Type)
	}
	if cfg.Database.SQLite.Path != "/data/sqlite.db" {
		t.Errorf("expected /data/sqlite.db, got %s", cfg.Database.SQLite.Path)
	}
	if cfg.Storage.LibraryDir != "/library" {
		t.Errorf("expected /library, got %s", cfg.Storage.LibraryDir)
	}
	if cfg.AI.EmbeddingDimensions != 1536 {
		t.Errorf("expected 1536, got %d", cfg.AI.EmbeddingDimensions)
	}
}

func TestParseYAMLWithEnv(t *testing.T) {
	os.Setenv("TEST_SERVER_PORT", "9090")
	os.Setenv("TEST_JWT_SECRET", "super-secret-token")
	defer os.Unsetenv("TEST_SERVER_PORT")
	defer os.Unsetenv("TEST_JWT_SECRET")

	yamlContent := `
server:
  host: "127.0.0.1"
  port: ${TEST_SERVER_PORT:-8080}
  jwt_secret: "${TEST_JWT_SECRET}"

database:
  type: "sqlite"
  sqlite:
    path: "${TEST_DB_PATH:-/custom/path/sqlite.db}"

storage:
  library_dir: "/mnt/books"
  data_dir: "/var/shelfd"

ai:
  provider: "openai"
  base_url: "https://api.openai.com/v1"
  api_key: "sk-mock-key"
  embedding_dimensions: 1536

mcp:
  enabled: true
  path: "/mcp"
`

	cfg, err := config.Parse([]byte(yamlContent))
	if err != nil {
		t.Fatalf("unexpected error parsing yaml: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090 from env, got %d", cfg.Server.Port)
	}
	if cfg.Server.JWTSecret != "super-secret-token" {
		t.Errorf("expected secret from env, got %s", cfg.Server.JWTSecret)
	}
	if cfg.Database.SQLite.Path != "/custom/path/sqlite.db" {
		t.Errorf("expected fallback path, got %s", cfg.Database.SQLite.Path)
	}
	if cfg.Storage.LibraryDir != "/mnt/books" {
		t.Errorf("expected /mnt/books, got %s", cfg.Storage.LibraryDir)
	}
	if cfg.AI.Provider != "openai" {
		t.Errorf("expected openai, got %s", cfg.AI.Provider)
	}
}

func TestPostgresConfig(t *testing.T) {
	yamlContent := `
database:
  type: "postgres"
  postgres:
    dsn: "postgres://user:pass@localhost:5432/shelfd"
`
	cfg, err := config.Parse([]byte(yamlContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database.Type != "postgres" || cfg.Database.Postgres.DSN == "" {
		t.Errorf("expected valid postgres configuration, got %v", cfg.Database)
	}
}

func TestLoadFromFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	content := `
server:
  port: 8081
database:
  type: "sqlite"
  sqlite:
    path: ":memory:"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Server.Port != 8081 {
		t.Errorf("expected 8081, got %d", cfg.Server.Port)
	}
	if cfg.Database.SQLite.Path != ":memory:" {
		t.Errorf("expected :memory:, got %s", cfg.Database.SQLite.Path)
	}
	if cfg.Storage.LibraryDir != "/library" {
		t.Errorf("expected /library, got %s", cfg.Storage.LibraryDir)
	}

	// Test non-existent file
	if _, err := config.Load(filepath.Join(tempDir, "non_existent.yaml")); err == nil {
		t.Error("expected error loading non-existent file")
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "invalid port zero",
			yaml:    `server: { port: 0 }`,
			wantErr: "server.port must be between 1 and 65535",
		},
		{
			name:    "invalid port too high",
			yaml:    `server: { port: 70000 }`,
			wantErr: "server.port must be between 1 and 65535",
		},
		{
			name:    "invalid database type",
			yaml:    `database: { type: "mysql" }`,
			wantErr: "database.type must be 'sqlite' or 'postgres'",
		},
		{
			name:    "missing sqlite path",
			yaml:    `database: { type: "sqlite", sqlite: { path: "" } }`,
			wantErr: "database.sqlite.path is required",
		},
		{
			name:    "missing postgres dsn",
			yaml:    `database: { type: "postgres", postgres: { dsn: "" } }`,
			wantErr: "database.postgres.dsn is required",
		},
		{
			name:    "empty library dir",
			yaml:    `storage: { library_dir: "   " }`,
			wantErr: "storage.library_dir must not be empty",
		},
		{
			name:    "empty data dir",
			yaml:    `storage: { data_dir: "   " }`,
			wantErr: "storage.data_dir must not be empty",
		},
		{
			name:    "invalid ai provider",
			yaml:    `ai: { provider: "invalid-provider" }`,
			wantErr: "ai.provider must be 'ollama' or 'openai'",
		},
		{
			name:    "ssrf metadata ip blocked",
			yaml:    `ai: { base_url: "http://169.254.169.254/latest/meta-data" }`,
			wantErr: "prohibited cloud metadata address",
		},
		{
			name:    "invalid yaml syntax",
			yaml:    `server: [invalid yaml`,
			wantErr: "unmarshaling config yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.Parse([]byte(tt.yaml))
			if err == nil {
				t.Fatalf("expected error containing '%s', got nil", tt.wantErr)
			}
			if !stringsContains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing '%s', got '%s'", tt.wantErr, err.Error())
			}
		})
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || filepath.Base(s) != "" && len(substr) > 0 && contains(s, substr))
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
