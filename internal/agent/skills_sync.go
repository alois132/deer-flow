package agent

import (
	"context"
	"github.com/alois132/deer-flow/internal/global"
	"github.com/alois132/deer-flow/pkg/sandbox"
	"github.com/alois132/deer-flow/pkg/skills"
)

func syncSkillsToSandbox(ctx context.Context, sb sandbox.Sandbox) error {
	cfg := global.GetCfg()
	if cfg == nil {
		return nil
	}
	if !skillsEnabled(cfg.Sandbox.Skills) || !skillsSyncEnabled(cfg.Sandbox.Skills) {
		return nil
	}
	store := skills.NewLocalStore(cfg.Sandbox.Skills.Path, skills.WithCleanRemote(skillsCleanEnabled(cfg.Sandbox.Skills)))
	return store.Load(ctx, sb, cfg.Sandbox.Skills.ContainerPath)
}

func syncSkillsFromSandbox(ctx context.Context, sb sandbox.Sandbox) error {
	cfg := global.GetCfg()
	if cfg == nil {
		return nil
	}
	if !skillsEnabled(cfg.Sandbox.Skills) || !skillsSyncEnabled(cfg.Sandbox.Skills) {
		return nil
	}
	store := skills.NewLocalStore(cfg.Sandbox.Skills.Path)
	return store.Save(ctx, sb, cfg.Sandbox.Skills.ContainerPath)
}

func skillsEnabled(cfg sandbox.Skills) bool {
	if cfg.Enabled == nil {
		return true
	}
	return *cfg.Enabled
}

func skillsSyncEnabled(cfg sandbox.Skills) bool {
	if cfg.Sync == nil {
		return true
	}
	return *cfg.Sync
}

func skillsCleanEnabled(cfg sandbox.Skills) bool {
	if cfg.Clean == nil {
		return false
	}
	return *cfg.Clean
}
