package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsPersistedSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"workspace_dir":"/tmp/da","oauth_token":"persisted"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "secrets are memory-only") {
		t.Fatalf("error = %v, want secret persistence error", err)
	}
}

func TestLoadAllowsOnlyNonSecretSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"workspace_dir":"/tmp/da","output_format":"json"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.WorkspaceDir != "/tmp/da" || cfg.OutputFormat != "json" {
		t.Fatalf("cfg = %+v, want workspace_dir and json output", cfg)
	}
}
