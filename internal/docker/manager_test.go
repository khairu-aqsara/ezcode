package docker

import (
	"context"
	"testing"
)

// MockCommander allows us to mock os/exec calls for testing
type MockCommander struct {
	Outputs map[string]string
	Errors  map[string]error
}

func (m *MockCommander) Run(ctx context.Context, env map[string]string, name string, args ...string) (string, error) {
	cmdStr := name
	for _, arg := range args {
		cmdStr += " " + arg
	}

	if err, ok := m.Errors[cmdStr]; ok && err != nil {
		return "", err
	}

	return m.Outputs[cmdStr], nil
}

func TestCheckDockerInstalled(t *testing.T) {
	mockCmd := &MockCommander{
		Outputs: map[string]string{
			"docker --version": "Docker version 24.0.5, build ced0996",
		},
	}

	manager := NewManager(mockCmd)
	installed, err := manager.IsInstalled(context.Background())

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !installed {
		t.Error("Expected Docker to be installed")
	}
}

func TestCheckDockerDaemonRunning(t *testing.T) {
	mockCmd := &MockCommander{
		Outputs: map[string]string{
			"docker info": "Client:\n Context:    default\n...",
		},
	}

	manager := NewManager(mockCmd)
	running, err := manager.IsDaemonRunning(context.Background())

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !running {
		t.Error("Expected Docker daemon to be running")
	}
}

func TestComposeUpSubprocess(t *testing.T) {
	mockCmd := &MockCommander{
		Outputs: map[string]string{
			"docker compose -f docker-compose.yaml up -d": "Container started",
		},
	}

	manager := NewManager(mockCmd)
	err := manager.ComposeUp(context.Background(), "docker-compose.yaml", nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestComposeDownSubprocess(t *testing.T) {
	mockCmd := &MockCommander{
		Outputs: map[string]string{
			"docker compose -f docker-compose.yaml down -v": "Container stopped",
		},
	}

	manager := NewManager(mockCmd)
	err := manager.ComposeDown(context.Background(), "docker-compose.yaml", nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
