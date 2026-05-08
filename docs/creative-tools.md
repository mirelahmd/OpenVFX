# Creative Tools

BYOM Video now supports a provider-agnostic creative tool registry under `tools:` in `byom-video.yaml`.

This layer is for config, validation, requirements detection, and planning only. It does not call providers.

## Design

- backends are logical names chosen by the user
- providers are freeform strings
- routes map creative tasks to logical backends
- secrets stay in environment variables
- config and inspection commands only print env var names, never values

## Example

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

## Commands

```sh
./byom-video tools
./byom-video tools --json
./byom-video tools validate
./byom-video tools validate --strict
./byom-video tools validate --check-env
./byom-video tools requirements --goal "make a cinematic short with narration and AI b-roll"
```

`tools validate` is structural only. `--check-env` checks whether referenced env vars exist, but never prints values.

## Capability Kinds

Known kinds are:

- `text_generation`
- `voice_generation`
- `image_generation`
- `video_generation`
- `caption_generation`
- `audio_generation`
- `music_generation`
- `sound_effect_generation`
- `audio_cleanup`
- `object_removal`
- `style_transfer`
- `translation`
- `render_composition`
- `local_command`
- `custom`

Unknown kinds produce warnings by default and errors in `--strict` mode.

## Important Limits

- these configs do not enable new providers by themselves
- only existing implemented execution backends remain executable
- current real model execution is still local Ollama only
- cloud-oriented examples are illustrative placeholders
