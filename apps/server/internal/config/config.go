package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
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
}

type ServerConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	JWTSecret string `yaml:"jwt_secret"`
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
	Provider            string `yaml:"provider"` // "ollama" or "openai"
	BaseURL             string `yaml:"base_url"`
	APIKey              string `yaml:"api_key"`
	EmbeddingModel      string `yaml:"embedding_model"`
	EmbeddingDimensions int    `yaml:"embedding_dimensions"`
	SummaryModel        string `yaml:"summary_model"`
}

type MCPConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// DefaultConfig returns the production/homelab default configuration.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:      "0.0.0.0",
			Port:      8080,
			JWTSecret: "",
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
			EmbeddingDimensions: 1536,
			SummaryModel:        "llama3.2:3b",
		},
		MCP: MCPConfig{
			Enabled: true,
			Path:    "/mcp",
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

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config yaml: %w", err)
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
		if provider != "ollama" && provider != "openai" {
			return fmt.Errorf("ai.provider must be 'ollama' or 'openai', got '%s'", c.AI.Provider)
		}
		c.AI.Provider = provider
	}

	if c.AI.EmbeddingDimensions <= 0 {
		c.AI.EmbeddingDimensions = 1536
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

	return nil
}
