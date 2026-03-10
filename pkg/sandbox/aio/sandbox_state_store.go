package aio

import (
	"context"
	"encoding/json"
	"github.com/alois132/deer-flow/pkg/log/zlog"
	"os"
	"path/filepath"
	"syscall"
)

const (
	SANDBOX_STATE_FILE = "sandbox.json"
	SANDBOX_LOCK_FILE  = "sandbox.lock"
)

type UnLock func()

type SandboxStateStore interface {
	Save(ctx context.Context, threadID string, info *SandboxInfo) error
	Load(ctx context.Context, threadID string) (*SandboxInfo, error)
	Remove(ctx context.Context, threadID string) error
	Lock(ctx context.Context, threadID string) (UnLock, error)
}

type FileSandboxStateStore struct {
	Paths *Paths
}

func (f *FileSandboxStateStore) Save(ctx context.Context, threadID string, info *SandboxInfo) error {
	dir, err := f.Paths.ThreadDir(threadID)
	if err != nil {
		return err
	}
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	stateFile := filepath.Join(dir, SANDBOX_STATE_FILE)
	infoB, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = os.WriteFile(stateFile, infoB, 0644)
	if err != nil {
		zlog.CtxErrorf(ctx, "write sandbox state file error: %v", err)
		return err
	}
	return nil
}

func (f *FileSandboxStateStore) Load(ctx context.Context, threadID string) (*SandboxInfo, error) {
	dir, err := f.Paths.ThreadDir(threadID)
	if err != nil {
		return nil, err
	}
	info := new(SandboxInfo)
	stateFile := filepath.Join(dir, SANDBOX_STATE_FILE)
	file, err := os.ReadFile(stateFile)
	if os.IsNotExist(err) {
		return info, nil
	}
	if err != nil {
		zlog.CtxErrorf(ctx, "read sandbox state file error: %v", err)
		return nil, err
	}
	err = json.Unmarshal(file, info)
	if err != nil {
		zlog.CtxErrorf(ctx, "parse sandbox state file error: %v", err)
		return nil, err
	}
	return info, nil
}

func (f *FileSandboxStateStore) Remove(ctx context.Context, threadID string) error {
	dir, err := f.Paths.ThreadDir(threadID)
	if err != nil {
		zlog.CtxErrorf(ctx, "get thread dir error: %v", err)
		return err
	}
	stateFile := filepath.Join(dir, SANDBOX_STATE_FILE)
	err = os.Remove(stateFile)
	if os.IsNotExist(err) {
		zlog.CtxWarnf(ctx, "sandbox state file(%s) not exists", stateFile)
		return nil
	}
	if err != nil {
		zlog.CtxErrorf(ctx, "remove sandbox state file error: %v", err)
		return err
	}
	return nil
}

func (f *FileSandboxStateStore) Lock(ctx context.Context, threadID string) (UnLock, error) {
	dir, err := f.Paths.ThreadDir(threadID)
	if err != nil {
		zlog.CtxErrorf(ctx, "get thread dir error: %v", err)
		return nil, err
	}
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		zlog.CtxErrorf(ctx, "mkdir thread dir error: %v", err)
		return nil, err
	}
	lockFile := filepath.Join(dir, SANDBOX_LOCK_FILE)
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		zlog.CtxErrorf(ctx, "open sandbox lock file error: %v", err)
		return nil, err
	}
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
	if err != nil {
		file.Close()
		zlog.CtxErrorf(ctx, "lock sandbox lock file error: %v", err)
		return nil, err
	}
	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}

func NewFileSandboxStateStore(paths *Paths) SandboxStateStore {
	return &FileSandboxStateStore{
		Paths: paths,
	}
}
