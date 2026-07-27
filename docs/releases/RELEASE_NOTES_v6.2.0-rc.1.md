# Pulse v6.2.0-rc.1 Release Notes

`v6.2.0-rc.1` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2` and is the first candidate on the `v6.2.0` line. It
introduces External Probes for Pulse Pro, closes the remaining multi-site
Proxmox identity-isolation gaps reported through support, and carries the
update-recovery, TrueNAS, and telemetry work completed since the previous
stable cut.

## Highlights

- **External Probes (Pulse Pro)**: an availability check can now run from a
  connected Pulse agent outside the monitored site, so a site can be watched
  from the outside instead of only from the Pulse server.
- Proxmox clusters, nodes, and host agents at different sites stay separate
  even when they reuse cluster names, node names, and private addressing.
- Adding a second site whose nodes share hostnames with an existing site no
  longer lets those nodes take over the first site's node records.
- Host agents deployed from a cloned template image are surfaced instead of
  silently collapsing two physical machines into one host row.
- Failed unattended updates restore service availability instead of leaving
  Pulse stopped, and TrueNAS appliances behind an HTTP-to-HTTPS redirect
  connect again.

## Added

- **External Probes (Pulse Pro).** An availability check can be assigned to a
  connected Pulse agent through the new **Run from** control in the
  availability check editor. Assignments travel over the signed agent
  configuration channel, the assigned agent runs the same ICMP, TCP, HTTP, and
  UDP probe core the server uses, and results come back with the agent's
  regular reports carrying per-target source attribution. Statuses show a
  "via agent" chip when the latest observation came from a probe. Only the
  currently assigned agent's results are accepted, an assigned check never
  also runs locally, probes whose reports go stale surface as indeterminate
  rather than as a false outage, and a check resumes local execution
  automatically if the entitlement lapses. Local availability checks remain
  free and unchanged in every edition; only the assignment requires Pulse Pro.
- Host agents surface an identity collapse when two machines share a cloned
  `/etc/machine-id`. Repeated hostname and report-IP revisits for one resolved
  agent identity raise a conflict warning on the Machines page instead of
  letting the two hosts silently overwrite each other. A one-time hostname
  rename never revisits and is not flagged, and the conflict clears itself
  once only one machine keeps reporting.
- Usage telemetry counts `availability_probe_targets` and
  `availability_probe_agents`. Both are counts only, with no agent names,
  addresses, or target details, and both are disclosed in `docs/PRIVACY.md`.

## Changed

- Approved-action telemetry separates success, pre-dispatch refusal, execution
  failure, unverified completion, stuck execution, in-flight work, and
  unclassified attempts.
- Pre-dispatch refusals use fixed plan, policy, capability, and other
  categories without transmitting action, resource, command, actor, finding, or
  evidence content.
- The adoption report defaults to the latest stable release and reports
  action-outcome and refusal-category reconciliation gaps.
- The frontend build dependency `postcss` is updated to `8.5.23`, resolving the
  source-map path-traversal advisory affecting older build tooling.

## Fixed

- Same-name Proxmox clusters at different sites are no longer consolidated
  solely because they reuse endpoint addresses. Contradicting captured TLS
  certificate fingerprints now veto the merge.
- Cross-instance Proxmox node and agent aggregation uses the same TLS identity
  doctrine as connection consolidation, preventing telemetry from one site
  attaching to another site's node.
- Adding a second site whose cluster membership has not been detected yet no
  longer lets its first-poll nodes merge into an established cluster at another
  site. A node that is still unclassified now needs positive same-machine
  evidence before it can fold into a named cluster, and Proxmox polling
  resolves cluster membership before the first node-state write. This closes a
  support case where adding a second site with the same node hostnames took
  over the first site's node record and made an existing node look renamed.
- An installer failure after stopping `pulse.service` no longer leaves the
  service down or causes every later timer run to be skipped (#1630).
- Installer writes to protected helper, PATH, and symlink locations are
  idempotent and non-fatal under the hardened unattended-update unit (#1630).
- TrueNAS websocket setup retries a same-host HTTP-to-HTTPS redirect once while
  refusing cross-host redirects and TLS downgrades (#1631).
- Resolved-loop telemetry no longer joins an unrelated Patrol resolution to an
  unrelated successful action within the same reporting window.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.1` only when you are
comfortable testing an RC. The rollback target is `v6.1.2`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

External Probes need the `external_probe` Pulse Pro entitlement on the Pulse
server. The capability ships in the community binary and the entitlement alone
gates the assignment, so no separate build is required. ICMP probes use the
system `ping` binary, so a probe host running in a container or a hardened
service unit needs `CAP_NET_RAW`; prefer TCP or HTTP checks there, or grant the
capability. See `docs/UNIFIED_AGENT.md`.

The telemetry receiver is backward compatible and was deployed before this
client release. Existing installs continue to report the earlier schema until
they upgrade.

This server candidate has no mobile compatibility change and does not require a
companion build upload. No public mobile-store rollout is part of this RC.

Windows Unified Agent binaries in this candidate keep checksum and
detached-signature verification, but they are not yet Authenticode-signed and
Windows may show an unknown-publisher warning. No unsigned-Windows exception
applies to any `v6.2.0` release: the owner-approved exception was bounded to
`v6.1.0`, `v6.1.1`, and `v6.1.2`, and stable `v6.2.0` must publish Windows
agents through the mandatory SignPath Authenticode path.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
