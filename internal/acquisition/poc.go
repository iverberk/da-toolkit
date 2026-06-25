package acquisition

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kluctl/go-embed-python/python"
)

const (
	PythonVersion        = "3.12.8"
	GoEmbedPythonVersion = "v0.0.0-3.12.8-20241219-1"
	AcquireVersion       = "3.22"
	AcquirePackageSpec   = "acquire==" + AcquireVersion
	embeddedSitePackages = "embedded/site-packages/linux-arm64"
)

//go:embed all:embedded/site-packages/linux-arm64
var embeddedAcquireSitePackages embed.FS

type Options struct {
	RuntimeDir       string
	SitePackagesDir  string
	BootstrapAcquire bool
	CleanupRuntime   bool
	AcquireArgs      []string
	Stdout           io.Writer
	Stderr           io.Writer
}

func DefaultRuntimeDir(tempDir string) string {
	return filepath.Join(tempDir, "da-toolkit-acquisition-poc", "python-runtime")
}

func DefaultSitePackagesDir(tempDir string) string {
	return filepath.Join(tempDir, "da-toolkit-acquisition-poc", "site-packages", "acquire-"+AcquireVersion)
}

func RunPoC(ctx context.Context, opts Options) error {
	if runtime.GOOS != "linux" || runtime.GOARCH != "arm64" {
		return fmt.Errorf("embedded acquire PoC evidence is currently scoped to linux-arm64 only; current platform is %s-%s", runtime.GOOS, runtime.GOARCH)
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	runtimeDir := firstNonEmpty(opts.RuntimeDir, DefaultRuntimeDir(os.TempDir()))
	sitePackagesDir := firstNonEmpty(opts.SitePackagesDir, DefaultSitePackagesDir(os.TempDir()))

	ep, err := python.NewEmbeddedPythonWithTmpDir(runtimeDir, true)
	if err != nil {
		return fmt.Errorf("prepare embedded Python runtime: %w", err)
	}
	if opts.CleanupRuntime {
		defer ep.Cleanup()
	}

	if opts.BootstrapAcquire {
		if err := bootstrapAcquire(sitePackagesDir); err != nil {
			return err
		}
	}

	if _, err := os.Stat(sitePackagesDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("pinned acquire site-packages not found at %s (run with --bootstrap or provide --site-packages)", sitePackagesDir)
		}
		return fmt.Errorf("check acquire site-packages: %w", err)
	}
	ep.AddPythonPath(sitePackagesDir)

	args := opts.AcquireArgs
	if len(args) == 0 {
		args = []string{"--help"}
	}

	fmt.Fprintf(stdout, "embedded-python=%s\n", GoEmbedPythonVersion)
	fmt.Fprintf(stdout, "python=%s\n", PythonVersion)
	fmt.Fprintf(stdout, "acquire=%s\n", AcquirePackageSpec)
	fmt.Fprintf(stdout, "platform=%s-%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(stdout, "runtime-dir=%s\n", ep.GetExtractedPath())
	fmt.Fprintf(stdout, "site-packages=%s\n", sitePackagesDir)
	fmt.Fprintf(stdout, "cleanup-runtime=%t\n", opts.CleanupRuntime)
	fmt.Fprintln(stdout, "invoke=python -c from acquire.acquire import main; main() -- "+strings.Join(args, " "))

	cmd, err := ep.PythonCmd(acquirePythonInvocation(args)...)
	if err != nil {
		return fmt.Errorf("prepare acquire subprocess: %w", err)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return runCommand(ctx, cmd, "invoke acquire")
}

func bootstrapAcquire(sitePackagesDir string) error {
	if err := os.RemoveAll(sitePackagesDir); err != nil {
		return fmt.Errorf("reset acquire site-packages: %w", err)
	}
	if err := extractEmbeddedSitePackages(sitePackagesDir); err != nil {
		return fmt.Errorf("extract embedded acquire site-packages: %w", err)
	}
	return nil
}

func extractEmbeddedSitePackages(sitePackagesDir string) error {
	root, err := fs.Sub(embeddedAcquireSitePackages, embeddedSitePackages)
	if err != nil {
		return fmt.Errorf("open embedded site-packages: %w", err)
	}
	return fs.WalkDir(root, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(sitePackagesDir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		return os.WriteFile(target, data, mode)
	})
}

func acquirePythonInvocation(args []string) []string {
	py := "import sys; from acquire.acquire import main; sys.argv = ['acquire'] + sys.argv[1:]; main()"
	return append([]string{"-c", py}, args...)
}

func runCommand(ctx context.Context, cmd *exec.Cmd, action string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-done
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s: %w", action, err)
		}
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func ValidatePrivilegeNote() error {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return errors.New("live acquisition privilege requirements are documented for linux and windows only")
	}
	return nil
}
