# Policy Review Artifact

`policy_review.json` evaluates a proposed agent plan against local execution policy.

Schema version:

- `openvfx_policy_review.v1`

It records:

- overall policy status
- approval/provider/network/overwrite requirements
- checks
- warnings
- errors
- next commands

Conversion respects policy review:

- blocked plans are refused unless explicitly forced
- provider-required actions require `agent-plan-to-job --allow-provider-calls`
- overwrite-required actions require `agent-plan-to-job --allow-overwrite`
- make and revise actions require plan approval before conversion unless `--yes` is used
