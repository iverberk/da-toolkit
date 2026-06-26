package vol

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultRuntimeAndSitePackagesDirsAreAppOwned(t *testing.T) {
	tmp := t.TempDir()
	if got := RuntimeDir(tmp); got != filepath.Join(tmp, "python-runtime") {
		t.Fatalf("RuntimeDir() = %q", got)
	}
	if got := SitePackagesDir(tmp); got != filepath.Join(tmp, "site-packages", "volatility3-"+VolatilityVersion) {
		t.Fatalf("SitePackagesDir() = %q", got)
	}
}

func TestDefaultHomeDirUsesCWDRelativeHiddenDirectory(t *testing.T) {
	t.Setenv("DA_TOOLKIT_VOL_HOME", "")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	got, err := DefaultHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, ".da-toolkit", "vol"); got != want {
		t.Fatalf("DefaultHomeDir() = %q, want %q", got, want)
	}
}

func TestDefaultHomeDirUsesEnvironmentOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "vol-home")
	t.Setenv("DA_TOOLKIT_VOL_HOME", want)
	got, err := DefaultHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("DefaultHomeDir() = %q, want %q", got, want)
	}
}

func TestVolPythonInvocationPreservesArgumentsAndConsoleScriptName(t *testing.T) {
	got := volPythonInvocation([]string{"--help", "-f", "memory.raw", "windows.info"})
	wantSuffix := []string{"--help", "-f", "memory.raw", "windows.info"}
	if len(got) < 2 || got[0] != "-c" {
		t.Fatalf("invocation prefix = %v, want python -c", got)
	}
	if !strings.Contains(got[1], "from volatility3.cli import main") || !strings.Contains(got[1], "sys.argv = ['vol']") {
		t.Fatalf("invocation script = %q, want volatility3 cli main with argv[0] vol", got[1])
	}
	if !reflect.DeepEqual(got[len(got)-len(wantSuffix):], wantSuffix) {
		t.Fatalf("invocation args = %v, want suffix %v", got, wantSuffix)
	}
}

func TestBootstrapVolExtractsEmbeddedSitePackages(t *testing.T) {
	requireEmbeddedVolSitePackages(t)

	sitePackagesDir := filepath.Join(t.TempDir(), "site-packages")
	if err := bootstrapVol(sitePackagesDir); err != nil {
		t.Fatalf("bootstrapVol() error = %v", err)
	}
	for _, want := range []string{
		filepath.Join(sitePackagesDir, "volatility3", "cli", "__init__.py"),
		filepath.Join(sitePackagesDir, "volatility3-"+VolatilityVersion+".dist-info", "METADATA"),
		filepath.Join(sitePackagesDir, "volatility3", "cli"),
	} {
		if _, err := os.Stat(want); err != nil {
			t.Fatalf("embedded site-packages missing %s: %v", want, err)
		}
	}
}

func TestEnsureVolSitePackagesReusesVerifiedExtraction(t *testing.T) {
	requireEmbeddedVolSitePackages(t)

	sitePackagesDir := filepath.Join(t.TempDir(), "site-packages")
	if err := ensureVolSitePackages(sitePackagesDir, false, nil); err != nil {
		t.Fatalf("ensureVolSitePackages() extract error = %v", err)
	}
	marker := filepath.Join(sitePackagesDir, "reuse-marker")
	if err := os.WriteFile(marker, []byte("preserved"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureVolSitePackages(sitePackagesDir, false, nil); err != nil {
		t.Fatalf("ensureVolSitePackages() reuse error = %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("verified site-packages were not reused: %v", err)
	}
}

func TestEnsureVolSitePackagesFailsClearlyWhenEmbeddedPayloadMissing(t *testing.T) {
	if err := verifyEmbeddedVolSitePackages(); err == nil {
		t.Skip("full embedded volatility3 site-packages are present")
	}

	sitePackagesDir := filepath.Join(t.TempDir(), "site-packages")
	err := ensureVolSitePackages(sitePackagesDir, false, nil)
	if err == nil {
		t.Fatal("ensureVolSitePackages() error = nil, want missing payload error")
	}
	for _, want := range []string{
		"embedded volatility3 site-packages payload is not prepared",
		"go run from source",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ensureVolSitePackages() error = %q, want to contain %q", err.Error(), want)
		}
	}
	if _, statErr := os.Stat(filepath.Join(sitePackagesDir, "placeholder.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("placeholder payload was extracted before readiness check: %v", statErr)
	}
}

func TestDebugfWritesOnlyWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	debugf(&buf, false, "runtime-dir=%s\n", "hidden")
	if buf.String() != "" {
		t.Fatalf("debug output when disabled = %q", buf.String())
	}
	debugf(&buf, true, "runtime-dir=%s\n", "shown")
	if got := buf.String(); got != "runtime-dir=shown\n" {
		t.Fatalf("debug output = %q", got)
	}
}

func TestEmbeddedSitePackagesPathUsesRuntimePlatform(t *testing.T) {
	want := embeddedSitePackagesRoot + "/" + runtimePlatform()
	if got := embeddedSitePackagesPath(); got != want {
		t.Fatalf("embeddedSitePackagesPath() = %q, want %q", got, want)
	}
}

func requireEmbeddedVolSitePackages(t *testing.T) {
	t.Helper()
	if _, err := fs.Stat(embeddedVolSitePackages, embeddedSitePackagesPath()+"/volatility3/cli/__init__.py"); err != nil {
		t.Skip("full embedded volatility3 site-packages are generated only during binary builds")
	}
}
