package oslayer

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// SafeCommandExec coordinates secure, injection-safe OS calls
type SafeCommandExec struct{}

var bannedShells = []string{"sh", "bash", "cmd", "powershell", "cmd.exe", "powershell.exe", "zsh", "ash"}

// Execute executes a command securely using parameterized arguments only
func (s *SafeCommandExec) Execute(ctx context.Context, binary string, args []string, timeout time.Duration) (string, error) {
	// 1. Audit binary to prevent shell wrapping bypasses
	lowerBinary := strings.ToLower(binary)
	for _, banned := range bannedShells {
		if lowerBinary == banned || strings.HasSuffix(lowerBinary, "/"+banned) || strings.HasSuffix(lowerBinary, "\\"+banned) {
			return "", errors.New("security audit failure: raw shell invocations are strictly forbidden in NeoCP core")
		}
	}

	// 2. Enforce context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 3. Structured invocation preventing shell evaluation
	cmd := exec.CommandContext(cmdCtx, binary, args...)

	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	output := outBuf.String()

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return output, errors.New("process execution exceeded timeout limits")
		}
		return output, err
	}

	return output, nil
}
