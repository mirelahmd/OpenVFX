package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Snapshot struct {
	SnapshotID string    `json:"snapshot_id"`
	CreatedAt  time.Time `json:"created_at"`
	Reason     string    `json:"reason"`
	Plan       Plan      `json:"plan"`
}

func CreateSnapshot(plan Plan, reason string, now time.Time) (Snapshot, error) {
	next, err := nextSnapshotID(plan.PlanID)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{SnapshotID: next, CreatedAt: now.UTC(), Reason: reason, Plan: plan}
	dir := filepath.Join(PlansRoot, plan.PlanID, "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Snapshot{}, fmt.Errorf("create snapshots directory: %w", err)
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode snapshot: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, next+".json"), data, 0o644); err != nil {
		return Snapshot{}, fmt.Errorf("write snapshot: %w", err)
	}
	return snapshot, nil
}

func ListSnapshots(planID string) ([]Snapshot, error) {
	if err := validatePlanID(planID); err != nil {
		return nil, err
	}
	dir := filepath.Join(PlansRoot, planID, "snapshots")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Snapshot{}, nil
		}
		return nil, fmt.Errorf("read snapshots directory: %w", err)
	}
	snapshots := []Snapshot{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		snapshot, err := ReadSnapshot(planID, strings.TrimSuffix(entry.Name(), ".json"))
		if err == nil {
			snapshots = append(snapshots, snapshot)
		}
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].SnapshotID < snapshots[j].SnapshotID
	})
	return snapshots, nil
}

func ReadSnapshot(planID string, snapshotID string) (Snapshot, error) {
	if err := validatePlanID(planID); err != nil {
		return Snapshot{}, err
	}
	if snapshotID == "" || snapshotID != filepath.Base(snapshotID) || strings.Contains(snapshotID, "..") {
		return Snapshot{}, fmt.Errorf("invalid snapshot id %q", snapshotID)
	}
	data, err := os.ReadFile(filepath.Join(PlansRoot, planID, "snapshots", snapshotID+".json"))
	if err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	NormalizePlan(&snapshot.Plan)
	return snapshot, nil
}

func nextSnapshotID(planID string) (string, error) {
	snapshots, err := ListSnapshots(planID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("snapshot_%04d", len(snapshots)+1), nil
}

func validatePlanID(planID string) error {
	if planID == "" || planID != filepath.Base(planID) || strings.Contains(planID, "..") {
		return fmt.Errorf("invalid plan id %q", planID)
	}
	return nil
}
