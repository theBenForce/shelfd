package config_test

import (
	"os"
	"path/filepath"
	"strings"
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
			wantErr: "ai.provider must be 'ollama', 'openai', or 'google'",
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

func TestGoogleAIConfig(t *testing.T) {
	yamlContent := `
ai:
  provider: "google"
  api_key: "ai-test-key"
  embedding_model: "text-embedding-004"
  embedding_dimensions: 256
  chat_model: "gemini-1.5-flash"
`
	cfg, err := config.Parse([]byte(yamlContent))
	if err != nil {
		t.Fatalf("unexpected error parsing google ai config: %v", err)
	}
	if cfg.AI.Provider != "google" {
		t.Errorf("expected provider google, got %s", cfg.AI.Provider)
	}
	if cfg.AI.ChatModel != "gemini-1.5-flash" {
		t.Errorf("expected chat_model gemini-1.5-flash, got %s", cfg.AI.ChatModel)
	}
	if cfg.AI.SummaryModel != "gemini-1.5-flash" {
		t.Errorf("expected summary_model alias gemini-1.5-flash, got %s", cfg.AI.SummaryModel)
	}

	// Test gemini alias
	geminiYaml := `
ai:
  provider: "gemini"
  api_key: "ai-test-key"
`
	cfg2, err := config.Parse([]byte(geminiYaml))
	if err != nil {
		t.Fatalf("unexpected error parsing gemini alias config: %v", err)
	}
	if cfg2.AI.Provider != "gemini" {
		t.Errorf("expected provider gemini, got %s", cfg2.AI.Provider)
	}
}

func TestAuthConfig(t *testing.T) {
	// 1. Default config
	cfg := config.DefaultConfig()
	if cfg.Auth.AdminUsername != "admin" {
		t.Errorf("expected default admin_username admin, got %s", cfg.Auth.AdminUsername)
	}

	// 2. YAML parsing
	yamlContent := `
auth:
  admin_username: "custom_admin"
  admin_password: "custom_password"
`
	parsed, err := config.Parse([]byte(yamlContent))
	if err != nil {
		t.Fatalf("unexpected error parsing auth config: %v", err)
	}
	if parsed.Auth.AdminUsername != "custom_admin" {
		t.Errorf("expected admin_username custom_admin, got %s", parsed.Auth.AdminUsername)
	}
	if parsed.Auth.AdminPassword != "custom_password" {
		t.Errorf("expected admin_password custom_password, got %s", parsed.Auth.AdminPassword)
	}

	// 3. Env overrides
	os.Setenv("SHELFD_ADMIN_USERNAME", "env_admin")
	os.Setenv("SHELFD_ADMIN_PASSWORD", "env_password")
	defer os.Unsetenv("SHELFD_ADMIN_USERNAME")
	defer os.Unsetenv("SHELFD_ADMIN_PASSWORD")

	cfg2 := config.DefaultConfig()
	config.ApplyEnvOverrides(cfg2)
	if cfg2.Auth.AdminUsername != "env_admin" {
		t.Errorf("expected env override admin_username env_admin, got %s", cfg2.Auth.AdminUsername)
	}
	if cfg2.Auth.AdminPassword != "env_password" {
		t.Errorf("expected env override admin_password env_password, got %s", cfg2.Auth.AdminPassword)
	}
}

func TestLoggingConfig(t *testing.T) {
	// 1. Defaults
	cfg := config.DefaultConfig()
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level info, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("expected default log format text, got %s", cfg.Logging.Format)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config validation failed: %v", err)
	}

	// 2. YAML parsing
	yamlContent := `
logging:
  level: "debug"
  format: "json"
`
	parsed, err := config.Parse([]byte(yamlContent))
	if err != nil {
		t.Fatalf("unexpected error parsing logging config: %v", err)
	}
	if parsed.Logging.Level != "debug" {
		t.Errorf("expected debug, got %s", parsed.Logging.Level)
	}
	if parsed.Logging.Format != "json" {
		t.Errorf("expected json, got %s", parsed.Logging.Format)
	}

	// 3. Validation of invalid values
	invalidLevelCfg := config.DefaultConfig()
	invalidLevelCfg.Logging.Level = "invalid_level"
	if err := invalidLevelCfg.Validate(); err == nil {
		t.Errorf("expected error for invalid logging.level, got nil")
	}

	invalidFormatCfg := config.DefaultConfig()
	invalidFormatCfg.Logging.Format = "xml"
	if err := invalidFormatCfg.Validate(); err == nil {
		t.Errorf("expected error for invalid logging.format, got nil")
	}

	// 4. Env overrides
	os.Setenv("SHELFD_LOG_LEVEL", "warn")
	os.Setenv("SHELFD_LOG_FORMAT", "json")
	defer os.Unsetenv("SHELFD_LOG_LEVEL")
	defer os.Unsetenv("SHELFD_LOG_FORMAT")

	cfg2 := config.DefaultConfig()
	config.ApplyEnvOverrides(cfg2)
	if cfg2.Logging.Level != "warn" {
		t.Errorf("expected env override warn, got %s", cfg2.Logging.Level)
	}
	if cfg2.Logging.Format != "json" {
		t.Errorf("expected env override json, got %s", cfg2.Logging.Format)
	}
}

func TestSplitModelEndpointsConfig(t *testing.T) {
	yamlContent := `
ai:
  embedder:
    provider: "ollama"
    base_url: "http://localhost:11434"
    api_key: "emb-key"
    model: "nomic-embed-text"
    dimensions: 384
  chat:
    provider: "openai"
    base_url: "https://api.openai.com/v1"
    api_key: "sk-chatkey12345"
    model: "gpt-4o-mini"
`
	parsed, err := config.Parse([]byte(yamlContent))
	if err != nil {
		t.Fatalf("unexpected error parsing split model config: %v", err)
	}

	if parsed.AI.Embedder.Provider != "ollama" {
		t.Errorf("expected embedder provider ollama, got %s", parsed.AI.Embedder.Provider)
	}
	if parsed.AI.Embedder.BaseURL != "http://localhost:11434" {
		t.Errorf("expected embedder base_url http://localhost:11434, got %s", parsed.AI.Embedder.BaseURL)
	}
	if parsed.AI.Embedder.Dimensions != 384 || parsed.AI.EmbeddingDimensions != 384 {
		t.Errorf("expected dimensions 384, got embedder=%d, global=%d", parsed.AI.Embedder.Dimensions, parsed.AI.EmbeddingDimensions)
	}
	if parsed.AI.Chat.Provider != "openai" {
		t.Errorf("expected chat provider openai, got %s", parsed.AI.Chat.Provider)
	}
	if parsed.AI.Chat.Model != "gpt-4o-mini" {
		t.Errorf("expected chat model gpt-4o-mini, got %s", parsed.AI.Chat.Model)
	}

	// Masking test
	masked := parsed.AI.Chat.String()
	if strings.Contains(masked, "sk-chatkey12345") {
		t.Errorf("expected API key to be masked in String(), got %s", masked)
	}
	if !strings.Contains(masked, "****") {
		t.Errorf("expected **** in masked string, got %s", masked)
	}
}

func TestSplitModelEndpointsFallbackInheritance(t *testing.T) {
	// 1. Legacy flat config inheritance
	legacyYAML := `
ai:
  provider: "openai"
  base_url: "https://api.openai.com/v1"
  api_key: "sk-legacy-secret"
  embedding_model: "text-embedding-3-small"
  embedding_dimensions: 512
  chat_model: "gpt-4o"
`
	parsed, err := config.Parse([]byte(legacyYAML))
	if err != nil {
		t.Fatalf("unexpected error parsing legacy config: %v", err)
	}

	if parsed.AI.Embedder.Provider != "openai" {
		t.Errorf("expected embedder provider openai, got %s", parsed.AI.Embedder.Provider)
	}
	if parsed.AI.Embedder.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("expected embedder base_url https://api.openai.com/v1, got %s", parsed.AI.Embedder.BaseURL)
	}
	if parsed.AI.Embedder.APIKey != "sk-legacy-secret" {
		t.Errorf("expected embedder api_key sk-legacy-secret, got %s", parsed.AI.Embedder.APIKey)
	}
	if parsed.AI.Embedder.Model != "text-embedding-3-small" {
		t.Errorf("expected embedder model text-embedding-3-small, got %s", parsed.AI.Embedder.Model)
	}
	if parsed.AI.Embedder.Dimensions != 512 {
		t.Errorf("expected embedder dimensions 512, got %d", parsed.AI.Embedder.Dimensions)
	}
	if parsed.AI.Chat.Provider != "openai" {
		t.Errorf("expected chat provider openai, got %s", parsed.AI.Chat.Provider)
	}
	if parsed.AI.Chat.APIKey != "sk-legacy-secret" {
		t.Errorf("expected chat api_key sk-legacy-secret, got %s", parsed.AI.Chat.APIKey)
	}
	if parsed.AI.Chat.Model != "gpt-4o" {
		t.Errorf("expected chat model gpt-4o, got %s", parsed.AI.Chat.Model)
	}

	// 2. Partial override with provider mismatch prevents key/URL bleed
	partialYAML := `
ai:
  provider: "openai"
  base_url: "https://api.openai.com/v1"
  api_key: "sk-openai-key"
  embedder:
    provider: "ollama"
    model: "nomic-embed-text"
`
	partialParsed, err := config.Parse([]byte(partialYAML))
	if err != nil {
		t.Fatalf("unexpected error parsing partial config: %v", err)
	}
	if partialParsed.AI.Embedder.Provider != "ollama" {
		t.Errorf("expected embedder provider ollama, got %s", partialParsed.AI.Embedder.Provider)
	}
	// Embedder baseURL should be default Ollama, NOT OpenAI
	if partialParsed.AI.Embedder.BaseURL != config.DefaultBaseURLForProvider("ollama") {
		t.Errorf("expected embedder base_url default ollama, got %s", partialParsed.AI.Embedder.BaseURL)
	}
	// Embedder API key must NOT inherit OpenAI key
	if partialParsed.AI.Embedder.APIKey != "" {
		t.Errorf("expected empty embedder api_key for ollama, got %s", partialParsed.AI.Embedder.APIKey)
	}
	// Chat should still inherit OpenAI settings
	if partialParsed.AI.Chat.Provider != "openai" || partialParsed.AI.Chat.APIKey != "sk-openai-key" {
		t.Errorf("expected chat to inherit openai settings, got provider=%s, key=%s", partialParsed.AI.Chat.Provider, partialParsed.AI.Chat.APIKey)
	}
}

func TestSplitModelEndpointsEnvOverrides(t *testing.T) {
	os.Setenv("SHELFD_AI_PROVIDER", "ollama")
	os.Setenv("SHELFD_AI_EMBEDDER_PROVIDER", "google")
	os.Setenv("SHELFD_AI_EMBEDDER_BASE_URL", "https://generativelanguage.googleapis.com")
	os.Setenv("SHELFD_AI_EMBEDDER_API_KEY", "google-api-key")
	os.Setenv("SHELFD_AI_EMBEDDER_MODEL", "text-embedding-004")
	os.Setenv("SHELFD_AI_EMBEDDER_DIMENSIONS", "768")
	os.Setenv("SHELFD_AI_CHAT_PROVIDER", "openai")
	os.Setenv("SHELFD_AI_CHAT_BASE_URL", "https://api.openai.com/v1")
	os.Setenv("SHELFD_AI_CHAT_API_KEY", "sk-env-openai")
	os.Setenv("SHELFD_AI_CHAT_MODEL", "gpt-4o")

	defer func() {
		os.Unsetenv("SHELFD_AI_PROVIDER")
		os.Unsetenv("SHELFD_AI_EMBEDDER_PROVIDER")
		os.Unsetenv("SHELFD_AI_EMBEDDER_BASE_URL")
		os.Unsetenv("SHELFD_AI_EMBEDDER_API_KEY")
		os.Unsetenv("SHELFD_AI_EMBEDDER_MODEL")
		os.Unsetenv("SHELFD_AI_EMBEDDER_DIMENSIONS")
		os.Unsetenv("SHELFD_AI_CHAT_PROVIDER")
		os.Unsetenv("SHELFD_AI_CHAT_BASE_URL")
		os.Unsetenv("SHELFD_AI_CHAT_API_KEY")
		os.Unsetenv("SHELFD_AI_CHAT_MODEL")
	}()

	cfg := config.DefaultConfig()
	config.ApplyEnvOverrides(cfg)

	if cfg.AI.Embedder.Provider != "google" {
		t.Errorf("expected embedder provider google, got %s", cfg.AI.Embedder.Provider)
	}
	if cfg.AI.Embedder.Dimensions != 768 || cfg.AI.EmbeddingDimensions != 768 {
		t.Errorf("expected dimensions 768, got %d and %d", cfg.AI.Embedder.Dimensions, cfg.AI.EmbeddingDimensions)
	}
	if cfg.AI.Chat.Provider != "openai" {
		t.Errorf("expected chat provider openai, got %s", cfg.AI.Chat.Provider)
	}
	if cfg.AI.Chat.APIKey != "sk-env-openai" {
		t.Errorf("expected chat api key sk-env-openai, got %s", cfg.AI.Chat.APIKey)
	}
}

func TestSplitModelEndpointsSSRFValidation(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		blocked bool
	}{
		{"ipv4_metadata_exact", "http://169.254.169.254/latest/meta-data", true},
		{"ipv4_metadata_prefix", "http://169.254.10.20:8080", true},
		{"ipv6_metadata", "http://[fd00:ec2::254]/v1", true},
		{"ipv6_link_local", "http://[fe80::1]/v1", true},
		{"google_metadata_host", "http://metadata.google.internal/computeMetadata/v1", true},
		{"valid_localhost", "http://localhost:11434", false},
		{"valid_docker_host", "http://host.docker.internal:11434", false},
		{"valid_private_network", "http://192.168.1.100:11434", false},
		{"valid_openai", "https://api.openai.com/v1", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := config.ValidateSSRF(tc.url)
			if tc.blocked && err == nil {
				t.Errorf("expected SSRF error for %s, got nil", tc.url)
			}
			if !tc.blocked && err != nil {
				t.Errorf("expected valid URL for %s, got error: %v", tc.url, err)
			}
		})
	}
}



