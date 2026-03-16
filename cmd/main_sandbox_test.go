package main

import (
	"strings"
	"testing"
	"time"
)

func TestMainSandboxFlowE2E(t *testing.T) {
	threadID := "t_sandbox_" + time.Now().Format("20060102150405")
	output := runMainE2E(
		t,
		"u_sandbox_e2e",
		threadID,
		"请回复一句：sandbox-e2e-ok",
	)

	if !strings.Contains(output, "sandbox acquired") {
		t.Fatalf("expected sandbox acquire log, got:\n%s", output)
	}
	if !strings.Contains(output, "shutdown sandbox provider") {
		t.Fatalf("expected sandbox shutdown log, got:\n%s", output)
	}
	if strings.Contains(output, "invalid thread ID") {
		t.Fatalf("unexpected invalid thread ID error, got:\n%s", output)
	}
}
