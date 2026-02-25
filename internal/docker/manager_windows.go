//go:build windows

package docker

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

// Run executes a command silently on Windows
func (c *OSCommander) Run(ctx context.Context, env map[string]string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	if env != nil {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	// Prevent console window from flashing on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	out, err := cmd.CombinedOutput()
	return string(out), err
}
