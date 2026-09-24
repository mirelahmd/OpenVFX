# OpenVFX — Agent Working Context

> Audit date: 2026-09-21. Written for AI coding agents and the maintainer.
> Read this before changing anything. It records what is **actually real** in this
> repository versus what merely exists as files.

---

## 0c. STATUS — Installer + release packaging SHIPPED (2026-09-23, Prompt 074)

One-command install, no Go and no clone:

```sh
curl -fsSL https://raw.githubusercontent.com/mirelahmd/OpenVFX/main/install.sh | sh
```

Release archives carry the CLI **and** the Python agent sidecar.
`scripts/build-release.sh` refuses to build if
`workers/openvfx_agent_graph/creative` is missing — a release must never ship a
CLI with no Creative Director.

**Runtime path resolution is now installed-location-first**
(`internal/production/runtime.go`): `ResolveWorkersDir` and
`ResolveSidecarPython` check `$OPENVFX_HOME` / `$XDG_DATA_HOME` /
`~/.local/share/openvfx` / `/usr/local/share/openvfx` before any checkout probe.
A configured `python.interpreter` is honoured only if the file exists, so a
stale `byom-video.yaml` cannot break an installed CLI. Do not reintroduce
CWD-relative sidecar lookup as the primary path.

Installed layout:

```
~/.local/bin/openvfx                         (or /usr/local/bin when writable)
~/.local/bin/byom-video -> openvfx           compatibility symlink
~/.local/share/openvfx/<version>/workers/    Python agent sidecar
~/.local/share/openvfx/current -> <version>  what the CLI resolves
~/.local/share/openvfx/venv/                 isolated agent environment
```

`scripts/smoke-install.sh` is the guard: throwaway `HOME`, dev env vars unset,
installer piped through stdin, run from an unrelated directory, and it asserts
the resolved sidecar is **not** the source checkout. Run it after any change to
path resolution or packaging.

Version pinning through a pipe must put the variable **after** the pipe
(`| OPENVFX_VERSION=v0.2.0 sh`); before `curl` it is scoped to curl.

Caveats: faster-whisper is **not** installed by default (large; opt in with
`OPENVFX_WITH_TRANSCRIBE=1`), no release is published yet, and only
darwin/arm64 of the four cross-built archives has actually been executed.

Public name on the release surface is now `openvfx`; internals stay
`byom-video` (§13 naming boundary — do not mass-rename).

---

## 0b. STATUS — Creative Director SHIPPED (2026-09-22, Prompt 073)

`produce` now runs a **Creative Director** (LangGraph, `workers/openvfx_agent_graph/creative/`)
before deterministic planning. A rich creative request becomes
`creative_treatment.json`, and that treatment drives the real edit.

**§5's verdict is now false for the `produce` path in both planning and
reasoning**; it remains true for every other command.

Invariants that must not be broken:

1. **The director decides WHAT; Go keeps all execution authority.** LangGraph
   produces a treatment and nothing else.
2. **Treatment output is untrusted input.** `CreativeTreatment.Validate` rejects
   hallucinated asset ids, inverted ranges, out-points past a clip end and
   dangling decision refs. A failing treatment is discarded whole, never partly
   applied.
3. **`semantic_reasoning` is true only when a model actually completed a call.**
   Rules-based output claiming it is rejected by the validator. Modes: `llm`,
   `deterministic`, unavailable.
4. **One critique pass, bounded structurally** — the graph is linear with no
   edge back into itself.
5. **Config resolution stays in Go.** It resolves `models.routes.creative_director`
   and hands Python a JSON config; the sidecar never parses YAML, never picks a
   provider, and imports no vendor SDK (`llm.py` is stdlib urllib).

Provenance chain now runs end to end:
`dec_0001 "Start 1s into clip_1" → seg_0001 source_in 1.0 → ffmpeg -ss 1.000`.

Tests: 61 Python (31 new), 24 Go packages. Smoke: `scripts/smoke-creative-director.sh`.
Docs: `docs/artifacts/creative-treatment.md`, `docs/production.md`.

**Honest gap: the LLM route has never run against a real model.** Ollama is not
installed on this machine; verification used scripted stubs and a stub HTTP
endpoint. Prompt quality is unproven — that is milestone one below.

`langgraph` is now installed in both `.venv` and `~/.byom-venv` and is a hard
requirement for the director; without it `produce` degrades to intent-only
planning and prints why.

---

## 0a. STATUS — the vertical slice SHIPPED (2026-09-21)

`byom-video produce` is built, tested and verified end-to-end. **Section 5's
verdict ("there is no OBSERVE→PLAN→EXECUTE→VALIDATE→REVISE loop") is now false
for the `produce` path** and remains true for every other command.

New package `internal/production/` + `internal/commands/produce.go`.
New root `.byom-video/productions/<production_id>/`. New commands: `produce`,
`productions`, `replay`. Nothing existing was modified except CLI dispatch,
usage text, README, and the stale `Makefile` MODULE var.

Verified on this machine with real execution (no mocks anywhere):

```
observe    3 assets probed by real ffprobe
capabilities  ffmpeg.filter.subtitles UNAVAILABLE (libass not compiled in)
plan v1    6 typed stages, each declaring cpu/memory/gpu/runtime/image-hint
execute    stage_0006 BLOCKED: ffmpeg.filter.subtitles unavailable
revise     rev_0001 capability_unavailable -> sidecar.srt; 5 of 6 stages reused
validate   7 assertions probed against the real output file
complete   completed_degraded, 0 external network calls
replay     8 commands re-issued from persisted argv, byte-identical output
```

Both revision triggers work:
- `capability_unavailable` — subtitles→sidecar substitution (real gap on this ffmpeg)
- `validation_failed` — 18.04s FAIL → corrective pass → 30.04s PASS

**Key design decision made during implementation:** the executor does **not**
substitute backends on its own. It binds to the declared capability or blocks.
Substitution is the reviser's job, because choosing a different means to the
same end is a planning decision and must carry a recorded reason. An executor
that quietly degraded work would reproduce exactly the problem this slice
exists to fix. See `TestBindRefusesToSubstituteSilently`.

Second such decision: the planner never sets `ExtendToTarget` on a first pass.
Looping footage to hit a duration target is a creative choice; doing it silently
would hide that the source could not cover the brief. It is authorised only by
a revision, after the shortfall is recorded as a failed assertion. See
`TestPlannerDoesNotAutoExtendOnFirstPass`.

Docs: [docs/production.md](docs/production.md) — includes an honest limitations list.
Smoke: `scripts/smoke-produce.sh` (asserts plan v2 + rev_0001 on a no-libass machine).
Fixtures: `scripts/make-produce-fixtures.sh` — synthesises assets with real
speech via `say`/`espeak`; commits no binaries.

Tests: 29 packages pass, including 30 new tests in `internal/production/`.

### What the slice did NOT do (still open)

- The 11 hardcoded CWD-relative artifact roots (`produce` added a 12th, but it
  is the only one that keeps everything under one id).
- The dead `creative-execute-stub` inside `make --yes`.
- The five overlapping plan schemas.
- No OpenTimelineIO, no timecode/frames, no color management — see §12.

---

## 0. What this project is

OpenVFX is a **local-first agentic control plane for media/VFX production**.
It is not a timeline editor, not a wrapper around one generative model, not a chat UI.

Long-term loop:

```
brief + assets → observe → plan → execute typed stages → persist → validate
              → reason about failure/gaps → revise → final media + provenance
```

- Go module: `github.com/mirelahmd/OpenVFX`
- Binary: `byom-video` (`go build -o byom-video ./cmd/byom-video`)
- Config: `byom-video.yaml` at project root
- Artifact store: `.byom-video/` (gitignored)
- **Zero external Go dependencies.** No `go.sum`. Hand-rolled CLI (a 690-case
  `switch` in `internal/cli/root.go`). Keep it that way unless there is a hard reason.

---

## 1. CURRENT REALITY (audited, not inferred)

Verified on this machine: `go build ./...` clean, `go test ./...` **all 28 packages pass**
(940 test functions), ffmpeg 8.1 + ffprobe on PATH, `faster_whisper` importable in `.venv`,
`langgraph` **not** installed, Ollama **not** running.

### COMPLETE — real execution, verified end-to-end

| Thing | Where | Evidence |
|---|---|---|
| ffprobe metadata extraction | `internal/media/ffprobe.go` | real subprocess, `metadata.json` written |
| Real transcription (faster-whisper) | `workers/byom_video_workers/transcribe.py` via `internal/workers/python.go` | produced `"mode":"real","engine":"faster-whisper"` transcript in smoke run |
| Deterministic chunk → highlight → roughcut | `internal/{chunks,highlights,roughcut}` | real artifacts + per-stage validators |
| SRT caption generation | `internal/captions` | `captions.srt` written from transcript |
| **Real ffmpeg render** (clip extract → concat → re-encode → scale/pad → amix → subtitle burn) | `internal/commands/creative_assemble.go` | produced a real `draft.mp4`, 4.50s, 1280x720 |
| `make` end-to-end | `internal/commands/make.go` | full pipeline→plan→timeline→assemble in one command, verified in a clean workspace |
| Per-run event log + manifest | `internal/events`, `internal/manifest` | `events.jsonl` JSONL trace + `manifest.json` artifact index |
| Job queue + worker + daemon | `internal/commands/{job,job_run,job_worker,daemon}.go` | durable JSON jobs, approval gate, file-lock worker, PID daemon |
| Generic HTTP provider execution | `internal/commands/visual_execution.go` | config-driven, no vendor SDK, writes scrubbed request/response audits. Tested against fal.ai (`fal-ai/cogvideox-5b`) per `working_instructions.md` |
| Ollama text generation | `internal/modelrouter/ollama.go` | real `/api/generate` call (server currently down) |
| Synthetic timeline fallback | `internal/commands/timeline_source.go` | ffprobe-driven clip synthesis for silent/generated video |

### PARTIAL

- **`agent-plan` (v1 contract)** — `internal/commands/agent_plan_v1.go`. Real schema, real
  policy review, real approval/conversion to jobs. But `buildAgentActions` emits **exactly one
  action**, chosen from 4 types (`make` / `revise_make` / `validate_creative_assemble` /
  `queue_health`) by `strings.Contains` on the goal. It is a router, not a planner.
- **Capability registry** — `internal/commands/creative_tools.go`. Config schema, validation,
  and route resolution are real. Requirement *detection* is keyword substring matching
  (`detectCapabilityRequirements`, line ~777). Status is `satisfied`/`missing` from config only —
  it never probes whether the tool actually works.
- **`doctor`** — real probing of ffmpeg filters, python modules, binaries. **Prints to stdout only.**
  No machine-readable capability artifact. This is a cheap, high-value gap.
- **Validation** — `internal/runvalidate` + `ValidateCreativeAssemble` check *schema and file
  existence*. They never check the output against the *goal*. A run that silently dropped
  captions still reports `validation: ok`.
- **Revision** — `revise-make`, `revise-plan`, `revise-mask` exist and work, but are
  **user-initiated and keyword-parsed**. Nothing revises automatically on failure.

### STUB (writes JSON, nothing consumes it)

- `creative-execute-stub` (`creative_stub_execution.go`) — writes `composition_plan.json` and
  `caption_plan.json`. **It is step 4 of `make --yes` and its output is never read by the
  ffmpeg path.** Pure decoration in the critical path.
- `mask.go` / `mask_decisions.go` / `mask_revision.go` (~2700 lines) — "inference mask"
  artifacts. No execution consumes them.
- `expansion_stub.go`, `verify_expansions.go` — expansion task plumbing, no live expander.
- `editor_artifacts.go`, `export_handoff.go`, `goalartifacts` — handoff JSON writers.
- Timeline tracks `track_voiceover` and `track_visual_overlays` emit
  `"placeholder — no audio generated"` items that the renderer ignores.

### BROKEN

- **LangGraph sidecar** (`workers/openvfx_agent_graph/`, `agent-graph-run`) —
  `No module named 'langgraph'`. Non-fatal (warns and continues). Note: even when installed
  it contains **no LLM** — the five nodes are a deterministic schema validator wearing a
  graph costume.
- **`Makefile` MODULE var is stale** — `github.com/mirelahmd/byom-video`, but `go.mod` says
  `github.com/mirelahmd/OpenVFX`. Version ldflags therefore silently do nothing.
- **README install instructions point at a repo named `byom-video`** that no longer matches
  the module path.

### DUPLICATED

Five overlapping "plan" concepts, each with its own schema, root directory and lifecycle:

| Concept | Root | Schema |
|---|---|---|
| legacy agent plan | `.byom-video/plans` | `internal/agent.Plan` |
| agent plan v1 | `.byom-video/agent_plans` | `openvfx_agent_plan.v1` |
| creative plan | `.byom-video/creative_plans` | `creative_plan.v1` |
| make summary | `.byom-video/makes` | `make_summary.v1` |
| create session | `.byom-video/create_sessions` | `create_session.v1` |

Plus `runs/`, `jobs/`, `batches/`, `queue/`, `daemon/`, `worker/`, `watch/`.
**11 independent artifact roots, every one a hardcoded CWD-relative string literal.**

Three separate `events.jsonl` files (run, creative plan, job) with no correlation ID between them.

### DOCUMENTATION-ONLY

- `docs/` has 36 files + 45 artifact schema docs. Many describe artifacts nothing executes.
- `docs/architecture.md` is 18 lines.
- `PROGRESS.md` is 10,052 lines / 483KB of prompt-by-prompt handoffs (prompts 001–071).
  It is a changelog, not an architecture document.
- **Zero container work.** No Dockerfile, no compose, no image references, no resource
  declarations anywhere in the codebase.

---

## 2. CURRENT ARCHITECTURE (as actually implemented)

```
media file + --goal string
   │
   ▼
byom-video make            internal/commands/make.go — a hardcoded 6-step function
   │
   ├─1─ Run()              internal/commands/run.go — a fixed if-chain, NOT plan-driven:
   │                       ffprobe → whisper → captions → chunks → highlights →
   │                       roughcut → ffmpeg_commands.sh → report.html
   │                       writes .byom-video/runs/<run_id>/{manifest,events.jsonl,*.json}
   │
   ├─3─ CreativePlanCommand()   keyword-match goal → capability table
   │                            writes .byom-video/creative_plans/<plan_id>/creative_plan.json
   │
   ├─4─ CreativeExecuteStub()   writes 2 JSON files nobody reads  ← DEAD PATH
   │
   ├─5─ CreativeTimeline()      roughcut.json → creative_timeline.json (4 tracks)
   │    CreativeRenderPlan()    → creative_render_plan.json
   │
   └─6─ CreativeAssemble()      ← THE ONLY REAL MEDIA EXECUTION
                                reads creative_timeline.json (NOT creative_plan.json)
                                ffmpeg per clip → concat → [amix] → [scale/pad] → [subtitles]
                                writes outputs/draft.mp4 + creative_assemble_result.json
   │
   ▼
ValidateCreativeAssemble()  schema + file-existence only
writeMakeSummary()          .byom-video/makes/<make_id>/make_summary.json
```

**The single most important architectural fact:**

> The plan is not the program.
>
> `creative_plan.json` contains `steps[]` with `status: "stub_completed"`.
> Those steps did not drive anything. The actual ffmpeg work is driven by
> `creative_timeline.json` plus Go struct flags (`CreativeAssembleOptions`).
> A reader of the plan cannot reconstruct what ran.

### Persistence and replay semantics

- **Write-through, append-only, filesystem-as-database.** Correct instinct, well executed
  at the leaf level. No DB, and none is needed.
- **Replay: none.** `rerun` re-executes the source from scratch into a *new* run id. There is
  no stage-level resume, no idempotency key, no way to re-run only a failed stage.
- **Command provenance: none.** `ffmpeg_commands.sh` is generated by the *export* path and is
  unrelated to the args `creative-assemble` actually executed. The real argv is never persisted.

---

## 3. ARCHITECTURAL DEBT (only what blocks the vertical slice)

1. **Plan/execution disconnect** — BLOCKING. Plan steps and executed stages are separate
   data with no linkage. This defeats the entire north-star claim.
2. **No machine-readable capability state** — BLOCKING. `doctor` prints; nothing can reason
   over it. Capability "status" in creative plans reflects config, not reality.
3. **Goal-blind validation** — BLOCKING. Cannot detect "we didn't deliver what was asked",
   so there is nothing for a reviser to react to.
4. **No correlation across artifact roots** — BLOCKING for a single replayable run record.
5. **Executed argv not persisted** — BLOCKING for provenance credibility with VFX engineers.
6. **11 hardcoded CWD-relative artifact roots** — NOT blocking; the slice gets one new root.
   Do not refactor the existing ten.
7. **Dead stub path inside `make --yes`** — NOT blocking; leave `make` alone.
8. **No resource/runtime declarations anywhere** — blocks the container story, cheap to fix
   *declaratively* inside the new plan schema.

Loops / unclear ownership: no runaway loops exist (nothing is iterative yet). Ownership is
unclear between `make`, `create`, `agent-run`, and `job-run` — four entry points that all
ultimately call `Make()`. Do not try to unify them in this slice.

---

## 4. PERSISTENCE ASSESSMENT — what survives process termination

For one `make` run today, on disk, you **can** answer:

- ✅ what the user asked for — `make_summary.json.goal`
- ✅ what the input asset was — `manifest.json.input_path` + `metadata.json` (full ffprobe)
- ✅ what pipeline stages ran — `runs/<id>/events.jsonl` (`*_STARTED`/`*_COMPLETED`/`*_SKIPPED`)
- ✅ what intermediate artifacts exist — `manifest.json.artifacts[]`
- ✅ what failed — `CREATIVE_ASSEMBLE_CAPTIONS_FAILED` events + `warnings[]`
- ✅ what the final output is — `outputs/draft.mp4` + `creative_assemble_result.json.stages[]`

You **cannot** answer:

- ❌ **what plan was created** — `creative_plan.json.steps[]` describes nothing that executed
- ❌ **what commands/models actually ran** — no argv, no tool version per stage, no model id
- ❌ **what validation occurred** — only pass/fail of a schema check; no assertions, no evidence
- ❌ **whether a revision happened** — no revision record unless a human ran `revise-make`
- ❌ **why the final result looks the way it does** — the causal chain
  (goal → capability gap → substitution → output) is not recorded anywhere
- ❌ **one run id that spans all of it** — `make_id`, `run_id`, `plan_id` are three ids in
  three trees; the only link is fields inside `make_summary.json`

**Smallest missing pieces:** (a) a stage-level execution record carrying argv + exit code +
duration + tool version; (b) a goal-derived assertion set with results; (c) a revision record
holding `{from_plan, to_plan, trigger, reason}`; (d) one production id owning all of it.

---

## 5. AGENTIC ASSESSMENT — ruthless

**There is no Observe → Plan → Execute → Validate → Revise loop today.**

| Phase | Reality |
|---|---|
| OBSERVE | Partial and real. ffprobe runs; `context_snapshot.json` captures runtime/queue state. But asset observation is single-file only and capability observation is config-derived, not probed. |
| PLAN | **Not planning.** `buildAgentActions` is a 4-branch `switch` over `strings.Contains(goal, ...)` that emits one action. `detectCapabilityRequirements` is ~12 substring rules. The optional Ollama planner (`agent_planner_ollama.go`) exists and can emit actions, but it selects from the same 4 types — an LLM choosing between `make` and `queue_health`. |
| EXECUTE | Real and good — but executes hardcoded Go call chains, not the plan. |
| VALIDATE | Schema/file-existence only. Goal-blind. |
| REVISE | Human-triggered keyword parsing. No automatic trigger, no bounded retry. |
| COMPLETE | `make_summary.json` — a report, not a closure of the loop. |

The LangGraph sidecar looks agentic in the source tree and is not: `nodes.py` contains regex
validation and a `decide_node` of `if issues: "repair"`. There is no model in the graph.

**Honest summary:** OpenVFX today is an **excellent artifact-first deterministic media
pipeline** with an agentic-looking naming convention layered on top. The execution spine is
genuinely good and worth preserving. The agent loop is aspirational.

This is a *fixable* gap, not a rewrite: the executors are solid, they are just not addressed
by a plan.

---

## 6. CONTAINER / PORTABILITY ASSESSMENT

Target: `typed stage → declared requirements → portable execution → artifact → telemetry`.

Current state per link:

| Link | Status |
|---|---|
| typed stage | ❌ no stage type exists; execution is Go function calls |
| declared requirements | ❌ nothing anywhere declares cpu / memory / gpu / runtime / model |
| portable execution | ⚠️ partial by accident — every external call is already a subprocess or plain HTTP with zero SDK coupling. That is genuinely container-friendly. |
| artifact output | ✅ strong — every stage already writes files to a known directory |
| telemetry | ⚠️ events exist; no duration, exit code, resource usage, or tool version per stage |

Blockers to containerization, in order:
1. Artifact roots are CWD-relative literals → a container would need the exact working dir.
2. `media.FindExecutable("ffmpeg")` assumes host PATH.
3. Python interpreter resolution is host-specific (`BYOM_VIDEO_PYTHON` → config → `python3`).
4. No requirement declaration means a scheduler has nothing to place against.

Good news: (4) is pure schema work and costs almost nothing. Declaring requirements on a
typed stage is the entire credibility win; actually running in containers is not needed for
the 2027 story yet.

---

## 7. TWO-DAY SHIP TARGET — `openvfx produce`

**One new command. One new package. One new artifact root. Zero changes to existing behavior.**

```sh
byom-video produce ./assets \
  --brief "Cut these clips into a 20-second vertical teaser with burned-in captions"
```

Pipeline:

```
OBSERVE   ffprobe every asset in the directory  → observation.json
          probe real capabilities (ffmpeg filter list, python modules,
          configured backends, reachability)   → capabilities.json

PLAN      deterministic planner emits a typed stage DAG where every stage declares
          its required capability AND its resource/runtime requirements
                                               → plan/v1/production_plan.json

EXECUTE   a real plan interpreter walks the DAG, binds each stage to an available
          backend, and dispatches to EXISTING Go executors
                                               → stages/<stage_id>/execution.json
                                                 (argv, exit code, duration, tool version)
                                               → stages/<stage_id>/<media artifacts>

VALIDATE  assertions derived from the brief, checked by ffprobe against the real output
                                               → validation/v1/validation.json

REVISE    on capability-unavailable or failed assertion, emit a revised plan with the
          substituted binding + the reason, re-execute only affected stages
                                               → plan/v2/production_plan.json
                                               → revisions/rev_0001.json

COMPLETE  → run_record.json  (one id spanning everything)
          → handoff.md       (human-readable provenance)
          → output/final.mp4
```

All under one root: `.byom-video/productions/<production_id>/`.

**Real execution only.** Stages 1–4 are ffmpeg/ffprobe/faster-whisper — all verified working
on this machine. Ollama and the custom-HTTP visual backend are *declared as capabilities* and
will honestly report `unavailable` when absent; they are never faked.

### Why this slice and not something else

- It reuses every working executor and rewrites none of them.
- It creates the missing spine (typed plan → bound execution → assertion → revision) which is
  the *only* thing separating this repo from "another AI video wrapper".
- The capability gap it demonstrates is **real on this machine and reproducible**: this
  ffmpeg 8.1 build has **no `subtitles` filter and no `drawtext` filter** (verified —
  `ffmpeg -filters | grep subtitles` returns nothing). Caption burn genuinely cannot run here.

---

## 8. THE WOW MOMENT

> **The same production plan executes through a different backend, and the substitution is a
> first-class persisted fact — not a warning string.**

Concretely, on stage: the brief asks for burned-in captions.

```
$ byom-video produce ./assets --brief "...20-second vertical teaser with burned-in captions"

  observe    4 assets, 47.3s total, 2 with audio
  capabilities
    ffmpeg.filter.scale        available
    ffmpeg.filter.amix         available
    ffmpeg.filter.subtitles    UNAVAILABLE  (libass not compiled in)
    ffmpeg.filter.drawtext     UNAVAILABLE  (freetype not compiled in)
    asr.faster_whisper         available    (tiny)
    text.ollama                UNAVAILABLE  (connection refused :11434)

  plan v1    6 stages   stage_0005 text_overlay -> ffmpeg.subtitles

  execute    stage_0005 BLOCKED: required capability ffmpeg.filter.subtitles unavailable

  revise     rev_0001: capability_unavailable
             stage_0005 text_overlay  ffmpeg.subtitles -> sidecar.srt
             plan v2 written; 5 of 6 stages reused from v1

  execute    stage_0005 completed via sidecar.srt

  validate   duration_within_tolerance   PASS  (19.8s, target 20s ±10%)
             aspect_ratio_is_9x16        PASS  (1080x1920)
             has_video_stream            PASS
             captions_delivered          PASS  (degraded: sidecar, not burned)

  complete   output/final.mp4 + output/final.srt
             replay: byom-video replay 20260921T233015Z-a4f2
```

Then the part that lands with an infrastructure engineer:

```sh
$ cat .byom-video/productions/<id>/stages/stage_0005/execution.json
```

…and it contains the **exact argv**, exit code, duration, and `ffmpeg -version` string.
And `diff plan/v1/production_plan.json plan/v2/production_plan.json` shows a one-node change
with `revisions/rev_0001.json` explaining why.

Why this is the right moment for this audience:
- It is the opposite of a hallucinated success. The system says "I cannot do this," proves it
  by probe, does something else, and keeps the receipt.
- Both plans survive. The causal chain is diffable.
- Pipeline TDs recognize this immediately — it is how farm/render provenance is supposed to work.
- It emerges from architecture (capability binding is late, not baked into the plan), not marketing.

Second beat, if time allows: force a validation failure (`--target-duration 30` against 24s of
source) and show a bounded corrective pass that extends via the existing loop logic in
`timeline_parser.go`, producing a *valid* artifact and a second revision record.

---

## 9. BUILD PLAN

New package `internal/production/`. New root `.byom-video/productions/`.
**Do not modify** `make.go`, `create.go`, `agent_plan_v1.go`, `job*.go`, `daemon.go`,
`mask*.go`, or `creative_assemble.go`'s existing entry points.

| # | Item | Files | Behavior | Acceptance | Effort | Parallel |
|---|---|---|---|---|---|---|
| 1 | **Capability probe** | `internal/production/capability.go` (+test) | Probe ffmpeg filter list, ffmpeg/ffprobe versions, python modules, configured `tools.backends` reachability. Emit `capabilities.json` with `{id, status: available\|unavailable, detail, probed_at, version}`. Reuses `doctor.go` logic. | On this machine: `ffmpeg.filter.subtitles` → `unavailable`, `ffmpeg.filter.scale` → `available`, `asr.faster_whisper` → `available`. Unit-tested with a fake prober. | S | ✅ independent |
| 2 | **Observation** | `internal/production/observe.go` (+test) | Walk an input dir, ffprobe each media file, emit `observation.json` (per-asset duration, dims, fps, codecs, streams, has_audio; plus totals). | 4 mixed assets → one JSON with correct per-asset ffprobe facts; non-media files skipped with a reason. | S | ✅ independent |
| 3 | **Plan schema + planner** | `internal/production/plan.go`, `planner.go` (+tests) | `ProductionPlan{schema_version, production_id, brief, stages[]}`. Each `Stage{id, type, description, inputs[], outputs[], requires: Capability, requirements: {cpu, memory_mb, gpu, runtime, container_image_hint}, binding: {backend, bound_at}, status}`. Deterministic planner maps brief + observation → 5–6 stages. | Plan round-trips through JSON; every stage declares a capability id and a resource block; planner is a pure function (no I/O) and fully unit-testable. | **M** | ⚠️ item 4 depends on the schema — land this first |
| 4 | **Executor** | `internal/production/execute.go`, `stages_ffmpeg.go`, `stages_asr.go` (+tests) | Walk the DAG. For each stage: resolve binding against `capabilities.json`; if unavailable → return `ErrCapabilityUnavailable` (do **not** fail the run). Otherwise dispatch to a real executor. Write `stages/<id>/execution.json` with **argv, exit code, duration_ms, tool version, output artifacts**. Append to a single `events.jsonl`. | A 4-stage plan produces 4 real media artifacts and 4 execution records containing real argv. Killing the process mid-run leaves completed stages recorded on disk. | **L** | ❌ depends on 1+3 |
| 5 | **Goal-aware validator** | `internal/production/validate.go` (+test) | Derive assertions from the brief (target duration ±tolerance, aspect ratio, has_video, has_audio, captions_delivered). Check each by ffprobe-ing the **real output**. Emit `validation.json` with per-assertion `{id, expected, actual, status, evidence}`. | A deliberately-wrong output (15s vs 20s target) yields `duration_within_tolerance: FAIL` with both numbers recorded. | M | ⚠️ depends on 3 for assertion types; logic parallelizable |
| 6 | **Reviser** | `internal/production/revise.go` (+test) | On `ErrCapabilityUnavailable` or a failed assertion: pick a fallback binding from a small ordered table (`ffmpeg.subtitles → ffmpeg.drawtext → sidecar.srt`), write `plan/v2/production_plan.json` + `revisions/rev_0001.json` `{trigger, stage_id, from, to, reason, at}`. **Bounded: max 2 revisions per production.** Re-execute only affected stages. | Caption stage revises exactly once on this machine, both plans persist, v2 differs from v1 in exactly one stage, and the reason is machine-readable. Revision cap is enforced by test. | **M** | ❌ depends on 4+5 |
| 7 | **`produce` command + CLI wiring** | `internal/commands/produce.go`, `internal/cli/root.go` | One command orchestrating 1→6, plus `--dry-run` (plan only), `--json`, `--target-duration`. Writes `run_record.json` + `handoff.md`. | `byom-video produce ./assets --brief "..."` runs to completion on this machine and prints the transcript in §8. | M | ❌ depends on all |
| 8 | **`replay` command** | `internal/commands/produce.go` | Read `run_record.json` + every `execution.json`; re-issue the recorded argv in order into a fresh output dir; diff resulting file sizes/durations. | `byom-video replay <production_id>` reproduces `final.mp4` from persisted argv alone, with no planner involvement. | S | ❌ depends on 4 |
| 9 | **Demo assets + smoke script** | `scripts/smoke-produce.sh`, `examples/fixtures/` | 3–4 short generated clips (ffmpeg `testsrc`/`sine` — committable, no binaries needed) + a scripted end-to-end assertion. | Script exits 0 on a clean checkout with ffmpeg present; asserts plan v2 and rev_0001 exist. | S | ✅ independent |
| 10 | **Docs + stale-config fix** | `docs/production.md`, `Makefile`, `README.md` | Document the schema and what is real vs unavailable. Fix `Makefile` MODULE (`byom-video` → `OpenVFX`) so version ldflags work. Optionally add `cmd/openvfx` alias. | `byom-video version` reports the real version; docs state plainly which capabilities are live. | S | ✅ independent |

**Suggested parallelization:** items 1, 2, 9, 10 can run immediately and concurrently.
Item 3 is the critical path — land it first, then 4 and 5 in parallel, then 6, then 7 and 8.

**Explicitly out of scope:** actual container execution, OmniCompute, a scheduler, a database,
a web UI, LLM planning, a plugin framework, refactoring the existing 10 artifact roots,
touching `make`/`create`/`agent-plan`/jobs/daemon.

---

## 10. RULES OF ENGAGEMENT for agents working here

1. **Verify before claiming.** This repo has a long history of files that look like features.
   Run the thing. Read the artifact it produced.
2. **Never present a stub as working.** If a capability is unavailable, the correct output is
   `status: unavailable` with a probed reason — never a fabricated success.
3. **Real execution in anything user-facing.** No mocked model responses in a demo path.
   A boring real ffmpeg operation beats a spectacular fake one.
4. **Do not run the full test suite repeatedly.** Test the package you changed.
   `go test ./...` at most once near the end, and only if justified.
5. **No new dependencies.** Go stdlib only. No Kubernetes, Kafka, microservices, database,
   web dashboard, or generic plugin framework.
6. **Do not refactor unrelated code** or rewrite working components for style.
7. **Preserve existing behavior.** `make`, `create`, `agent-run`, jobs, and the daemon must
   keep working exactly as they do today.
8. **Files on disk are the contract.** Every meaningful decision must survive process death
   as inspectable JSON/JSONL.
9. **Declare requirements explicitly** (cpu/memory/gpu/runtime/inputs/outputs) on every new
   stage type, so a future scheduler could place the work. Declaring is enough; do not build
   the scheduler.
10. **Append handoffs to `PROGRESS.md`** using the existing `<!-- PROMPT NNN START -->` /
    `<!-- HANDOFF NNN START -->` convention. Current head: **Prompt 071**.

## 11a. Next milestone — showcase credibility (post-slice)

**Now ahead of the list below:** run the Creative Director against a real local
Ollama and iterate on prompt quality. Everything else is scaffolding until that
is proven. Then add `langgraph` to `install.sh` (long-standing pending TODO).


The slice proved the architecture. The gap between this and an ASWF Open Source
Days / HPA-credible project is **professional interop**, not features. In
priority order:

1. **OpenTimelineIO in/out.** Highest leverage single change. `EditDecisionList`
   in `stages.go` maps onto OTIO almost directly. A custom timeline schema reads
   as a silo; OTIO reads as ecosystem membership. ~1 week.
2. **Frames and timecode as first-class types.** Retire `float64` seconds from
   the plan schema. Professional conform is frames + timecode, drop-frame
   included.
3. **Pick one name.** `go.mod` says OpenVFX, the binary says `byom-video`, the
   README says "BYOM Video", the repo dir says BYOMVIDEO. Four names.
4. **Cut the command surface.** 143 documented commands with a large stub
   fraction reads as unfocused to a reviewer counting them.
5. **Enforce and publish the no-egress guarantee.** `run_record.json` already
   carries `network_egress_calls`. Make it a hard mode (`--local-only` that
   refuses to bind any egress capability) and document it. This is the
   MPA/TPN content-security conversation that leads to paid pilots.

Do NOT chase: generative-model quality (that is a model-company fight OpenVFX
should not pick), a web UI, or a scheduler.

## 12. Environment facts (2026-09-21, this machine)

```
go build ./...            clean
go test ./...             28 packages, all pass
ffmpeg                    8.1  (/opt/homebrew/bin) — NO subtitles filter, NO drawtext
ffprobe                   8.1  available
.venv/bin/python          3.14; faster_whisper OK; langgraph MISSING
ollama                    NOT INSTALLED (director LLM route unverified against a real model)
langgraph                 1.2.12 in .venv and ~/.byom-venv
pytest                    installed in .venv
git branch                main  (PR base is Feature/openvfx)
uncommitted               PROGRESS.md, creative_timeline.go, make.go, run.go, transcribe.py
untracked                 timeline_parser*.go, timeline_source*.go, docs/timeline-source.md,
                          scripts/smoke-synthetic-timeline.sh   (Prompt 071 work)
```

Isolated smoke workspace used for the audit (safe to delete):
`/private/tmp/claude-501/.../scratchpad/audit/`
