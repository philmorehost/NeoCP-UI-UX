package oslayer

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSafeCommandExec_Execute(t *testing.T) {
	execEngine := &SafeCommandExec{}
	ctx := context.Background()

	t.Run("SuccessCase", func(t *testing.T) {
		output, err := execEngine.Execute(ctx, "go", []string{"version"}, 5*time.Second)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !strings.Contains(output, "go version") {
			t.Errorf("Unexpected output: %s", output)
		}
	})

	t.Run("BannedShellBlock", func(t *testing.T) {
		shells := []string{"bash", "sh", "cmd.exe", "/bin/sh", "C:\\Windows\\System32\\cmd.exe"}
		for _, shell := range shells {
			_, err := execEngine.Execute(ctx, shell, []string{"-c", "ls"}, 2*time.Second)
			if err == nil {
				t.Errorf("Security Breach: Banned shell '%s' was not blocked", shell)
			}
			if !strings.Contains(err.Error(), "raw shell invocations are strictly forbidden") {
				t.Errorf("Unexpected error message for shell '%s': %v", shell, err)
			}
		}
	})

	t.Run("TimeoutEnforcement", func(t *testing.T) {
		// Attempt to sleep longer than the timeout
		// Using 'go' to simulate a wait since sleep might not be on all paths easily
		// Actually, standard 'sleep' is fine on Linux.
		_, err := execEngine.Execute(ctx, "sleep", []string{"10"}, 100*time.Millisecond)
		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "exceeded timeout limits") {
			t.Errorf("Expected timeout message, got: %v", err)
		}
	})

	t.Run("InjectionPrevention", func(t *testing.T) {
		// If arguments were evaluated by a shell, 'echo' would print 'hello' and then run 'ls'.
		// In a parameterized execution, 'echo' treats '; ls' as a literal string.
		output, err := execEngine.Execute(ctx, "echo", []string{"hello;", "ls"}, 2*time.Second)
		if err != nil {
			t.Errorf("Echo failed: %v", err)
		}
		// Parameterized 'echo' usually prints all args separated by spaces.
		// Expected: "hello; ls\n" or similar.
		if strings.Contains(output, "NeoCP-UI-UX") { // Assuming ls would show a file in the dir
			t.Error("Potential injection vulnerability: semicolon was interpreted by a shell")
		}
		if !strings.Contains(output, "hello;") {
			t.Errorf("Expected output to contain literal semicolon string, got: %s", output)
		}
	})
}
