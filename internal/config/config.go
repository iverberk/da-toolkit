package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config contains only non-secret user preferences. API credentials, OAuth
// tokens, and encryption keys must be supplied to future workflows in memory.
type Config struct {
	WorkspaceDir string `json:"workspace_dir,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
}

var secretFieldNames = []string{
	"api_key",
	"apikey",
	"aes_key",
	"client_secret",
	"credential",
	"oauth_token",
	"password",
	"refresh_token",
	"secret",
	"token",
}

// Load reads a JSON config file containing non-secret settings. Empty paths use
// defaults and do not create or persist anything.
func Load(path string) (Config, error) {
	cfg := Config{OutputFormat: "text"}
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	for key := range raw {
		if isSecretLike(key) {
			return Config{}, fmt.Errorf("config field %q is not allowed because secrets are memory-only", key)
		}
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config fields: %w", err)
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "text"
	}
	if cfg.OutputFormat != "text" && cfg.OutputFormat != "json" {
		return Config{}, errors.New("output_format must be text or json")
	}
	return cfg, nil
}

func isSecretLike(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	for _, secret := range secretFieldNames {
		if normalized == secret || strings.Contains(normalized, "_"+secret) || strings.Contains(normalized, secret+"_") {
			return true
		}
	}
	return false
}
