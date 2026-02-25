package components

import (
	"testing"

	"github.com/khairu-aqsara/ezcode/internal/config"
)

func TestConfigFormInitialization(t *testing.T) {
	cfg := config.NewDefaultConfig()
	form := NewConfigForm(cfg, true)

	// 11 env-var inputs + 2 top-level fields (server_mode, server_path) = 13
	if len(form.inputs) != 13 {
		t.Errorf("Expected 13 inputs (11 env vars + server_mode + server_path), got %d", len(form.inputs))
	}

	if form.inputs[0].Value() != cfg.Env["OPENAI_API_KEY"] {
		t.Errorf("Expected first input to match config OPENAI_API_KEY")
	}
}

func TestConfigFormValidation(t *testing.T) {
	cfg := config.NewDefaultConfig()
	form := NewConfigForm(cfg, true)

	// Simulate user clearing the API key
	form.inputs[0].SetValue("")

	err := form.Validate()
	if err == nil {
		t.Error("Expected validation error for empty API key")
	}

	form.inputs[0].SetValue("valid_key")
	err = form.Validate()
	if err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}
}
