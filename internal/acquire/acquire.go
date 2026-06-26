package acquire

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
	PythonVersion            = "3.12.8"
	GoEmbedPythonVersion     = "v0.0.0-3.12.8-20241219-1"
	AcquireVersion           = "3.22"
	AcquirePackageSpec       = "acquire==" + AcquireVersion
	embeddedSitePackagesRoot = "embedded/site-packages"
)

//go:embed all:embedded/site-packages
var embeddedAcquireSitePackages embed.FS

type Options struct {
	Home   string
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
}

func DefaultHomeDir() (string, error) {
	if home := strings.TrimSpace(os.Getenv("DA_TOOLKIT_ACQUIRE_HOME")); home != "" {
		return home, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current working directory: %w", err)
	}
	return filepath.Join(cwd, ".da-toolkit", "acquire"), nil
}

func RuntimeDir(home string) string {
	return filepath.Join(home, "python-runtime")
}

func SitePackagesDir(home string) string {
	return filepath.Join(home, "site-packages", "acquire-"+AcquireVersion)
}

func Run(ctx context.Context, opts Options) error {
	if !supportedPlatform() {
		return fmt.Errorf("embedded acquire is currently scoped to linux-arm64, linux-amd64, and windows-amd64 only; current platform is %s", runtimePlatform())
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	debug := os.Getenv("DA_TOOLKIT_ACQUIRE_DEBUG") == "1"

	home := strings.TrimSpace(opts.Home)
	if home == "" {
		var err error
		home, err = DefaultHomeDir()
		if err != nil {
			return err
		}
	}
	runtimeDir := RuntimeDir(home)
	sitePackagesDir := SitePackagesDir(home)
	debugf(stderr, debug, "acquire-home=%s\n", home)
	debugf(stderr, debug, "runtime-dir=%s\n", runtimeDir)
	debugf(stderr, debug, "site-packages=%s\n", sitePackagesDir)

	ep, err := python.NewEmbeddedPythonWithTmpDir(runtimeDir, true)
	if err != nil {
		return fmt.Errorf("prepare embedded Python runtime: %w", err)
	}
	debugf(stderr, debug, "extracted-runtime=%s\n", ep.GetExtractedPath())

	if err := ensureAcquireSitePackages(sitePackagesDir, debug, stderr); err != nil {
		return err
	}
	ep.AddPythonPath(sitePackagesDir)

	cmd, err := ep.PythonCmd(acquirePythonInvocation(opts.Args)...)
	if err != nil {
		return fmt.Errorf("prepare acquire subprocess: %w", err)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return runCommand(ctx, cmd, "invoke acquire")
}

func ensureAcquireSitePackages(sitePackagesDir string, debug bool, diagnostics io.Writer) error {
	if err := verifyAcquireSitePackages(sitePackagesDir); err == nil {
		debugf(diagnostics, debug, "site-packages-status=reuse\n")
		return nil
	}
	debugf(diagnostics, debug, "site-packages-status=extract\n")
	if err := bootstrapAcquire(sitePackagesDir); err != nil {
		return err
	}
	if err := verifyAcquireSitePackages(sitePackagesDir); err != nil {
		return fmt.Errorf("verify extracted acquire site-packages: %w", err)
	}
	return nil
}

func verifyAcquireSitePackages(sitePackagesDir string) error {
	for _, path := range []string{
		filepath.Join(sitePackagesDir, "acquire", "acquire.py"),
		filepath.Join(sitePackagesDir, "acquire-"+AcquireVersion+".dist-info", "METADATA"),
		filepath.Join(sitePackagesDir, "dissect", "target", "__init__.py"),
	} {
		if _, err := os.Stat(path); err != nil {
			return err
		}
	}
	return nil
}

func debugf(w io.Writer, enabled bool, format string, args ...any) {
	if enabled && w != nil {
		fmt.Fprintf(w, format, args...)
	}
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
	root, err := fs.Sub(embeddedAcquireSitePackages, embeddedSitePackagesPath())
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

func supportedPlatform() bool {
	switch runtimePlatform() {
	case "linux-arm64", "linux-amd64", "windows-amd64":
		return true
	default:
		return false
	}
}

func runtimePlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func embeddedSitePackagesPath() string {
	return embeddedSitePackagesRoot + "/" + runtimePlatform()
}

func acquirePythonInvocation(args []string) []string {
	py := "import sys; from acquire.acquire import main; sys.argv = ['acquire'] + sys.argv[1:]; sys.exit(main())"
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

func ValidatePrivilegeNote() error {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return errors.New("live acquire privilege requirements are documented for linux and windows only")
	}
	return nil
}
