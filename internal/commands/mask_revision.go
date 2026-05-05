package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"byom-video/internal/events"
	"byom-video/internal/runstore"
)

type ReviseMaskOptions struct {
	Request  string
	DryRun   bool
	JSON     bool
	ShowDiff bool
}

type ReviseMaskResult struct {
	RunID      string   `json:"run_id"`
	Request    string   `json:"request"`
	Applied    bool     `json:"applied"`
	DryRun     bool     `json:"dry_run"`
	Changes    []string `json:"changes"`
	SnapshotID string   `json:"snapshot_id,omitempty"`
}

type MaskSnapshotsOptions struct{ JSON bool }
type InspectMaskSnapshotOptions struct{ JSON bool }

type DiffMaskOptions struct {
	JSON          bool
	WriteArtifact bool
}

type MaskDiff struct {
	RunID      string       `json:"run_id"`
	SnapshotID string       `json:"snapshot_id"`
	Changes    []DiffChange `json:"changes"`
}

type DiffChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

type MaskSnapshotEntry struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

func ReviseMask(runID string, stdout io.Writer, opts ReviseMaskOptions) error {
	if strings.TrimSpace(opts.Request) == "" {
		return fmt.Errorf("--request is required")
	}

	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
	}

	maskPath := filepath.Join(runDir, "inference_mask.json")
	original, err := readInferenceMask(maskPath)
	if err != nil {
		writeMaskFailure(log, "MASK_REVISION_FAILED", err.Error())
		return err
	}

	revised := deepCopyMask(original)
	changes, err := applyMaskRevision(&revised, opts.Request)
	if err != nil {
		writeMaskFailure(log, "MASK_REVISION_FAILED", err.Error())
		return err
	}

	result := ReviseMaskResult{
		RunID:   runID,
		Request: opts.Request,
		DryRun:  opts.DryRun,
		Applied: !opts.DryRun,
		Changes: changes,
	}

	if !opts.DryRun {
		snapshotID, err := createMaskSnapshot(runDir, original)
		if err != nil {
			writeMaskFailure(log, "MASK_REVISION_FAILED", err.Error())
			return err
		}
		result.SnapshotID = snapshotID

		if err := writeJSONFile(maskPath, revised); err != nil {
			writeMaskFailure(log, "MASK_REVISION_FAILED", err.Error())
			return err
		}

		if log != nil {
			_ = log.Write("MASK_REVISED", map[string]any{
				"request":     opts.Request,
				"changes":     len(changes),
				"snapshot_id": snapshotID,
			})
		}
	}

	if opts.JSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	if opts.DryRun {
		fmt.Fprintln(stdout, "Mask revision (dry run)")
	} else {
		fmt.Fprintln(stdout, "Mask revised")
	}
	fmt.Fprintf(stdout, "  run id:  %s\n", runID)
	fmt.Fprintf(stdout, "  request: %s\n", opts.Request)
	fmt.Fprintf(stdout, "  changes: %d\n", len(changes))
	for _, c := range changes {
		fmt.Fprintf(stdout, "  - %s\n", c)
	}
	if result.SnapshotID != "" {
		fmt.Fprintf(stdout, "  snapshot: %s\n", result.SnapshotID)
	}

	if opts.ShowDiff {
		snapshotID := result.SnapshotID
		if snapshotID == "" {
			snapshotID = "proposed"
		}
		diff := buildMaskDiff(runID, snapshotID, original, revised)
		printMaskDiff(stdout, diff)
	}

	return nil
}

func deepCopyMask(mask InferenceMask) InferenceMask {
	data, _ := json.Marshal(mask)
	var copy InferenceMask
	_ = json.Unmarshal(data, &copy)
	return copy
}

func applyMaskRevision(mask *InferenceMask, request string) ([]string, error) {
	req := strings.ToLower(strings.TrimSpace(request))
	changes := []string{}

	switch {
	case req == "make captions shorter":
		before := mask.Constraints.MaxCaptionWords
		newVal := before - 3
		if newVal < 5 {
			newVal = 5
		}
		changes = append(changes, fmt.Sprintf("max_caption_words: %d → %d", before, newVal))
		mask.Constraints.MaxCaptionWords = newVal

	case req == "make captions longer":
		before := mask.Constraints.MaxCaptionWords
		newVal := before + 3
		if newVal > 40 {
			newVal = 40
		}
		changes = append(changes, fmt.Sprintf("max_caption_words: %d → %d", before, newVal))
		mask.Constraints.MaxCaptionWords = newVal

	case strings.HasPrefix(req, "set captions to ") && strings.HasSuffix(req, " words"):
		middle := strings.TrimPrefix(req, "set captions to ")
		middle = strings.TrimSuffix(middle, " words")
		n, err := strconv.Atoi(strings.TrimSpace(middle))
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid revision request: could not parse word count from %q", request)
		}
		changes = append(changes, fmt.Sprintf("max_caption_words: %d → %d", mask.Constraints.MaxCaptionWords, n))
		mask.Constraints.MaxCaptionWords = n

	case req == "make tone more technical":
		if !strings.Contains(mask.Constraints.Tone, "technical") {
			before := mask.Constraints.Tone
			if before == "" {
				mask.Constraints.Tone = "technical"
			} else {
				mask.Constraints.Tone = before + ", technical"
			}
			changes = append(changes, fmt.Sprintf("tone: %q → %q", before, mask.Constraints.Tone))
		}

	case req == "make tone more casual":
		if !strings.Contains(mask.Constraints.Tone, "casual") {
			before := mask.Constraints.Tone
			if before == "" {
				mask.Constraints.Tone = "casual"
			} else {
				mask.Constraints.Tone = before + ", casual"
			}
			changes = append(changes, fmt.Sprintf("tone: %q → %q", before, mask.Constraints.Tone))
		}

	case req == "avoid hype":
		if addToStringSlice(&mask.Constraints.MustNotInclude, "hype") {
			changes = append(changes, `must_not_include: added "hype"`)
		}
		if addToStringSlice(&mask.Constraints.MustNotInclude, "exaggerated claims") {
			changes = append(changes, `must_not_include: added "exaggerated claims"`)
		}

	case req == "avoid unsupported claims":
		if addToStringSlice(&mask.Constraints.MustNotInclude, "unsupported claims") {
			changes = append(changes, `must_not_include: added "unsupported claims"`)
		}

	case req == "require hook":
		if addToStringSlice(&mask.Constraints.MustInclude, "strong hook") {
			changes = append(changes, `must_include: added "strong hook"`)
		}

	default:
		return nil, fmt.Errorf("unknown revision request %q; supported: make captions shorter, make captions longer, set captions to N words, make tone more technical, make tone more casual, avoid hype, avoid unsupported claims, require hook", request)
	}

	return changes, nil
}

func addToStringSlice(slice *[]string, value string) bool {
	for _, existing := range *slice {
		if existing == value {
			return false
		}
	}
	*slice = append(*slice, value)
	return true
}

func createMaskSnapshot(runDir string, mask InferenceMask) (string, error) {
	snapDir := filepath.Join(runDir, "mask_snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return "", fmt.Errorf("create snapshot dir: %w", err)
	}
	entries, _ := os.ReadDir(snapDir)
	n := len(entries) + 1
	id := fmt.Sprintf("mask_snapshot_%04d", n)
	path := filepath.Join(snapDir, id+".json")
	if err := writeJSONFile(path, mask); err != nil {
		return "", err
	}
	return id, nil
}

func MaskSnapshots(runID string, stdout io.Writer, opts MaskSnapshotsOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	snapDir := filepath.Join(runDir, "mask_snapshots")
	dirEntries, readErr := os.ReadDir(snapDir)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			if opts.JSON {
				fmt.Fprintln(stdout, "[]")
			} else {
				fmt.Fprintln(stdout, "No mask snapshots found")
			}
			return nil
		}
		return fmt.Errorf("read snapshots: %w", readErr)
	}

	snapshots := []MaskSnapshotEntry{}
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(de.Name(), ".json")
		info, _ := de.Info()
		entry := MaskSnapshotEntry{
			ID:   id,
			Path: filepath.Join(snapDir, de.Name()),
		}
		if info != nil {
			entry.CreatedAt = info.ModTime()
		}
		snapshots = append(snapshots, entry)
	}

	if opts.JSON {
		data, err := json.MarshalIndent(snapshots, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	if len(snapshots) == 0 {
		fmt.Fprintln(stdout, "No mask snapshots found")
		return nil
	}
	fmt.Fprintf(stdout, "%-30s %s\n", "SNAPSHOT", "CREATED AT")
	for _, s := range snapshots {
		fmt.Fprintf(stdout, "%-30s %s\n", s.ID, s.CreatedAt.Format(time.RFC3339))
	}
	return nil
}

func InspectMaskSnapshot(runID string, snapshotID string, stdout io.Writer, opts InspectMaskSnapshotOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	path := filepath.Join(runDir, "mask_snapshots", snapshotID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read snapshot %s: %w", snapshotID, err)
	}
	if opts.JSON {
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	var mask InferenceMask
	if err := json.Unmarshal(data, &mask); err != nil {
		return fmt.Errorf("decode snapshot: %w", err)
	}
	fmt.Fprintln(stdout, "Mask snapshot")
	fmt.Fprintf(stdout, "  run_id:      %s\n", runID)
	fmt.Fprintf(stdout, "  snapshot_id: %s\n", snapshotID)
	fmt.Fprintf(stdout, "  intent:      %s\n", mask.Intent)
	fmt.Fprintf(stdout, "  tone:        %s\n", mask.Constraints.Tone)
	fmt.Fprintf(stdout, "  max words:   %d\n", mask.Constraints.MaxCaptionWords)
	fmt.Fprintf(stdout, "  decisions:   %d\n", len(mask.Decisions))
	return nil
}

func DiffMask(runID string, snapshotID string, stdout io.Writer, opts DiffMaskOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	current, err := readInferenceMask(filepath.Join(runDir, "inference_mask.json"))
	if err != nil {
		return err
	}
	snapPath := filepath.Join(runDir, "mask_snapshots", snapshotID+".json")
	snapshot, err := readInferenceMask(snapPath)
	if err != nil {
		return fmt.Errorf("read snapshot %s: %w", snapshotID, err)
	}

	diff := buildMaskDiff(runID, snapshotID, snapshot, current)

	if opts.JSON {
		data, err := json.MarshalIndent(diff, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	printMaskDiff(stdout, diff)

	if opts.WriteArtifact {
		diffDir := filepath.Join(runDir, "mask_diffs")
		if err := os.MkdirAll(diffDir, 0o755); err != nil {
			return fmt.Errorf("create diff dir: %w", err)
		}
		name := fmt.Sprintf("diff_current_vs_%s.md", snapshotID)
		path := filepath.Join(diffDir, name)
		if err := writeMaskDiffArtifact(path, diff); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "  artifact: %s\n", path)
	}

	return nil
}

func buildMaskDiff(runID, snapshotID string, from, to InferenceMask) MaskDiff {
	diff := MaskDiff{RunID: runID, SnapshotID: snapshotID, Changes: []DiffChange{}}

	if from.Intent != to.Intent {
		diff.Changes = append(diff.Changes, DiffChange{Field: "intent", From: from.Intent, To: to.Intent})
	}
	if from.Constraints.Tone != to.Constraints.Tone {
		diff.Changes = append(diff.Changes, DiffChange{
			Field: "constraints.tone",
			From:  from.Constraints.Tone,
			To:    to.Constraints.Tone,
		})
	}
	if from.Constraints.MaxCaptionWords != to.Constraints.MaxCaptionWords {
		diff.Changes = append(diff.Changes, DiffChange{
			Field: "constraints.max_caption_words",
			From:  fmt.Sprintf("%d", from.Constraints.MaxCaptionWords),
			To:    fmt.Sprintf("%d", to.Constraints.MaxCaptionWords),
		})
	}
	fromMustInclude := strings.Join(from.Constraints.MustInclude, ", ")
	toMustInclude := strings.Join(to.Constraints.MustInclude, ", ")
	if fromMustInclude != toMustInclude {
		diff.Changes = append(diff.Changes, DiffChange{
			Field: "constraints.must_include",
			From:  fromMustInclude,
			To:    toMustInclude,
		})
	}
	fromMustNot := strings.Join(from.Constraints.MustNotInclude, ", ")
	toMustNot := strings.Join(to.Constraints.MustNotInclude, ", ")
	if fromMustNot != toMustNot {
		diff.Changes = append(diff.Changes, DiffChange{
			Field: "constraints.must_not_include",
			From:  fromMustNot,
			To:    toMustNot,
		})
	}
	if len(from.Decisions) != len(to.Decisions) {
		diff.Changes = append(diff.Changes, DiffChange{
			Field: "decisions.count",
			From:  fmt.Sprintf("%d", len(from.Decisions)),
			To:    fmt.Sprintf("%d", len(to.Decisions)),
		})
	}
	return diff
}

func printMaskDiff(stdout io.Writer, diff MaskDiff) {
	fmt.Fprintln(stdout, "Mask diff")
	fmt.Fprintf(stdout, "  run id:   %s\n", diff.RunID)
	fmt.Fprintf(stdout, "  snapshot: %s\n", diff.SnapshotID)
	fmt.Fprintf(stdout, "  changes:  %d\n", len(diff.Changes))
	for _, c := range diff.Changes {
		fmt.Fprintf(stdout, "  - %s: %q → %q\n", c.Field, c.From, c.To)
	}
	if len(diff.Changes) == 0 {
		fmt.Fprintln(stdout, "  (no changes)")
	}
}

func writeMaskDiffArtifact(path string, diff MaskDiff) error {
	var builder strings.Builder
	builder.WriteString("# Mask Diff\n\n")
	builder.WriteString(fmt.Sprintf("- generated_at: %s\n", time.Now().UTC().Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- run_id: %s\n", diff.RunID))
	builder.WriteString(fmt.Sprintf("- snapshot: %s\n", diff.SnapshotID))
	builder.WriteString(fmt.Sprintf("- changes: %d\n", len(diff.Changes)))
	builder.WriteString("\n## Changes\n\n")
	for _, c := range diff.Changes {
		builder.WriteString(fmt.Sprintf("- **%s**: `%s` → `%s`\n", c.Field, c.From, c.To))
	}
	if len(diff.Changes) == 0 {
		builder.WriteString("_(no changes)_\n")
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write mask diff: %w", err)
	}
	return nil
}
