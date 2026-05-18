package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentPlanDryRunWritesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	err := AgentPlanCommand(&out, AgentPlanCommandOptions{
		Goal:   "queue health",
		DryRun: true,
		JSON:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(agentPlansV1Root); !os.IsNotExist(err) {
		t.Fatalf("agent_plans root should not exist")
	}
}

func TestAgentPlanWithInputCreatesMakeAction(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      "make a vertical short with captions and narration",
		InputPath: input,
	}); err != nil {
		t.Fatal(err)
	}
	plan := readLatestAgentPlan(t)
	if len(plan.Actions) != 1 || plan.Actions[0].Type != agentActionTypeMake {
		t.Fatalf("actions = %#v", plan.Actions)
	}
}

func TestAgentPlanWritesCreativeBriefDeliverablesAndAssets(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	goal := "Make a 35-second luxury fitness Instagram Reel. Use my talking clip as narration and gym clips as b-roll. Make it cinematic, darker, premium, fast cuts in the first 5 seconds. Generate futuristic gym city b-roll if the backend exists. Use bold lower-third captions. Give me Instagram caption options."
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      goal,
		InputPath: input,
	}); err != nil {
		t.Fatal(err)
	}
	planID := latestAgentPlanID(t)
	brief, err := readCreativeBrief(planID)
	if err != nil {
		t.Fatal(err)
	}
	if brief.Duration.TargetSeconds != 35 || brief.Platform != "instagram-reel" {
		t.Fatalf("brief = %#v", brief)
	}
	if brief.Captions.Style != "bold" || brief.Captions.Position != "lower-third" {
		t.Fatalf("captions = %#v", brief.Captions)
	}
	if !brief.Requests.GeneratedBRoll || !brief.Requests.InstagramCaptions || !brief.Requests.UseTalkingClipNarration {
		t.Fatalf("requests = %#v", brief.Requests)
	}
	deliverables, err := readDeliverables(planID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deliverables.Deliverables) < 2 {
		t.Fatalf("deliverables = %#v", deliverables.Deliverables)
	}
	assets, err := readAssetRequirements(planID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets.Requirements) == 0 {
		t.Fatalf("expected asset requirements")
	}
	plan := readLatestAgentPlan(t)
	action := plan.Actions[0]
	if !strings.Contains(action.Description, "35-second") {
		t.Fatalf("description = %q", action.Description)
	}
	if inputString(action.Input, "creative_brief_ref") != creativeBriefArtifact {
		t.Fatalf("action input = %#v", action.Input)
	}
}

func TestAgentPlanWritesVisualRequestsDryRunForGeneratedBRoll(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	goal := "make a cinematic reel and generate futuristic gym city b-roll with darker lighting"
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      goal,
		InputPath: input,
	}); err != nil {
		t.Fatal(err)
	}
	planID := latestAgentPlanID(t)
	plan := readLatestAgentPlan(t)
	if plan.References.VisualRequests != visualRequestsArtifact {
		t.Fatalf("visual reference = %#v", plan.References)
	}
	artifact, err := readVisualRequestsDryRun(planID)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.SchemaVersion != visualRequestsDryRunSchema || len(artifact.Requests) == 0 {
		t.Fatalf("artifact = %#v", artifact)
	}
	foundMissing := false
	for _, req := range artifact.Requests {
		if req.Status == "missing_backend" {
			foundMissing = true
		}
		if req.RequestPreview["no_provider_calls"] != true {
			t.Fatalf("request should be dry-run only: %#v", req.RequestPreview)
		}
	}
	if !foundMissing {
		t.Fatalf("expected missing backend without tools routes: %#v", artifact.Requests)
	}
}

func TestVisualRequestsResolveConfiguredBackendAndHideSecrets(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("byom-video.yaml", []byte(`
tools:
  enabled: true
  backends:
    local_video:
      kind: video_generation
      provider: custom-http
      model: visual-test-model
      endpoint: http://localhost:9999/generate
      auth:
        type: bearer_env
        env: VISUAL_API_KEY
  routes:
    creative.broll_generate: local_video
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL_API_KEY", "super-secret")
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      "make a reel and generate AI b-roll",
		InputPath: input,
	}); err != nil {
		t.Fatal(err)
	}
	planID := latestAgentPlanID(t)
	var out bytes.Buffer
	if err := VisualRequests(planID, &out, VisualRequestsOptions{JSON: true, Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "super-secret") {
		t.Fatalf("visual request output leaked secret: %s", out.String())
	}
	if !strings.Contains(out.String(), "local_video") || !strings.Contains(out.String(), "VISUAL_API_KEY") {
		t.Fatalf("visual request output missing backend/env name: %s", out.String())
	}
	artifact, err := readVisualRequestsDryRun(planID)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifact.Requests) == 0 || artifact.Requests[0].Status != "previewed" || artifact.Requests[0].Backend != "local_video" {
		t.Fatalf("artifact = %#v", artifact)
	}
}

func TestAgentOrchestrateSkipGraphCreatesPlanArtifacts(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	var out bytes.Buffer
	err := AgentOrchestrate(&out, AgentOrchestrateOptions{
		AgentPlanCommandOptions: AgentPlanCommandOptions{
			Goal:      "make a 35-second cinematic Instagram Reel with bold captions",
			InputPath: input,
		},
		SkipGraph: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "graph:   skipped") {
		t.Fatalf("output = %s", out.String())
	}
	planID := latestAgentPlanID(t)
	for _, name := range []string{creativeBriefArtifact, deliverablesArtifact, assetRequirementsArtifact, visualRequestsArtifact} {
		if _, err := os.Stat(filepath.Join(agentPlansV1Root, planID, name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
}

func TestAgentPlanWithMakeIDCreatesReviseMakeAction(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:   "make it shorter and reassemble",
		MakeID: "mk_123",
	}); err != nil {
		t.Fatal(err)
	}
	plan := readLatestAgentPlan(t)
	if plan.Actions[0].Type != agentActionTypeReviseMake {
		t.Fatalf("action = %#v", plan.Actions[0])
	}
}

func TestAgentPlanQueueHealthCreatesQueueAction(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal: "queue health",
	}); err != nil {
		t.Fatal(err)
	}
	plan := readLatestAgentPlan(t)
	if plan.Actions[0].Type != agentActionTypeQueue {
		t.Fatalf("action = %#v", plan.Actions[0])
	}
}

func TestGoalParsingDetectsPlatformCaptionsScriptAndVoiceover(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      "make a tiktok vertical short with boxed captions and narration script",
		InputPath: input,
	}); err != nil {
		t.Fatal(err)
	}
	plan := readLatestAgentPlan(t)
	inputMap := plan.Actions[0].Input
	if inputString(inputMap, "platform") != "tiktok" {
		t.Fatalf("platform = %v", inputMap["platform"])
	}
	if !inputBool(inputMap, "generate_captions") || !inputBool(inputMap, "generate_script") || !inputBool(inputMap, "prepare_voiceover") {
		t.Fatalf("input map = %#v", inputMap)
	}
	if inputString(inputMap, "caption_style") != "boxed" {
		t.Fatalf("caption style = %#v", inputMap)
	}
}

func TestVoiceoverWithoutProviderPermissionDegradesAndWarns(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      "make a short with narration",
		InputPath: input,
	}); err != nil {
		t.Fatal(err)
	}
	plan := readLatestAgentPlan(t)
	if inputBool(plan.Actions[0].Input, "generate_voiceover") {
		t.Fatalf("generate_voiceover should be false")
	}
	found := false
	for _, warning := range plan.Warnings {
		if strings.Contains(warning, "voiceover requested") {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %#v", plan.Warnings)
	}
}

func TestMissingInputMediaBlocksMakePlanInPolicy(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      "make a short",
		InputPath: "missing.mov",
	}); err != nil {
		t.Fatal(err)
	}
	planID := latestAgentPlanID(t)
	policy, err := readAgentPlanPolicy(planID)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Status != "blocked" {
		t.Fatalf("policy = %#v", policy)
	}
}

func TestPolicyMarksMakeApprovalRequiredAndQueueAllowed(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      "make a short",
		InputPath: input,
	}); err != nil {
		t.Fatal(err)
	}
	makePlanID := latestAgentPlanID(t)
	makePolicy, err := readAgentPlanPolicy(makePlanID)
	if err != nil {
		t.Fatal(err)
	}
	if makePolicy.Status != "approval_required" {
		t.Fatalf("make policy = %#v", makePolicy)
	}

	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal: "queue health",
	}); err != nil {
		t.Fatal(err)
	}
	queuePolicy, err := readAgentPlanPolicy(latestAgentPlanID(t))
	if err != nil {
		t.Fatal(err)
	}
	if queuePolicy.Status != "allowed" {
		t.Fatalf("queue policy = %#v", queuePolicy)
	}
}

func TestContextSnapshotRecordsSafeToDeleteAndTTLDays(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      "make a short",
		InputPath: input,
	}); err != nil {
		t.Fatal(err)
	}
	ctx, err := readAgentPlanContext(latestAgentPlanID(t))
	if err != nil {
		t.Fatal(err)
	}
	if !ctx.SafeToDelete || ctx.TTLDays != 7 {
		t.Fatalf("context = %#v", ctx)
	}
}

func TestAgentPlansListsNewestFirst(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{Goal: "queue health"}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{Goal: "queue status"}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := AgentPlans(&out, AgentPlansListOptions{}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("output = %s", out.String())
	}
	if !strings.Contains(lines[1], "agentplan-") || !strings.Contains(lines[2], "agentplan-") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestInspectAgentPlanAndAgentPolicyJSONValid(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{Goal: "queue health"}); err != nil {
		t.Fatal(err)
	}
	planID := latestAgentPlanID(t)
	var out bytes.Buffer
	if err := InspectAgentPlan(planID, &out, InspectAgentPlanCommandOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var inspect map[string]any
	if err := json.Unmarshal(out.Bytes(), &inspect); err != nil {
		t.Fatalf("invalid inspect json: %v", err)
	}
	out.Reset()
	if err := AgentPolicy(planID, &out, AgentPolicyCommandOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var policy map[string]any
	if err := json.Unmarshal(out.Bytes(), &policy); err != nil {
		t.Fatalf("invalid policy json: %v", err)
	}
}

func TestReviewAgentPlanWritesArtifactAndPlanIsCompact(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      "make a short with captions",
		InputPath: input,
	}); err != nil {
		t.Fatal(err)
	}
	planID := latestAgentPlanID(t)
	if err := ReviewAgentPlan(planID, ioDiscard{}, ReviewAgentPlanCommandOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(agentPlansV1Root, planID, "plan_review.md")); err != nil {
		t.Fatal(err)
	}
	plan, err := readAgentPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "policy checks") || strings.Contains(text, "safe_to_delete") {
		t.Fatalf("agent_plan.json should stay compact: %s", text)
	}
}

func TestAgentPlanEventsWritten(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:        "queue health",
		WriteReview: true,
	}); err != nil {
		t.Fatal(err)
	}
	planID := latestAgentPlanID(t)
	data, err := os.ReadFile(filepath.Join(agentPlansV1Root, planID, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"AGENT_PLAN_CREATED",
		"AGENT_CONTEXT_SNAPSHOT_WRITTEN",
		"AGENT_POLICY_REVIEW_WRITTEN",
		"AGENT_PLAN_REVIEW_WRITTEN",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
}

func latestAgentPlanID(t *testing.T) string {
	t.Helper()
	plans, err := listAgentPlans()
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) == 0 {
		t.Fatal("no plans found")
	}
	return plans[0].PlanID
}

func readLatestAgentPlan(t *testing.T) AgentPlanV1 {
	t.Helper()
	plan, err := readAgentPlan(latestAgentPlanID(t))
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func writeAgentPlanInputFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
