package agent

import (
	"context"
	"strings"
	"testing"
)

func TestBuildSkillScriptCommandForPython(t *testing.T) {
	cmd := buildSkillScriptCommand("/mnt/skills/skill-creator", "/mnt/skills/skill-creator/scripts/run_eval.py", "python3", "--help")
	if !strings.Contains(cmd, "PYTHONPATH='/mnt/skills/skill-creator'") {
		t.Fatalf("expected PYTHONPATH in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "python3 '/mnt/skills/skill-creator/scripts/run_eval.py'") {
		t.Fatalf("expected python invocation in command, got: %s", cmd)
	}
	if !strings.HasSuffix(cmd, "--help") {
		t.Fatalf("expected args in command, got: %s", cmd)
	}
}

func TestBuildSkillScriptCommandForShell(t *testing.T) {
	cmd := buildSkillScriptCommand("/mnt/skills/any", "/mnt/skills/any/scripts/run.sh", "bash", "foo bar")
	if strings.Contains(cmd, "PYTHONPATH=") {
		t.Fatalf("did not expect PYTHONPATH for non-python interpreter, got: %s", cmd)
	}
	if cmd != "bash '/mnt/skills/any/scripts/run.sh' foo bar" {
		t.Fatalf("unexpected command: %s", cmd)
	}
}

func TestNormalizeSkillSubPath(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		parent string
		want   string
		hasErr bool
	}{
		{name: "script plain", in: "run_eval.py", parent: "scripts", want: "run_eval.py"},
		{name: "script with prefix", in: "scripts/run_eval.py", parent: "scripts", want: "run_eval.py"},
		{name: "ref plain", in: "schemas.md", parent: "references", want: "schemas.md"},
		{name: "ref with prefix", in: "references/schemas.md", parent: "references", want: "schemas.md"},
		{name: "invalid only parent", in: "scripts", parent: "scripts", hasErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeSkillSubPath(tt.in, tt.parent)
			if tt.hasErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected result: want=%q got=%q", tt.want, got)
			}
		})
	}
}

func TestResolveSkillScriptPath(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   string
		hasErr bool
	}{
		{name: "plain filename", in: "run_eval.py", want: "scripts/run_eval.py"},
		{name: "scripts prefixed", in: "scripts/run_eval.py", want: "scripts/run_eval.py"},
		{name: "root subdir path", in: "eval-viewer/generate_review.py", want: "eval-viewer/generate_review.py"},
		{name: "invalid traversal", in: "../hack.sh", hasErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveSkillScriptPath(tt.in)
			if tt.hasErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected path: want=%q got=%q", tt.want, got)
			}
		})
	}
}

func TestNormalizeTaskArg(t *testing.T) {
	tests := []struct {
		name   string
		input  TaskArg
		hasErr bool
	}{
		{
			name: "run default mode and subagent",
			input: TaskArg{
				Operation:   "",
				Description: "do it",
			},
		},
		{
			name: "run async",
			input: TaskArg{
				Operation:    "run",
				Mode:         "async",
				SubagentType: "general",
				Description:  "do it",
			},
		},
		{
			name: "result with task id",
			input: TaskArg{
				Operation: "result",
				TaskID:    "task_123",
			},
		},
		{
			name: "run missing description",
			input: TaskArg{
				Operation: "run",
			},
			hasErr: true,
		},
		{
			name: "result missing task id",
			input: TaskArg{
				Operation: "result",
			},
			hasErr: true,
		},
		{
			name: "invalid operation",
			input: TaskArg{
				Operation: "invalid",
			},
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeTaskArg(tt.input)
			if tt.hasErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.hasErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetToolsWithTaskExecutor(t *testing.T) {
	tools, err := GetTools(WithTaskExecutor(&mockTaskExecutor{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundTask := false
	for _, tl := range tools {
		info, err := tl.Info(context.Background())
		if err != nil {
			t.Fatalf("unexpected info error: %v", err)
		}
		if info.Name == ToolTask {
			foundTask = true
			break
		}
	}
	if !foundTask {
		t.Fatalf("expected task tool to be registered")
	}
}

type mockTaskExecutor struct{}

func (m *mockTaskExecutor) RunSync(ctx context.Context, subagentType, description string) (string, error) {
	return "ok", nil
}

func (m *mockTaskExecutor) RunAsync(ctx context.Context, subagentType, description string) (string, error) {
	return "task_1", nil
}

func (m *mockTaskExecutor) GetAsyncResult(ctx context.Context, taskID string) (*TaskResult, error) {
	return &TaskResult{
		TaskID:       taskID,
		SubagentType: DefaultSubagentType,
		Description:  "x",
		Status:       TaskStatusSucceeded,
	}, nil
}
