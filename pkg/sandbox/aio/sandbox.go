package aio

import (
	"context"
	"encoding/base64"
	"fmt"
	api "github.com/agent-infra/sandbox-sdk-go"
	"github.com/agent-infra/sandbox-sdk-go/client"
	"github.com/agent-infra/sandbox-sdk-go/option"
	"github.com/alois132/deer-flow/pkg/sandbox"
	"github.com/alois132/deer-flow/utils/safe"
	"strings"
)

type Sandbox struct {
	id        string
	baseUrl   string
	homeDir   string
	aioClient *client.Client
}

func NewSandbox(id string, baseUrl string, homeDir string) *Sandbox {
	s := &Sandbox{
		id:      id,
		baseUrl: baseUrl,
		homeDir: homeDir,
	}
	s.aioClient = client.NewClient(option.WithBaseURL(baseUrl))
	return s
}

func (s *Sandbox) GetID(ctx context.Context) string {
	return s.id
}

func (s *Sandbox) ExecuteCommand(ctx context.Context, command string) (string, error) {
	req := new(api.ShellExecRequest)
	req.SetCommand(command)
	execCommand, err := s.aioClient.Shell.ExecCommand(ctx, req)
	if err != nil {
		return "", err
	}
	data := execCommand.GetData()
	if data == nil {
		return "", sandbox.ErrNoData
	}
	return safe.Value(data.GetOutput()), nil
}

func (s *Sandbox) ReadFile(ctx context.Context, path string) (string, error) {
	req := new(api.FileReadRequest)
	req.SetFile(path)
	file, err := s.aioClient.File.ReadFile(ctx, req)
	if err != nil {
		return "", err
	}
	data := file.GetData()
	if data == nil {
		return "", sandbox.ErrNoData
	}
	return data.GetContent(), nil
}

func (s *Sandbox) ListDir(ctx context.Context, path string, maxDepth int) ([]string, error) {
	cmdFmt := "find %s -maxdepth %d -type f -o -type d 2>/dev/null | head -500"
	cmd := fmt.Sprintf(cmdFmt, path, maxDepth)
	req := new(api.ShellExecRequest)
	req.SetCommand(cmd)
	execCommand, err := s.aioClient.Shell.ExecCommand(ctx, req)
	if err != nil {
		return nil, err
	}
	data := execCommand.GetData()
	if data == nil {
		return nil, sandbox.ErrNoData
	}

	output := strings.TrimSpace(safe.Value(data.GetOutput()))
	lines := strings.Split(output, "\n")

	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result, nil
}

func (s *Sandbox) WriteFile(ctx context.Context, path string, content string, append bool) error {
	if append {
		file, err := s.ReadFile(ctx, path)
		if err == nil {
			content = file + content
		}
	}
	req := new(api.FileWriteRequest)
	req.SetContent(content)
	req.SetFile(path)
	_, err := s.aioClient.File.WriteFile(ctx, req)
	return err
}

func (s *Sandbox) UpdateFile(ctx context.Context, path string, content string) error {
	toString := base64.StdEncoding.EncodeToString([]byte(content))
	req := new(api.FileWriteRequest)
	req.SetContent(toString)
	req.SetFile(path)
	_, err := s.aioClient.File.WriteFile(ctx, req)
	return err
}

var _ sandbox.Sandbox = &Sandbox{}
