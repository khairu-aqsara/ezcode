//go:build !windows

package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Run executes a command
func (c *OSCommander) Run(ctx context.Context, env map[string]string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	if env != nil {
		// Start with the current environment
		envMap := make(map[string]string)
		for _, e := range os.Environ() {
			// Find first '=' to split correctly on values with '='
			for i := 0; i < len(e); i++ {
				if e[i] == '=' {
					envMap[e[:i]] = e[i+1:]
					break
				}
			}
		}

		// Override/add our custom environment variables
		for k, v := range env {
			envMap[k] = v
		}

		// Rebuild cmd.Env
		cmd.Env = make([]string, 0, len(envMap))
		for k, v := range envMap {
			if v != "" {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
		}
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		envDump := ""
		for _, e := range cmd.Env {
			envDump += e + "\n"
		}
		err = fmt.Errorf("command failed: %w, output: %s, env_dump: %s", err, string(out), envDump)
	}
	return string(out), err
}
