package components

import (
	"testing"

	"github.com/khairu-aqsara/ezcode/internal/config"
)

func TestAppInitializationFirstRun(t *testing.T) {
	cfg := config.NewDefaultConfig()
	app := NewApp(cfg, "dummy-path", "/tmp/project", true) // true for first run

	if app.chat.activeTab != TabIndexSetup {
		t.Errorf("Expected initial state to be TabIndexSetup setup (0) on first run, got %v", app.chat.activeTab)
	}

	if app.chat.configForm == nil {
		t.Error("Expected ConfigForm to be initialized")
	}
}

func TestAppInitializationNormal(t *testing.T) {
	cfg := config.NewDefaultConfig()
	app := NewApp(cfg, "dummy-path", "/tmp/project", false) // false for normal run

	if app.chat.activeTab != TabIndexChat {
		t.Errorf("Expected initial state to be TabIndexChat (3) on normal run, got %v", app.chat.activeTab)
	}

	if app.chat.dashboard == nil {
		t.Error("Expected Dashboard to be initialized")
	}
}
