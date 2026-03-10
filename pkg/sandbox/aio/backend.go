package aio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/alois132/deer-flow/pkg/log/zlog"
	"github.com/alois132/deer-flow/pkg/sandbox"
	"github.com/alois132/deer-flow/utils/safe"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ContainerRuntime 表示检测到的容器运行时类型
type ContainerRuntime string

const (
	RuntimeAppleContainer ContainerRuntime = "container" // Apple Container
	RuntimeDocker         ContainerRuntime = "docker"    // Docker
)

type SandboxBackend interface {
	Create(ctx context.Context, threadID string, sandboxID string, extraMounts []*sandbox.VolumeMount) (*SandboxInfo, error)
	Destroy(ctx context.Context, info *SandboxInfo) error
	IsAlive(ctx context.Context, info *SandboxInfo) (bool, error)
	Discover(ctx context.Context, sandboxID string) (*SandboxInfo, error)
}

type RemoteSandboxBackend struct {
	provisionerURL string
	client         *safe.HttpClient
}

type LocalSandboxBackend struct {
	image           string
	port            int
	containerPrefix string
	mounts          []*sandbox.VolumeMount
	environment     map[string]string
	runtime         ContainerRuntime
	client          *http.Client
}

func (l *LocalSandboxBackend) deleteRuntime() ContainerRuntime {
	// 只在 macOS 上检测 Apple Container
	if runtime.GOOS == "darwin" {
		// 创建带超时的上下文（5秒）
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 尝试执行 `container --version`
		cmd := exec.CommandContext(ctx, "container", "--version")
		output, err := cmd.CombinedOutput()

		if err == nil {
			version := strings.TrimSpace(string(output))
			zlog.Infof("检测到 Apple Container", "version", version)
			return RuntimeAppleContainer
		}

		// 处理不同类型的错误
		if ctx.Err() == context.DeadlineExceeded {
			zlog.Infof("检测 Apple Container 超时，回退到 Docker")
		} else if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			zlog.Infof("Apple Container 未安装，回退到 Docker")
		} else {
			zlog.Infof("Apple Container 不可用，回退到 Docker", "error", err)
		}
	}

	return RuntimeDocker
}

func (l *LocalSandboxBackend) startContainer(ctx context.Context, containerName string, port int, extraMounts []*sandbox.VolumeMount) (string, error) {
	cmd := []string{string(l.runtime), "run"}
	if l.runtime == RuntimeDocker {
		cmd = append(cmd, "--security-opt", "seccomp=unconfined")
	}
	cmd = append(cmd, "--rm", "-d", "-p",
		fmt.Sprintf("%d:8080", port),
		"--name",
		containerName)
	for k, v := range l.environment {
		cmd = append(cmd, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	for _, m := range l.mounts {
		mountSpec := fmt.Sprintf("%s:%s",
			m.HostPath,
			m.ContainerPath)
		if m.ReadOnly {
			mountSpec += ":ro"
		}
		cmd = append(cmd, "-v", mountSpec)
	}
	cmd = append(cmd, l.image)
	zlog.CtxInfof(ctx, "启动容器, cmd:%v", cmd)
	eCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	output, err := eCmd.CombinedOutput()
	if err != nil {
		zlog.CtxErrorf(ctx, "启动容器失败, error:%v; output:%s", err, string(output))
		return "", err
	}
	containerID := strings.TrimSpace(string(output))
	zlog.CtxInfof(ctx, "启动容器成功，container_id(%s)", containerID)
	return containerID, err
}

func (l *LocalSandboxBackend) stopContainer(ctx context.Context, containerID string) error {
	commandContext := exec.CommandContext(ctx, string(l.runtime), "stop", containerID)
	output, err := commandContext.CombinedOutput()
	if err != nil {
		zlog.CtxErrorf(ctx, "停止容器失败, error:%v; output:%s", err, string(output))
		return err
	}
	zlog.CtxInfof(ctx, "停止容器成功，container_id(%s)", containerID)
	return nil
}

func (l *LocalSandboxBackend) isContainerRunning(ctx context.Context, containerName string) (bool, error) {
	commandContext := exec.CommandContext(ctx, string(l.runtime), "inspect", "--format", "{{.State.Running}}", containerName)
	output, err := commandContext.CombinedOutput()
	if err != nil {
		zlog.CtxErrorf(ctx, "检查容器运行状态失败, error:%v; output:%s", err, string(output))
		return false, err
	}

	status := strings.TrimSpace(strings.ToLower(string(output)))
	return status == "true", nil
}

func (l *LocalSandboxBackend) getContainerPort(ctx context.Context, containerName string) (int, error) {
	commandContext := exec.CommandContext(ctx, string(l.runtime), "port", containerName, "8080")
	output, err := commandContext.CombinedOutput()
	if err != nil {
		zlog.CtxErrorf(ctx, "获取容器端口失败, error:%v; output:%s", err, string(output))
		return 0, err
	}
	var containerPort int
	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return 0, fmt.Errorf("port %d not mapped", containerPort)
	}

	// 取第一行，解析最后一部分
	lines := strings.Split(outputStr, "\n")
	hostAddr := strings.TrimSpace(lines[0])

	parts := strings.Split(hostAddr, ":")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid address format: %s", hostAddr)
	}

	var port int
	_, err = fmt.Sscanf(parts[len(parts)-1], "%d", &port)
	if err != nil {
		return 0, fmt.Errorf("failed to parse port: %w", err)
	}
	return port, nil
}

func (l *LocalSandboxBackend) waitForSandboxReady(ctx context.Context, sandboxURL string, timeout time.Duration) (bool, error) {
	endTime := time.Now().Add(timeout)
	for time.Now().Before(endTime) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, sandboxURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := l.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			return true, nil
		}
		time.Sleep(1 * time.Second)
	}
	return false, fmt.Errorf("timeout waiting for sandbox to be ready")
}

func (l *LocalSandboxBackend) Create(ctx context.Context, threadID string, sandboxID string, extraMounts []*sandbox.VolumeMount) (*SandboxInfo, error) {
	containerName := l.containerPrefix + "-" + sandboxID
	port, err := safe.GetFreePort(l.port, 100)
	if err != nil {
		return nil, err
	}
	containerID, err := l.startContainer(ctx, containerName, port, extraMounts)
	if err != nil {
		safe.ReleasePort(port)
		return nil, err
	}
	return &SandboxInfo{
		ContainerID:   containerID,
		ContainerName: containerName,
		CreatedAt:     time.Now(),
		SandboxID:     sandboxID,
		SandboxURL:    fmt.Sprintf("http://localhost:%d", port),
		HostPort:      port,
	}, nil
}

func (l *LocalSandboxBackend) Destroy(ctx context.Context, info *SandboxInfo) error {
	err := l.stopContainer(ctx, info.ContainerID)
	if err != nil {
		return err
	}
	safe.ReleasePort(info.HostPort)
	return nil
}

func (l *LocalSandboxBackend) IsAlive(ctx context.Context, info *SandboxInfo) (bool, error) {
	return l.isContainerRunning(ctx, info.ContainerName)
}

func (l *LocalSandboxBackend) Discover(ctx context.Context, sandboxID string) (*SandboxInfo, error) {
	containerName := l.containerPrefix + "-" + sandboxID
	running, err := l.isContainerRunning(ctx, containerName)
	if err != nil {
		return nil, err
	}
	if !running {
		return nil, nil
	}
	port, err := l.getContainerPort(ctx, containerName)
	if err != nil {
		return nil, err
	}
	sandboxURL := fmt.Sprintf("http://localhost:%d", port)
	ready, err := l.waitForSandboxReady(ctx, sandboxURL, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, fmt.Errorf("sandbox not ready")
	}
	return &SandboxInfo{
		ContainerName: containerName,
		SandboxID:     sandboxID,
		SandboxURL:    sandboxURL,
		HostPort:      port,
	}, nil
}

type Data struct {
	SandboxURL string `json:"sandbox_url"`
}

func (b *RemoteSandboxBackend) Create(ctx context.Context, threadID string, sandboxID string, extraMounts []*sandbox.VolumeMount) (*SandboxInfo, error) {
	if b.client == nil {
		b.client = safe.NewHttpClient(3 * time.Second)
	}
	body := map[string]interface{}{
		"thread_id":  threadID,
		"sandbox_id": sandboxID,
	}
	marshal, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, b.provisionerURL+"/api/sandboxes", bytes.NewReader(marshal))
	if err != nil {
		return nil, err
	}
	resp, err := b.client.Do(req, safe.DefaultRetry)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("status code: %d", resp.StatusCode)
		return nil, err
	}
	var data Data
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}
	return &SandboxInfo{
		SandboxID:  sandboxID,
		SandboxURL: data.SandboxURL,
	}, nil

}

func (b *RemoteSandboxBackend) Destroy(ctx context.Context, info *SandboxInfo) error {
	if b.client == nil {
		b.client = safe.NewHttpClient(3 * time.Second)
	}
	url := b.provisionerURL + "/api/sandboxes/" + info.SandboxID
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := b.client.Do(req, safe.DefaultRetry)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("status code: %d", resp.StatusCode)
		return err
	}

	return nil
}

func (b *RemoteSandboxBackend) IsAlive(ctx context.Context, info *SandboxInfo) (bool, error) {
	if b.client == nil {
		b.client = safe.NewHttpClient(3 * time.Second)
	}
	url := b.provisionerURL + "/api/sandboxes/" + info.SandboxID
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	resp, err := b.client.Do(req, safe.DefaultRetry)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("status code: %d", resp.StatusCode)
		return false, err
	}
	data := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return false, err
	}
	return data["status"] == "Running", nil
}

func (b *RemoteSandboxBackend) Discover(ctx context.Context, sandboxID string) (*SandboxInfo, error) {
	if b.client == nil {
		b.client = safe.NewHttpClient(3 * time.Second)
	}
	url := b.provisionerURL + "/api/sandboxes/" + sandboxID
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.client.Do(req, safe.DefaultRetry)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("sandbox not found")
	}
	data := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}
	var sandboxURL string
	if v, ok := data["sandbox_url"].(string); ok {
		sandboxURL = v
	} else {
		return nil, fmt.Errorf("sandbox_url is not string")
	}

	return &SandboxInfo{
		SandboxID:  sandboxID,
		SandboxURL: sandboxURL,
	}, nil
}

func NewRemoteSandboxBackend(provisionerURL string, client *safe.HttpClient) SandboxBackend {
	return &RemoteSandboxBackend{
		provisionerURL: provisionerURL,
		client:         client,
	}
}

func NewLocalSandboxBackend(image string, port int, containerPrefix string, mounts []*sandbox.VolumeMount, Environment map[string]string) SandboxBackend {
	return &LocalSandboxBackend{
		image:           image,
		port:            port,
		containerPrefix: containerPrefix,
		mounts:          mounts,
		environment:     Environment,
		client:          http.DefaultClient,
	}
}
