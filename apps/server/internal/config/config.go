package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the Shelfd server configuration matching TECHNICAL_DESIGN_DOCUMENT.md.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	AI       AIConfig       `yaml:"ai"`
	MCP      MCPConfig      `yaml:"mcp"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type ServerConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	JWTSecret string `yaml:"jwt_secret"`
	WebDir    string `yaml:"web_dir"`
}

type DatabaseConfig struct {
	Type     string         `yaml:"type"` // "sqlite" or "postgres"
	SQLite   SQLiteConfig   `yaml:"sqlite"`
	Postgres PostgresConfig `yaml:"postgres"`
}

type SQLiteConfig struct {
	Path string `yaml:"path"`
}

type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

type StorageConfig struct {
	LibraryDir string `yaml:"library_dir"`
	DataDir    string `yaml:"data_dir"`
}

// ModelEndpointConfig defines the endpoint, provider, authentication, and model settings for an AI capability.
type ModelEndpointConfig struct {
	Provider   string `yaml:"provider"`
	BaseURL    string `yaml:"base_url"`
	APIKey     string `yaml:"api_key"`
	Model      string `yaml:"model"`
	Dimensions int    `yaml:"dimensions,omitempty"`
}

func (m ModelEndpointConfig) String() string {
	maskedKey := ""
	if m.APIKey != "" {
		if len(m.APIKey) > 7 {
			maskedKey = m.APIKey[:3] + "****" + m.APIKey[len(m.APIKey)-4:]
		} else {
			maskedKey = "****"
		}
	}
	return fmt.Sprintf("ModelEndpointConfig{Provider:%s, BaseURL:%s, APIKey:%s, Model:%s, Dimensions:%d}",
		m.Provider, m.BaseURL, maskedKey, m.Model, m.Dimensions)
}

// DefaultBaseURLForProvider returns the canonical default URL for known providers.
func DefaultBaseURLForProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "ollama":
		return "http://host.docker.internal:11434"
	case "openai":
		return "https://api.openai.com"
	case "google", "gemini":
		return "https://generativelanguage.googleapis.com"
	default:
		return ""
	}
}

type AIConfig struct {
	// Legacy top-level settings (maintained for backward compatibility)
	Provider            string `yaml:"provider"` // "ollama", "openai", or "google" ("gemini")
	BaseURL             string `yaml:"base_url"`
	APIKey              string `yaml:"api_key"`
	EmbeddingModel      string `yaml:"embedding_model"`
	EmbeddingDimensions int    `yaml:"embedding_dimensions"`
	ChatModel           string `yaml:"chat_model"`
	SummaryModel        string `yaml:"summary_model"` // Deprecated alias for ChatModel

	// Decoupled endpoint definitions
	Embedder ModelEndpointConfig `yaml:"embedder"`
	Chat     ModelEndpointConfig `yaml:"chat"`
}

type MCPConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type AuthConfig struct {
	AdminUsername string `yaml:"admin_username"`
	AdminPassword string `yaml:"admin_password"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`  // "debug", "info", "warn", "error" (default "info")
	Format string `yaml:"format"` // "text", "json" (default "text")
}

// DefaultConfig returns the production/homelab default configuration.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:      "0.0.0.0",
			Port:      8080,
			JWTSecret: "",
			WebDir:    "/usr/share/shelfd/web",
		},
		Database: DatabaseConfig{
			Type: "sqlite",
			SQLite: SQLiteConfig{
				Path: "/data/sqlite.db",
			},
		},
		Storage: StorageConfig{
			LibraryDir: "/library",
			DataDir:    "/data",
		},
		AI: AIConfig{
			Provider:            "ollama",
			BaseURL:             "http://host.docker.internal:11434",
			APIKey:              "",
			EmbeddingModel:      "nomic-embed-text",
			EmbeddingDimensions: 256,
			ChatModel:           "llama3.2:3b",
			SummaryModel:        "llama3.2:3b",
			Embedder: ModelEndpointConfig{
				Provider:   "ollama",
				BaseURL:    "http://host.docker.internal:11434",
				APIKey:     "",
				Model:      "nomic-embed-text",
				Dimensions: 256,
			},
			Chat: ModelEndpointConfig{
				Provider: "ollama",
				BaseURL:  "http://host.docker.internal:11434",
				APIKey:   "",
				Model:    "llama3.2:3b",
			},
		},
		MCP: MCPConfig{
			Enabled: true,
			Path:    "/mcp",
		},
		Auth: AuthConfig{
			AdminUsername: "admin",
			AdminPassword: "",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load reads and parses a YAML configuration file with environment variable substitution.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses YAML configuration data with environment variable substitution and validation.
func Parse(data []byte) (*Config, error) {
	expanded := ExpandEnv(string(data))

	var raw struct {
		AI struct {
			Provider            *string `yaml:"provider"`
			BaseURL             *string `yaml:"base_url"`
			APIKey              *string `yaml:"api_key"`
			EmbeddingModel      *string `yaml:"embedding_model"`
			EmbeddingDimensions *int    `yaml:"embedding_dimensions"`
			ChatModel           *string `yaml:"chat_model"`
			SummaryModel        *string `yaml:"summary_model"`
			Embedder            struct {
				Provider   *string `yaml:"provider"`
				BaseURL    *string `yaml:"base_url"`
				APIKey     *string `yaml:"api_key"`
				Model      *string `yaml:"model"`
				Dimensions *int    `yaml:"dimensions"`
			} `yaml:"embedder"`
			Chat struct {
				Provider *string `yaml:"provider"`
				BaseURL  *string `yaml:"base_url"`
				APIKey   *string `yaml:"api_key"`
				Model    *string `yaml:"model"`
			} `yaml:"chat"`
		} `yaml:"ai"`
	}
	_ = yaml.Unmarshal([]byte(expanded), &raw)

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config yaml: %w", err)
	}

	if raw.AI.ChatModel != nil && raw.AI.SummaryModel == nil {
		cfg.AI.SummaryModel = *raw.AI.ChatModel
	} else if raw.AI.SummaryModel != nil && raw.AI.ChatModel == nil {
		cfg.AI.ChatModel = *raw.AI.SummaryModel
	} else if raw.AI.ChatModel != nil && raw.AI.SummaryModel != nil {
		cfg.AI.SummaryModel = *raw.AI.ChatModel
	}

	// Resolve Embedder inheritance from top-level AI
	if raw.AI.Embedder.Provider != nil {
		cfg.AI.Embedder.Provider = *raw.AI.Embedder.Provider
	} else if raw.AI.Provider != nil {
		cfg.AI.Embedder.Provider = *raw.AI.Provider
	}
	if raw.AI.Embedder.Model != nil {
		cfg.AI.Embedder.Model = *raw.AI.Embedder.Model
	} else if raw.AI.EmbeddingModel != nil {
		cfg.AI.Embedder.Model = *raw.AI.EmbeddingModel
	}
	if raw.AI.Embedder.Dimensions != nil {
		cfg.AI.Embedder.Dimensions = *raw.AI.Embedder.Dimensions
	} else if raw.AI.EmbeddingDimensions != nil {
		cfg.AI.Embedder.Dimensions = *raw.AI.EmbeddingDimensions
	}
	if raw.AI.Embedder.BaseURL != nil {
		cfg.AI.Embedder.BaseURL = *raw.AI.Embedder.BaseURL
	} else if raw.AI.BaseURL != nil && (raw.AI.Embedder.Provider == nil || *raw.AI.Embedder.Provider == cfg.AI.Provider) {
		cfg.AI.Embedder.BaseURL = *raw.AI.BaseURL
	} else if raw.AI.Embedder.Provider != nil && *raw.AI.Embedder.Provider != cfg.AI.Provider {
		cfg.AI.Embedder.BaseURL = DefaultBaseURLForProvider(cfg.AI.Embedder.Provider)
	}
	if raw.AI.Embedder.APIKey != nil {
		cfg.AI.Embedder.APIKey = *raw.AI.Embedder.APIKey
	} else if raw.AI.APIKey != nil && (raw.AI.Embedder.Provider == nil || *raw.AI.Embedder.Provider == cfg.AI.Provider) {
		cfg.AI.Embedder.APIKey = *raw.AI.APIKey
	} else if raw.AI.Embedder.Provider != nil && *raw.AI.Embedder.Provider != cfg.AI.Provider {
		cfg.AI.Embedder.APIKey = ""
	}

	// Resolve Chat inheritance from top-level AI
	if raw.AI.Chat.Provider != nil {
		cfg.AI.Chat.Provider = *raw.AI.Chat.Provider
	} else if raw.AI.Provider != nil {
		cfg.AI.Chat.Provider = *raw.AI.Provider
	}
	if raw.AI.Chat.Model != nil {
		cfg.AI.Chat.Model = *raw.AI.Chat.Model
	} else if raw.AI.ChatModel != nil {
		cfg.AI.Chat.Model = *raw.AI.ChatModel
	} else if raw.AI.SummaryModel != nil {
		cfg.AI.Chat.Model = *raw.AI.SummaryModel
	}
	if raw.AI.Chat.BaseURL != nil {
		cfg.AI.Chat.BaseURL = *raw.AI.Chat.BaseURL
	} else if raw.AI.BaseURL != nil && (raw.AI.Chat.Provider == nil || *raw.AI.Chat.Provider == cfg.AI.Provider) {
		cfg.AI.Chat.BaseURL = *raw.AI.BaseURL
	} else if raw.AI.Chat.Provider != nil && *raw.AI.Chat.Provider != cfg.AI.Provider {
		cfg.AI.Chat.BaseURL = DefaultBaseURLForProvider(cfg.AI.Chat.Provider)
	}
	if raw.AI.Chat.APIKey != nil {
		cfg.AI.Chat.APIKey = *raw.AI.Chat.APIKey
	} else if raw.AI.APIKey != nil && (raw.AI.Chat.Provider == nil || *raw.AI.Chat.Provider == cfg.AI.Provider) {
		cfg.AI.Chat.APIKey = *raw.AI.APIKey
	} else if raw.AI.Chat.Provider != nil && *raw.AI.Chat.Provider != cfg.AI.Provider {
		cfg.AI.Chat.APIKey = ""
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

var envPattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)(?::-([^}]*))?\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

// ExpandEnv substitutes environment variables in the format ${VAR}, ${VAR:-default}, or $VAR.
func ExpandEnv(s string) string {
	return envPattern.ReplaceAllStringFunc(s, func(m string) string {
		matches := envPattern.FindStringSubmatch(m)
		if len(matches) > 1 && matches[1] != "" {
			val, exists := os.LookupEnv(matches[1])
			if exists && val != "" {
				return val
			}
			if len(matches) > 2 && matches[2] != "" {
				return matches[2]
			}
			return ""
		}
		if len(matches) > 3 && matches[3] != "" {
			return os.Getenv(matches[3])
		}
		return m
	})
}

// ValidateSSRF verifies that a URL does not target link-local or cloud metadata IP addresses.
func ValidateSSRF(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return nil
	}

	lowerHost := strings.ToLower(hostname)
	if lowerHost == "169.254.169.254" || strings.HasPrefix(lowerHost, "169.254.") {
		return fmt.Errorf("prohibited cloud metadata address: %s", hostname)
	}
	if lowerHost == "metadata.google.internal" || lowerHost == "metadata" {
		return fmt.Errorf("prohibited cloud metadata host: %s", hostname)
	}

	if ip := net.ParseIP(hostname); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			if ip4[0] == 169 && ip4[1] == 254 {
				return fmt.Errorf("prohibited IPv4 link-local address: %s", hostname)
			}
		} else {
			if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				return fmt.Errorf("prohibited IPv6 link-local address: %s", hostname)
			}
			if strings.EqualFold(ip.String(), "fd00:ec2::254") {
				return fmt.Errorf("prohibited IPv6 cloud metadata address: %s", hostname)
			}
		}
	}

	return nil
}

// Validate checks configuration invariants.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if strings.TrimSpace(c.Server.Host) == "" {
		c.Server.Host = "0.0.0.0"
	}

	dbType := strings.ToLower(strings.TrimSpace(c.Database.Type))
	if dbType != "sqlite" && dbType != "postgres" {
		return fmt.Errorf("database.type must be 'sqlite' or 'postgres', got '%s'", c.Database.Type)
	}
	c.Database.Type = dbType

	if c.Database.Type == "sqlite" && strings.TrimSpace(c.Database.SQLite.Path) == "" {
		return fmt.Errorf("database.sqlite.path is required when database.type is sqlite")
	}
	if c.Database.Type == "postgres" && strings.TrimSpace(c.Database.Postgres.DSN) == "" {
		return fmt.Errorf("database.postgres.dsn is required when database.type is postgres")
	}

	if strings.TrimSpace(c.Storage.LibraryDir) == "" {
		return fmt.Errorf("storage.library_dir must not be empty")
	}
	if strings.TrimSpace(c.Storage.DataDir) == "" {
		return fmt.Errorf("storage.data_dir must not be empty")
	}

	if c.AI.Provider != "" {
		provider := strings.ToLower(strings.TrimSpace(c.AI.Provider))
		if provider != "ollama" && provider != "openai" && provider != "google" && provider != "gemini" {
			return fmt.Errorf("ai.provider must be 'ollama', 'openai', or 'google', got '%s'", c.AI.Provider)
		}
		c.AI.Provider = provider
	}
	if c.AI.Embedder.Provider != "" {
		provider := strings.ToLower(strings.TrimSpace(c.AI.Embedder.Provider))
		if provider != "ollama" && provider != "openai" && provider != "google" && provider != "gemini" {
			return fmt.Errorf("ai.embedder.provider must be 'ollama', 'openai', or 'google', got '%s'", c.AI.Embedder.Provider)
		}
		c.AI.Embedder.Provider = provider
	} else if c.AI.Provider != "" {
		c.AI.Embedder.Provider = c.AI.Provider
	}
	if c.AI.Chat.Provider != "" {
		provider := strings.ToLower(strings.TrimSpace(c.AI.Chat.Provider))
		if provider != "ollama" && provider != "openai" && provider != "google" && provider != "gemini" {
			return fmt.Errorf("ai.chat.provider must be 'ollama', 'openai', or 'google', got '%s'", c.AI.Chat.Provider)
		}
		c.AI.Chat.Provider = provider
	} else if c.AI.Provider != "" {
		c.AI.Chat.Provider = c.AI.Provider
	}

	if c.AI.ChatModel == "" && c.AI.SummaryModel != "" {
		c.AI.ChatModel = c.AI.SummaryModel
	} else if c.AI.SummaryModel == "" && c.AI.ChatModel != "" {
		c.AI.SummaryModel = c.AI.ChatModel
	}

	// Bidirectional dimension synchronization
	if c.AI.Embedder.Dimensions > 0 {
		c.AI.EmbeddingDimensions = c.AI.Embedder.Dimensions
	} else if c.AI.EmbeddingDimensions > 0 {
		c.AI.Embedder.Dimensions = c.AI.EmbeddingDimensions
	} else {
		c.AI.EmbeddingDimensions = 256
		c.AI.Embedder.Dimensions = 256
	}

	// SSRF guards across all configured endpoints
	if err := ValidateSSRF(c.AI.BaseURL); err != nil {
		return fmt.Errorf("ai.base_url contains %w", err)
	}
	if err := ValidateSSRF(c.AI.Embedder.BaseURL); err != nil {
		return fmt.Errorf("ai.embedder.base_url contains %w", err)
	}
	if err := ValidateSSRF(c.AI.Chat.BaseURL); err != nil {
		return fmt.Errorf("ai.chat.base_url contains %w", err)
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	switch strings.ToLower(strings.TrimSpace(c.Logging.Level)) {
	case "debug", "info", "warn", "warning", "error":
		c.Logging.Level = strings.ToLower(strings.TrimSpace(c.Logging.Level))
		if c.Logging.Level == "warning" {
			c.Logging.Level = "warn"
		}
	default:
		return fmt.Errorf("logging.level must be 'debug', 'info', 'warn', or 'error', got '%s'", c.Logging.Level)
	}

	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}
	switch strings.ToLower(strings.TrimSpace(c.Logging.Format)) {
	case "text", "json":
		c.Logging.Format = strings.ToLower(strings.TrimSpace(c.Logging.Format))
	default:
		return fmt.Errorf("logging.format must be 'text' or 'json', got '%s'", c.Logging.Format)
	}

	return nil
}

// ApplyEnvOverrides overrides configuration values from standard SHELFD_* environment variables.
func ApplyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SHELFD_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("SHELFD_SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	} else if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("SHELFD_JWT_SECRET"); v != "" {
		cfg.Server.JWTSecret = v
	} else if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Server.JWTSecret = v
	}
	if v := os.Getenv("SHELFD_SERVER_WEB_DIR"); v != "" {
		cfg.Server.WebDir = v
	} else if v := os.Getenv("SHELFD_WEB_DIR"); v != "" {
		cfg.Server.WebDir = v
	}

	if v := os.Getenv("SHELFD_DATABASE_TYPE"); v != "" {
		cfg.Database.Type = strings.ToLower(strings.TrimSpace(v))
	}
	if v := os.Getenv("SHELFD_SQLITE_PATH"); v != "" {
		cfg.Database.SQLite.Path = v
	}
	if v := os.Getenv("SHELFD_DATABASE_POSTGRES_DSN"); v != "" {
		cfg.Database.Postgres.DSN = v
	} else if v := os.Getenv("SHELFD_POSTGRES_DSN"); v != "" {
		cfg.Database.Postgres.DSN = v
	}

	if v := os.Getenv("SHELFD_STORAGE_LIBRARY_DIR"); v != "" {
		cfg.Storage.LibraryDir = v
	} else if v := os.Getenv("SHELFD_LIBRARY_DIR"); v != "" {
		cfg.Storage.LibraryDir = v
	}
	if v := os.Getenv("SHELFD_STORAGE_DATA_DIR"); v != "" {
		cfg.Storage.DataDir = v
	} else if v := os.Getenv("SHELFD_DATA_DIR"); v != "" {
		cfg.Storage.DataDir = v
	}

	// Legacy AI environment overrides
	if v := os.Getenv("SHELFD_AI_PROVIDER"); v != "" {
		cfg.AI.Provider = strings.ToLower(strings.TrimSpace(v))
		cfg.AI.Embedder.Provider = cfg.AI.Provider
		cfg.AI.Chat.Provider = cfg.AI.Provider
	}
	if v := os.Getenv("SHELFD_AI_BASE_URL"); v != "" {
		cfg.AI.BaseURL = v
		cfg.AI.Embedder.BaseURL = v
		cfg.AI.Chat.BaseURL = v
	}
	if v := os.Getenv("SHELFD_AI_API_KEY"); v != "" {
		cfg.AI.APIKey = v
		cfg.AI.Embedder.APIKey = v
		cfg.AI.Chat.APIKey = v
	}
	if v := os.Getenv("SHELFD_AI_EMBEDDING_MODEL"); v != "" {
		cfg.AI.EmbeddingModel = v
		cfg.AI.Embedder.Model = v
	}
	if v := os.Getenv("SHELFD_AI_EMBEDDING_DIMENSIONS"); v != "" {
		if dims, err := strconv.Atoi(v); err == nil && dims > 0 {
			cfg.AI.EmbeddingDimensions = dims
			cfg.AI.Embedder.Dimensions = dims
		}
	}
	if v := os.Getenv("SHELFD_AI_CHAT_MODEL"); v != "" {
		cfg.AI.ChatModel = v
		cfg.AI.Chat.Model = v
		if cfg.AI.SummaryModel == "" {
			cfg.AI.SummaryModel = v
		}
	}
	if v := os.Getenv("SHELFD_AI_SUMMARY_MODEL"); v != "" {
		cfg.AI.SummaryModel = v
		if cfg.AI.Chat.Model == "" || cfg.AI.ChatModel == "" {
			cfg.AI.ChatModel = v
			cfg.AI.Chat.Model = v
		}
	}

	// Specific Embedder environment overrides
	if v := os.Getenv("SHELFD_AI_EMBEDDER_PROVIDER"); v != "" {
		cfg.AI.Embedder.Provider = strings.ToLower(strings.TrimSpace(v))
	}
	if v := os.Getenv("SHELFD_AI_EMBEDDER_BASE_URL"); v != "" {
		cfg.AI.Embedder.BaseURL = v
	}
	if v := os.Getenv("SHELFD_AI_EMBEDDER_API_KEY"); v != "" {
		cfg.AI.Embedder.APIKey = v
	}
	if v := os.Getenv("SHELFD_AI_EMBEDDER_MODEL"); v != "" {
		cfg.AI.Embedder.Model = v
	}
	if v := os.Getenv("SHELFD_AI_EMBEDDER_DIMENSIONS"); v != "" {
		if dims, err := strconv.Atoi(v); err == nil && dims > 0 {
			cfg.AI.Embedder.Dimensions = dims
			cfg.AI.EmbeddingDimensions = dims
		}
	}

	// Specific Chat environment overrides
	if v := os.Getenv("SHELFD_AI_CHAT_PROVIDER"); v != "" {
		cfg.AI.Chat.Provider = strings.ToLower(strings.TrimSpace(v))
	}
	if v := os.Getenv("SHELFD_AI_CHAT_BASE_URL"); v != "" {
		cfg.AI.Chat.BaseURL = v
	}
	if v := os.Getenv("SHELFD_AI_CHAT_API_KEY"); v != "" {
		cfg.AI.Chat.APIKey = v
	}
	if v := os.Getenv("SHELFD_AI_CHAT_MODEL"); v != "" {
		cfg.AI.Chat.Model = v
	}

	// Resync dimensions
	if cfg.AI.Embedder.Dimensions > 0 {
		cfg.AI.EmbeddingDimensions = cfg.AI.Embedder.Dimensions
	} else if cfg.AI.EmbeddingDimensions > 0 {
		cfg.AI.Embedder.Dimensions = cfg.AI.EmbeddingDimensions
	}

	if v := os.Getenv("SHELFD_MCP_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.MCP.Enabled = b
		}
	}
	if v := os.Getenv("SHELFD_MCP_PATH"); v != "" {
		cfg.MCP.Path = v
	}

	if v := os.Getenv("SHELFD_ADMIN_USERNAME"); v != "" {
		cfg.Auth.AdminUsername = v
	} else if v := os.Getenv("SHELFD_AUTH_ADMIN_USERNAME"); v != "" {
		cfg.Auth.AdminUsername = v
	}
	if v := os.Getenv("SHELFD_ADMIN_PASSWORD"); v != "" {
		cfg.Auth.AdminPassword = v
	} else if v := os.Getenv("SHELFD_AUTH_ADMIN_PASSWORD"); v != "" {
		cfg.Auth.AdminPassword = v
	}

	if v := os.Getenv("SHELFD_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = strings.ToLower(strings.TrimSpace(v))
	} else if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = strings.ToLower(strings.TrimSpace(v))
	}
	if v := os.Getenv("SHELFD_LOG_FORMAT"); v != "" {
		cfg.Logging.Format = strings.ToLower(strings.TrimSpace(v))
	} else if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Logging.Format = strings.ToLower(strings.TrimSpace(v))
	}
}

// ResolveConfigPath locates the active configuration file in order of priority:
// 1. Explicit CLI flag argument
// 2. SHELFD_CONFIG environment variable
// 3. Candidate files: config.yaml, /config/config.yaml, /data/config.yaml, /config.yaml
func ResolveConfigPath(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	if envPath := os.Getenv("SHELFD_CONFIG"); strings.TrimSpace(envPath) != "" {
		return envPath
	}
	candidates := []string{
		"config.yaml",
		"/config/config.yaml",
		"/data/config.yaml",
		"/config.yaml",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

