# Known RC Issue Closure For GA Record

- Date: `2026-07-04`
- Candidate: `pulse/v6-release` after the GA changelog reconciliation and
  this release-metadata cleanup pass
- Prior dry run: `28683144630` (green; superseded by the final dry run for the
  publish candidate)
- Gate: `known-rc-issue-closure-for-ga`
- Result: `pass`

## Scope

This record covers every open issue labeled `affects-6.0.0-rc.*` (`rc.2`
through `rc.6`) reviewed against the current GA branch candidate. Earlier RC
issues that were already closed under the `2026-04-21` closure record are not
re-walked here unless they have new activity.

## Candidate Issue Disposition

1. `#1465` (`[Bug]: 6.0rc.4 - Added 2 PBS Servers - showing error in Dashboard
   but looks fine in Settings / Infrastructure`)
   - Partially fixed in candidate. The dashboard-vs-settings contradiction
     (defer-evaluation bug in the PBS poller) is fixed by `bf6261adc` and
     `bd7d196c1`, both ancestors of `8bac64f90`.
   - The remaining connection-failure portion is the reporter's separate TLS
     cert-mismatch report (`#1476`), not an admitted Pulse defect.
   - Disposition: fixed in candidate (Pulse-owned portion); the GitHub issue
     can stay open until the reporter confirms on GA or closes it.

2. `#1464` (`[Bug]: Failed to fetch audit events: Internal Server Error`)
   - Logging gap fixed by `27bd31684` (in `8bac64f90`); the audit-list 500
     now records its underlying cause.
   - Root cause of the query failure itself was never confirmed by the
     reporter (suspected SQLite `database is locked` on a busy audit table).
     The diagnostics improvement is in; the actual query behavior is
     unverified.
   - Disposition: partially fixed in candidate. Not an admitted hard GA
     blocker because the handler now fails with diagnostic logging instead
     of a silent 500, but the underlying slowness/lock risk on sizeable
     audit tables is unresolved and should be watched post-GA.

3. `#1496` (`[Bug]: pulseV6, unified_resources.db growing, growing,...`)
   - Fixed in candidate by `3202b4ab5` (`fix: add retention pruning for
     unified_resources.db (issue #1496)`) and `028e8c8df` (space reclamation
     and retention for all append-only tables), both in `8bac64f90`.
   - Disposition: fixed in candidate. The reporter's own diagnosis pointed
     at exactly the missing retention/prune path that these commits added.

4. `#1470` (`[Bug]: Wrong install.sh for 6.0.0 rc.1-rc.5 releases?`)
   - Fix commits landed on `pulse/v6-release`: `49412357a`, `7b2cac08b`,
     `a3e36d787`, `bcbec3acd`. All four are ancestors of the GA branch
     candidate.
   - **Release-engineering caveat:** the fix has never shipped in a published
     RC release asset. The maintainer's stated condition for closing was
     "until the next RC carrying the fixes is published and validated," and
     the most recent user report (`dwhoban`, 2026-06-19) confirmed the
     symptom still reproduced on a clean `rc.6` install. The GA candidate
     is therefore the first release where the corrected `install.sh` asset
     would be validated by real users.
   - The stable release workflow now validates the published release asset
     packet and then runs `install-sh-smoke.yml` against the just-published
     GitHub Release URL before the release workflow can finish successfully.
   - Disposition: fixed in branch/candidate source, with the public
     release-asset path governed by the post-publish release workflow smoke.
     The release must not be treated as complete unless that workflow gate
     passes, because the historical failure mode (`[ERROR] Unknown option:
     --url`) breaks fresh installs at the first step.

5. `#1456` (`[Bug]: Workload shows no data on Pulse | Version: 6.0.0-rc.3`)
   - Not an admitted Pulse defect. Maintainer attributed the reported
     "no data" to a Proxmox 401 on the user's API token (revoked /
     insufficient role / regenerated secret). The retired `/workloads`
     aggregate page was replaced by platform-first pages in RC.5.
   - Disposition: invalid / config-side. Not a GA blocker.

6. `#1463` (`[Bug]: AI-Modell for Patrol unable to save and to run`)
   - Largely config-side: maintainer identified three user-config errors
     (wrong Ollama model name, missing provider API key, OpenRouter free
     model lacking tool support).
   - One UI improvement was promised to the reporter (toast that surfaces
     the actual server response instead of the generic "Failed to save
     advanced settings"). No commit was cited and none is present in the
     candidate search. Treated as a small outstanding polish item, not a
     GA blocker.
   - Disposition: mostly invalid (config-side); one minor UI improvement
     deferred to post-GA.

7. `#1498` (`[Bug]: Pulse v6.0.0-rc.6 agents not updated`)
   - The reporter expects agents to update to match the server "like in V5."
     The v6 product path keeps server upgrade and Unified Agent upgrade as
     separate operations, with agent self-update depending on agent
     authentication, trusted update transport, and release signing trust.
   - GA release notes and `docs/UPGRADE_v6.md` now state explicitly that
     moving the server to v6 does not itself upgrade installed agents, and
     that operators should use `Settings → Infrastructure → Install on a host`
     to generate each agent install/upgrade command and verify the reported
     agent version.
   - Disposition: documented intentional v6 upgrade boundary, not a silent
     GA blocker. The GitHub issue still needs a maintainer response so the
     reporter has the same answer publicly.

8. `#1493` (`[Bug]: Settings->infrastructure: Strange Error and API`)
   - Untriaged — zero maintainer comments on the issue. Reporter sees a
     stale error message and stale IP addresses on the infrastructure
     settings surface after their Proxmox cluster moved subnets
     (`10.32.21.x/24` to `10.32.20.x/22`). Also flags an "API" labeling
     concern where the surface lists per-node entries when one cluster-wide
     API token is in use.
   - No fix commit identified. Reads as a display / state-staleness bug,
     not a data-loss or upgrade-blocking defect.
   - Disposition: unresolved, low-severity. Not a GA blocker on its face,
     but genuinely unreviewed and should get a maintainer response before
     GA so it is not carried silently.

9. `#1485` (`[Bug]: Not urgent - Unraid parity alert`)
   - Untriaged — zero maintainer comments. Reporter says Pulse keeps
     alerting about an Unraid parity check that was cancelled on the host.
     Reads as an alert-state-clearing bug specific to Unraid's parity-check
     signal. No fix commit identified.
   - Disposition: unresolved, low-severity (Unraid-specific, reporter
     self-tagged "Not urgent"). Not a GA blocker, but should get a
     maintainer response.

10. `#1441` (`[Bug]: Server offline, shown as online`)
    - Untriaged by the maintainer on this issue. A second reporter
      (`asm-ch`) linked it to discussion `#1135` from 2026-01-21 and
      stated it has reproduced since `v5.0.17`. This is therefore a
      pre-existing v5-era Proxmox offline-state detection issue, not a
      v6 regression.
    - Disposition: pre-existing v5-era defect, not a v6 GA regression.
      Not a GA blocker, but a two-month-old customer-trust item that
      should not be carried into GA without a response.

## Outcome

- The RC-era issue set that received engineering attention is closed on the
  candidate: PBS dashboard contradiction (`#1465`), unified_resources
  retention (`#1496`), installer asset identity (`#1470` at the source
  level), and audit-list diagnostics (`#1464`).
- `known-rc-issue-closure-for-ga` is satisfied for the candidate source and
  GA documentation. The `#1470` public asset path remains guarded by the
  release workflow's post-publish `install-sh-smoke.yml` gate; the release
  must not be treated as complete if that gate fails.
- `#1498` is no longer carried silently: the GA release notes and upgrade
  guide state that server upgrade and Unified Agent upgrade are separate v6
  operations, and operators must verify agent versions after running the
  generated agent install/upgrade command.
- Three issues (`#1493`, `#1485`, `#1441`) are not GA blockers on their
  face but are unresponded and should not be carried into GA silently.
  They need a maintainer reply, even if only to set post-GA expectation.
- The 2026-04-21 closure record's convention that "GitHub issues may remain
  open until public maintainer triage catches up" still holds here: this
  record passing does not require every referenced GitHub issue to be
  closed, only that each has been examined and given an explicit
  disposition.
