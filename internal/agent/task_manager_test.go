package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type stubInvokableTool struct {
	runFn func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error)
}

func (s *stubInvokableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "stub"}, nil
}

func (s *stubInvokableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if s.runFn == nil {
		return "", nil
	}
	return s.runFn(ctx, argumentsInJSON, opts...)
}

func TestSubagentTaskManagerRunSync(t *testing.T) {
	manager := NewSubagentTaskManager(map[string]tool.InvokableTool{
		DefaultSubagentType: &stubInvokableTool{
			runFn: func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
				if !strings.Contains(argumentsInJSON, `"request":"hello"`) {
					t.Fatalf("unexpected payload: %s", argumentsInJSON)
				}
				return "done", nil
			},
		},
	})
	ctx, _ := InitCtx(context.Background())
	result, err := manager.RunSync(ctx, DefaultSubagentType, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "done" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestSubagentTaskManagerRunAsyncAndGetResult(t *testing.T) {
	manager := NewSubagentTaskManager(map[string]tool.InvokableTool{
		DefaultSubagentType: &stubInvokableTool{
			runFn: func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
				time.Sleep(30 * time.Millisecond)
				return "async done", nil
			},
		},
	})
	ctx, _ := InitCtx(context.Background())
	taskID, err := manager.RunAsync(ctx, DefaultSubagentType, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		result, err := manager.GetAsyncResult(ctx, taskID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Status == TaskStatusSucceeded {
			if result.Result != "async done" {
				t.Fatalf("unexpected result: %s", result.Result)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("task did not finish in time")
}

func TestSubagentTaskManagerRunAsyncFailure(t *testing.T) {
	manager := NewSubagentTaskManager(map[string]tool.InvokableTool{
		DefaultSubagentType: &stubInvokableTool{
			runFn: func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
				return "", errors.New("boom")
			},
		},
	})
	ctx, _ := InitCtx(context.Background())
	taskID, err := manager.RunAsync(ctx, DefaultSubagentType, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		result, err := manager.GetAsyncResult(ctx, taskID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Status == TaskStatusFailed {
			if !strings.Contains(result.Error, "boom") {
				t.Fatalf("unexpected error message: %s", result.Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("task did not fail in time")
}

func TestSubagentTaskManagerMissingSession(t *testing.T) {
	manager := NewSubagentTaskManager(map[string]tool.InvokableTool{
		DefaultSubagentType: &stubInvokableTool{
			runFn: func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
				return "ok", nil
			},
		},
	})
	_, err := manager.RunSync(context.Background(), DefaultSubagentType, "hello")
	if err == nil {
		t.Fatalf("expected missing session error")
	}
}
