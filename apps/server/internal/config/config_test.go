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
	if cfg.AI.EmbeddingDimensions != 256 {
		t.Errorf("expected 256, got %d", cfg.AI.EmbeddingDimensions)
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

func TestApplyEnvOverrides(t *testing.T) {
	cfg := config.DefaultConfig()

	os.Setenv("SHELFD_SERVER_HOST", "127.0.0.1")
	os.Setenv("SHELFD_SERVER_PORT", "9999")
	os.Setenv("SHELFD_JWT_SECRET", "custom-jwt-secret")
	os.Setenv("SHELFD_DATABASE_TYPE", "sqlite")
	os.Setenv("SHELFD_SQLITE_PATH", "/custom/data/shelfd.db")
	os.Setenv("SHELFD_STORAGE_LIBRARY_DIR", "/mnt/ebooks")
	os.Setenv("SHELFD_STORAGE_DATA_DIR", "/mnt/shelfd_data")
	os.Setenv("SHELFD_AI_PROVIDER", "openai")
	os.Setenv("SHELFD_AI_BASE_URL", "https://api.openai.com/v1")
	os.Setenv("SHELFD_AI_API_KEY", "sk-test-token")
	os.Setenv("SHELFD_AI_EMBEDDING_MODEL", "text-embedding-3-small")
	os.Setenv("SHELFD_AI_EMBEDDING_DIMENSIONS", "1536")
	os.Setenv("SHELFD_AI_SUMMARY_MODEL", "gpt-4o-mini")
	os.Setenv("SHELFD_MCP_ENABLED", "false")
	os.Setenv("SHELFD_MCP_PATH", "/custom_mcp")

	defer func() {
		os.Unsetenv("SHELFD_SERVER_HOST")
		os.Unsetenv("SHELFD_SERVER_PORT")
		os.Unsetenv("SHELFD_JWT_SECRET")
		os.Unsetenv("SHELFD_DATABASE_TYPE")
		os.Unsetenv("SHELFD_SQLITE_PATH")
		os.Unsetenv("SHELFD_STORAGE_LIBRARY_DIR")
		os.Unsetenv("SHELFD_STORAGE_DATA_DIR")
		os.Unsetenv("SHELFD_AI_PROVIDER")
		os.Unsetenv("SHELFD_AI_BASE_URL")
		os.Unsetenv("SHELFD_AI_API_KEY")
		os.Unsetenv("SHELFD_AI_EMBEDDING_MODEL")
		os.Unsetenv("SHELFD_AI_EMBEDDING_DIMENSIONS")
		os.Unsetenv("SHELFD_AI_SUMMARY_MODEL")
		os.Unsetenv("SHELFD_MCP_ENABLED")
		os.Unsetenv("SHELFD_MCP_PATH")
	}()

	config.ApplyEnvOverrides(cfg)

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Server.Port)
	}
	if cfg.Server.JWTSecret != "custom-jwt-secret" {
		t.Errorf("expected custom-jwt-secret, got %s", cfg.Server.JWTSecret)
	}
	if cfg.Database.SQLite.Path != "/custom/data/shelfd.db" {
		t.Errorf("expected /custom/data/shelfd.db, got %s", cfg.Database.SQLite.Path)
	}
	if cfg.Storage.LibraryDir != "/mnt/ebooks" {
		t.Errorf("expected /mnt/ebooks, got %s", cfg.Storage.LibraryDir)
	}
	if cfg.Storage.DataDir != "/mnt/shelfd_data" {
		t.Errorf("expected /mnt/shelfd_data, got %s", cfg.Storage.DataDir)
	}
	if cfg.AI.Provider != "openai" {
		t.Errorf("expected openai, got %s", cfg.AI.Provider)
	}
	if cfg.AI.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("expected https://api.openai.com/v1, got %s", cfg.AI.BaseURL)
	}
	if cfg.AI.APIKey != "sk-test-token" {
		t.Errorf("expected sk-test-token, got %s", cfg.AI.APIKey)
	}
	if cfg.AI.EmbeddingModel != "text-embedding-3-small" {
		t.Errorf("expected text-embedding-3-small, got %s", cfg.AI.EmbeddingModel)
	}
	if cfg.AI.EmbeddingDimensions != 1536 {
		t.Errorf("expected 1536, got %d", cfg.AI.EmbeddingDimensions)
	}
	if cfg.AI.SummaryModel != "gpt-4o-mini" {
		t.Errorf("expected gpt-4o-mini, got %s", cfg.AI.SummaryModel)
	}
	if cfg.MCP.Enabled != false {
		t.Errorf("expected MCP disabled, got %v", cfg.MCP.Enabled)
	}
	if cfg.MCP.Path != "/custom_mcp" {
		t.Errorf("expected /custom_mcp, got %s", cfg.MCP.Path)
	}
}

func TestResolveConfigPath(t *testing.T) {
	// 1. Explicit path parameter takes highest precedence
	res := config.ResolveConfigPath("/explicit/path/config.yaml")
	if res != "/explicit/path/config.yaml" {
		t.Errorf("expected explicit path, got %s", res)
	}

	// 2. SHELFD_CONFIG environment variable takes second precedence
	os.Setenv("SHELFD_CONFIG", "/env/config.yaml")
	defer os.Unsetenv("SHELFD_CONFIG")
	res = config.ResolveConfigPath("")
	if res != "/env/config.yaml" {
		t.Errorf("expected env path, got %s", res)
	}
	os.Unsetenv("SHELFD_CONFIG")

	// 3. Candidate search when none exists returns empty string
	res = config.ResolveConfigPath("")
	// Since standard candidate files (like /config/config.yaml or /data/config.yaml) don't exist in dev env,
	// either empty string or existing config.yaml is returned.
	if res != "" && res != "config.yaml" {
		t.Errorf("unexpected resolved path %s", res)
	}
}

func TestConfigExampleYAMLValid(t *testing.T) {
	examplePath := filepath.Join("..", "..", "..", "..", "config.example.yaml")
	cfg, err := config.Load(examplePath)
	if err != nil {
		t.Fatalf("failed to load and validate config.example.yaml: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Storage.LibraryDir != "/library" {
		t.Errorf("expected library dir /library, got %s", cfg.Storage.LibraryDir)
	}
}


