package main

import (
	"strings"
	"testing"
	"time"
)

func TestMainSkillFlowE2E(t *testing.T) {
	threadID := "t_skill_" + time.Now().Format("20060102150405")
	prompt := "请使用 skill-creator 创建一个名为 e2e-skill 的技能，目录放在 /mnt/user-data/workspace/e2e-skill 。必须实际创建 SKILL.md 和 scripts/hello.sh。"
	output := runMainE2E(
		t,
		"u_skill_e2e",
		threadID,
		prompt,
	)

	if !strings.Contains(output, "write file:/mnt/user-data/workspace/e2e-skill/SKILL.md") {
		t.Fatalf("expected SKILL.md creation log, got:\n%s", output)
	}
	if !strings.Contains(output, "write file:/mnt/user-data/workspace/e2e-skill/scripts/hello.sh") {
		t.Fatalf("expected hello.sh creation log, got:\n%s", output)
	}
}
