package acquire

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
	if got := SitePackagesDir(tmp); got != filepath.Join(tmp, "site-packages", "acquire-"+AcquireVersion) {
		t.Fatalf("SitePackagesDir() = %q", got)
	}
}

func TestDefaultHomeDirUsesCWDRelativeHiddenDirectory(t *testing.T) {
	t.Setenv("DA_TOOLKIT_ACQUIRE_HOME", "")
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
	if want := filepath.Join(dir, ".da-toolkit", "acquire"); got != want {
		t.Fatalf("DefaultHomeDir() = %q, want %q", got, want)
	}
}

func TestDefaultHomeDirUsesEnvironmentOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "acquire-home")
	t.Setenv("DA_TOOLKIT_ACQUIRE_HOME", want)
	got, err := DefaultHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("DefaultHomeDir() = %q, want %q", got, want)
	}
}

func TestAcquirePythonInvocationPreservesArguments(t *testing.T) {
	got := acquirePythonInvocation([]string{"--help", "--output", "case.ad"})
	wantSuffix := []string{"--help", "--output", "case.ad"}
	if len(got) < 2 || got[0] != "-c" {
		t.Fatalf("invocation prefix = %v, want python -c", got)
	}
	if !reflect.DeepEqual(got[len(got)-len(wantSuffix):], wantSuffix) {
		t.Fatalf("invocation args = %v, want suffix %v", got, wantSuffix)
	}
}

func TestBootstrapAcquireExtractsEmbeddedSitePackages(t *testing.T) {
	requireEmbeddedAcquireSitePackages(t)

	sitePackagesDir := filepath.Join(t.TempDir(), "site-packages")
	if err := bootstrapAcquire(sitePackagesDir); err != nil {
		t.Fatalf("bootstrapAcquire() error = %v", err)
	}
	for _, want := range []string{
		filepath.Join(sitePackagesDir, "acquire", "acquire.py"),
		filepath.Join(sitePackagesDir, "acquire-"+AcquireVersion+".dist-info", "METADATA"),
		filepath.Join(sitePackagesDir, "dissect", "target", "__init__.py"),
		filepath.Join(sitePackagesDir, "dissect", "target"),
	} {
		if _, err := os.Stat(want); err != nil {
			t.Fatalf("embedded site-packages missing %s: %v", want, err)
		}
	}
}

func TestEnsureAcquireSitePackagesReusesVerifiedExtraction(t *testing.T) {
	requireEmbeddedAcquireSitePackages(t)

	sitePackagesDir := filepath.Join(t.TempDir(), "site-packages")
	if err := ensureAcquireSitePackages(sitePackagesDir, false, nil); err != nil {
		t.Fatalf("ensureAcquireSitePackages() extract error = %v", err)
	}
	marker := filepath.Join(sitePackagesDir, "reuse-marker")
	if err := os.WriteFile(marker, []byte("preserved"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureAcquireSitePackages(sitePackagesDir, false, nil); err != nil {
		t.Fatalf("ensureAcquireSitePackages() reuse error = %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("verified site-packages were not reused: %v", err)
	}
}

func TestEnsureAcquireSitePackagesFailsClearlyWhenEmbeddedPayloadMissing(t *testing.T) {
	if err := verifyEmbeddedAcquireSitePackages(); err == nil {
		t.Skip("full embedded acquire site-packages are present")
	}

	sitePackagesDir := filepath.Join(t.TempDir(), "site-packages")
	err := ensureAcquireSitePackages(sitePackagesDir, false, nil)
	if err == nil {
		t.Fatal("ensureAcquireSitePackages() error = nil, want missing payload error")
	}
	for _, want := range []string{
		"embedded acquire site-packages payload is not prepared",
		"go run from source",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ensureAcquireSitePackages() error = %q, want to contain %q", err.Error(), want)
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

func requireEmbeddedAcquireSitePackages(t *testing.T) {
	t.Helper()
	if _, err := fs.Stat(embeddedAcquireSitePackages, embeddedSitePackagesPath()+"/acquire/acquire.py"); err != nil {
		t.Skip("full embedded acquire site-packages are generated only during binary builds")
	}
}
