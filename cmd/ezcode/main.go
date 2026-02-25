package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/khairu-aqsara/ezcode/internal/config"
	"github.com/khairu-aqsara/ezcode/internal/docker"
	"github.com/khairu-aqsara/ezcode/ui/components"
)

func main() {
	// Initialize config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	configDir := filepath.Join(homeDir, ".ezcode")
	configPath := filepath.Join(configDir, "config.json")

	// Ensure config dir exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(configPath)
	isFirstRun := false
	if err != nil {
		isFirstRun = true
		// If load fails (e.g., doesn't exist), fall back to defaults
		cfg = config.NewDefaultConfig()
		// Attempt to save the defaults immediately
		_ = config.SaveConfig(cfg, configPath)
	}

	// Always override the PROJECT_PATH with the current CWD where the binary is run.
	// This is not persisted to config — it is re-evaluated on every launch.
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}
	cfg.Env["PROJECT_PATH"] = cwd

	// Generate the Docker Compose file only when running in Docker mode.
	// In stdio mode there is no container, so no compose file is needed.
	composePath := filepath.Join(configDir, "docker-compose.yaml")
	if !cfg.IsStdioMode() {
		if err := docker.GenerateComposeFile(composePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating compose file: %v\n", err)
			os.Exit(1)
		}
	}

	// Initialize the top-level app model
	app := components.NewApp(cfg, composePath, cwd, isFirstRun)

	// Run the Bubble Tea program
	p := tea.NewProgram(app, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}

	// Only tear down Docker if this session was the one that started it.
	// Stdio mode never starts Docker so this is always false in that case.
	if finalApp, ok := finalModel.(*components.AppModel); ok && finalApp.WasDockerStarted() {
		fmt.Println("Shutting down MCP server...")
		if err := docker.NewManager(nil).ComposeDown(context.Background(), composePath, cfg.Env); err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping docker compose: %v\n", err)
		}
	}
}
