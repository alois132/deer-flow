package aio

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// VirtualPathPrefix is the mount point inside the sandbox
	VirtualPathPrefix = "/mnt/user-data"
)

// Safe thread ID validation regex
var safeThreadIDRe = regexp.MustCompile(`^[A-Za-z0-9_\-]+$`)

type Paths struct {
	baseDir string
}

func NewPaths(baseDir string) (*Paths, error) {
	if baseDir == "" {
		baseDir = os.Getenv("DEER_FLOW_HOME")
		if baseDir == "" {
			baseDir = "."
		}
	}
	baseDir, err := mustAbs(baseDir)
	if err != nil {
		return nil, err
	}
	return &Paths{
		baseDir: baseDir,
	}, nil
}

func (p *Paths) GetBaseDir() string {
	return p.baseDir
}

func (p *Paths) MemoryFile() string {
	return filepath.Join(p.GetBaseDir(), "memory.json")
}

func (p *Paths) ThreadDir(threadID string) (string, error) {
	if !safeThreadIDRe.MatchString(threadID) {
		return "", fmt.Errorf("invalid thread ID: %s", threadID)
	}
	return filepath.Join(p.GetBaseDir(), "threads", threadID), nil
}

func (p *Paths) SandboxWorkDir(threadID string) (string, error) {
	dir, err := p.ThreadDir(threadID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "user-data", "workspace"), nil
}

func (p *Paths) SandboxUploadsDir(threadID string) (string, error) {
	dir, err := p.ThreadDir(threadID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "user-data", "uploads"), nil
}

func (p *Paths) SandboxOutputsDir(threadID string) (string, error) {
	dir, err := p.ThreadDir(threadID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "user-data", "outputs"), nil
}

func (p *Paths) SandboxUserDataDir(threadID string) (string, error) {
	dir, err := p.ThreadDir(threadID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "user-data"), nil
}

func (p *Paths) EnsureThreadDirs(threadID string) error {
	var dirs []string
	dir, err := p.SandboxWorkDir(threadID)
	if err != nil {
		return err
	}
	dirs = append(dirs, dir)
	dir, err = p.SandboxUploadsDir(threadID)
	if err != nil {
		return err
	}
	dirs = append(dirs, dir)
	dir, err = p.SandboxOutputsDir(threadID)
	if err != nil {
		return err
	}
	dirs = append(dirs, dir)

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (p *Paths) ResolveVirtualPath(threadID, virtualPath string) (string, error) {
	stripped := strings.TrimLeft(virtualPath, "/")
	prefix := strings.TrimLeft(VirtualPathPrefix, "/")

	if stripped != prefix &&
		!strings.HasPrefix(stripped, prefix+"/") {
		return "", fmt.Errorf("invalid virtual path: %s", virtualPath)
	}
	relative := strings.TrimLeft(stripped[len(prefix):], "/")
	base, err := p.SandboxUserDataDir(threadID)
	if err != nil {
		return "", err
	}
	abs, err := mustAbs(base)
	if err != nil {
		return "", err
	}
	actual, err := mustAbs(filepath.Join(abs, relative))
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(actual, abs+string(os.PathSeparator)) &&
		actual != abs {
		return "", fmt.Errorf("invalid virtual path: %s", virtualPath)
	}
	return actual, nil
}
func mustAbs(path string) (string, error) {
	return filepath.Abs(path)
}
