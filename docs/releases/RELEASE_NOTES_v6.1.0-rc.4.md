# Pulse v6.1.0-rc.4 Release Notes

`v6.1.0-rc.4` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.0.5` and supersedes `v6.1.0-rc.3` with a canonical
Operational Trust workflow, clearer protection and availability monitoring,
report-only Unified Agent observer destinations, and fixes from continued
release-candidate testing.

## Highlights

- Patrol now brings protection gaps, availability failures, and actionable
  findings into one attention workbench instead of scattering the user's next
  steps across unrelated pages.
- Operational Trust actions use one reviewed lifecycle from evidence and policy
  through approval, execution, receipt, and independent verification.
- One Unified Agent can report the same collected inventory to a primary Pulse
  server and additional observer destinations without giving observers command
  or remote-configuration authority.
- Protection and availability state is attached to the resources users already
  monitor, with clear evidence and direct routes into the relevant Patrol work.
- Alert activation, notification, and resolution state now share one canonical
  boundary, reducing stale or contradictory attention state.

## Added

- A canonical Operational Trust lifecycle records detector evidence,
  notification linkage, policy provenance, reviewed plans, execution receipts,
  verification, and recovery state.
- The Patrol attention workbench groups protection, availability, alert, and
  governed-action work into a task-focused queue.
- Unified resource projections now carry protection posture and availability
  facets across supported infrastructure types.
- Governed Docker restart work can move from a Patrol finding through review,
  dispatch, durable receipt, and independent outcome verification.
- Unified Agents can be configured with report-only observer destinations.
  Each destination has its own identity, token, retry state, health metric, and
  explicit plaintext-HTTP policy.
- Pulse Mobile chat can propose typed work while keeping approval mandatory.

## Improved

- Patrol and monitored-resource surfaces lead with what needs attention and
  keep forensic detail behind the relevant finding or resource.
- The Assistant panel can be resized while its composer remains usable.
- Workload tables select their wider column set from the canonical responsive
  breakpoint instead of overflowing narrower shells.
- Container updates use one concise review step while retaining the governed
  action and verification contract.
- Backup tables keep their view control fixed under the pointer and align
  coverage rows with the by-date backup table.
- Pulse Mobile improves conversation loading, keyboard avoidance, large-text
  header controls, source renaming, and conversation deletion.

## Fixed

- Resolved-alert maps are guarded on canonical evaluation paths, preventing
  unsafe concurrent access while alert state changes.
- High-percentage thresholds remain evaluable when their critical cap collides
  with another configured boundary.
- PBS datastores are deduplicated across identity formats, and directory
  storages whose names start with `pbs-` retain their vzdump backups.
- Docker agent re-enrollment supersedes stale host records instead of leaving
  duplicate or misleading inventory.
- Remember-me browser sessions survive closing and reopening a tab.
- Docker container update review no longer repeats the same confirmation
  ceremony.
- Recovered integration journeys now exercise the current cookie-session,
  infrastructure-onboarding, Assistant, storage, and commercial surfaces.

## Security

- Observer destinations cannot inherit the primary server's process-wide
  plaintext override. Every non-loopback HTTP observer requires its own
  explicit opt-in.
- Observer acknowledgements cannot change the agent's authoritative
  configuration, and observer destinations do not receive command authority.
- Operational Trust dispatch stays bound to the reviewed plan, policy
  provenance, action identity, and independently verified terminal outcome.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.1.0-rc.4` only when you are
comfortable testing an RC. The rollback target is `v6.0.5`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.0.5
```

Existing Unified Agent configurations continue to use one authoritative Pulse
server. Observer reporting is opt-in through the new destination configuration;
no observer is created during upgrade.

Pulse Mobile iOS candidate build 10 and Android candidate versionCode 8 remain
the compatible store candidates. Candidate-channel update group
`9b78b108-2586-4b0f-91d3-afbed19b49b3`, built from mobile commit
`fddb091e683e84902de6aac680b08a47862b738b`, delivers the matching JavaScript
bundle to both platforms at runtime version 1. No public mobile-store rollout
is part of this RC.

Windows Unified Agent binaries retain checksum and detached-signature
verification, but they are not yet Authenticode-signed and Windows may show an
unknown-publisher warning. Public Windows Authenticode signing remains required
before stable promotion.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
