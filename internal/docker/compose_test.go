package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateComposeFile(t *testing.T) {
	tempDir := t.TempDir()
	composePath := filepath.Join(tempDir, "docker-compose.yaml")

	err := GenerateComposeFile(composePath)
	if err != nil {
		t.Fatalf("Failed to generate compose file: %v", err)
	}

	content, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("Failed to read generated compose file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Generated compose file is empty")
	}

	// Test it doesn't overwrite
	err = os.WriteFile(composePath, []byte("custom"), 0644)
	if err != nil {
		t.Fatalf("Failed to write custom content: %v", err)
	}

	err = GenerateComposeFile(composePath)
	if err != nil {
		t.Fatalf("GenerateComposeFile returned error on existing file: %v", err)
	}

	content, _ = os.ReadFile(composePath)
	if string(content) != "custom" {
		t.Error("GenerateComposeFile overwrote an existing file")
	}
}
