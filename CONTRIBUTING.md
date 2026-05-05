# Contributing

Scope for the alpha:

- keep changes local-first and artifact-first
- prefer additive schema/version changes over hidden behavior changes
- avoid introducing cloud provider execution unless explicitly scoped

Basic workflow:

1. run `go test ./...`
2. run `go build ./cmd/byom-video`
3. run `python3 -m compileall -q workers/byom_video_workers`
4. run the relevant smoke script
5. update docs and `PROGRESS.md` when behavior changes

Please include:

- what changed
- why it changed
- any artifact/schema impact
- commands run
