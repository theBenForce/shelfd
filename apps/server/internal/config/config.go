package config

import (
	"fmt"
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

type AIConfig struct {
	Provider            string `yaml:"provider"` // "ollama", "openai", or "google" ("gemini")
	BaseURL             string `yaml:"base_url"`
	APIKey              string `yaml:"api_key"`
	EmbeddingModel      string `yaml:"embedding_model"`
	EmbeddingDimensions int    `yaml:"embedding_dimensions"`
	ChatModel           string `yaml:"chat_model"`
	SummaryModel        string `yaml:"summary_model"` // Deprecated alias for ChatModel
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
			ChatModel    *string `yaml:"chat_model"`
			SummaryModel *string `yaml:"summary_model"`
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

	if c.AI.ChatModel == "" && c.AI.SummaryModel != "" {
		c.AI.ChatModel = c.AI.SummaryModel
	} else if c.AI.SummaryModel == "" && c.AI.ChatModel != "" {
		c.AI.SummaryModel = c.AI.ChatModel
	}

	if c.AI.EmbeddingDimensions <= 0 {
		c.AI.EmbeddingDimensions = 256
	}

	// SSRF guard: reject cloud metadata IP addresses in AI base_url
	if c.AI.BaseURL != "" {
		parsed, err := url.Parse(c.AI.BaseURL)
		if err != nil {
			return fmt.Errorf("invalid ai.base_url: %w", err)
		}
		hostname := parsed.Hostname()
		if hostname == "169.254.169.254" || strings.HasPrefix(hostname, "169.254.") {
			return fmt.Errorf("ai.base_url contains prohibited cloud metadata address: %s", hostname)
		}
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

	if v := os.Getenv("SHELFD_AI_PROVIDER"); v != "" {
		cfg.AI.Provider = strings.ToLower(strings.TrimSpace(v))
	}
	if v := os.Getenv("SHELFD_AI_BASE_URL"); v != "" {
		cfg.AI.BaseURL = v
	}
	if v := os.Getenv("SHELFD_AI_API_KEY"); v != "" {
		cfg.AI.APIKey = v
	}
	if v := os.Getenv("SHELFD_AI_EMBEDDING_MODEL"); v != "" {
		cfg.AI.EmbeddingModel = v
	}
	if v := os.Getenv("SHELFD_AI_EMBEDDING_DIMENSIONS"); v != "" {
		if dims, err := strconv.Atoi(v); err == nil && dims > 0 {
			cfg.AI.EmbeddingDimensions = dims
		}
	}
	if v := os.Getenv("SHELFD_AI_CHAT_MODEL"); v != "" {
		cfg.AI.ChatModel = v
		if cfg.AI.SummaryModel == "" {
			cfg.AI.SummaryModel = v
		}
	}
	if v := os.Getenv("SHELFD_AI_SUMMARY_MODEL"); v != "" {
		cfg.AI.SummaryModel = v
		if cfg.AI.ChatModel == "" {
			cfg.AI.ChatModel = v
		}
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

