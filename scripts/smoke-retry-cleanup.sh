#!/usr/bin/env bash
set -euo pipefail

batch_id="smoke-retry-failed"
batch_dir=".byom-video/batches/$batch_id"
mkdir -p "$batch_dir"

cat > "$batch_dir/batch_summary.json" <<JSON
{
  "schema_version": "batch_summary.v1",
  "batch_id": "$batch_id",
  "created_at": "2026-04-29T00:00:00Z",
  "input_dir": "$(pwd)/media",
  "preset": "metadata",
  "recursive": false,
  "dry_run": false,
  "totals": {
    "discovered": 1,
    "attempted": 1,
    "succeeded": 0,
    "failed": 1,
    "skipped": 0
  },
  "items": [
    {
      "input_path": "$(pwd)/media/does-not-exist-for-retry.mp4",
      "status": "failed",
      "run_id": "",
      "run_dir": "",
      "error": "simulated failure"
    }
  ]
}
JSON

./byom-video retry-batch "$batch_id" --dry-run
./byom-video cleanup

echo "Manual destructive cleanup examples, not run by this smoke test:"
echo "  ./byom-video cleanup --failed --delete"
echo "  ./byom-video cleanup --failed --delete --yes"
