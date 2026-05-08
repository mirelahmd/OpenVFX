# Progress

## Prompt 001 - Initial Skeleton + Metadata Run

<!-- PROMPT 001 START -->
Build an open-source, local-first BYOM agentic media/video workflow platform.

Project identity:
- Local-first AI media/video workflow control plane, not a generic agent platform.
- V1 takes a video/audio file, creates a replayable run folder, extracts metadata, and produces structured artifacts.
- Long-term direction includes transcription, highlight detection, rough-cut planning, FFmpeg exports, and NLE integration.
- Current milestone stays intentionally small.

Stack and scope:
- Go CLI/control plane now.
- Python AI/media workers later.
- FFmpeg/FFprobe for media inspection.
- Filesystem artifacts as the core contract.
- No database, vector DB, web app, agent framework, transcription, model router, or Docker in this milestone.

Milestone 0:
- Create a clean Go project skeleton.
- CLI binary name: `byom-video`.
- Commands: `byom-video doctor` and `byom-video run <input-file>`.

Milestone 1:
- Validate the input file exists.
- Create `.byom-video/runs/<run_id>/`.
- Write `manifest.json`.
- Write `events.jsonl`.
- Run `ffprobe` on the input file.
- Save raw ffprobe JSON output to `metadata.json`.
- Print run id, input file, run directory, duration, stream counts, and artifacts written.

Doctor command:
- Check Go runtime/build assumptions.
- Check whether `ffprobe` and `ffmpeg` are available.
- Print clear OK/MISSING statuses and install hints.

Architecture rules:
- Every workflow step writes an artifact.
- Every run should be replayable later.
- Do not hide important state only in memory.
- Use simple JSON files and JSONL events.
- Keep code clean and boring.
- Prefer clear interfaces over clever abstractions.
- Do not overbuild plugin systems yet.

Required structure:
- `cmd/byom-video/main.go`
- `internal/cli/root.go`
- `internal/commands/doctor.go`
- `internal/commands/run.go`
- `internal/runctx/run_context.go`
- `internal/manifest/manifest.go`
- `internal/events/events.go`
- `internal/media/ffprobe.go`
- `internal/fsx/fs.go`
- `examples/README.md`
- `PROGRESS.md`
- `README.md`
- `.gitignore`
- `go.mod`
<!-- PROMPT 001 END -->

## Handoff 001

<!-- HANDOFF 001 START -->
What was built:
- A dependency-free Go CLI named `byom-video`.
- `byom-video doctor` for local runtime and FFmpeg tool checks.
- `byom-video run <input-file>` for creating replayable run folders and artifact files.
- Filesystem artifact contract under `.byom-video/runs/<run_id>/`.
- JSON manifest and JSONL event log.
- FFprobe JSON metadata extraction to `metadata.json` when `ffprobe` is available.
- Clean failure handling that writes `RUN_FAILED` when ffprobe is unavailable or probing fails after a run is created.

Exact folder tree:
```text
.
├── .gitignore
├── PROGRESS.md
├── README.md
├── cmd
│   └── byom-video
│       └── main.go
├── examples
│   └── README.md
├── go.mod
├── internal
│   ├── cli
│   │   └── root.go
│   ├── commands
│   │   ├── doctor.go
│   │   └── run.go
│   ├── events
│   │   └── events.go
│   ├── fsx
│   │   └── fs.go
│   ├── manifest
│   │   └── manifest.go
│   ├── media
│   │   └── ffprobe.go
│   └── runctx
│       └── run_context.go
└── intro.md
```

Important files created:
- `cmd/byom-video/main.go`: binary entry point.
- `internal/cli/root.go`: minimal command dispatch.
- `internal/commands/doctor.go`: environment checks.
- `internal/commands/run.go`: run workflow orchestration and terminal summary.
- `internal/runctx/run_context.go`: run ID and run directory creation context.
- `internal/manifest/manifest.go`: manifest schema and writer.
- `internal/events/events.go`: JSONL event writer.
- `internal/media/ffprobe.go`: ffprobe execution and tool version helpers.
- `internal/fsx/fs.go`: filesystem validation helpers.
- `README.md`: project overview and quickstart.
- `examples/README.md`: placeholder examples guidance.
- `.gitignore`: ignores generated run data and local binary.

How to run it:
```sh
go build ./cmd/byom-video
./byom-video doctor
./byom-video run /path/to/input.mp4
```

How to test it:
```sh
go test ./...
go build ./cmd/byom-video
./byom-video doctor
```

Local verification performed:
- `go build ./cmd/byom-video` passed.
- `go test ./...` passed.
- `./byom-video doctor` ran and reported missing FFmpeg tools on this machine.
- `./byom-video run intro.md` failed cleanly because `ffprobe` is missing and wrote failure artifacts under `.byom-video/runs/<run_id>/`.

Known limitations:
- FFmpeg/FFprobe are required for successful metadata extraction and were not installed in this local environment.
- There is no transcription, model routing, worker system, database, vector DB, web app, plugin system, Docker setup, or NLE integration.
- No automated unit tests have been added yet; current verification is build plus command-level smoke checks.
- `metadata.json` is raw ffprobe output only.
- `manifest.json` is rewritten as artifacts are added; future milestones may want stricter artifact checksums.

Next recommended milestone:
- Add focused tests for run ID generation, manifest writing, event JSONL writing, and metadata summary parsing.
- Add a small media fixture generation path for local development when FFmpeg is installed.
- Start Milestone 2 with transcription planning only after the artifact contract is stable.

Errors or assumptions:
- Assumed dependency-free command dispatch is acceptable instead of Cobra for this small milestone.
- Assumed run directories should be created relative to the current working directory.
- Local machine reported `ffprobe` and `ffmpeg` as missing, so successful media probing could not be verified here.
<!-- HANDOFF 001 END -->

## Prompt 002 - Milestone 1 Hardening + Tests

<!-- PROMPT 002 START -->
Continue from the existing repo and `PROGRESS.md`.

Scope constraints:
- Do not add transcription.
- Do not add Python workers.
- Do not add model routing.
- Do not add Docker.
- Do not add a web app.
- Do not add a vector DB.
- Keep this as Milestone 1 hardening only.

Goal:
- Stabilize the Go CLI skeleton before adding AI/media workers.

Requested tasks:
- Add focused unit tests for run ID generation and run context creation, manifest creation and rewrite behavior, event JSONL writing, ffprobe metadata summary parsing, and input file validation.
- Add a simple developer fixture workflow that generates `examples/fixtures/tiny.mp4` using FFmpeg if installed, preferably via `scripts/make-fixture.sh`, without committing binary media.
- Improve `byom-video doctor` to print detected paths and versions for `ffmpeg` and `ffprobe` when found, keep missing-tool output clean, and avoid panics.
- Improve run summary to print duration, video stream count, audio stream count, and total streams, using `unknown` for missing fields.
- Improve manifest behavior so final status is `completed` on success and `failed` on failure after run folder creation, artifacts are listed, tool versions are included when available, and error message is included on failure.
- Keep architecture boring and artifact-first with no plugin system, unnecessary abstractions, or external dependencies.
<!-- PROMPT 002 END -->

## Handoff 002

<!-- HANDOFF 002 START -->
What changed:
- Added focused Go unit tests across run context, manifest, events, filesystem validation, and metadata summary parsing.
- Added `scripts/make-fixture.sh` to generate `examples/fixtures/tiny.mp4` when FFmpeg is installed.
- Updated `.gitignore` so generated fixture MP4 files are not committed.
- Improved `doctor` to print tool versions when tools are found.
- Updated run summary to always print duration and stream counts, using `unknown` when metadata fields are absent.
- Updated manifest schema with optional `error_message`.
- Updated failure handling after run folder creation so failed manifests include `status: failed` and an error message.
- Kept implementation dependency-free and within Milestone 1 scope.

Tests added:
- `internal/runctx/run_context_test.go`: run ID format and run context creation.
- `internal/manifest/manifest_test.go`: manifest defaults, rewrite behavior, unique artifact paths, and tool version persistence.
- `internal/events/events_test.go`: JSONL event writer creates decodable event lines.
- `internal/fsx/fs_test.go`: input file validation accepts files and rejects missing paths/directories.
- `internal/commands/run_test.go`: metadata summary duration and stream count parsing, including unknown vs explicit zero handling.

Exact commands run:
```sh
gofmt -w internal/commands/doctor.go internal/commands/run.go internal/manifest/manifest.go internal/runctx/run_context_test.go internal/manifest/manifest_test.go internal/events/events_test.go internal/fsx/fs_test.go internal/commands/run_test.go
chmod +x scripts/make-fixture.sh
go test ./...
go build ./cmd/byom-video
./byom-video doctor
scripts/make-fixture.sh
./byom-video run intro.md
```

Current folder tree:
```text
.
├── .gitignore
├── PROGRESS.md
├── README.md
├── cmd
│   └── byom-video
│       └── main.go
├── examples
│   └── README.md
├── go.mod
├── internal
│   ├── cli
│   │   └── root.go
│   ├── commands
│   │   ├── doctor.go
│   │   ├── run.go
│   │   └── run_test.go
│   ├── events
│   │   ├── events.go
│   │   └── events_test.go
│   ├── fsx
│   │   ├── fs.go
│   │   └── fs_test.go
│   ├── manifest
│   │   ├── manifest.go
│   │   └── manifest_test.go
│   ├── media
│   │   └── ffprobe.go
│   └── runctx
│       ├── run_context.go
│       └── run_context_test.go
├── intro.md
└── scripts
    └── make-fixture.sh
```

How to generate fixture:
```sh
scripts/make-fixture.sh
```

How to run against fixture:
```sh
go build ./cmd/byom-video
./byom-video run examples/fixtures/tiny.mp4
```

Known limitations:
- FFmpeg/FFprobe are still required for successful metadata extraction.
- FFmpeg/FFprobe were not installed in this local environment, so fixture generation and successful `metadata.json` extraction could not be completed here.
- No committed binary fixture is included.
- No transcription, Python workers, model router, Docker, web app, vector DB, or plugin system has been added.
- Tests are focused unit tests; there is no end-to-end successful media probe test because the local environment lacks FFmpeg tools.

Next recommended milestone:
- Add a small integration test path that runs only when FFmpeg/FFprobe are installed.
- Add checksum or size fields to artifacts if artifact integrity becomes important.
- Start the next media-worker planning milestone only after the artifact contract is considered stable.

Errors or assumptions:
- `scripts/make-fixture.sh` was run and failed cleanly because `ffmpeg` is missing.
- `./byom-video doctor` reported both `ffprobe` and `ffmpeg` as missing on PATH.
- `./byom-video run intro.md` failed cleanly because `ffprobe` is missing and wrote a failed manifest with `error_message`.
<!-- HANDOFF 002 END -->

## Prompt 003 - Inference Mask Architecture Doc

<!-- PROMPT 003 START -->
Continue from the existing repo and `PROGRESS.md`.

Scope constraints:
- Do not implement transcription.
- Do not add Python workers.
- Do not add model routing code.
- Do not add NVIDIA API code.
- Do not add OpenAI, Claude, Groq, or Ollama clients.
- Do not add Docker.
- Do not add vector DB.
- Do not change CLI behavior except docs if needed.

Goal:
- Add architecture documentation for a future optional Inference Mask layer.

Context:
- BYOM Video is a local-first BYOM AI media/video workflow control plane.
- The future architecture may use premium models as compact reasoners and cheap/free/local models as constrained expanders.
- The premium reasoner produces compact structured intent and constraints as an `inference_mask.json` artifact.
- Expanders create captions, labels, descriptions, timeline notes, rough-cut explanations, and similar artifacts without inventing facts or changing decisions.
- A verifier checks cheap expansion against the mask.
- BYOM providers may eventually include OpenAI, Claude, Groq, NVIDIA-hosted free/cheap LLMs, Ollama/local, and others.
- NVIDIA should be treated as one optional future provider, not a required dependency.

Required doc:
- Create `docs/architecture/inference-mask.md`.
- Cover what the Inference Mask is, why it exists, cost control, quality risk, and the invariant: "Cheap models can expand style. Cheap models cannot expand truth."
- Include the future pipeline from transcript chunks to final artifacts.
- Include artifact examples for `inference_mask.json`, `expansion_tasks.json`, `expansions/captions.json`, and `verification.json`.
- Include an `inference_mask.json` schema sketch.
- Include a future config sketch with premium reasoner, NVIDIA/free expander, Ollama/local expander, and routing examples.
- Include non-goals for the current MVP and how this fits into the media workflow.
<!-- PROMPT 003 END -->

## Handoff 003

<!-- HANDOFF 003 START -->
What changed:
- Added architecture documentation for a future Inference Mask layer.
- Kept the change documentation-only.
- Did not alter CLI behavior, runtime code, tests, provider logic, or generated artifacts.

Files added:
- `docs/architecture/inference-mask.md`

Architecture captured:
- Premium model acts as the reasoner.
- The reasoner writes compact structured decisions, evidence, and constraints into `inference_mask.json`.
- Cheap/free/local models act as expanders.
- Expanders may create captions, labels, descriptions, timeline notes, and rough-cut explanations.
- Expanders must not introduce facts, timestamps, speakers, claims, or editorial decisions outside the mask.
- A verifier checks expansions against the mask before final artifacts are accepted.
- NVIDIA-hosted free/cheap models are described as one optional future provider, not a required dependency.
- The invariant is documented: "Cheap models can expand style. Cheap models cannot expand truth."
- The future pipeline and artifact-first contract are documented.

What was intentionally not implemented:
- No transcription.
- No Python workers.
- No model routing code.
- No NVIDIA API code.
- No OpenAI, Claude, Groq, or Ollama clients.
- No provider SDKs.
- No verifier.
- No Docker.
- No vector DB.
- No web app.
- No CLI behavior changes.

Next recommended milestone:
- Add a short architecture index under `docs/architecture/README.md` if more architecture docs are added.
- Add a future artifact schema planning document only when the project is ready to define transcription/chunking artifacts.
- Keep implementation focused on the current media artifact pipeline until the worker boundary is ready.

Errors or assumptions:
- Assumed this should be a planning document only.
- Assumed YAML is acceptable for the future config sketch even though no config parser exists.
- Assumed provider names in examples are illustrative placeholders, not committed integration targets.
<!-- HANDOFF 003 END -->

## Prompt 004 - Python Worker Bridge + Transcript Stub

<!-- PROMPT 004 START -->
Continue from the existing repo and `PROGRESS.md`.

Goal:
- Add the first Python worker bridge while keeping it stubbed and artifact-first.

Scope:
- Add a Python worker project skeleton.
- Add a stub transcription worker.
- Make Go able to call the Python worker optionally.
- The worker should write a fake `transcript.json` artifact.
- Do not add real Whisper.
- Do not add OpenAI, Claude, Groq, Ollama, or NVIDIA clients.
- Do not add model routing.
- Do not add Docker.
- Do not add vector DB.
- Do not add web app.
- Do not implement Inference Mask.

Architecture requirement:
- Go remains the control plane.
- Python workers are execution units.
- Filesystem artifacts are the contract between Go and Python.

Requested files:
- `workers/pyproject.toml`
- `workers/byom_video_workers/__init__.py`
- `workers/byom_video_workers/cli.py`
- `workers/byom_video_workers/transcribe_stub.py`

Python worker behavior:
- Runnable as `python -m byom_video_workers.cli transcribe-stub --input <file> --run-dir <run_dir>`.
- Validate input file exists.
- Validate run directory exists.
- Write `transcript.json` into the run directory.
- Use a simple `transcript.v1` stub schema.
- Print a concise success line.
- Exit non-zero with clean error messages on failure.

Go CLI change:
- Add optional flag: `byom-video run <input-file> --with-transcript-stub`.
- Default `byom-video run <input-file>` remains metadata-only.
- When enabled, run metadata extraction first, invoke the Python worker after metadata succeeds, record `transcript.json` in manifest, write transcript stub events, and fail cleanly if the worker fails.

Doctor:
- Add Python check for `python3`, path, and version.

Docs:
- Update README with metadata-only run, transcript stub run, Python worker setup, and artifact outputs.

Tests:
- Add Go unit tests for flag parsing / command behavior if supported.
- Ensure `go test ./...` passes.
<!-- PROMPT 004 END -->

## Handoff 004

<!-- HANDOFF 004 START -->
What changed:
- Added the first stubbed Python worker project under `workers/`.
- Added a `transcribe-stub` Python worker that writes `transcript.json`.
- Added Go worker invocation through `internal/workers`.
- Added optional CLI flag `--with-transcript-stub`.
- Kept default `byom-video run <input-file>` metadata-only behavior unchanged.
- Added Python detection to `byom-video doctor`.
- Updated README with metadata-only usage, transcript-stub usage, Python worker setup, and artifact outputs.
- Added Go unit tests for run flag parsing.

Files added/modified:
- Added `workers/pyproject.toml`.
- Added `workers/byom_video_workers/__init__.py`.
- Added `workers/byom_video_workers/cli.py`.
- Added `workers/byom_video_workers/transcribe_stub.py`.
- Added `internal/workers/python.go`.
- Added `internal/cli/root_test.go`.
- Modified `internal/cli/root.go`.
- Modified `internal/commands/run.go`.
- Modified `internal/commands/doctor.go`.
- Modified `README.md`.
- Modified `.gitignore`.
- Modified `PROGRESS.md`.

How the Python worker is invoked directly:
```sh
PYTHONPATH=workers python3 -m byom_video_workers.cli transcribe-stub \
  --input examples/fixtures/tiny.mp4 \
  --run-dir .byom-video/runs/<run_id>
```

Alternatively, after editable install:
```sh
python3 -m pip install -e workers
python3 -m byom_video_workers.cli transcribe-stub \
  --input examples/fixtures/tiny.mp4 \
  --run-dir .byom-video/runs/<run_id>
```

How Go invokes it:
- `byom-video run <input-file> --with-transcript-stub` runs normal ffprobe metadata extraction first.
- After metadata succeeds, Go invokes:
```sh
python3 -m byom_video_workers.cli transcribe-stub --input <input-file> --run-dir <run_dir>
```
- Go sets `PYTHONPATH=workers` for that subprocess.
- `BYOM_VIDEO_PYTHON` can override the Python interpreter.

Commands run:
```sh
gofmt -w internal/cli/root.go internal/cli/root_test.go internal/commands/doctor.go internal/commands/run.go internal/workers/python.go
go test ./...
go build ./cmd/byom-video
./byom-video doctor
scripts/make-fixture.sh
PYTHONPATH=workers python3 -m byom_video_workers.cli transcribe-stub --input examples/fixtures/tiny.mp4 --run-dir .byom-video/runs/worker-direct-test-004
./byom-video run examples/fixtures/tiny.mp4
./byom-video run examples/fixtures/tiny.mp4 --with-transcript-stub
python3 -m compileall -q workers/byom_video_workers
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- Direct Python worker invocation wrote `transcript.json`.
- Metadata-only Go run wrote `manifest.json`, `events.jsonl`, and `metadata.json`.
- Transcript-stub Go run wrote `manifest.json`, `events.jsonl`, `metadata.json`, and `transcript.json`.
- The transcript-stub run manifest ended with `status: completed`.

Known limitations:
- The transcript is fake stub content only.
- No Whisper or real transcription is implemented.
- No Python dependency installation is required for the stub, but direct module execution from the repo uses `PYTHONPATH=workers` unless the worker package is installed.
- No OpenAI, Claude, Groq, Ollama, or NVIDIA clients were added.
- No model routing, Docker, vector DB, web app, verifier, or Inference Mask implementation was added.
- The Go worker bridge is intentionally simple and only supports the transcript stub path.

Next recommended milestone:
- Add a lightweight integration test that exercises `--with-transcript-stub` when `ffmpeg`, `ffprobe`, and `python3` are available.
- Add artifact schema docs for `transcript.json` before implementing real transcription.
- Keep real transcription behind the same artifact contract when introduced.

Errors or assumptions:
- Assumed Python worker direct invocation can use `PYTHONPATH=workers` for local development without requiring package installation.
- Assumed `python3` is the default interpreter and `BYOM_VIDEO_PYTHON` is sufficient configurability for now.
- Assumed the transcript stub should run only after successful ffprobe metadata extraction.
<!-- HANDOFF 004 END -->

## Prompt 005 - Artifact Schema Documentation

<!-- PROMPT 005 START -->
Continue from the existing repo and `PROGRESS.md`.

Goal:
- Document artifact schemas before adding real transcription.

Scope:
- Documentation only, except README links if needed.
- Do not add real Whisper.
- Do not change Python worker behavior.
- Do not change Go run behavior.
- Do not add model routing.
- Do not add provider clients.
- Do not add Docker.
- Do not add vector DB.
- Do not implement Inference Mask.

Requested docs:
- `docs/artifacts/README.md`
- `docs/artifacts/manifest.md`
- `docs/artifacts/events.md`
- `docs/artifacts/metadata.md`
- `docs/artifacts/transcript.md`

Document the current artifact contract:
- `manifest.json`: purpose, lifecycle status values, `run_id`, `input_path`, `created_at`, artifacts list, `tool_versions`, `error_message`, and allowance for future optional fields.
- `events.jsonl`: purpose, append-only run timeline, one JSON object per line, event naming style, and current event examples.
- `metadata.json`: raw ffprobe JSON output, not normalized yet, preserved for replay/debugging, and usable for downstream summaries.
- `transcript.json`: current `transcript.v1` schema, source object, language, duration, segments, segment fields, stub status, and requirement that future real transcribers write the same schema when compatible.
- Add example JSON snippets for each artifact.
- Add architecture rule: artifacts are contracts; workers may change internally, but artifact schemas should remain stable or versioned.
- Add README links to artifact docs.
<!-- PROMPT 005 END -->

## Handoff 005

<!-- HANDOFF 005 START -->
What changed:
- Added artifact schema documentation under `docs/artifacts/`.
- Added README links to the artifact docs.
- Kept the change documentation-only.
- Did not change Go run behavior or Python worker behavior.

Files added/modified:
- Added `docs/artifacts/README.md`.
- Added `docs/artifacts/manifest.md`.
- Added `docs/artifacts/events.md`.
- Added `docs/artifacts/metadata.md`.
- Added `docs/artifacts/transcript.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

Artifact schemas documented:
- `manifest.json`: run index, lifecycle status, run identity, input path, creation time, artifact list, tool versions, error message, and forward-compatible optional fields.
- `events.jsonl`: append-only run timeline, JSON Lines format, event object shape, event naming convention, and current event names.
- `metadata.json`: raw ffprobe JSON artifact, intentionally not normalized, preserved for replay/debugging and downstream summaries.
- `transcript.json`: `transcript.v1` schema, source object, language, duration, ordered segments, segment fields, stub behavior, and future real transcriber compatibility rule.

What was intentionally not implemented:
- No real Whisper transcription.
- No Go run behavior changes.
- No Python worker behavior changes.
- No model routing.
- No provider clients.
- No Docker.
- No vector DB.
- No Inference Mask implementation.
- No schema validation code.

Next recommended milestone:
- Add lightweight schema validation tests for generated artifacts once schemas are stable enough to enforce.
- Add artifact docs for future chunking or transcription-derived artifacts before implementing real workers.
- Keep future worker changes constrained by the documented artifact contracts.

Errors or assumptions:
- Assumed Markdown schema docs are sufficient for this stage.
- Assumed future fields should be allowed when backward compatible and ignored by existing readers.
- Assumed schema validation code should wait until the artifact contract has one more implementation pass.
<!-- HANDOFF 005 END -->

## Prompt 006 - Real Local Transcription Worker

<!-- PROMPT 006 START -->
Continue from the existing repo and `PROGRESS.md`.

Goal:
- Add a real local transcription worker while preserving the existing artifact contract.

Scope:
- Add an optional real transcription worker using Python.
- Prefer `faster-whisper` if practical.
- Keep the existing `transcript.v1` schema.
- Keep `transcribe-stub` working.
- Add a new Go flag: `--with-transcript`.
- Keep default `byom-video run <input-file>` metadata-only.
- Keep `--with-transcript-stub` unchanged.
- Do not add highlight detection.
- Do not add rough-cut planning.
- Do not add model routing.
- Do not add OpenAI, Claude, Groq, NVIDIA, or Ollama clients.
- Do not add Inference Mask implementation.
- Do not add Docker.
- Do not add vector DB.
- Do not add web app.

Architecture:
- Go remains the control plane.
- Python worker performs transcription.
- Filesystem artifacts remain the contract.
- Real transcription must write the same `transcript.json` schema as the stub when compatible.

Python changes:
- Add `workers/byom_video_workers/transcribe.py`.
- Add worker CLI command: `python -m byom_video_workers.cli transcribe --input <file> --run-dir <run_dir> --model-size tiny`.
- Validate input file and run directory.
- Import `faster_whisper` only inside the transcribe command.
- If `faster_whisper` is missing, print a clean install error.
- Write `transcript.json` with `schema_version: transcript.v1`, `source.mode: real`, `source.engine: faster-whisper`, `source.model_size`, language, duration when available, and segments.
- Update `workers/pyproject.toml` with optional transcription dependencies.

Go changes:
- Add `byom-video run <input-file> --with-transcript`.
- Run metadata extraction first, then invoke the Python real transcription worker.
- Record `transcript.json` as an artifact.
- Write `TRANSCRIBE_STARTED`, `TRANSCRIBE_COMPLETED`, and `TRANSCRIBE_FAILED` events.
- On failure after run folder creation, mark manifest failed, include `error_message`, and write `RUN_FAILED`.
- Reject `--with-transcript` and `--with-transcript-stub` together.

Doctor and docs:
- Keep Python check.
- Note that real transcription requires optional worker dependencies.
- Do not make `faster-whisper` mandatory for metadata-only usage.
- Update README and transcript artifact docs.

Tests:
- Ensure `go test ./...` passes.
- Add tests for transcript flag mutual exclusion if feasible.
- Add Python compile check.
- Do not require `faster-whisper` in tests unless installed.
<!-- PROMPT 006 END -->

## Handoff 006

<!-- HANDOFF 006 START -->
What changed:
- Added optional real local transcription worker using `faster-whisper`.
- Added Python CLI command `transcribe`.
- Added Go run flag `--with-transcript`.
- Kept metadata-only default behavior unchanged.
- Kept `--with-transcript-stub` working.
- Added mutual exclusion between `--with-transcript` and `--with-transcript-stub`.
- Added real transcription events: `TRANSCRIBE_STARTED`, `TRANSCRIBE_COMPLETED`, and `TRANSCRIBE_FAILED`.
- Added optional dependency metadata for transcription.
- Updated README with install and run instructions.
- Updated transcript artifact docs for `source.mode: stub|real`, `source.engine`, and `source.model_size`.
- Updated event artifact docs with real transcription events.

Files added/modified:
- Added `workers/byom_video_workers/transcribe.py`.
- Modified `workers/byom_video_workers/cli.py`.
- Modified `workers/pyproject.toml`.
- Modified `internal/cli/root.go`.
- Modified `internal/cli/root_test.go`.
- Modified `internal/commands/run.go`.
- Modified `internal/commands/doctor.go`.
- Modified `internal/workers/python.go`.
- Modified `README.md`.
- Modified `docs/artifacts/transcript.md`.
- Modified `docs/artifacts/events.md`.
- Modified `PROGRESS.md`.

How to install transcription dependencies:
```sh
python3 -m pip install -e "workers[transcribe]"
```

How to run metadata-only:
```sh
go build ./cmd/byom-video
./byom-video run examples/fixtures/tiny.mp4
```

How to run transcript stub:
```sh
./byom-video run examples/fixtures/tiny.mp4 --with-transcript-stub
```

How to run real transcription:
```sh
./byom-video run examples/fixtures/tiny.mp4 --with-transcript
```

Direct Python worker command:
```sh
PYTHONPATH=workers python3 -m byom_video_workers.cli transcribe \
  --input examples/fixtures/tiny.mp4 \
  --run-dir .byom-video/runs/<run_id> \
  --model-size tiny
```

Commands run:
```sh
gofmt -w internal/cli/root.go internal/cli/root_test.go internal/commands/doctor.go internal/commands/run.go internal/workers/python.go
python3 -m compileall -q workers/byom_video_workers
go test ./...
go build ./cmd/byom-video
./byom-video doctor
./byom-video run examples/fixtures/tiny.mp4
./byom-video run examples/fixtures/tiny.mp4 --with-transcript-stub
./byom-video run examples/fixtures/tiny.mp4 --with-transcript --with-transcript-stub
PYTHONPATH=workers python3 -m byom_video_workers.cli transcribe --input examples/fixtures/tiny.mp4 --run-dir .byom-video/runs/worker-direct-test-004 --model-size tiny
./byom-video run examples/fixtures/tiny.mp4 --with-transcript
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- Metadata-only run succeeded.
- Transcript stub run succeeded.
- Mutual exclusion returned a clean CLI error.
- Direct real transcription worker returned a clean missing dependency error because `faster-whisper` is not installed.
- Go real transcription mode wrote metadata, then failed cleanly with `TRANSCRIBE_FAILED`, `RUN_FAILED`, and a failed manifest containing `error_message`.

Known limitations:
- Real transcription could not be completed locally because `faster-whisper` is not installed.
- First successful `faster-whisper` run may download model weights.
- No Whisper model management, cache controls, device controls, compute type flags, or progress reporting were added.
- No highlight detection, rough-cut planning, model routing, provider clients, Inference Mask implementation, Docker, vector DB, or web app was added.
- Real transcription currently uses model size `tiny` from Go with no CLI flag for model selection at the Go layer.

Next recommended milestone:
- Install optional transcription dependencies and run one successful end-to-end real transcription smoke test.
- Add a lightweight integration test gated on `faster-whisper` availability.
- Add optional Go flag for transcription model size only after the basic real worker path is verified.
- Add schema validation tests for `transcript.json`.

Errors or assumptions:
- Assumed `faster-whisper` is the preferred local transcription backend for this milestone.
- Assumed optional dependency installation should remain user-controlled and not be performed automatically.
- Assumed `tiny` is the correct default model size for first local testing.
- Assumed real transcription should run only after successful ffprobe metadata extraction.
<!-- HANDOFF 006 END -->

## Prompt 007 - Transcription Hardening + Validation

<!-- PROMPT 007 START -->
Continue from the existing repo and `PROGRESS.md`.

Goal:
- Harden the real transcription path now that `faster-whisper` has been installed and a real local speech test succeeded.

Context:
- A real transcription run was verified locally with `BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video run media/Untitled.mov --with-transcript`.
- It produced `metadata.json`, `transcript.json`, `manifest.json`, and `events.jsonl`.
- The transcript captured: "Okay, okay, okay."

Scope:
- Improve transcription usability and validation.
- Keep artifact-first architecture.
- Do not add highlight detection.
- Do not add rough-cut planning.
- Do not add model routing.
- Do not add OpenAI, Claude, Groq, NVIDIA, or Ollama clients.
- Do not add Inference Mask implementation.
- Do not add Docker.
- Do not add vector DB.
- Do not add web app.

Required changes:
- Add Go CLI flag: `--transcript-model-size`.
- Default model size remains `tiny`.
- Supported values: `tiny`, `base`, `small`, `medium`, `large-v3`.
- Reject `--transcript-model-size` without `--with-transcript`.
- Reject unsupported model sizes with a clean CLI error.
- Keep rejecting `--with-transcript` and `--with-transcript-stub` together.
- Pass model size from Go to the Python worker.
- Improve successful real transcription terminal summary with transcript artifact path, language, segment count, transcript duration, and model size.
- Add transcript schema validation after the Python real transcription worker completes and before marking the run completed.
- Write `TRANSCRIPT_VALIDATION_STARTED`, `TRANSCRIPT_VALIDATION_COMPLETED`, and `TRANSCRIPT_VALIDATION_FAILED` events.
- Add tests for model size validation, transcript flag behavior, and transcript schema validation.
- Add `scripts/smoke-transcribe.sh`.
- Update README and artifact docs.
<!-- PROMPT 007 END -->

## Handoff 007

<!-- HANDOFF 007 START -->
What changed:
- Added `--transcript-model-size` to `byom-video run`.
- Added supported transcript model size validation.
- Passed selected model size from Go to the Python transcribe worker.
- Added transcript schema validation before successful run completion.
- Added transcript validation events.
- Added transcript details to terminal summary for successful real transcription.
- Added `scripts/smoke-transcribe.sh`.
- Updated README and artifact docs.
- Added unit tests for CLI model-size behavior and transcript schema validation.

Files added/modified:
- Added `internal/transcript/validate.go`.
- Added `internal/transcript/validate_test.go`.
- Added `scripts/smoke-transcribe.sh`.
- Modified `internal/cli/root.go`.
- Modified `internal/cli/root_test.go`.
- Modified `internal/commands/run.go`.
- Modified `README.md`.
- Modified `docs/artifacts/transcript.md`.
- Modified `docs/artifacts/events.md`.
- Modified `PROGRESS.md`.

New flags:
```sh
--with-transcript
--transcript-model-size <tiny|base|small|medium|large-v3>
```

Default model size:
```text
tiny
```

Validation behavior:
- Real transcription must write `transcript.json`.
- Go validates `transcript.json` before adding it to the manifest and before marking the run completed.
- Validation requires `schema_version: transcript.v1`.
- `source.mode` must exist.
- `segments` must exist and be an array.
- Each segment must include `id`, numeric `start`, numeric `end`, and string `text`.
- Each segment must satisfy `end >= start`.
- On validation failure, Go writes `TRANSCRIPT_VALIDATION_FAILED`, marks the manifest `failed`, writes `error_message`, and writes `RUN_FAILED`.
- On validation success, Go writes `TRANSCRIPT_VALIDATION_COMPLETED`, records `transcript.json`, and continues to `RUN_COMPLETED`.

Commands run:
```sh
gofmt -w internal/cli/root.go internal/cli/root_test.go internal/commands/run.go internal/transcript/validate.go internal/transcript/validate_test.go
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
./byom-video run media/Untitled.mov --transcript-model-size tiny
./byom-video run media/Untitled.mov --with-transcript --transcript-model-size huge
./byom-video run media/Untitled.mov --with-transcript --with-transcript-stub
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-transcribe.sh media/Untitled.mov
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `--transcript-model-size` without `--with-transcript` returned a clean CLI error.
- Unsupported model size returned a clean CLI error.
- Transcript mode mutual exclusion returned a clean CLI error.
- Smoke transcription test succeeded with `tiny`.
- Successful smoke test produced `metadata.json`, `transcript.json`, `manifest.json`, and `events.jsonl`.
- Events included `TRANSCRIPT_VALIDATION_STARTED` and `TRANSCRIPT_VALIDATION_COMPLETED`.

How to run smoke transcription test:
```sh
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-transcribe.sh media/Untitled.mov
```

Known limitations:
- The Go CLI model-size flag is limited to fixed supported values.
- The smoke script assumes `./byom-video` has already been built.
- The smoke script uses `tiny`.
- Larger models may download additional weights and take longer.
- No device, compute type, cache path, or progress options were added.
- No highlight detection, rough-cut planning, model routing, provider clients, Inference Mask implementation, Docker, vector DB, or web app was added.

Next recommended milestone:
- Add schema validation for `manifest.json` and `events.jsonl`.
- Add a gated integration test for real transcription that skips cleanly when `.venv` or `faster-whisper` is unavailable.
- Consider adding `--transcript-model-size` to direct docs examples for larger local accuracy tests.

Errors or assumptions:
- Assumed the existing `.venv` with `faster-whisper` should be used for smoke testing.
- Assumed `media/Untitled.mov` is acceptable as the local smoke input.
- Assumed model size should remain a small allowlist rather than arbitrary backend model names.
<!-- HANDOFF 007 END -->

## Prompt 008 - Deterministic Transcript Chunking

<!-- PROMPT 008 START -->
Continue from the existing repo and `PROGRESS.md`.

Goal:
- Add deterministic transcript chunking as the next artifact stage.

Context:
- The project supports metadata-only runs, transcript stub runs, real local `faster-whisper` transcription, transcript model-size selection, and transcript schema validation.
- A stable `chunks.json` artifact is needed before highlight detection.

Scope:
- Add transcript chunking.
- Keep it deterministic and local.
- Do not add LLM calls.
- Do not add highlight detection.
- Do not add rough-cut planning.
- Do not add model routing.
- Do not add OpenAI, Claude, Groq, NVIDIA, Ollama, Gemma, Kimi, or Qwen clients.
- Do not implement Inference Mask.
- Do not add Docker.
- Do not add vector DB.
- Do not add web app.

Required behavior:
- Add `byom-video run <input-file> --with-transcript --with-chunks`.
- `--with-chunks` requires `--with-transcript` or `--with-transcript-stub`.
- Chunking runs after transcript generation and transcript validation.
- Default metadata-only behavior remains unchanged.
- Default transcript behavior remains unchanged unless `--with-chunks` is passed.
- Add deterministic Go chunking from `transcript.json` to `chunks.json`.
- Add optional chunk flags: `--chunk-target-seconds` and `--chunk-max-gap-seconds`.
- Add chunking and chunks validation events.
- Record `chunks.json` in manifest on success.
- Mark run failed with `error_message` on chunking or validation failure.
- Print chunk artifact path, chunk count, target seconds, and max gap seconds in the terminal summary.
- Add docs, tests, and `scripts/smoke-chunks.sh`.
<!-- PROMPT 008 END -->

## Handoff 008

<!-- HANDOFF 008 START -->
What changed:
- Added deterministic transcript chunking in Go.
- Added `chunks.json` artifact generation.
- Added `chunks.v1` validation.
- Added CLI support for `--with-chunks`.
- Added chunk tuning flags.
- Added chunking and chunks validation events.
- Added chunk summary output to successful runs.
- Added `scripts/smoke-chunks.sh`.
- Added artifact docs for `chunks.json`.
- Updated README and event docs.
- Added Go tests for chunking, validation, and CLI flag behavior.

Files added/modified:
- Added `internal/chunks/chunks.go`.
- Added `internal/chunks/validate.go`.
- Added `internal/chunks/chunks_test.go`.
- Added `docs/artifacts/chunks.md`.
- Added `scripts/smoke-chunks.sh`.
- Modified `internal/cli/root.go`.
- Modified `internal/cli/root_test.go`.
- Modified `internal/commands/run.go`.
- Modified `README.md`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/artifacts/events.md`.
- Modified `docs/artifacts/manifest.md`.
- Modified `PROGRESS.md`.

New flags:
```sh
--with-chunks
--chunk-target-seconds <number>
--chunk-max-gap-seconds <number>
```

Chunking behavior:
- Reads `transcript.json`.
- Groups transcript segments in order.
- Starts a new chunk when adding a segment would exceed `target_seconds`.
- Starts a new chunk when the gap between adjacent segments exceeds `max_gap_seconds`.
- Preserves segment IDs.
- Joins segment text with spaces.
- Computes chunk duration and word count.
- Defaults are `target_seconds: 30` and `max_gap_seconds: 2.0`.
- A transcript with one short segment produces one chunk.

Validation behavior:
- Validates `chunks.json` after generation and before run completion.
- Requires `schema_version: chunks.v1`.
- Requires `chunks` array.
- Each chunk must have `id`, `start`, `end`, `duration_seconds`, `text`, `segment_ids`, and `word_count`.
- Requires `end >= start`, `duration_seconds >= 0`, and `word_count >= 0`.
- On failure, writes `CHUNKS_VALIDATION_FAILED`, marks manifest failed, records `error_message`, and writes `RUN_FAILED`.
- On success, writes `CHUNKS_VALIDATION_COMPLETED`, `CHUNKING_COMPLETED`, and records `chunks.json`.

Commands run:
```sh
gofmt -w internal/cli/root.go internal/cli/root_test.go internal/commands/run.go internal/chunks/chunks.go internal/chunks/validate.go internal/chunks/chunks_test.go
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
./byom-video run media/Untitled.mov --with-chunks
./byom-video run media/Untitled.mov --with-transcript --chunk-target-seconds 30
./byom-video run media/Untitled.mov --with-transcript --with-chunks --chunk-target-seconds 0
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-chunks.sh media/Untitled.mov
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- CLI rejected `--with-chunks` without transcript mode.
- CLI rejected chunk flags without `--with-chunks`.
- CLI rejected invalid chunk numeric values.
- Smoke chunking test succeeded with `media/Untitled.mov`.
- Smoke run produced `manifest.json`, `events.jsonl`, `metadata.json`, `transcript.json`, and `chunks.json`.
- Events included `CHUNKING_STARTED`, `CHUNKS_VALIDATION_STARTED`, `CHUNKS_VALIDATION_COMPLETED`, and `CHUNKING_COMPLETED`.

How to run smoke chunking test:
```sh
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-chunks.sh media/Untitled.mov
```

Known limitations:
- Chunking is deterministic and simple; it does not use semantic boundaries.
- No sentence merging, speaker labels, paragraphing, or token budgeting were added.
- The smoke script uses real transcription and requires `faster-whisper`.
- No highlight detection, rough-cut planning, LLM calls, model routing, provider clients, Inference Mask implementation, Docker, vector DB, or web app was added.

Next recommended milestone:
- Add schema validation for `manifest.json` and `events.jsonl`.
- Add a gated integration test for `--with-transcript --with-chunks`.
- Add future highlight planning docs that consume `chunks.json`, without implementing highlight detection yet.

Errors or assumptions:
- Assumed Go is the right place for deterministic chunking because it is schema-based and local.
- Assumed chunk text should be joined with single spaces.
- Assumed chunking can operate on both stub and real transcript artifacts.
- Assumed `media/Untitled.mov` remains the local smoke input.
<!-- HANDOFF 008 END -->

## Prompt 009 - Highlights + Roughcut Planning

<!-- PROMPT 009 START -->
Continue from the existing repo and `PROGRESS.md`.

Goal:
- Add deterministic highlight candidate detection and rough-cut planning in one milestone.

Context:
- The project supports metadata extraction, real local `faster-whisper` transcription, transcript validation, deterministic transcript chunking, chunks validation, and artifact-first run folders.
- The first editor-intelligence layer should produce `highlights.json` and `roughcut.json`.

Scope:
- Keep this deterministic and local.
- Do not add LLM calls.
- Do not add model routing.
- Do not add provider clients.
- Do not add OpenAI, Claude, Groq, NVIDIA, Ollama, Gemma, Kimi, or Qwen clients.
- Do not implement Inference Mask.
- Do not add Docker.
- Do not add vector DB.
- Do not add web app.

Required behavior:
- Add `--with-highlights` and `--with-roughcut`.
- `--with-highlights` requires `--with-chunks`.
- `--with-roughcut` requires `--with-highlights`; if roughcut is passed with chunks, highlights are automatically enabled.
- Generate deterministic `highlights.json` from `chunks.json`.
- Score highlights with duration, word count, hook phrase, question, and emphasis heuristics.
- Add highlight tuning flags.
- Validate `highlights.json`.
- Generate deterministic `roughcut.json` from `highlights.json`.
- Select top highlights, limit clips, order by timeline, and compute roughcut duration.
- Validate `roughcut.json`.
- Add events, manifest entries, terminal summaries, docs, tests, and `scripts/smoke-roughcut.sh`.
<!-- PROMPT 009 END -->

## Handoff 009

<!-- HANDOFF 009 START -->
What changed:
- Added deterministic highlight candidate generation.
- Added deterministic rough-cut planning.
- Added `highlights.json` and `roughcut.json` artifacts.
- Added schema validation for both artifacts.
- Added highlight and roughcut CLI flags.
- Added highlight and roughcut event streams.
- Added highlight and roughcut terminal summary sections.
- Added `scripts/smoke-roughcut.sh`.
- Added artifact docs for `highlights.json` and `roughcut.json`.
- Updated README and artifact event docs.
- Added Go unit tests for highlight scoring/sorting/validation, roughcut selection/order/validation, and CLI flag rules.

Files added/modified:
- Added `internal/highlights/highlights.go`.
- Added `internal/highlights/validate.go`.
- Added `internal/highlights/highlights_test.go`.
- Added `internal/roughcut/roughcut.go`.
- Added `internal/roughcut/validate.go`.
- Added `internal/roughcut/roughcut_test.go`.
- Added `docs/artifacts/highlights.md`.
- Added `docs/artifacts/roughcut.md`.
- Added `scripts/smoke-roughcut.sh`.
- Modified `internal/cli/root.go`.
- Modified `internal/cli/root_test.go`.
- Modified `internal/commands/run.go`.
- Modified `README.md`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/artifacts/events.md`.
- Modified `PROGRESS.md`.

New flags:
```sh
--with-highlights
--with-roughcut
--highlight-top-k <int>
--highlight-min-duration-seconds <number>
--highlight-max-duration-seconds <number>
--roughcut-max-clips <int>
```

Highlight behavior:
- Reads `chunks.json`.
- Scores chunks with deterministic heuristics.
- Prefers useful duration and enough words.
- Boosts hook phrases, questions, and emphasis markers.
- Penalizes empty or near-empty text.
- Normalizes score to `0.0` through `1.0`.
- Sorts by descending score.
- Keeps default top `10`.

Roughcut behavior:
- Reads `highlights.json`.
- Selects top highlights by score.
- Keeps default max `5` clips.
- Orders selected clips by timeline start time.
- Computes total rough-cut duration.
- Preserves source highlight text.
- Produces a planning artifact only; no media is cut.

Validation behavior:
- `highlights.json` must use `schema_version: highlights.v1`.
- Each highlight must include required timing, score, label, reason, text, and signals fields.
- Highlight scores must be between `0` and `1`.
- `roughcut.json` must use `schema_version: roughcut.v1`.
- Each clip must include required highlight, timing, order, score, intent, and text fields.
- Roughcut scores must be between `0` and `1`; clip order must be `>= 1`.
- On failure, the run manifest is marked `failed`, `error_message` is written, and `RUN_FAILED` is emitted.

Commands run:
```sh
gofmt -w internal/cli/root.go internal/cli/root_test.go internal/commands/run.go internal/highlights/highlights.go internal/highlights/validate.go internal/highlights/highlights_test.go internal/roughcut/roughcut.go internal/roughcut/validate.go internal/roughcut/roughcut_test.go
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
./byom-video run media/Untitled.mov --with-highlights
./byom-video run media/Untitled.mov --with-transcript --with-chunks --with-highlights --highlight-top-k 0
./byom-video run media/Untitled.mov --with-transcript --with-chunks --with-highlights --roughcut-max-clips 2
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-roughcut.sh media/Untitled.mov
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- CLI rejected `--with-highlights` without chunks.
- CLI rejected invalid highlight flags.
- CLI rejected roughcut flags without roughcut.
- Smoke roughcut test succeeded with `media/Untitled.mov`.
- Smoke run produced `manifest.json`, `events.jsonl`, `metadata.json`, `transcript.json`, `chunks.json`, `highlights.json`, and `roughcut.json`.
- Events included highlight and roughcut generation and validation events.

How to run smoke roughcut test:
```sh
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-roughcut.sh media/Untitled.mov
```

Known limitations:
- Highlight detection is deterministic heuristic scoring, not semantic understanding.
- Roughcut is a planning artifact only; it does not cut or export media.
- No NLE timeline export, FFmpeg cutting, LLM calls, model routing, provider clients, Inference Mask implementation, Docker, vector DB, or web app was added.
- The local smoke input is very short, so it produces one chunk, one highlight, and one roughcut clip.

Next recommended milestone:
- Add schema validation for `manifest.json` and `events.jsonl`.
- Add deterministic FFmpeg export planning docs before implementing real media cuts.
- Add optional timeline/export planning artifact that consumes `roughcut.json`.

Errors or assumptions:
- Assumed highlight and roughcut should be deterministic planning artifacts before any LLM or export work.
- Assumed roughcut should auto-enable highlights only when chunks are present.
- Assumed the first roughcut strategy should prioritize top scores, then restore timeline order.
<!-- HANDOFF 009 END -->

## Prompt 010 - Captions + FFmpeg Export Script

<!-- PROMPT 010 START -->
Continue from the existing repo and `PROGRESS.md`.

Goal:
- Add deterministic captions and FFmpeg export script generation.

Context:
- The project supports metadata extraction, real local `faster-whisper` transcription, transcript validation, deterministic transcript chunking, chunks validation, deterministic highlight detection, highlights validation, deterministic roughcut planning, and roughcut validation.
- The first tangible editor outputs should be captions and an inspectable FFmpeg export script.

Scope:
- Keep this deterministic and local.
- Generate files only; do not automatically execute FFmpeg cuts.
- Do not add LLM calls, model routing, provider clients, OpenAI, Claude, Groq, NVIDIA, Ollama, Gemma, Kimi, Qwen, Inference Mask implementation, Docker, vector DB, web app, DaVinci integration, or Premiere integration.

Required behavior:
- Add `--with-captions`, requiring `--with-transcript` or `--with-transcript-stub`.
- Generate `captions.srt` from `transcript.json`.
- Add `CAPTIONS_STARTED`, `CAPTIONS_COMPLETED`, and `CAPTIONS_FAILED`.
- Add `--with-ffmpeg-script`, requiring `--with-roughcut`.
- Generate `ffmpeg_commands.sh` from `roughcut.json`.
- Add `FFMPEG_SCRIPT_STARTED`, `FFMPEG_SCRIPT_COMPLETED`, and `FFMPEG_SCRIPT_FAILED`.
- Add `--ffmpeg-output-format mp4`, supporting only `mp4` for now.
- Update docs, tests, smoke script, and `PROGRESS.md`.
<!-- PROMPT 010 END -->

## Handoff 010

<!-- HANDOFF 010 START -->
What changed:
- Added deterministic SRT caption generation.
- Added inspectable FFmpeg export script generation.
- Added `captions.srt` artifact.
- Added `ffmpeg_commands.sh` artifact.
- Added captions and FFmpeg script events.
- Added terminal summary sections for captions and FFmpeg script generation.
- Added CLI flags for captions and FFmpeg script generation.
- Added artifact docs for captions and FFmpeg script artifacts.
- Added `scripts/smoke-export-plan.sh`.
- Added Go tests for SRT formatting, captions generation, FFmpeg script generation, quoting, and CLI validation.

Files added/modified:
- Added `internal/captions/captions.go`.
- Added `internal/captions/captions_test.go`.
- Added `internal/exportscript/ffmpeg.go`.
- Added `internal/exportscript/ffmpeg_test.go`.
- Added `docs/artifacts/captions.md`.
- Added `docs/artifacts/ffmpeg-script.md`.
- Added `scripts/smoke-export-plan.sh`.
- Modified `internal/cli/root.go`.
- Modified `internal/cli/root_test.go`.
- Modified `internal/commands/run.go`.
- Modified `README.md`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/artifacts/events.md`.
- Modified `PROGRESS.md`.

New flags:
```sh
--with-captions
--with-ffmpeg-script
--ffmpeg-output-format mp4
```

Caption behavior:
- Reads `transcript.json`.
- Writes `captions.srt`.
- Uses transcript segments as SRT cues.
- Preserves segment start/end timing.
- Formats timestamps as `HH:MM:SS,mmm`.
- Numbers cues from `1`.
- Uses segment text as caption text.
- Empty transcripts produce an empty SRT file rather than invented captions.

FFmpeg script behavior:
- Reads `roughcut.json`.
- Writes executable `ffmpeg_commands.sh`.
- Does not execute FFmpeg.
- Includes `#!/usr/bin/env bash` and `set -euo pipefail`.
- Creates `exports/` when executed from the run directory.
- Emits one stream-copy FFmpeg command per roughcut clip.
- Uses output names like `exports/clip_0001.mp4`.
- Includes comments warning that stream-copy cuts may not be frame-perfect.
- Includes commented re-encode command templates.
- Supports only `mp4` output format for now.

Commands run:
```sh
gofmt -w internal/cli/root.go internal/cli/root_test.go internal/commands/run.go internal/captions/captions.go internal/captions/captions_test.go internal/exportscript/ffmpeg.go internal/exportscript/ffmpeg_test.go
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
./byom-video run media/Untitled.mov --with-captions
./byom-video run media/Untitled.mov --with-transcript --with-chunks --with-highlights --with-ffmpeg-script
./byom-video run media/Untitled.mov --with-transcript --with-chunks --with-highlights --with-roughcut --with-ffmpeg-script --ffmpeg-output-format mov
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-export-plan.sh media/Untitled.mov
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- CLI rejected captions without transcript mode.
- CLI rejected FFmpeg script generation without roughcut.
- CLI rejected unsupported FFmpeg output format.
- Smoke export-plan test succeeded with `media/Untitled.mov`.
- Smoke run produced `manifest.json`, `events.jsonl`, `metadata.json`, `transcript.json`, `captions.srt`, `chunks.json`, `highlights.json`, `roughcut.json`, and `ffmpeg_commands.sh`.

How to run smoke export-plan test:
```sh
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-export-plan.sh media/Untitled.mov
```

Known limitations:
- FFmpeg script is generated but not executed.
- Stream-copy cuts may not be frame-perfect.
- Only `mp4` output format is supported.
- No concat/export execution, frame-accurate re-encode mode, NLE export, DaVinci/Premiere integration, LLM calls, model routing, provider clients, Inference Mask implementation, Docker, vector DB, or web app was added.

Next recommended milestone:
- Add optional execution of generated FFmpeg scripts behind an explicit command or flag.
- Add frame-accurate re-encode script mode.
- Add schema validation for `manifest.json` and `events.jsonl`.
- Add export artifact docs for actual rendered clips once execution exists.

Errors or assumptions:
- Assumed captions should be SRT only for this milestone.
- Assumed empty transcripts should produce an empty SRT file rather than fail.
- Assumed generated FFmpeg scripts should be executable but never run automatically.
- Assumed stream-copy is the safest first export plan because it is fast and inspectable.
<!-- HANDOFF 010 END -->

## Prompt 011 - Export Execution + Local HTML Report

<!-- PROMPT 011 START -->
Continue from the existing repo and `PROGRESS.md`.

Goal:
- Add explicit FFmpeg export execution and a local HTML run report.

Context:
- The project supports metadata extraction, real local `faster-whisper` transcription, transcript validation, `captions.srt` generation, deterministic chunking, highlights, rough-cut planning, and `ffmpeg_commands.sh` generation.

Required behavior:
- Add `byom-video export <run_id>` to execute the generated `ffmpeg_commands.sh` from inside `.byom-video/runs/<run_id>`.
- Keep export execution explicit; do not execute FFmpeg during `byom-video run`.
- Record `EXPORT_STARTED`, `EXPORT_COMPLETED`, and `EXPORT_FAILED` events.
- Update manifest export status fields, exported time, exports directory, and exported files when possible.
- Add `--with-report` to generate a local deterministic `report.html` artifact at the end of successful runs.
- Add report events: `REPORT_STARTED`, `REPORT_COMPLETED`, and `REPORT_FAILED`.
- Add optional `byom-video open-report <run_id>` behavior that prints the report path.
- Add docs, tests, and `scripts/smoke-report-export.sh`.
- Keep this local-first and artifact-first. Do not add LLM calls, model routing, provider clients, Inference Mask implementation, Docker, vector DB, web app server, or NLE integrations.
<!-- PROMPT 011 END -->

## Handoff 011

<!-- HANDOFF 011 START -->
What changed:
- Added explicit export execution through `byom-video export <run_id>`.
- Added local HTML report generation through `--with-report`.
- Added path-safe run directory resolution under `.byom-video/runs`.
- Added manifest export status fields.
- Added export and report events.
- Added `open-report` command that prints the local report path.
- Added report refresh after successful export when `report.html` already exists.
- Added docs and smoke script for report/export workflows.
- Added tests for run path safety, export file discovery, export command failures, report generation, HTML escaping, and `--with-report` parsing.

Files added/modified:
- Added `internal/runstore/runstore.go`.
- Added `internal/runstore/runstore_test.go`.
- Added `internal/exporter/exporter.go`.
- Added `internal/exporter/exporter_test.go`.
- Added `internal/report/report.go`.
- Added `internal/report/report_test.go`.
- Added `internal/commands/export.go`.
- Added `internal/commands/export_test.go`.
- Added `internal/commands/open_report.go`.
- Added `docs/artifacts/report.md`.
- Added `docs/artifacts/exports.md`.
- Added `scripts/smoke-report-export.sh`.
- Modified `internal/manifest/manifest.go`.
- Modified `internal/commands/run.go`.
- Modified `internal/cli/root.go`.
- Modified `internal/cli/root_test.go`.
- Modified `README.md`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/artifacts/events.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video export <run_id>
./byom-video open-report <run_id>
```

New flags:
```sh
--with-report
```

Export behavior:
- `byom-video export <run_id>` locates `.byom-video/runs/<run_id>`.
- It rejects missing runs and unsafe run ids.
- It requires `ffmpeg_commands.sh` inside the run directory.
- It executes `bash ffmpeg_commands.sh` from inside the run directory.
- It records export events in `events.jsonl`.
- It updates `manifest.json` with `export_status`, `exported_at`, `exports_dir`, and `exported_files`.
- It does not accept arbitrary script paths or shell commands.

Report behavior:
- `--with-report` writes `report.html` at the end of a successful run.
- The report is local plain HTML/CSS with no external assets or web server.
- It includes available metadata, transcript, captions, chunks, highlights, roughcut, FFmpeg script, exports, and manifest artifact information.
- If earlier pipeline steps fail, the report is not generated.
- If report generation fails when explicitly requested, the run is marked failed.
- `open-report` prints the report path; it does not launch a browser.

Commands run:
```sh
gofmt -w internal/manifest/manifest.go internal/runstore/runstore.go internal/runstore/runstore_test.go internal/exporter/exporter.go internal/exporter/exporter_test.go internal/commands/export.go internal/commands/export_test.go internal/commands/open_report.go internal/commands/run.go internal/report/report.go internal/report/report_test.go internal/cli/root.go internal/cli/root_test.go
chmod +x scripts/smoke-report-export.sh
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-report-export.sh media/Untitled.mov
./byom-video export 20260428T232952Z-e00caff8
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- Report/export smoke run succeeded with `media/Untitled.mov`.
- Explicit export succeeded and wrote `exports/clip_0001.mp4`.

How to run smoke report/export test:
```sh
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-report-export.sh media/Untitled.mov
```

Run smoke test and execute export:
```sh
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-report-export.sh --execute-export media/Untitled.mov
```

Known limitations:
- Export execution uses the generated stream-copy commands; cuts may not be frame-perfect.
- Only `mp4` export script output is supported.
- The report is static local HTML.
- `open-report` prints the path only and does not open a browser.
- No concat export, frame-accurate re-encode mode, DaVinci/Premiere integration, LLM calls, model routing, provider clients, Inference Mask implementation, Docker, vector DB, or web app server was added.

Next recommended milestone:
- Add frame-accurate re-encode export script mode.
- Add optional concat/export assembly planning.
- Add schema validation for `manifest.json` and `events.jsonl`.
- Add a small export integrity check that probes rendered clips after export.

Errors or assumptions:
- Assumed explicit export should run only the generated script inside the known run directory.
- Assumed report generation should be success-only during `run`.
- Assumed `open-report` should avoid OS browser launching for now and only print the local path.
<!-- HANDOFF 011 END -->

## Prompt 012 - Init + Config + Pipeline Presets

<!-- PROMPT 012 START -->
Continue from the existing repo and `PROGRESS.md`.

Goal:
- Add project initialization, config file support, and named pipeline presets so users do not need huge CLI flag chains.

Context:
- The project supports metadata extraction, local `faster-whisper` transcription, captions, chunks, highlights, roughcut, FFmpeg script generation, local HTML reports, and explicit export execution.

Required behavior:
- Add `byom-video init`.
- Create `byom-video.yaml` if missing.
- Create `media/`, `exports/`, and `.byom-video/` if missing.
- Support `byom-video init --force` to overwrite config without deleting run history.
- Load `byom-video.yaml` for run defaults when present.
- Let CLI flags override config values.
- Add `byom-video pipeline <input-file> --preset shorts`.
- Add optional `metadata` preset.
- Update doctor to show config detection and configured Python interpreter.
- Add docs, tests, `scripts/smoke-pipeline.sh`, and update `PROGRESS.md`.
- Keep this local-first and artifact-first. Do not add LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, web app server, or NLE integrations.
<!-- PROMPT 012 END -->

## Handoff 012

<!-- HANDOFF 012 START -->
What changed:
- Added `byom-video init`.
- Added default `byom-video.yaml` generation.
- Added project folder initialization for `media/`, `exports/`, and `.byom-video/`.
- Added simple config loading for documented `byom-video.yaml` fields.
- Added `byom-video pipeline <input-file> --preset shorts`.
- Added `metadata` preset for metadata-only sanity checks.
- Added config-aware run defaults.
- Added configured Python interpreter support while preserving `BYOM_VIDEO_PYTHON` override.
- Updated doctor to detect config and configured Python.
- Added config and preset docs.
- Added `scripts/smoke-pipeline.sh`.
- Added tests for init, config loading, CLI override precedence, preset mapping, and unknown preset rejection.

Files added/modified:
- Added `internal/config/config.go`.
- Added `internal/config/config_test.go`.
- Added `internal/commands/init.go`.
- Added `internal/commands/init_test.go`.
- Added `docs/config.md`.
- Added `docs/presets.md`.
- Added `scripts/smoke-pipeline.sh`.
- Generated `byom-video.yaml` through `byom-video init`.
- Modified `internal/cli/root.go`.
- Modified `internal/cli/root_test.go`.
- Modified `internal/commands/doctor.go`.
- Modified `internal/commands/run.go`.
- Modified `internal/workers/python.go`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video init
./byom-video init --force
./byom-video pipeline <input-file> --preset shorts
./byom-video pipeline <input-file> --preset metadata
```

Config behavior:
- `byom-video.yaml` is loaded from the current directory when present.
- Config values become defaults for `run`.
- CLI flags override config values where flags exist.
- Unknown config fields are ignored.
- The config parser intentionally supports only the documented simple YAML shape.
- `python.interpreter` sets the worker Python interpreter.
- `BYOM_VIDEO_PYTHON` still overrides the configured interpreter when set.

Preset behavior:
- `shorts` enables real transcription, captions, chunks, highlights, roughcut, FFmpeg script generation, and report generation.
- `metadata` runs metadata extraction only.
- Presets reuse the existing `commands.Run` implementation.
- Presets do not execute export automatically.

Commands run:
```sh
gofmt -w internal/config/config.go internal/config/config_test.go internal/commands/init.go internal/commands/init_test.go internal/commands/doctor.go internal/cli/root.go internal/cli/root_test.go internal/workers/python.go internal/commands/run.go
chmod +x scripts/smoke-pipeline.sh
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-pipeline.sh media/Untitled.mov
./byom-video doctor
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- Smoke pipeline test succeeded with `media/Untitled.mov`.
- Doctor detected `byom-video.yaml` and `.venv/bin/python`.

How to run smoke pipeline test:
```sh
BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-pipeline.sh media/Untitled.mov
```

Known limitations:
- Config parsing is intentionally minimal and supports only the documented shape.
- Unknown config fields are ignored rather than warned.
- There are no `--no-*` flags yet, so config-enabled stages are disabled by editing config or using `pipeline --preset metadata`.
- Only `shorts` and `metadata` presets exist.
- No LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, web app server, DaVinci/Premiere integration, or automatic export execution was added.

Next recommended milestone:
- Add explicit disable flags if config-enabled defaults need command-line suppression.
- Add schema validation for `manifest.json` and `events.jsonl`.
- Add export integrity checks that probe rendered clips after explicit export.
- Consider a `byom-video config show` command once config grows.

Errors or assumptions:
- Assumed a dependency-free known-key YAML parser is preferable for the current simple config shape.
- Assumed unknown config fields should be ignored and documented rather than treated as fatal.
- Assumed presets should map to existing run options and not duplicate pipeline logic.
<!-- HANDOFF 012 END -->

## Prompt 013 - Run Discovery + Inspection

<!-- PROMPT 013 START -->
Continue from the existing repo and `PROGRESS.md`.

Goal:
- Add run discovery and inspection commands so users can navigate generated runs without manually digging through `.byom-video/runs`.

Context:
- The project supports init/config, pipeline presets, metadata extraction, transcription, captions, chunks, highlights, roughcut, FFmpeg script generation, local reports, and explicit export execution.

Required behavior:
- Add `byom-video runs` to list runs under `.byom-video/runs`.
- Sort newest first by `created_at` from manifest when available.
- Show run id, status, created time, input file basename, artifact count, and export status.
- Support `runs --limit <n>` and `runs --all`.
- Add `byom-video inspect <run_id>` with readable run details, artifact paths, export details, report path, and artifact summary counts.
- Support `inspect --json`.
- Add `byom-video artifacts <run_id>` with optional `--type <name>` filtering.
- Improve `open-report <run_id>` with optional `--open`.
- Reuse runstore path safety for all run-id based commands.
- Add docs, tests, smoke script, and update `PROGRESS.md`.
- Keep local-first and artifact-first. Do not add a database, web server, LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, or NLE integrations.
<!-- PROMPT 013 END -->

## Handoff 013

<!-- HANDOFF 013 START -->
What changed:
- Added run listing with `byom-video runs`.
- Added run inspection with `byom-video inspect <run_id>`.
- Added machine-readable inspect output with `--json`.
- Added artifact path listing with `byom-video artifacts <run_id>`.
- Added artifact type filtering with `--type`.
- Added optional OS report opening with `open-report <run_id> --open`.
- Added reusable run inspection logic in `internal/runinfo`.
- Added docs and smoke script for run discovery.
- Added tests for listing, missing manifests, inspection, artifact filtering, unsafe run ids, open-report path printing, and OS opener selection.

Files added/modified:
- Added `internal/runinfo/runinfo.go`.
- Added `internal/runinfo/runinfo_test.go`.
- Added `internal/commands/runs.go`.
- Added `internal/commands/runs_test.go`.
- Added `docs/runs.md`.
- Added `scripts/smoke-runs.sh`.
- Modified `internal/cli/root.go`.
- Modified `internal/cli/root_test.go`.
- Modified `internal/commands/open_report.go`.
- Modified `README.md`.
- Modified `docs/artifacts/report.md`.
- Modified `docs/presets.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video runs
./byom-video inspect <run_id>
./byom-video artifacts <run_id>
```

New flags:
```sh
./byom-video runs --limit <n>
./byom-video runs --all
./byom-video inspect <run_id> --json
./byom-video artifacts <run_id> --type <name>
./byom-video open-report <run_id> --open
```

Run listing behavior:
- Reads `.byom-video/runs`.
- Sorts newest first by manifest `created_at` when available.
- Shows unreadable or missing-manifest runs with status `unknown`.
- Defaults to 20 runs.
- `--all` shows every run.

Inspect behavior:
- Prints run id, status, input path, created time, error message, artifacts with full paths, export status, exported files, and report path when available.
- Prints transcript segment count, chunk count, highlight count, roughcut clip count, and exported file count when artifacts are available.
- Warns about missing artifact files instead of failing the whole inspection.
- `--json` emits a machine-readable summary.

Artifacts behavior:
- Prints artifact paths one per line.
- Supports filtering by `manifest`, `events`, `metadata`, `transcript`, `captions`, `chunks`, `highlights`, `roughcut`, `ffmpeg-script`, `report`, and `exports`.
- Unknown artifact types return a clean error.

Commands run:
```sh
gofmt -w internal/runinfo/runinfo.go internal/runinfo/runinfo_test.go internal/commands/runs.go internal/commands/runs_test.go internal/commands/open_report.go internal/cli/root.go internal/cli/root_test.go
chmod +x scripts/smoke-runs.sh
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-runs.sh
./byom-video inspect 20260428T234027Z-d162b0c9 --json
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-runs.sh` passed against the latest local run.
- `inspect --json` returned valid JSON summary for the latest local run.

How to run smoke runs test:
```sh
scripts/smoke-runs.sh
```

Known limitations:
- Run discovery is filesystem-based; there is still no database.
- `open-report --open` depends on OS tools such as `open`, `xdg-open`, or `cmd`.
- Artifact counts are derived from current JSON artifact shapes and skip unreadable files.
- No deletion, archiving, tagging, search, database, web server, LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, or NLE integrations were added.

Next recommended milestone:
- Add manifest and events schema validation commands.
- Add export integrity checks that probe rendered clips after explicit export.
- Add optional run cleanup/archive commands once artifact retention rules are defined.

Errors or assumptions:
- Assumed run management should remain read-only in this milestone.
- Assumed missing artifacts should be warnings for inspection, not hard failures.
- Assumed `open-report --open` should only attempt to open the known local `report.html` inside the resolved run directory.
<!-- HANDOFF 013 END -->

## Prompt 014 - Run Validation + Export Integrity

<!-- PROMPT 014 START -->
Goal:
- Add trust checks for existing filesystem-first runs.
- Add `byom-video validate <run_id>` and `--json`.
- Validate `manifest.json`, `events.jsonl`, listed artifact existence, and known artifact schemas for transcript, chunks, highlights, and roughcut.
- Validate manifest fields, lifecycle statuses, export status values, and safe relative artifact paths.
- Validate event log JSONL, event names, and timestamps using current `type`/`time` naming while allowing compatible event/timestamp naming.
- After explicit export succeeds, probe rendered clips with `ffprobe` when available.
- Write `export_validation.json` with checked files, existence, duration, video stream count, audio stream count, status, and error.
- Add export validation events and manifest export validation status fields.
- Refresh `report.html` after export validation so exports include validation status and durations.
- Update docs, add tests, add `scripts/smoke-validate-export.sh`, and keep the scope local/filesystem-first.

Out of scope:
- No database, LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 014 END -->

## Handoff 014

<!-- HANDOFF 014 START -->
What changed:
- Added run validation with `byom-video validate <run_id>`.
- Added machine-readable validation output with `--json`.
- Added manifest validation for required fields, status values, safe artifact paths, artifact list presence, failed-run error message warnings, and export status values.
- Added events validation for JSONL decoding, event names, timestamps, and lifecycle event warnings.
- Added artifact existence validation for manifest artifacts and exported files.
- Added known artifact schema validation for `transcript.json`, `chunks.json`, `highlights.json`, and `roughcut.json`.
- Added post-export integrity probing that writes `export_validation.json`.
- Added export validation lifecycle events.
- Added manifest fields for export validation status and error.
- Updated report refresh behavior so reports can show export validation status and exported clip durations.
- Added docs and smoke script for run validation and export integrity.
- Added tests for validation behavior and ffprobe metadata parsing.

Files added/modified:
- Added `internal/runvalidate/runvalidate.go`.
- Added `internal/runvalidate/runvalidate_test.go`.
- Added `internal/commands/validate.go`.
- Added `internal/exporter/validation.go`.
- Added `docs/artifacts/export-validation.md`.
- Added `scripts/smoke-validate-export.sh`.
- Modified `internal/manifest/manifest.go`.
- Modified `internal/exporter/exporter.go`.
- Modified `internal/exporter/exporter_test.go`.
- Modified `internal/report/report.go`.
- Modified `internal/commands/export.go`.
- Modified `internal/commands/runs.go`.
- Modified `internal/commands/runs_test.go`.
- Modified `internal/cli/root.go`.
- Modified `internal/runinfo/runinfo.go`.
- Modified `README.md`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/artifacts/events.md`.
- Modified `docs/artifacts/exports.md`.
- Modified `docs/artifacts/manifest.md`.
- Modified `docs/artifacts/report.md`.
- Modified `docs/runs.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video validate <run_id>
./byom-video validate <run_id> --json
```

New artifacts:
```text
.byom-video/runs/<run_id>/export_validation.json
```

Validation behavior:
- Resolves run ids safely under `.byom-video/runs`.
- Validates `manifest.json` structure and accepted values.
- Validates `events.jsonl` as JSON Lines with event names and timestamps.
- Warns when expected lifecycle events are absent.
- Validates listed artifact files exist.
- Validates known JSON artifact schemas when present.
- Exits non-zero when errors are found.
- Exits zero when only warnings are found.

Export integrity behavior:
- After successful `byom-video export <run_id>`, exported files are discovered under `exports/`.
- Each exported file is probed with `ffprobe`.
- Validation records duration, video stream count, audio stream count, status, and error.
- Successful validation records `export_validation.json` in the manifest and sets `export_validation_status: completed`.
- Failed validation leaves exported files in place and records `export_validation_status: failed` with `export_validation_error`.
- If `report.html` exists, export refreshes it after validation.

Commands run:
```sh
gofmt -w internal/manifest/manifest.go internal/exporter/exporter.go internal/exporter/validation.go internal/report/report.go internal/commands/export.go internal/commands/runs.go internal/commands/validate.go internal/cli/root.go internal/runinfo/runinfo.go internal/runvalidate/runvalidate.go
gofmt -w internal/runvalidate/runvalidate_test.go internal/exporter/exporter_test.go internal/commands/runs_test.go
chmod +x scripts/smoke-validate-export.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-validate-export.sh
./byom-video validate 20260428T234027Z-d162b0c9 --json
./byom-video export 20260428T234027Z-d162b0c9
./byom-video validate 20260428T234027Z-d162b0c9
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-validate-export.sh` passed against latest local run `20260428T234027Z-d162b0c9`.
- `validate --json` returned valid JSON with no errors.
- Explicit export succeeded and export validation completed for `exports/clip_0001.mp4`.

How to run smoke validate/export test:
```sh
scripts/smoke-validate-export.sh
scripts/smoke-validate-export.sh --export-first <run_id>
```

Known limitations:
- Validation is filesystem-based; there is still no database.
- Export validation depends on `ffprobe`; failure is recorded but exported files are preserved.
- Export validation currently records duration and stream counts only.
- Validate does not repair runs or rewrite old artifacts.
- Event validation accepts current `type`/`time` naming and compatible future-style `event`/`timestamp` naming.
- No deletion, archiving, tagging, search, database, web server, LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, or NLE integrations were added.

Next recommended milestone:
- Add deeper export integrity checks for zero-duration clips and expected roughcut clip count matching.
- Add optional cleanup/archive commands once retention rules are defined.
- Add checksums or size metadata for artifacts if reproducibility requirements increase.

Errors or assumptions:
- Assumed validation errors should be reported as command errors after printing the full validation result.
- Assumed missing failed-run `error_message` should be a warning, not an error.
- Assumed export validation failure should not fail the export command when FFmpeg export itself succeeded.
- Assumed event validation should accept the existing `type` and `time` fields as the current schema.
<!-- HANDOFF 014 END -->

## Prompt 015 - Batch Processing

<!-- PROMPT 015 START -->
Goal:
- Add batch processing for folders of media files.
- Add `byom-video batch <input-dir> --preset shorts`.
- Process multiple files sequentially by default.
- Keep each file as its own normal run.
- Produce a batch summary artifact under `.byom-video/batches/<batch_id>/batch_summary.json`.
- Add `batches` and `inspect-batch` commands.
- Support media file detection for common audio/video extensions.
- Support non-recursive default scanning, optional recursive scanning, limit, fail-fast, dry-run, validation, export, and export-and-validate flags.
- Keep batch local/filesystem-first and do not add database, web server, LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, or NLE integrations.
<!-- PROMPT 015 END -->

## Handoff 015

<!-- HANDOFF 015 START -->
What changed:
- Added batch processing with `byom-video batch <input-dir>`.
- Added batch listing with `byom-video batches`.
- Added batch inspection with `byom-video inspect-batch <batch_id>`.
- Added JSON batch inspection with `--json`.
- Added deterministic media file discovery with supported audio/video extensions.
- Added non-recursive default scans and optional recursive scans.
- Added dry-run mode that prints planned files without creating runs or batch artifacts.
- Added batch summary artifact creation for non-dry-run batches.
- Added optional post-run validation and export hooks.
- Added docs and smoke script for batch workflows.
- Added tests for detection, scanning, hidden file skipping, limits, dry-run, summary generation, fail-fast, list/inspect, invalid presets, and export flag rejection.

Files added/modified:
- Added `internal/batch/batch.go`.
- Added `internal/batch/batch_test.go`.
- Added `internal/commands/batch.go`.
- Added `internal/commands/batch_test.go`.
- Added `docs/batch.md`.
- Added `scripts/smoke-batch.sh`.
- Modified `internal/cli/root.go`.
- Modified `internal/cli/root_test.go`.
- Modified `README.md`.
- Modified `docs/presets.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video batch <input-dir> --preset metadata
./byom-video batch <input-dir> --preset shorts
./byom-video batches
./byom-video inspect-batch <batch_id>
./byom-video inspect-batch <batch_id> --json
```

New flags:
```sh
./byom-video batch <input-dir> --recursive
./byom-video batch <input-dir> --limit <n>
./byom-video batch <input-dir> --fail-fast
./byom-video batch <input-dir> --dry-run
./byom-video batch <input-dir> --preset <shorts|metadata>
./byom-video batch <input-dir> --validate
./byom-video batch <input-dir> --export
./byom-video batch <input-dir> --export-and-validate
```

Batch behavior:
- Scans a directory for supported media extensions: `.mp4`, `.mov`, `.m4v`, `.mp3`, `.wav`, `.m4a`, `.aac`, `.flac`, `.webm`, and `.mkv`.
- Extension matching is case-insensitive.
- Skips directories and hidden files.
- Sorts discovered files by path for deterministic order.
- Runs sequentially.
- Creates one normal run per attempted file.
- Continues after failures unless `--fail-fast` is passed.
- Does not export automatically.
- `--export` and `--export-and-validate` require a preset that generates `ffmpeg_commands.sh`; currently `shorts` qualifies and `metadata` does not.
- `--dry-run` prints what would be processed and writes no runs or batch artifacts.

Batch summary behavior:
- Non-dry-run batches write `.byom-video/batches/<batch_id>/batch_summary.json`.
- Summary schema is `batch_summary.v1`.
- Summary records batch id, created time, input dir, preset, recursive/dry-run flags, totals, item input paths, statuses, run ids, run dirs, and errors.
- `batches` lists newest summaries first.
- `inspect-batch` prints readable details or JSON.

Commands run:
```sh
gofmt -w internal/batch/batch.go internal/batch/batch_test.go internal/commands/batch.go internal/commands/batch_test.go internal/cli/root.go internal/cli/root_test.go
chmod +x scripts/smoke-batch.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-batch.sh
./byom-video batch media/batch-smoke --preset metadata --dry-run
./byom-video inspect-batch 20260429T040401Z-e39ae232 --json
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-batch.sh` passed with two metadata runs.
- `batch --dry-run` printed planned files and created no batch artifact.
- `inspect-batch --json` returned valid machine-readable batch summary.

How to run smoke batch test:
```sh
scripts/smoke-batch.sh
```

Known limitations:
- Batch execution is sequential only.
- Dry-run prints a generated batch id for display but intentionally writes no batch artifact.
- Batch summary run ids are captured from successful run output; very early failures may not record a run id even if a partial run folder exists.
- There are no batch-level events; only `batch_summary.json` is written.
- No deletion, archiving, tagging, search, database, web server, LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, or NLE integrations were added.

Next recommended milestone:
- Add optional batch resume/retry for failed items.
- Add batch-level validation/report aggregation.
- Add cleanup/archive commands once retention rules are defined.

Errors or assumptions:
- Chose dry-run to write no artifacts for safety.
- Chose simple `batch_summary.json` only instead of batch-level events.
- Assumed batch command failures should be item-level failures unless setup or summary writing fails.
- Assumed `--export` should be rejected for presets that do not generate FFmpeg scripts.
<!-- HANDOFF 015 END -->

## Prompt 016 - Watch Folder Mode

<!-- PROMPT 016 START -->
Goal:
- Add local watch-folder mode so BYOM Video can monitor a folder and process new media files automatically.
- Add `byom-video watch <input-dir> --preset shorts`.
- Use polling by default for portability.
- Detect supported media files using the same extension rules as batch.
- Process newly discovered stable files sequentially.
- Avoid processing files while they are still being copied by checking size and modified time stability.
- Avoid reprocessing the same file fingerprint with a local processed registry.
- Add `byom-video watch-status`.
- Keep local-first and filesystem-first.
- Do not add database, web server, LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, or NLE integrations.
<!-- PROMPT 016 END -->

## Handoff 016

<!-- HANDOFF 016 START -->
What changed:
- Added watch-folder mode with `byom-video watch <input-dir>`.
- Added watch status inspection with `byom-video watch-status`.
- Added polling-based discovery with configurable interval.
- Added stable-file detection using file size and modified time.
- Added local processed registry at `.byom-video/watch/processed.json`.
- Added registry fingerprinting using absolute path, file size, and modified time.
- Added retry support with `--ignore-registry`.
- Added once mode for automation and tests.
- Added optional post-run export and validation flags.
- Added docs and smoke script for watch workflows.
- Added tests for stable detection, fingerprints, registry load/save/update, registry skip behavior, ignore-registry retry behavior, once mode, hidden file skipping, limit behavior, invalid interval rejection, and watch-status JSON.

Files added/modified:
- Added `internal/watch/watch.go`.
- Added `internal/watch/watch_test.go`.
- Added `internal/commands/watch.go`.
- Added `internal/commands/watch_test.go`.
- Added `docs/watch.md`.
- Added `scripts/smoke-watch.sh`.
- Modified `internal/cli/root.go`.
- Modified `internal/cli/root_test.go`.
- Modified `README.md`.
- Modified `docs/batch.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video watch <input-dir> --preset metadata
./byom-video watch <input-dir> --preset shorts
./byom-video watch-status
./byom-video watch-status --json
```

New flags:
```sh
./byom-video watch <input-dir> --preset <shorts|metadata>
./byom-video watch <input-dir> --interval-seconds <n>
./byom-video watch <input-dir> --recursive
./byom-video watch <input-dir> --once
./byom-video watch <input-dir> --limit <n>
./byom-video watch <input-dir> --fail-fast
./byom-video watch <input-dir> --export
./byom-video watch <input-dir> --validate
./byom-video watch <input-dir> --export-and-validate
./byom-video watch <input-dir> --ignore-registry
```

Watch behavior:
- Uses portable polling, not platform-specific file watching.
- Default interval is 5 seconds.
- Interval must be positive.
- Uses batch media detection for supported extensions.
- Skips hidden files and directories.
- Processes files sequentially.
- Creates one normal run per processed file.
- Does not export automatically unless `--export` or `--export-and-validate` is passed.
- Handles Ctrl+C through signal-aware context cancellation.
- `--once` scans once, processes stable unprocessed files, and exits.
- `--limit` caps processed files in the invocation.
- `--fail-fast` stops after the first processing failure.

Registry behavior:
- Registry path is `.byom-video/watch/processed.json`.
- Registry schema is `watch_processed.v1`.
- Fingerprint is absolute path, file size, and modified time.
- Matching registry entries prevent reprocessing by default.
- Failed processing attempts are recorded in the registry.
- `--ignore-registry` allows retrying matching fingerprints and updates the registry afterward.

Commands run:
```sh
gofmt -w internal/watch/watch.go internal/watch/watch_test.go internal/commands/watch.go internal/commands/watch_test.go internal/cli/root.go internal/cli/root_test.go
chmod +x scripts/smoke-watch.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-watch.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-watch.sh` passed.
- Smoke watch processed a metadata fixture once, wrote the registry, and confirmed the second `--once` pass did not reprocess it.

How to run smoke watch test:
```sh
scripts/smoke-watch.sh
```

Known limitations:
- Watch mode is polling-only.
- There is no debounce beyond size/mtime stability.
- Fingerprints do not hash file contents.
- Watch registry is a single local JSON file; there is no locking for multiple concurrent watch processes.
- `--once` only processes files stable at scan time; recently copied files may require a later invocation.
- No deletion, archiving, tagging, search, database, web server, LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, or NLE integrations were added.

Next recommended milestone:
- Add retry/resume commands for failed watch and batch items.
- Add aggregated watch/batch reports.
- Add optional registry locking if concurrent automation becomes a requirement.

Errors or assumptions:
- Assumed polling is preferable to adding a watcher dependency.
- Assumed first-seen files older than the polling interval can be considered stable.
- Assumed failed registry entries should prevent repeated failures unless `--ignore-registry` is used.
- Assumed watch status should report the latest 10 registry items in readable mode.
<!-- HANDOFF 016 END -->

## Prompt 017 - Retry Resume + Safe Cleanup

<!-- PROMPT 017 START -->
Goal:
- Add retry/resume commands for failed batch and watch items.
- Add rerun support for a single input from an existing run.
- Add basic safe cleanup commands for failed and incomplete run folders.
- Keep local-first and filesystem-first.
- Do not add database, web server, LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, NLE integrations, or destructive cleanup without explicit flags.
<!-- PROMPT 017 END -->

## Handoff 017

<!-- HANDOFF 017 START -->
What changed:
- Added `retry-batch <batch_id>` for failed batch items.
- Added `retry-watch` for failed watch registry items.
- Added `rerun <run_id>` for creating a new run from an existing run manifest input.
- Added `cleanup` for failed, stale-running, and missing-manifest run folder candidates.
- Added cleanup dry-run by default.
- Added cleanup deletion only with `--delete` plus interactive confirmation or `--yes`.
- Added cleanup path safety under `.byom-video/runs`.
- Added docs and smoke script for retry and cleanup.
- Added tests for retry dry-runs, missing input handling, preset inference, cleanup candidates, cleanup non-delete behavior, unsafe delete rejection, confirmation requirement, and confirmed deletion.

Files added/modified:
- Added `internal/cleanup/cleanup.go`.
- Added `internal/cleanup/cleanup_test.go`.
- Added `internal/commands/recovery.go`.
- Added `internal/commands/recovery_test.go`.
- Added `docs/retry.md`.
- Added `docs/cleanup.md`.
- Added `scripts/smoke-retry-cleanup.sh`.
- Modified `internal/cli/root.go`.
- Modified `README.md`.
- Modified `docs/batch.md`.
- Modified `docs/watch.md`.
- Modified `docs/runs.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video retry-batch <batch_id>
./byom-video retry-watch
./byom-video rerun <run_id>
./byom-video cleanup
```

New flags:
```sh
./byom-video retry-batch <batch_id> --limit <n>
./byom-video retry-batch <batch_id> --fail-fast
./byom-video retry-batch <batch_id> --dry-run
./byom-video retry-batch <batch_id> --export
./byom-video retry-batch <batch_id> --validate
./byom-video retry-batch <batch_id> --export-and-validate

./byom-video retry-watch --preset <shorts|metadata>
./byom-video retry-watch --limit <n>
./byom-video retry-watch --fail-fast
./byom-video retry-watch --dry-run
./byom-video retry-watch --export
./byom-video retry-watch --validate
./byom-video retry-watch --export-and-validate

./byom-video rerun <run_id> --preset <shorts|metadata>
./byom-video rerun <run_id> --dry-run
./byom-video rerun <run_id> --export
./byom-video rerun <run_id> --validate
./byom-video rerun <run_id> --export-and-validate

./byom-video cleanup --failed
./byom-video cleanup --stale-running
./byom-video cleanup --missing-manifest
./byom-video cleanup --older-than-hours <n>
./byom-video cleanup --delete
./byom-video cleanup --limit <n>
./byom-video cleanup --json
./byom-video cleanup --yes
```

Retry behavior:
- `retry-batch` loads the original batch summary, retries only failed items, and writes a new batch summary under a new batch id.
- `retry-batch --dry-run` prints failed items and writes no artifacts.
- `retry-watch` loads `.byom-video/watch/processed.json`, retries only failed items, and updates the watch registry.
- Missing input files are handled as clean failed retry items.
- Retry commands create new normal runs and do not modify original run folders.

Rerun behavior:
- `rerun <run_id>` reads the old run manifest and original `input_path`.
- It creates a new run and does not modify the old run.
- If no preset override is provided, it infers `shorts` when roughcut/report/ffmpeg script artifacts are present and `metadata` otherwise.
- `--dry-run` prints the inferred work without creating a run.

Cleanup behavior:
- `cleanup` is dry-run by default.
- Candidates are failed runs, running runs older than 24 hours, and run directories with missing manifests.
- Candidate kinds can be filtered by flag.
- `--delete` is required to remove anything.
- Without `--yes`, deletion asks for interactive confirmation.
- Deletion resolves only under `.byom-video/runs` and removes only selected run directories.
- Cleanup never deletes input media.

Commands run:
```sh
gofmt -w internal/cleanup/cleanup.go internal/commands/recovery.go internal/cli/root.go
gofmt -w internal/cleanup/cleanup_test.go internal/commands/recovery_test.go
chmod +x scripts/smoke-retry-cleanup.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-retry-cleanup.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-retry-cleanup.sh` passed.
- Smoke retry/cleanup ran retry-batch dry-run and cleanup dry-run only; no directories were deleted.

How to run smoke retry/cleanup test:
```sh
scripts/smoke-retry-cleanup.sh
```

Known limitations:
- Retry commands rerun from the original input path and do not resume partially completed internal pipeline stages.
- Inferred rerun presets are broad: `shorts` or `metadata`.
- Retry batch uses the original batch preset but not every original CLI/config nuance.
- Cleanup has no archive mode yet.
- Cleanup has no age filter for failed runs beyond candidate kind filters.
- No database, web server, LLM calls, model routing/provider clients, Inference Mask implementation, Docker, vector DB, or NLE integrations were added.

Next recommended milestone:
- Add archive mode before deletion.
- Add aggregated batch/watch reports for operations history.
- Add more precise retry metadata so retries can preserve exact original flags/config.

Errors or assumptions:
- Assumed retries should create fresh runs instead of modifying failed run folders.
- Assumed cleanup should be dry-run by default and require explicit delete confirmation.
- Assumed failed watch registry items should remain retryable through `retry-watch` without needing `--ignore-registry`.
- Assumed rerun preset inference should stay intentionally coarse.
<!-- HANDOFF 017 END -->

## Prompt 018 - Agent Task Planner v1

<!-- PROMPT 018 START -->
Goal:
- Add Agent Task Planner v1: a local deterministic planner that turns simple user goals into executable BYOM Video pipeline plans.
- Add `byom-video plan <input-file> --goal "make 5 shorts"`.
- Write plan artifacts under `.byom-video/plans/<plan_id>/agent_plan.json`.
- Write separate action logs under `.byom-video/plans/<plan_id>/actions.jsonl`.
- Add optional execution with `--execute`.
- Keep exports explicit with `--with-export`.
- Add `plans` and `inspect-plan`.
- Keep deterministic and local.
- Do not add LLM calls, model routing/provider clients, OpenAI/Claude/Groq/NVIDIA/Ollama/Gemma/Kimi/Qwen clients, Inference Mask implementation, Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 018 END -->

## Handoff 018

<!-- HANDOFF 018 START -->
What changed:
- Added deterministic agent task planner with `byom-video plan`.
- Added plan artifact writing to `.byom-video/plans/<plan_id>/agent_plan.json`.
- Added plan action log writing to `.byom-video/plans/<plan_id>/actions.jsonl`.
- Added optional plan execution with action status updates.
- Added deterministic goal parser for metadata, transcription, captions, highlights, clips, roughcuts, and shorts goals.
- Added max clip extraction from goals such as `make 5 shorts`.
- Added `plans` listing command.
- Added `inspect-plan <plan_id>` with readable and JSON output.
- Added docs and smoke script for agent planning.
- Added tests for goal mapping, max clip extraction, unknown goals, artifact generation, action logs, dry-run planning, plan listing, and JSON inspection.

Files added/modified:
- Added `internal/agent/agent.go`.
- Added `internal/agent/agent_test.go`.
- Added `internal/commands/agent.go`.
- Added `internal/commands/agent_test.go`.
- Added `docs/agent.md`.
- Added `scripts/smoke-agent-plan.sh`.
- Modified `internal/cli/root.go`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video plan <input-file> --goal "make 5 shorts"
./byom-video plan <input-file> --goal "make 5 shorts" --execute
./byom-video plans
./byom-video inspect-plan <plan_id>
./byom-video inspect-plan <plan_id> --json
```

New flags:
```sh
./byom-video plan <input-file> --goal <text>
./byom-video plan <input-file> --execute
./byom-video plan <input-file> --preset <shorts|metadata>
./byom-video plan <input-file> --max-clips <n>
./byom-video plan <input-file> --with-export
./byom-video plan <input-file> --with-validate
./byom-video plan <input-file> --with-report
./byom-video plan <input-file> --dry-run
```

Goal mapping behavior:
- `metadata only` maps to metadata preset.
- `transcribe this` maps to transcript-only options.
- `make captions` maps to transcript plus captions.
- `find highlights` maps to transcript, chunks, and highlights.
- `roughcut`, `clips`, or `shorts` goals map to transcript, captions, chunks, highlights, roughcut, FFmpeg script, and report.
- A number before `shorts` or `clips` sets `roughcut_max_clips`.
- Unknown goals return a clean error with examples and do not call an LLM.

Plan artifact behavior:
- Plans use schema `agent_plan.v1`.
- Plans include plan id, creation time, input path, goal, deterministic mode, preset, status, actions, action options, and safety fields.
- `--execute` updates action statuses and records resulting run id when available.
- `--dry-run` still writes the plan artifact but does not execute anything.

Action log behavior:
- Action logs are JSONL files at `.byom-video/plans/<plan_id>/actions.jsonl`.
- Current events include `PLAN_CREATED`, `PLAN_EXECUTION_STARTED`, `ACTION_STARTED`, `ACTION_COMPLETED`, `ACTION_FAILED`, `PLAN_EXECUTION_COMPLETED`, and `PLAN_EXECUTION_FAILED`.
- Plan action logs are separate from run-level `events.jsonl`.

Commands run:
```sh
gofmt -w internal/agent/agent.go internal/commands/agent.go internal/cli/root.go
gofmt -w internal/agent/agent_test.go internal/commands/agent_test.go
chmod +x scripts/smoke-agent-plan.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-agent-plan.sh
./byom-video inspect-plan 20260429T045949Z-9dd67b5f --json
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-agent-plan.sh` passed.
- `inspect-plan --json` returned a valid `agent_plan.v1` plan with `roughcut_max_clips: 3`.
- `actions.jsonl` contained `PLAN_CREATED` for the smoke plan.

How to run smoke agent-plan test:
```sh
scripts/smoke-agent-plan.sh
```

Optional execution:
```sh
scripts/smoke-agent-plan.sh --execute
```

Known limitations:
- Planner is deterministic and rule-based only.
- No LLM/provider clients are present.
- Goal parsing is intentionally simple string matching.
- Execution only supports local pipeline/export/validate actions.
- Plan execution does not yet support batch or watch planning.
- No Inference Mask, Docker, vector DB, web server, NLE integrations, or model routing were added.

Next recommended milestone:
- Add richer plan templates for batch/watch goals.
- Add plan validation before execution.
- Add exact command previews for every planned action.
- Later, add an Inference Mask design layer before any LLM/provider integration.

Errors or assumptions:
- Assumed `--dry-run` should write plan artifacts but execute nothing.
- Assumed shorts-style goals should include reports by default.
- Assumed export must remain opt-in through `--with-export`.
- Assumed unknown goals should fail cleanly rather than falling back to any model.
<!-- HANDOFF 018 END -->

## Prompt 019 - Agent Plan Validation + Command Previews

<!-- PROMPT 019 START -->
Goal:
- Harden Agent Task Planner v1 with plan validation, exact command previews, and batch/watch planning templates.
- Add plan validation before execution.
- Add `command_preview` to each planned action.
- Extend deterministic goal mapping for batch and watch plans.
- Add `--mode file|batch|watch`, `--recursive`, `--once`, and `--limit` to planning.
- Keep watch execution safe by requiring `--once` for watch plan execution in this version.
- Keep deterministic and local.
- Do not add LLM calls, model routing/provider clients, OpenAI/Claude/Groq/NVIDIA/Ollama/Gemma/Kimi/Qwen clients, Inference Mask implementation, Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 019 END -->

## Handoff 019

<!-- HANDOFF 019 START -->
What changed:
- Added plan validation before execution.
- Added validation status and validation errors to plan artifacts.
- Added plan validation action-log events.
- Added `command_preview` to plan actions.
- Added target type to plan artifacts: `file`, `batch`, or `watch`.
- Added batch planning with `batch_pipeline` actions.
- Added watch planning with `watch_pipeline` actions.
- Added execution routing for batch and watch plans through existing local commands.
- Added safety check that watch plan execution requires `--once`.
- Added docs for agent safety and expanded planning.
- Added expanded agent smoke script.
- Added tests for plan validation, unsupported action types, missing safety fields, command previews, batch/watch mapping, mode override, watch execution safety, and inspect-plan previews.

Files added/modified:
- Modified `internal/agent/agent.go`.
- Modified `internal/agent/agent_test.go`.
- Modified `internal/commands/agent.go`.
- Modified `internal/commands/agent_test.go`.
- Modified `internal/cli/root.go`.
- Added `docs/agent-safety.md`.
- Added `scripts/smoke-agent-expanded.sh`.
- Modified `docs/agent.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New flags:
```sh
./byom-video plan <path> --mode <file|batch|watch>
./byom-video plan <path> --recursive
./byom-video plan <path> --once
./byom-video plan <path> --limit <n>
```

Plan validation behavior:
- Execution validates plans before running actions.
- Validation checks schema version, plan id, input path, goal, non-empty actions, supported action types, valid statuses, required action fields, action options, and safety fields.
- Supported action types are `run_pipeline`, `batch_pipeline`, `watch_pipeline`, `export_run`, and `validate_run`.
- Supported action statuses are `planned`, `running`, `completed`, `failed`, and `skipped`.
- Validation writes `PLAN_VALIDATION_STARTED`, `PLAN_VALIDATION_COMPLETED`, or `PLAN_VALIDATION_FAILED`.
- If validation fails, the plan is marked failed, validation errors are written, and no actions execute.

Command preview behavior:
- Every planned action includes `command_preview`.
- File plans preview commands such as:
```sh
./byom-video pipeline "media/input.mov" --preset shorts
```
- Batch plans preview commands such as:
```sh
./byom-video batch "media/folder" --preset shorts
```
- Watch plans preview commands such as:
```sh
./byom-video watch "media/inbox" --preset shorts --once
```
- Export and validation previews use `<run_id>` until execution has a concrete run id.
- `plan` and `inspect-plan` readable output show command previews.

Batch/watch planning behavior:
- Directory input plus batch/process goals maps to `batch_pipeline`.
- Directory input plus watch/monitor/keep-processing goals maps to `watch_pipeline`.
- `--mode` overrides inference.
- `--recursive` and `--limit` apply to batch/watch previews and execution.
- `--once` applies to watch previews and execution.

Execution safety behavior:
- Watch plan execution without `--once` fails with:
```text
watch plan execution requires --once in this version
```
- This prevents accidental endless agent execution.
- Exports remain opt-in through `--with-export`.
- Planner remains deterministic and local.

Commands run:
```sh
gofmt -w internal/agent/agent.go internal/commands/agent.go internal/cli/root.go
gofmt -w internal/agent/agent_test.go internal/commands/agent_test.go
chmod +x scripts/smoke-agent-expanded.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-agent-expanded.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-agent-expanded.sh` passed.
- Expanded smoke created file, batch, and watch plans with command previews and did not execute them.

How to run smoke expanded-agent test:
```sh
scripts/smoke-agent-expanded.sh
```

Known limitations:
- Planner is still deterministic and rule-based only.
- Batch/watch execution uses existing command paths and does not yet support all future planner action types.
- Watch execution requires `--once`; long-running watch plans are intentionally blocked.
- Plan validation is structural and local; it is not a semantic optimizer.
- No LLM/provider clients, Inference Mask, Docker, vector DB, web server, NLE integrations, or model routing were added.

Next recommended milestone:
- Add richer command preview rendering for custom transcript/caption/highlight-only plans.
- Add plan diff/review command before execution.
- Add exact preservation of config/flags in retry and planner execution metadata.
- Start Inference Mask design docs before any provider integration.

Errors or assumptions:
- Assumed action previews should show local CLI command shapes, not shell-escaped scripts.
- Assumed watch execution should remain gated by `--once` for safety.
- Assumed directory input should infer batch unless goal clearly asks for watch/monitor.
- Assumed old plan fields should remain readable while new fields are added.
<!-- HANDOFF 019 END -->

## Prompt 020 - Plan Review Diff + Approval Gate

<!-- PROMPT 020 START -->
Goal:
- Add plan review, plan diff, and approval-gated execution.
- Add `review-plan <plan_id>`.
- Add `approve-plan <plan_id>`.
- Add `execute-plan <plan_id>`.
- Add `diff-plan <plan_id_a> <plan_id_b>`.
- Require explicit approval before executing saved plans unless `--yes` is passed.
- Keep deterministic and local.
- Do not add LLM calls, model routing/provider clients, OpenAI/Claude/Groq/NVIDIA/Ollama/Gemma/Kimi/Qwen clients, Inference Mask implementation, Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 020 END -->

## Handoff 020

<!-- HANDOFF 020 START -->
What changed:
- Added `review-plan` for human-readable and JSON plan review.
- Added `approve-plan` for manual approval metadata.
- Added `execute-plan` for approval-gated saved-plan execution.
- Added `diff-plan` for local plan comparison.
- Added approval fields to `agent_plan.json`.
- Added review status to plans.
- Added `PLAN_APPROVED` action log event.
- Updated inline `plan --execute` to record `approval_mode: inline_execute`.
- Added docs and smoke script for approval workflow.
- Added tests for review previews, approval metadata, unapproved execution rejection, `--yes` approval bypass logging, validation before execution, diff behavior, backward-compatible pending approval, and inline execute approval mode.

Files added/modified:
- Modified `internal/agent/agent.go`.
- Modified `internal/commands/agent.go`.
- Added `internal/commands/plan_review.go`.
- Added `internal/commands/plan_review_test.go`.
- Modified `internal/cli/root.go`.
- Added `scripts/smoke-agent-approval.sh`.
- Modified `docs/agent.md`.
- Modified `docs/agent-safety.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video review-plan <plan_id>
./byom-video approve-plan <plan_id>
./byom-video execute-plan <plan_id>
./byom-video diff-plan <plan_id_a> <plan_id_b>
```

New flags:
```sh
./byom-video review-plan <plan_id> --json
./byom-video execute-plan <plan_id> --yes
./byom-video execute-plan <plan_id> --dry-run
./byom-video execute-plan <plan_id> --with-export
./byom-video execute-plan <plan_id> --with-validate
./byom-video diff-plan <plan_id_a> <plan_id_b> --json
```

Review behavior:
- `review-plan` loads `agent_plan.json`.
- It prints plan id, goal, input path, target type, preset, approval status, safety flags, validation status, actions, command previews, export inclusion, validation inclusion, and watch mode.
- If validation has not run, review validates read-only from the user's perspective, stores validation status/errors, and does not execute actions.
- `--json` emits a machine-readable review summary.

Approval behavior:
- `approve-plan` validates the plan first.
- If valid, it writes `approval_status: approved`, `approved_at`, and `approval_mode: manual`.
- It writes `PLAN_APPROVED` to `actions.jsonl`.
- Existing plans without approval fields are treated as `approval_status: pending`.

Execute-plan behavior:
- `execute-plan` requires `approval_status: approved`.
- `--yes` bypasses the approval requirement and records `approval_mode: yes_flag`.
- `--dry-run` prints the planned execution without running actions.
- It validates again before execution.
- It rejects `--with-export` and `--with-validate` instead of mutating saved plans; users should create a new plan with those flags.
- Existing `plan --execute` still works and records `approval_mode: inline_execute`.

Diff behavior:
- `diff-plan` compares goal, input path, target type, preset, safety fields, action count, action types, command previews, and action options.
- `--json` emits structured differences.
- No external diff dependency was added.

Commands run:
```sh
gofmt -w internal/agent/agent.go internal/commands/agent.go internal/commands/plan_review.go internal/cli/root.go
gofmt -w internal/commands/plan_review_test.go
chmod +x scripts/smoke-agent-approval.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-agent-approval.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-agent-approval.sh` passed.
- Smoke approval created a plan, reviewed it, approved it, dry-ran execution, and diffed it against itself without executing the media pipeline.

How to run smoke approval test:
```sh
scripts/smoke-agent-approval.sh
```

Known limitations:
- Approval is local metadata only; there is no user identity or signature.
- `execute-plan --dry-run` does not validate approval state because it intentionally does not execute.
- Plan diff is field-based and not a semantic diff.
- Saved-plan execution cannot add export/validation flags; create a new plan instead.
- No LLM/provider clients, Inference Mask, Docker, vector DB, web server, NLE integrations, or model routing were added.

Next recommended milestone:
- Add plan review/diff snapshots to artifacts.
- Add signed/local user approval metadata if multi-user workflows matter.
- Add command preview improvements for custom transcript/caption/highlight-only plans.
- Add Inference Mask design before any future provider integration.

Errors or assumptions:
- Assumed approval metadata is enough for local single-user workflows.
- Assumed `--yes` should record approval bypass in the plan.
- Assumed saved plans should not be silently mutated by execution flags.
- Assumed diff output should stay simple and dependency-free.
<!-- HANDOFF 020 END -->

## Prompt 021 - Plan Snapshots + Revision Loop v1

<!-- PROMPT 021 START -->
Goal:
- Add plan snapshots and deterministic revision loop v1.
- Preserve plan snapshots before modifications.
- Add safe deterministic plan revisions.
- Compare revised plans.
- Support requests such as `make it shorter`, `make 3 clips instead`, `metadata only`, `add validation`, `remove export`, `make captions only`, and `focus on highlights`.
- Keep deterministic and local.
- Do not add LLM calls, model routing/provider clients, OpenAI/Claude/Groq/NVIDIA/Ollama/Gemma/Kimi/Qwen clients, Inference Mask implementation, Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 021 END -->

## Handoff 021

<!-- HANDOFF 021 START -->
What changed:
- Added plan snapshots under `.byom-video/plans/<plan_id>/snapshots/`.
- Added snapshot creation before plan approval, inline execute approval, `execute-plan --yes`, and revisions.
- Added `snapshots <plan_id>`.
- Added `inspect-snapshot <plan_id> <snapshot_id>`.
- Added `revise-plan <plan_id> --request <text>`.
- Added `diff-snapshot <plan_id> <snapshot_id>`.
- Added deterministic revision mappings for shorter/longer, N clips/shorts, validation, export, captions-only, metadata-only, and highlights.
- Added approval reset when executable actions/options change.
- Added revision dry-run and JSON output.
- Added revision diff output with `--show-diff`.
- Added docs and smoke script for plan revisions.
- Added tests for snapshot creation/listing/inspection, revision mappings, approval reset, dry-run non-mutation, unknown request non-mutation, and show-diff output.

Files added/modified:
- Added `internal/agent/snapshot.go`.
- Added `internal/agent/snapshot_test.go`.
- Added `internal/commands/revision.go`.
- Added `internal/commands/revision_test.go`.
- Added `docs/plan-revisions.md`.
- Added `scripts/smoke-plan-revision.sh`.
- Modified `internal/commands/agent.go`.
- Modified `internal/commands/plan_review.go`.
- Modified `internal/cli/root.go`.
- Modified `docs/agent.md`.
- Modified `docs/agent-safety.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video snapshots <plan_id>
./byom-video inspect-snapshot <plan_id> <snapshot_id>
./byom-video revise-plan <plan_id> --request <text>
./byom-video diff-snapshot <plan_id> <snapshot_id>
```

New flags:
```sh
./byom-video inspect-snapshot <plan_id> <snapshot_id> --json
./byom-video revise-plan <plan_id> --request <text>
./byom-video revise-plan <plan_id> --dry-run
./byom-video revise-plan <plan_id> --json
./byom-video revise-plan <plan_id> --show-diff
./byom-video diff-snapshot <plan_id> <snapshot_id> --json
```

Snapshot behavior:
- Snapshots are written as `snapshot_0001.json`, `snapshot_0002.json`, etc.
- Each snapshot includes snapshot id, creation time, reason, and a full plan copy.
- Snapshot listing shows snapshot id, created time, and reason.
- Snapshot inspection supports readable and JSON output.
- Approval and revision commands snapshot before modifying a plan when practical.

Revision behavior:
- `make it shorter` reduces `roughcut_max_clips` by 1, minimum 1.
- `make it longer` increases `roughcut_max_clips` by 1, max 20.
- `make 3 clips` or `make 3 shorts` sets `roughcut_max_clips` to 3.
- `add validation` adds a `validate_run` action if missing.
- `remove validation` removes validation actions.
- `add export` adds an explicit `export_run` action and resets approval.
- `remove export` removes export actions.
- `captions only` changes pipeline options to transcript plus captions.
- `metadata only` changes pipeline options to metadata.
- `find highlights` changes pipeline options to transcript, chunks, and highlights.
- Unknown revision requests return a clean error and do not mutate the plan.

Approval reset behavior:
- Revisions that change executable actions or options reset `approval_status` to `pending`.
- `approved_at`, `approval_mode`, validation status, and validation errors are cleared on executable revision.
- Dry-run revisions do not mutate the plan or reset approval.

Diff behavior:
- `revise-plan --show-diff` shows the diff between the pre-revision snapshot and revised plan.
- `diff-snapshot` compares a current plan against a snapshot.
- Diff remains local and field-based.

Commands run:
```sh
gofmt -w internal/agent/snapshot.go internal/commands/revision.go internal/cli/root.go
gofmt -w internal/agent/snapshot_test.go internal/commands/revision_test.go
chmod +x scripts/smoke-plan-revision.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-plan-revision.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-plan-revision.sh` passed.
- Smoke plan revision created a plan, revised `roughcut_max_clips` from 5 to 3, wrote `snapshot_0001`, showed a diff, listed snapshots, and reviewed the revised plan without execution.

How to run smoke plan-revision test:
```sh
scripts/smoke-plan-revision.sh
```

Known limitations:
- Revision parser is deterministic string matching only.
- Revision diff is field-based, not semantic.
- Snapshot files are local JSON artifacts without signing or compression.
- `metadata only`, `captions only`, and `find highlights` revise action options but command previews for custom non-preset plans still use broad local command shapes.
- No LLM/provider clients, Inference Mask, Docker, vector DB, web server, NLE integrations, or model routing were added.

Next recommended milestone:
- Improve exact command previews for custom non-preset plans.
- Add plan review/diff snapshot export artifacts.
- Add richer deterministic revision requests for batch/watch controls.
- Begin Inference Mask design docs before any provider integration.

Errors or assumptions:
- Assumed snapshots should be created before plan mutations, not after.
- Assumed adding export is allowed only as an explicit revision request and must reset approval.
- Assumed approval reset should happen for any executable action/option change.
- Fixed revision diffing to deep-copy plan maps before mutation so approval resets cannot be skipped by shallow-copy aliasing.
<!-- HANDOFF 021 END -->

## Prompt 022 - Exact Previews + Plan Review Artifacts

<!-- PROMPT 022 START -->
Goal:
- Improve exact command previews for custom non-preset plans.
- Add durable plan review artifacts.
- Add durable plan diff artifacts.
- Add plan artifact navigation with `plan-artifacts`.
- Keep deterministic and local.
- Do not add LLM calls, model routing/provider clients, OpenAI/Claude/Groq/NVIDIA/Ollama/Gemma/Kimi/Qwen clients, Inference Mask implementation, Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 022 END -->

## Handoff 022

<!-- HANDOFF 022 START -->
What changed:
- Added exact command preview generation for `run_pipeline` actions with explicit pipeline options.
- Updated new file plans and revised plans to show exact `byom-video run` flags.
- Added generated review artifacts with `review-plan --write-artifact`.
- Added generated diff artifacts with `diff-plan --write-artifact`.
- Added generated snapshot diff artifacts with `diff-snapshot --write-artifact`.
- Added `plan-artifacts <plan_id>` for local plan artifact navigation.
- Updated `inspect-plan` readable output to include action log path, review artifact path, snapshot count, and diff artifact paths.
- Added action log events for review and diff artifact writes.
- Added docs and smoke script for plan artifacts.
- Added tests for exact previews, revised preview updates, review/diff artifact writes, inspect artifact paths, and plan-artifacts readable/JSON output.

Files added/modified:
- Added `scripts/smoke-plan-artifacts.sh`.
- Modified `internal/agent/agent.go`.
- Modified `internal/agent/agent_test.go`.
- Modified `internal/commands/agent.go`.
- Modified `internal/commands/agent_test.go`.
- Modified `internal/commands/plan_review.go`.
- Modified `internal/commands/plan_review_test.go`.
- Modified `internal/commands/revision.go`.
- Modified `internal/commands/revision_test.go`.
- Modified `internal/cli/root.go`.
- Modified `docs/agent.md`.
- Modified `docs/agent-safety.md`.
- Modified `docs/plan-revisions.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video plan-artifacts <plan_id>
./byom-video plan-artifacts <plan_id> --json
```

New flags:
```sh
./byom-video review-plan <plan_id> --write-artifact
./byom-video diff-plan <plan_id_a> <plan_id_b> --write-artifact
./byom-video diff-snapshot <plan_id> <snapshot_id> --write-artifact
```

Exact preview behavior:
- Metadata-only file plans preview `./byom-video run "<input>"`.
- Transcript-only plans preview `--with-transcript --transcript-model-size tiny`.
- Captions-only plans preview `--with-transcript --with-captions --transcript-model-size tiny`.
- Highlights plans preview `--with-transcript --with-chunks --with-highlights --transcript-model-size tiny`.
- Shorts plans preview transcript, captions, chunks, highlights, roughcut, FFmpeg script, report, transcript model size, and roughcut max clips.
- Revision refreshes command previews after changing executable options.
- Batch and watch previews still use their existing command shapes with recursive, once, and limit flags when present.

Review artifact behavior:
- `review-plan --write-artifact` writes `.byom-video/plans/<plan_id>/review.md`.
- Review artifacts include timestamp, plan id, goal, input path, target type, preset, approval status, validation status, safety fields, actions, command previews, and errors when present.
- Review artifacts are generated and overwritten on each write.
- Writes `PLAN_REVIEW_ARTIFACT_WRITTEN` to the plan action log.

Diff artifact behavior:
- `diff-plan --write-artifact` writes `.byom-video/plans/<plan_id_a>/diffs/diff_<plan_id_a>_vs_<plan_id_b>.md`.
- `diff-snapshot --write-artifact` writes `.byom-video/plans/<plan_id>/diffs/diff_current_vs_<snapshot_id>.md`.
- Diff artifacts include compared ids, timestamp, readable field differences, command preview differences, and action option differences.
- Writes `PLAN_DIFF_ARTIFACT_WRITTEN` to the plan action log.

Plan artifact navigation behavior:
- `inspect-plan` shows the action log path, review path if present, snapshot count, and diff artifact paths.
- `plan-artifacts` prints paths for `agent_plan.json`, `actions.jsonl`, `review.md`, snapshots, and diffs.
- `plan-artifacts --json` emits the same paths in machine-readable form.

Commands run:
```sh
gofmt -w internal/agent/agent.go internal/agent/agent_test.go internal/commands/agent.go internal/commands/agent_test.go internal/commands/plan_review.go internal/commands/plan_review_test.go internal/commands/revision.go internal/commands/revision_test.go internal/cli/root.go
chmod +x scripts/smoke-plan-artifacts.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-plan-artifacts.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-plan-artifacts.sh` passed.
- Smoke plan artifacts created a shorts plan, revised it to captions-only, wrote `review.md`, wrote `diff_current_vs_snapshot_0001.md`, and listed all plan artifacts.

How to run smoke plan-artifacts test:
```sh
scripts/smoke-plan-artifacts.sh
```

Known limitations:
- Exact previews are still deterministic command previews, not a shell-escaping library.
- Custom preview support is focused on current file pipeline options.
- Review and diff artifacts are local markdown files without signing or compression.
- Diff artifacts are field-based, not semantic.
- No LLM/provider clients, Inference Mask, Docker, vector DB, web server, NLE integrations, or model routing were added.

Next recommended milestone:
- Add review/diff export bundles if plan artifacts need to be shared outside the local workspace.
- Add richer deterministic revision requests for batch/watch controls.
- Add plan metadata for exact originating config values.
- Begin Inference Mask design docs before any provider integration.

Errors or assumptions:
- Assumed generated review artifacts should overwrite existing `review.md`.
- Assumed `review-plan --json --write-artifact` and diff JSON plus write should keep JSON stdout clean while writing artifacts silently.
- Assumed exact previews should use current default transcript model size `tiny` when transcript is enabled and no explicit model size is stored.
- Assumed plan artifact paths should be local relative paths under `.byom-video/plans`.
<!-- HANDOFF 022 END -->

## Prompt 023 - BYOM Config Skeleton + Inference Mask Contracts

<!-- PROMPT 023 START -->
Goal:
- Add BYOM model configuration skeleton and Inference Mask artifact design contracts without calling model providers.
- Extend `byom-video.yaml` with disabled `models` provider/routing config.
- Add config/model inspection commands.
- Add Inference Mask, expansion task, and verification artifact contract docs.
- Add template-only mask commands for existing runs.
- Keep deterministic and local.
- Do not call OpenAI, Claude, Groq, NVIDIA, Ollama, Gemma, Kimi, Qwen, or any model provider.
- Do not add API clients, model routing execution, semantic highlight reranking, Inference Mask generation, Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 023 END -->

## Handoff 023

<!-- HANDOFF 023 START -->
What changed:
- Added disabled BYOM model configuration skeleton to default config content and local `byom-video.yaml`.
- Added model config structs and parsing for `models.enabled`, `models.providers`, and `models.routing`.
- Added `config show` with readable and JSON output.
- Added `models` with readable and JSON output.
- Added `mask-template <run_id>` to write Inference Mask template contracts into a run directory.
- Added `inspect-mask <run_id>` to inspect mask-related artifacts/templates.
- Added Inference Mask, expansion task, and verification artifact contract docs.
- Updated architecture, config, artifact, and README docs.
- Added smoke script for model config and mask templates.
- Added tests for model config parsing, secret redaction behavior, model command output, mask template writing, and mask inspection.

Files added/modified:
- Added `internal/commands/config.go`.
- Added `internal/commands/config_test.go`.
- Added `internal/commands/mask.go`.
- Added `internal/commands/mask_test.go`.
- Added `docs/models.md`.
- Added `docs/artifacts/inference-mask.md`.
- Added `docs/artifacts/expansion-tasks.md`.
- Added `docs/artifacts/verification.md`.
- Added `scripts/smoke-model-mask.sh`.
- Modified `internal/config/config.go`.
- Modified `internal/config/config_test.go`.
- Modified `internal/cli/root.go`.
- Modified `docs/config.md`.
- Modified `docs/architecture/inference-mask.md`.
- Modified `docs/artifacts/README.md`.
- Modified `README.md`.
- Modified `byom-video.yaml`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video config show
./byom-video config show --json
./byom-video models
./byom-video models --json
./byom-video mask-template <run_id>
./byom-video inspect-mask <run_id>
./byom-video inspect-mask <run_id> --json
```

New config fields:
```yaml
models:
  enabled: false
  providers:
    premium_reasoner:
      provider: openai
      model: gpt-4.1
      api_key_env: OPENAI_API_KEY
    free_expander:
      provider: nvidia
      model: placeholder-nvidia-model
      api_key_env: NVIDIA_API_KEY
    local_expander:
      provider: ollama
      model: qwen2.5:7b
      base_url: http://localhost:11434
  routing:
    highlight_reasoning: premium_reasoner
    caption_expansion: local_expander
    timeline_labeling: local_expander
    verification: premium_reasoner
```

BYOM model config behavior:
- Model config is optional and disabled by default.
- Missing `models` config keeps existing behavior unchanged.
- Config parsing supports logical provider names, provider type, model, `api_key_env`, optional `base_url`, and routing keys.
- `config show` displays pipeline defaults and model config summary.
- `models` displays only model config and says models are disabled when disabled or absent.
- Commands print environment variable names only and never print environment variable values.
- No API keys are validated and no provider connectivity is tested.
- No provider SDKs, API clients, or model routing execution were added.

Inference Mask template behavior:
- `mask-template <run_id>` writes:
```text
.byom-video/runs/<run_id>/inference_mask.template.json
.byom-video/runs/<run_id>/expansion_tasks.template.json
.byom-video/runs/<run_id>/verification.template.json
```
- Templates use existing `chunks.json`, `highlights.json`, and `roughcut.json` source fields when present.
- Templates are not added to `manifest.json` because they are contracts/templates, not generated inference artifacts.
- `inspect-mask <run_id>` reports present/missing mask artifacts and templates.
- No normal run generates `inference_mask.json`.
- No model provider is called.

Commands run:
```sh
gofmt -w internal/config/config.go internal/config/config_test.go internal/commands/config.go internal/commands/config_test.go internal/commands/mask.go internal/commands/mask_test.go internal/cli/root.go
chmod +x scripts/smoke-model-mask.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-model-mask.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-model-mask.sh` passed.
- Smoke showed config/model summaries, wrote mask templates for latest local run, and inspected present/missing mask artifacts.

How to run smoke model/mask test:
```sh
scripts/smoke-model-mask.sh
```

Known limitations:
- Model config is parsed and displayed only.
- No provider clients, API calls, connectivity checks, or API key validation exist.
- Inference Mask artifacts are design contracts only.
- `mask-template` writes templates but does not generate real inference decisions.
- `inspect-mask` checks artifact/template existence only and does not deeply validate schemas.
- No semantic highlight reranking, model routing execution, Inference Mask implementation, Docker, vector DB, web server, or NLE integrations were added.

Next recommended milestone:
- Add schema validation for mask template files.
- Add deterministic mask planning/review commands before any model execution.
- Add provider-neutral model routing interfaces as dry-run contracts only.
- Continue Inference Mask design before adding any provider integration.

Errors or assumptions:
- Assumed disabled model config should still be visible in `config show` for review, while `models` can simply report disabled.
- Assumed template files should not be added to run manifests.
- Assumed current template timestamps are acceptable because these are generated local templates.
- Assumed `api_key_env` names are safe to display but environment variable values must never be read or printed.
<!-- HANDOFF 023 END -->

## Prompt 024 - Dynamic Model Config + Mask Validation

<!-- PROMPT 024 START -->
Goal:
- Generalize BYOM model configuration so providers are dynamic and user-defined rather than hardcoded.
- Prefer `models.entries` and `models.routes`.
- Preserve backward compatibility with Prompt 023 `models.providers` and `models.routing`.
- Add structural model config validation with `models validate`.
- Add Inference Mask template/artifact validation with `mask-validate`.
- Keep deterministic and local.
- Do not call any model provider, add provider SDKs, add model routing execution, implement semantic highlight reranking, implement real Inference Mask generation, add Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 024 END -->

## Handoff 024

<!-- HANDOFF 024 START -->
What changed:
- Reworked model config normalization around provider-neutral `entries` and `routes`.
- Kept backward compatibility for old `providers` and `routing` config keys.
- Added freeform per-entry `options` parsing.
- Added optional `role` parsing for `reasoner`, `expander`, `verifier`, and `general`.
- Added structural model config validation with `byom-video models validate`.
- Added Inference Mask template/schema validation with `byom-video mask-validate <run_id>`.
- Added provider-neutral example config files.
- Updated default config content and local `byom-video.yaml` to use `entries` and `routes`.
- Updated model, config, artifact, architecture, and README docs.
- Added dynamic model/mask smoke script.
- Added tests for new config shape, old shape compatibility, new-shape precedence, unknown providers, options parsing, model validation, and mask validation.

Files added/modified:
- Added `examples/configs/local-only.yaml`.
- Added `examples/configs/openai-ollama.yaml`.
- Added `examples/configs/groq-local.yaml`.
- Added `examples/configs/nvidia-expander.yaml`.
- Added `examples/configs/custom-http.yaml`.
- Added `scripts/smoke-dynamic-models.sh`.
- Modified `internal/config/config.go`.
- Modified `internal/config/config_test.go`.
- Modified `internal/commands/config.go`.
- Modified `internal/commands/config_test.go`.
- Modified `internal/commands/mask.go`.
- Modified `internal/commands/mask_test.go`.
- Modified `internal/cli/root.go`.
- Modified `docs/models.md`.
- Modified `docs/config.md`.
- Modified `docs/architecture/inference-mask.md`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/artifacts/inference-mask.md`.
- Modified `docs/artifacts/verification.md`.
- Modified `README.md`.
- Modified `byom-video.yaml`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video models validate
./byom-video models validate --json
./byom-video mask-validate <run_id>
./byom-video mask-validate <run_id> --json
```

New config shape:
```yaml
models:
  enabled: false
  entries:
    local_qwen:
      provider: ollama
      model: qwen2.5:7b
      role: expander
      base_url: http://localhost:11434
      options:
        temperature: 0.2
    premium_reasoner:
      provider: openai
      model: gpt-4.1
      role: reasoner
      api_key_env: OPENAI_API_KEY
      options:
        temperature: 0.1
        max_tokens: 1200
  routes:
    highlight_reasoning: premium_reasoner
    caption_expansion: local_qwen
    timeline_labeling: local_qwen
    verification: premium_reasoner
```

Backward compatibility behavior:
- Old `models.providers` is still accepted.
- Old `models.routing` is still accepted.
- Normalized output uses `entries` and `routes`.
- If both old and new shapes exist, `entries` and `routes` win.

Model validation behavior:
- Validates structure only.
- Allows any non-empty provider string.
- Requires each entry to have non-empty `provider` and `model`.
- Allows optional `api_key_env` and `base_url`.
- Allows freeform `options`.
- Allows optional `role`; valid roles are `reasoner`, `expander`, `verifier`, and `general`.
- Requires each route target to reference an existing entry.
- Does not read API key values, test connectivity, or call providers.

Mask validation behavior:
- `mask-validate` checks `inference_mask.template.json` or `inference_mask.json`.
- Checks `expansion_tasks.template.json` or `expansion_tasks.json`.
- Checks `verification.template.json` or `verification.json`.
- Validates schema version and required top-level fields only.
- Reports missing templates/artifacts cleanly.
- Does not call any model provider.

Commands run:
```sh
gofmt -w internal/config/config.go internal/config/config_test.go internal/commands/config.go internal/commands/config_test.go internal/commands/mask.go internal/commands/mask_test.go internal/cli/root.go
chmod +x scripts/smoke-dynamic-models.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-dynamic-models.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-dynamic-models.sh` passed.
- Smoke validated model config, showed config, wrote mask templates for the latest local run, and validated the mask templates.

How to run smoke dynamic-models test:
```sh
scripts/smoke-dynamic-models.sh
```

Known limitations:
- Model config remains parse/display/validate only.
- No provider clients, SDKs, API calls, connectivity checks, or API key validation exist.
- `mask-validate` validates only top-level template/schema shape.
- Inference Mask generation and semantic reranking are still not implemented.
- No Docker, vector DB, web server, or NLE integrations were added.

Next recommended milestone:
- Add deterministic mask planning/review commands before any provider integration.
- Add provider-neutral dry-run routing plans.
- Add deeper schema validation for individual mask decisions/tasks/checks.
- Continue Inference Mask design before adding any model execution.

Errors or assumptions:
- Assumed old `providers/routing` should remain accepted but not preferred.
- Assumed unknown provider strings should be accepted if non-empty.
- Assumed `models validate` should return non-zero when structural errors are found.
- Assumed `mask-validate` should return non-zero when required templates/artifacts are missing or malformed.
<!-- HANDOFF 024 END -->

## Prompt 025 - Deterministic Inference Mask Planning

<!-- PROMPT 025 START -->
Goal:
- Add deterministic Inference Mask planning and review commands before any model execution.
- Generate `inference_mask.json` from existing run artifacts.
- Add mask review, expansion task planning, and verification planning artifacts.
- Improve mask inspection and validation for real generated artifacts.
- Keep deterministic and local.
- Do not call any model provider, add provider SDKs, add model routing execution, implement semantic reranking, generate model captions/descriptions, add Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 025 END -->

## Handoff 025

<!-- HANDOFF 025 START -->
What changed:
- Added deterministic `mask-plan <run_id>`.
- Added generated `inference_mask.json` artifacts from `roughcut.json` or `highlights.json`.
- Added mask planning events.
- Added manifest recording for generated mask artifacts.
- Extended `mask-validate` to validate generated masks, expansion plans, and verification plans.
- Added `review-mask <run_id>` with readable, JSON, and markdown artifact output.
- Added `expansion-plan <run_id>` for deterministic expansion task planning.
- Added `verification-plan <run_id>` for deterministic pending verification checks.
- Improved `inspect-mask` to show real artifacts, templates, review artifacts, and validation status when available.
- Updated Inference Mask, expansion task, verification, architecture, model, artifact, and README docs.
- Added smoke script for deterministic mask planning.
- Added tests for roughcut/highlight mask planning, overwrite refusal, numeric validation, bad decision timing, review artifacts, expansion tasks, verification checks, inspect-mask output, and manifest recording.

Files added/modified:
- Added `scripts/smoke-mask-plan.sh`.
- Modified `internal/commands/mask.go`.
- Modified `internal/commands/mask_test.go`.
- Modified `internal/cli/root.go`.
- Modified `docs/architecture/inference-mask.md`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/artifacts/inference-mask.md`.
- Modified `docs/artifacts/expansion-tasks.md`.
- Modified `docs/artifacts/verification.md`.
- Modified `docs/models.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video mask-plan <run_id>
./byom-video review-mask <run_id>
./byom-video expansion-plan <run_id>
./byom-video verification-plan <run_id>
```

New flags:
```sh
./byom-video mask-plan <run_id> --intent <text>
./byom-video mask-plan <run_id> --tone <text>
./byom-video mask-plan <run_id> --max-caption-words <n>
./byom-video mask-plan <run_id> --top-k <n>
./byom-video mask-plan <run_id> --overwrite
./byom-video review-mask <run_id> --json
./byom-video review-mask <run_id> --write-artifact
./byom-video expansion-plan <run_id> --overwrite
./byom-video expansion-plan <run_id> --caption-variants <n>
./byom-video expansion-plan <run_id> --label-max-words <n>
./byom-video expansion-plan <run_id> --description-max-words <n>
./byom-video verification-plan <run_id> --overwrite
```

Mask planning behavior:
- Resolves runs safely under `.byom-video/runs`.
- Prefers `roughcut.json` and creates one `keep` decision per roughcut clip.
- Falls back to `highlights.json` and creates `candidate_keep` decisions.
- Fails cleanly if neither roughcut nor highlights are available.
- Writes `inference_mask.json` with `schema_version: inference_mask.v1`.
- Includes source artifact fields, deterministic mode, `deterministic_mask_planner_v1`, intent, constraints, decisions, timing, ids, reason, and text preview.
- Refuses to overwrite existing `inference_mask.json` unless `--overwrite` is passed.
- Writes `MASK_PLAN_STARTED`, `MASK_PLAN_COMPLETED`, and `MASK_PLAN_FAILED`.
- Records `inference_mask.json` in the manifest on success.

Mask review behavior:
- `review-mask` prints intent, source artifacts, constraints, decision count, and decisions.
- `review-mask --json` emits machine-readable review.
- `review-mask --write-artifact` writes `mask_review.md`.
- `mask_review.md` is recorded in the manifest.

Expansion planning behavior:
- `expansion-plan` requires `inference_mask.json`.
- Writes `expansion_tasks.json`.
- Creates deterministic tasks for `caption_variants`, `timeline_labels`, and `short_descriptions`.
- Uses route names `caption_expansion`, `timeline_labeling`, and `description_expansion` when configured, otherwise caption expansion for descriptions.
- Records `expansion_tasks.json` in the manifest.
- Writes `EXPANSION_PLAN_STARTED`, `EXPANSION_PLAN_COMPLETED`, and `EXPANSION_PLAN_FAILED`.

Verification planning behavior:
- `verification-plan` requires `inference_mask.json`.
- Optionally references `expansion_tasks.json` if present.
- Writes `verification.json` with pending checks for `must_not_include`, `timestamp_drift`, `missing_required_decisions`, and `output_contract_compliance`.
- Records `verification.json` in the manifest.
- Writes `VERIFICATION_PLAN_STARTED`, `VERIFICATION_PLAN_COMPLETED`, and `VERIFICATION_PLAN_FAILED`.

Commands run:
```sh
gofmt -w internal/commands/mask.go internal/commands/mask_test.go internal/cli/root.go
chmod +x scripts/smoke-mask-plan.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-mask-plan.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-mask-plan.sh` passed.
- Smoke generated `inference_mask.json`, validated it, wrote `mask_review.md`, wrote `expansion_tasks.json`, wrote `verification.json`, and inspected mask artifacts.

How to run smoke mask-plan test:
```sh
scripts/smoke-mask-plan.sh
```

Known limitations:
- Mask planning is deterministic and heuristic only.
- No provider clients, SDKs, API calls, connectivity checks, API key validation, or model routing execution exist.
- No semantic reranking or model-generated captions/descriptions are implemented.
- `mask-validate` performs structural validation, not semantic verification.
- Existing expansion and verification artifacts are plans only; no expansion/verifier execution exists.
- No Docker, vector DB, web server, or NLE integrations were added.

Next recommended milestone:
- Add deterministic mask revision commands.
- Add deeper schema validation for individual expansion task output contracts.
- Add provider-neutral dry-run routing plans that explain which route would be used without executing it.
- Continue Inference Mask review workflow before adding provider execution.

Errors or assumptions:
- Assumed roughcut clips are stronger source decisions than raw highlights.
- Assumed `mask-validate` should allow missing expansion/verification plans after mask planning, while still failing if the inference mask is missing or malformed.
- Assumed generated `mask_review.md` should be recorded in the manifest as a generated review artifact.
- Assumed standalone mask planning failures should write events but not mark the whole run failed.
<!-- HANDOFF 025 END -->

## Prompt 026 - Routing Dry Run + Mask Revision

<!-- PROMPT 026 START -->
Goal:
- Add provider-neutral routing dry-run command that explains which configured model route would be used for expansion/verification tasks.
- Add deterministic mask revision commands before any provider execution.
- Add mask snapshot list/inspect and mask diff commands.
- Keep deterministic and local.
- Do not call any model provider, add provider SDKs, add real model routing execution, generate model captions/descriptions, implement semantic reranking, add Docker, vector DB, web server, or NLE integrations.

Part A: routes-plan <run_id> [--json] [--write-artifact] [--strict]
Part B: revise-mask <run_id> --request <text> [--dry-run] [--json] [--show-diff]
Part C: mask-snapshots <run_id> [--json] / inspect-mask-snapshot <run_id> <snapshot_id> [--json]
Part D: diff-mask <run_id> <snapshot_id> [--json] [--write-artifact]
Part E: Docs (inference-mask.md, models.md, README.md)
Part F: Tests (routes-plan, revise-mask, snapshots, diff-mask)
Part G: scripts/smoke-routes-mask-revision.sh
Part H: PROGRESS.md
<!-- PROMPT 026 END -->

## Handoff 026

<!-- HANDOFF 026 START -->
What changed:
- Added deterministic `routes-plan <run_id>` with `--json`, `--write-artifact`, `--strict`.
- Added `revise-mask <run_id> --request <text>` with `--dry-run`, `--json`, `--show-diff`.
- Added `mask-snapshots <run_id>` and `inspect-mask-snapshot <run_id> <snapshot_id>` with `--json`.
- Added `diff-mask <run_id> <snapshot_id>` with `--json`, `--write-artifact`.
- Added `routes_plan.json` artifact generation and manifest recording.
- Added `mask_snapshots/mask_snapshot_NNNN.json` snapshot creation on every real `revise-mask`.
- Added `mask_diffs/diff_current_vs_<snapshot_id>.md` artifact on `diff-mask --write-artifact`.
- Added events: `ROUTES_PLAN_STARTED`, `ROUTES_PLAN_COMPLETED`, `ROUTES_PLAN_FAILED`, `MASK_REVISED`, `MASK_REVISION_FAILED`.
- Updated usage string, docs (inference-mask.md, models.md, README.md).
- Added 22 new tests in `mask_revision_test.go`.

Files added/modified:
- Added `internal/commands/mask_routes.go`.
- Added `internal/commands/mask_revision.go`.
- Added `internal/commands/mask_revision_test.go`.
- Added `scripts/smoke-routes-mask-revision.sh`.
- Modified `internal/cli/root.go`.
- Modified `docs/architecture/inference-mask.md`.
- Modified `docs/models.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video routes-plan <run_id>
./byom-video revise-mask <run_id> --request <text>
./byom-video mask-snapshots <run_id>
./byom-video inspect-mask-snapshot <run_id> <snapshot_id>
./byom-video diff-mask <run_id> <snapshot_id>
```

New flags:
```sh
./byom-video routes-plan <run_id> --json
./byom-video routes-plan <run_id> --write-artifact
./byom-video routes-plan <run_id> --strict
./byom-video revise-mask <run_id> --request <text> --dry-run
./byom-video revise-mask <run_id> --request <text> --json
./byom-video revise-mask <run_id> --request <text> --show-diff
./byom-video mask-snapshots <run_id> --json
./byom-video inspect-mask-snapshot <run_id> <snapshot_id> --json
./byom-video diff-mask <run_id> <snapshot_id> --json
./byom-video diff-mask <run_id> <snapshot_id> --write-artifact
```

Routes dry-run behavior:
- Reads `byom-video.yaml` model config and `expansion_tasks.json` / `verification.json` from the run dir.
- Does not call any provider. No connectivity checks.
- Resolves each task's `model_route` to a config entry and prints provider, model, role, status.
- Status values: `configured` (enabled + found), `models_disabled` (found but enabled:false), `missing_route` (not in routes), `missing_entry` (route points to unknown entry).
- `--strict`: exits non-zero if any route is `missing_route` or `missing_entry`.
- `--write-artifact`: writes `routes_plan.json` (schema `routes_plan.v1`) and records it in manifest.
- Warns (not fails) if no expansion_tasks.json or verification.json found.

Mask revision behavior:
- Loads `inference_mask.json`, creates a deep copy, applies deterministic revision, snapshots original before write.
- `--dry-run`: prints proposed changes, creates no snapshot, does not mutate mask.
- `--show-diff`: prints field-level diff from snapshot to revised mask.
- Snapshots stored at `mask_snapshots/mask_snapshot_NNNN.json` (auto-incremented).
- Supported requests: make captions shorter, make captions longer, set captions to N words, make tone more technical, make tone more casual, avoid hype, avoid unsupported claims, require hook.
- Unknown requests return clean error with no mutation.

Mask snapshot/diff behavior:
- `mask-snapshots` lists all `mask_snapshot_NNNN.json` files in `mask_snapshots/` dir.
- `inspect-mask-snapshot` reads and displays (or emits raw JSON with `--json`) a snapshot file.
- `diff-mask` compares current `inference_mask.json` to a snapshot, reports changed fields.
- `diff-mask --write-artifact` writes `mask_diffs/diff_current_vs_<snapshot_id>.md`.

Commands run:
```sh
gofmt -w internal/commands/mask_routes.go internal/commands/mask_revision.go internal/commands/mask_revision_test.go internal/cli/root.go
chmod +x scripts/smoke-routes-mask-revision.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-routes-mask-revision.sh
```

Test results:
- `go test ./...` passed (all packages).
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-routes-mask-revision.sh` passed.
- Smoke ran routes-plan (4 routes, all models_disabled), revise-mask (18→12 words, snapshot created), mask-snapshots (listed mask_snapshot_0001), diff-mask (showed constraint change, wrote artifact), inspect-mask.

How to run smoke routes/mask-revision test:
```sh
scripts/smoke-routes-mask-revision.sh
```

Known limitations:
- `routes-plan` only reads expansion_tasks.json and verification.json; does not scan for other task sources.
- Mask revision is deterministic string-matching on the request; no fuzzy or synonym matching.
- `mask-snapshots` shows mod-time as created_at (filesystem-based, not embedded in snapshot).
- Revision idempotency: repeating "make tone more technical" when tone already contains "technical" produces no changes (silent no-op, not an error).
- No provider calls, SDKs, API keys, connectivity checks, semantic reranking, Docker, vector DB, web server, or NLE integrations exist.

Next recommended milestone:
- Add deterministic mask revision commands for decisions (accept/reject/reorder specific decisions).
- Add provider-neutral dry-run execution plan that shows exact CLI args each route would receive.
- Add deeper schema validation for routes_plan.json.
- Continue Inference Mask review workflow before adding provider execution.

Errors or assumptions:
- Assumed `routes-plan` should read both expansion_tasks.json and verification.json as route sources, not only expansion tasks.
- Assumed `revise-mask` should snapshot before every non-dry-run mutation (not only on change).
- Assumed `diff-mask` compares snapshot→current (snapshot as "from"), so changes show what was added since snapshot.
- Assumed `mask-snapshots` should silently return empty list / no-snapshots message if directory doesn't exist (not an error).
<!-- HANDOFF 026 END -->

## Prompt 027 - Decision-Level Mask Editing and Route Execution Previews

<!-- PROMPT 027 START -->
Add decision-level Inference Mask editing and provider-neutral route execution previews without calling any model providers.

New commands:
- `mask-decisions <run_id> [--json]` — list all decisions in inference_mask.json
- `mask-decision <run_id> <decision_id> --set <keep|reject|candidate_keep> [--reason <text>] [--dry-run] [--json]` — update a single decision value
- `mask-remove-decision <run_id> <decision_id> [--dry-run] [--json]` — remove a decision by ID
- `mask-reorder <run_id> --order <decision_id,...> [--dry-run] [--json]` — reorder all decisions
- `route-preview <run_id> [--json] [--write-artifact]` — build logical payload previews per expansion task, no provider call
<!-- PROMPT 027 END -->

<!-- HANDOFF 027 START -->
Files created/modified:
- `internal/commands/mask_decisions.go` (new): MaskDecisionsList, MaskDecisionCommand, MaskRemoveDecisionCommand, MaskReorderCommand, RoutePreviewCommand, validateProposedMask, buildRoutePreviewTask, buildPayloadPreview, splitAndTrim, joinOr
- `internal/commands/mask_decisions_test.go` (new): 25 tests covering all new commands
- `internal/cli/root.go` (modified): usage string, 5 switch cases, 5 parse functions
- `scripts/smoke-mask-decision-route-preview.sh` (new): smoke script for new commands
- `docs/artifacts/inference-mask.md` (modified): decision values, decision-editing commands section
- `docs/architecture/inference-mask.md` (modified): decision-level editing and route-preview sections, commands table
- `docs/models.md` (modified): route-preview commands and description
- `README.md` (modified): decision-level mask editing examples section

Key implementation details:
- `validateProposedMask`: in-memory check before write — validates schema_version, intent, each decision value against `validDecisionValues` map, and end >= start.
- `MaskDecisionCommand`: validates --set value in validDecisionValues, finds decision index, runs validateProposedMask, snapshots original, writes updated mask, logs MASK_DECISION_UPDATED. Appends "Manual note: " + reason to existing reason if --reason provided.
- `MaskRemoveDecisionCommand`: snapshots, removes from slice, logs MASK_DECISION_REMOVED.
- `MaskReorderCommand`: validates all IDs present exactly once, snapshots, writes reordered, logs MASK_DECISIONS_REORDERED.
- `RoutePreviewCommand`: requires expansion_tasks.json, reads optional config, builds RoutePreviewTask per task with InputDecisions (resolved from decision map), PayloadPreview (instruction text + "preview_only" schema), logs ROUTE_PREVIEW_STARTED/COMPLETED. Writes route_preview.json (schema route_preview.v1) when --write-artifact.
- All decision-editing commands snapshot before real mutations (not before dry-run).
- Deep copy for proposed mask via JSON round-trip before validateProposedMask to avoid side effects on validation failure.

New types in mask_decisions.go:
- MaskDecisionOptions, MaskDecisionResult
- MaskRemoveDecisionOptions, MaskRemoveDecisionResult
- MaskReorderOptions, MaskReorderResult
- MaskDecisionsOptions
- RoutePreviewOptions, RoutePreviewTask, DecisionPreview, PayloadPreview, RoutePreview

Test helpers reused from mask_revision_test.go (same package):
- writeTestConfig(t, content)
- testConfigWithRoutes constant
- containsString(slice, value)

Commands run:
```sh
gofmt -l internal/commands/mask_decisions.go internal/commands/mask_revision.go internal/commands/mask_routes.go internal/cli/root.go
go test ./...
go build ./cmd/byom-video
chmod +x scripts/smoke-mask-decision-route-preview.sh
```

Test results:
- `gofmt -l` produced no output (all files already formatted).
- `go test ./...` passed (all 25 packages).
- `go build ./cmd/byom-video` passed.

Known limitations:
- `mask-reorder` requires all existing decision IDs to be listed; partial reorders are not supported.
- PayloadPreview instruction is a template string, not a provider-specific format.
- `route-preview` only reads expansion_tasks.json; does not scan verification.json for additional tasks.
- No provider calls, SDKs, API keys, connectivity checks, semantic reranking, Docker, vector DB, web server, or NLE integrations exist.

Next recommended milestone:
- Add `mask-accept-all`/`mask-reject-all` bulk decision commands.
- Add `route-preview --filter <task_type>` to scope previews to a specific task type.
- Extend `validateProposedMask` to check for duplicate decision IDs.
- Continue towards actual provider execution (BYOM model routing) once mask workflow is stable.

How to run smoke test:
```sh
scripts/smoke-mask-decision-route-preview.sh
```
<!-- HANDOFF 027 END -->

## Prompt 028 - Stub Expansion Execution

<!-- PROMPT 028 START -->
Add provider-free stub expansion execution for Inference Mask expansion tasks. Prove the expansion pipeline end-to-end without calling any real model provider: read expansion_tasks.json and inference_mask.json, generate deterministic stub expansion artifacts, validate them, review them. Prepare for future provider-backed execution.

New commands:
- `expand-stub <run_id> [--overwrite] [--json] [--task-type <type>]`
- `expansion-validate <run_id> [--json]`
- `review-expansions <run_id> [--json] [--write-artifact]`

`inspect-mask` now also reports expansion output files under `expansions/`.
<!-- PROMPT 028 END -->

<!-- HANDOFF 028 START -->
Files created/modified:
- `internal/commands/expansion_stub.go` (new): ExpandStub, ExpansionValidate, ReviewExpansions, buildStubOutput, validateExpansionOutput, writeExpansionReviewMarkdown, contractInt, firstNWords
- `internal/commands/expansion_stub_test.go` (new): 22 tests
- `internal/commands/mask.go` (modified): maskArtifactNames extended with 4 expansion-related paths
- `internal/cli/root.go` (modified): usage string, 3 switch cases, 3 parse functions (parseExpandStubArgs, parseExpansionValidateArgs, parseReviewExpansionsArgs)
- `scripts/smoke-expand-stub.sh` (new)
- `docs/artifacts/expansions.md` (new)
- `docs/architecture/inference-mask.md` (modified): Stub Expansion Execution section, updated Non-Goals
- `README.md` (modified): stub expansion examples section

New types:
- ExpansionOutputItem, ExpansionOutputSource, ExpansionOutput (schema: expansion_output.v1)
- ExpandStubOptions, ExpandStubSummary
- ExpansionValidateOptions, ExpansionFileValidation, ExpansionValidationResult
- ReviewExpansionsOptions, ExpansionReview, ExpansionFileReview

Stub expansion behavior:
- Reads inference_mask.json + expansion_tasks.json
- Groups tasks by type (caption_variants, timeline_labels, short_descriptions)
- Filtered by --task-type when set; errors if type not found
- Skips decisions with decision=="reject"; includes keep and candidate_keep
- caption_variants: up to output_contract.max_items variants per decision, text capped to max_words
- timeline_labels: one label per decision, text from first N words of text_preview/reason
- short_descriptions: one description per decision with timing and reason
- Writes expansions/caption_variants.json, expansions/timeline_labels.json, expansions/short_descriptions.json
- Records each in run manifest; refuses overwrite without --overwrite
- Events: EXPAND_STUB_STARTED, EXPAND_STUB_COMPLETED, EXPAND_STUB_FAILED
- If all decisions rejected: produces empty items + warning

Expansion validation behavior:
- Checks all three known task-type files under expansions/
- Validates schema_version==expansion_output.v1, created_at, mode, task_type, source fields, items shape
- Each item must have id, task_id, decision_id, text; end>=start if both present
- Cross-checks against inference_mask.json: no item may reference a rejected decision
- Missing files reported as missing (not an error by themselves); invalid files fail validation
- Events: EXPANSION_VALIDATION_STARTED, EXPANSION_VALIDATION_COMPLETED, EXPANSION_VALIDATION_FAILED

Expansion review behavior:
- Prints task_type, item count, decision IDs, text previews (up to 3 per file)
- --write-artifact writes expansions_review.md and records in manifest
- inspect-mask now lists expansions/caption_variants.json, expansions/timeline_labels.json, expansions/short_descriptions.json, expansions_review.md

Commands run:
```sh
gofmt -w internal/commands/expansion_stub.go
go test ./...
go build ./cmd/byom-video
chmod +x scripts/smoke-expand-stub.sh
```

Test results:
- `go test ./...` passed (all 25 packages; 22 new tests in expansion_stub_test.go all pass)
- `go build ./cmd/byom-video` passed

Known limitations:
- expand-stub only handles task types: caption_variants, timeline_labels, short_descriptions; unknown types fall through to a generic "Stub item" text
- Validation only checks the three known task types; expansion files for custom task types are not scanned
- Output contract fields (max_items, max_words) are read from JSON as float64 and cast to int
- Stub text is intentionally simple; it is not semantically meaningful, only structurally correct

Next recommended milestone:
- Add real provider execution (expand <run_id>) that calls configured model routes when models.enabled: true
- Add verification execution (verify <run_id>) that runs checks from verification.json against expansion outputs
- Extend expansion-validate to also validate custom task type files found in expansions/

How to run smoke test:
```sh
scripts/smoke-expand-stub.sh
```
<!-- HANDOFF 028 END -->

## Prompt 029 - Deterministic Verification Execution

<!-- PROMPT 029 START -->
Add deterministic verification execution for expansion outputs before any real model provider execution. Execute verification checks from verification.json against expansion outputs, produce verification_results.json, review results. Keep everything deterministic and local — no provider calls.

New commands:
- `verify-expansions <run_id> [--json] [--tolerance-seconds <n>]`
- `review-verification <run_id> [--json] [--write-artifact]`

`inspect-mask` and `mask-validate` now also cover verification_results.json.
<!-- PROMPT 029 END -->

<!-- HANDOFF 029 START -->
Files created/modified:
- `internal/commands/verify_expansions.go` (new): VerifyExpansions, ReviewVerification, runVerificationCheck, runMustNotInclude, runTimestampDrift, runMissingRequiredDecisions, runOutputContractCompliance, printVerificationResults, writeVerificationReviewMarkdown, validateVerificationResultsShape, abs64
- `internal/commands/verify_expansions_test.go` (new): 19 tests
- `internal/commands/mask.go` (modified): maskArtifactNames extended with verification_results.json and verification_review.md; validateMaskArtifacts extended with verification_results spec using validateVerificationResultsShape
- `internal/cli/root.go` (modified): usage string, 2 switch cases, 2 parse functions (parseVerifyExpansionsArgs, parseReviewVerificationArgs)
- `scripts/smoke-verify-expansions.sh` (new)
- `docs/artifacts/verification-results.md` (new)
- `docs/architecture/inference-mask.md` (modified): Deterministic Verification Execution section, updated Non-Goals
- `README.md` (modified): verify-expansions examples section

New types:
- VerificationResultCheck (id, type, status, message, details)
- VerificationResultSummary (checks_total, checks_passed, checks_failed, warnings)
- VerificationResultSource (inference_mask_artifact, verification_artifact, expansion_artifacts)
- VerificationResults (schema: verification_results.v1)
- VerifyExpansionsOptions, ReviewVerificationOptions

Verification behavior:
- Reads inference_mask.json + verification.json (required) + expansions/*.json (optional per type)
- Runs each check from verification.json in order
- must_not_include: case-insensitive scan of all item text against constraints.must_not_include
- timestamp_drift: compares item start/end to referenced decision timing within --tolerance-seconds (default 0.25)
- missing_required_decisions: every non-rejected decision must appear in at least one expansion item
- output_contract_compliance: word count per item and item count per decision checked against expansion_tasks.json contracts
- Unknown check types are skipped (status: skipped) rather than failing
- Always writes verification_results.json; records in manifest
- Events: VERIFICATION_STARTED, VERIFICATION_COMPLETED, VERIFICATION_FAILED
- Overall status: passed (no failures), failed (any failure), warning (only warnings)

Review-verification behavior:
- Reads verification_results.json; errors if missing
- Prints status, check totals, per-check status and message
- --write-artifact writes verification_review.md and records in manifest
- --json emits the full VerificationResults struct

inspect-mask integration:
- verification_results.json and verification_review.md now appear in maskArtifactNames
- mask-validate now validates verification_results.json shape (only fails if file exists and is malformed)

Commands run:
```sh
gofmt -w internal/commands/verify_expansions.go
go test ./...
go build ./cmd/byom-video
chmod +x scripts/smoke-verify-expansions.sh
```

Test results:
- `go test ./...` passed (all 25 packages; 19 new tests all pass; 150 total passing tests in commands package)
- `go build ./cmd/byom-video` passed

Known limitations:
- output_contract_compliance uses the first task of each type for the contract; if multiple tasks have different contracts, only the first is used
- timestamp_drift only checks items that have non-zero start/end; items with both zero are skipped
- missing_required_decisions does not distinguish between task types; a decision covered by any expansion type satisfies the check
- Verification does not re-run expansion generation; it only validates what's already on disk

Next recommended milestone:
- Add real provider expansion execution (expand <run_id>) that replaces stub with actual model output
- Add incremental re-verification after mask edits (detect which checks are affected by a decision change)
- Extend output_contract_compliance to check per-task max_items separately per task ID (not aggregated)

How to run smoke test:
```sh
scripts/smoke-verify-expansions.sh
```
<!-- HANDOFF 029 END -->

## Prompt 030 - Model Adapter Interface + Dry Run Expansion

<!-- PROMPT 030 START -->
Goal:
- Add provider-neutral model adapter interfaces and dry-run expansion execution contracts without calling real providers.
- Add `internal/modelrouter/`.
- Add `expand-dry-run <run_id>` and `expand-local-stub <run_id>`.
- Keep deterministic and local.
- Do not call real providers, add provider SDKs, read API key values, execute HTTP requests, add Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 030 END -->

<!-- HANDOFF 030 START -->
What changed:
- Added provider-neutral adapter package under `internal/modelrouter`.
- Added registry-backed `dry-run` and `stub` adapters.
- Added `expand-dry-run <run_id>` to build provider-ready request previews without calling any provider.
- Added `expand-local-stub <run_id>` to execute expansion output generation through the adapter path while keeping deterministic local stub behavior.
- Added `model_requests.dryrun.json` artifact generation and validation.
- Extended `mask-validate` and `inspect-mask` to include `model_requests.dryrun.json`.
- Added docs for the adapter layer and dry-run request artifact.
- Added smoke script for model-router dry-run flow.
- Added tests for registry behavior, dry-run request generation, strict routing failures, task-type filtering, dry-run artifact validation, local stub execution, and manifest recording.

Files added/modified:
- Added `internal/modelrouter/adapter.go`.
- Added `internal/modelrouter/registry.go`.
- Added `internal/modelrouter/request.go`.
- Added `internal/modelrouter/dryrun.go`.
- Added `internal/modelrouter/stub.go`.
- Added `internal/modelrouter/registry_test.go`.
- Added `internal/commands/model_router.go`.
- Added `internal/commands/model_router_test.go`.
- Added `docs/model-router.md`.
- Added `docs/artifacts/model-requests.md`.
- Added `scripts/smoke-model-router-dryrun.sh`.
- Modified `internal/commands/mask.go`.
- Modified `internal/cli/root.go`.
- Modified `docs/models.md`.
- Modified `docs/architecture/inference-mask.md`.
- Modified `docs/artifacts/README.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video expand-dry-run <run_id>
./byom-video expand-local-stub <run_id>
```

New flags:
```sh
./byom-video expand-dry-run <run_id> --json
./byom-video expand-dry-run <run_id> --strict
./byom-video expand-dry-run <run_id> --task-type <caption_variants|timeline_labels|short_descriptions>

./byom-video expand-local-stub <run_id> --overwrite
./byom-video expand-local-stub <run_id> --json
./byom-video expand-local-stub <run_id> --task-type <caption_variants|timeline_labels|short_descriptions>
```

Adapter interface behavior:
- `internal/modelrouter` defines a provider-neutral `Adapter` interface with `Name`, `Supports`, `BuildRequest`, and `Execute`.
- The registry registers only `dry-run` and `stub` adapters in this milestone.
- No real provider adapters exist yet.
- Unknown provider strings remain allowed in config; unresolved provider-specific adapters do not trigger any provider call.

Dry-run expansion behavior:
- `expand-dry-run` requires `inference_mask.json` and `expansion_tasks.json`.
- It resolves routes from model config, builds request previews, and always writes `.byom-video/runs/<run_id>/model_requests.dryrun.json`.
- `--strict` fails when routes or entries are missing.
- `--task-type` limits request generation to one expansion task type.
- Events: `EXPAND_DRY_RUN_STARTED`, `EXPAND_DRY_RUN_COMPLETED`, `EXPAND_DRY_RUN_FAILED`.
- Records `model_requests.dryrun.json` in the run manifest.

Local stub adapter behavior:
- `expand-local-stub` uses the adapter registry path and stub adapter, then writes normal deterministic expansion outputs under `expansions/`.
- Output schema remains `expansion_output.v1`.
- Rejected mask decisions are skipped.
- `expand-stub` remains unchanged as the direct deterministic command.
- Events: `EXPAND_LOCAL_STUB_STARTED`, `EXPAND_LOCAL_STUB_COMPLETED`, `EXPAND_LOCAL_STUB_FAILED`.

Commands run:
```sh
gofmt -w internal/modelrouter/adapter.go internal/modelrouter/registry.go internal/modelrouter/request.go internal/modelrouter/dryrun.go internal/modelrouter/stub.go internal/modelrouter/registry_test.go internal/commands/model_router.go internal/commands/model_router_test.go internal/commands/mask.go internal/cli/root.go
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
chmod +x scripts/smoke-model-router-dryrun.sh
scripts/smoke-model-router-dryrun.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-model-router-dryrun.sh` passed.
- Smoke wrote `model_requests.dryrun.json`, ran `expand-local-stub --overwrite`, validated expansions, verified them, and showed the new dry-run artifact in `inspect-mask`.

How to run smoke model-router dryrun test:
```sh
scripts/smoke-model-router-dryrun.sh
```

Known limitations:
- `dry-run` and `stub` are the only adapters registered.
- No provider-specific request translation exists beyond generic request previews.
- `expand-dry-run` writes request previews, not provider payloads for any real SDK.
- `expand-local-stub` still relies on deterministic local stub text generation; it does not simulate provider reasoning.
- `mask-validate` validates `model_requests.dryrun.json` structurally only.
- No provider SDKs, API calls, HTTP execution, Docker, vector DB, web server, or NLE integrations were added.

Next recommended milestone:
- Add provider-specific request shapers that still run in dry-run mode only.
- Add a real `expand <run_id>` execution path once provider adapters exist.
- Add deeper validation for request preview schema fields and per-task payload contracts.
- Add review commands for `model_requests.dryrun.json` before any provider execution.

Errors or assumptions:
- Chose to always write `model_requests.dryrun.json` because dry-run artifact generation is the command’s purpose.
- Chose to select dry-run and stub adapters by adapter name, not by config provider string, so unknown providers remain inert.
- Kept `expand-stub` unchanged and introduced `expand-local-stub` as the adapter-path proof command.
<!-- HANDOFF 030 END -->

## Prompt 031 - Ollama Adapter v1

<!-- PROMPT 031 START -->
Goal:
- Add the first real provider adapter: Ollama local HTTP adapter v1.
- Add a real `expand <run_id>` command.
- Add `models doctor` for explicit local Ollama connectivity checks.
- Keep deterministic/stub paths intact.
- Do not add cloud providers, API key reads, provider SDKs, Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 031 END -->

<!-- HANDOFF 031 START -->
What changed:
- Added a real local-only Ollama adapter to `internal/modelrouter`.
- Registered the Ollama adapter alongside existing `dry-run` and `stub` adapters.
- Added the real `expand <run_id>` command with provider-backed execution.
- Kept `expand-dry-run`, `expand-local-stub`, and `expand-stub` intact.
- Added explicit `models doctor` for local Ollama availability checks.
- Updated dry-run prompt previews to be task-specific and more conservative.
- Updated the local-only example config to show an enabled Ollama route setup.
- Added dry-run and optional real Ollama smoke scripts.
- Added tests for Ollama adapter behavior, provider-backed expand behavior, and models doctor failure handling.

Files added/modified:
- Added `internal/modelrouter/ollama.go`.
- Added `internal/modelrouter/ollama_test.go`.
- Modified `internal/modelrouter/request.go`.
- Modified `internal/modelrouter/registry.go`.
- Modified `internal/modelrouter/dryrun.go`.
- Modified `internal/commands/model_router.go`.
- Modified `internal/commands/model_router_test.go`.
- Modified `internal/commands/config.go`.
- Modified `internal/cli/root.go`.
- Modified `examples/configs/local-only.yaml`.
- Added `scripts/smoke-ollama-dryrun.sh`.
- Added `scripts/smoke-ollama-real.sh`.
- Added `docs/artifacts/model-router.md`.
- Modified `docs/model-router.md`.
- Modified `docs/models.md`.
- Modified `docs/architecture/inference-mask.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video expand <run_id>
./byom-video models doctor
```

New flags:
```sh
./byom-video expand <run_id> --overwrite
./byom-video expand <run_id> --json
./byom-video expand <run_id> --task-type <caption_variants|timeline_labels|short_descriptions>
./byom-video expand <run_id> --strict
./byom-video expand <run_id> --dry-run
./byom-video expand <run_id> --max-tasks <n>

./byom-video models doctor --json
```

Ollama adapter behavior:
- Supports provider strings `ollama` and `ollama-local`.
- Uses local HTTP only.
- Default base URL is `http://localhost:11434`.
- Uses `POST /api/generate` with `stream: false`.
- Sends `model`, `prompt`, and optional freeform `options`.
- Does not read API key env values.
- Returns a clean error when Ollama is unavailable:
  `Ollama request failed. Is Ollama running at <base_url>?`

Expand command behavior:
- Requires `models.enabled: true`.
- Requires `inference_mask.json` and `expansion_tasks.json`.
- Resolves routes through the modelrouter registry.
- `--dry-run` reuses the dry-run generation path and does not call Ollama.
- Real provider execution currently supports only Ollama.
- Writes normal `expansion_output.v1` artifacts under `expansions/`.
- Falls back to storing plain text safely when the provider response is not structured JSON.
- Respects `--overwrite`, `--task-type`, `--strict`, and `--max-tasks`.
- Writes `EXPAND_STARTED`, `EXPAND_COMPLETED`, and `EXPAND_FAILED`.

Models doctor behavior:
- Explicit command only.
- Checks configured local Ollama entries when `models.enabled: true`.
- Uses `/api/tags` for availability checks.
- Does not check cloud providers.
- Does not run during normal `doctor`.

Commands run:
```sh
gofmt -w internal/modelrouter/request.go internal/modelrouter/registry.go internal/modelrouter/dryrun.go internal/modelrouter/ollama.go internal/modelrouter/ollama_test.go internal/commands/config.go internal/commands/model_router.go internal/commands/model_router_test.go internal/cli/root.go
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
chmod +x scripts/smoke-ollama-dryrun.sh scripts/smoke-ollama-real.sh
scripts/smoke-ollama-dryrun.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-ollama-dryrun.sh` passed.
- Dry-run smoke ran `expand-dry-run` and `expand --dry-run` with the local-only example config and printed optional real Ollama instructions.

How to run dry-run smoke test:
```sh
scripts/smoke-ollama-dryrun.sh
```

How to run optional real Ollama smoke test:
```sh
scripts/smoke-ollama-real.sh
```

Known limitations:
- Real provider execution supports only local Ollama.
- The adapter uses a simple generate call and non-streaming response handling only.
- Provider response parsing is intentionally conservative and falls back to plain text.
- `expand` calls Ollama per decision for predictable artifact shaping; it is not yet batched or optimized.
- No cloud providers, API key reads, SDKs, Docker, vector DB, web server, or NLE integrations were added.

Next recommended milestone:
- Add richer structured response parsing for Ollama JSON outputs.
- Add batched provider request execution per task when output contracts can be preserved safely.
- Add provider-specific dry-run request shapers beyond the generic preview.
- Add review commands for provider request artifacts before execution.

Errors or assumptions:
- Chose local Ollama only for the first real adapter.
- Kept `expand --dry-run` on the expand command path but still implemented it through the existing dry-run artifact builder.
- Chose per-decision provider calls to keep `expansion_output.v1` stable and make plain-text fallback predictable.
<!-- HANDOFF 031 END -->

## Prompt 032 - Ollama Structured Output + Model Request Review

<!-- PROMPT 032 START -->
Goal:
- Harden Ollama expansion output handling and add provider request review artifacts.
- Improve structured JSON parsing for Ollama responses.
- Add `review-model-requests <run_id>`.
- Add executed request logging, partial failure handling, and safer prompt templates.
- Keep local-first and Ollama-only for real provider execution.
<!-- PROMPT 032 END -->

<!-- HANDOFF 032 START -->
What changed:
- Improved Ollama response parsing with structured JSON handling for `items`, `captions`, `labels`, and `descriptions`.
- Added safe plain-text fallback and JSON-shape fallback warnings.
- Added stronger task-specific prompt previews with expected JSON response shapes.
- Added executed provider request logging via `model_requests.executed.json`.
- Added `review-model-requests <run_id>` with readable, JSON, and markdown artifact output.
- Added partial failure handling for `expand <run_id>` with optional `--fail-fast`.
- Added response metadata on expansion output items for provider/model/request mode/truncation.
- Extended mask validation and inspection to cover executed request artifacts and model request review artifacts.
- Added smoke script for model-request review and updated the optional real Ollama smoke script.
- Added tests for structured parsing, truncation, executed request logging, review summaries, and partial failure behavior.

Files added/modified:
- Modified `internal/modelrouter/request.go`.
- Modified `internal/modelrouter/dryrun.go`.
- Modified `internal/modelrouter/ollama.go`.
- Modified `internal/modelrouter/ollama_test.go`.
- Modified `internal/commands/model_router.go`.
- Modified `internal/commands/model_router_test.go`.
- Modified `internal/commands/mask.go`.
- Modified `internal/cli/root.go`.
- Added `scripts/smoke-model-request-review.sh`.
- Modified `scripts/smoke-ollama-real.sh`.
- Modified `docs/model-router.md`.
- Modified `docs/artifacts/model-requests.md`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/models.md`.
- Modified `docs/architecture/inference-mask.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video review-model-requests <run_id>
```

New flags:
```sh
./byom-video review-model-requests <run_id> --json
./byom-video review-model-requests <run_id> --write-artifact

./byom-video expand <run_id> --fail-fast
```

Structured parsing behavior:
- Ollama responses are first parsed as JSON.
- Supported JSON response shapes:
  - `{"items":[{"text":"..."}]}`
  - `{"captions":["..."]}`
  - `{"labels":["..."]}`
  - `{"descriptions":["..."]}`
- If JSON parse fails, BYOM Video falls back to plain text.
- If JSON parses but does not match the expected shape for the task type, BYOM Video falls back to plain text and records a warning.
- No panic paths were added.
- Best-effort `max_words` truncation is enforced after provider response shaping.

Prompt template behavior:
- `caption_variants`, `timeline_labels`, and `short_descriptions` now each use concise task-specific prompts.
- Prompts include:
  - decision text preview
  - timing
  - mask constraints
  - output contract
  - instruction not to invent facts
  - desired JSON response shape

Executed request artifact behavior:
- Real `expand <run_id>` now writes `.byom-video/runs/<run_id>/model_requests.executed.json`.
- Each executed request record includes task id, decision id, task type, route, model entry, provider, model, status, request preview, response mode, and error.
- Events:
  - `MODEL_REQUEST_STARTED`
  - `MODEL_REQUEST_COMPLETED`
  - `MODEL_REQUEST_FAILED`
- `model_requests.executed.json` is recorded in the manifest.

Review-model-requests behavior:
- Reads `model_requests.dryrun.json` and `model_requests.executed.json` when present.
- Prints:
  - request counts
  - provider/model distribution
  - task types
  - statuses
  - response modes
  - failures
- `--write-artifact` writes `model_requests_review.md` and records it in the manifest.

Partial failure behavior:
- `expand <run_id>` now supports partial failures.
- Default behavior continues other requests after a provider failure.
- `--fail-fast` stops on the first provider failure.
- Successful items are still written to expansion artifacts.
- Failed requests are still written to `model_requests.executed.json`.
- If any request fails, `expand` exits non-zero after writing artifacts and logs.

Commands run:
```sh
gofmt -w internal/modelrouter/request.go internal/modelrouter/dryrun.go internal/modelrouter/ollama.go internal/modelrouter/ollama_test.go internal/commands/model_router.go internal/commands/model_router_test.go internal/commands/mask.go internal/cli/root.go
go test ./...
go build ./cmd/byom-video
go build -o byom-video ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
chmod +x scripts/smoke-model-request-review.sh scripts/smoke-ollama-real.sh
scripts/smoke-model-request-review.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-model-request-review.sh` passed.
- Smoke reviewed the latest dry-run request artifact, wrote `model_requests_review.md`, and showed the new artifacts in `inspect-mask`.

How to run smoke model-request review test:
```sh
scripts/smoke-model-request-review.sh
```

Optional real Ollama smoke instructions:
```sh
scripts/smoke-ollama-real.sh
```

Known limitations:
- Real provider execution remains Ollama-only.
- JSON parsing is stronger but still intentionally conservative.
- `timeline_labels` and `short_descriptions` still shape to one item per decision in this milestone.
- Request review summarizes local artifacts only; it does not inspect raw provider transcripts.
- `inspect-mask` shows absent optional request artifacts as missing and invalid-looking rows even when the overall validation stays acceptable.
- No cloud providers, API key reads, SDKs, Docker, vector DB, web server, or NLE integrations were added.

Next recommended milestone:
- Add dedicated review of raw provider responses when needed for debugging.
- Add richer structured response validation per task type.
- Add batching for multiple decisions per request once output shaping guarantees are clear.
- Add provider request approval/review artifacts before real multi-provider expansion.

Errors or assumptions:
- Chose to keep one-item shaping for timeline labels and short descriptions even when the model returns more.
- Chose best-effort truncation with metadata warnings instead of rejecting long provider output outright.
- Chose to always write executed request logs during real expand, even when the run ends with partial failures.
<!-- HANDOFF 032 END -->

## Prompt 033 - Clip Cards + Enhanced Roughcut

<!-- PROMPT 033 START -->
Goal:
- Turn expansion outputs into editor-facing artifacts: clip cards, enhanced roughcut notes, and report integration.
- Add `clip-cards <run_id>`, `review-clips <run_id>`, and `enhance-roughcut <run_id>`.
- Use existing roughcut, mask, expansion, and verification artifacts only.
- Keep local-first and artifact-first.
- Do not add new provider calls, new model providers, Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 033 END -->

## Handoff 033

<!-- HANDOFF 033 START -->
What changed:
- Added `clip-cards <run_id>` to build editor-facing `clip_cards.json` from roughcut, mask, expansion, and verification artifacts.
- Added `review-clips <run_id>` with readable, JSON, and markdown review output.
- Added `enhance-roughcut <run_id>` to build additive `enhanced_roughcut.json`.
- Added shared editor artifact schemas and validation helpers.
- Extended run validation to validate `clip_cards.json` and `enhanced_roughcut.json` when present.
- Extended `inspect <run_id>` to show clip card and enhanced roughcut counts.
- Updated HTML report generation to include Clip Cards, Enhanced Roughcut, Expansion Outputs, and Verification Summary sections when artifacts exist.
- Added smoke script for clip-card and enhanced roughcut flow.
- Added tests for clip card generation, review, enhanced roughcut generation, validation, inspect counts, and report integration.

Files added/modified:
- Added `internal/editorartifacts/artifacts.go`.
- Added `internal/commands/editor_artifacts.go`.
- Added `internal/commands/editor_artifacts_test.go`.
- Added `docs/artifacts/clip-cards.md`.
- Added `docs/artifacts/enhanced-roughcut.md`.
- Added `scripts/smoke-clip-cards.sh`.
- Modified `internal/cli/root.go`.
- Modified `internal/commands/runs.go`.
- Modified `internal/runinfo/runinfo.go`.
- Modified `internal/runvalidate/runvalidate.go`.
- Modified `internal/report/report.go`.
- Modified `internal/report/report_test.go`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/architecture/inference-mask.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video clip-cards <run_id>
./byom-video review-clips <run_id>
./byom-video enhance-roughcut <run_id>
```

New flags:
```sh
./byom-video clip-cards <run_id> --overwrite
./byom-video clip-cards <run_id> --json

./byom-video review-clips <run_id> --json
./byom-video review-clips <run_id> --write-artifact

./byom-video enhance-roughcut <run_id> --overwrite
./byom-video enhance-roughcut <run_id> --json
```

Clip card behavior:
- `clip-cards` requires `roughcut.json`.
- It prefers roughcut clips as the base card source.
- It uses `inference_mask.json` when present to map decisions to clips.
- It attaches expansion outputs by `decision_id`:
  - `timeline_labels.json` -> title
  - `short_descriptions.json` -> description
  - `caption_variants.json` -> captions
- If expansions are absent, it falls back to roughcut text and edit intent.
- If verification results are present, it carries forward verification status and warnings.
- It writes `clip_cards.json`, records it in the manifest, and emits `CLIP_CARDS_STARTED`, `CLIP_CARDS_COMPLETED`, or `CLIP_CARDS_FAILED`.

Enhanced roughcut behavior:
- `enhance-roughcut` reads `roughcut.json` and optionally `clip_cards.json`.
- If clip cards exist, it uses their titles, descriptions, caption suggestions, source text, and verification status.
- If clip cards do not exist, it falls back to roughcut-only content.
- It writes additive `enhanced_roughcut.json` without modifying the original roughcut.
- It records `enhanced_roughcut.json` in the manifest and emits `ENHANCED_ROUGHCUT_STARTED`, `ENHANCED_ROUGHCUT_COMPLETED`, or `ENHANCED_ROUGHCUT_FAILED`.

Report integration behavior:
- `report.html` now includes:
  - Clip Cards section when `clip_cards.json` exists
  - Enhanced Roughcut section when `enhanced_roughcut.json` exists
  - Expansion Outputs summary when expansion artifacts exist
  - Verification Summary when `verification_results.json` exists
- `clip-cards`, `review-clips --write-artifact`, and `enhance-roughcut` refresh `report.html` when the run already has a report artifact or report file.
- `inspect <run_id>` now shows clip card count and enhanced roughcut clip count.

Commands run:
```sh
gofmt -w internal/editorartifacts/artifacts.go internal/commands/editor_artifacts.go internal/commands/editor_artifacts_test.go internal/commands/runs.go internal/runinfo/runinfo.go internal/runvalidate/runvalidate.go internal/report/report.go internal/report/report_test.go internal/cli/root.go
go test ./...
go build ./cmd/byom-video
go build -o byom-video ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
chmod +x scripts/smoke-clip-cards.sh
scripts/smoke-clip-cards.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `go build -o byom-video ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-clip-cards.sh` passed.
- Smoke created `clip_cards.json`, `clip_cards_review.md`, and `enhanced_roughcut.json`, refreshed the existing report path, and showed the new counts in `inspect`.

How to run smoke clip-cards test:
```sh
scripts/smoke-clip-cards.sh
```

Known limitations:
- Clip card verification mapping is best-effort; global verification failures may appear as per-card warnings when check details are not decision-specific.
- Fallback title and description generation is deterministic text shaping only.
- These commands do not call any model provider and do not generate new expansion outputs.
- No Docker, vector DB, web server, or NLE integrations were added.

Next recommended milestone:
- Add clip-card sorting and filtering artifacts for editor review.
- Add richer per-card verification attribution once verification details carry stronger decision-level references.
- Add export-facing handoff artifacts that connect clip cards to future NLE or timeline packaging formats.
- Add review/open helpers for clip card markdown and enhanced roughcut artifacts.

Errors or assumptions:
- Assumed `inspect-mask` should remain focused on mask/expansion/verification artifacts, while `inspect` surfaces clip cards and enhanced roughcut.
- Assumed report refresh should happen automatically only when a report already exists for the run.
- Assumed roughcut clips remain the authoritative base ordering for editor-facing cards.
<!-- HANDOFF 033 END -->

## Prompt 034 - Export Handoff Artifacts

<!-- PROMPT 034 START -->
Goal:
- Add export-facing handoff artifacts that connect clip cards/enhanced roughcut to rendered outputs and future NLE/timeline packaging.
- Add `selected-clips <run_id>`, `export-manifest <run_id>`, `ffmpeg-script <run_id>`, and `concat-plan <run_id>`.
- Add optional frame-accurate FFmpeg reencode mode and concat planning artifacts.
- Keep local-first and artifact-first.
- Do not add DaVinci/Premiere integration, web server, Docker, vector DB, new model providers, or new provider calls.
<!-- PROMPT 034 END -->

## Handoff 034

<!-- HANDOFF 034 START -->
What changed:
- Added `selected-clips <run_id>` to produce `selected_clips.json` from enhanced roughcut, clip cards, or roughcut fallback.
- Added `export-manifest <run_id>` to produce `export_manifest.json` from selected clips, local exports, and export validation data.
- Added `ffmpeg-script <run_id>` to regenerate `ffmpeg_commands.sh` from selected clips or roughcut with explicit `stream-copy` or `reencode` mode.
- Added `concat-plan <run_id>` to write `concat_list.txt` and `ffmpeg_concat.sh` planning artifacts.
- Added shared export-facing artifact schemas and validation helpers.
- Extended `validate <run_id>` to validate `selected_clips.json` and `export_manifest.json` when present.
- Extended `inspect <run_id>` to show selected clip count, export manifest summary, and concat plan presence.
- Updated report generation to include Selected Clips, Export Manifest, Concat Plan, and FFmpeg script mode when available.
- Added smoke script for export handoff flow.
- Added tests for selected clips, export manifest, FFmpeg modes, concat planning, validation, inspect integration, and report integration.

Files added/modified:
- Added `internal/exportartifacts/artifacts.go`.
- Added `internal/commands/export_handoff.go`.
- Added `internal/commands/export_handoff_test.go`.
- Added `docs/artifacts/selected-clips.md`.
- Added `docs/artifacts/export-manifest.md`.
- Added `docs/artifacts/concat-plan.md`.
- Added `scripts/smoke-export-handoff.sh`.
- Modified `internal/exportscript/ffmpeg.go`.
- Modified `internal/exportscript/ffmpeg_test.go`.
- Modified `internal/commands/run.go`.
- Modified `internal/commands/runs.go`.
- Modified `internal/runinfo/runinfo.go`.
- Modified `internal/runvalidate/runvalidate.go`.
- Modified `internal/report/report.go`.
- Modified `internal/report/report_test.go`.
- Modified `internal/cli/root.go`.
- Modified `internal/config/config.go`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/artifacts/ffmpeg-script.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video selected-clips <run_id>
./byom-video export-manifest <run_id>
./byom-video ffmpeg-script <run_id>
./byom-video concat-plan <run_id>
```

New flags:
```sh
./byom-video selected-clips <run_id> --overwrite
./byom-video selected-clips <run_id> --json

./byom-video export-manifest <run_id> --overwrite
./byom-video export-manifest <run_id> --json

./byom-video ffmpeg-script <run_id> --mode <stream-copy|reencode>
./byom-video ffmpeg-script <run_id> --overwrite
./byom-video ffmpeg-script <run_id> --json

./byom-video concat-plan <run_id> --overwrite
./byom-video concat-plan <run_id> --json
```

Selected clips behavior:
- `selected-clips` prefers `enhanced_roughcut.json`.
- If enhanced roughcut is missing, it uses `clip_cards.json` plus `roughcut.json` when available.
- If neither enhanced roughcut nor clip cards are available, it falls back to `roughcut.json`.
- It writes deterministic `output_filename` values like `clip_0001.mp4`.
- It records `selected_clips.json` in the manifest and emits `SELECTED_CLIPS_STARTED`, `SELECTED_CLIPS_COMPLETED`, or `SELECTED_CLIPS_FAILED`.

Export manifest behavior:
- `export-manifest` prefers `selected_clips.json` and falls back to generating selected clips from existing roughcut/enhanced roughcut inputs if needed.
- It always writes planned outputs under `exports/`.
- If rendered exports already exist, it marks `exists: true`.
- If `export_validation.json` exists, it marks `validated: true` for files with `status: ok`.
- It records `export_manifest.json` in the manifest and emits `EXPORT_MANIFEST_STARTED`, `EXPORT_MANIFEST_COMPLETED`, or `EXPORT_MANIFEST_FAILED`.

FFmpeg script mode behavior:
- Existing script generation now supports:
  - `stream-copy`
  - `reencode`
- Default mode remains `stream-copy`.
- `reencode` writes commands using `-c:v libx264 -c:a aac`.
- Generated scripts now include a header comment like `# mode: reencode`.
- `run` and preset-based pipeline generation now carry `FFmpegMode` internally, with default `stream-copy`.
- `ffmpeg-script <run_id>` regenerates `ffmpeg_commands.sh` from `selected_clips.json` when present, or `roughcut.json` otherwise.
- It requires `--overwrite` if `ffmpeg_commands.sh` already exists.

Concat plan behavior:
- `concat-plan` requires `selected_clips.json`.
- It writes:
  - `concat_list.txt`
  - `ffmpeg_concat.sh`
- `concat_list.txt` uses FFmpeg concat demuxer format and points at `exports/<output_filename>`.
- `ffmpeg_concat.sh` plans `exports/assembly.mp4`.
- These are planning artifacts only; they are not executed automatically.
- Both files are recorded in the manifest and `CONCAT_PLAN_STARTED`, `CONCAT_PLAN_COMPLETED`, or `CONCAT_PLAN_FAILED` are emitted.

Report/inspect integration:
- `inspect <run_id>` now shows:
  - selected clip count
  - export manifest summary
  - concat plan presence
- `report.html` now includes:
  - Selected Clips
  - Export Manifest
  - Concat Plan
  - FFmpeg script mode if discoverable from the script header
- `selected-clips`, `export-manifest`, `ffmpeg-script`, and `concat-plan` refresh `report.html` when a report already exists.

Commands run:
```sh
gofmt -w internal/exportartifacts/artifacts.go internal/exportscript/ffmpeg.go internal/exportscript/ffmpeg_test.go internal/commands/export_handoff.go internal/commands/export_handoff_test.go internal/commands/run.go internal/commands/runs.go internal/runinfo/runinfo.go internal/runvalidate/runvalidate.go internal/report/report.go internal/report/report_test.go internal/cli/root.go internal/config/config.go
go test ./...
go build ./cmd/byom-video
go build -o byom-video ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
chmod +x scripts/smoke-export-handoff.sh
scripts/smoke-export-handoff.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `go build -o byom-video ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-export-handoff.sh` passed.
- Smoke created `selected_clips.json`, regenerated `ffmpeg_commands.sh` in `reencode` mode, wrote `export_manifest.json`, wrote concat planning artifacts, validated the run, and showed the new export handoff summary in `inspect`.

How to run smoke export-handoff test:
```sh
scripts/smoke-export-handoff.sh
```

Known limitations:
- Export handoff remains FFmpeg-only.
- `concat-plan` assumes rendered clips already exist under `exports/`, but it does not verify codec compatibility for concat.
- `export-manifest` tracks planned/exported/validated state only; it does not yet include per-file checksum data.
- No DaVinci, Premiere, web server, Docker, vector DB, new model providers, or new provider calls were added.

Next recommended milestone:
- Add assembly validation after concat execution.
- Add per-export checksums and richer export provenance metadata.
- Add export packaging artifacts for future NLE/timeline handoff without implementing NLE integration yet.
- Add optional clip subset selection and ordering adjustments for export-specific handoff.

Errors or assumptions:
- Assumed `selected_clips.json` should be the export-facing source of truth once present.
- Assumed report refresh should remain conditional on an existing report artifact or file.
- Assumed `ffmpeg-script` should refuse overwrite unless `--overwrite` is passed.
<!-- HANDOFF 034 END -->

## Prompt 035 - OSS Alpha Release Candidate Polish

<!-- PROMPT 035 START -->
Goal:
- Prepare BYOM Video as an OSS alpha release candidate.
- Add public-facing docs, examples, release hygiene, sanity scripts, and version reporting.
- Keep scope to polish, documentation, examples, and release readiness.
- Do not add new model providers, new feature families, web server, Docker, vector DB, or NLE integrations.
<!-- PROMPT 035 END -->

## Handoff 035

<!-- HANDOFF 035 START -->
What changed:
- Rewrote `README.md` into a public OSS-ready alpha README with practical quickstart, workflow, privacy, limitations, and roadmap sections.
- Added release-oriented docs for quickstart, demo flow, roadmap, security, architecture, release checklist, limitations, and a docs index.
- Added documented example workflows for local shorts, batch, watch, agent planning, inference mask flow, and local Ollama usage.
- Added OSS repository scaffolding with MIT license, contribution guide, issue templates, and pull request template.
- Added `byom-video version` with alpha version, commit, and build date reporting.
- Added `scripts/release-smoke.sh` to run the core test/build/smoke release candidate checks and skip optional steps cleanly when local prerequisites are missing.
- Added a `Makefile` with `build`, `test`, `smoke`, `release-smoke`, and non-destructive `clean-local-artifacts` targets.
- Hardened `scripts/smoke-runs.sh` so release smoke can choose a run with an existing report and skip `open-report` cleanly when no report is present.

Files added/modified:
- Added `internal/commands/version.go`.
- Added `docs/README.md`.
- Added `docs/quickstart.md`.
- Added `docs/demo.md`.
- Added `docs/roadmap.md`.
- Added `docs/security.md`.
- Added `docs/architecture.md`.
- Added `docs/release-checklist.md`.
- Added `docs/limitations.md`.
- Added `examples/workflows/shorts-local.md`.
- Added `examples/workflows/batch-folder.md`.
- Added `examples/workflows/watch-folder.md`.
- Added `examples/workflows/agent-plan.md`.
- Added `examples/workflows/inference-mask.md`.
- Added `examples/workflows/ollama-local.md`.
- Added `LICENSE`.
- Added `CONTRIBUTING.md`.
- Added `.github/ISSUE_TEMPLATE/bug_report.md`.
- Added `.github/ISSUE_TEMPLATE/feature_request.md`.
- Added `.github/pull_request_template.md`.
- Added `Makefile`.
- Added `scripts/release-smoke.sh`.
- Modified `README.md`.
- Modified `examples/README.md`.
- Modified `scripts/smoke-runs.sh`.
- Modified `internal/cli/root.go`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video version
```

Docs added:
```text
docs/README.md
docs/quickstart.md
docs/demo.md
docs/roadmap.md
docs/security.md
docs/architecture.md
docs/release-checklist.md
docs/limitations.md
examples/workflows/shorts-local.md
examples/workflows/batch-folder.md
examples/workflows/watch-folder.md
examples/workflows/agent-plan.md
examples/workflows/inference-mask.md
examples/workflows/ollama-local.md
```

Release smoke behavior:
- `scripts/release-smoke.sh` runs:
  - `go test ./...`
  - `go build ./cmd/byom-video`
  - `python3 -m compileall -q workers/byom_video_workers`
- If `media/Untitled.mov` exists and the selected Python can import `faster_whisper`, it runs `scripts/smoke-pipeline.sh media/Untitled.mov`; otherwise it skips cleanly with an explanation.
- If local runs exist, it runs `scripts/smoke-runs.sh`.
- If local run artifacts exist, it attempts `scripts/smoke-mask-plan.sh` and `scripts/smoke-export-handoff.sh`, relying on those scripts to skip cleanly when prerequisites are missing.
- `--with-ollama` runs the optional local Ollama smoke flow.
- No cloud providers are called.

Version behavior:
- `./byom-video version` prints:
  - version
  - commit
  - build date
- Current defaults are:
  - version: `v0.1.0-alpha`
  - commit: `unknown`
  - build date: `unknown`

Commands run:
```sh
gofmt -w internal/commands/version.go internal/cli/root.go
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
chmod +x scripts/release-smoke.sh
scripts/release-smoke.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/release-smoke.sh` passed.
- Release smoke skipped the pipeline smoke cleanly when `faster_whisper` was not importable from plain `python3`, then completed the remaining local release checks successfully.

Known limitations:
- This remains alpha software with local-first workflows and CLI-first UX.
- FFmpeg is required for export workflows.
- `faster-whisper` is optional in installation but required for real transcription/captions/highlights workflows.
- Real provider-backed expansion currently supports only local Ollama.
- Cloud providers remain unimplemented even if example configs mention future shapes.
- There is no web UI.
- There is no DaVinci/Premiere or other NLE integration yet.
- Watch mode remains polling-based.
- Release smoke intentionally skips optional checks when local prerequisites or artifacts are missing.

Recommended v0.1-alpha release checklist:
- Run `go test ./...`.
- Run `go build ./cmd/byom-video`.
- Run `python3 -m compileall -q workers/byom_video_workers`.
- Run `scripts/release-smoke.sh`.
- Optionally run `scripts/release-smoke.sh --with-ollama` on a machine with Ollama running and a pulled local model.
- Verify `./byom-video version`.
- Review `README.md`, `docs/`, and `examples/workflows/` for broken links and command drift.
- Confirm local sample media/demo workflow still matches the documented quickstart.

Next recommended milestone after alpha:
- Add tagged build metadata injection for commit/build date in release binaries.
- Add richer release packaging and changelog automation.
- Deepen verification and export provenance before broader provider support.
- Add future handoff packaging for NLE/timeline exchange without implementing integrations yet.

Errors or assumptions:
- Assumed MIT was the simplest default OSS license for alpha release readiness.
- Assumed example cloud-provider config files should remain documented as illustrative future shapes, not supported execution paths.
- Assumed release smoke should favor clean skips over hard failures for optional local dependencies such as `faster_whisper` and Ollama.
<!-- HANDOFF 035 END -->


## Test Phase 1 — External User Install + Pipeline Smoke

<!-- TEST PHASE 1 START -->
Date: 2026-05-05

Goal: Verify the full external user install and basic pipeline flow from a clean machine perspective.

### Install Issues Found and Fixed

| Issue | Fix |
|---|---|
| `.byom-video/` local run data committed to git | `git rm -r --cached .byom-video/` |
| `internal/media/` package not committed (ignored by overbroad `media/` gitignore rule) | Changed `media/` to `/media/` in `.gitignore`, committed `internal/media/ffprobe.go` |
| GitHub CDN cached old module zip for reused tag names | Used new tag names (`v0.1.1`, `v0.1.2`, `v0.1.3`) each time |
| `byom_video_workers` Python package not installed by `install.sh` | Added `git clone + pip install workers/` step to `install.sh` |
| `byom-video doctor` showed MISSING configured python even when `BYOM_VIDEO_PYTHON` was set | Fixed `doctor.go` to check env var before falling back to config file |

### Commands Tested

| Command | Result |
|---|---|
| `curl -fsSL .../install.sh \| sh` | Passed after above fixes |
| `byom-video version` | Passed — printed v0.1.0-alpha |
| `byom-video doctor` | All OK after env var fix |
| `byom-video init` | Created byom-video.yaml |
| `byom-video pipeline Untitledvi.mov --preset shorts` | Passed — transcribed, highlighted, roughcut, FFmpeg script, report |
| `byom-video plan --goal "make a short clip under 60 seconds"` | Plan created |
| `byom-video review-plan <plan_id>` | Showed planned actions |
| `byom-video approve-plan <plan_id>` | Approved |
| `byom-video execute-plan <plan_id>` | Executed, produced new run |

### Known Limitations Surfaced

- `go install` requires `GOPROXY=direct GONOSUMDB='*'` due to uppercase letters in module path (`OpenVFX`) causing sum database friction. Long-term fix: rename module to lowercase.
- The "agentic" planning layer is deterministic only — the `--goal` text is stored but does not influence highlight selection or roughcut decisions. Real LLM integration is needed for goal-driven editing.
- `execute-plan` re-runs the pipeline rather than acting on prior run artifacts, producing a duplicate run.

### Next Recommended Step

Wire an LLM (local Ollama) into the highlight selection and roughcut decisions so the `--goal` text actually influences what gets cut.
<!-- TEST PHASE 1 END -->


## Prompt 036 - Install Module Hygiene Hardening

<!-- PROMPT 036 START -->
Goal: Harden install/module/release hygiene after external-user install testing exposed alpha install issues.

Parts:
- A: Module path hygiene — rename to lowercase github.com/mirelahmd/byom-video
- B: .gitignore hardening — anchor root patterns, add docs note
- C: install.sh hardening — env vars, persistent clone dir, proper workers install
- D: doctor hardening — resolution order, worker import checks, --transcription flag, OPTIONAL status
- E: External install smoke script
- F: Version build metadata injection via ldflags, Makefile targets
- G: Docs update — quickstart, release-checklist, limitations, README
- H: Tests — doctor python resolution order, worker import check, version defaults
- I: PROGRESS.md
<!-- PROMPT 036 END -->

## Handoff 036

<!-- HANDOFF 036 START -->
What changed:

**Module path:**
- Before: `github.com/mirelahmd/OpenVFX`
- After: `github.com/mirelahmd/byom-video`
- All 54 Go source files updated via sed.
- `go test ./...` passes after rename.
- NOTE: `go install github.com/mirelahmd/byom-video/cmd/byom-video@latest` requires renaming the GitHub repo from `OpenVFX` to `byom-video`. Until the rename, use `GOPROXY=direct GONOSUMDB='*'` with a versioned tag.

**.gitignore changes:**
- Already correct from Test Phase 1 fixes.
- Added tag policy and gitignore hygiene steps to `docs/release-checklist.md`.

**install.sh behavior:**
- Accepts env vars: `BYOM_VIDEO_REPO_URL`, `BYOM_VIDEO_REF`, `BYOM_VIDEO_INSTALL_DIR`, `BYOM_VIDEO_PYTHON`, `BYOM_VIDEO_SKIP_PYTHON`.
- Clones worker source to `$HOME/.byom-video/src/byom-video` (persistent, updates on re-run).
- Installs `workers[transcribe]` with pip install -e.
- Appends to shell rc without duplicating lines.
- Prints clear steps and warnings without pretending success on failure.
- Never touches user media or run artifacts.

**doctor behavior:**
- New `DoctorOptions{Transcription bool}` struct.
- Python resolution order: `BYOM_VIDEO_PYTHON` → config `python.interpreter` → `python3` on PATH.
- Shows source label in output: `OK configured python [BYOM_VIDEO_PYTHON]`, `[config]`, or `[PATH]`.
- Checks `byom_video_workers` importable.
- Checks `faster_whisper` importable.
- When `--transcription` not set: shows `OPTIONAL` for missing worker/whisper.
- When `--transcription` set: shows `MISSING` and prints transcription check active notice.
- `byom-video doctor --transcription` is the new strict transcription check.

**version/build metadata:**
- `make build` injects version, commit (git short hash), and build date via ldflags.
- `make install-local` installs with metadata to `~/go/bin`.
- `make release-build` adds `-trimpath` for clean release binaries.

**Files added/modified:**
- Modified `go.mod` — module path rename.
- Modified all 54 `.go` files — import path update.
- Modified `internal/commands/doctor.go` — full rewrite with DoctorOptions, resolution order, import checks.
- Added `internal/commands/doctor_test.go` — 10 tests.
- Modified `internal/cli/root.go` — doctor case updated, parseDoctorArgs added, usage line updated.
- Modified `install.sh` — full rewrite with env vars and persistent clone dir.
- Modified `Makefile` — ldflags, install-local, release-build, external-install-smoke targets.
- Added `scripts/smoke-external-install.sh`.
- Modified `docs/release-checklist.md` — artifact hygiene, module path policy, tag policy.
- Modified `docs/quickstart.md` — updated install paths and Python setup.
- Modified `docs/limitations.md` — added install/module/tag limitations.
- Modified `README.md` — updated install section and go install note.

Commands run:
```sh
sed -i '' 's|github.com/mirelahmd/OpenVFX|github.com/mirelahmd/byom-video|g' go.mod
find . -name "*.go" -exec sed -i '' 's|github.com/mirelahmd/OpenVFX|github.com/mirelahmd/byom-video|g' {} \;
make build
make install-local
go test ./...
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-external-install.sh
```

Test results:
- `go test ./...` — ALL PASS (23 packages, 160+ tests).
- `go build ./cmd/byom-video` — passed.
- `python3 -m compileall -q workers/byom_video_workers` — passed.
- `scripts/smoke-external-install.sh` — passed (go install from remote skipped pending GitHub repo rename).
- `make build` — version/commit/build-date injected correctly.

External install smoke result:
- Passed. Binary built locally, init/doctor/version all ran correctly.
- go install from remote skipped — requires GitHub repo rename from `OpenVFX` to `byom-video`.

Known limitations:
- `go install github.com/mirelahmd/byom-video/cmd/byom-video@latest` will not work until GitHub repo is renamed. Current workaround: `GOPROXY=direct GONOSUMDB='*' go install github.com/mirelahmd/OpenVFX/cmd/byom-video@<tag>`.
- External install smoke does not test the full remote `go install` path for the same reason.
- Never reuse pushed tag names — CDN caches by tag name.

Next recommended milestone after Prompt 036:
- Rename GitHub repo from `OpenVFX` to `byom-video` and push a new tag for the full `go install` path to work cleanly.
- Wire LLM (local Ollama) into highlight selection so `--goal` text influences what gets cut.
<!-- HANDOFF 036 END -->

## Prompt 037 - Goal-Aware Roughcut

<!-- PROMPT 037 START -->
Goal:
- Add goal-aware highlight reranking and roughcut selection using local Ollama when explicitly requested, while preserving deterministic fallback.
- Add `goal-rerank <run_id> --goal "<text>"`.
- Add `goal-roughcut <run_id>`.
- Keep local-first and artifact-first.
- Do not call providers during the normal pipeline unless a goal-aware model flag is explicitly passed.
- Do not add cloud providers, web server, Docker, vector DB, or NLE integrations.
<!-- PROMPT 037 END -->

## Handoff 037

<!-- HANDOFF 037 START -->
What changed:
- Added `goal-rerank <run_id> --goal <text>` to produce `goal_rerank.json` from existing `highlights.json`.
- Added deterministic goal parsing and deterministic highlight reranking.
- Added optional local Ollama reranking behind `--use-ollama`.
- Added `--fallback-deterministic` so Ollama rerank can fall back visibly instead of failing closed.
- Added `goal-roughcut <run_id>` to produce `goal_roughcut.json` from ranked highlights.
- Added goal-aware artifact schemas and validation helpers.
- Extended inspect output to show goal rerank count, goal rerank mode, and goal roughcut clip count.
- Extended report generation to include Goal Rerank and Goal Roughcut sections.
- Extended run validation to validate `goal_rerank.json` and `goal_roughcut.json` when present.
- Added a deterministic smoke script for the goal-aware flow.
- Updated local config defaults and example config to include `goal_reranking` route examples.
- Updated README, agent docs, model docs, and artifact docs for goal-aware flows.

Files added/modified:
- Added `internal/goalartifacts/artifacts.go`.
- Added `internal/commands/goal_aware.go`.
- Added `internal/commands/goal_aware_test.go`.
- Added `docs/artifacts/goal-rerank.md`.
- Added `docs/artifacts/goal-roughcut.md`.
- Added `scripts/smoke-goal-aware.sh`.
- Modified `internal/modelrouter/dryrun.go`.
- Modified `internal/modelrouter/ollama.go`.
- Modified `internal/runinfo/runinfo.go`.
- Modified `internal/runvalidate/runvalidate.go`.
- Modified `internal/report/report.go`.
- Modified `internal/commands/runs.go`.
- Modified `internal/cli/root.go`.
- Modified `internal/config/config.go`.
- Modified `byom-video.yaml`.
- Modified `examples/configs/local-only.yaml`.
- Modified `docs/artifacts/README.md`.
- Modified `docs/agent.md`.
- Modified `docs/models.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video goal-rerank <run_id> --goal "<text>"
./byom-video goal-roughcut <run_id>
```

New flags:
```sh
./byom-video goal-rerank <run_id> --goal "<text>" --use-ollama
./byom-video goal-rerank <run_id> --goal "<text>" --fallback-deterministic

./byom-video goal-roughcut <run_id> --overwrite
./byom-video goal-roughcut <run_id> --json
```

Goal parsing behavior:
- Deterministically extracts:
  - `under 60 seconds` / `less than 60 seconds` -> `max_total_duration_seconds`
  - `make 3 clips` / `3 shorts` -> `max_clips`
  - `short`, `shorts`, `reel`, `tiktok`, `instagram` -> `preferred_style: shorts`
  - `cinematic`, `technical`, `funny`, `emotional` -> matching style
- If no constraints are found, defaults are:
  - `max_total_duration_seconds: 60`
  - `max_clips: 5`
  - `preferred_style: general`

Deterministic rerank behavior:
- Uses existing highlight score plus simple goal matching.
- Boosts highlights when goal keywords appear in highlight text.
- Penalizes highlights that exceed the parsed max duration.
- Prefers shorter clips for shorts-style goals.
- Writes `goal_rerank.json` with:
  - original score
  - goal score
  - rank
  - deterministic reason
- Does not call any model provider.

Ollama rerank behavior:
- Only runs when `--use-ollama` is passed.
- Requires:
  - `models.enabled: true`
  - configured `models.routes.goal_reranking`
- Sends compact highlight candidate data, goal text, and parsed constraints to the existing local Ollama adapter.
- Expects JSON shaped like:
```json
{
  "ranked_highlights": [
    {
      "highlight_id": "hl_0001",
      "goal_score": 0.91,
      "reason": "Strong match for the goal."
    }
  ]
}
```
- If the response JSON is invalid or unusable:
  - fails cleanly by default
  - falls back to deterministic mode only when `--fallback-deterministic` is passed
- No cloud providers or API keys are used.

Goal roughcut behavior:
- Reads `goal_rerank.json`.
- Selects ranked highlights in rank order.
- Respects:
  - `max_total_duration_seconds`
  - `max_clips`
- Then reorders selected clips into original timeline order.
- Writes additive `goal_roughcut.json`.
- Leaves the original `roughcut.json` unchanged.
- Records `goal_roughcut.json` in the manifest.

Report/inspect integration:
- `inspect <run_id>` now shows:
  - goal rerank count
  - goal rerank mode
  - goal roughcut clip count
- `report.html` now includes:
  - Goal Rerank section
  - Goal Roughcut section
  - user goal
  - selected clips
  - goal scores
  - reasons

Commands run:
```sh
gofmt -w internal/goalartifacts/artifacts.go internal/commands/goal_aware.go internal/commands/goal_aware_test.go internal/modelrouter/dryrun.go internal/modelrouter/ollama.go internal/runinfo/runinfo.go internal/runvalidate/runvalidate.go internal/report/report.go internal/cli/root.go internal/commands/runs.go internal/config/config.go
chmod +x scripts/smoke-goal-aware.sh
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-goal-aware.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-goal-aware.sh` passed.
- Smoke created `goal_rerank.json`, created `goal_roughcut.json`, validated both artifacts, and showed the new goal-aware counts in `inspect`.

How to run smoke goal-aware test:
```sh
scripts/smoke-goal-aware.sh
```

Optional Ollama goal-aware command:
```sh
./byom-video goal-rerank <run_id> --goal "make a cinematic short" --use-ollama --fallback-deterministic
```

Known limitations:
- Goal-aware reranking is command-level and explicit; it is not yet integrated into the default pipeline or agent plan execution path.
- Ollama reranking is local-only.
- Goal parsing is intentionally simple string matching and numeric extraction.
- Deterministic reranking remains heuristic.
- Goal roughcut selects from ranked highlights only; it does not re-segment media.
- No cloud providers, web server, Docker, vector DB, or NLE integrations were added.

Next recommended milestone:
- Add minimal plan-level `--goal-aware` support so approved plans can optionally invoke goal-aware rerank and goal roughcut explicitly.
- Add richer deterministic goal parsing for clip subsets and more style controls.
- Add deeper Ollama response validation and optional rerank review artifacts.
- Extend goal-aware selection into export-facing clip-card and selected-clip ordering workflows.

Errors or assumptions:
- Assumed the first explicit count in a goal mentioning `clips` or `shorts` should become `max_clips`.
- Assumed goal-aware reranking should overwrite `goal_rerank.json` by design so iterative reruns stay simple.
- Assumed goal-aware roughcut should stay additive and should never mutate the original `roughcut.json`.
- Kept agent-plan integration minimal in this prompt and documented it as future work.
<!-- HANDOFF 037 END -->

## Prompt 038 - Agent Goal-Aware Execution

<!-- PROMPT 038 START -->
Goal:
- Wire goal-aware reranking and goal-aware roughcut into agent plans, execution, clip cards, selected clips, and export handoff.
- Add `plan --goal-aware`.
- Add explicit goal-aware plan actions and execution support.
- Add `clip-cards --prefer-goal-roughcut`, `selected-clips --prefer-goal-roughcut`, and `goal-handoff <run_id>`.
- Keep local-first and artifact-first.
- Use Ollama only when explicitly requested.
- Do not add cloud providers, Docker, vector DB, web server, or NLE integrations.
<!-- PROMPT 038 END -->

## Handoff 038

<!-- HANDOFF 038 START -->
What changed:
- Added agent plan support for `--goal-aware`, `--goal-use-ollama`, and `--goal-fallback-deterministic`.
- Added explicit `goal_rerank` and `goal_roughcut` action types to plan artifacts, validation, previews, review, diff, and execution.
- Added goal-aware post-processing execution after `run_pipeline` when those actions are present in an approved plan.
- Added `clip-cards --prefer-goal-roughcut` so editor-facing cards can use `goal_roughcut.json` as the base source.
- Added `selected-clips --prefer-goal-roughcut` so export handoff can use the goal-aware cut path explicitly.
- Added `goal-handoff <run_id>` to generate clip cards, selected clips, FFmpeg script, and export manifest from `goal_roughcut.json`.
- Extended inspect output to show selected clip source alongside goal-aware counts.
- Extended report output to show a Goal-Aware Editing section and goal-aware artifact sources.
- Added tests for goal-aware plan creation, review output, goal-aware execution, goal-source clip cards/selected clips, goal-handoff, inspect source output, and report source output.
- Added a smoke script for agent goal-aware dry-run flow.

Files added/modified:
- Added `scripts/smoke-agent-goal-aware.sh`.
- Modified `internal/agent/agent.go`.
- Modified `internal/agent/agent_test.go`.
- Modified `internal/commands/agent.go`.
- Modified `internal/commands/agent_test.go`.
- Modified `internal/commands/plan_review.go`.
- Modified `internal/commands/revision.go`.
- Modified `internal/commands/editor_artifacts.go`.
- Modified `internal/commands/editor_artifacts_test.go`.
- Modified `internal/commands/export_handoff.go`.
- Modified `internal/commands/export_handoff_test.go`.
- Modified `internal/commands/runs.go`.
- Modified `internal/runinfo/runinfo.go`.
- Modified `internal/report/report.go`.
- Modified `internal/report/report_test.go`.
- Modified `internal/cli/root.go`.
- Modified `docs/agent.md`.
- Modified `docs/artifacts/goal-rerank.md`.
- Modified `docs/artifacts/goal-roughcut.md`.
- Modified `docs/artifacts/clip-cards.md`.
- Modified `docs/artifacts/selected-clips.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video goal-handoff <run_id>
```

New flags:
```sh
./byom-video plan <path> --goal-aware
./byom-video plan <path> --goal-use-ollama
./byom-video plan <path> --goal-fallback-deterministic

./byom-video clip-cards <run_id> --prefer-goal-roughcut
./byom-video selected-clips <run_id> --prefer-goal-roughcut

./byom-video goal-handoff <run_id> --overwrite
./byom-video goal-handoff <run_id> --json
```

Agent goal-aware behavior:
- `plan --goal-aware` adds explicit `goal_rerank` and `goal_roughcut` actions after the normal file pipeline action.
- Goal-aware planning currently supports file-target plans only.
- Command previews show:
  - `./byom-video goal-rerank <run_id> --goal "..."`
  - `./byom-video goal-roughcut <run_id>`
- `--goal-use-ollama` and `--goal-fallback-deterministic` are preserved in the action options and previews.
- Plan validation accepts `goal_rerank` and `goal_roughcut`.
- Review and inspect surfaces now show goal-aware execution intent, Ollama usage, fallback allowance, and command previews.

Execution behavior:
- Plan execution still runs the pipeline action first.
- After a run id is produced, `goal_rerank` executes against that run.
- `goal_roughcut` then executes against the same run and writes additive goal-aware artifacts.
- Ollama is only called if the saved action options explicitly request it.
- If goal reranking fails without fallback, plan execution fails cleanly. If fallback is enabled, reranking stays explicit and local.

Clip cards/selected clips goal source behavior:
- `clip-cards --prefer-goal-roughcut` fails cleanly if `goal_roughcut.json` is missing.
- When present, it uses goal-aware clips as the base source and records `goal_roughcut.json` in the artifact source.
- `selected-clips --prefer-goal-roughcut` does the same for export-facing clip selection and ordering.
- Default clip-card and selected-clip behavior is unchanged when the flag is not used.

Goal handoff behavior:
- `goal-handoff <run_id>` is an explicit helper that runs:
  - `clip-cards --prefer-goal-roughcut`
  - `selected-clips --prefer-goal-roughcut`
  - `ffmpeg-script --overwrite`
  - `export-manifest --overwrite`
- It does not export media.
- It keeps the goal-aware path explicit and local.

Report/inspect integration:
- `inspect <run_id>` now shows selected clip source when `selected_clips.json` exists.
- If goal-aware artifacts exist, report generation now adds a Goal-Aware Editing section.
- `clip_cards.json` and `selected_clips.json` report sections now show when they were sourced from `goal_roughcut.json`.

Commands run:
```sh
gofmt -w internal/agent/agent.go internal/agent/agent_test.go internal/commands/agent.go internal/commands/agent_test.go internal/commands/plan_review.go internal/commands/revision.go internal/commands/runs.go internal/runinfo/runinfo.go internal/report/report.go internal/report/report_test.go internal/commands/editor_artifacts.go internal/commands/editor_artifacts_test.go internal/commands/export_handoff.go internal/commands/export_handoff_test.go internal/cli/root.go
chmod +x scripts/smoke-agent-goal-aware.sh
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-agent-goal-aware.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-agent-goal-aware.sh` passed.
- Smoke created a dry-run goal-aware plan, showed `goal_rerank` and `goal_roughcut` action previews, and printed the explicit goal-handoff follow-up path.

How to run smoke agent-goal-aware test:
```sh
scripts/smoke-agent-goal-aware.sh
```

Known limitations:
- Goal-aware planning is still explicit and file-plan-only in this milestone.
- Default clip-card and selected-clip generation still use the existing roughcut/enhanced roughcut path unless `--prefer-goal-roughcut` is passed.
- `goal-handoff` assumes `goal_roughcut.json` already exists.
- Ollama goal reranking remains local-only and explicit.
- No cloud providers, Docker, vector DB, web server, or NLE integrations were added.

Next recommended milestone:
- Add minimal plan revision support for toggling goal-aware plan options after plan creation.
- Add goal-aware selected-clip ordering controls and subset selection.
- Add review/export bundle artifacts that summarize the goal-aware cut path for editors.
- Consider optional agent execution helpers that surface the resulting run id and goal-handoff path together.

Errors or assumptions:
- Assumed goal-aware planning should stay limited to file plans for now.
- Assumed `goal-handoff` should always regenerate the FFmpeg script with `stream-copy` mode unless the user separately regenerates it.
- Assumed goal-aware clip-card and selected-clip source switching should remain explicit flags rather than automatic behavior.
<!-- HANDOFF 038 END -->

## Prompt 039 - Agent Result Summary + Goal Review Bundle

<!-- PROMPT 039 START -->
Goal:
- Add agent execution result surfacing and goal-aware review bundles so users can clearly see what an approved/executed plan produced and what to do next.
- Improve `execute-plan` terminal summaries.
- Add `agent-result <plan_id>`.
- Add `goal-review-bundle <run_id>`.
- Keep local-first and artifact-first.
- Do not add new providers, cloud calls, web server, Docker, vector DB, or NLE integrations.
<!-- PROMPT 039 END -->

## Handoff 039

<!-- HANDOFF 039 START -->
What changed:
- Improved successful `execute-plan` output with plan id, final status, run id, run directory, report path, goal-aware artifact paths, and suggested next commands.
- Improved failed `execute-plan` output with failed action id/type, error, and follow-up `inspect-plan` and `review-plan` suggestions.
- Added `agent-result <plan_id>` to summarize what a plan produced after planning or execution.
- Added `agent_result.md` artifact writing with `agent-result --write-artifact`.
- Added `goal-review-bundle <run_id>` to produce a readable goal-aware review bundle from rerank, roughcut, clip-card, selected-clip, export-manifest, and report artifacts when present.
- Recorded `goal_review_bundle.md` in the run manifest.
- Extended `plan-artifacts` and `inspect-plan` to show `agent_result.md` and resulting run ids / batch ids.
- Extended `inspect <run_id>` to show `goal_review_bundle.md` and associated `agent_result.md` when a plan/result mapping is available.
- Extended run report generation to surface `goal_review_bundle.md` inside the Goal-Aware Editing section.
- Extended run validation to treat `goal_review_bundle.md` as a known local artifact.
- Added smoke script for `agent-result` and goal-review-bundle flow.
- Added tests for execute-plan summaries, `agent-result`, `agent_result.md`, plan-artifact visibility, goal-review-bundle generation, inspect visibility, and validation behavior.

Files added/modified:
- Added `internal/commands/agent_result.go`.
- Added `scripts/smoke-agent-result.sh`.
- Modified `internal/commands/agent.go`.
- Modified `internal/commands/agent_test.go`.
- Modified `internal/commands/goal_aware_test.go`.
- Modified `internal/cli/root.go`.
- Modified `internal/runinfo/runinfo.go`.
- Modified `internal/commands/runs.go`.
- Modified `internal/runvalidate/runvalidate.go`.
- Modified `internal/report/report.go`.
- Modified `docs/agent.md`.
- Modified `docs/artifacts/goal-rerank.md`.
- Modified `docs/artifacts/goal-roughcut.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video agent-result <plan_id>
./byom-video goal-review-bundle <run_id>
```

New flags:
```sh
./byom-video agent-result <plan_id> --json
./byom-video agent-result <plan_id> --write-artifact

./byom-video goal-review-bundle <run_id> --json
./byom-video goal-review-bundle <run_id> --overwrite
```

Execute-plan summary behavior:
- On success, `execute-plan` now prints:
  - plan id
  - final status
  - resulting run id
  - run directory
  - report path when present
  - `goal_rerank.json` and `goal_roughcut.json` paths when present
  - suggested next commands such as `inspect`, `open-report`, `goal-handoff`, `export`, and `validate`
- On failure, `execute-plan` now prints:
  - failed action id/type
  - error
  - `inspect-plan` and `review-plan` suggestions

Agent-result behavior:
- `agent-result <plan_id>` reads `agent_plan.json` and action execution state.
- It reports:
  - plan id
  - goal
  - status
  - approval status
  - execution status
  - resulting run ids
  - resulting batch ids
  - important artifact paths
  - suggested next commands
- `agent-result --write-artifact` writes:
```text
.byom-video/plans/<plan_id>/agent_result.md
```
- Writing the artifact records `AGENT_RESULT_ARTIFACT_WRITTEN` in the plan action log.

Goal-review-bundle behavior:
- `goal-review-bundle <run_id>` always writes:
```text
.byom-video/runs/<run_id>/goal_review_bundle.md
```
- It requires `goal_rerank.json` and `goal_roughcut.json`.
- If present, it also uses:
  - `clip_cards.json`
  - `selected_clips.json`
  - `export_manifest.json`
  - `report.html`
- The bundle includes:
  - run id
  - goal text
  - rerank mode
  - goal constraints
  - selected goal-aware clips
  - scores and reasons
  - clip card titles/captions when available
  - selected output filenames when available
  - export manifest summary when available
  - next commands
- It records `goal_review_bundle.md` in the run manifest and writes:
  - `GOAL_REVIEW_BUNDLE_STARTED`
  - `GOAL_REVIEW_BUNDLE_COMPLETED`
  - `GOAL_REVIEW_BUNDLE_FAILED`

Report/inspect integration:
- `inspect <run_id>` now shows:
  - `goal_review_bundle.md` path when present
  - associated `agent_result.md` path when a plan action references the run id and the artifact exists
- `report.html` now shows the `goal_review_bundle.md` path inside the Goal-Aware Editing section when present
- `plan-artifacts` and `inspect-plan` now show:
  - `agent_result.md`
  - resulting run ids
  - resulting batch ids

Commands run:
```sh
gofmt -w internal/commands/agent.go internal/commands/agent_result.go internal/runinfo/runinfo.go internal/commands/runs.go internal/runvalidate/runvalidate.go internal/report/report.go internal/cli/root.go internal/commands/agent_test.go internal/commands/goal_aware_test.go
chmod +x scripts/smoke-agent-result.sh
go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-agent-result.sh
```

Test results:
- `go test ./...` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-agent-result.sh` passed.
- Smoke wrote `agent_result.md` for the latest available plan and printed the clean follow-up instruction when no executed goal-aware run was available.

How to run smoke agent-result test:
```sh
scripts/smoke-agent-result.sh
```

Known limitations:
- `agent-result` reflects the execution state recorded in `agent_plan.json`; it does not reconstruct missing run ids if actions were edited manually.
- `goal-review-bundle` requires the goal-aware artifacts to exist already; it does not generate them.
- Run-to-plan mapping for `agent_result.md` in `inspect <run_id>` is based on matching run ids in plan actions.
- No new providers, cloud calls, web server, Docker, vector DB, or NLE integrations were added.

Next recommended milestone:
- Add optional automatic `agent-result --write-artifact` after successful `execute-plan`.
- Add richer per-run and per-plan next-step suggestions for batch/watch execution paths.
- Add a compact shareable review bundle index for goal-aware export handoff.
- Add optional result summaries that group normal roughcut and goal-aware roughcut side by side.

Errors or assumptions:
- Chose to make `goal-review-bundle` always write its markdown artifact because that is the command’s purpose.
- Assumed plan-to-run association in `inspect <run_id>` can stay best-effort via action `run_id` matching.
- Kept execution semantics unchanged; only result surfacing and artifact writing were added.
<!-- HANDOFF 039 END -->

## Prompt 040 - Creative Capability Registry

<!-- PROMPT 040 START -->
Goal:
- Add a dynamic Creative Capability Registry and provider-agnostic tool backend configuration skeleton.
- Add `tools` config parsing, inspection, validation, requirements detection, and creative planning artifacts.
- Keep local-first and artifact-first.
- Do not call new providers, add cloud execution, add web server, Docker, vector DB, or NLE integrations.
<!-- PROMPT 040 END -->

## Handoff 040

<!-- HANDOFF 040 START -->
What changed:
- Added a new provider-agnostic `tools` config section with dynamic backend and route parsing.
- Added structural tool backend/auth config support under `internal/config`.
- Added `byom-video tools` for human-readable and JSON inspection of creative backends and routes.
- Added `byom-video tools validate` with structural checks, strict mode, and optional env-presence checks.
- Added `byom-video tools requirements --goal "<text>"` for deterministic capability detection from creative goals.
- Added `byom-video creative-plan <input-file> --goal "<text>"` to write deterministic creative planning artifacts without provider calls.
- Added `byom-video creative-plans`, `inspect-creative-plan`, and `review-creative-plan`.
- Added illustrative creative config examples for local-only, placeholder cloud-style, and custom HTTP backends.
- Updated config, security, models, and README docs to document the Creative Capability Registry.
- Added a smoke script for tools inspection, validation, requirements detection, creative plan creation, listing, inspection, and review.

Files added/modified:
- Added `internal/commands/creative_tools.go`.
- Added `internal/commands/creative_tools_test.go`.
- Modified `internal/config/config.go`.
- Modified `internal/config/config_test.go`.
- Modified `internal/commands/config.go`.
- Modified `internal/cli/root.go`.
- Added `docs/creative-tools.md`.
- Added `docs/creative-plans.md`.
- Added `examples/configs/creative-local-only.yaml`.
- Added `examples/configs/creative-openai-elevenlabs-placeholder.yaml`.
- Added `examples/configs/creative-custom-http.yaml`.
- Added `scripts/smoke-creative-tools.sh`.
- Modified `docs/config.md`.
- Modified `docs/security.md`.
- Modified `docs/models.md`.
- Modified `README.md`.
- Modified `PROGRESS.md`.

New commands:
```sh
./byom-video tools
./byom-video tools validate
./byom-video tools requirements --goal "<text>"
./byom-video creative-plan <input-file> --goal "<text>"
./byom-video creative-plans
./byom-video inspect-creative-plan <creative_plan_id>
./byom-video review-creative-plan <creative_plan_id>
```

New config fields:
```yaml
tools:
  enabled: false

  backends:
    local_writer:
      kind: text_generation
      provider: ollama
      model: qwen2.5:7b
      endpoint: http://localhost:11434
      auth:
        type: none
      options:
        temperature: 0.2

    voice_backend:
      kind: voice_generation
      provider: elevenlabs-compatible
      model: voice-model-name
      endpoint: https://api.example.com
      auth:
        type: header_env
        header: xi-api-key
        env: ELEVENLABS_API_KEY

  routes:
    creative.script: local_writer
    creative.voiceover: voice_backend
```

Tools config behavior:
- Backend logical names are fully user-defined.
- Provider strings are fully user-defined.
- Route keys are fully user-defined.
- Known capability kinds are documented, but unknown kinds are warnings by default.
- Auth config supports:
  - `none`
  - `bearer_env`
  - `header_env`
  - `query_env`
  - `basic_env`
- Commands only print env var names, never values.
- No provider calls are made by config show, tools inspection, tools validation, requirements detection, or creative planning.

Tools validation behavior:
- Structural validation only.
- Checks:
  - backend kind present
  - provider present
  - endpoint warnings for non-local backends when missing
  - model warnings for generation-like kinds when missing
  - auth type validity
  - required auth env/header fields
  - route target existence
- Unknown capability kinds are warnings unless `--strict`, where they become errors.
- `--check-env` checks env presence only and never prints values.
- Missing env vars become warnings by default and errors in `--strict`.

Requirements detection behavior:
- Deterministically infers capabilities from goal text.
- Supported examples include:
  - narration / voiceover
  - cinematic short
  - AI b-roll
  - captions
  - object removal
  - translation / Spanish
- Reports capability status as:
  - `satisfied`
  - `partial`
  - `missing`
- Shows matching routes and backends when configured.
- Suggests route names such as `creative.video_broll` when capabilities are missing.

Creative plan behavior:
- `creative-plan` writes:
```text
.byom-video/creative_plans/<creative_plan_id>/creative_plan.json
```
- Planning is deterministic and artifact-only.
- It does not execute tools or call providers.
- Missing capabilities produce warnings and do not block planning unless `--strict` is used.
- `review-creative-plan --write-artifact` writes:
```text
.byom-video/creative_plans/<creative_plan_id>/creative_plan_review.md
```

Commands run:
```sh
gofmt -w internal/config/config.go internal/config/config_test.go internal/commands/config.go internal/commands/creative_tools.go internal/commands/creative_tools_test.go internal/cli/root.go
chmod +x scripts/smoke-creative-tools.sh
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go test ./...
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build -o byom-video ./cmd/byom-video
GOCACHE=/Users/mireliftikharahmed/Documents/BYOMVIDEO/.cache/go-build go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers
scripts/smoke-creative-tools.sh
```

Test results:
- `go test ./...` passed.
- `go build -o byom-video ./cmd/byom-video` passed.
- `go build ./cmd/byom-video` passed.
- `python3 -m compileall -q workers/byom_video_workers` passed.
- `scripts/smoke-creative-tools.sh` passed.
- Smoke showed disabled tools config, successful structural validation, deterministic capability detection, creative plan creation, creative plan listing, inspection, and markdown review artifact writing.

How to run smoke creative-tools test:
```sh
scripts/smoke-creative-tools.sh
```

Known limitations:
- The Creative Capability Registry is config/contracts/planning only in this milestone.
- It does not execute creative tools.
- Cloud-oriented backend examples are illustrative placeholders only.
- Only existing implemented execution providers should be treated as executable.
- Capability detection is deterministic keyword matching, not semantic reasoning.
- No new providers, cloud calls, web server, Docker, vector DB, or NLE integrations were added.

Next recommended milestone:
- Add creative plan approval and execution skeletons that consume the registry without hardcoding providers.
- Add capability-to-artifact review bundles for script, voiceover, and visual generation planning.
- Add backend dry-run request previews for creative tools similar to model request dry-runs.
- Add explicit local-command backend planning contracts.

Errors or assumptions:
- Assumed `tools` should remain separate from `models` because capability routing and model routing solve different problems.
- Assumed planning should still succeed when capabilities are missing unless `--strict` is used.
- Assumed cloud-style example configs should stay clearly documented as illustrative and non-executable in the current implementation.
<!-- HANDOFF 040 END -->

<!-- HANDOFF 041 START -->
## Prompt 041 — Creative Plan Approval + Dry Run Preview

### START

**Goal:** Add an approval gate and dry-run execution skeleton to the creative capability registry. No provider calls are made.

**Implemented:**
- `approve-creative-plan <plan_id>` — patches `approval_status=approved`, `approved_at`, `approval_mode=manual` onto `creative_plan.json`; writes CREATIVE_PLAN_APPROVED event
- `creative-plan-events <plan_id> [--json]` — reads and prints `events.jsonl` for a creative plan
- `creative-preview <plan_id> [--json] [--strict] [--overwrite] [--check-env]` — builds provider-agnostic dry-run request stubs per step; writes `creative_requests.dryrun.json`; events CREATIVE_PREVIEW_STARTED/COMPLETED/FAILED
- `execute-creative-plan <plan_id> [--yes] [--dry-run] [--strict] [--check-env] [--json]` — requires approval (or --yes); runs preview internally; patches `execution_status=dry_run_completed` and `request_preview_artifact`; events CREATIVE_EXECUTION_STARTED/STEP_PREVIEWED/STEP_SKIPPED/COMPLETED/FAILED
- `creative-result <plan_id> [--json] [--write-artifact]` — summarizes approval/execution state; writes `creative_result.md`
- `validate-creative-plan <plan_id> [--json]` — validates `creative_plan.json`, `creative_requests.dryrun.json` (if present), `events.jsonl` (if present)
- Updated `inspect-creative-plan` — now shows approval_status, execution_status, preview artifact path, events path, step statuses
- Updated `review-creative-plan` — now shows approval_status, execution_status, preview artifact path, next suggested commands; --write-artifact includes Next Steps section

**New files:**
- `internal/commands/creative_plan_approval.go` — all new command implementations
- `internal/commands/creative_plan_approval_test.go` — 15 tests (all pass)
- `scripts/smoke-creative-plan-approval.sh` — end-to-end smoke test

**New artifacts per plan:**
```text
.byom-video/creative_plans/<plan_id>/creative_plan.json        (patched with approval/execution fields)
.byom-video/creative_plans/<plan_id>/creative_requests.dryrun.json
.byom-video/creative_plans/<plan_id>/creative_result.md
.byom-video/creative_plans/<plan_id>/events.jsonl
```

**Schema: creative_requests.dryrun.v1**
Each request item includes: step_id, step_type, capability, route, backend, provider, model, endpoint, auth (env var name only, never value), status (previewed|missing_backend), request_preview (instruction, input_summary, output_contract).

**Test results:**
- `go test ./...` — all packages pass
- `scripts/smoke-creative-plan-approval.sh` — passes end-to-end

### HANDOFF

Commands tested in smoke:
1. `creative-plan` — creates plan
2. `creative-plans` — lists plans
3. `inspect-creative-plan` — shows approval_status=pending, execution_status=not_started, step statuses
4. `review-creative-plan --write-artifact` — shows next commands, writes MD artifact
5. `approve-creative-plan` — approval_status=approved, approval_mode=manual
6. `creative-plan-events` — CREATIVE_PLAN_APPROVED event visible
7. `creative-preview` — writes creative_requests.dryrun.json; --overwrite works
8. `execute-creative-plan` — writes execution_status=dry_run_completed
9. `creative-result --write-artifact` — shows approved/dry_run_completed; writes creative_result.md
10. `validate-creative-plan` — status: ok

Known limitations:
- No provider calls are made. This is dry-run planning only.
- `execute-creative-plan` always produces execution_status=dry_run_completed; live provider dispatch is not implemented.
- Request preview payloads are template-generated from step type, not from actual content.
- `--check-env` warns on missing env vars but does not fail unless `--strict` is also set.

Next recommended milestone:
- Add local-command backend execution (shell script dispatch) as the first non-dry-run execution path.
- Add capability-to-artifact mapping so downstream commands can consume preview outputs.
- Add `creative-plan-diff` for comparing two creative plans.
<!-- HANDOFF 041 END -->

<!-- PROMPT 042 START -->
## Prompt 042 — Creative Stub Execution Artifacts

Goal: Add creative stub execution so approved creative plans can produce structured local placeholder artifacts without calling providers or executing shell commands.

Scope: Local-first, artifact-first. No provider calls. No shell execution. No API keys read. Stub outputs only.

Parts implemented:
- Part A: creative-execute-stub command
- Part B: 8 output artifact schemas (script, voiceover, visual assets, captions, audio, visual transform, translation, composition)
- Part C: outputs/creative_outputs.json index (creative_outputs.v1)
- Part D: review-creative-outputs command
- Part E: extend validate-creative-plan to validate outputs
- Part F: update inspect-creative-plan, creative-result, review-creative-plan with output artifact summary
- Part G: update docs/creative-plans.md, README.md; create docs/artifacts/creative-outputs.md
- Part H: 13 tests (all pass)
- Part I: scripts/smoke-creative-stub-execution.sh (passes)
<!-- PROMPT 042 END -->

<!-- HANDOFF 042 START -->
## Handoff 042

### What changed

New file: `internal/commands/creative_stub_execution.go`
- All new types: CreativeOutputsIndex, CreativeOutputArtifact, CreativeScriptOutput, VoiceoverPlanOutput, VisualAssetPromptsOutput, VisualPromptItem, CaptionPlanOutput, AudioAssetPlanOutput, VisualTransformPlanOutput, TranslationPlanOutput, CompositionPlanOutput, CompositionPlanInputs
- New options: CreativeExecuteStubOptions, ReviewCreativeOutputsOptions
- Functions: CreativeExecuteStub, ReviewCreativeOutputs, writeStubArtifact, stubArtifactName, stubArtifactType, expectedArtifactSchemaVersion, visualKindForGoal

Modified: `internal/commands/creative_plan_approval.go`
- ValidateCreativePlan: added validation of outputs/creative_outputs.json (schema, artifacts, path existence, per-artifact schema_version)
- CreativeResult: reads creative_outputs.json if present, shows output artifact count + types, adds creative-execute-stub / review-creative-outputs / validate-creative-plan to next_commands

Modified: `internal/commands/creative_tools.go`
- InspectCreativePlan: reads outputs index if present, shows outputs dir, outputs index path, creative_outputs_review.md path, output artifact count, reads step statuses from map (post-stub patch)
- ReviewCreativePlan: reads outputs index if present, shows output artifact count + list, adds creative-execute-stub / review-creative-outputs to next_commands, write-artifact includes Stub Outputs section

Modified: `internal/cli/root.go`
- Added usage lines for creative-execute-stub and review-creative-outputs
- Added case statements for both new commands
- Added parseCreativeExecuteStubArgs (supports --yes --overwrite --json --dry-run --step-type <type>)
- Added parseReviewCreativeOutputsArgs

New: `internal/commands/creative_stub_execution_test.go` — 13 tests (all pass)
New: `scripts/smoke-creative-stub-execution.sh` — end-to-end smoke (11 steps, passes)
New: `docs/artifacts/creative-outputs.md`
Updated: `docs/creative-plans.md`, `README.md`

### New commands

```sh
byom-video creative-execute-stub <id> [--yes] [--overwrite] [--json] [--step-type <type>] [--dry-run]
byom-video review-creative-outputs <id> [--json] [--write-artifact]
```

### Stub execution behavior

- Reads creative_plan.json, validates schema
- Requires approval_status=approved unless --yes
- --yes auto-approves with approval_mode=yes_flag
- --overwrite required if outputs/ already exists
- --step-type filters to one step type; all other steps marked skipped
- --dry-run prints planned writes, writes nothing
- Creates outputs/ dir, writes per-step artifact file(s)
- Writes outputs/creative_outputs.json index
- Patches creative_plan.json: execution_status=stub_completed, per-step status=stub_completed|skipped|failed, output_artifacts=[list]
- Events: CREATIVE_STUB_EXECUTION_STARTED, CREATIVE_STUB_STEP_COMPLETED, CREATIVE_STUB_STEP_SKIPPED, CREATIVE_STUB_EXECUTION_COMPLETED, CREATIVE_STUB_EXECUTION_FAILED

### Output artifact behavior

| step_type | file(s) | schema_version |
|---|---|---|
| generate_script | script_draft.json + script_draft.txt | creative_script.v1 |
| generate_voiceover | voiceover_plan.json | voiceover_plan.v1 |
| generate_visual_asset | visual_asset_prompts.json | visual_asset_prompts.v1 |
| generate_captions_or_caption_variants | caption_plan.json | caption_plan.v1 |
| generate_audio_asset | audio_asset_plan.json | audio_asset_plan.v1 |
| visual_transform | visual_transform_plan.json | visual_transform_plan.v1 |
| translate_text | translation_plan.json | translation_plan.v1 |
| render_draft | composition_plan.json | composition_plan.v1 |
| unknown | (none, step marked skipped) | — |

### Commands run in smoke

1. creative-plan — creates plan with 5 steps
2. approve-creative-plan — marks approved
3. creative-execute-stub --dry-run — shows planned writes, writes nothing
4. creative-execute-stub — 5 artifacts created, execution_status=stub_completed
5. creative-execute-stub --overwrite — idempotent
6. review-creative-outputs --write-artifact — lists 5 artifacts, writes creative_outputs_review.md
7. creative-result --write-artifact — shows stub_completed, 5 output artifacts, next commands
8. validate-creative-plan — status: ok
9. inspect-creative-plan — shows stub_completed, 5 step statuses, outputs dir, outputs index
10. review-creative-plan --write-artifact — shows output artifact summary, next commands
11. creative-plan-events — APPROVED + STUB_EXECUTION events visible

### Test results

go test ./... — all 28 packages pass
scripts/smoke-creative-stub-execution.sh — passes

### How to run smoke

```sh
bash scripts/smoke-creative-stub-execution.sh
```

### Known limitations

- All stub outputs are template-generated; content is not derived from the actual video or transcript.
- visual_kind in visual_asset_prompts.json is derived from goal keyword matching, not semantic reasoning.
- Only one prompt per generate_visual_asset step. Multi-shot visual prompts not yet supported.
- No provider calls — stub execution is a planning and artifact skeleton only.
- execute-creative-plan (Prompt 041) still produces execution_status=dry_run_completed. creative-execute-stub is the path to stub_completed.

### Next recommended milestone

- Add local-command backend execution (dispatch configured shell scripts) as the first real execution path.
- Add capability-to-artifact mapping so downstream commands (export, clip-cards) can consume stub outputs.
- Add creative-plan-diff for comparing two creative plans.
- Add step dependency ordering to creative_plan.json so render_draft can depend on generate_script.
<!-- HANDOFF 042 END -->

## Prompt 043 - Creative Composition Timeline + Render Plan

<!-- HANDOFF 043 START -->

### What was added

**New commands:**

- `creative-timeline <id> [--run-id <run_id>] [--overwrite] [--json] [--prefer-goal]`
  - Assembles four tracks from stub outputs + optional run clips into `outputs/creative_timeline.json`
  - Tracks: `track_video_main` (video), `track_voiceover` (audio), `track_captions` (text), `track_visual_overlays` (visual)
  - Clip source priority (with `--run-id`): `selected_clips.json`; with `--prefer-goal`: `goal_roughcut.json → enhanced_roughcut.json → roughcut.json → selected_clips.json`
  - Total duration computed from clip durations; empty if no run clips provided
  - Voiceover: single placeholder spanning full duration, notes reference voiceover_plan.json if present
  - Visual overlays: one placeholder per visual prompt from `visual_asset_prompts.json`, or one global placeholder
  - Updates `creative_outputs.json` index; fires events CREATIVE_TIMELINE_STARTED/COMPLETED/FAILED

- `creative-render-plan <id> [--overwrite] [--json]`
  - Converts timeline items to render steps; requires `creative_timeline.json`
  - Operations: `cut_source_clip`, `attach_voiceover_placeholder`, `add_caption_placeholder`, `add_visual_overlay_placeholder`
  - Writes `outputs/creative_render_plan.json` (schema: `creative_render_plan.v1`)
  - `planned_output.planned_file = "outputs/draft.mp4"` in stub mode
  - Updates `creative_outputs.json` index; fires events CREATIVE_RENDER_PLAN_STARTED/COMPLETED/FAILED

- `review-creative-timeline <id> [--json] [--write-artifact]`
  - Prints summary of all tracks and render steps
  - `--write-artifact` writes `outputs/creative_timeline_review.md`, updates index

**Extended commands:**

- `validate-creative-plan` — validates `creative_timeline.json` (schema, tracks, duration) and `creative_render_plan.json` (schema, planned_output, steps)
- `inspect-creative-plan` — shows timeline path, track count, duration, render plan path, step count, planned output
- `creative-result` — adds `creative-timeline`/`creative-render-plan`/`review-creative-timeline` to next_commands when not yet run
- `review-creative-outputs` — shows timeline and render plan paths if present

**New files:**

- `internal/commands/creative_timeline.go` — all types, helpers, and command implementations
- `internal/commands/creative_timeline_test.go` — 16 tests
- `scripts/smoke-creative-timeline.sh` — 13-step smoke test
- `docs/artifacts/creative-timeline.md`
- `docs/artifacts/creative-render-plan.md`

**Updated files:**

- `internal/commands/creative_plan_approval.go` — extended validate, creative-result
- `internal/commands/creative_tools.go` — extended inspect
- `internal/commands/creative_stub_execution.go` — extended review-creative-outputs, expectedArtifactSchemaVersion
- `internal/cli/root.go` — usage lines, switch cases, parse functions for 3 new commands
- `docs/creative-plans.md` — new workflow steps, commands, artifacts, Timeline and Render Plan section
- `README.md` — new table row, new command block entries

### Key helpers added

- `readClipsFromArtifact(path)` — reads clips from any run artifact using `clips`/`items`/`segments` key
- `updateCreativeOutputsIndex(planID, type, path, stepID)` — adds/updates entry in creative_outputs.json, creates index if absent
- `timelineTruncate(s, n)` — local truncate helper (avoids collision with `truncate` in runs.go)
- `jsonFloat(m, key)` — type-safe float extraction from map[string]any

### Type naming note

`CreativeTimelineArtifact` and `CreativeRenderPlanArtifact` are the struct types (function names `CreativeTimeline` / `CreativeRenderPlan` would conflict).

### Test results

go test ./... — all 28 packages pass (16 new tests)
scripts/smoke-creative-timeline.sh — passes

### How to run smoke

```sh
bash scripts/smoke-creative-timeline.sh
```

### Known limitations

- Timeline clips only come from run artifact JSON files; no ffprobe or media inspection in this layer.
- Visual overlay placement is proportional (equal segments) when multiple prompts exist, not scene-aware.
- Timeline duration is 0 when no run clips are provided (stub mode with no `--run-id`).
- No audio waveform analysis; voiceover is a single placeholder spanning the full timeline.
- No rendering — creative_render_plan.json is a planning artifact only.

### Next recommended milestone

- Add `creative-assemble` that applies ffmpeg operations from the render plan to produce an actual draft video.
- Add `--run-id` auto-detection from creative_plan.json when a run was used during planning.
- Add step dependency ordering so `render_draft` depends on `generate_script` in the render plan.
- Add capability-to-artifact mapping so downstream export commands can consume timeline outputs.
<!-- HANDOFF 043 END -->

## Prompt 044 - Creative Assemble v1

<!-- PROMPT 044 START -->
Add Creative Assemble v1: a safe FFmpeg-based render command that turns creative_timeline.json / creative_render_plan.json into a local draft video when source clips are available. Add creative-assemble (reencode/stream-copy, dry-run, max-clips), validate-creative-assemble, review-creative-assemble, result artifact (creative_assemble_result.v1), creative_outputs.json index update, creative_plan.json execution_status=assembled patching, validation/review integration, ffmpeg executor abstraction for testable code, 20 unit tests with fake runner, smoke script, docs.
<!-- PROMPT 044 END -->

<!-- HANDOFF 044 START -->

### What changed

**New commands:**

- `creative-assemble <id> [--overwrite] [--json] [--mode reencode|stream-copy] [--keep-work] [--dry-run] [--max-clips <n>]`
  - Reads `creative_timeline.json` + `creative_render_plan.json`
  - Finds `source_clip` items in `track_video_main` with `source_end > source_start`
  - Cuts clips: reencode (`-c:v libx264 -c:a aac`) or stream-copy (`-c copy`)
  - Writes `outputs/render_work/clip_NNNN.mp4` + `concat_list.txt`
  - Assembles via FFmpeg concat demuxer → `outputs/draft.mp4`
  - Single clip: remuxed directly via ffmpeg (not renamed)
  - Writes `outputs/creative_assemble_result.json` (`creative_assemble_result.v1`)
  - Updates `creative_outputs.json` (adds `draft_video`, `creative_assemble_result` entries)
  - Patches `creative_plan.json.execution_status = "assembled"`
  - Dry-run: prints planned commands, writes nothing
  - Requires ffmpeg on PATH (unless `--dry-run`)
  - Fails cleanly when no source clips: "use creative-timeline --run-id"
  - Work files kept in `render_work/` by default (alpha transparency)
  - Events: CREATIVE_ASSEMBLE_STARTED/CLIP_RENDERED/COMPLETED/FAILED

- `validate-creative-assemble <id> [--json]`
  - Checks schema_version, output_file non-empty, draft.mp4 exists, work clips exist, ffprobe probe if available

- `review-creative-assemble <id> [--json] [--write-artifact]`
  - Reads result, prints clip table, status, mode
  - `--write-artifact` → `outputs/creative_assemble_review.md`, updates index

**Extended commands:**

- `validate-creative-plan` — validates `creative_assemble_result.json` if present (schema, draft existence, work clips)
- `inspect-creative-plan` — shows assemble status, mode, draft output path, assemble review path
- `creative-result` — adds `creative-assemble`/`validate-creative-assemble`/`review-creative-assemble` to next_commands; shows `draft:` field
- `review-creative-outputs` — shows assemble status, mode, draft file when present
- `review-creative-timeline` — shows Assemble section if result exists; adds `creative-assemble` to next commands

**New files:**

- `internal/commands/creative_assemble.go` — all types, runner abstraction, command implementations
- `internal/commands/creative_assemble_test.go` — 20 tests
- `scripts/smoke-creative-assemble.sh` — smoke test (dry-run always; real render if ffmpeg + source clips available)
- `docs/artifacts/creative-assemble.md`

**Updated files:**

- `internal/commands/creative_plan_approval.go` — extended validate-creative-plan, creative-result
- `internal/commands/creative_tools.go` — extended inspect-creative-plan
- `internal/commands/creative_stub_execution.go` — extended review-creative-outputs
- `internal/commands/creative_timeline.go` — extended review-creative-timeline
- `internal/cli/root.go` — usage lines, switch cases, 3 parse functions
- `docs/creative-plans.md` — workflow steps 8–9, commands, artifacts, Creative Assemble section
- `README.md` — new table row, assemble commands

### New files and types

**`creative_assemble.go`:**
- `CreativeAssembleResult` — schema `creative_assemble_result.v1`
- `AssembledClip` — per-clip result with source_path, start, end, work_file, status, error
- `ffmpegRunner` interface — `Run(args []string) ([]byte, error)`
- `realFFmpegRunner` — calls `exec.Command(ffmpegPath, args...)`
- `creativeAssembleWithRunner` — injectable runner for testability
- `buildClipArgs(mode, start, end, input, output)` — builds reencode or stream-copy args slice

### Safety properties

- Source path comes only from `creative_timeline.json.input_path` (plan's original media)
- FFmpeg called via `exec.Command` with arg slices, never shell strings
- No shell metacharacters in any arg path
- Original media never touched
- All writes go to `outputs/` under the plan directory

### FFmpeg sequence (reencode)

```
# Per clip:
ffmpeg -y -ss 0.000000 -to 12.500000 -i /path/to/source.mov -c:v libx264 -c:a aac /plan/outputs/render_work/clip_0001.mp4

# Concat list: outputs/render_work/concat_list.txt
file '/abs/path/clip_0001.mp4'
file '/abs/path/clip_0002.mp4'

# Assemble:
ffmpeg -y -f concat -safe 0 -i concat_list.txt -c copy outputs/draft.mp4

# Single clip (remux):
ffmpeg -y -i clip_0001.mp4 -c copy outputs/draft.mp4
```

### Test results

go test ./... — all 23 packages pass (20 new tests)
scripts/smoke-creative-assemble.sh — passes (dry-run + no-source-clips path)

### How to run smoke

```sh
bash scripts/smoke-creative-assemble.sh
```

For real render test, first build a pipeline run with source clips, then:
```sh
byom-video creative-timeline <plan_id> --run-id <run_id>
byom-video creative-render-plan <plan_id>
bash scripts/smoke-creative-assemble.sh
```

### Known limitations

- Only `track_video_main` source clips are assembled; voiceover/visual overlay placeholders are not rendered.
- No audio mixing — draft.mp4 contains only the cut video audio, not the voiceover.
- No caption burn-in.
- `--keep-work` flag is accepted but always keeps (default); no `--clean-work` option yet.
- Work clip existence is checked during `validate-creative-assemble` but not guaranteed to match actual ffmpeg output if the source file is very short or malformed.
- FFmpeg duration probe uses ffprobe format.duration string only (basic check).

### Next recommended milestone

- Add voiceover mixing: combine draft.mp4 audio with voiceover placeholder audio track.
- Add caption burn-in pass using subtitle filter.
- Add `--clean-work` flag to remove render_work after successful assembly.
- Add ffprobe duration validation against expected total_duration_seconds from the timeline.
- Add `creative-assemble --run-id <id>` shortcut that also runs creative-timeline automatically.
<!-- HANDOFF 044 END -->

<!-- HANDOFF 045 START -->
## Prompt 045 — creative-assemble: captions and voiceover

### What was built

Extended `creative-assemble` with staged post-processing for captions and voiceover.

**New flags:**
- `--burn-captions` — burn SRT captions via FFmpeg `subtitles` filter
- `--captions <path>` — explicit SRT path (auto-discovered from run if omitted)
- `--allow-missing-captions` — skip caption stage if no file found (warn, continue)
- `--mix-voiceover` — mix audio via FFmpeg `amix` filter
- `--voiceover <path>` — explicit audio path (auto-discovered from outputs if omitted)
- `--allow-missing-voiceover` — skip voiceover stage if no file found (warn, continue)
- `--run-id <id>` — used for captions auto-discovery from a pipeline run

**Staged render pipeline:**
`draft_assembled.mp4` → (voiceover) `draft_audio.mp4` → (captions) `draft.mp4`

Final output is always `draft.mp4`. Intermediate stage files live in `render_work/`.

**Extended result schema (`creative_assemble_result.v1`):**
- `final_output_file` — final path after all stages
- `captions` — `{requested, source_path, status: applied|skipped|failed}`
- `voiceover` — `{requested, source_path, status: applied|skipped|failed}`
- `stages` — per-stage `{name, file, status}` records

**Safety additions:**
- `escapeFilterPath()` escapes `\`, `:`, `'` for FFmpeg filter graph without shell involvement
- Caption/voiceover paths validated to exist before any FFmpeg work begins
- Pre-validation fails fast with a clear error; `--allow-missing-*` flags skip gracefully

**Inspect/result/review-outputs** extended to display captions and voiceover status.

### Files changed

| File | Change |
|---|---|
| `internal/commands/creative_assemble.go` | Full rewrite with staged render, new types, new helpers |
| `internal/commands/creative_assemble_test.go` | +11 tests (31 total) |
| `internal/commands/creative_tools.go` | Show captions/voiceover in inspect |
| `internal/commands/creative_plan_approval.go` | Show captions/voiceover in result |
| `internal/commands/creative_stub_execution.go` | Show captions/voiceover in review-outputs |
| `internal/cli/root.go` | Parse 7 new flags for creative-assemble |
| `docs/artifacts/creative-assemble.md` | Full update |
| `docs/creative-plans.md` | Updated command reference and assemble section |
| `README.md` | Added captions/voiceover example |
| `scripts/smoke-creative-assemble-media.sh` | New smoke test (requires BYOM_SMOKE_INPUT) |

### Tests

31 tests in `creative_assemble_test.go` — all pass.

```sh
go test ./internal/commands/ -run "TestCreativeAssemble|TestValidateCreativeAssemble|TestReviewCreativeAssemble|TestEscapeFilter|TestBuildVoiceover|TestBuildCaption" -count=1
```

### Known limitations

- Caption auto-discovery only finds `captions.srt` from a run via `--run-id`; it does not search arbitrary locations.
- Voiceover auto-discovery only looks in `outputs/voiceover.{wav,mp3,m4a,aac}`.
- No `--clean-work` flag to remove render_work intermediates after success.
- No ffprobe duration check on intermediate stage files, only on final `draft.mp4`.

### Smoke test

```sh
# Requires a real video file
BYOM_SMOKE_INPUT=/path/to/clip.mov bash scripts/smoke-creative-assemble-media.sh
```

### Next recommended milestone

- Add `creative-assemble --run-id <id>` shortcut that chains `creative-timeline` automatically.
- Add `--clean-work` flag.
- Add ffprobe duration validation for intermediate stage files.
- Add `--no-audio` flag for caption-only assembly without voiceover.
<!-- HANDOFF 045 END -->
