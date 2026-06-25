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
		new            bool
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

	for _, want := range []string{"Create, inspect, and retrieve data-analyses, and run acquisition workflows.", "Available Commands:", "analysis", "acquisition"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help missing %q:\n%s", want, stdout)
		}
	}
	for _, removed := range []string{"Command groups:", "Configuration:", "User-facing errors are written as:", "Do not store API credentials"} {
		if strings.Contains(stdout, removed) {
			t.Fatalf("help includes verbose custom text %q:\n%s", removed, stdout)
		}
	}
	for _, removed := range []string{"upload", "result", "gui"} {
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

func TestRemovedTopLevelCommandsAreUnavailable(t *testing.T) {
	for _, name := range []string{"upload", "result", "gui"} {
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

func TestAnalysisCreateRequiresFilesOrResume(t *testing.T) {
	_, _, err := executeCommand("analysis", "create", "--type", "other")
	if err == nil || !strings.Contains(err.Error(), "at least one local file is required") {
		t.Fatalf("error = %v, want missing file validation", err)
	}
}

func TestAnalysisCreateRequiresTypeForFreshCreation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "evidence.txt")
	if err := os.WriteFile(source, []byte("evidence"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := executeCommand("analysis", "create", source)
	if err == nil || !strings.Contains(err.Error(), "--type is required") {
		t.Fatalf("error = %v, want missing type validation", err)
	}
}

func TestAnalysisCreateResumeReadsCheckpointBeforeCredentials(t *testing.T) {
	t.Setenv("DA_TOOLKIT_CLIENT_ID", "")
	t.Setenv("DA_TOOLKIT_CLIENT_SECRET", "")
	t.Setenv("DA_TOOLKIT_TOKEN_URL", "")
	missing := filepath.Join(t.TempDir(), "missing-checkpoint.json")
	_, _, err := executeCommand("analysis", "create", "--resume", "--checkpoint", missing)
	if err == nil || !strings.Contains(err.Error(), "--resume requires a usable incomplete checkpoint") {
		t.Fatalf("error = %v, want checkpoint read error", err)
	}
	if strings.Contains(err.Error(), "oauth client id") || strings.Contains(err.Error(), "client secret") || strings.Contains(err.Error(), "token URL") {
		t.Fatalf("resume required credentials before reading checkpoint: %v", err)
	}
}

func TestAnalysisCreateHelpDocumentsAutoResumeAndNew(t *testing.T) {
	stdout, stderr, err := executeCommand("analysis", "create", "--help")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	for _, want := range []string{"--resume", "usable incomplete checkpoint", "--new", "fresh data-analysis", "--type", "required for fresh creation"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "default \"other\"") {
		t.Fatalf("help documents old type default:\n%s", stdout)
	}
}

func TestAnalysisCreateResumeAndNewConflict(t *testing.T) {
	dir := t.TempDir()
	checkpoint := filepath.Join(dir, "checkpoint.json")
	_, _, err := executeCommand("analysis", "create", "--resume", "--new", "--checkpoint", checkpoint)
	if err == nil || !strings.Contains(err.Error(), "--resume and --new cannot be used together") {
		t.Fatalf("error = %v, want flag conflict", err)
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
