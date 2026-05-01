# Known RC Issue Closure For GA v5.1.29 Delta Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

After the Docker agent reconnect fix landed, the RC3 sweep was continued against
the current open GitHub issue and discussion queue plus the published
`release/5.1` `v5.1.29` delta. This pass focused on whether any latest v5
maintenance fixes or newly updated public threads still implied a v6 RC3
regression.

No public GitHub comments, issue closes, retitles, or labels were sent during
this pass.

## Disposition

1. `release/5.1` `v5.1.29` delta:
   - Alert notification spam fix `#1444` is already present in v6 through
     `3d3b1a964` and the later RC3 maintenance port.
   - Bootstrap token display fix `#1451` is already present in v6 through the
     installer port: the installer reveals the setup token through
     `pulse bootstrap-token` instead of printing encrypted `.bootstrap_token`
     JSON.
   - SSE EOF tool-call finalization from v5 is already present in v6 through
     the OpenAI provider stream parser and EOF/no-`[DONE]` tests.
   - Update-progress modal closability, stable containerized agent identity,
     guest snapshot carry-forward, mdstat RAID operation gating, linked guest
     filesystems, and Ollama `keep_alive=30s` already have v6 equivalents.
   - Security dependency fixes are at or above the v5 floor in v6:
     `toolchain go1.25.9`, `golang.org/x/net v0.52.0`, and `dompurify` locked
     to `3.4.1`.

2. `#1435` latest installer sequencing:
   - The current GitHub release state was rechecked. `/releases/latest` now
     points at `v5.1.29`, not `v6.0.0-rc.2`.
   - The stable `release/5.1` installer also filters prereleases and keeps RC
     installs opt-in.
   - This is no longer an RC3 code blocker.

3. `#1451` restored Proxmox LXC mount failure:
   - The screenshot was inspected. The restore failure is a stale
     `/run/pulse-sensor-proxy` bind mount in the restored LXC config.
   - The v6 installer already carries `cleanup_stale_sensor_proxy_mounts`,
     which removes stale `pulse-sensor-proxy` `mp<N>` and `lxc.mount.entry`
     lines before installation.
   - The remaining restored-container support question does not imply an
     unported v6 RC3 regression.

4. `#1443` v6 interface density feedback:
   - The screenshots were inspected. They show a real preference and
     information-density concern comparing v5 table-first status with rc.2
     dashboard/resource-map charts.
   - The current v6 candidate has already moved authenticated landing to
     Infrastructure and keeps Workloads table-first with charts behind an
     explicit control.
   - This remains valid product feedback, but not a narrow unhandled RC3 defect
     in this sweep.

5. `#1449` and discussion `#1450`:
   - `#1449` is pricing/packaging feedback about self-hosted free tier and SSO,
     not a runtime regression in the current candidate.
   - `#1450` is project-activity/license-confidence feedback, with a later
     commenter follow-up acknowledging activity resumed.
   - Both may need maintainer-facing response, but neither changes the RC3
     code readiness decision.

## Proof

- `gh issue list --repo rcourtman/Pulse --state open --limit 60 --json number,title,author,createdAt,updatedAt,labels,comments,url`
- `gh issue view 1451 --repo rcourtman/Pulse --json number,title,body,comments,labels,createdAt,updatedAt,url`
- `gh issue view 1444 --repo rcourtman/Pulse --json number,title,body,comments,labels,createdAt,updatedAt,url`
- `gh issue view 1441 --repo rcourtman/Pulse --json number,title,body,comments,labels,createdAt,updatedAt,url`
- `gh issue view 1449 --repo rcourtman/Pulse --json number,title,body,comments,labels,createdAt,updatedAt,url`
- `gh issue view 1443 --repo rcourtman/Pulse --json number,title,body,comments,labels,createdAt,updatedAt,url`
- `gh api graphql` discussion read for `#1450`
- `gh release list --repo rcourtman/Pulse --limit 20 --json tagName,isDraft,isPrerelease,isLatest,createdAt,publishedAt,name`
- `curl -I -L https://github.com/rcourtman/Pulse/releases/latest/download/install.sh`
- `git -C ../pulse-5.1.x log --oneline --decorate --since='2026-04-20' --no-merges --`
- `git log --oneline --decorate --since='2026-04-20' --no-merges --`
- Screenshot inspection for `#1451`, `#1444`, `#1441`, and `#1443`.

## Outcome

The continued RC3 sweep did not find another unhandled v5-to-v6 regression or
new GitHub issue/discussion item that should block the next v6 release
candidate. Public thread hygiene is still separate maintainer work, but the
`known-rc-issue-closure-for-ga` gate remains satisfied for the current code and
documentation state.
