# Architecture

BYOM Video is an artifact-first local workflow controller.

Core layers:

1. Media inspection and deterministic pipeline artifacts.
2. Operational commands for batch, watch, retry, cleanup, export, and validation.
3. Deterministic agent planning and inference-mask planning.
4. Local model-router abstractions with dry-run, stub, and Ollama-local execution.
5. Editor-facing and export-facing handoff artifacts.

The main design rule is unchanged:

> Files on disk are the contract.

The system favors inspectable JSON, markdown, shell scripts, and reports over hidden state.
