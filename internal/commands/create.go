package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/events"
)

const (
	createSessionsRoot    = ".byom-video/create_sessions"
	createSessionSchema   = "openvfx_create_session.v1"
	createStatusPlanned   = "planned"
	createStatusReady     = "ready_for_approval"
	createStatusConverted = "converted"
	createStatusRunning   = "running"
	createStatusCompleted = "completed"
	createStatusFailed    = "failed"
	createStatusBlocked   = "blocked"
)

type CreateOptions struct {
	InputPath   string
	Goal        string
	JSON        bool
	DryRun      bool
	WriteReview bool

	PlannerName           string
	PlannerModel          string
	FallbackDeterministic bool
	WorkersDir            string
	SkipGraph             bool

	Yes                  bool
	ApprovalScope        string
	AllowOverwrite       bool
	AllowProviderCalls   bool
	AllowExternalNetwork bool
	ApproveJobs          bool

	Convert     bool
	RunJobs     bool
	WorkerOnce  bool
	StartDaemon bool
	FailFast    bool

	Platform             string
	BurnCaptions         bool
	AllowMissingCaptions bool
	CaptionPosition      string
	CaptionStyle         string
	GenerateScript       bool
	GenerateCaptions     bool
	PrepareVoiceover     bool
	GenerateVoiceover    bool
	MixVoiceover         bool
}

type CreateResultOptions struct {
	JSON          bool
	WriteArtifact bool
}

type CreateSessionsOptions struct {
	JSON   bool
	Status string
	Limit  int
}

type InspectCreateSessionOptions struct {
	JSON bool
}

type CreateResultSummary struct {
	Session           CreateSession                 `json:"session"`
	AgentPlan         *AgentPlanV1                  `json:"agent_plan,omitempty"`
	Policy            *AgentPlanPolicyReview        `json:"policy_review,omitempty"`
	CreativeBrief     *CreativeBriefArtifact        `json:"creative_brief,omitempty"`
	Deliverables      *DeliverablesArtifact         `json:"deliverables,omitempty"`
	AssetRequirements *AssetRequirementsArtifact    `json:"asset_requirements,omitempty"`
	VisualRequests    *VisualRequestsDryRunArtifact `json:"visual_requests,omitempty"`
	GeneratedAssets   *GeneratedAssetsArtifact      `json:"generated_assets,omitempty"`
	AgentDecision     map[string]any                `json:"agent_decision,omitempty"`
	LinkedJobs        []CreateJobResultView         `json:"linked_jobs,omitempty"`
	Outputs           CreateOutputSummary           `json:"outputs"`
	NextCommands      []string                      `json:"next_commands,omitempty"`
}

type CreateJobResultView struct {
	JobID          string         `json:"job_id"`
	Type           string         `json:"type"`
	Status         string         `json:"status"`
	ApprovalStatus string         `json:"approval_status"`
	SourceAction   string         `json:"source_action,omitempty"`
	Output         map[string]any `json:"output,omitempty"`
	Errors         []string       `json:"errors,omitempty"`
	Warnings       []string       `json:"warnings,omitempty"`
}

type CreateOutputSummary struct {
	DraftVideo     string   `json:"draft_video,omitempty"`
	Captions       string   `json:"captions,omitempty"`
	Script         string   `json:"script,omitempty"`
	VoiceoverText  string   `json:"voiceover_text,omitempty"`
	VoiceoverAudio string   `json:"voiceover_audio,omitempty"`
	ResultReports  []string `json:"result_reports,omitempty"`
}

type CreateSession struct {
	SchemaVersion   string                `json:"schema_version"`
	CreateSessionID string                `json:"create_session_id"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	Status          string                `json:"status"`
	InputPath       string                `json:"input_path"`
	Goal            string                `json:"goal"`
	ApprovalScope   CreateApprovalScope   `json:"approval_scope"`
	Linked          CreateLinkedArtifacts `json:"linked"`
	Deliverables    CreateDeliverables    `json:"deliverables"`
	CapabilityGaps  []string              `json:"capability_gaps,omitempty"`
	Warnings        []string              `json:"warnings,omitempty"`
	Errors          []string              `json:"errors,omitempty"`
	NextCommands    []string              `json:"next_commands,omitempty"`
}

type CreateApprovalScope struct {
	Mode                 string                    `json:"mode"`
	Approved             bool                      `json:"approved"`
	AllowOverwrite       bool                      `json:"allow_overwrite"`
	AllowProviderCalls   bool                      `json:"allow_provider_calls"`
	AllowExternalNetwork bool                      `json:"allow_external_network"`
	AllowMediaWrites     bool                      `json:"allow_media_writes"`
	ExpiresAt            string                    `json:"expires_at"`
	Scope                CreateApprovalScopeTarget `json:"scope"`
}

type CreateApprovalScopeTarget struct {
	AgentPlanID string   `json:"agent_plan_id"`
	JobIDs      []string `json:"job_ids,omitempty"`
}

type CreateLinkedArtifacts struct {
	AgentPlanID       string `json:"agent_plan_id,omitempty"`
	AgentPlanPath     string `json:"agent_plan_path,omitempty"`
	AgentDecisionPath string `json:"agent_decision_path,omitempty"`
	LinkedJobsPath    string `json:"linked_jobs_path,omitempty"`
}

type CreateDeliverables struct {
	DraftVideo        string `json:"draft_video,omitempty"`
	ReviewReport      string `json:"review_report,omitempty"`
	CaptionVariants   string `json:"caption_variants,omitempty"`
	VoiceoverText     string `json:"voiceover_text,omitempty"`
	AssetRequirements string `json:"asset_requirements,omitempty"`
}

type createDeps struct {
	agentPlan   func(io.Writer, AgentPlanCommandOptions) error
	graphRun    func(io.Writer, string, AgentGraphRunOptions) error
	approvePlan func(string, io.Writer, ApproveAgentPlanOptions) error
	convertPlan func(string, io.Writer, AgentPlanToJobOptions) error
	runJob      func(string, io.Writer, JobRunOptions) error
	runWorker   func(io.Writer, JobWorkerOptions) error
	startDaemon func(io.Writer, DaemonStartOptions) error
	now         func() time.Time
}

var defaultCreateDeps = createDeps{
	agentPlan:   AgentPlanCommand,
	graphRun:    AgentGraphRunCommand,
	approvePlan: ApproveAgentPlan,
	convertPlan: AgentPlanToJob,
	runJob:      JobRun,
	runWorker:   JobWorker,
	startDaemon: DaemonStart,
	now:         func() time.Time { return time.Now().UTC() },
}

func Create(stdout io.Writer, opts CreateOptions) error {
	return createWithDeps(stdout, opts, defaultCreateDeps)
}

func CreateResult(sessionID string, stdout io.Writer, opts CreateResultOptions) error {
	session, err := readCreateSession(sessionID)
	if err != nil {
		return err
	}
	summary := buildCreateResultSummary(session)
	if opts.WriteArtifact {
		if err := writeCreateReviewSummary(summary); err != nil {
			return err
		}
		appendCreateEvent(sessionID, "CREATE_REVIEW_WRITTEN", map[string]any{"session_id": sessionID})
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	printCreateResultSummary(stdout, summary)
	return nil
}

func CreateSessions(stdout io.Writer, opts CreateSessionsOptions) error {
	sessions, err := listCreateSessions()
	if err != nil {
		return err
	}
	if opts.Status != "" {
		filtered := []CreateSession{}
		for _, session := range sessions {
			if session.Status == opts.Status {
				filtered = append(filtered, session)
			}
		}
		sessions = filtered
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if len(sessions) > opts.Limit {
		sessions = sessions[:opts.Limit]
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(sessions, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	if len(sessions) == 0 {
		fmt.Fprintln(stdout, "No create sessions found.")
		return nil
	}
	fmt.Fprintf(stdout, "%-36s %-18s %-25s %s\n", "SESSION ID", "STATUS", "CREATED", "GOAL")
	for _, session := range sessions {
		fmt.Fprintf(stdout, "%-36s %-18s %-25s %s\n", session.CreateSessionID, session.Status, session.CreatedAt.Format(time.RFC3339), truncate(session.Goal, 70))
	}
	return nil
}

func InspectCreateSession(sessionID string, stdout io.Writer, opts InspectCreateSessionOptions) error {
	session, err := readCreateSession(sessionID)
	if err != nil {
		return err
	}
	summary := buildCreateResultSummary(session)
	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	printCreateResultSummary(stdout, summary)
	return nil
}

func createWithDeps(stdout io.Writer, opts CreateOptions, deps createDeps) error {
	if strings.TrimSpace(opts.Goal) == "" {
		return fmt.Errorf("--goal is required")
	}
	if opts.InputPath == "" {
		return fmt.Errorf("create requires an input path")
	}
	if opts.RunJobs && opts.WorkerOnce {
		return fmt.Errorf("--run-jobs and --worker-once cannot both be set")
	}
	if opts.DryRun {
		session := buildCreateSession("createsession-dry-run", deps.now(), opts)
		session.Status = createStatusPlanned
		session.NextCommands = buildCreateNextCommands(session)
		if opts.JSON {
			data, _ := json.MarshalIndent(session, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintln(stdout, "Create dry-run")
		fmt.Fprintf(stdout, "  input: %s\n", opts.InputPath)
		fmt.Fprintf(stdout, "  goal:  %s\n", opts.Goal)
		fmt.Fprintf(stdout, "  scope: %s\n", session.ApprovalScope.Mode)
		return nil
	}

	now := deps.now()
	sessionID := "createsession-" + now.Format("20060102T150405.000000000Z")
	session := buildCreateSession(sessionID, now, opts)
	if err := os.MkdirAll(createSessionDir(sessionID), 0o755); err != nil {
		return err
	}
	appendCreateEvent(sessionID, "CREATE_SESSION_STARTED", map[string]any{"session_id": sessionID, "input": opts.InputPath})

	var planOut bytes.Buffer
	planOpts := createAgentPlanOptions(opts)
	if err := deps.agentPlan(&planOut, planOpts); err != nil {
		session.Status = createStatusFailed
		session.Errors = append(session.Errors, err.Error())
		_ = writeCreateSession(session)
		appendCreateEvent(sessionID, "CREATE_SESSION_FAILED", map[string]any{"error": err.Error()})
		return err
	}
	planID, err := latestAgentPlanIDForCreate()
	if err != nil {
		session.Status = createStatusFailed
		session.Errors = append(session.Errors, err.Error())
		_ = writeCreateSession(session)
		return err
	}
	session.Linked.AgentPlanID = planID
	session.Linked.AgentPlanPath = filepath.Join(agentPlansV1Root, planID, "agent_plan.json")
	session.Deliverables.AssetRequirements = filepath.Join(agentPlansV1Root, planID, assetRequirementsArtifact)
	session.ApprovalScope.Scope.AgentPlanID = planID
	appendCreateEvent(sessionID, "CREATE_PLAN_CREATED", map[string]any{"agent_plan_id": planID})
	_ = writeCreateLinkedAgentPlan(sessionID, planID)

	if !opts.SkipGraph {
		var graphOut bytes.Buffer
		if err := deps.graphRun(&graphOut, planID, AgentGraphRunOptions{WorkersDir: opts.WorkersDir, JSON: true}); err != nil {
			session.Warnings = append(session.Warnings, "graph review failed: "+err.Error())
		} else {
			session.Linked.AgentDecisionPath = filepath.Join(agentPlansV1Root, planID, "agent_decision.json")
			appendCreateEvent(sessionID, "CREATE_GRAPH_REVIEW_COMPLETED", map[string]any{"agent_plan_id": planID})
		}
	}
	session.CapabilityGaps = collectCreateCapabilityGaps(planID)

	policy, _ := readAgentPlanPolicy(planID)
	scopeBlocks := validateCreateScope(opts, session.ApprovalScope, policy)
	if len(scopeBlocks) > 0 {
		session.Status = createStatusBlocked
		session.Errors = append(session.Errors, scopeBlocks...)
		session.NextCommands = buildCreateNextCommands(session)
		_ = writeCreateSession(session)
		if opts.WriteReview {
			_ = writeCreateReview(session)
		}
		appendCreateEvent(sessionID, "CREATE_SESSION_BLOCKED", map[string]any{"errors": scopeBlocks})
		printCreateResult(stdout, session)
		return nil
	}
	appendCreateEvent(sessionID, "CREATE_APPROVAL_SCOPE_APPLIED", map[string]any{"scope": session.ApprovalScope.Mode})

	if !opts.Yes {
		session.Status = createStatusReady
		session.NextCommands = buildCreateNextCommands(session)
		_ = writeCreateSession(session)
		if opts.WriteReview {
			_ = writeCreateReview(session)
		}
		if opts.JSON {
			data, _ := json.MarshalIndent(session, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		printCreateResult(stdout, session)
		return nil
	}

	if opts.Convert {
		if err := deps.approvePlan(planID, io.Discard, ApproveAgentPlanOptions{}); err != nil {
			session.Status = createStatusFailed
			session.Errors = append(session.Errors, err.Error())
			_ = writeCreateSession(session)
			return err
		}
		appendCreateEvent(sessionID, "CREATE_PLAN_APPROVED", map[string]any{"agent_plan_id": planID})
		var convertOut bytes.Buffer
		if err := deps.convertPlan(planID, &convertOut, AgentPlanToJobOptions{
			Yes:                true,
			AllowProviderCalls: opts.AllowProviderCalls,
			AllowOverwrite:     opts.AllowOverwrite,
			ApproveJobs:        opts.ApproveJobs && createScopeCanApproveJobs(session.ApprovalScope),
		}); err != nil {
			session.Status = createStatusFailed
			session.Errors = append(session.Errors, err.Error())
			_ = writeCreateSession(session)
			appendCreateEvent(sessionID, "CREATE_SESSION_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		session.Status = createStatusConverted
		session.Linked.LinkedJobsPath = filepath.Join(agentPlansV1Root, planID, "linked_jobs.json")
		if linked, err := readAgentLinkedJobs(planID); err == nil {
			linked = refreshAgentLinkedJobsInMemory(linked)
			session.ApprovalScope.Scope.JobIDs = linkedJobIDs(linked)
			_ = writeCreateLinkedJobs(sessionID, linked)
		}
		appendCreateEvent(sessionID, "CREATE_JOBS_CONVERTED", map[string]any{"agent_plan_id": planID, "approve_jobs": opts.ApproveJobs})
		if opts.ApproveJobs {
			appendCreateEvent(sessionID, "CREATE_JOBS_APPROVED", map[string]any{"agent_plan_id": planID})
		}
	}

	if opts.RunJobs {
		session.Status = createStatusRunning
		if linked, err := readAgentLinkedJobs(planID); err == nil {
			for _, item := range refreshAgentLinkedJobsInMemory(linked).Jobs {
				job, err := readJob(item.JobID)
				if err != nil {
					session.Errors = append(session.Errors, err.Error())
					continue
				}
				if job.Status != JobStatusPending || (job.ApprovalStatus != JobApprovalApproved && job.ApprovalStatus != JobApprovalNotRequired) {
					continue
				}
				appendCreateEvent(sessionID, "CREATE_JOB_STARTED", map[string]any{"job_id": job.JobID})
				if err := deps.runJob(job.JobID, stdout, JobRunOptions{Yes: true, AllowProviderCalls: opts.AllowProviderCalls, AllowOverwrite: opts.AllowOverwrite}); err != nil {
					session.Errors = append(session.Errors, err.Error())
					appendCreateEvent(sessionID, "CREATE_JOB_FAILED", map[string]any{"job_id": job.JobID, "error": err.Error()})
					if opts.FailFast {
						break
					}
				} else {
					appendCreateEvent(sessionID, "CREATE_JOB_COMPLETED", map[string]any{"job_id": job.JobID})
				}
			}
		}
	}
	if opts.WorkerOnce {
		session.Status = createStatusRunning
		if err := deps.runWorker(stdout, JobWorkerOptions{Once: true, AllowProviderCalls: opts.AllowProviderCalls, AllowOverwrite: opts.AllowOverwrite, FailFast: opts.FailFast}); err != nil {
			session.Errors = append(session.Errors, err.Error())
		}
	}
	if opts.StartDaemon {
		if err := deps.startDaemon(stdout, DaemonStartOptions{Interval: 10 * time.Second, AllowProviderCalls: opts.AllowProviderCalls, AllowOverwrite: opts.AllowOverwrite, FailFast: opts.FailFast}); err != nil {
			session.Errors = append(session.Errors, err.Error())
		} else {
			appendCreateEvent(sessionID, "CREATE_DAEMON_STARTED", map[string]any{"session_id": sessionID})
		}
	}
	if len(session.Errors) > 0 {
		session.Status = createStatusFailed
	} else if opts.RunJobs || opts.WorkerOnce {
		session.Status = createStatusCompleted
	} else if opts.Convert {
		session.Status = createStatusConverted
	} else {
		session.Status = createStatusReady
	}
	session.NextCommands = buildCreateNextCommands(session)
	if err := writeCreateSession(session); err != nil {
		return err
	}
	if opts.WriteReview {
		_ = writeCreateReview(session)
	}
	appendCreateEvent(sessionID, "CREATE_SESSION_COMPLETED", map[string]any{"status": session.Status})
	if opts.JSON {
		data, _ := json.MarshalIndent(session, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	printCreateResult(stdout, session)
	return nil
}

func buildCreateSession(sessionID string, now time.Time, opts CreateOptions) CreateSession {
	scope := BuildApprovalScope(opts)
	return CreateSession{
		SchemaVersion:   createSessionSchema,
		CreateSessionID: sessionID,
		CreatedAt:       now,
		UpdatedAt:       now,
		Status:          createStatusPlanned,
		InputPath:       opts.InputPath,
		Goal:            opts.Goal,
		ApprovalScope:   scope,
		Deliverables:    CreateDeliverables{},
	}
}

func BuildApprovalScope(opts CreateOptions) CreateApprovalScope {
	mode := opts.ApprovalScope
	if mode == "" {
		if opts.Yes {
			mode = "local"
		} else {
			mode = "preview"
		}
	}
	return CreateApprovalScope{
		Mode:                 mode,
		Approved:             opts.Yes && mode != "preview",
		AllowOverwrite:       opts.AllowOverwrite,
		AllowProviderCalls:   opts.AllowProviderCalls && (mode == "provider" || mode == "full"),
		AllowExternalNetwork: opts.AllowExternalNetwork && (mode == "provider" || mode == "full"),
		AllowMediaWrites:     true,
	}
}

func validateCreateScope(opts CreateOptions, scope CreateApprovalScope, policy AgentPlanPolicyReview) []string {
	blocks := []string{}
	if scope.Mode == "preview" && (opts.Yes || opts.Convert || opts.RunJobs || opts.WorkerOnce || opts.StartDaemon) {
		blocks = append(blocks, "preview approval scope cannot approve, convert, run jobs, or start daemon")
	}
	if policy.RequiresProviderPermission && scope.Mode == "local" {
		blocks = append(blocks, "provider calls requested but approval_scope=local")
	}
	if policy.RequiresProviderPermission && !scope.AllowProviderCalls {
		blocks = append(blocks, "provider calls requested but --allow-provider-calls not set for provider/full scope")
	}
	if policy.RequiresExternalNetwork && !scope.AllowExternalNetwork {
		blocks = append(blocks, "external network requested but --allow-external-network not set")
	}
	if policy.RequiresOverwritePermission && !scope.AllowOverwrite {
		blocks = append(blocks, "overwrite requested but --allow-overwrite not set")
	}
	return dedupeStrings(blocks)
}

func createScopeCanApproveJobs(scope CreateApprovalScope) bool {
	return scope.Approved && scope.Mode != "preview"
}

func createAgentPlanOptions(opts CreateOptions) AgentPlanCommandOptions {
	goal := opts.Goal
	if opts.GenerateScript && !strings.Contains(strings.ToLower(goal), "script") {
		goal += " generate script"
	}
	if opts.GenerateCaptions && !strings.Contains(strings.ToLower(goal), "caption") {
		goal += " with captions"
	}
	if opts.PrepareVoiceover && !strings.Contains(strings.ToLower(goal), "voiceover") {
		goal += " prepare voiceover"
	}
	if opts.GenerateVoiceover && !strings.Contains(strings.ToLower(goal), "voiceover") {
		goal += " generate voiceover"
	}
	if opts.BurnCaptions && !strings.Contains(strings.ToLower(goal), "caption") {
		goal += " burn captions"
	}
	if opts.CaptionStyle != "" && opts.CaptionStyle != "default" {
		goal += " " + opts.CaptionStyle + " captions"
	}
	if opts.CaptionPosition != "" && opts.CaptionPosition != "auto" {
		goal += " captions " + opts.CaptionPosition
	}
	return AgentPlanCommandOptions{
		Goal:                  goal,
		InputPath:             opts.InputPath,
		JSON:                  false,
		WriteReview:           true,
		AllowProviderCalls:    opts.AllowProviderCalls && (opts.ApprovalScope == "provider" || opts.ApprovalScope == "full"),
		AllowOverwrite:        opts.AllowOverwrite,
		Platform:              opts.Platform,
		PlannerName:           opts.PlannerName,
		PlannerModel:          opts.PlannerModel,
		FallbackDeterministic: opts.FallbackDeterministic,
	}
}

func latestAgentPlanIDForCreate() (string, error) {
	plans, err := listAgentPlans()
	if err != nil {
		return "", err
	}
	if len(plans) == 0 {
		return "", fmt.Errorf("no agent plan created")
	}
	return plans[0].PlanID, nil
}

func collectCreateCapabilityGaps(planID string) []string {
	assets, err := readAssetRequirements(planID)
	if err != nil {
		return nil
	}
	gaps := []string{}
	for _, req := range assets.Requirements {
		if req.Status == "missing" || req.Status == "missing_env" {
			gaps = append(gaps, fmt.Sprintf("%s: %s", req.Capability, req.DegradedPath))
		}
	}
	return dedupeStrings(gaps)
}

func linkedJobIDs(linked AgentLinkedJobs) []string {
	ids := []string{}
	for _, item := range linked.Jobs {
		ids = append(ids, item.JobID)
	}
	return ids
}

func createSessionDir(sessionID string) string {
	return filepath.Join(createSessionsRoot, sessionID)
}

func createSessionPath(sessionID string) string {
	return filepath.Join(createSessionDir(sessionID), "create_session.json")
}

func readCreateSession(sessionID string) (CreateSession, error) {
	var session CreateSession
	err := readJSONFile(createSessionPath(sessionID), &session)
	return session, err
}

func writeCreateSession(session CreateSession) error {
	session.UpdatedAt = time.Now().UTC()
	return writeJSONFile(createSessionPath(session.CreateSessionID), session)
}

func writeCreateLinkedAgentPlan(sessionID string, planID string) error {
	payload := map[string]any{
		"agent_plan_id":   planID,
		"agent_plan_path": filepath.Join(agentPlansV1Root, planID, "agent_plan.json"),
	}
	return writeJSONFile(filepath.Join(createSessionDir(sessionID), "linked_agent_plan.json"), payload)
}

func writeCreateLinkedJobs(sessionID string, linked AgentLinkedJobs) error {
	return writeJSONFile(filepath.Join(createSessionDir(sessionID), "linked_jobs.json"), linked)
}

func writeCreateReview(session CreateSession) error {
	return writeCreateReviewSummary(buildCreateResultSummary(session))
}

func writeCreateReviewSummary(summary CreateResultSummary) error {
	session := summary.Session
	var b strings.Builder
	b.WriteString("# OpenVFX Create Review\n\n")
	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "- Session: `%s`\n", session.CreateSessionID)
	fmt.Fprintf(&b, "- Status: `%s`\n", session.Status)
	fmt.Fprintf(&b, "- Input: `%s`\n", session.InputPath)
	fmt.Fprintf(&b, "- Goal: %s\n", session.Goal)
	fmt.Fprintf(&b, "- Approval Scope: `%s` approved=`%t`\n", session.ApprovalScope.Mode, session.ApprovalScope.Approved)
	if session.Linked.AgentPlanID != "" {
		fmt.Fprintf(&b, "- Agent Plan: `%s`\n", session.Linked.AgentPlanID)
	}
	if summary.AgentDecision != nil {
		fmt.Fprintf(&b, "- Graph Decision: `%v`\n", summary.AgentDecision["decision"])
	}
	if summary.CreativeBrief != nil {
		brief := summary.CreativeBrief
		b.WriteString("\n## Creative Brief\n\n")
		fmt.Fprintf(&b, "- Platform: `%s`\n", emptyDash(brief.Platform))
		fmt.Fprintf(&b, "- Duration: `%d`\n", brief.Duration.TargetSeconds)
		fmt.Fprintf(&b, "- Style: `%s`\n", strings.Join(append(append([]string{}, brief.Style.Tone...), brief.Style.VisualStyle...), "`, `"))
		fmt.Fprintf(&b, "- Tone: `%s`\n", strings.Join(brief.Style.Mood, "`, `"))
		fmt.Fprintf(&b, "- Pacing: `%s` %s\n", emptyDash(brief.Pacing.CutStyle), brief.Pacing.FirstSecondsNote)
		fmt.Fprintf(&b, "- Captions: required=`%t`, style=`%s`, position=`%s`\n", brief.Captions.Required, emptyDash(brief.Captions.Style), emptyDash(brief.Captions.Position))
		fmt.Fprintf(&b, "- Voiceover: requested=`%t`, talking_clip=`%t`\n", brief.Requests.Voiceover, brief.Requests.UseTalkingClipNarration)
		fmt.Fprintf(&b, "- Visual requests: generated_broll=`%t`, visual_transform=`%t`\n", brief.Requests.GeneratedBRoll, brief.Requests.VisualTransform)
	}
	b.WriteString("\n## Planned Deliverables\n\n")
	b.WriteString("| Deliverable | Required | Status | Artifact |\n|---|---:|---|---|\n")
	if summary.Deliverables != nil && len(summary.Deliverables.Deliverables) > 0 {
		for _, item := range summary.Deliverables.Deliverables {
			fmt.Fprintf(&b, "| %s | yes | planned | %s |\n", item.Title, emptyDash(item.Format))
		}
	} else {
		b.WriteString("| Draft video | yes | planned | - |\n")
	}
	b.WriteString("\n## Asset Requirements\n\n")
	b.WriteString("| Asset | Capability | Status | Route | Prompt/Description |\n|---|---|---|---|---|\n")
	if summary.AssetRequirements != nil && len(summary.AssetRequirements.Requirements) > 0 {
		for _, req := range summary.AssetRequirements.Requirements {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", req.Kind, req.Capability, req.Status, emptyDash(req.Route), req.Description)
		}
	} else {
		b.WriteString("| Source media | source_media | planned | - | Use provided input media. |\n")
	}
	b.WriteString("\n## Visual Generation Dry-Run Requests\n\n")
	b.WriteString("| Request | Capability | Status | Route | Backend | Prompt |\n|---|---|---|---|---|---|\n")
	if summary.VisualRequests != nil && len(summary.VisualRequests.Requests) > 0 {
		for _, req := range summary.VisualRequests.Requests {
			prompt := ""
			if raw, ok := req.RequestPreview["prompt"]; ok {
				prompt = fmt.Sprint(raw)
			}
			backend := req.Backend
			if req.Provider != "" || req.Model != "" {
				backend = strings.TrimSpace(strings.Join([]string{req.Backend, req.Provider, req.Model}, " "))
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", req.ID, req.Capability, req.Status, emptyDash(req.Route), emptyDash(backend), truncate(prompt, 120))
		}
	} else {
		b.WriteString("| - | - | not_requested | - | - | No visual generation dry-run requests recorded. |\n")
	}
	b.WriteString("\n## Capability Gaps\n\n")
	b.WriteString("| Capability | Requested | Status | Message | Suggested Fix |\n|---|---:|---|---|---|\n")
	if len(session.CapabilityGaps) > 0 {
		for _, gap := range session.CapabilityGaps {
			fmt.Fprintf(&b, "| %s | yes | missing | %s | Configure a matching tools backend/route or use degraded path. |\n", strings.Split(gap, ":")[0], gap)
		}
	} else {
		b.WriteString("| - | no | ok | No capability gaps recorded. | - |\n")
	}
	b.WriteString("\n## Agent Plan\n\n")
	if summary.AgentPlan != nil {
		fmt.Fprintf(&b, "- Plan ID: `%s`\n", summary.AgentPlan.PlanID)
		if summary.Policy != nil {
			fmt.Fprintf(&b, "- Policy Status: `%s`\n", summary.Policy.Status)
		}
		fmt.Fprintf(&b, "- Planner: `%s/%s`\n", summary.AgentPlan.Planner.Mode, summary.AgentPlan.Planner.Version)
		fmt.Fprintf(&b, "- Fallback used: `%t`\n", summary.AgentPlan.Planner.FallbackUsed)
		for _, warning := range summary.AgentPlan.Warnings {
			fmt.Fprintf(&b, "- Warning: %s\n", warning)
		}
	}
	b.WriteString("\n## Jobs\n\n")
	b.WriteString("| Job ID | Type | Status | Approval | Source Action | Outputs |\n|---|---|---|---|---|---|\n")
	if len(summary.LinkedJobs) > 0 {
		for _, job := range summary.LinkedJobs {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", job.JobID, job.Type, job.Status, job.ApprovalStatus, job.SourceAction, formatOutputKeys(job.Output))
		}
	} else {
		b.WriteString("| - | - | not_created | - | - | - |\n")
	}
	b.WriteString("\n## Outputs\n\n")
	fmt.Fprintf(&b, "- Draft video: `%s`\n", emptyDash(summary.Outputs.DraftVideo))
	if summary.GeneratedAssets != nil {
		for _, output := range sortedGeneratedAssetOutputs(*summary.GeneratedAssets) {
			fmt.Fprintf(&b, "- Generated visual asset: `%s`\n", output)
		}
	}
	fmt.Fprintf(&b, "- Captions: `%s`\n", emptyDash(summary.Outputs.Captions))
	fmt.Fprintf(&b, "- Script: `%s`\n", emptyDash(summary.Outputs.Script))
	fmt.Fprintf(&b, "- Voiceover text: `%s`\n", emptyDash(summary.Outputs.VoiceoverText))
	fmt.Fprintf(&b, "- Voiceover audio: `%s`\n", emptyDash(summary.Outputs.VoiceoverAudio))
	for _, report := range summary.Outputs.ResultReports {
		fmt.Fprintf(&b, "- Result report: `%s`\n", report)
	}
	if len(summary.NextCommands) > 0 {
		b.WriteString("\n## Next Commands\n\n```sh\n")
		for _, cmd := range summary.NextCommands {
			fmt.Fprintf(&b, "%s\n", cmd)
		}
		b.WriteString("```\n")
	}
	return os.WriteFile(filepath.Join(createSessionDir(session.CreateSessionID), "create_review.md"), []byte(b.String()), 0o644)
}

func printCreateResult(stdout io.Writer, session CreateSession) {
	printCreateResultSummary(stdout, buildCreateResultSummary(session))
}

func printCreateResultSummary(stdout io.Writer, summary CreateResultSummary) {
	session := summary.Session
	fmt.Fprintln(stdout, "Create session")
	fmt.Fprintf(stdout, "  session id: %s\n", session.CreateSessionID)
	fmt.Fprintf(stdout, "  status:     %s\n", session.Status)
	fmt.Fprintf(stdout, "  input:      %s\n", session.InputPath)
	fmt.Fprintf(stdout, "  goal:       %s\n", session.Goal)
	fmt.Fprintf(stdout, "  scope:      %s approved=%t\n", session.ApprovalScope.Mode, session.ApprovalScope.Approved)
	if session.Linked.AgentPlanID != "" {
		fmt.Fprintf(stdout, "  agent plan: %s\n", session.Linked.AgentPlanID)
	}
	if summary.AgentDecision != nil {
		fmt.Fprintf(stdout, "  decision:   %v\n", summary.AgentDecision["decision"])
	}
	if summary.CreativeBrief != nil {
		fmt.Fprintf(stdout, "  platform:   %s\n", summary.CreativeBrief.Platform)
		fmt.Fprintf(stdout, "  duration:   %d\n", summary.CreativeBrief.Duration.TargetSeconds)
	}
	if session.Deliverables.AssetRequirements != "" {
		fmt.Fprintf(stdout, "  assets:     %s\n", session.Deliverables.AssetRequirements)
	}
	if summary.VisualRequests != nil {
		fmt.Fprintf(stdout, "  visual dry-run: %d request(s), %d missing\n", len(summary.VisualRequests.Requests), len(summary.VisualRequests.MissingCapabilities))
	}
	if len(summary.LinkedJobs) > 0 {
		fmt.Fprintf(stdout, "  jobs:       %d\n", len(summary.LinkedJobs))
		for _, job := range summary.LinkedJobs {
			fmt.Fprintf(stdout, "    - %s %s approval=%s\n", job.JobID, job.Status, job.ApprovalStatus)
		}
	}
	if summary.Outputs.DraftVideo != "" {
		fmt.Fprintf(stdout, "  draft:      %s\n", summary.Outputs.DraftVideo)
	}
	if summary.GeneratedAssets != nil {
		fmt.Fprintf(stdout, "  generated visual assets: %d (%s)\n", len(summary.GeneratedAssets.Assets), summary.GeneratedAssets.Status)
	}
	for _, gap := range session.CapabilityGaps {
		fmt.Fprintf(stdout, "  gap:        %s\n", gap)
	}
	for _, warning := range session.Warnings {
		fmt.Fprintf(stdout, "  warning:    %s\n", warning)
	}
	for _, errText := range session.Errors {
		fmt.Fprintf(stdout, "  error:      %s\n", errText)
	}
	for _, cmd := range session.NextCommands {
		fmt.Fprintf(stdout, "  next:       %s\n", cmd)
	}
}

func buildCreateResultSummary(session CreateSession) CreateResultSummary {
	summary := CreateResultSummary{
		Session:      session,
		NextCommands: append([]string{}, session.NextCommands...),
	}
	if session.Linked.AgentPlanID != "" {
		if plan, err := readAgentPlan(session.Linked.AgentPlanID); err == nil {
			summary.AgentPlan = &plan
		}
		if policy, err := readAgentPlanPolicy(session.Linked.AgentPlanID); err == nil {
			summary.Policy = &policy
		}
		if brief, err := readCreativeBrief(session.Linked.AgentPlanID); err == nil && brief.SchemaVersion != "" {
			summary.CreativeBrief = &brief
		}
		if deliverables, err := readDeliverables(session.Linked.AgentPlanID); err == nil && deliverables.SchemaVersion != "" {
			summary.Deliverables = &deliverables
		}
		if assets, err := readAssetRequirements(session.Linked.AgentPlanID); err == nil && assets.SchemaVersion != "" {
			summary.AssetRequirements = &assets
		}
		if visualRequests, err := readVisualRequestsDryRun(session.Linked.AgentPlanID); err == nil && visualRequests.SchemaVersion != "" {
			summary.VisualRequests = &visualRequests
		}
		if generatedAssets, err := readGeneratedAssets(session.Linked.AgentPlanID); err == nil && generatedAssets.SchemaVersion != "" {
			summary.GeneratedAssets = &generatedAssets
		}
		decisionPath := filepath.Join(agentPlansV1Root, session.Linked.AgentPlanID, "agent_decision.json")
		if decision, err := readJSONMap(decisionPath); err == nil {
			summary.AgentDecision = decision
		}
		if linked, err := readAgentLinkedJobs(session.Linked.AgentPlanID); err == nil {
			linked = refreshAgentLinkedJobsInMemory(linked)
			for _, item := range linked.Jobs {
				view := CreateJobResultView{
					JobID:          item.JobID,
					Type:           item.JobType,
					Status:         item.Status,
					ApprovalStatus: item.ApprovalStatus,
					SourceAction:   item.ActionID,
					Output:         item.Output,
					Errors:         item.Errors,
					Warnings:       item.Warnings,
				}
				summary.LinkedJobs = append(summary.LinkedJobs, view)
				mergeCreateOutputs(&summary.Outputs, item)
			}
		}
	}
	if session.Linked.AgentPlanID != "" {
		summary.Outputs.ResultReports = append(summary.Outputs.ResultReports, filepath.Join(agentPlansV1Root, session.Linked.AgentPlanID, "plan_review.md"))
	}
	if session.CreateSessionID != "" {
		summary.Outputs.ResultReports = append(summary.Outputs.ResultReports, filepath.Join(createSessionDir(session.CreateSessionID), "create_review.md"))
	}
	if len(summary.NextCommands) == 0 {
		summary.NextCommands = buildCreateNextCommands(session)
	}
	return summary
}

func mergeCreateOutputs(outputs *CreateOutputSummary, item AgentLinkedJobItem) {
	for key, value := range item.Output {
		text := fmt.Sprint(value)
		switch key {
		case "draft_path", "draft_video":
			outputs.DraftVideo = text
		case "captions", "caption_path", "captions_path":
			outputs.Captions = text
		case "script", "script_path":
			outputs.Script = text
		case "voiceover_text", "voiceover_text_path":
			outputs.VoiceoverText = text
		case "voiceover_audio", "voiceover_audio_path", "generated_voiceover_audio_path":
			outputs.VoiceoverAudio = text
		case "make_id":
			outputs.ResultReports = append(outputs.ResultReports, "byom-video make-result "+text)
		case "run_id":
			outputs.ResultReports = append(outputs.ResultReports, "byom-video inspect "+text)
		}
	}
}

func readJSONMap(path string) (map[string]any, error) {
	var out map[string]any
	err := readJSONFile(path, &out)
	return out, err
}

func listCreateSessions() ([]CreateSession, error) {
	entries, err := os.ReadDir(createSessionsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []CreateSession{}, nil
		}
		return nil, err
	}
	sessions := []CreateSession{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		session, err := readCreateSession(entry.Name())
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})
	return sessions, nil
}

func formatOutputKeys(output map[string]any) string {
	if len(output) == 0 {
		return "-"
	}
	keys := []string{}
	for key := range output {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func buildCreateNextCommands(session CreateSession) []string {
	next := []string{}
	if session.Linked.AgentPlanID != "" {
		next = append(next, fmt.Sprintf("byom-video review-agent-plan %s", session.Linked.AgentPlanID))
		next = append(next, fmt.Sprintf("byom-video agent-result %s", session.Linked.AgentPlanID))
	}
	switch session.Status {
	case createStatusReady:
		next = append(next, fmt.Sprintf("byom-video create-result %s", session.CreateSessionID))
		if session.Linked.AgentPlanID != "" {
			next = append(next, fmt.Sprintf("byom-video agent-run %s --yes --convert --approve-jobs", session.Linked.AgentPlanID))
		}
	case createStatusConverted:
		next = append(next, fmt.Sprintf("byom-video create-result %s", session.CreateSessionID))
		next = append(next, "byom-video job-worker --once")
	case createStatusBlocked, createStatusFailed:
		next = append(next, fmt.Sprintf("byom-video create-result %s", session.CreateSessionID))
	}
	return dedupeStrings(next)
}

func appendCreateEvent(sessionID string, eventType string, details map[string]any) {
	log, err := events.Open(filepath.Join(createSessionDir(sessionID), "events.jsonl"))
	if err != nil {
		return
	}
	defer log.Close()
	_ = log.Write(eventType, details)
}
