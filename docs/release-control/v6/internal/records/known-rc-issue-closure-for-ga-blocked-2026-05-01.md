# Known RC Issue Closure For GA Blocked Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `blocked`

## Context

The RC3 maintenance audit compared the active v5 maintenance line against
`pulse/v6-release` and reviewed recently updated GitHub issues, comments, and
discussion signals before cutting another v6 release candidate.

## Fixed In The Current RC3 Patch

1. v5 `#1444`: alert cooldown set to disabled no longer means "notify every
   metric tick." The v6 alert cooldown gate now sends the first notification for
   an alert occurrence and suppresses subsequent re-notifications while
   cooldown remains disabled.
2. v5 `#1440`: unsaved SMTP test-send settings no longer mutate or inherit the
   shared production email manager when the SMTP/auth transport differs. This
   prevents relay-mode tests from leaking stale saved authentication.
3. v5 DOMPurify security bump: the v6 frontend lockfile now resolves
   `dompurify` to `3.4.1`.
4. `#1451` bootstrap-token confusion: the root installer no longer prints the
   encrypted `.bootstrap_token` file contents as the user-facing token. It uses
   the canonical `pulse bootstrap-token` command with the install data directory
   instead.

Proof run during the audit:

- `go test ./internal/alerts ./internal/notifications`
- `go test ./scripts/installtests`
- `bash -n install.sh`
- `npm --prefix frontend-modern ls dompurify --package-lock-only`

## v5 Fixes Already Carried By v6

The v5 maintenance audit found that several recent v5 fixes already have v6
equivalents on `pulse/v6-release`, including QNAP update/boot continuity,
unified host/docker row merging, mdstat operation gating for RAID rebuild
alerts, stable agent identity files, linked guest filesystem display,
unreachable-guest snapshot carry-forward, update progress modal closability,
Ollama `keep_alive=30s`, and SSE EOF tool-call finalization.

## Remaining RC3 Triage Candidates

1. `#1441` affects `6.0.0-rc.2`: the reporter's screenshot shows the Proxmox
   settings row reporting `Online` while the cluster popover reports Proxmox
   offline for the same nodes. This is a v6 RC2 user-visible trust issue and
   needs a root status-model fix, invalidation proof, or explicit supersession
   before this gate can be called clear again.
2. `#1452` affects `5.1.28`: screenshots show graph tooltip content covering
   the hovered chart point and surrounding graph detail. v6 shares the history
   chart tooltip model, so RC3 should either fix the shared tooltip placement or
   prove the v6 surface is not affected with browser evidence.
3. `#1448` discussion: PBS alert thresholds were reported as firing despite the
   threshold being off. The alert cooldown fix may reduce notification spam, but
   this still needs threshold-specific triage if reproducible on v6.
4. `#1435` release sequencing: the stable LXC installer branch now filters
   prereleases correctly, and GitHub latest points at `v5.1.28`, but the
   affected release asset is not repaired for stable users until the next v5
   stable asset is published or backfilled. This is not a v6 code blocker, but
   it remains part of the RC3/v5 sequencing checklist.

## Outcome

The audit removed several high-confidence v5-to-v6 regressions from the RC3
candidate, but later RC issue intake is not fully dispositioned. The
`known-rc-issue-closure-for-ga` gate must remain blocked until the remaining
RC3 triage candidates above are fixed, proven invalid, or conservatively
superseded.
