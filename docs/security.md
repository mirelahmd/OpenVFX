# Security

BYOM Video is designed for local-first operation.

Current properties:

- runs, plans, masks, and reports are written to local disk
- real cloud provider calls are not implemented
- local Ollama execution is explicit and opt-in
- original input media is not modified
- cleanup requires explicit command flags and confirmation for deletion

Operational caveats:

- exported media and generated reports may contain sensitive content if your source media does
- local shell scripts such as `ffmpeg_commands.sh` and `ffmpeg_concat.sh` are generated for inspection before execution
- watch mode is polling-based and uses local registry files
