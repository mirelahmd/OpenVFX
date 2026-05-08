# Configuration

BYOM Video looks for `byom-video.yaml` in the current directory.

Create one with:

```sh
./byom-video init
```

Recreate the default config without deleting run history:

```sh
./byom-video init --force
```

## Example

```yaml
project:
  name: byom-video-project

python:
  interpreter: .venv/bin/python

transcription:
  enabled: true
  model_size: tiny

captions:
  enabled: true

chunks:
  enabled: true
  target_seconds: 30
  max_gap_seconds: 2.0

highlights:
  enabled: true
  top_k: 10
  min_duration_seconds: 3
  max_duration_seconds: 90

roughcut:
  enabled: true
  max_clips: 5

ffmpeg_script:
  enabled: true
  output_format: mp4

report:
  enabled: true

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

## Behavior

- If `byom-video.yaml` exists, `run` uses it for defaults.
- CLI flags override config values.
- Unknown config fields are ignored.
- The parser intentionally supports only the documented simple config shape.
- Model config is parsed and displayed only. It does not add provider clients or model routing execution.

## Python

`python.interpreter` controls the Python executable used for workers unless `BYOM_VIDEO_PYTHON` is set directly in the environment.

For local transcription:

```yaml
python:
  interpreter: .venv/bin/python
```

## BYOM Models

Model configuration is disabled by default:

```sh
./byom-video config show
./byom-video config show --json
./byom-video models
./byom-video models --json
./byom-video models validate
./byom-video models validate --json
```

The commands show logical model entry names, provider type, model, role, optional `base_url`, and `api_key_env` names. They never print environment variable values and never call provider APIs. Provider strings are freeform; examples are illustrative.

## Creative Tools

The `tools` section is a provider-agnostic creative capability registry. It is for config, validation, and planning only.

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

  routes:
    creative.script: local_writer
    creative.captions: local_writer
```

Use:

```sh
./byom-video tools
./byom-video tools --json
./byom-video tools validate
./byom-video tools validate --strict
./byom-video tools validate --check-env
./byom-video tools requirements --goal "make a cinematic short with narration"
```

Known capability kinds are documented, but unknown kinds are warnings by default and strict errors only when `--strict` is used.

This layer does not call providers. Secret values should stay in environment variables. Inspection commands only print env var names.
