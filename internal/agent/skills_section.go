package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alois132/deer-flow/internal/global"
	"github.com/alois132/deer-flow/pkg/sandbox"
	"gopkg.in/yaml.v3"
)

type skillMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func buildSkillsSection(cfg *global.Config) string {
	if cfg == nil || !isSkillFeatureEnabled(cfg.Sandbox.Skills) {
		return ""
	}
	root := strings.TrimSpace(cfg.Sandbox.Skills.Path)
	if root == "" {
		root = "./skills"
	}
	skills, err := discoverSkills(root)
	if err != nil || len(skills) == 0 {
		return ""
	}

	lines := make([]string, 0, len(skills)+12)
	lines = append(lines, "<skills>")
	lines = append(lines, "Available local skills (synced to /mnt/skills):")
	for _, item := range skills {
		if item.Description == "" {
			lines = append(lines, fmt.Sprintf("- %s", item.Name))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", item.Name, item.Description))
	}
	lines = append(lines, "")
	lines = append(lines, "Skill workflow:")
	lines = append(lines, "1) If the task matches a skill description, call `read_skill` with that skill name.")
	lines = append(lines, "2) Follow SKILL.md and call `read_reference` only for needed reference files.")
	lines = append(lines, "3) Use `use_script` for scripts under the skill's scripts directory when needed.")
	lines = append(lines, "</skills>")
	return strings.Join(lines, "\n")
}

func discoverSkills(root string) ([]skillMetadata, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(absRoot)
	if err != nil {
		return nil, err
	}

	skills := make([]skillMetadata, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skillPath := filepath.Join(absRoot, entry.Name(), "SKILL.md")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		meta := parseSkillFrontmatter(string(content))
		if meta.Name == "" {
			meta.Name = entry.Name()
		}
		skills = append(skills, meta)
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

func parseSkillFrontmatter(content string) skillMetadata {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return skillMetadata{}
	}
	trimmed := strings.TrimPrefix(content, "---\n")
	end := strings.Index(trimmed, "\n---\n")
	if end < 0 {
		return skillMetadata{}
	}
	yamlBlock := trimmed[:end]
	var meta skillMetadata
	if err := yaml.Unmarshal([]byte(yamlBlock), &meta); err != nil {
		return skillMetadata{}
	}
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Description = strings.TrimSpace(meta.Description)
	return meta
}

func isSkillFeatureEnabled(cfg sandbox.Skills) bool {
	if cfg.Enabled == nil {
		return true
	}
	return *cfg.Enabled
}
