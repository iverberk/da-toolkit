package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func executeCommand(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs(nil)
	return stdout.String(), stderr.String(), err
}

func TestHelpIncludesCommandGroupsAndSecretConvention(t *testing.T) {
	stdout, stderr, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	for _, want := range []string{"analysis", "upload", "result", "acquisition", "gui", "Future workflow settings must be non-secret only", "Do not store API credentials, OAuth tokens, or AES keys"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help missing %q:\n%s", want, stdout)
		}
	}
}

func TestPlaceholderCommandsDoNotRequireCredentials(t *testing.T) {
	for _, name := range []string{"analysis", "upload", "result", "acquisition", "gui"} {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, err := executeCommand(name)
			if err != nil {
				t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stdout, "not implemented yet") || !strings.Contains(stdout, "does not require network access or credentials") {
				t.Fatalf("placeholder output did not explain safe no-op behavior: %q", stdout)
			}
		})
	}
}

func TestUnknownCommandUsesUserFacingErrorPattern(t *testing.T) {
	_, _, err := executeCommand("missing")
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %v, want unknown command", err)
	}
}
