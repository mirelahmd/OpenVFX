package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotCreateListRead(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := NewPlan(input, "make 5 shorts", GoalOptions{}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePlan(plan); err != nil {
		t.Fatal(err)
	}
	snapshot, err := CreateSnapshot(plan, "test", time.Now().UTC())
	if err != nil {
		t.Fatalf("CreateSnapshot returned error: %v", err)
	}
	if snapshot.SnapshotID != "snapshot_0001" {
		t.Fatalf("snapshot id = %q", snapshot.SnapshotID)
	}
	list, err := ListSnapshots(plan.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Reason != "test" {
		t.Fatalf("snapshots = %#v", list)
	}
	read, err := ReadSnapshot(plan.PlanID, "snapshot_0001")
	if err != nil {
		t.Fatal(err)
	}
	if read.Plan.PlanID != plan.PlanID {
		t.Fatalf("snapshot plan id = %q", read.Plan.PlanID)
	}
}
