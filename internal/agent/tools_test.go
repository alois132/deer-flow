package agent

import (
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
