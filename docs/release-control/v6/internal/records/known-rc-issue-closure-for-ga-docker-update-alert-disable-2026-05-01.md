# Known RC Issue Closure For GA Docker Update Alert Disable Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

The v5 maintenance delta audit found that `#1355` had not been fully carried
into v6. Pulse v5 cleared Docker image-update alerts when Docker update alerts
were disabled globally or for a specific container, but the v6 alerts runtime
could preserve the active alert and first-seen tracking after the setting no
longer permitted that alert to fire.

That meant an operator could disable Docker update alerts, or disable alerting
for a single Docker container, and still see stale image-update alert state
survive into the v6 candidate.

## Disposition

The fix is in the alerts runtime Docker update-alert lifecycle:

- `UpdateConfig` now clears active Docker container update alerts and both
  resource and identity first-seen tracking when Docker update alerts are
  disabled.
- Active-alert reevaluation now treats Docker update alerts as lifecycle alerts,
  not generic threshold alerts, so disabled Docker containers, ignored prefixes,
  and disabled update-alert settings resolve stale update incidents.
- Per-container disabled overrides now clear the canonical image-update alert,
  the legacy update alias, and update tracking before skipping evaluation.
- The enabled path preserves existing update alerts and first-seen tracking
  when unrelated Docker threshold settings change.

## Proof

- `go test ./internal/alerts -run 'Test(UpdateConfig(Clears|Keeps)DockerContainerUpdateAlerts|EvaluateDockerContainerClearsUpdateAlert|DockerUpdateTracking)' -count=1`
- `go test ./internal/alerts -run 'TestCheckDockerContainerImageUpdate' -count=1`
- `go test ./internal/alerts -run 'Test.*Docker.*Update|TestCheckDockerContainerImageUpdate|TestUpdateConfig' -count=1`
- `go test ./internal/alerts -count=1`

## Outcome

The v6 candidate no longer knowingly regresses v5 `#1355`. Docker image-update
alerts are removed when the owning configuration disables them, and the runtime
does not leave stale first-seen tracking behind that could resurrect an already
disabled update alert.
