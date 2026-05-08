# Release Checklist

## Pre-release checks

- run `go test ./...`
- run `go build ./cmd/byom-video`
- run `python3 -m compileall -q workers/byom_video_workers`
- run `scripts/release-smoke.sh`
- optionally run `scripts/release-smoke.sh --with-ollama`
- verify `byom-video version` prints expected version/commit/build date

## Artifact hygiene

- run `git status --ignored` and confirm no local run artifacts are staged
- confirm `.byom-video/` is not tracked: `git ls-files .byom-video/` should be empty
- confirm `media/` is not tracked: `git ls-files media/` should be empty
- confirm built binary is not tracked: `git ls-files byom-video` should be empty

## Module path policy

- module path must remain lowercase: `github.com/mirelahmd/byom-video`
- module path must match the GitHub repo name for `go install` to work
- **never reuse a git tag after pushing it publicly** — the Go module proxy and GitHub CDN cache zips by tag name; deleting and recreating the same tag does not invalidate those caches
- always use a new incremented tag name for each release

## Docs review

- review README.md and docs/ for broken links and command drift
- confirm sample workflows in `examples/workflows/` match current commands
- confirm issue templates and PR template exist
- review known limitations before tagging

## Install script

- test `scripts/smoke-external-install.sh` passes or skips cleanly
- confirm `install.sh` works from a clean directory with no existing config
