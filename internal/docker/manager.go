package docker

import (
	"context"
)

// Commander is an interface to allow mocking of os/exec calls
type Commander interface {
	Run(ctx context.Context, env map[string]string, name string, args ...string) (string, error)
}

// OSCommander is the default implementation that actually runs commands
type OSCommander struct{}

// Run wrapper logic is defined in manager_unix.go and manager_windows.go

// Manager handles Docker operations
type Manager struct {
	cmd Commander
}

// NewManager creates a new Docker manager
func NewManager(cmd Commander) *Manager {
	if cmd == nil {
		cmd = &OSCommander{}
	}
	return &Manager{cmd: cmd}
}

// IsInstalled checks if the docker CLI is available
func (m *Manager) IsInstalled(ctx context.Context) (bool, error) {
	_, err := m.cmd.Run(ctx, nil, "docker", "--version")
	if err != nil {
		return false, nil // Might legitimately not be installed, not necessarily an error we return up
	}
	return true, nil
}

// IsDaemonRunning checks if we can connect to the Docker daemon
func (m *Manager) IsDaemonRunning(ctx context.Context) (bool, error) {
	_, err := m.cmd.Run(ctx, nil, "docker", "info")
	if err != nil {
		return false, nil
	}
	return true, nil
}

// ComposeUp starts the MCP server
func (m *Manager) ComposeUp(ctx context.Context, composeFile string, env map[string]string) error {
	_, err := m.cmd.Run(ctx, env, "docker", "compose", "-f", composeFile, "up", "-d")
	return err
}

// ComposeDown stops and removes the MCP server
func (m *Manager) ComposeDown(ctx context.Context, composeFile string, env map[string]string) error {
	_, err := m.cmd.Run(ctx, env, "docker", "compose", "-f", composeFile, "down", "-v")
	return err
}
