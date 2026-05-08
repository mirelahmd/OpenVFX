# Limitations

## Alpha status

- alpha software — schemas and commands are still evolving
- no stable API guarantee until v1.0

## Install

- `go install` requires the GitHub repo name to match the module path (`github.com/mirelahmd/byom-video`)
- reusing Git tag names after public release causes the Go module proxy and GitHub CDN to serve stale cached zips — always create a new tag name
- the install script requires `git` to clone the Python worker source

## Dependencies

- `ffmpeg` and `ffprobe` are required for core media workflows
- `faster-whisper` is optional but required for real local transcription
- Python 3.10+ required for any transcription workflow

## Model providers

- only local Ollama expansion is implemented
- no cloud provider execution (OpenAI, Anthropic, Groq, etc.)
- `--goal` text in agent plans is stored but does not influence highlight selection or roughcut decisions — real LLM integration is needed

## Features not yet present

- no web UI
- no DaVinci Resolve or Premiere integration
- no Docker workflow
- no automatic destructive edits without explicit flags
- watch mode is polling-based
- no NLE packaging yet
