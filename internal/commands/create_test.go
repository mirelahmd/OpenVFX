package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreatePreviewWritesSessionPlanArtifactsButNoJobs(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	var out bytes.Buffer
	err := createWithDeps(&out, CreateOptions{
		InputPath:   input,
		Goal:        "make a 35-second luxury fitness Instagram Reel with bold lower-third captions",
		WriteReview: true,
		SkipGraph:   true,
	}, defaultCreateDeps)
	if err != nil {
		t.Fatal(err)
	}
	session := latestCreateSession(t)
	if session.Status != createStatusReady {
		t.Fatalf("session status = %s", session.Status)
	}
	if session.Linked.AgentPlanID == "" {
		t.Fatalf("expected linked agent plan")
	}
	if _, err := os.Stat(filepath.Join(agentPlansV1Root, session.Linked.AgentPlanID, creativeBriefArtifact)); err != nil {
		t.Fatalf("creative brief missing: %v", err)
	}
	if _, err := os.Stat(jobsRoot); !os.IsNotExist(err) {
		t.Fatalf("jobs root should not exist in preview")
	}
	if _, err := os.Stat(filepath.Join(createSessionDir(session.CreateSessionID), "create_review.md")); err != nil {
		t.Fatalf("create review missing: %v", err)
	}
}

func TestCreateDryRunWritesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	err := createWithDeps(&out, CreateOptions{InputPath: "clip.mov", Goal: "make a short", DryRun: true}, defaultCreateDeps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(createSessionsRoot); !os.IsNotExist(err) {
		t.Fatalf("create sessions root should not exist")
	}
	if _, err := os.Stat(agentPlansV1Root); !os.IsNotExist(err) {
		t.Fatalf("agent plans root should not exist")
	}
}

func TestCreateLocalScopeRefusesProviderRequiredWork(t *testing.T) {
	t.Chdir(t.TempDir())
	err := createWithDeps(io.Discard, CreateOptions{
		InputPath:          "clip.mov",
		Goal:               "make a short with generated voiceover",
		Yes:                true,
		ApprovalScope:      "local",
		AllowProviderCalls: true,
		SkipGraph:          true,
	}, fakeCreateDepsWithPolicy(t, true, false))
	if err != nil {
		t.Fatal(err)
	}
	session := latestCreateSession(t)
	if session.Status != createStatusBlocked {
		t.Fatalf("status = %s errors=%v", session.Status, session.Errors)
	}
	if !createContainsString(session.Errors, "provider calls requested but approval_scope=local") {
		t.Fatalf("errors = %#v", session.Errors)
	}
}

func TestCreateProviderScopeAllowsProviderWhenFlagsSet(t *testing.T) {
	t.Chdir(t.TempDir())
	err := createWithDeps(io.Discard, CreateOptions{
		InputPath:            "clip.mov",
		Goal:                 "make a short with generated voiceover",
		Yes:                  true,
		ApprovalScope:        "provider",
		AllowProviderCalls:   true,
		AllowExternalNetwork: true,
		SkipGraph:            true,
	}, fakeCreateDepsWithPolicy(t, true, false))
	if err != nil {
		t.Fatal(err)
	}
	session := latestCreateSession(t)
	if session.Status == createStatusBlocked {
		t.Fatalf("provider scope should not block: %#v", session.Errors)
	}
}

func TestCreateLocalScopeRequiresAllowOverwrite(t *testing.T) {
	t.Chdir(t.TempDir())
	err := createWithDeps(io.Discard, CreateOptions{
		InputPath:     "clip.mov",
		Goal:          "make a short overwrite",
		Yes:           true,
		ApprovalScope: "local",
		SkipGraph:     true,
	}, fakeCreateDepsWithPolicy(t, false, true))
	if err != nil {
		t.Fatal(err)
	}
	session := latestCreateSession(t)
	if session.Status != createStatusBlocked || !createContainsString(session.Errors, "overwrite requested but --allow-overwrite not set") {
		t.Fatalf("session = %#v", session)
	}
}

func TestCreateConvertApproveJobs(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	err := createWithDeps(io.Discard, CreateOptions{
		InputPath:     input,
		Goal:          "make a short with captions",
		Yes:           true,
		ApprovalScope: "local",
		Convert:       true,
		ApproveJobs:   true,
		SkipGraph:     true,
	}, defaultCreateDeps)
	if err != nil {
		t.Fatal(err)
	}
	session := latestCreateSession(t)
	if session.Status != createStatusConverted {
		t.Fatalf("status = %s errors=%v", session.Status, session.Errors)
	}
	linked, err := readAgentLinkedJobs(session.Linked.AgentPlanID)
	if err != nil {
		t.Fatal(err)
	}
	if len(linked.Jobs) == 0 || linked.Jobs[0].ApprovalStatus != JobApprovalApproved {
		t.Fatalf("linked = %#v", linked.Jobs)
	}
}

func TestCreateRunJobsUsesFakeDepsAndRejectsConflict(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	runCount := 0
	deps := defaultCreateDeps
	deps.runJob = func(jobID string, stdout io.Writer, opts JobRunOptions) error {
		runCount++
		return nil
	}
	err := createWithDeps(io.Discard, CreateOptions{
		InputPath:     input,
		Goal:          "make a short",
		Yes:           true,
		ApprovalScope: "local",
		Convert:       true,
		ApproveJobs:   true,
		RunJobs:       true,
		SkipGraph:     true,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	if runCount != 1 {
		t.Fatalf("runCount = %d", runCount)
	}
	err = createWithDeps(io.Discard, CreateOptions{InputPath: input, Goal: "make", RunJobs: true, WorkerOnce: true, DryRun: false}, deps)
	if err == nil || !strings.Contains(err.Error(), "cannot both be set") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestCreateResultReadsAndWritesReview(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := createWithDeps(io.Discard, CreateOptions{InputPath: input, Goal: "make a short", SkipGraph: true}, defaultCreateDeps); err != nil {
		t.Fatal(err)
	}
	session := latestCreateSession(t)
	var out bytes.Buffer
	if err := CreateResult(session.CreateSessionID, &out, CreateResultOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), session.CreateSessionID) {
		t.Fatalf("output = %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(createSessionDir(session.CreateSessionID), "create_review.md")); err != nil {
		t.Fatalf("review missing: %v", err)
	}
	review, err := os.ReadFile(filepath.Join(createSessionDir(session.CreateSessionID), "create_review.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# OpenVFX Create Review", "## Creative Brief", "## Planned Deliverables", "## Asset Requirements", "## Visual Generation Dry-Run Requests", "## Jobs", "## Outputs"} {
		if !strings.Contains(string(review), expected) {
			t.Fatalf("review missing %q:\n%s", expected, string(review))
		}
	}
}

func TestCreateResultIncludesVisualRequests(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := createWithDeps(io.Discard, CreateOptions{
		InputPath: input,
		Goal:      "make a premium reel and generate futuristic gym b-roll",
		SkipGraph: true,
	}, defaultCreateDeps); err != nil {
		t.Fatal(err)
	}
	session := latestCreateSession(t)
	var out bytes.Buffer
	if err := CreateResult(session.CreateSessionID, &out, CreateResultOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "visual_requests") || !strings.Contains(out.String(), "visual_req_") {
		t.Fatalf("create-result json = %s", out.String())
	}
}

func TestCreateSessionsAndInspectJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := createWithDeps(io.Discard, CreateOptions{InputPath: input, Goal: "make a short", SkipGraph: true}, defaultCreateDeps); err != nil {
		t.Fatal(err)
	}
	session := latestCreateSession(t)
	var out bytes.Buffer
	if err := CreateSessions(&out, CreateSessionsOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), session.CreateSessionID) {
		t.Fatalf("create-sessions json = %s", out.String())
	}
	out.Reset()
	if err := InspectCreateSession(session.CreateSessionID, &out, InspectCreateSessionOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "creative_brief") || !strings.Contains(out.String(), "asset_requirements") {
		t.Fatalf("inspect json = %s", out.String())
	}
}

func fakeCreateDepsWithPolicy(t *testing.T, provider bool, overwrite bool) createDeps {
	t.Helper()
	deps := defaultCreateDeps
	deps.agentPlan = func(stdout io.Writer, opts AgentPlanCommandOptions) error {
		now := time.Now().UTC()
		planID := "agentplan-" + now.Format("20060102T150405.000000000Z")
		dir := filepath.Join(agentPlansV1Root, planID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		plan := AgentPlanV1{
			SchemaVersion: agentPlanV1Schema,
			PlanID:        planID,
			CreatedAt:     now,
			UpdatedAt:     now,
			Status:        agentPlanStatusDraft,
			Intent:        opts.Goal,
			Input:         AgentPlanInput{MediaPath: opts.InputPath, Goal: opts.Goal},
			Planner:       AgentPlannerInfo{Mode: "deterministic", Version: "v1"},
			Actions: []AgentActionV1{{
				ID:                "action_0001",
				Type:              agentActionTypeMake,
				Description:       "fake make",
				Input:             map[string]any{"input_path": opts.InputPath, "goal": opts.Goal},
				RequiresApproval:  true,
				RequiresProvider:  provider,
				RequiresNetwork:   provider,
				RequiresOverwrite: overwrite,
				PolicyStatus:      "approval_required",
				Status:            "planned",
			}},
			References: AgentPlanReferences{ContextSnapshot: "context_snapshot.json", PolicyReview: "policy_review.json", PlanReview: "plan_review.md"},
		}
		policy := AgentPlanPolicyReview{
			SchemaVersion:               policyReviewV1Schema,
			CreatedAt:                   now,
			PlanID:                      planID,
			Status:                      "approval_required",
			RequiresUserApproval:        true,
			RequiresProviderPermission:  provider,
			RequiresExternalNetwork:     provider,
			RequiresOverwritePermission: overwrite,
		}
		if err := writeJSONFile(filepath.Join(dir, "agent_plan.json"), plan); err != nil {
			return err
		}
		return writeJSONFile(filepath.Join(dir, "policy_review.json"), policy)
	}
	return deps
}

func latestCreateSession(t *testing.T) CreateSession {
	t.Helper()
	entries, err := os.ReadDir(createSessionsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no create sessions")
	}
	var newest CreateSession
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		session, err := readCreateSession(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if newest.CreateSessionID == "" || session.CreatedAt.After(newest.CreatedAt) {
			newest = session
		}
	}
	return newest
}

func createContainsString(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
