package sandbox

import (
	"context"
)

type Provider interface {
	Acquire(ctx context.Context, threadID string) (string, error)
	Get(ctx context.Context, sandboxID string) (Sandbox, error)
	Release(ctx context.Context, sandboxID string) error
	Shutdown(ctx context.Context) error
}

var defaultProvider Provider

func SetSandboxProvider(provider Provider) {
	defaultProvider = provider
}

func ResetSandboxProvider() {
	defaultProvider = nil
}

func ShutdownSandboxProvider(ctx context.Context) error {
	if defaultProvider != nil {
		err := defaultProvider.Shutdown(ctx)
		if err != nil {
			return err
		}
		defaultProvider = nil
	}
	return nil
}
