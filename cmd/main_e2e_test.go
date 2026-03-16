package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const deerflowE2EEnv = "DEERFLOW_E2E"

func runMainE2E(t *testing.T, userID, threadID, prompt string) string {
	t.Helper()
	if os.Getenv(deerflowE2EEnv) != "1" {
		t.Skip("skip e2e test; set DEERFLOW_E2E=1 to enable")
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, ".."))

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/main.go")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"DEERFLOW_USER_ID="+userID,
		"DEERFLOW_THREAD_ID="+threadID,
		"DEERFLOW_PROMPT="+prompt,
	)

	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("run main timeout: %s", string(out))
	}
	if err != nil {
		t.Fatalf("run main failed: %v\noutput:\n%s", err, string(out))
	}
	return string(out)
}
