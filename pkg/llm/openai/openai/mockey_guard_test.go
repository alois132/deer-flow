package openai

import (
	"os"
	"testing"
)

func requireMockeyTests(t *testing.T) {
	t.Helper()
	// Monkey patch tests require special compiler flags on many Go/arch combos.
	// Keep default `go test ./...` stable, and run these tests only when explicitly enabled.
	if os.Getenv("ENABLE_MOCKEY_TESTS") != "1" {
		t.Skip("set ENABLE_MOCKEY_TESTS=1 to run mockey-based tests")
	}
}
