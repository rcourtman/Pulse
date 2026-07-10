# Pulse v6.0.6-rc.1 Release Notes

`v6.0.6-rc.1` is a release candidate for the next Pulse v6 patch line. It
follows stable `v6.0.5` with a larger monitor-first product update, a typed
Pulse Intelligence action lifecycle, safer native-agent update and recovery
behavior, and broad security and reliability hardening.

## Added

- Pulse Intelligence now has explicit detection and investigation profiles,
  a typed proposal-to-action lifecycle, and post-action verification for
  supported Docker and Kubernetes operations.
- Patrol action state now reconciles from the authoritative action audit and
  stays current across investigation history, desktop approval controls, and
  Pulse Mobile approve or reject flows.
- Local AI setup includes a guided Ollama quickstart for `qwen3:8b`, with
  clearer Provider & Models readiness guidance.
- Cluster members can override their connection addresses when the discovered
  address is not the one Pulse should use.
- The Unified Agent Windows service now writes owner-controlled rotating logs,
  verifies logged readiness during installation, and carries native lifecycle
  proof for install, replacement, recovery, persistence, and uninstall.

## Improved

- Platform and connected-system pages lead with monitor-first attention and
  task-oriented workflows, with more coherent responsive and mobile layouts.
- Pulse Intelligence settings and daily-use surfaces use one consistent
  product vocabulary while keeping Patrol focused on detection and
  investigation.
- The provider MSP portal uses the product design system, supports dark mode,
  and presents self-service behavior honestly when an email provider is not
  configured.
- Update execution now uses one canonical lifecycle with clearer completion,
  rollback, and operator feedback.
- Investigation prompts receive the real typed capability catalog, including
  approval requirements and parameter constraints, instead of asking the
  model to guess which actions are available.

## Fixed

- Native updates self-test the replacement binary before swapping it in,
  reject silent edition downgrades, preserve a sanctioned rollback path, and
  fail fast when signing configuration is incomplete.
- Docker updates now recreate the container instead of attempting a restart
  that cannot apply a new image.
- Docker containers retain their grouped-by-host view and open configured web
  links consistently after REST resource snapshot hydration.
- Docker and Kubernetes agents tolerate realistic clock skew when evaluating
  liveness, and posture alerts no longer ignore the intended guest-suppression
  rules.
- Legacy OIDC callbacks recover the initiating provider correctly.
- Provider/runtime failures and proposal-validation failures remain separate,
  so a failed investigation cannot be misreported as a completed
  needs-attention result.
- First-run, request parsing, storage, cookie, remediation-lock, and remote
  deployment boundaries now fail closed across the hardened paths included in
  this candidate.
- FreeBSD agent update recovery and Windows service recovery now preserve a
  usable runtime across replacement and restart paths.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.0.6-rc.1` only when you are
comfortable testing an RC. The rollback target for this release candidate is
`v6.0.5`.

This candidate changes authentication and native installer/updater boundaries,
so it is intentionally using the governed RC path rather than the direct
stable-patch path.

Pulse Mobile candidate builds with runtime version 1 receive the matching
typed-action approval client through the candidate OTA channel; no public store
rollout is part of this RC.

Windows Unified Agent binaries in this release candidate retain the same
checksum and detached-signature verification used by `v6.0.5`, but they are
not yet Authenticode-signed and Windows may show an unknown-publisher warning.
Public Windows Authenticode signing remains required before stable promotion.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
