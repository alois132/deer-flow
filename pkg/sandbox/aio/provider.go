package aio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/alois132/deer-flow/pkg/log/zlog"
	"github.com/alois132/deer-flow/pkg/sandbox"
	"github.com/alois132/deer-flow/utils/safe"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DEFAULT_IMAGE            = "enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest"
	DEFAULT_PORT             = 8080
	DEFAULT_CONTAINER_PREFIX = "deer-flow-sandbox"
	DEFAULT_IDLE_TIMEOUT     = 600 // 10 minutes in seconds
	IDLE_CHECK_INTERVAL      = 60  // Check every 60 seconds
)

type Provider struct {
	mu              sync.RWMutex
	sandboxes       map[string]sandbox.Sandbox
	sandboxInfos    map[string]*SandboxInfo
	threadSandboxes map[string]string
	threadLocks     map[string]*sync.Mutex
	lastActivity    map[string]time.Time
	idleCheckerDone chan struct{}
	idleCheckerStop chan struct{}
	shutdownCalled  bool
	backend         SandboxBackend
	stateStore      SandboxStateStore
	cfg             *sandbox.Config
	idleTimeout     int
	paths           *Paths
}

func (p *Provider) Acquire(ctx context.Context, threadID string) (string, error) {
	if threadID == "" {
		return "", fmt.Errorf("threadID is empty")
	}
	if p.threadLocks[threadID] == nil {
		p.threadLocks[threadID] = &sync.Mutex{}
	}
	p.threadLocks[threadID].Lock()
	defer p.threadLocks[threadID].Unlock()

	p.mu.RLock()
	if existingID, ok := p.threadSandboxes[threadID]; ok {
		zlog.CtxInfof(ctx, "thread %s already has a sandbox, reusing it", threadID)
		p.mu.RUnlock()
		p.mu.Lock()
		p.lastActivity[existingID] = time.Now()
		p.mu.Unlock()
		return existingID, nil
	} else {
		p.mu.RUnlock()
	}

	sum256 := sha256.Sum256([]byte(threadID))
	sandboxID := hex.EncodeToString(sum256[:])[:8]

	unlock, err := p.stateStore.Lock(ctx, threadID)
	if err != nil {
		return "", err
	}
	defer unlock()
	newSandboxID, _ := p.tryRecover(ctx, threadID)
	if newSandboxID != "" {
		return newSandboxID, nil
	}

	return p.createSandbox(ctx, threadID, sandboxID)
}

func (p *Provider) createSandbox(ctx context.Context, threadID, sandboxID string) (string, error) {
	mounts, err := p.getExtraMounts(ctx, threadID)
	if err != nil {
		zlog.CtxErrorf(ctx, "get extra mounts error: %v", err)
		return "", err
	}
	info, err := p.backend.Create(ctx, threadID, sandboxID, mounts)
	if err != nil {
		zlog.CtxErrorf(ctx, "create sandbox error: %v", err)
		return "", err
	}
	ready, _ := WaitForSandboxReady(ctx, info.SandboxURL, 60*time.Second)
	if !ready {
		err := p.backend.Destroy(ctx, info)
		if err != nil {
			zlog.CtxErrorf(ctx, "destroy sandbox error: %v", err)
		}
		zlog.CtxErrorf(ctx, "sandbox %s is not ready", sandboxID)
		return "", fmt.Errorf("sandbox %s is not ready", sandboxID)
	}
	newSandbox := NewSandbox(info.SandboxID, info.SandboxURL, "")
	p.mu.Lock()
	p.sandboxes[sandboxID] = newSandbox
	p.sandboxInfos[sandboxID] = info
	p.lastActivity[sandboxID] = time.Now()
	p.threadSandboxes[threadID] = sandboxID
	p.mu.Unlock()
	err = p.stateStore.Save(ctx, threadID, info)
	if err != nil {
		zlog.CtxErrorf(ctx, "save sandbox state error: %v", err)
		return "", err
	}
	return sandboxID, nil
}

func (p *Provider) getExtraMounts(ctx context.Context, threadID string) ([]*sandbox.VolumeMount, error) {
	var mounts []*sandbox.VolumeMount
	threadMounts, err := p.getThreadMounts(ctx, threadID)
	if err != nil {
		zlog.CtxErrorf(ctx, "get thread mounts error: %v", err)
		return nil, err
	}
	mounts = append(mounts, threadMounts...)
	skillsMounts, err := p.getSkillsMount(ctx)
	if err != nil {
		zlog.CtxErrorf(ctx, "get skills skillsMounts error: %v", err)
		return nil, err
	}
	mounts = append(mounts, skillsMounts...)
	return mounts, nil
}

func (p *Provider) getSkillsMount(ctx context.Context) ([]*sandbox.VolumeMount, error) {
	skills := p.cfg.Skills
	path := skills.Path
	containerPath := skills.ContainerPath
	abs, err := filepath.Abs(path)
	if err != nil {
		zlog.CtxErrorf(ctx, "resolve path error: %v", err)
		return nil, err
	}
	err = os.MkdirAll(abs, 0755)
	if err != nil {
		zlog.CtxErrorf(ctx, "mkdir skills dir error: %v", err)
		return nil, err
	}
	return []*sandbox.VolumeMount{
		{
			ContainerPath: containerPath,
			HostPath:      abs,
			ReadOnly:      true,
		},
	}, nil
}

func (p *Provider) getThreadMounts(ctx context.Context, threadID string) ([]*sandbox.VolumeMount, error) {
	var mounts []*sandbox.VolumeMount
	err := p.paths.EnsureThreadDirs(threadID)
	if err != nil {
		zlog.CtxErrorf(ctx, "ensure thread dirs error: %v", err)
		return nil, err
	}
	dir, err := p.paths.SandboxWorkDir(threadID)
	if err != nil {
		zlog.CtxErrorf(ctx, "ensure sandbox work dir error: %v", err)
		return nil, err
	}
	mounts = append(mounts, &sandbox.VolumeMount{
		ContainerPath: VirtualPathPrefix + "/workspace",
		HostPath:      dir,
		ReadOnly:      false,
	})
	dir, err = p.paths.SandboxUploadsDir(threadID)
	if err != nil {
		zlog.CtxErrorf(ctx, "ensure sandbox uploads dir error: %v", err)
		return nil, err
	}
	mounts = append(mounts, &sandbox.VolumeMount{
		ContainerPath: VirtualPathPrefix + "/uploads",
		HostPath:      dir,
		ReadOnly:      false,
	})
	dir, err = p.paths.SandboxOutputsDir(threadID)
	if err != nil {
		zlog.CtxErrorf(ctx, "ensure sandbox outputs dir error: %v", err)
		return nil, err
	}
	mounts = append(mounts, &sandbox.VolumeMount{
		ContainerPath: VirtualPathPrefix + "/outputs",
		HostPath:      dir,
		ReadOnly:      false,
	})
	return mounts, nil
}

func (p *Provider) tryRecover(ctx context.Context, threadID string) (string, error) {
	info, err := p.stateStore.Load(ctx, threadID)
	if err != nil {
		return "", err
	}
	dis, err := p.backend.Discover(ctx, info.SandboxID)
	if err != nil {
		err := p.stateStore.Remove(ctx, threadID)
		if err != nil {
			zlog.CtxErrorf(ctx, "remove sandbox state error: %v", err)
		}
		return "", err
	}
	sandbox := NewSandbox(dis.SandboxID, dis.SandboxURL, "")
	p.mu.Lock()
	p.sandboxes[info.SandboxID] = sandbox
	p.sandboxInfos[info.SandboxID] = dis
	p.lastActivity[threadID] = time.Now()
	p.threadSandboxes[threadID] = dis.SandboxID
	p.mu.Unlock()

	if dis.SandboxURL != info.SandboxURL {
		err := p.stateStore.Save(ctx, threadID, dis)
		if err != nil {
			zlog.CtxErrorf(ctx, "save sandbox state error: %v", err)
			return "", err
		}
	}

	zlog.CtxInfof(ctx, "thread %s acquired sandbox %s", threadID, dis.SandboxID)
	return dis.SandboxID, nil
}

func (p *Provider) Get(ctx context.Context, sandboxID string) (sandbox.Sandbox, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	sandbox, ok := p.sandboxes[sandboxID]
	if !ok {
		return nil, fmt.Errorf("sandbox %s not found", sandboxID)
	}
	p.lastActivity[sandboxID] = time.Now()
	return sandbox, nil
}

func (p *Provider) startIdleChecker() {
	go p.idleCheckerLoop()
}

func (p *Provider) idleCheckerLoop() {
	defer close(p.idleCheckerStop)
	ctx := context.Background()
	ticker := time.NewTicker(time.Duration(IDLE_CHECK_INTERVAL) * time.Second)
	for {
		select {
		case <-ticker.C:
			p.cleanupIdle(ctx)
		case <-p.idleCheckerDone:
			return
		}
	}
}

func (p *Provider) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	if p.shutdownCalled {
		p.mu.Unlock()
		return nil
	}

	p.shutdownCalled = true

	var sandboxIDs []string
	for id := range p.sandboxes {
		sandboxIDs = append(sandboxIDs, id)
	}
	p.mu.Unlock()
	close(p.idleCheckerDone)
	<-p.idleCheckerStop

	for _, id := range sandboxIDs {
		err := p.Release(ctx, id)
		if err != nil {
			zlog.CtxErrorf(ctx, "release sandbox error: %v", err)
		}
	}
	zlog.CtxInfof(ctx, "shutdown sandbox provider")
	return nil
}

func (p *Provider) cleanupIdle(ctx context.Context) {
	now := time.Now()

	p.mu.Lock()
	toRelease := make([]string, 0)
	for id, last := range p.lastActivity {
		if now.Sub(last) > time.Duration(p.idleTimeout)*time.Second {
			toRelease = append(toRelease, id)
		}
	}
	p.mu.Unlock()

	for _, id := range toRelease {
		p.Release(ctx, id)
	}
}

func (p *Provider) Release(ctx context.Context, sandboxID string) error {
	var info *SandboxInfo
	var threadIDs []string
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.sandboxes, sandboxID)
	info = p.sandboxInfos[sandboxID]
	delete(p.sandboxInfos, sandboxID)
	for _, id := range p.threadSandboxes {
		if id == sandboxID {
			threadIDs = append(threadIDs, p.threadSandboxes[id])
		}
	}
	for _, id := range threadIDs {
		delete(p.threadSandboxes, id)
	}
	delete(p.lastActivity, sandboxID)
	for _, id := range threadIDs {
		err := p.stateStore.Remove(ctx, id)
		if err != nil {
			zlog.CtxErrorf(ctx, "remove sandbox state error: %v", err)
			return err
		}
	}

	if info != nil {
		err := p.backend.Destroy(ctx, info)
		if err != nil {
			zlog.CtxErrorf(ctx, "destroy sandbox error: %v", err)
			return err
		}
	}
	return nil
}

func NewProvider(cfg *sandbox.Config) (sandbox.Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	var backend SandboxBackend
	if cfg.ProvisionerURL != "" {
		zlog.Infof("使用远程沙盒服务, provisioner_url:%s", cfg.ProvisionerURL)
		backend = NewRemoteSandboxBackend(cfg.ProvisionerURL, safe.NewHttpClient(3*time.Second))
	} else {
		zlog.Infof("使用本地沙盒服务")
		backend = NewLocalSandboxBackend(cfg.Image, cfg.Port, cfg.ContainerPrefix, cfg.Mounts, cfg.Environment)
	}
	paths, err := NewPaths(cfg.BaseDir)
	if err != nil {
		return nil, err
	}
	store := NewFileSandboxStateStore(paths)
	idleTimeout := cfg.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = DEFAULT_IDLE_TIMEOUT
	}
	provider := &Provider{
		sandboxes:       make(map[string]sandbox.Sandbox),
		sandboxInfos:    make(map[string]*SandboxInfo),
		threadSandboxes: make(map[string]string),
		threadLocks:     make(map[string]*sync.Mutex),
		lastActivity:    make(map[string]time.Time),
		idleCheckerDone: make(chan struct{}),
		idleCheckerStop: make(chan struct{}),
		shutdownCalled:  false,
		backend:         backend,
		stateStore:      store,
		cfg:             cfg,
		idleTimeout:     idleTimeout,
		paths:           paths,
	}

	provider.startIdleChecker()
	return provider, nil
}
