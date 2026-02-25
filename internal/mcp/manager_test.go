package mcp

import (
	"context"
	"testing"
	"time"
)

func TestProjectNameFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/Users/dev/code", "code"},
		{"/app/project", "project"},
		{"code", "code"},
		{"", "project"},
		{".", "project"},
		{"/some/path/my-project", "my_project"},
		{"/path/with.dots", "with_dots"},
	}
	for _, tt := range tests {
		got := ProjectNameFromPath(tt.path)
		if got != tt.want {
			t.Errorf("ProjectNameFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// Disabled mock tests until interface is correctly re-abstracted around client.Client

func TestInitializeIntegration(t *testing.T) {
	// Only run if qdrant-mcp container is actually up on 3000
	client, err := GetDefaultClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	mgr := NewManager(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = mgr.Initialize(ctx)
	if err != nil {
		t.Skipf("Skipping integration test, server likely offline: %v", err)
	}

	err = mgr.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}
