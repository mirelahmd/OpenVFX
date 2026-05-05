# Examples

This directory contains:

- `configs/`: example BYOM model-routing shapes
- `workflows/`: command walkthroughs for common alpha flows
- `fixtures/`: tiny local media fixtures when available

Provider notes:

- `configs/local-only.yaml` matches the current implemented local execution path.
- `configs/openai-ollama.yaml`, `configs/groq-local.yaml`, and `configs/custom-http.yaml` are future illustrative routing examples only.
- Only local Ollama execution exists right now.

Generate a tiny local fixture when FFmpeg is installed:

```sh
scripts/make-fixture.sh
./byom-video pipeline examples/fixtures/tiny.mp4 --preset metadata
```
