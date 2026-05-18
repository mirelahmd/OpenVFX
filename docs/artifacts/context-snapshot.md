# Context Snapshot Artifact

`context_snapshot.json` is a cache-like observation artifact for the planner.

Schema version:

- `openvfx_context_snapshot.v1`

Properties:

- `safe_to_delete: true`
- `ttl_days: 7`

It summarizes:

- input media presence
- style pack presence
- queue/runtime summary
- high-level capabilities

It should stay small and disposable.
