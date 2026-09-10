package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shelfd/shelfd/internal/config"
)

func TestLoadDotEnv_LoadsVariables(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	content := `
# Comment line
TEST_DOTENV_A=foo
export TEST_DOTENV_B="bar with spaces"
TEST_DOTENV_C='single quotes'
TEST_DOTENV_D=unquoted_value
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test .env file: %v", err)
	}

	loaded := config.LoadDotEnv(envPath)
	if loaded != envPath {
		t.Errorf("expected loaded path %s, got %s", envPath, loaded)
	}

	defer func() {
		os.Unsetenv("TEST_DOTENV_A")
		os.Unsetenv("TEST_DOTENV_B")
		os.Unsetenv("TEST_DOTENV_C")
		os.Unsetenv("TEST_DOTENV_D")
	}()

	if os.Getenv("TEST_DOTENV_A") != "foo" {
		t.Errorf("expected foo, got %s", os.Getenv("TEST_DOTENV_A"))
	}
	if os.Getenv("TEST_DOTENV_B") != "bar with spaces" {
		t.Errorf("expected 'bar with spaces', got %s", os.Getenv("TEST_DOTENV_B"))
	}
	if os.Getenv("TEST_DOTENV_C") != "single quotes" {
		t.Errorf("expected 'single quotes', got %s", os.Getenv("TEST_DOTENV_C"))
	}
	if os.Getenv("TEST_DOTENV_D") != "unquoted_value" {
		t.Errorf("expected unquoted_value, got %s", os.Getenv("TEST_DOTENV_D"))
	}
}

func TestLoadDotEnv_DoesNotOverwriteExisting(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	os.Setenv("TEST_EXISTING_KEY", "original_value")
	defer os.Unsetenv("TEST_EXISTING_KEY")

	content := "TEST_EXISTING_KEY=new_value\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test .env file: %v", err)
	}

	config.LoadDotEnv(envPath)

	if os.Getenv("TEST_EXISTING_KEY") != "original_value" {
		t.Errorf("expected original_value to be preserved, got %s", os.Getenv("TEST_EXISTING_KEY"))
	}
}

func TestLoadDotEnv_MissingFile(t *testing.T) {
	loaded := config.LoadDotEnv("/non/existent/path/.env")
	if loaded != "" {
		t.Errorf("expected empty string for missing file, got %s", loaded)
	}
}
