package skills

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	DefaultLocalPath     = "./skills"
	DefaultContainerPath = "/mnt/skills"
)

type Client interface {
	UploadFile(ctx context.Context, localPath string, remotePath string) error
	DownloadFile(ctx context.Context, remotePath string, localPath string) error
	ExecuteCommand(ctx context.Context, command string) (string, error)
}

type Store interface {
	Load(ctx context.Context, client Client, containerPath string) error
	Save(ctx context.Context, client Client, containerPath string) error
}

type LocalStore struct {
	root        string
	cleanRemote bool
}

type LocalStoreOption func(*LocalStore)

func WithCleanRemote(clean bool) LocalStoreOption {
	return func(store *LocalStore) {
		store.cleanRemote = clean
	}
}

func NewLocalStore(root string, opts ...LocalStoreOption) *LocalStore {
	if strings.TrimSpace(root) == "" {
		root = DefaultLocalPath
	}
	store := &LocalStore{root: root}
	for _, opt := range opts {
		if opt != nil {
			opt(store)
		}
	}
	return store
}

func (s *LocalStore) Load(ctx context.Context, client Client, containerPath string) error {
	containerPath = normalizeContainerPath(containerPath)
	root, err := filepath.Abs(s.root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	createdDirs := make(map[string]struct{})
	localFiles := make(map[string]struct{})
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		remotePath := path.Join(containerPath, filepath.ToSlash(rel))
		localFiles[remotePath] = struct{}{}
		remoteDir := path.Dir(remotePath)
		if _, ok := createdDirs[remoteDir]; !ok {
			if _, err := client.ExecuteCommand(ctx, fmt.Sprintf("mkdir -p %s", shellQuote(remoteDir))); err != nil {
				return err
			}
			createdDirs[remoteDir] = struct{}{}
		}
		return client.UploadFile(ctx, p, remotePath)
	})
	if err != nil {
		return err
	}
	if !s.cleanRemote {
		return nil
	}
	return cleanRemoteExtras(ctx, client, containerPath, localFiles)
}

func (s *LocalStore) Save(ctx context.Context, client Client, containerPath string) error {
	containerPath = normalizeContainerPath(containerPath)
	root, err := filepath.Abs(s.root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	cmd := fmt.Sprintf("find %s -type f 2>/dev/null", shellQuote(containerPath))
	output, err := client.ExecuteCommand(ctx, cmd)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		remotePath := strings.TrimSpace(line)
		if remotePath == "" {
			continue
		}
		rel := strings.TrimPrefix(remotePath, containerPath)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || strings.HasPrefix(rel, "..") {
			continue
		}
		localPath := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return err
		}
		if err := client.DownloadFile(ctx, remotePath, localPath); err != nil {
			return err
		}
	}
	return nil
}

func cleanRemoteExtras(ctx context.Context, client Client, containerPath string, localFiles map[string]struct{}) error {
	cmd := fmt.Sprintf("find %s -type f 2>/dev/null", shellQuote(containerPath))
	output, err := client.ExecuteCommand(ctx, cmd)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		remotePath := strings.TrimSpace(line)
		if remotePath == "" {
			continue
		}
		if _, ok := localFiles[remotePath]; ok {
			continue
		}
		if _, err := client.ExecuteCommand(ctx, fmt.Sprintf("rm -f %s", shellQuote(remotePath))); err != nil {
			return err
		}
	}
	_, err = client.ExecuteCommand(ctx, fmt.Sprintf("find %s -type d -empty -delete 2>/dev/null", shellQuote(containerPath)))
	return err
}

func normalizeContainerPath(containerPath string) string {
	containerPath = strings.TrimSpace(containerPath)
	if containerPath == "" {
		containerPath = DefaultContainerPath
	}
	containerPath = path.Clean(containerPath)
	if containerPath == "." {
		containerPath = DefaultContainerPath
	}
	return containerPath
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
