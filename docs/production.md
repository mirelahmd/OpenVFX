# `produce` — the OpenVFX production control loop

`byom-video produce` runs one complete pass of the OpenVFX architecture:

```
OBSERVE → PLAN → EXECUTE → VALIDATE → REVISE → COMPLETE
```

It is the first command in which **the plan is the program**: every media
operation that runs is described by a stage in a persisted plan, and every stage
that runs leaves a record containing the exact `argv` that was invoked.

```sh
byom-video produce ./assets --brief "Cut these clips into a 12-second vertical teaser with burned-in captions"
byom-video productions
byom-video replay <production_id>
```

## What is real, and what is not

Everything in this command executes for real. Nothing is mocked or stubbed.

| Stage | Backend | Status |
|---|---|---|
| `probe_assets` | `ffprobe` | real subprocess |
| `transcribe` | `faster-whisper` via the Python worker | real, requires the worker installed |
| `select_clips` | native Go arithmetic over probed durations | real |
| `assemble_video` | `ffmpeg` cut + concat | real subprocess |
| `format_output` | `ffmpeg` scale + pad | real subprocess |
| `text_overlay` | `ffmpeg subtitles` → `ffmpeg drawtext` → sidecar SRT | real; which one runs depends on the probe |

Capabilities that are **not** wired into `produce` yet: generative video/image
backends, voice generation, and LLM planning. They exist elsewhere in the
codebase (`execute-visual-requests`, `generate-voiceover`, `agent-plan
--planner ollama`) but are not stages in a production plan. `produce` reports
them as unavailable rather than pretending otherwise.

## Creative direction

By default `produce` runs the **Creative Director** first — a LangGraph agent
that interprets the request into a structured
[`creative_treatment.json`](artifacts/creative-treatment.md) before any
deterministic planning happens.

```
OBSERVE assets -> interpret_brief -> assign_asset_roles -> design_story
      -> identify_gaps -> design_treatment -> critique_treatment -> finalize
      -> deterministic production plan -> execute/validate/revise
```

The director decides WHAT should be made. It never gains execution authority:
capability binding, policy, execution, failure handling, revision and validation
all stay in the Go runtime exactly as before.

```sh
byom-video produce ./assets --goal "<rich creative request>"          # deterministic reasoning
byom-video produce ./assets --goal "..." --director llm               # model reasoning, routed from config
byom-video produce ./assets --goal "..." --director llm --director-model llama3
byom-video produce ./assets --goal "..." --no-director                # skip it entirely
byom-video creative-treatment <production_id> [--json]
```

`--goal` and `--brief` are interchangeable.

### Reasoning modes are reported, never assumed

| Mode | What happened |
|---|---|
| `llm` | a model completed at least one call; `semantic_reasoning: true` |
| `deterministic` | the graph ran with no model; rules produced a real but limited treatment |
| unavailable | the sidecar could not run; no treatment, intent-only planning, reason printed |

The deterministic path is genuinely useful — it assigns roles from audio and
transcript presence, builds segments from real durations, and detects coverage
shortfalls — but it records its own limits in `uncertainties` and never sets
`semantic_reasoning`. A treatment that claimed otherwise is rejected by the Go
validator.

Model routing is config-driven and carries no vendor SDK. The Go runtime
resolves `models.routes.creative_director` and hands the Python side a resolved
JSON config, so the sidecar never parses YAML and never picks a provider itself.
Ollama is the first working route; an OpenAI-compatible route exists behind the
same interface.

### From creative decision to ffmpeg argv

The treatment's `segments` become the edit. A decision like *"start 1s into
clip_1 rather than at frame zero"* lands in `edit_decisions.json` and then in a
literal command, and the stage carries `treatment_decision_id` back to it:

```
stage_0003  select_clips   ↳ treatment: Start 1s into clip_1 rather than at frame zero. (dec_0001)
ffmpeg -hide_banner -y -ss 1.000 -i .../clip_1.mp4 -t 4.000 ...
```

### Bounded critique

Exactly one critique pass runs, and the bound is structural: the graph is linear
with a single critique node and no edge back into itself. There is no
configuration under which it can loop.

## Artifact layout

One production id owns everything. There is no second directory to correlate.

```
.byom-video/productions/<production_id>/
  observation.json          what the assets actually are (real ffprobe)
  asset_observations.json   the compact asset contract the director reasons over
  capabilities.json         what this machine can actually do (probed, not configured)
  creative_treatment.json   what should be made, and why
  director/
    director_config.json    the routing the Go runtime resolved
    brief.txt               the creator's request, verbatim
    graph_trace.json        per-node trace: which nodes reasoned, which fell back
  plan/
    v1/production_plan.json the original plan
    v2/production_plan.json the revised plan, if a revision happened
  stages/<stage_id>/
    execution.json          argv, exit code, duration, tool version
    <media artifacts>
  validation/validation.json per-assertion results with expected vs actual
  revisions/rev_0001.json   trigger, from/to plan, change, reason
  events.jsonl              one ordered trace for the whole production
  output/final.mp4          the deliverable
  output/final.srt          sidecar, when captions degraded to one
  run_record.json           the single replayable account
  handoff.md                human-readable provenance
```

## Capability probing

Capability status is **probed, never configured**. `capabilities.json` records
what was asked and what answered:

```json
{
  "id": "ffmpeg.filter.subtitles",
  "status": "unavailable",
  "detail": "libass not compiled in; caption burn-in unavailable",
  "probed_at": "2026-09-21T23:56:51Z"
}
```

A Homebrew ffmpeg on macOS commonly ships without `libass` and without
`freetype`, so neither `subtitles` nor `drawtext` is present. That is a real
capability gap, and `produce` treats it as one.

## Late binding and revision

A stage declares the **capability** it requires. It does not name a backend.
The backend is chosen at execution time, against the probe.

Critically, **the executor does not substitute on its own.** If the declared
capability is unavailable the stage is blocked, and the *reviser* decides what
to do — because choosing a different means to the same end is a planning
decision, and planning decisions must be recorded with a reason.

```
execute    stage_0006 BLOCKED: required capability ffmpeg.filter.subtitles unavailable
revise     rev_0001: capability_unavailable
           stage_0006: requires ffmpeg.filter.subtitles -> sidecar.srt
           plan v2 written; 5 of 6 stages reused from v1
```

Plan v1 survives untouched, so the substitution is diffable:

```sh
diff .byom-video/productions/<id>/plan/v1/production_plan.json \
     .byom-video/productions/<id>/plan/v2/production_plan.json
```

The declared compute footprint changes with the binding — a sidecar write is
not a 4-core ffmpeg job, and the revised plan says so:

```diff
-      "requires": "ffmpeg.filter.subtitles",
+      "requires": "sidecar.srt",
-        "cpu_cores": 4,
-        "memory_mb": 2048,
+        "cpu_cores": 0.1,
+        "memory_mb": 64,
```

## Goal-aware validation

Assertions are derived from the brief and checked by probing the **real output
file** — not by trusting that the commands exited zero.

| Assertion | What it checks |
|---|---|
| `output_exists` | the deliverable is on disk and readable |
| `output_is_probeable` | ffprobe can parse it |
| `has_video_stream` | a video stream is present |
| `duration_within_tolerance` | measured duration vs the brief's target (±10%) |
| `aspect_ratio_matches` | measured dimensions vs the requested format |
| `has_audio_stream` | audio survived the pipeline |
| `captions_delivered` | captions were delivered, and in what fidelity |

Degraded delivery is a first-class result. Captions shipped as a sidecar when
the brief asked for burn-in **pass** the assertion but are marked `degraded`,
and the production's status becomes `completed_degraded`. The system never
reports full success for partial delivery.

## Bounded revision

`MaxRevisions` is 2. Two triggers exist:

- `capability_unavailable` — substitute the binding for the blocked stage.
- `validation_failed` — currently corrects `duration_within_tolerance` only, by
  authorising the selector to repeat the tail of the edit.

The planner deliberately does **not** extend to target on the first pass.
Looping footage is a creative decision; doing it silently would hide the fact
that the source could not cover the brief. It happens only after the shortfall
has been recorded as a failed assertion:

```
validate   duration_within_tolerance  FAIL  18.040s (off by 11.960s)
revise     rev_0001: validation_failed
validate   duration_within_tolerance  PASS  30.040s (off by 0.040s)
complete   completed
```

If no corrective pass is defined for a failure, `produce` says so and stops
rather than looping uselessly.

## Resource declarations

Every stage declares its compute footprint explicitly:

```json
"requirements": {
  "cpu_cores": 2,
  "memory_mb": 2048,
  "gpu": false,
  "runtime": "python>=3.10+faster-whisper",
  "container_image_hint": "ghcr.io/openvfx/faster-whisper:tiny",
  "network_egress": false
}
```

OpenVFX does **not** schedule against these, and does not run stages in
containers. It declares them so that something else could. Hidden requirements
cannot be placed by any scheduler; declared ones can.

## Replay

`replay` re-issues the recorded `argv` in stage order, consulting no planner:

```sh
byom-video replay <production_id>
```

If replay works, the execution records are a complete account of what happened.
On a deterministic pipeline the re-rendered artifacts are byte-identical.

## Local-first and egress

`run_record.json` carries `network_egress_calls`. A production that used only
local capabilities reports `0`, and the capability set records which configured
backends would have sent bytes off the machine. This is a claim a security team
can check rather than a promise in a README.

## Limitations (honest list)

- Transcription runs on the **first** asset with audio, not all of them.
  Captions therefore cover only that asset's speech.
- `select_clips` takes assets in directory order; there is no content-aware
  selection in this path (the older `highlights`/`roughcut` path does that, and
  is not yet a production stage).
- `drawtext` fallback renders only the first cue as a static overlay.
- Without `--director llm` (or with no model configured), brief interpretation
  is keyword/regex matching. It is honest about that — `effective_mode` is
  `deterministic` and `semantic_reasoning` is false — but it is not understanding.
- The LLM route has been exercised against a stub endpoint and unit-tested with
  scripted responses; it has not yet been run against a real local model.
- Asset roles are inferred from duration, audio and transcript presence only.
  There is no content analysis, so roles are frequently `unknown`/`uncertain`.
- Generated-asset needs are detected and reported but not fulfilled: `produce`
  has no generation stage, so a gap stays a gap.
- Frames and timecode are not modelled; durations are float seconds.
- No OpenTimelineIO interchange yet.

## Relationship to the existing commands

`produce` is additive. It does not change `make`, `create`, `agent-plan`,
`job-*`, or `daemon`, which continue to work exactly as before. Those commands
use their own artifact roots and their own plan schemas.
