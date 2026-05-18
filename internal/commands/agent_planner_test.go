package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/modelrouter"
)

// ---- schema validation tests ----

func TestValidateAgentActions_Valid(t *testing.T) {
	actions := []AgentActionV1{
		{ID: "action_0001", Type: agentActionTypeMake, Description: "make a video", Status: "planned", PolicyStatus: "review_pending"},
	}
	if err := validateAgentActions(actions); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateAgentActions_Empty(t *testing.T) {
	err := validateAgentActions(nil)
	if err == nil || !strings.Contains(err.Error(), "zero actions") {
		t.Errorf("expected zero actions error, got: %v", err)
	}
}

func TestValidateAgentActions_TooMany(t *testing.T) {
	actions := make([]AgentActionV1, 11)
	for i := range actions {
		actions[i] = AgentActionV1{
			ID: fmt.Sprintf("action_%04d", i+1), Type: agentActionTypeQueue, Description: "x",
		}
	}
	err := validateAgentActions(actions)
	if err == nil || !strings.Contains(err.Error(), "maximum is 10") {
		t.Errorf("expected max error, got: %v", err)
	}
}

func TestValidateAgentActions_BadID(t *testing.T) {
	actions := []AgentActionV1{
		{ID: "bad-id", Type: agentActionTypeMake, Description: "x"},
	}
	err := validateAgentActions(actions)
	if err == nil || !strings.Contains(err.Error(), "action_NNNN") {
		t.Errorf("expected ID format error, got: %v", err)
	}
}

func TestValidateAgentActions_DuplicateID(t *testing.T) {
	actions := []AgentActionV1{
		{ID: "action_0001", Type: agentActionTypeMake, Description: "a"},
		{ID: "action_0001", Type: agentActionTypeQueue, Description: "b"},
	}
	err := validateAgentActions(actions)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestValidateAgentActions_UnknownType(t *testing.T) {
	actions := []AgentActionV1{
		{ID: "action_0001", Type: "launch_rockets", Description: "boom"},
	}
	err := validateAgentActions(actions)
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("expected unknown type error, got: %v", err)
	}
}

func TestValidateAgentActions_EmptyDescription(t *testing.T) {
	actions := []AgentActionV1{
		{ID: "action_0001", Type: agentActionTypeMake, Description: ""},
	}
	err := validateAgentActions(actions)
	if err == nil || !strings.Contains(err.Error(), "description is empty") {
		t.Errorf("expected description error, got: %v", err)
	}
}

// ---- planner dispatch tests ----

func TestSelectPlanner_DefaultIsDeterministic(t *testing.T) {
	p := selectPlanner(AgentPlanCommandOptions{Goal: "test"})
	if p.Name() != "deterministic" {
		t.Errorf("expected deterministic, got: %s", p.Name())
	}
}

func TestSelectPlanner_EmptyNameIsDeterministic(t *testing.T) {
	p := selectPlanner(AgentPlanCommandOptions{PlannerName: ""})
	if p.Name() != "deterministic" {
		t.Errorf("expected deterministic, got: %s", p.Name())
	}
}

func TestSelectPlanner_Ollama(t *testing.T) {
	p := selectPlanner(AgentPlanCommandOptions{PlannerName: "ollama", PlannerModel: "llama3"})
	if p.Name() != "ollama" {
		t.Errorf("expected ollama, got: %s", p.Name())
	}
}

func TestSelectPlanner_OllamaCaseInsensitive(t *testing.T) {
	p := selectPlanner(AgentPlanCommandOptions{PlannerName: "OLLAMA"})
	if p.Name() != "ollama" {
		t.Errorf("expected ollama, got: %s", p.Name())
	}
}

// ---- deterministic planner tests ----

func TestDeterministicPlanner_MakeAction(t *testing.T) {
	t.Chdir(t.TempDir())
	p := deterministicAgentPlanner{}
	ctx := PlannerContext{
		Intent:    "make a product short",
		InputPath: "clip.mov",
		Context:   AgentPlanContextSnapshot{},
		Options:   PlannerOptions{},
	}
	result, err := p.Plan(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Actions) == 0 || result.Actions[0].Type != agentActionTypeMake {
		t.Errorf("expected make action, got: %+v", result.Actions)
	}
	if result.Actions[0].Status != "planned" {
		t.Errorf("expected status planned, got: %s", result.Actions[0].Status)
	}
	if result.EffectiveMode != "deterministic" {
		t.Errorf("expected effective_mode deterministic, got: %s", result.EffectiveMode)
	}
	if result.RequestArtifact == nil {
		t.Error("expected request artifact for deterministic planner")
	}
}

func TestDeterministicPlanner_QueueAction(t *testing.T) {
	t.Chdir(t.TempDir())
	p := deterministicAgentPlanner{}
	ctx := PlannerContext{
		Intent:  "queue health",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{},
	}
	result, err := p.Plan(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Actions) == 0 || result.Actions[0].Type != agentActionTypeQueue {
		t.Errorf("expected queue action, got: %+v", result.Actions)
	}
}

func TestDeterministicPlanner_ReviseMakeAction(t *testing.T) {
	t.Chdir(t.TempDir())
	p := deterministicAgentPlanner{}
	ctx := PlannerContext{
		Intent: "make it shorter",
		MakeID: "mk-abc",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{},
	}
	result, err := p.Plan(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Actions) == 0 || result.Actions[0].Type != agentActionTypeReviseMake {
		t.Errorf("expected revise_make action, got: %+v", result.Actions)
	}
}

// ---- Ollama response parser tests ----

func TestParseOllamaPlannerResponse_ValidJSON(t *testing.T) {
	raw := `{
		"actions": [
			{
				"id": "action_0001",
				"type": "make",
				"description": "Create a video",
				"input": {"goal": "product demo"},
				"requires_approval": true,
				"requires_provider": false,
				"requires_network": false,
				"requires_overwrite": false
			}
		],
		"warnings": []
	}`
	actions, warnings, err := parseOllamaPlannerResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != "make" {
		t.Errorf("expected type make, got: %s", actions[0].Type)
	}
	_ = warnings
}

func TestParseOllamaPlannerResponse_WithCodeFences(t *testing.T) {
	raw := "```json\n{\"actions\":[{\"id\":\"action_0001\",\"type\":\"queue_health\",\"description\":\"Check queue\",\"input\":{}}],\"warnings\":[]}\n```"
	actions, _, err := parseOllamaPlannerResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 || actions[0].Type != "queue_health" {
		t.Errorf("expected queue_health action, got: %+v", actions)
	}
}

func TestParseOllamaPlannerResponse_EmptyResponse(t *testing.T) {
	_, _, err := parseOllamaPlannerResponse("")
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected empty response error, got: %v", err)
	}
}

func TestParseOllamaPlannerResponse_InvalidJSON(t *testing.T) {
	_, _, err := parseOllamaPlannerResponse("not json at all")
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected JSON error, got: %v", err)
	}
}

func TestParseOllamaPlannerResponse_NoActions(t *testing.T) {
	_, _, err := parseOllamaPlannerResponse(`{"actions": [], "warnings": []}`)
	if err == nil || !strings.Contains(err.Error(), "no actions") {
		t.Errorf("expected no actions error, got: %v", err)
	}
}

func TestParseOllamaPlannerResponse_NilInput(t *testing.T) {
	raw := `{"actions":[{"id":"action_0001","type":"queue_health","description":"Check"}]}`
	actions, _, err := parseOllamaPlannerResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions[0].Input == nil {
		t.Error("input should default to empty map, not nil")
	}
}

// ---- code fence stripping tests ----

func TestStripJSONCodeFences_Plain(t *testing.T) {
	s := `{"actions":[]}`
	if got := stripJSONCodeFences(s); got != s {
		t.Errorf("expected no change, got: %q", got)
	}
}

func TestStripJSONCodeFences_WithFences(t *testing.T) {
	s := "```json\n{\"actions\":[]}\n```"
	want := `{"actions":[]}`
	if got := stripJSONCodeFences(s); got != want {
		t.Errorf("expected %q, got: %q", want, got)
	}
}

func TestStripJSONCodeFences_PlainFences(t *testing.T) {
	s := "```\n{\"actions\":[]}\n```"
	want := `{"actions":[]}`
	if got := stripJSONCodeFences(s); got != want {
		t.Errorf("expected %q, got: %q", want, got)
	}
}

// ---- buildPlannerContext tests ----

func TestBuildPlannerContext(t *testing.T) {
	opts := AgentPlanCommandOptions{
		Goal:                  "make a video",
		InputPath:             "/tmp/clip.mov",
		MakeID:                "mk-1",
		PlannerName:           "ollama",
		PlannerModel:          "llama3",
		FallbackDeterministic: true,
		PlannerTimeoutSeconds: 60,
		PlannerTemperature:    0.5,
		PlannerMaxOutputChars: 4096,
	}
	ctx := AgentPlanContextSnapshot{}
	planCtx := buildPlannerContext(opts, ctx)

	if planCtx.Intent != "make a video" {
		t.Errorf("intent = %q", planCtx.Intent)
	}
	if planCtx.MakeID != "mk-1" {
		t.Errorf("make_id = %q", planCtx.MakeID)
	}
	if planCtx.Options.PlannerName != "ollama" {
		t.Errorf("planner name = %q", planCtx.Options.PlannerName)
	}
	if planCtx.Options.PlannerModel != "llama3" {
		t.Errorf("planner model = %q", planCtx.Options.PlannerModel)
	}
	if !planCtx.Options.FallbackDeterministic {
		t.Error("expected FallbackDeterministic = true")
	}
	if planCtx.Options.TimeoutSeconds != 60 {
		t.Errorf("timeout = %d", planCtx.Options.TimeoutSeconds)
	}
	if planCtx.Options.Temperature != 0.5 {
		t.Errorf("temperature = %f", planCtx.Options.Temperature)
	}
	if planCtx.Options.MaxOutputChars != 4096 {
		t.Errorf("max output chars = %d", planCtx.Options.MaxOutputChars)
	}
}

// ---- Ollama planner end-to-end with stub HTTP ----

func makeOllamaStubResponse(t *testing.T, body string) func(timeout time.Duration) modelrouter.HTTPDoer {
	t.Helper()
	return func(_ time.Duration) modelrouter.HTTPDoer {
		return stubHTTPDoer{body: body, status: http.StatusOK}
	}
}

type stubHTTPDoer struct {
	body   string
	status int
}

func (s stubHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	payload, _ := json.Marshal(map[string]any{"response": s.body})
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}, nil
}

func TestOllamaPlanner_Success(t *testing.T) {
	t.Chdir(t.TempDir())

	ollamaBody := `{
		"actions": [
			{
				"id": "action_0001",
				"type": "queue_health",
				"description": "Check the queue",
				"input": {},
				"requires_approval": false,
				"requires_provider": false,
				"requires_network": false,
				"requires_overwrite": false,
				"status": "planned",
				"policy_status": "review_pending"
			}
		],
		"warnings": ["test warning"]
	}`

	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, ollamaBody))
	defer restore()

	p := newOllamaAgentPlanner(AgentPlanCommandOptions{PlannerModel: "llama3"})
	planCtx := PlannerContext{
		Intent:  "check queue status",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{PlannerModel: "llama3"},
	}

	result, err := p.Plan(planCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Actions) != 1 || result.Actions[0].Type != agentActionTypeQueue {
		t.Errorf("expected queue_health action, got: %+v", result.Actions)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected at least one warning from stub response")
	}
	if result.EffectiveMode != "ollama" {
		t.Errorf("expected effective_mode ollama, got: %s", result.EffectiveMode)
	}
	if result.ResolvedModel != "llama3" {
		t.Errorf("expected resolved model llama3, got: %s", result.ResolvedModel)
	}
	if result.RequestArtifact == nil {
		t.Error("expected request artifact for ollama planner")
	} else {
		if result.RequestArtifact.SystemPrompt == "" {
			t.Error("expected non-empty system prompt in request artifact")
		}
		if result.RequestArtifact.UserPrompt == "" {
			t.Error("expected non-empty user prompt in request artifact")
		}
	}
}

func TestOllamaPlanner_FallbackOnInvalidResponse(t *testing.T) {
	t.Chdir(t.TempDir())

	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, "not json"))
	defer restore()

	p := newOllamaAgentPlanner(AgentPlanCommandOptions{PlannerModel: "llama3"})
	planCtx := PlannerContext{
		Intent:  "queue health",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{
			PlannerModel:          "llama3",
			FallbackDeterministic: true,
		},
	}

	result, err := p.Plan(planCtx)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
	if len(result.Actions) == 0 {
		t.Error("expected fallback actions")
	}
	fallbackWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "deterministic fallback") {
			fallbackWarning = true
		}
	}
	if !fallbackWarning {
		t.Errorf("expected fallback warning in: %v", result.Warnings)
	}
	if !result.FallbackUsed {
		t.Error("expected FallbackUsed = true")
	}
	if result.EffectiveMode != "deterministic" {
		t.Errorf("expected effective_mode deterministic after fallback, got: %s", result.EffectiveMode)
	}
	if result.ResolvedModel != "llama3" {
		t.Errorf("expected resolved model preserved through fallback, got: %s", result.ResolvedModel)
	}
}

func TestOllamaPlanner_ErrorWithoutFallback(t *testing.T) {
	t.Chdir(t.TempDir())

	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, "not json"))
	defer restore()

	p := newOllamaAgentPlanner(AgentPlanCommandOptions{PlannerModel: "llama3"})
	planCtx := PlannerContext{
		Intent:  "queue health",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{
			PlannerModel:          "llama3",
			FallbackDeterministic: false,
		},
	}

	_, err := p.Plan(planCtx)
	if err == nil {
		t.Error("expected error when response is invalid JSON and no fallback")
	}
}

func TestOllamaPlanner_MissingModel(t *testing.T) {
	t.Chdir(t.TempDir())
	p := newOllamaAgentPlanner(AgentPlanCommandOptions{})
	planCtx := PlannerContext{
		Intent:  "queue health",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{},
	}
	_, err := p.Plan(planCtx)
	if err == nil || !strings.Contains(err.Error(), "requires a model") {
		t.Errorf("expected missing model error, got: %v", err)
	}
}

func TestOllamaPlanner_SchemaValidationRejectsInvalidType(t *testing.T) {
	t.Chdir(t.TempDir())

	ollamaBody := `{"actions":[{"id":"action_0001","type":"launch_rockets","description":"go boom","input":{}}]}`

	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, ollamaBody))
	defer restore()

	p := newOllamaAgentPlanner(AgentPlanCommandOptions{PlannerModel: "llama3"})
	planCtx := PlannerContext{
		Intent:  "do something",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{PlannerModel: "llama3"},
	}

	_, err := p.Plan(planCtx)
	if err == nil || !strings.Contains(err.Error(), "invalid actions") {
		t.Errorf("expected schema validation error, got: %v", err)
	}
}

func TestOllamaPlanner_MaxOutputCharsTruncates(t *testing.T) {
	t.Chdir(t.TempDir())

	// This will be truncated and fail JSON parse
	longBody := strings.Repeat("x", 10000)
	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, longBody))
	defer restore()

	p := newOllamaAgentPlanner(AgentPlanCommandOptions{PlannerModel: "llama3"})
	planCtx := PlannerContext{
		Intent:  "queue health",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{
			PlannerModel:          "llama3",
			MaxOutputChars:        10, // very small limit
			FallbackDeterministic: false,
		},
	}

	_, err := p.Plan(planCtx)
	if err == nil {
		t.Error("expected error due to truncated (unparseable) response")
	}
}

// ---- agent-plan command integration tests (with planner flag) ----

func TestAgentPlanCommand_DeterministicDefault(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal: "queue health",
	}); err != nil {
		t.Fatal(err)
	}
	plan := readLatestAgentPlan(t)
	if plan.Planner.Mode != "deterministic" {
		t.Errorf("expected mode deterministic, got: %s", plan.Planner.Mode)
	}
	if plan.Planner.RequestedMode != "deterministic" {
		t.Errorf("expected requested_mode deterministic, got: %s", plan.Planner.RequestedMode)
	}
	if plan.Planner.EffectiveMode != "deterministic" {
		t.Errorf("expected effective_mode deterministic, got: %s", plan.Planner.EffectiveMode)
	}
	if plan.Planner.FallbackUsed {
		t.Error("expected fallback_used = false for deterministic planner")
	}
	if plan.Planner.Provider != "deterministic" {
		t.Errorf("expected provider deterministic, got: %s", plan.Planner.Provider)
	}
	// planner_request.json should be written
	planDir := filepath.Join(agentPlansV1Root, plan.PlanID)
	if _, err := os.Stat(filepath.Join(planDir, "planner_request.json")); err != nil {
		t.Errorf("expected planner_request.json to exist: %v", err)
	}
}

func TestAgentPlanCommand_OllamaModeWritesPlannerInfo(t *testing.T) {
	t.Chdir(t.TempDir())

	ollamaBody := `{"actions":[{"id":"action_0001","type":"queue_health","description":"check queue","input":{}}]}`
	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, ollamaBody))
	defer restore()

	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:           "check queue status",
		PlannerName:    "ollama",
		PlannerModel:   "llama3",
		PlannerBackend: "http://localhost:11434",
	}); err != nil {
		t.Fatal(err)
	}
	plan := readLatestAgentPlan(t)
	if plan.Planner.Mode != "ollama" {
		t.Errorf("expected planner mode ollama, got: %s", plan.Planner.Mode)
	}
	if plan.Planner.RequestedMode != "ollama" {
		t.Errorf("expected requested_mode ollama, got: %s", plan.Planner.RequestedMode)
	}
	if plan.Planner.EffectiveMode != "ollama" {
		t.Errorf("expected effective_mode ollama, got: %s", plan.Planner.EffectiveMode)
	}
	if plan.Planner.Model != "llama3" {
		t.Errorf("expected planner model llama3, got: %s", plan.Planner.Model)
	}
	if plan.Planner.Backend != "http://localhost:11434" {
		t.Errorf("expected backend set, got: %s", plan.Planner.Backend)
	}
	if plan.Planner.Provider != "ollama" {
		t.Errorf("expected provider ollama, got: %s", plan.Planner.Provider)
	}
	if plan.Planner.FallbackUsed {
		t.Error("expected fallback_used = false for successful ollama run")
	}
	// planner_request.json should be written with prompts
	planDir := filepath.Join(agentPlansV1Root, plan.PlanID)
	reqPath := filepath.Join(planDir, "planner_request.json")
	if _, err := os.Stat(reqPath); err != nil {
		t.Errorf("expected planner_request.json to exist: %v", err)
	} else {
		data, _ := os.ReadFile(reqPath)
		var req PlannerRequestArtifact
		if err := json.Unmarshal(data, &req); err != nil {
			t.Errorf("planner_request.json is not valid JSON: %v", err)
		}
		if req.SystemPrompt == "" {
			t.Error("expected system_prompt in planner_request.json")
		}
		if req.UserPrompt == "" {
			t.Error("expected user_prompt in planner_request.json")
		}
		if req.Model != "llama3" {
			t.Errorf("expected model llama3 in planner_request.json, got: %s", req.Model)
		}
	}
}

func TestAgentPlanCommand_OllamaFallbackDeterministic(t *testing.T) {
	t.Chdir(t.TempDir())

	// Return invalid JSON so the Ollama planner fails and falls back
	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, "oops not json"))
	defer restore()

	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:                  "queue health",
		PlannerName:           "ollama",
		PlannerModel:          "llama3",
		FallbackDeterministic: true,
	}); err != nil {
		t.Fatal(err)
	}
	plan := readLatestAgentPlan(t)
	if len(plan.Actions) == 0 {
		t.Error("expected at least one action from fallback")
	}
	hasFallbackWarning := false
	for _, w := range plan.Warnings {
		if strings.Contains(w, "fallback") {
			hasFallbackWarning = true
		}
	}
	if !hasFallbackWarning {
		t.Errorf("expected fallback warning in plan.Warnings: %v", plan.Warnings)
	}
	// requested_mode = ollama, effective_mode = deterministic
	if plan.Planner.RequestedMode != "ollama" {
		t.Errorf("expected requested_mode ollama, got: %s", plan.Planner.RequestedMode)
	}
	if plan.Planner.EffectiveMode != "deterministic" {
		t.Errorf("expected effective_mode deterministic after fallback, got: %s", plan.Planner.EffectiveMode)
	}
	if !plan.Planner.FallbackUsed {
		t.Error("expected fallback_used = true")
	}
	if plan.Planner.FallbackReason == "" {
		t.Error("expected fallback_reason to be set")
	}
	if plan.Planner.Model != "llama3" {
		t.Errorf("expected model preserved through fallback, got: %s", plan.Planner.Model)
	}
}

func TestAgentPlanCommand_OllmaModeNoModelFails(t *testing.T) {
	t.Chdir(t.TempDir())
	err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:        "queue health",
		PlannerName: "ollama",
		// No PlannerModel set, no config
	})
	if err == nil || !strings.Contains(err.Error(), "requires a model") {
		t.Errorf("expected missing model error, got: %v", err)
	}
}

// ---- planner diagnose command tests ----

func TestPlannerDiagnose_DeterministicDefault(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf strings.Builder
	err := AgentPlannerDiagnoseCommand(&buf, PlannerDiagnoseOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "deterministic") {
		t.Errorf("expected 'deterministic' in output, got: %s", out)
	}
}

func TestPlannerDiagnose_OllamaMode(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf strings.Builder
	err := AgentPlannerDiagnoseCommand(&buf, PlannerDiagnoseOptions{
		PlannerName:    "ollama",
		PlannerModel:   "llama3",
		PlannerBackend: "http://localhost:11434",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ollama") {
		t.Errorf("expected 'ollama' in output, got: %s", out)
	}
	if !strings.Contains(out, "llama3") {
		t.Errorf("expected 'llama3' in output, got: %s", out)
	}
	if !strings.Contains(out, "http://localhost:11434") {
		t.Errorf("expected backend in output, got: %s", out)
	}
}

func TestPlannerDiagnose_JSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf strings.Builder
	err := AgentPlannerDiagnoseCommand(&buf, PlannerDiagnoseOptions{
		PlannerName:  "ollama",
		PlannerModel: "qwen2.5",
		JSON:         true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result PlannerDiagnoseResult
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if result.RequestedMode != "ollama" {
		t.Errorf("expected requested_mode ollama, got: %s", result.RequestedMode)
	}
	if result.ResolvedModel != "qwen2.5" {
		t.Errorf("expected resolved_model qwen2.5, got: %s", result.ResolvedModel)
	}
}

func TestPlannerDiagnose_ConfigRouteResolution(t *testing.T) {
	t.Chdir(t.TempDir())
	// Write a byom-video.yaml with a model route
	cfgContent := `models:
  enabled: true
  routes:
    agent.planning: my-planner
  entries:
    my-planner:
      model: qwen2.5:7b
      base_url: http://localhost:11434
`
	if err := os.WriteFile("byom-video.yaml", []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	err := AgentPlannerDiagnoseCommand(&buf, PlannerDiagnoseOptions{
		PlannerName: "ollama",
		JSON:        true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result PlannerDiagnoseResult
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result.ResolvedModel != "qwen2.5:7b" {
		t.Errorf("expected model qwen2.5:7b from config, got: %s", result.ResolvedModel)
	}
	if result.ResolvedBackend != "http://localhost:11434" {
		t.Errorf("expected backend from config, got: %s", result.ResolvedBackend)
	}
	if result.ConfigEntryName != "my-planner" {
		t.Errorf("expected config entry name my-planner, got: %s", result.ConfigEntryName)
	}
}

func TestPlannerDiagnose_CheckFlag_UnreachableServer(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf strings.Builder
	err := AgentPlannerDiagnoseCommand(&buf, PlannerDiagnoseOptions{
		PlannerName:           "ollama",
		PlannerModel:          "llama3",
		PlannerBackend:        "http://127.0.0.1:19999",
		PlannerTimeoutSeconds: 1,
		Check:                 true,
		JSON:                  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result PlannerDiagnoseResult
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result.OllamaReachable == nil {
		t.Fatal("expected OllamaReachable to be set when --check used")
	}
	if *result.OllamaReachable {
		t.Error("expected OllamaReachable = false for unavailable server")
	}
	if result.OllamaCheckError == "" {
		t.Error("expected OllamaCheckError to be set")
	}
}

// ---- planner request artifact tests ----

func TestDeterministicPlannerRequestArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	p := deterministicAgentPlanner{}
	result, err := p.Plan(PlannerContext{
		Intent:    "make a short",
		InputPath: "clip.mov",
		Context:   AgentPlanContextSnapshot{},
		Options:   PlannerOptions{Platform: "tiktok"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequestArtifact == nil {
		t.Fatal("expected request artifact")
	}
	if result.RequestArtifact.PlannerMode != "deterministic" {
		t.Errorf("expected planner_mode deterministic, got: %s", result.RequestArtifact.PlannerMode)
	}
	if result.RequestArtifact.GoalHints == nil {
		t.Error("expected goal_hints in request artifact")
	}
	if result.RequestArtifact.SchemaVersion != "openvfx_planner_request.v1" {
		t.Errorf("expected schema version, got: %s", result.RequestArtifact.SchemaVersion)
	}
}

func TestOllamaPlannerRequestArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	ollamaBody := `{"actions":[{"id":"action_0001","type":"queue_health","description":"check queue","input":{}}]}`
	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, ollamaBody))
	defer restore()

	p := newOllamaAgentPlanner(AgentPlanCommandOptions{PlannerModel: "llama3"})
	result, err := p.Plan(PlannerContext{
		Intent:  "queue health",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{PlannerModel: "llama3"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequestArtifact == nil {
		t.Fatal("expected request artifact for ollama planner")
	}
	if result.RequestArtifact.PlannerMode != "ollama" {
		t.Errorf("expected planner_mode ollama, got: %s", result.RequestArtifact.PlannerMode)
	}
	if result.RequestArtifact.SystemPrompt == "" {
		t.Error("expected system_prompt in artifact")
	}
	if result.RequestArtifact.UserPrompt == "" {
		t.Error("expected user_prompt in artifact")
	}
}

func TestOllamaFallbackPreservesPrompts(t *testing.T) {
	t.Chdir(t.TempDir())
	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, "bad json"))
	defer restore()

	p := newOllamaAgentPlanner(AgentPlanCommandOptions{PlannerModel: "mistral"})
	result, err := p.Plan(PlannerContext{
		Intent:  "queue health",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{PlannerModel: "mistral", FallbackDeterministic: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequestArtifact == nil {
		t.Fatal("expected request artifact after fallback")
	}
	// Fallback carries the ollama prompts for observability
	if result.RequestArtifact.SystemPrompt == "" {
		t.Error("expected system_prompt preserved through fallback for observability")
	}
	if !result.RequestArtifact.FallbackUsed {
		t.Error("expected fallback_used = true in request artifact")
	}
	if result.RequestArtifact.EffectiveMode != "deterministic" {
		t.Errorf("expected effective_mode deterministic in artifact, got: %s", result.RequestArtifact.EffectiveMode)
	}
}

func TestPlannerRequestArtifactWrittenToDisk(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal: "check queue",
	}); err != nil {
		t.Fatal(err)
	}
	plan := readLatestAgentPlan(t)
	planDir := filepath.Join(agentPlansV1Root, plan.PlanID)
	reqPath := filepath.Join(planDir, "planner_request.json")
	data, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("planner_request.json not written: %v", err)
	}
	var artifact PlannerRequestArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("invalid planner_request.json: %v", err)
	}
	if artifact.SchemaVersion != "openvfx_planner_request.v1" {
		t.Errorf("expected schema version, got: %s", artifact.SchemaVersion)
	}
	// references should mention planner_request.json
	if plan.References.PlannerRequest != "planner_request.json" {
		t.Errorf("expected references.planner_request = planner_request.json, got: %q", plan.References.PlannerRequest)
	}
}

// ---- config route resolution tests ----

func TestOllamaPlannerResolvesModelFromConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	cfgContent := `models:
  enabled: true
  routes:
    agent.planning: test-planner
  entries:
    test-planner:
      model: phi3:mini
      base_url: http://localhost:11434
`
	if err := os.WriteFile("byom-video.yaml", []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	ollamaBody := `{"actions":[{"id":"action_0001","type":"queue_health","description":"check queue","input":{}}]}`
	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, ollamaBody))
	defer restore()

	// No explicit model — should be resolved from config
	p := newOllamaAgentPlanner(AgentPlanCommandOptions{})
	result, err := p.Plan(PlannerContext{
		Intent:  "queue health",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedModel != "phi3:mini" {
		t.Errorf("expected model phi3:mini from config, got: %s", result.ResolvedModel)
	}
	if result.ResolvedBackend != "http://localhost:11434" {
		t.Errorf("expected backend from config, got: %s", result.ResolvedBackend)
	}
}

func TestOllamaPlannerCustomRouteKey(t *testing.T) {
	t.Chdir(t.TempDir())
	cfgContent := `models:
  enabled: true
  routes:
    my.custom.route: custom-entry
  entries:
    custom-entry:
      model: mistral:7b
      base_url: http://localhost:11434
`
	if err := os.WriteFile("byom-video.yaml", []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	ollamaBody := `{"actions":[{"id":"action_0001","type":"queue_health","description":"check","input":{}}]}`
	restore := modelrouter.SetHTTPClientFactoryForTests(makeOllamaStubResponse(t, ollamaBody))
	defer restore()

	p := newOllamaAgentPlanner(AgentPlanCommandOptions{})
	result, err := p.Plan(PlannerContext{
		Intent:  "queue health",
		Context: AgentPlanContextSnapshot{},
		Options: PlannerOptions{PlannerRoute: "my.custom.route"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedModel != "mistral:7b" {
		t.Errorf("expected model mistral:7b from custom route, got: %s", result.ResolvedModel)
	}
	if result.ResolvedRoute != "my.custom.route" {
		t.Errorf("expected resolved_route my.custom.route, got: %s", result.ResolvedRoute)
	}
}

func TestApplyDefaultActionFields(t *testing.T) {
	actions := []AgentActionV1{
		{ID: "action_0001", Type: agentActionTypeMake, Description: "x"},
		{ID: "action_0002", Type: agentActionTypeQueue, Description: "y", Status: "planned", PolicyStatus: "allowed"},
	}
	result := applyDefaultActionFields(actions)
	if result[0].Status != "planned" {
		t.Errorf("expected status planned, got: %s", result[0].Status)
	}
	if result[0].PolicyStatus != "review_pending" {
		t.Errorf("expected policy_status review_pending, got: %s", result[0].PolicyStatus)
	}
	// Pre-set values should not be overwritten
	if result[1].PolicyStatus != "allowed" {
		t.Errorf("expected pre-set policy_status allowed, got: %s", result[1].PolicyStatus)
	}
}
