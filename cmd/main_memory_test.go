package main

import (
	"strings"
	"testing"
	"time"
)

func TestMainMemoryFlowE2E(t *testing.T) {
	threadID := "t_memory_" + time.Now().Format("20060102150405")
	output := runMainE2E(
		t,
		"u_memory_e2e",
		threadID,
		"请回复一句：memory-e2e-ok",
	)

	if !strings.Contains(output, "[memory] load memory for user u_memory_e2e") {
		t.Fatalf("expected memory load log, got:\n%s", output)
	}
	if !strings.Contains(output, "[memory] update memory for user u_memory_e2e") {
		t.Fatalf("expected memory update log, got:\n%s", output)
	}
}
