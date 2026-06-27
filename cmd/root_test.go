package cmd

import (
	"bytes"
	"context"
	"da-toolkit/internal/acquire"
	"da-toolkit/internal/vol"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func executeCommand(args ...string) (string, string, error) {
	resetAnalysisCreateCommandFlags()
	resetAnalysisUploadCommandFlags()
	resetAnalysisCommandFlags()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs(nil)
	resetAcquireCommandFlags()
	resetVolCommandFlags()
	resetAnalysisCreateCommandFlags()
	resetAnalysisUploadCommandFlags()
	resetAnalysisCommandFlags()
	return stdout.String(), stderr.String(), err
}

func resetAcquireCommandFlags() {
	runAcquire = acquire.Run
}

func resetVolCommandFlags() {
	runVol = vol.Run
}

func resetAnalysisCreateCommandFlags() {
	analysisCreateFlags = struct {
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
		deferred       bool
		output         string
	}{}
	analysisCreateCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
	analysisCreateFlags.zipPasswords = nil
	analysisCreateFlags.requested = nil
	analysisCreateFlags.diskKeys = nil
}

func resetAnalysisUploadCommandFlags() {
	analysisUploadFlags = struct {
		filename       string
		aesKey         string
		sasURL         string
		handoff        string
		workspaceDir   string
		checkpointPath string
		yes            bool
	}{}
	analysisUploadCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
}

func resetAnalysisCommandFlags() {
	analysisFlags = struct {
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
		yes           bool
		output        string
		overwrite     bool
		wait          bool
		download      bool
	}{}
	for _, command := range []*cobra.Command{analysisCmd, analysisListCmd, analysisReadCmd, analysisDeleteCmd, analysisArchiveCmd, analysisFileCmd, analysisUploadCmd} {
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

	for _, want := range []string{"Create, inspect, and retrieve data-analyses, and run acquire workflows.", "Available Commands:", "analysis", "acquire", "vol"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help missing %q:\n%s", want, stdout)
		}
	}
	if commandNamed(rootCmd, "acquisition") || strings.Contains(stdout, "  acquisition ") {
		t.Fatalf("help includes old acquire command name:\n%s", stdout)
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
	for _, name := range []string{"upload", "result", "gui", "acquisition", "volshell"} {
		t.Run(name, func(t *testing.T) {
			_, _, err := executeCommand(name)
			if err == nil || !strings.Contains(err.Error(), "unknown command") {
				t.Fatalf("error = %v, want unknown command", err)
			}
		})
	}
}

func TestAcquireForwardsHelpToEmbeddedAcquire(t *testing.T) {
	var gotArgs []string
	runAcquire = func(_ context.Context, opts acquire.Options) error {
		gotArgs = append([]string(nil), opts.Args...)
		_, _ = opts.Stdout.Write([]byte("Dissect acquire help\n"))
		return nil
	}
	stdout, stderr, err := executeCommand("acquire", "--help")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	if !reflect.DeepEqual(gotArgs, []string{"--help"}) {
		t.Fatalf("forwarded args = %v, want --help", gotArgs)
	}
	if !strings.Contains(stdout, "Dissect acquire help") {
		t.Fatalf("stdout missing forwarded help:\n%s", stdout)
	}
}

func TestAcquireForwardsArbitraryArgsAndHidesPocSurface(t *testing.T) {
	var gotArgs []string
	runAcquire = func(_ context.Context, opts acquire.Options) error {
		gotArgs = append([]string(nil), opts.Args...)
		return nil
	}
	_, _, err := executeCommand("acquire", "--output", "case.ad", "poc")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if want := []string{"--output", "case.ad", "poc"}; !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("forwarded args = %v, want %v", gotArgs, want)
	}
	if commandNamed(acquireCmd, "poc") {
		t.Fatal("acquire poc is still registered as a public subcommand")
	}
	for _, removed := range []string{"runtime-dir", "site-packages", "bootstrap", "cleanup", "acquire-arg"} {
		if acquireCmd.Flags().Lookup(removed) != nil || acquireCmd.PersistentFlags().Lookup(removed) != nil {
			t.Fatalf("PoC-only flag %q is still registered", removed)
		}
	}
}

func TestVolForwardsHelpToEmbeddedVolatility(t *testing.T) {
	var gotArgs []string
	runVol = func(_ context.Context, opts vol.Options) error {
		gotArgs = append([]string(nil), opts.Args...)
		_, _ = opts.Stdout.Write([]byte("Volatility 3 Framework help\n"))
		return nil
	}
	stdout, stderr, err := executeCommand("vol", "--help")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	if !reflect.DeepEqual(gotArgs, []string{"--help"}) {
		t.Fatalf("forwarded args = %v, want --help", gotArgs)
	}
	if !strings.Contains(stdout, "Volatility 3 Framework help") {
		t.Fatalf("stdout missing forwarded help:\n%s", stdout)
	}
}

func TestVolForwardsArbitraryArgsAndDoesNotExposeVolshell(t *testing.T) {
	var gotArgs []string
	runVol = func(_ context.Context, opts vol.Options) error {
		gotArgs = append([]string(nil), opts.Args...)
		return nil
	}
	_, _, err := executeCommand("vol", "-f", "memory.raw", "windows.info")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if want := []string{"-f", "memory.raw", "windows.info"}; !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("forwarded args = %v, want %v", gotArgs, want)
	}
	if commandNamed(volCmd, "volshell") || commandNamed(rootCmd, "volshell") {
		t.Fatal("volshell is registered as a public command")
	}
	for _, removed := range []string{"runtime-dir", "site-packages", "bootstrap", "cleanup", "vol-arg"} {
		if volCmd.Flags().Lookup(removed) != nil || volCmd.PersistentFlags().Lookup(removed) != nil {
			t.Fatalf("wrapper-specific flag %q is registered", removed)
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

func TestDefaultAnalysisWorkspaceIsStableForSameFiles(t *testing.T) {
	rel := filepath.Join(".", "testdata", "evidence.txt")
	abs, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}

	first := defaultAnalysisWorkspace([]string{abs})
	second := defaultAnalysisWorkspace([]string{abs})
	fromRelative := defaultAnalysisWorkspace([]string{rel})

	if first != second || first != fromRelative {
		t.Fatalf("default workspace is not stable: %q %q %q", first, second, fromRelative)
	}
	if !strings.HasPrefix(first, filepath.Join(".da-toolkit", "analysis-")) || len(strings.TrimPrefix(filepath.Base(first), "analysis-")) != 16 {
		t.Fatalf("default workspace = %q, want static analysis identifier", first)
	}
}

func TestUnknownCommandUsesUserFacingErrorPattern(t *testing.T) {
	_, _, err := executeCommand("missing")
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %v, want unknown command", err)
	}
}
