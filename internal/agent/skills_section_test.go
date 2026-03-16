package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alois132/deer-flow/internal/global"
	"github.com/alois132/deer-flow/pkg/sandbox"
)

func TestParseSkillFrontmatter(t *testing.T) {
	content := `---
name: skill-creator
description: create and improve skills
---

# Skill`
	meta := parseSkillFrontmatter(content)
	if meta.Name != "skill-creator" {
		t.Fatalf("unexpected name: %q", meta.Name)
	}
	if meta.Description != "create and improve skills" {
		t.Fatalf("unexpected description: %q", meta.Description)
	}
}

func TestBuildSkillsSection(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skill-creator"), 0755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	skillMD := `---
name: skill-creator
description: create and improve skills
---
`
	if err := os.WriteFile(filepath.Join(root, "skill-creator", "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("write skill md: %v", err)
	}
	cfg := &global.Config{
		Sandbox: sandbox.Config{
			Skills: sandbox.Skills{
				Path: root,
			},
		},
	}

	section := buildSkillsSection(cfg)
	if !strings.Contains(section, "skill-creator: create and improve skills") {
		t.Fatalf("skills section missing metadata: %s", section)
	}
	if !strings.Contains(section, "read_skill") {
		t.Fatalf("skills section missing workflow guide: %s", section)
	}
}

func TestBuildSkillsSectionDisabled(t *testing.T) {
	disabled := false
	cfg := &global.Config{
		Sandbox: sandbox.Config{
			Skills: sandbox.Skills{
				Enabled: &disabled,
				Path:    t.TempDir(),
			},
		},
	}
	section := buildSkillsSection(cfg)
	if section != "" {
		t.Fatalf("expected empty section when disabled, got: %s", section)
	}
}
