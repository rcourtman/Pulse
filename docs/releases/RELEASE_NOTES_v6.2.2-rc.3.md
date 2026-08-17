# Pulse v6.2.2-rc.3 Release Notes

`v6.2.2-rc.3` is a release candidate for the next Pulse v6 patch. It follows
stable `v6.2.1` and supersedes `v6.2.2-rc.2`. This candidate includes the
complete earlier packet and introduces the Patrol v2 operating model, guarded
agent action preflight, large-estate response improvements, and monitoring
correctness fixes.

## Highlights

- Patrol now works from durable outcomes, scoped investigations, and recent
  verified-work receipts.
- Read-only observers extend Patrol coverage between full model investigations
  without granting mutation authority.
- Approved actions gain agent preflight and stable refusal telemetry; large
  installations gain compressed APIs and indexed lookups.

## Added

- Durable Patrol objectives with pause, archive, resource-scope, coverage, and
  observer-health state.
- Model-authored observer proposals that are validated, installed as bounded
  read-only checks, and kept separate from action authority.
- Verified Patrol work receipts and clearer navigation between findings,
  objectives, attention items, and governed actions.
- Unified Agent preflight contracts for package updates, package-cache cleanup,
  and Docker lifecycle or update operations.
- Production security deployment guidance and a focused security-review packet.

## Improved

- Patrol investigations preserve objective and resource intent across retries,
  provider interruptions, chat restarts, truncated responses, and retained
  objective runs.
- Finding identity, evidence, causal conclusions, and remediation proposals are
  canonicalized and validated before Patrol writes or acts on them.
- Autonomous execution remains bounded by advertised capabilities, explicit
  policy, agent preflight, current target state, and post-action verification.
- Action refusal telemetry now classifies target changes, prerequisites,
  contract failures, capability limits, policy decisions, and stale plans
  instead of collapsing the new agent reason codes into the catch-all bucket.
- API gzip handling preserves informational and bodyless responses, while
  polling, metric lookup, registry resolution, and source-target mapping avoid
  repeated large-estate scans.
- OpenRouter, Ollama, and subscription-backed Patrol routes handle reasoning
  limits, readiness checks, deadlines, and continuation latency more reliably.
- Subscription-backed turns now complete their idle timeout promptly even when
  a canceled CLI descendant still holds an inherited output pipe open.

## Fixed

- Patrol no longer accepts empty, blocked, contradicted, incoherent, unscoped,
  or unsupported findings and proposals as successful investigation output.
- Enabling full AI mode or restarting the chat provider now preserves and
  rewires Patrol controls and investigation dependencies.
- Docker health-check dependencies and app-container scope now remain attached
  to the correct canonical findings.
- Stale ZFS alerts clear when storage loses its pool attachment, and node-local
  ZFS pools are no longer attached to shared storage records.
- vSphere backup status, agent thermal history, explicit cluster-member address
  overrides, and discovery-analysis request timeouts now reflect their actual
  runtime state.

## Security

- Patrol observers are read-only and proposal-only. Mutating operations still
  require the normal action-policy, approval, capability, preflight, and
  verification path.
- Agent preflight responses expose bounded machine reason codes rather than
  command output, paths, package names, or provider-specific error text.
- Investigation tooling is projected from the selected resource scope and
  rejects tools that were not advertised for that run.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.2-rc.3` only when you are
comfortable testing a release candidate. The rollback target is `v6.2.1`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.2.1
```

The changes since `v6.2.2-rc.2` do not require a Pulse Mobile client change and
preserve the existing mobile, Relay, onboarding, and mobile-facing API
contracts. No companion mobile build upload or public mobile-store rollout is
part of this candidate.

Windows Unified Agent binaries in this prerelease retain exact-SHA, checksum,
and detached-signature verification but are not Authenticode-signed, so Windows
may display an Unknown Publisher warning. Stable `v6.2.2` still requires the
normal SignPath Authenticode lane unless a separate version-bound owner decision
is recorded.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
