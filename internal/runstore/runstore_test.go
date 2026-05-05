package runstore

import "testing"

func TestResolveRunDirAcceptsRunID(t *testing.T) {
	runDir, err := ResolveRunDir("20260428T000000Z-abcdef12")
	if err != nil {
		t.Fatalf("ResolveRunDir returned error: %v", err)
	}
	if runDir != ".byom-video/runs/20260428T000000Z-abcdef12" {
		t.Fatalf("runDir = %q", runDir)
	}
}

func TestResolveRunDirRejectsEscapes(t *testing.T) {
	cases := []string{"../outside", "nested/run", "..", ""}
	for _, runID := range cases {
		if _, err := ResolveRunDir(runID); err == nil {
			t.Fatalf("ResolveRunDir(%q) returned nil error", runID)
		}
	}
}
