package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- parser tests ----

func TestParseRevisionRequest_Platform(t *testing.T) {
	cases := []struct {
		request  string
		platform string
	}{
		{"switch to TikTok", "tiktok"},
		{"make it TikTok", "tiktok"},
		{"tiktok", "tiktok"},
		{"vertical", "tiktok"},
		{"instagram reel", "instagram-reel"},
		{"instagram", "instagram-reel"},
		{"reels", "instagram-reel"},
		{"youtube short", "youtube-short"},
		{"youtube-short", "youtube-short"},
		{"switch to square", "square"},
		{"square format", "square"},
		{"youtube", "youtube"},
		{"make it youtube", "youtube"},
	}
	for _, tc := range cases {
		t.Run(tc.request, func(t *testing.T) {
			actions, err := ParseRevisionRequest(tc.request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actions) != 1 {
				t.Fatalf("expected 1 action, got %d", len(actions))
			}
			a := actions[0]
			if a.Type != RevisionActionSetPlatform {
				t.Fatalf("expected set_platform, got %s", a.Type)
			}
			if a.Params["platform"] != tc.platform {
				t.Fatalf("expected platform %q, got %q", tc.platform, a.Params["platform"])
			}
			if a.RequiresProvider {
				t.Fatal("set_platform should not require provider")
			}
		})
	}
}

func TestParseRevisionRequest_CaptionPosition(t *testing.T) {
	cases := []struct {
		request  string
		position string
	}{
		{"move captions to center", "center"},
		{"captions center", "center"},
		{"move captions to top", "top"},
		{"captions top", "top"},
		{"move captions to bottom", "bottom"},
		{"captions bottom", "bottom"},
		{"set captions to center", "center"},
		{"move subtitle to top", "top"},
	}
	for _, tc := range cases {
		t.Run(tc.request, func(t *testing.T) {
			actions, err := ParseRevisionRequest(tc.request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actions) != 1 {
				t.Fatalf("expected 1 action, got %d", len(actions))
			}
			a := actions[0]
			if a.Type != RevisionActionSetCaptionPosition {
				t.Fatalf("expected set_caption_position, got %s", a.Type)
			}
			if a.Params["position"] != tc.position {
				t.Fatalf("expected position %q, got %q", tc.position, a.Params["position"])
			}
		})
	}
}

func TestParseRevisionRequest_CaptionStyle(t *testing.T) {
	cases := []struct {
		request string
		style   string
	}{
		{"make captions boxed", "boxed"},
		{"boxed captions", "boxed"},
		{"make captions bold", "bold"},
		{"bold captions", "bold"},
		{"default captions", "default"},
		{"reset captions", "default"},
	}
	for _, tc := range cases {
		t.Run(tc.request, func(t *testing.T) {
			actions, err := ParseRevisionRequest(tc.request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actions) != 1 {
				t.Fatalf("expected 1 action, got %d", len(actions))
			}
			a := actions[0]
			if a.Type != RevisionActionSetCaptionStyle {
				t.Fatalf("expected set_caption_style, got %s", a.Type)
			}
			if a.Params["style"] != tc.style {
				t.Fatalf("expected style %q, got %q", tc.style, a.Params["style"])
			}
		})
	}
}

func TestParseRevisionRequest_Script(t *testing.T) {
	cases := []struct {
		request string
		tone    string
	}{
		{"regenerate script", ""},
		{"rewrite script", ""},
		{"make script more cinematic", "cinematic"},
		{"more cinematic script", "cinematic"},
		{"make script funnier", "funny"},
		{"more professional script", "professional"},
	}
	for _, tc := range cases {
		t.Run(tc.request, func(t *testing.T) {
			actions, err := ParseRevisionRequest(tc.request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actions) != 1 {
				t.Fatalf("expected 1 action, got %d", len(actions))
			}
			a := actions[0]
			if a.Type != RevisionActionGenerateScript {
				t.Fatalf("expected generate_script, got %s", a.Type)
			}
			if !a.RequiresProvider {
				t.Fatal("generate_script should require provider")
			}
			if tc.tone != "" && a.Params["tone"] != tc.tone {
				t.Fatalf("expected tone %q, got %q", tc.tone, a.Params["tone"])
			}
		})
	}
}

func TestParseRevisionRequest_Captions(t *testing.T) {
	requests := []string{
		"regenerate captions",
		"new captions",
		"more caption options",
		"caption variants",
	}
	for _, req := range requests {
		t.Run(req, func(t *testing.T) {
			actions, err := ParseRevisionRequest(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actions) != 1 || actions[0].Type != RevisionActionGenerateCaptions {
				t.Fatalf("expected generate_captions, got %+v", actions)
			}
			if !actions[0].RequiresProvider {
				t.Fatal("generate_captions should require provider")
			}
		})
	}
}

func TestParseRevisionRequest_Voiceover(t *testing.T) {
	cases := []struct {
		request    string
		actionType string
		needsProv  bool
	}{
		{"prepare voiceover", RevisionActionPrepareVoiceover, false},
		{"voiceover text", RevisionActionPrepareVoiceover, false},
		{"generate voiceover", RevisionActionGenerateVoiceover, true},
		{"regenerate voiceover", RevisionActionGenerateVoiceover, true},
		{"mix voiceover", RevisionActionMixVoiceover, false},
	}
	for _, tc := range cases {
		t.Run(tc.request, func(t *testing.T) {
			actions, err := ParseRevisionRequest(tc.request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actions) != 1 || actions[0].Type != tc.actionType {
				t.Fatalf("expected %s, got %+v", tc.actionType, actions)
			}
			if actions[0].RequiresProvider != tc.needsProv {
				t.Fatalf("requires_provider: expected %v, got %v", tc.needsProv, actions[0].RequiresProvider)
			}
		})
	}
}

func TestParseRevisionRequest_Reassemble(t *testing.T) {
	requests := []string{
		"reassemble",
		"render again",
		"make new draft",
		"re-assemble",
		"make it shorter",
		"make it longer",
	}
	for _, req := range requests {
		t.Run(req, func(t *testing.T) {
			actions, err := ParseRevisionRequest(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actions) != 1 || actions[0].Type != RevisionActionReassemble {
				t.Fatalf("expected reassemble, got %+v", actions)
			}
		})
	}
}

func TestParseRevisionRequest_Unknown(t *testing.T) {
	_, err := ParseRevisionRequest("do something weird with purple fire")
	if err == nil {
		t.Fatal("expected error for unknown request")
	}
	if !strings.Contains(err.Error(), "could not map revision request") {
		t.Fatalf("error should mention mapping failure, got: %v", err)
	}
	// Should contain examples
	if !strings.Contains(err.Error(), "Platform:") {
		t.Fatalf("error should include examples, got: %v", err)
	}
}

func TestParseRevisionRequest_Empty(t *testing.T) {
	_, err := ParseRevisionRequest("")
	if err == nil {
		t.Fatal("expected error for empty request")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error should mention empty request, got: %v", err)
	}
}

// ---- ReviseMake tests ----

func seedFakeMake(t *testing.T, makeID string, summary MakeSummary) {
	t.Helper()
	makeDir := filepath.Join(makesRoot, makeID)
	_ = os.MkdirAll(makeDir, 0o755)
	_ = writeJSONFile(filepath.Join(makeDir, "make_summary.json"), summary)
}

func TestReviseMake_RequiresRequest(t *testing.T) {
	t.Chdir(t.TempDir())
	err := ReviseMake("any-make", ioDiscard{}, ReviseMakeOptions{})
	if err == nil || !strings.Contains(err.Error(), "--request") {
		t.Fatalf("expected --request error, got: %v", err)
	}
}

func TestReviseMake_NotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	err := ReviseMake("nonexistent-make", ioDiscard{}, ReviseMakeOptions{Request: "reassemble"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got: %v", err)
	}
}

func TestReviseMake_DryRun_WritesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-dry-001"
	seedFakeMake(t, makeID, MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Status:        "completed",
		Goal:          "test goal",
	})

	var out bytes.Buffer
	err := ReviseMake(makeID, &out, ReviseMakeOptions{
		Request: "switch to square",
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// revisions dir should not exist
	revDir := filepath.Join(makesRoot, makeID, makeRevisionsDir)
	if _, err := os.Stat(revDir); err == nil {
		t.Fatal("revisions dir should not be created in dry-run")
	}

	// Output should mention dry-run
	if !strings.Contains(out.String(), "dry-run") {
		t.Fatalf("expected dry-run in output, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "set_platform") {
		t.Fatalf("expected set_platform in dry-run output, got: %s", out.String())
	}
}

func TestReviseMake_PlannedMode_WritesRevisionSummary(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-plan-001"
	seedFakeMake(t, makeID, MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Status:        "completed",
		Goal:          "test goal",
	})

	var out bytes.Buffer
	err := ReviseMake(makeID, &out, ReviseMakeOptions{
		Request: "move captions to center",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// revision_summary.json should exist
	revBase := filepath.Join(makesRoot, makeID, makeRevisionsDir)
	entries, _ := os.ReadDir(revBase)
	if len(entries) == 0 {
		t.Fatal("expected at least one revision directory")
	}
	revDir := filepath.Join(revBase, entries[0].Name())
	data, err := os.ReadFile(filepath.Join(revDir, "revision_summary.json"))
	if err != nil {
		t.Fatalf("revision_summary.json not found: %v", err)
	}
	var rs RevisionSummary
	if err := json.Unmarshal(data, &rs); err != nil {
		t.Fatalf("revision_summary.json malformed: %v", err)
	}
	if rs.Status != "planned" {
		t.Fatalf("expected status=planned, got %s", rs.Status)
	}
	if rs.Request != "move captions to center" {
		t.Fatalf("expected request in summary, got %q", rs.Request)
	}
	if len(rs.PlannedActions) == 0 {
		t.Fatal("expected planned_actions")
	}
	if rs.PlannedActions[0].Type != RevisionActionSetCaptionPosition {
		t.Fatalf("expected set_caption_position, got %s", rs.PlannedActions[0].Type)
	}

	// Output should include next command
	if !strings.Contains(out.String(), "--yes") {
		t.Fatalf("expected --yes in next command output, got: %s", out.String())
	}
}

func TestReviseMake_PlannedMode_WritesSnapshot(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-snap-001"
	summary := MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Status:        "completed",
		Goal:          "test goal",
	}
	seedFakeMake(t, makeID, summary)

	_ = ReviseMake(makeID, ioDiscard{}, ReviseMakeOptions{Request: "reassemble"})

	// before_make_summary.json should exist
	revBase := filepath.Join(makesRoot, makeID, makeRevisionsDir)
	entries, _ := os.ReadDir(revBase)
	if len(entries) == 0 {
		t.Fatal("expected revision directory")
	}
	revDir := filepath.Join(revBase, entries[0].Name())
	if _, err := os.Stat(filepath.Join(revDir, "before_make_summary.json")); err != nil {
		t.Fatal("before_make_summary.json should exist in snapshot")
	}
	// request.txt should exist
	if _, err := os.Stat(filepath.Join(revDir, "request.txt")); err != nil {
		t.Fatal("request.txt should exist in revision dir")
	}
}

func TestReviseMake_NoMediaInSnapshot(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-nomedia-001"
	planID := "test-plan-001"

	// Create a fake plan with draft.mp4
	planOutDir := filepath.Join(creativePlansRoot, planID, "outputs")
	_ = os.MkdirAll(planOutDir, 0o755)
	_ = os.WriteFile(filepath.Join(planOutDir, "draft.mp4"), []byte("fake video"), 0o644)
	_ = os.WriteFile(filepath.Join(planOutDir, "creative_assemble_result.json"), []byte(`{}`), 0o644)

	summary := MakeSummary{
		SchemaVersion:  "make_summary.v1",
		MakeID:         makeID,
		Status:         "completed",
		Goal:           "test goal",
		CreativePlanID: planID,
	}
	seedFakeMake(t, makeID, summary)

	_ = ReviseMake(makeID, ioDiscard{}, ReviseMakeOptions{Request: "reassemble"})

	revBase := filepath.Join(makesRoot, makeID, makeRevisionsDir)
	entries, _ := os.ReadDir(revBase)
	if len(entries) == 0 {
		t.Fatal("expected revision directory")
	}
	revDir := filepath.Join(revBase, entries[0].Name())

	// draft.mp4 should NOT be in snapshot
	if _, err := os.Stat(filepath.Join(revDir, "before_draft.mp4")); err == nil {
		t.Fatal("draft.mp4 should NOT be copied into snapshot")
	}
	// creative_assemble_result.json should be in snapshot
	if _, err := os.Stat(filepath.Join(revDir, "before_creative_assemble_result.json")); err != nil {
		t.Fatal("creative_assemble_result.json should be in snapshot")
	}
}

func TestReviseMake_ProviderActionBlocked(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-prov-001"
	seedFakeMake(t, makeID, MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Status:        "completed",
		Goal:          "test goal",
	})

	err := ReviseMake(makeID, ioDiscard{}, ReviseMakeOptions{
		Request: "regenerate script",
	})
	if err == nil {
		t.Fatal("expected provider-blocked error")
	}
	if !strings.Contains(err.Error(), "--allow-provider-calls") {
		t.Fatalf("error should mention --allow-provider-calls, got: %v", err)
	}
}

func TestReviseMake_Yes_CallsAssemble(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-yes-001"
	planID := "test-plan-yes-001"

	summary := MakeSummary{
		SchemaVersion:  "make_summary.v1",
		MakeID:         makeID,
		Status:         "completed",
		Goal:           "test goal",
		CreativePlanID: planID,
		CaptionStatus:  "applied",
		CaptionPosition: "bottom",
		PlatformPreset: "instagram-reel",
	}
	seedFakeMake(t, makeID, summary)

	assembleCalled := false
	var assembleCalledPlanID string
	var assembleCalledPlatform string

	fakeDeps := revisionDeps{
		assemble: func(pid string, stdout io.Writer, opts CreativeAssembleOptions) error {
			assembleCalled = true
			assembleCalledPlanID = pid
			assembleCalledPlatform = opts.Platform
			return nil
		},
		validate:             func(pid string, stdout io.Writer, opts ValidateCreativeAssembleOptions) error { return nil },
		prepareVoiceoverText: func(pid string, stdout io.Writer, opts VoiceoverTextOptions) error { return nil },
		generateScript:       func(pid string, stdout io.Writer, opts CreativeGenerateScriptOptions) error { return nil },
		generateCaptions:     func(pid string, stdout io.Writer, opts CaptionVariantsOptions) error { return nil },
		generateVoiceover:    func(pid string, stdout io.Writer, opts GenerateVoiceoverOptions) error { return nil },
	}

	err := reviseMakeWithDeps(makeID, ioDiscard{}, ReviseMakeOptions{
		Request:   "switch to square",
		Yes:       true,
		Overwrite: true,
		Reassemble: true,
	}, fakeDeps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !assembleCalled {
		t.Fatal("expected assemble to be called")
	}
	if assembleCalledPlanID != planID {
		t.Fatalf("expected assemble called with plan %q, got %q", planID, assembleCalledPlanID)
	}
	if assembleCalledPlatform != "square" {
		t.Fatalf("expected platform=square, got %q", assembleCalledPlatform)
	}
}

func TestReviseMake_Yes_UpdatesMakeSummary(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-update-001"

	summary := MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Status:        "completed",
		Goal:          "test goal",
	}
	seedFakeMake(t, makeID, summary)

	fakeDeps := revisionDeps{
		assemble:             func(pid string, stdout io.Writer, opts CreativeAssembleOptions) error { return nil },
		validate:             func(pid string, stdout io.Writer, opts ValidateCreativeAssembleOptions) error { return nil },
		prepareVoiceoverText: func(pid string, stdout io.Writer, opts VoiceoverTextOptions) error { return nil },
		generateScript:       func(pid string, stdout io.Writer, opts CreativeGenerateScriptOptions) error { return nil },
		generateCaptions:     func(pid string, stdout io.Writer, opts CaptionVariantsOptions) error { return nil },
		generateVoiceover:    func(pid string, stdout io.Writer, opts GenerateVoiceoverOptions) error { return nil },
	}

	_ = reviseMakeWithDeps(makeID, ioDiscard{}, ReviseMakeOptions{
		Request: "captions center",
		Yes:     true,
	}, fakeDeps)

	// Read back make_summary.json
	data, _ := os.ReadFile(filepath.Join(makesRoot, makeID, "make_summary.json"))
	var updated MakeSummary
	_ = json.Unmarshal(data, &updated)

	if updated.LatestRevisionID == "" {
		t.Fatal("expected latest_revision_id to be set in make_summary")
	}
	if updated.RevisionCount != 1 {
		t.Fatalf("expected revision_count=1, got %d", updated.RevisionCount)
	}
}

func TestReviseMake_Yes_RevisionStatusCompleted(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-status-001"
	planID := "test-plan-status-001"
	seedFakeMake(t, makeID, MakeSummary{
		SchemaVersion:  "make_summary.v1",
		MakeID:         makeID,
		Status:         "completed",
		Goal:           "test goal",
		CreativePlanID: planID,
	})

	fakeDeps := revisionDeps{
		assemble:             func(pid string, stdout io.Writer, opts CreativeAssembleOptions) error { return nil },
		validate:             func(pid string, stdout io.Writer, opts ValidateCreativeAssembleOptions) error { return nil },
		prepareVoiceoverText: func(pid string, stdout io.Writer, opts VoiceoverTextOptions) error { return nil },
		generateScript:       func(pid string, stdout io.Writer, opts CreativeGenerateScriptOptions) error { return nil },
		generateCaptions:     func(pid string, stdout io.Writer, opts CaptionVariantsOptions) error { return nil },
		generateVoiceover:    func(pid string, stdout io.Writer, opts GenerateVoiceoverOptions) error { return nil },
	}

	_ = reviseMakeWithDeps(makeID, ioDiscard{}, ReviseMakeOptions{
		Request: "reassemble",
		Yes:     true,
	}, fakeDeps)

	revBase := filepath.Join(makesRoot, makeID, makeRevisionsDir)
	entries, _ := os.ReadDir(revBase)
	if len(entries) == 0 {
		t.Fatal("expected revision directory")
	}
	data, _ := os.ReadFile(filepath.Join(revBase, entries[0].Name(), "revision_summary.json"))
	var rs RevisionSummary
	_ = json.Unmarshal(data, &rs)
	if rs.Status != "completed" {
		t.Fatalf("expected status=completed, got %s", rs.Status)
	}
}

func TestReviseMake_Yes_ProviderWithFallbackStub(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-stub-001"
	planID := "test-plan-stub-001"

	summary := MakeSummary{
		SchemaVersion:  "make_summary.v1",
		MakeID:         makeID,
		Status:         "completed",
		Goal:           "test goal",
		CreativePlanID: planID,
	}
	seedFakeMake(t, makeID, summary)

	scriptCalled := false
	fakeDeps := revisionDeps{
		assemble:             func(pid string, stdout io.Writer, opts CreativeAssembleOptions) error { return nil },
		validate:             func(pid string, stdout io.Writer, opts ValidateCreativeAssembleOptions) error { return nil },
		prepareVoiceoverText: func(pid string, stdout io.Writer, opts VoiceoverTextOptions) error { return nil },
		generateScript: func(pid string, stdout io.Writer, opts CreativeGenerateScriptOptions) error {
			scriptCalled = true
			if !opts.FallbackStub {
				return fmt.Errorf("expected FallbackStub=true")
			}
			return nil
		},
		generateCaptions:  func(pid string, stdout io.Writer, opts CaptionVariantsOptions) error { return nil },
		generateVoiceover: func(pid string, stdout io.Writer, opts GenerateVoiceoverOptions) error { return nil },
	}

	err := reviseMakeWithDeps(makeID, ioDiscard{}, ReviseMakeOptions{
		Request:      "regenerate script",
		Yes:          true,
		FallbackStub: true,
	}, fakeDeps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scriptCalled {
		t.Fatal("expected generateScript to be called")
	}
}

// ---- MakeRevisions tests ----

func TestMakeRevisions_Empty(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-revisions-empty"
	_ = os.MkdirAll(filepath.Join(makesRoot, makeID), 0o755)

	var out bytes.Buffer
	err := MakeRevisions(makeID, &out, MakeRevisionsOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "No revisions") {
		t.Fatalf("expected 'No revisions', got: %s", out.String())
	}
}

func TestMakeRevisions_ListsRevisions(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-revisions-list"
	planID := "test-plan-list"

	summary := MakeSummary{
		SchemaVersion:  "make_summary.v1",
		MakeID:         makeID,
		Status:         "completed",
		Goal:           "test",
		CreativePlanID: planID,
	}
	seedFakeMake(t, makeID, summary)

	// Create two revisions via planned mode
	_ = ReviseMake(makeID, ioDiscard{}, ReviseMakeOptions{Request: "switch to square"})
	_ = ReviseMake(makeID, ioDiscard{}, ReviseMakeOptions{Request: "captions center"})

	var out bytes.Buffer
	if err := MakeRevisions(makeID, &out, MakeRevisionsOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "revision_0001") {
		t.Fatalf("expected revision_0001 in output, got: %s", text)
	}
	if !strings.Contains(text, "revision_0002") {
		t.Fatalf("expected revision_0002 in output, got: %s", text)
	}
}

// ---- InspectMakeRevision tests ----

func TestInspectMakeRevision_NotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	err := InspectMakeRevision("no-make", "revision_0001", ioDiscard{}, InspectMakeRevisionOptions{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got: %v", err)
	}
}

func TestInspectMakeRevision_ReadsJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-inspect-rev"
	seedFakeMake(t, makeID, MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Status:        "completed",
	})

	_ = ReviseMake(makeID, ioDiscard{}, ReviseMakeOptions{Request: "reassemble"})

	revBase := filepath.Join(makesRoot, makeID, makeRevisionsDir)
	entries, _ := os.ReadDir(revBase)
	revID := entries[0].Name()

	var out bytes.Buffer
	err := InspectMakeRevision(makeID, revID, &out, InspectMakeRevisionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), revID) {
		t.Fatalf("expected revision ID in output, got: %s", out.String())
	}
}

func TestInspectMakeRevision_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-inspect-rev-json"
	seedFakeMake(t, makeID, MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Status:        "completed",
	})

	_ = ReviseMake(makeID, ioDiscard{}, ReviseMakeOptions{Request: "reassemble"})

	revBase := filepath.Join(makesRoot, makeID, makeRevisionsDir)
	entries, _ := os.ReadDir(revBase)
	revID := entries[0].Name()

	var out bytes.Buffer
	_ = InspectMakeRevision(makeID, revID, &out, InspectMakeRevisionOptions{JSON: true})

	var rs RevisionSummary
	if err := json.Unmarshal(out.Bytes(), &rs); err != nil {
		t.Fatalf("JSON output malformed: %v\noutput: %s", err, out.String())
	}
	if rs.SchemaVersion != "make_revision.v1" {
		t.Fatalf("expected schema_version make_revision.v1, got %s", rs.SchemaVersion)
	}
}

// ---- ReviewMakeRevision tests ----

func TestReviewMakeRevision_WritesMarkdown(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-review-rev"
	seedFakeMake(t, makeID, MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Status:        "completed",
	})

	_ = ReviseMake(makeID, ioDiscard{}, ReviseMakeOptions{Request: "switch to square"})

	revBase := filepath.Join(makesRoot, makeID, makeRevisionsDir)
	entries, _ := os.ReadDir(revBase)
	revID := entries[0].Name()

	var out bytes.Buffer
	err := ReviewMakeRevision(makeID, revID, &out, ReviewMakeRevisionOptions{WriteArtifact: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have written the MD file
	mdPath := filepath.Join(makesRoot, makeID, makeRevisionsDir, revID, "revision_review.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("revision_review.md not written: %v", err)
	}
	// Content should contain markdown headers
	data, _ := os.ReadFile(mdPath)
	if !strings.Contains(string(data), "# Make Revision Review") {
		t.Fatalf("expected markdown header, got: %s", string(data))
	}
}

// ---- MakeSummary revision field integration ----

func TestMakeResult_ShowsLatestRevision(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-result-rev"
	summary := MakeSummary{
		SchemaVersion:    "make_summary.v1",
		MakeID:           makeID,
		Status:           "completed",
		Goal:             "test goal",
		LatestRevisionID: "revision_0001",
		RevisionCount:    1,
		RevisionStatus:   "completed",
		NextCommands:     []string{},
	}
	seedFakeMake(t, makeID, summary)

	var out bytes.Buffer
	_ = MakeResult(makeID, &out, MakeResultOptions{})
	text := out.String()
	if !strings.Contains(text, "revision_0001") {
		t.Fatalf("expected revision_0001 in make-result output, got: %s", text)
	}
}

func TestInspectMake_ShowsRevisionCount(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-inspect-make-rev"
	summary := MakeSummary{
		SchemaVersion:    "make_summary.v1",
		MakeID:           makeID,
		Status:           "completed",
		Goal:             "test goal",
		LatestRevisionID: "revision_0002",
		RevisionCount:    2,
		RevisionStatus:   "completed",
		NextCommands:     []string{},
	}
	seedFakeMake(t, makeID, summary)

	var out bytes.Buffer
	_ = InspectMake(makeID, &out, InspectMakeOptions{})
	text := out.String()
	if !strings.Contains(text, "revision_0002") {
		t.Fatalf("expected revision_0002 in inspect-make output, got: %s", text)
	}
	if !strings.Contains(text, "2") {
		t.Fatalf("expected revision count in inspect-make output, got: %s", text)
	}
}

// ---- nextRevisionID tests ----

func TestNextRevisionID_StartsAtOne(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-revid-start"
	_ = os.MkdirAll(filepath.Join(makesRoot, makeID), 0o755)

	id, err := nextRevisionID(makeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "revision_0001" {
		t.Fatalf("expected revision_0001, got %s", id)
	}
}

func TestNextRevisionID_Increments(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-revid-incr"
	revDir := filepath.Join(makesRoot, makeID, makeRevisionsDir, "revision_0003")
	_ = os.MkdirAll(revDir, 0o755)

	id, err := nextRevisionID(makeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "revision_0004" {
		t.Fatalf("expected revision_0004, got %s", id)
	}
}

// ---- buildReassembleOpts tests ----

func TestBuildReassembleOpts_InheritsFromSummary(t *testing.T) {
	state := &execRevisionState{
		Platform:        "tiktok",
		CaptionPosition: "",
		CaptionStyle:    "",
		BurnCaptions:    true,
	}
	summary := MakeSummary{
		PlatformPreset:  "instagram-reel",
		CaptionPosition: "bottom",
		CaptionStyle:    "bold",
		CaptionMargin:   120,
	}
	opts := buildReassembleOpts(state, summary, true)
	if opts.Platform != "tiktok" {
		t.Fatalf("expected tiktok (from state), got %s", opts.Platform)
	}
	if opts.CaptionPosition != "bottom" {
		t.Fatalf("expected bottom (from summary), got %s", opts.CaptionPosition)
	}
	if opts.CaptionStyle != "bold" {
		t.Fatalf("expected bold (from summary), got %s", opts.CaptionStyle)
	}
	if !opts.BurnCaptions {
		t.Fatal("expected BurnCaptions=true")
	}
	if !opts.AllowMissingCaptions {
		t.Fatal("expected AllowMissingCaptions=true")
	}
}

func TestBuildReassembleOpts_StateOverridesSummary(t *testing.T) {
	state := &execRevisionState{
		Platform:        "square",
		CaptionPosition: "center",
		CaptionStyle:    "boxed",
	}
	summary := MakeSummary{
		PlatformPreset:  "tiktok",
		CaptionPosition: "bottom",
		CaptionStyle:    "default",
	}
	opts := buildReassembleOpts(state, summary, false)
	if opts.Platform != "square" {
		t.Fatalf("expected square, got %s", opts.Platform)
	}
	if opts.CaptionPosition != "center" {
		t.Fatalf("expected center, got %s", opts.CaptionPosition)
	}
	if opts.CaptionStyle != "boxed" {
		t.Fatalf("expected boxed, got %s", opts.CaptionStyle)
	}
}

