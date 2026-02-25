package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := NewDefaultConfig()

	if config.Image != "mhalder/qdrant-mcp-server" {
		t.Errorf("Expected image %s, got %s", "mhalder/qdrant-mcp-server", config.Image)
	}

	expectedEnvs := map[string]string{
		"OPENAI_API_KEY":       "",
		"QDRANT_API_KEY":       "",
		"EMBEDDING_MODEL":      "models/gemini-embedding-001",
		"TRANSPORT_MODE":       "http",
		"EMBEDDING_PROVIDER":   "openai",
		"OPENAI_BASE_URL":      "https://generativelanguage.googleapis.com/v1beta/openai",
		"LOG_LEVEL":            "info",
		"QDRANT_URL":           "http://localhost:6333",
		"HTTP_PORT":            "3000",
		"EMBEDDING_DIMENSIONS": "3072",
	}

	for k, expectedVal := range expectedEnvs {
		if val, ok := config.Env[k]; !ok || val != expectedVal {
			t.Errorf("Expected env %s=%s, got %s", k, expectedVal, val)
		}
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	config := NewDefaultConfig()
	config.Env["OPENAI_API_KEY"] = "new_test_key"

	err := SaveConfig(config, configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	loadedConfig, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loadedConfig.Env["OPENAI_API_KEY"] != "new_test_key" {
		t.Errorf("Expected modified key 'new_test_key', got %s", loadedConfig.Env["OPENAI_API_KEY"])
	}
}
