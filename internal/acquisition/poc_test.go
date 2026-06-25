package acquisition

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultRuntimeAndSitePackagesDirsAreAppOwned(t *testing.T) {
	tmp := t.TempDir()
	if got := DefaultRuntimeDir(tmp); got != filepath.Join(tmp, "da-toolkit-acquisition-poc", "python-runtime") {
		t.Fatalf("DefaultRuntimeDir() = %q", got)
	}
	if got := DefaultSitePackagesDir(tmp); got != filepath.Join(tmp, "da-toolkit-acquisition-poc", "site-packages", "acquire-"+AcquireVersion) {
		t.Fatalf("DefaultSitePackagesDir() = %q", got)
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
