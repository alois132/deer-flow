package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"

	"github.com/cloudwego/eino/components/tool"
)

const (
	DefaultSubagentType = "general"

	TaskOperationRun    = "run"
	TaskOperationResult = "result"

	TaskModeSync  = "sync"
	TaskModeAsync = "async"

	TaskStatusQueued    = "queued"
	TaskStatusRunning   = "running"
	TaskStatusSucceeded = "succeeded"
	TaskStatusFailed    = "failed"
)

type TaskExecutor interface {
	RunSync(ctx context.Context, subagentType, description string) (string, error)
	RunAsync(ctx context.Context, subagentType, description string) (string, error)
	GetAsyncResult(ctx context.Context, taskID string) (*TaskResult, error)
}

type TaskResult struct {
	TaskID       string `json:"task_id"`
	SubagentType string `json:"subagent_type"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Result       string `json:"result,omitempty"`
	Error        string `json:"error,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type SubagentTaskManager struct {
	mu        sync.RWMutex
	subagents map[string]tool.InvokableTool
	tasks     map[string]*subagentTask
}

type subagentTask struct {
	id           string
	subagentType string
	description  string
	status       string
	result       string
	errMsg       string
	createdAt    time.Time
	updatedAt    time.Time
}

func NewSubagentTaskManager(subagents map[string]tool.InvokableTool) *SubagentTaskManager {
	copied := make(map[string]tool.InvokableTool, len(subagents))
	for k, v := range subagents {
		copied[k] = v
	}
	return &SubagentTaskManager{
		subagents: copied,
		tasks:     map[string]*subagentTask{},
	}
}

func (m *SubagentTaskManager) RunSync(ctx context.Context, subagentType, description string) (string, error) {
	subagent, err := m.getSubagent(subagentType)
	if err != nil {
		return "", err
	}
	taskCtx, err := cloneSessionContext(ctx, false)
	if err != nil {
		return "", err
	}
	return invokeSubagent(taskCtx, subagent, description)
}

func (m *SubagentTaskManager) RunAsync(ctx context.Context, subagentType, description string) (string, error) {
	subagent, err := m.getSubagent(subagentType)
	if err != nil {
		return "", err
	}
	taskCtx, err := cloneSessionContext(ctx, true)
	if err != nil {
		return "", err
	}

	now := time.Now()
	taskID := "task_" + uuid.NewString()
	task := &subagentTask{
		id:           taskID,
		subagentType: subagentType,
		description:  description,
		status:       TaskStatusQueued,
		createdAt:    now,
		updatedAt:    now,
	}

	m.mu.Lock()
	m.tasks[taskID] = task
	m.mu.Unlock()

	go m.runAsyncTask(taskCtx, subagent, task)
	return taskID, nil
}

func (m *SubagentTaskManager) GetAsyncResult(_ context.Context, taskID string) (*TaskResult, error) {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	out := task.toResult()
	m.mu.RUnlock()
	return out, nil
}

func (m *SubagentTaskManager) runAsyncTask(ctx context.Context, subagent tool.InvokableTool, task *subagentTask) {
	defer func() {
		panicErr := recover()
		if panicErr == nil {
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		task.status = TaskStatusFailed
		task.errMsg = fmt.Sprintf("panic: %v", panicErr)
		task.updatedAt = time.Now()
	}()

	m.mu.Lock()
	task.status = TaskStatusRunning
	task.updatedAt = time.Now()
	description := task.description
	m.mu.Unlock()

	result, err := invokeSubagent(ctx, subagent, description)

	m.mu.Lock()
	defer m.mu.Unlock()
	task.updatedAt = time.Now()
	if err != nil {
		task.status = TaskStatusFailed
		task.errMsg = err.Error()
		return
	}
	task.status = TaskStatusSucceeded
	task.result = result
}

func (m *SubagentTaskManager) getSubagent(subagentType string) (tool.InvokableTool, error) {
	m.mu.RLock()
	subagent, ok := m.subagents[subagentType]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("subagent not found: %s", subagentType)
	}
	return subagent, nil
}

func invokeSubagent(ctx context.Context, subagent tool.InvokableTool, description string) (string, error) {
	payload, err := sonic.MarshalString(map[string]string{
		"request": description,
	})
	if err != nil {
		return "", err
	}
	return subagent.InvokableRun(ctx, payload)
}

func cloneSessionContext(ctx context.Context, detached bool) (context.Context, error) {
	session, err := GetSession(ctx)
	if err != nil {
		return nil, err
	}
	copied := *session
	if detached {
		return context.WithValue(context.Background(), SessionKey, &copied), nil
	}
	return context.WithValue(ctx, SessionKey, &copied), nil
}

func (t *subagentTask) toResult() *TaskResult {
	return &TaskResult{
		TaskID:       t.id,
		SubagentType: t.subagentType,
		Description:  t.description,
		Status:       t.status,
		Result:       t.result,
		Error:        t.errMsg,
		CreatedAt:    t.createdAt.Format(time.RFC3339Nano),
		UpdatedAt:    t.updatedAt.Format(time.RFC3339Nano),
	}
}
