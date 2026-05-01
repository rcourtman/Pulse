# Known RC Issue Closure For GA Late Issue Intake Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

After the RC3 defect fixes landed, a second pass reviewed the late open issue
and discussion intake against the current `pulse/v6-release` candidate. The
pass included live GitHub issue reads, screenshot inspection for screenshot-led
reports, and browser exercise of the local v6 dev build.

No public GitHub comments, issue closes, retitles, or labels were sent during
this pass.

## Disposition

1. `#1443` interface too busy / shows too little:
   - The attached rc.2 screenshots were inspected. They show the old rc.2
     top-row layout and the resource-map/trend chart presentation.
   - The current v6 browser build now lands directly on `Infrastructure`
     rather than the old dashboard, and the Infrastructure view is a dense table
     with CPU, memory, disk, network, disk I/O, system, uptime, and temperature
     columns.
   - The current `Workloads` view defaults to a dense workload table; charts are
     behind the explicit `Charts` control instead of being the default first
     read.
   - The issue remains valid product feedback around density, classic-look
     preference, and reduced visual complexity, but the screenshots no longer
     identify an unhandled narrow RC3 blocker in the current candidate.

2. `#1429` missing Docker info and v5/v6 dashboard confusion:
   - The attached screenshots were inspected. The core mismatch was the user
     comparing the old v6 Dashboard summary to the v5 Proxmox table and then
     looking for Docker under the old tab model.
   - The current v6 candidate routes the primary landing experience to
     `Infrastructure`; Docker hosts are available through source filtering, and
     containers are available through `Workloads`.
   - The earlier entitlement-cap and empty-trends concerns are already covered
     by the RC3 entitlement/installer work and by the changed default landing
     surface. This thread can remain open for reporter retest guidance without
     blocking RC3.

3. `#1451` bootstrap token formatting:
   - The screenshots were inspected, including the sensitive token-format
     screenshot without copying or quoting the token value.
   - The reporter later confirmed that v5.1.29 restored the normal token format
     on a fresh install.
   - The matching v6 installer fix is already in the current RC3 patch. The
     separate Proxmox Backup Server restore mount error is a support follow-up,
     not a known v6 RC3 product regression.

4. `#1435` and discussion `#876` installer/latest and entitlement confusion:
   - The stable `/latest` installer path has been rechecked against the current
     GitHub release state and now points at v5.1.29 rather than the rc.2 asset.
   - The stable installer branch also has the prerelease guard and explicit RC
     opt-in path, so this is no longer a v6 RC3 code blocker.
   - Any remaining public hygiene is reporter-facing follow-up, not candidate
     correctness work.

5. Discussion `#1445` root/privileged agent concern:
   - The discussion raises a real security-disclosure and operator-expectation
     gap around the systemd agent running as `root`, lower-privilege operation,
     command execution, and release supply-chain trust.
   - The RC3 response is documentation, not a runtime permission downgrade:
     `docs/AGENT_SECURITY.md` now documents the root-by-default telemetry
     posture, unsupported lower-privilege tradeoffs, command execution being
     disabled by default, and the distinct installer trust boundary.
   - This keeps the release candidate honest about the current agent model
     without inventing an unsupported least-privilege profile during RC soak.

6. Already-fixed technical candidates:
   - `#1441`, `#1442`, `#1452`, discussion `#1448`, and discussion `#1290`
     remain covered by the earlier RC3 follow-up, duplicate metrics, tooltip,
     PBS threshold, and Ceph monitor records.
   - Those public threads may still need maintainer follow-up or reporter
     retesting, but they no longer represent unexamined RC3 blockers in the
     candidate.

## Proof

- `gh issue list --repo rcourtman/Pulse --state open --limit 30 --json number,title,labels,updatedAt`
- `gh issue view 1451 --repo rcourtman/Pulse --json number,title,state,comments,labels,updatedAt`
- `gh issue view 1443 --repo rcourtman/Pulse --json number,title,state,comments,labels,updatedAt`
- `gh issue view 1429 --repo rcourtman/Pulse --json number,title,state,comments,labels,updatedAt`
- `gh issue view 1452 --repo rcourtman/Pulse --json number,title,state,labels,updatedAt,comments`
- `gh issue view 1441 --repo rcourtman/Pulse --json number,title,state,labels,updatedAt,comments`
- `gh api repos/rcourtman/Pulse/discussions/1445`
- `gh api repos/rcourtman/Pulse/discussions/876`
- Local browser exercise of `http://127.0.0.1:5173` after login, covering the
  current `Infrastructure` and `Workloads` surfaces.
- `npm run dev:status`

## Outcome

The late issue/discussion pass did not find a new unhandled v6 RC3 blocker. The
current candidate still needs public issue hygiene before or after RC3, but the
`known-rc-issue-closure-for-ga` release gate remains satisfied for the code and
documentation state being prepared.
