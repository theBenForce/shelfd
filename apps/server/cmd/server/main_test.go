package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/api"
)

func TestEnsureJWTSecretPersistsAcrossRestarts(t *testing.T) {
	tmpDir := t.TempDir()

	// First startup: generates and persists secret to <dataDir>/.jwt_secret
	secret1 := ensureJWTSecret(tmpDir, nil)
	if len(secret1) != 64 {
		t.Fatalf("expected 64-char hex secret, got %d chars (%q)", len(secret1), secret1)
	}

	secretPath := filepath.Join(tmpDir, ".jwt_secret")
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("expected .jwt_secret file to exist at %s: %v", secretPath, err)
	}
	if strings.TrimSpace(string(raw)) != secret1 {
		t.Fatalf("expected persisted secret %q, got %q", secret1, strings.TrimSpace(string(raw)))
	}

	// Issue a JWT with the secret from first startup
	token, _, err := api.GenerateJWT("user-1", "admin", secret1, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	// Second startup (simulating container update/restart): loads persisted secret
	secret2 := ensureJWTSecret(tmpDir, nil)
	if secret2 != secret1 {
		t.Fatalf("expected second startup secret %q to match first %q", secret2, secret1)
	}

	// Token issued before restart must still validate after restart
	claims, err := api.ValidateJWT(token, secret2)
	if err != nil {
		t.Fatalf("expected token from before restart to remain valid, got error: %v", err)
	}
	if claims.UserID != "user-1" || claims.Username != "admin" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}
