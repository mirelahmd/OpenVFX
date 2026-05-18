# Queue Artifacts

Queue health reports live under:

```text
.byom-video/queue/
  queue_summary.json
  queue_health.md
```

They are written by:

```sh
./byom-video queue health --write-report
```

## queue_summary.json

Schema version:

- `openvfx_queue_summary.v1`

Fields include:

- daemon summary
- worker summary
- job counts and attention lists
- health checks
- next command suggestions

## queue_health.md

Markdown report of the same runtime state for quick review.
