# Pulse v6.1.0-rc.1 Release Notes

`v6.1.0-rc.1` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.0.5` with a substantial monitor-first product update, a
typed Pulse Intelligence action lifecycle, a dedicated Actions workspace,
safer native-agent update and recovery behavior, and broad security and
reliability hardening.

## Highlights

- Patrol findings can now move through one reviewed Actions inbox with clear
  approval, execution, and verification state.
- Pulse can safely carry out a wider set of explicitly governed Docker,
  Proxmox, host-update, package-maintenance, and storage-cleanup actions.
- Platform pages, connected systems, responsive layouts, and Assistant
  conversations are more task-focused and easier to operate day to day.
- Native updates, agent recovery, authentication boundaries, and service
  hardening fail closed across more installation and recovery paths.

## Added

- Pulse Intelligence now has explicit detection and investigation profiles,
  a typed proposal-to-action lifecycle, and post-action verification for
  supported Docker and Kubernetes operations.
- Patrol action state now reconciles from the authoritative action audit and
  stays current across investigation history, desktop approval controls, and
  Pulse Mobile approve or reject flows.
- Actions provides a dedicated inbox for reviewing proposed work, checking
  policy and verification details, and seeing pending approvals without
  searching through Assistant history.
- Patrol can authorize low-risk Docker and Podman restarts through explicit
  per-resource capability allowlists and optional recurring maintenance
  windows, while unsupported, out-of-window, or downgraded-mode actions remain
  approval-gated and fail closed.
- Governed host updates, Debian and Ubuntu package maintenance, storage-pressure
  cleanup, and supported Proxmox guest lifecycle operations now use reviewed
  plans, durable execution receipts, and independent outcome verification.
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
- Assistant conversations can be retried, regenerated, edited and resent, and
  steered while a response is running. Long pasted input is collapsed into a
  manageable composer attachment, and the last-turn summary reports estimated
  model cost when available.
- Patrol handoffs now open the related Actions review directly, background
  Patrol work stays out of the Assistant quick-resume list, and the Actions tab
  shows its pending-approval count.
- Docker, Kubernetes, TrueNAS, vSphere, and Proxmox node tables preserve
  user-controlled column sorting through one shared platform-table model.

## Fixed

- Native updates self-test the replacement binary before swapping it in,
  reject silent edition downgrades, preserve a sanctioned rollback path, and
  fail fast when signing configuration is incomplete.
- Native updates fail closed instead of silently falling back to a community
  build, preserve writable configuration backups under hardened services, and
  publish verification keys in the exact OpenSSH `allowed_signers` form used
  by the documented verification command.
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
- Cluster re-registration preserves an operator-selected member address, moved
  guests keep their alert ownership aligned with the new node, and unavailable
  guest-agent disk data is no longer presented as a real measurement.
- Physical disks no longer disappear on wide node layouts, standby SSDs no
  longer report misleading state, shared Docker network namespaces survive
  container updates, and SSO administrators retain the expected settings
  authority.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.1.0-rc.1` only when you are
comfortable testing an RC. The rollback target for this release candidate is
`v6.0.5`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.0.5
```

This candidate changes authentication and native installer/updater boundaries,
so it is intentionally using the governed RC path rather than the direct
stable-patch path.

Pulse Mobile iOS candidate build 8 and Android candidate versionCode 7 carry
the matching plan-bound action review and approval client. They remain on the
TestFlight and Google Play internal-testing tracks; no public store rollout is
part of this RC.

Windows Unified Agent binaries in this release candidate retain the same
checksum and detached-signature verification used by `v6.0.5`, but they are
not yet Authenticode-signed and Windows may show an unknown-publisher warning.
Public Windows Authenticode signing remains required before stable promotion.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
