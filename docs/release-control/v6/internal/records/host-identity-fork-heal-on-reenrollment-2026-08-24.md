# Host identity fork is permanent after re-enrollment (2026-08-24)

## Context

PR #1715 (landed trimmed as `0e8f63cdd`) reported and fixed the visible
symptom: the Discovery tab said "No saved discovery run" for hosts that had
been analyzed, because discovery records were stored under a forked host
identity while the PVE node kept reporting the base agent UUID as
`LinkedAgentID`. The lookup side now resolves equivalent spellings
(`equivalentDiscoveryTargetIDs` in `internal/servicediscovery/service.go`),
but the identity itself remains forked.

## Root cause of the fork's permanence

`internal/monitoring/monitor_agents.go` forks a host onto `<base>-<hex>`
when an incoming report collides with an existing record under a different
token (clone safety, #1584). Re-running an agent install with `--proxmox` on
the same machine is enough: same machine, same hostname, new token.

The only heal path, `hostRenameHealSource` (monitor_agents.go), requires all
of:

- the same reporting token,
- a **changed** hostname (`hostAgentHostnamesMatch` returning false),
- no report for 3 health windows.

A re-enrollment fork has a **new** token and the **same** hostname, so it
fails two of the three conditions structurally — the fork is held open
forever, not merely until the old record goes quiet. The #1667 heal only
covers the rename case it was built for.

## Why this is a governed gap rather than a quick fix

Every identity consumer downstream of the fork must either resolve
equivalent spellings forever (as discovery now does) or silently miss
records. Healing at the source is a host-identity lifecycle change in clone
territory: rebinding must not merge genuine clones that share a machine ID,
which is the exact hazard #1584 forked to avoid. The machine-id evidence
already carried on host records, and the cross-estate veto signals from the
#1753 fix (`53ba9786c`), plausibly give a re-enrollment enough evidence to
rebind the existing base identity instead of forking — but that needs an
owned slice with clone-safety regression coverage, not an opportunistic
patch.

## Affected spellings observed in the field (from PR #1715)

- `<uuid>-bffd0339` — single fork.
- `<uuid>-d3b-83adbde6` — twice-forked: a base over 40 characters is
  truncated before the next suffix lands, leaving an amputated first suffix
  plus a full second one.

## Prevention boundary landed (2026-08-27)

New re-enrollments no longer fork merely because the retiring agent delivered
one final in-flight report after the replacement install token was created.
`staleHostIdentityForReenrollment` now admits that overlap for at most the old
agent's own health window, and only for a Pulse-issued install token with one
unambiguous same-machine, equivalent-hostname candidate. A changed non-empty
report IP, an active identity conflict, an arbitrary API token, multiple
matching records, or longer overlap still takes the clone-safe fork. The
handoff also retires bindings for the superseded token so a late old process
cannot reclaim the stable identity.

Regression coverage is in `TestInstallHandoffReusesIdentityAcrossFinalInFlightReport`
and `TestInstallHandoffRequiresTrustedUnambiguousIdentityEvidence` alongside
the existing #1654 lifecycle cases.

This is the source-side prevention slice. Hosts already persisted under one or
more suffixed identities still require a separately governed migration that
preserves identity-keyed profile, metadata, availability, history, and alert
state. The coverage gap therefore remains triaged rather than being declared
closed by prevention alone.
