package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func executeCommand(args ...string) (string, string, error) {
	resetUploadCommandFlags()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs(nil)
	resetUploadCommandFlags()
	return stdout.String(), stderr.String(), err
}

func resetUploadCommandFlags() {
	uploadFlags = struct {
		baseURL        string
		tokenURL       string
		clientID       string
		clientSecret   string
		workspaceDir   string
		checkpointPath string
		resume         bool
		yes            bool
		analysisType   string
		zipPasswords   []string
	}{}
	uploadCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
	uploadFlags.analysisType = "other"
	uploadFlags.zipPasswords = nil
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
	for _, name := range []string{"analysis", "result", "acquisition", "gui"} {
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

func TestUploadRequiresFilesOrResume(t *testing.T) {
	_, _, err := executeCommand("upload")
	if err == nil || !strings.Contains(err.Error(), "at least one local file is required") {
		t.Fatalf("error = %v, want missing file validation", err)
	}
}

func TestUploadResumeReadsCheckpointBeforeCredentials(t *testing.T) {
	t.Setenv("DA_TOOLKIT_CLIENT_ID", "")
	t.Setenv("DA_TOOLKIT_CLIENT_SECRET", "")
	t.Setenv("DA_TOOLKIT_TOKEN_URL", "")
	missing := filepath.Join(t.TempDir(), "missing-checkpoint.json")
	_, _, err := executeCommand("upload", "--resume", "--checkpoint", missing)
	if err == nil || !strings.Contains(err.Error(), "read checkpoint") {
		t.Fatalf("error = %v, want checkpoint read error", err)
	}
	if strings.Contains(err.Error(), "oauth client id") || strings.Contains(err.Error(), "client secret") || strings.Contains(err.Error(), "token URL") {
		t.Fatalf("resume required credentials before reading checkpoint: %v", err)
	}
}

func TestDefaultUploadWorkspaceIsStableForSameFiles(t *testing.T) {
	rel := filepath.Join(".", "testdata", "evidence.txt")
	abs, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}

	first := defaultUploadWorkspace([]string{abs})
	second := defaultUploadWorkspace([]string{abs})
	fromRelative := defaultUploadWorkspace([]string{rel})

	if first != second || first != fromRelative {
		t.Fatalf("default workspace is not stable: %q %q %q", first, second, fromRelative)
	}
	if !strings.HasPrefix(first, filepath.Join(".da-toolkit", "upload-")) || len(strings.TrimPrefix(filepath.Base(first), "upload-")) != 16 {
		t.Fatalf("default workspace = %q, want static upload identifier", first)
	}
}

func TestUnknownCommandUsesUserFacingErrorPattern(t *testing.T) {
	_, _, err := executeCommand("missing")
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %v, want unknown command", err)
	}
}
