package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/editorartifacts"
	"github.com/mirelahmd/OpenVFX/internal/manifest"
)

func TestClipCardsFromRoughcutOnly(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)

	if err := ClipCardsCommand(runID, &bytes.Buffer{}, ClipCardsOptions{}); err != nil {
		t.Fatal(err)
	}

	doc, err := editorartifacts.ReadClipCards(filepath.Join(runDir, "clip_cards.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Cards) != 1 {
		t.Fatalf("cards = %d", len(doc.Cards))
	}
	if doc.Cards[0].Title == "" || doc.Cards[0].Description == "" {
		t.Fatalf("unexpected card: %+v", doc.Cards[0])
	}
}

func TestClipCardsWithCaptionVariants(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)
	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := ClipCardsCommand(runID, &bytes.Buffer{}, ClipCardsOptions{}); err != nil {
		t.Fatal(err)
	}

	doc, err := editorartifacts.ReadClipCards(filepath.Join(runDir, "clip_cards.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Cards[0].Captions) == 0 {
		t.Fatalf("expected captions on card: %+v", doc.Cards[0])
	}
}

func TestClipCardsWithLabelsAndDescriptions(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)
	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := ClipCardsCommand(runID, &bytes.Buffer{}, ClipCardsOptions{}); err != nil {
		t.Fatal(err)
	}

	doc, err := editorartifacts.ReadClipCards(filepath.Join(runDir, "clip_cards.json"))
	if err != nil {
		t.Fatal(err)
	}
	card := doc.Cards[0]
	if !strings.Contains(card.Title, "Label:") {
		t.Fatalf("expected label-derived title, got %+v", card)
	}
	if !strings.Contains(card.Description, "Stub description") {
		t.Fatalf("expected description-derived card, got %+v", card)
	}
}

func TestClipCardsIncludesVerificationWarnings(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)
	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	capPath := filepath.Join(runDir, "expansions", "caption_variants.json")
	output := readExpansionOutput(t, capPath)
	output.Items[0].Text = "unsupported claims in caption"
	if err := writeJSONFile(capPath, output); err != nil {
		t.Fatal(err)
	}
	if err := VerificationPlanCommand(runID, &bytes.Buffer{}, VerificationPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	_ = VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{})

	if err := ClipCardsCommand(runID, &bytes.Buffer{}, ClipCardsOptions{}); err != nil {
		t.Fatal(err)
	}
	doc, err := editorartifacts.ReadClipCards(filepath.Join(runDir, "clip_cards.json"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Cards[0].VerificationStatus != "failed" {
		t.Fatalf("verification_status = %q", doc.Cards[0].VerificationStatus)
	}
	if len(doc.Cards[0].Warnings) == 0 {
		t.Fatalf("expected verification warnings: %+v", doc.Cards[0])
	}
}

func TestClipCardsRefusesOverwrite(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := ClipCardsCommand(runID, &bytes.Buffer{}, ClipCardsOptions{}); err != nil {
		t.Fatal(err)
	}
	err := ClipCardsCommand(runID, &bytes.Buffer{}, ClipCardsOptions{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected overwrite error, got %v", err)
	}
}

func TestReviewClipsWritesMarkdown(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := ClipCardsCommand(runID, &bytes.Buffer{}, ClipCardsOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ReviewClips(runID, &bytes.Buffer{}, ReviewClipsOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "clip_cards_review.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Clip Cards Review") {
		t.Fatalf("unexpected markdown: %s", string(data))
	}
}

func TestEnhanceRoughcutFromClipCards(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)
	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ClipCardsCommand(runID, &bytes.Buffer{}, ClipCardsOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := EnhanceRoughcut(runID, &bytes.Buffer{}, EnhanceRoughcutOptions{}); err != nil {
		t.Fatal(err)
	}
	doc, err := editorartifacts.ReadEnhancedRoughcut(filepath.Join(runDir, "enhanced_roughcut.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Clips) != 1 {
		t.Fatalf("clips = %d", len(doc.Clips))
	}
	if !strings.Contains(doc.Clips[0].Title, "Label:") {
		t.Fatalf("expected clip card title, got %+v", doc.Clips[0])
	}
}

func TestEnhanceRoughcutFallbackFromRoughcutOnly(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := EnhanceRoughcut(runID, &bytes.Buffer{}, EnhanceRoughcutOptions{}); err != nil {
		t.Fatal(err)
	}
	doc, err := editorartifacts.ReadEnhancedRoughcut(filepath.Join(runDir, "enhanced_roughcut.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Clips) != 1 {
		t.Fatalf("clips = %d", len(doc.Clips))
	}
	if doc.Clips[0].Title == "" || doc.Clips[0].Description == "" {
		t.Fatalf("unexpected clip: %+v", doc.Clips[0])
	}
}

func TestValidateCatchesInvalidClipCardsTiming(t *testing.T) {
	t.Chdir(t.TempDir())
	runDir := writeCommandRunManifest(t, "run-1")
	writeEventsForValidation(t, runDir)
	doc := editorartifacts.ClipCards{
		SchemaVersion: "clip_cards.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         "run-1",
		Source: editorartifacts.ClipCardsSource{
			RoughcutArtifact: "roughcut.json",
		},
		Cards: []editorartifacts.ClipCard{
			{
				ID:                 "card_0001",
				ClipID:             "clip_0001",
				Start:              10,
				End:                2,
				DurationSeconds:    0,
				Title:              "Bad",
				Description:        "Bad timing",
				VerificationStatus: "unknown",
			},
		},
	}
	if err := writeJSONFile(filepath.Join(runDir, "clip_cards.json"), doc); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.AddArtifact("events", "events.jsonl")
	m.AddArtifact("clip_cards", "clip_cards.json")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = Validate("run-1", &out, ValidateOptions{})
	if err == nil || !strings.Contains(out.String(), "clip_cards.json") {
		t.Fatalf("expected clip_cards validation failure, err=%v out=%s", err, out.String())
	}
}

func TestInspectShowsClipCardCounts(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)
	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ClipCardsCommand(runID, &bytes.Buffer{}, ClipCardsOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := EnhanceRoughcut(runID, &bytes.Buffer{}, EnhanceRoughcutOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Inspect(runID, &out, InspectOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "clip cards:") || !strings.Contains(text, "enhanced roughcut:") {
		t.Fatalf("inspect output missing new counts: %s", text)
	}
}

func writeEventsForValidation(t *testing.T, runDir string) {
	t.Helper()
	content := `{"type":"RUN_STARTED","time":"2026-04-28T00:00:00Z"}` + "\n" + `{"type":"RUN_COMPLETED","time":"2026-04-28T00:00:01Z"}` + "\n"
	if err := os.WriteFile(filepath.Join(runDir, "events.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClipCardsJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	var out bytes.Buffer
	if err := ClipCardsCommand(runID, &out, ClipCardsOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var summary ClipCardsSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if summary.Count != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}
