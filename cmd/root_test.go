package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func executeCommand(args ...string) (string, string, error) {
	resetAcquisitionCommandFlags()
	resetUploadCommandFlags()
	resetResultCommandFlags()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs(nil)
	resetAcquisitionCommandFlags()
	resetUploadCommandFlags()
	resetResultCommandFlags()
	return stdout.String(), stderr.String(), err
}

func resetAcquisitionCommandFlags() {
	acquisitionPocFlags = struct {
		runtimeDir      string
		sitePackagesDir string
		bootstrap       bool
		cleanup         bool
		acquireArgs     []string
	}{}
	acquisitionPocCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
	acquisitionPocFlags.acquireArgs = nil
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
		requested      []string
		diskKeys       []string
		manualAllowed  bool
	}{}
	uploadCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
	uploadFlags.analysisType = "other"
	uploadFlags.zipPasswords = nil
	uploadFlags.requested = nil
	uploadFlags.diskKeys = nil
}

func resetResultCommandFlags() {
	resultFlags = struct {
		baseURL       string
		tokenURL      string
		clientID      string
		clientSecret  string
		page          int
		size          int
		filters       string
		sort          string
		search        string
		searchFields  string
		caseSensitive bool
		output        string
		overwrite     bool
	}{}
	for _, command := range []*cobra.Command{resultCmd, resultListCmd, resultReadCmd, resultArchiveCmd, resultFileCmd} {
		command.Flags().VisitAll(func(flag *pflag.Flag) {
			_ = flag.Value.Set(flag.DefValue)
			flag.Changed = false
		})
		command.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
			_ = flag.Value.Set(flag.DefValue)
			flag.Changed = false
		})
	}
}

func TestHelpIsConciseAndUsesCobraCommandList(t *testing.T) {
	stdout, stderr, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	for _, want := range []string{"Upload files, retrieve results, and run acquisition workflows.", "Available Commands:", "upload", "result", "acquisition"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help missing %q:\n%s", want, stdout)
		}
	}
	for _, removed := range []string{"Command groups:", "Configuration:", "User-facing errors are written as:", "Do not store API credentials"} {
		if strings.Contains(stdout, removed) {
			t.Fatalf("help includes verbose custom text %q:\n%s", removed, stdout)
		}
	}
	for _, removed := range []string{"analysis", "gui"} {
		if commandNamed(rootCmd, removed) || strings.Contains(stdout, "  "+removed+" ") {
			t.Fatalf("help includes removed placeholder command %q:\n%s", removed, stdout)
		}
	}
}

func commandNamed(command *cobra.Command, name string) bool {
	for _, child := range command.Commands() {
		if child.Name() == name {
			return true
		}
	}
	return false
}

func TestPlaceholderCommandsAreRemoved(t *testing.T) {
	for _, name := range []string{"analysis", "gui"} {
		t.Run(name, func(t *testing.T) {
			_, _, err := executeCommand(name)
			if err == nil || !strings.Contains(err.Error(), "unknown command") {
				t.Fatalf("error = %v, want unknown command", err)
			}
		})
	}
}

func TestAcquisitionPocHelpDocumentsBootstrapAndRuntimeFlags(t *testing.T) {
	stdout, stderr, err := executeCommand("acquisition", "poc", "--help")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, want := range []string{"go-embed-python", "--bootstrap", "--runtime-dir", "--site-packages", "--cleanup", "does not require Internet access"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("acquisition poc help missing %q:\n%s", want, stdout)
		}
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
