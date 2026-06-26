package vol

import (
	"context"
	"embed"
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
	VolatilityVersion        = "2.26.2"
	VolatilityPackageSpec    = "volatility3[full]==" + VolatilityVersion
	embeddedSitePackagesRoot = "embedded/site-packages"
)

//go:embed all:embedded/site-packages
var embeddedVolSitePackages embed.FS

type Options struct {
	Home   string
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
}

func DefaultHomeDir() (string, error) {
	if home := strings.TrimSpace(os.Getenv("DA_TOOLKIT_VOL_HOME")); home != "" {
		return home, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current working directory: %w", err)
	}
	return filepath.Join(cwd, ".da-toolkit", "vol"), nil
}

func RuntimeDir(home string) string {
	return filepath.Join(home, "python-runtime")
}

func SitePackagesDir(home string) string {
	return filepath.Join(home, "site-packages", "volatility3-"+VolatilityVersion)
}

func Run(ctx context.Context, opts Options) error {
	if !supportedPlatform() {
		return fmt.Errorf("embedded volatility3 is currently scoped to linux-arm64, linux-amd64, and windows-amd64 only; current platform is %s", runtimePlatform())
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	debug := os.Getenv("DA_TOOLKIT_VOL_DEBUG") == "1"

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
	debugf(stderr, debug, "vol-home=%s\n", home)
	debugf(stderr, debug, "runtime-dir=%s\n", runtimeDir)
	debugf(stderr, debug, "site-packages=%s\n", sitePackagesDir)

	ep, err := python.NewEmbeddedPythonWithTmpDir(runtimeDir, true)
	if err != nil {
		return fmt.Errorf("prepare embedded Python runtime: %w", err)
	}
	debugf(stderr, debug, "extracted-runtime=%s\n", ep.GetExtractedPath())

	if err := ensureVolSitePackages(sitePackagesDir, debug, stderr); err != nil {
		return err
	}
	ep.AddPythonPath(sitePackagesDir)

	cmd, err := ep.PythonCmd(volPythonInvocation(opts.Args)...)
	if err != nil {
		return fmt.Errorf("prepare volatility3 subprocess: %w", err)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return runCommand(ctx, cmd, "invoke volatility3")
}

func ensureVolSitePackages(sitePackagesDir string, debug bool, diagnostics io.Writer) error {
	if err := verifyVolSitePackages(sitePackagesDir); err == nil {
		debugf(diagnostics, debug, "site-packages-status=reuse\n")
		return nil
	}
	if err := verifyEmbeddedVolSitePackages(); err != nil {
		return err
	}
	debugf(diagnostics, debug, "site-packages-status=extract\n")
	if err := bootstrapVol(sitePackagesDir); err != nil {
		return err
	}
	if err := verifyVolSitePackages(sitePackagesDir); err != nil {
		return fmt.Errorf("verify extracted volatility3 site-packages: %w", err)
	}
	return nil
}

func verifyEmbeddedVolSitePackages() error {
	for _, path := range []string{
		filepath.ToSlash(filepath.Join(embeddedSitePackagesPath(), "volatility3", "cli", "__init__.py")),
		filepath.ToSlash(filepath.Join(embeddedSitePackagesPath(), "volatility3-"+VolatilityVersion+".dist-info", "METADATA")),
	} {
		if _, err := fs.Stat(embeddedVolSitePackages, path); err != nil {
			return fmt.Errorf("embedded volatility3 site-packages payload is not prepared for %s; run scripts/build-binary.sh to build a binary with embedded payloads, or set DA_TOOLKIT_KEEP_EMBEDDED_PAYLOADS=1 while running that script before using go run from source: %w", runtimePlatform(), err)
		}
	}
	return nil
}

func verifyVolSitePackages(sitePackagesDir string) error {
	for _, path := range []string{
		filepath.Join(sitePackagesDir, "volatility3", "cli", "__init__.py"),
		filepath.Join(sitePackagesDir, "volatility3-"+VolatilityVersion+".dist-info", "METADATA"),
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

func bootstrapVol(sitePackagesDir string) error {
	if err := os.RemoveAll(sitePackagesDir); err != nil {
		return fmt.Errorf("reset volatility3 site-packages: %w", err)
	}
	if err := extractEmbeddedSitePackages(sitePackagesDir); err != nil {
		return fmt.Errorf("extract embedded volatility3 site-packages: %w", err)
	}
	return nil
}

func extractEmbeddedSitePackages(sitePackagesDir string) error {
	root, err := fs.Sub(embeddedVolSitePackages, embeddedSitePackagesPath())
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

func volPythonInvocation(args []string) []string {
	py := "import sys; sys.argv = ['vol'] + sys.argv[1:]; from volatility3.cli import main; sys.exit(main())"
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
