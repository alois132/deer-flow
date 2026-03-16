package sandbox

import (
	"context"
	"errors"
)

var (
	ErrNoData = errors.New("data is validate")
)

type Sandbox interface {
	GetID(ctx context.Context) string
	ExecuteCommand(ctx context.Context, command string) (string, error)
	ReadFile(ctx context.Context, path string) (string, error)
	ListDir(ctx context.Context, path string, maxDepth int) ([]string, error)
	WriteFile(ctx context.Context, path string, content string, append bool) error
	UpdateFile(ctx context.Context, path string, content string) error
	UploadFile(ctx context.Context, localPath string, remotePath string) error
	DownloadFile(ctx context.Context, remotePath string, localPath string) error
}
