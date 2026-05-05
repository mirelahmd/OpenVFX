# Release Checklist

- run `go test ./...`
- run `go build ./cmd/byom-video`
- run `python3 -m compileall -q workers/byom_video_workers`
- run `scripts/release-smoke.sh`
- optionally run `scripts/release-smoke.sh --with-ollama`
- review README and docs index for broken references
- verify `byom-video version`
- confirm sample workflows in `examples/workflows/`
- confirm issue templates and PR template exist
- review known limitations before tagging
