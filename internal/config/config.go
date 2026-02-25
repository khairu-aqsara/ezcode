package config

import (
	"encoding/json"
	"os"
)

// ServerMode controls how the MCP server is launched.
// "docker" (default) uses docker compose; "stdio" spawns the Node.js binary directly.
type ServerMode string

const (
	ServerModeDocker ServerMode = "docker"
	ServerModeStdio  ServerMode = "stdio"
)

// Config represents the application configuration
type Config struct {
	Image      string            `json:"image"`
	ServerMode ServerMode        `json:"server_mode,omitempty"` // "docker" | "stdio"
	ServerPath string            `json:"server_path,omitempty"` // path to node build/index.js (stdio mode only)
	Env        map[string]string `json:"env"`
}

// IsStdioMode returns true when the MCP server should be launched as a local
// Node.js subprocess rather than via Docker Compose.
func (c *Config) IsStdioMode() bool {
	return c.ServerMode == ServerModeStdio
}

// NewDefaultConfig returns a configuration struct with the required defaults
func NewDefaultConfig() *Config {
	return &Config{
		Image:      "mhalder/qdrant-mcp-server",
		ServerMode: ServerModeDocker,
		ServerPath: "",
		Env: map[string]string{
			"OPENAI_API_KEY":       "", // Required — set via the Setup tab or edit ~/.ezcode/config.json
			"QDRANT_API_KEY":       "",
			"EMBEDDING_MODEL":      "models/gemini-embedding-001",
			"LLM_MODEL":            "gemini-2.5-flash",
			"TRANSPORT_MODE":       "http",
			"EMBEDDING_PROVIDER":   "openai",
			"OPENAI_BASE_URL":      "https://generativelanguage.googleapis.com/v1beta/openai",
			"LOG_LEVEL":            "info",
			"QDRANT_URL":           "http://localhost:6333", // Docker users connecting to a host Qdrant should use host.docker.internal
			"HTTP_PORT":            "3000",
			"EMBEDDING_DIMENSIONS": "3072",
		},
	}
}

// SaveConfig writes the configuration to the specified path
func SaveConfig(config *Config, path string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadConfig reads the configuration from the specified path
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
