# Pulse v6.2.0-rc.2 Release Notes

`v6.2.0-rc.2` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2` and supersedes `v6.2.0-rc.1`. It is a hardening and
bugfix candidate: it closes the highest-impact defects reported against
v6.1.x — nightly service outages from the unattended updater, missing PBS
backups on multi-cluster estates, broken Proxmox agent installs, and a DNS
query flood — and carries the fixes from an adversarial review pass over every
change since the previous candidate.

## Highlights

- Unattended updates no longer stop Pulse in the night. The update timer fired
  twice per day and a failed update left the service down until someone
  restarted it; the timer now runs once, failures always restart the service,
  and already-deployed installations repair their own stale update units on
  the next update cycle.
- PBS backups are attributed to the right guest on estates with more than one
  Proxmox cluster. VMIDs that exist on both clusters no longer show "no
  backup" or an ancient backup age when the real backups are in the root
  namespace (#1639).
- Installing the agent on a Proxmox host from **Settings → Infrastructure**
  works again, including combined PVE+PBS hosts, and failures are reported
  loudly in the installer output instead of a buried journal warning (#1644).
- The Patrol model readiness check survives reverse proxies with short read
  timeouts, reports an interrupted check neutrally instead of blaming the
  model, and never renders a proxy's HTML error page into the results (#1640).
- Pulse no longer floods the local resolver. A discovery-policy check that
  resolved every cluster node's hostname on every 10-second poll now uses the
  shared DNS cache, cutting hundreds of thousands of daily queries to a
  handful (#1638).

## Fixed

- **Auto-update reliability.** The generated `pulse-update.timer` carried two
  `OnCalendar` lines and attempted two updates per day; it now runs once
  daily in the 02:00–06:00 window. Asset staging is guarded: a failed copy or
  write can no longer install a broken helper or a truncated systemd unit
  while reporting success. Existing installations whose hardened update unit
  cannot rewrite its own files migrate automatically through a transient
  `systemd-run` unit on their next update; if the migration cannot run, the
  update itself still completes as before.
- **PBS backup attribution across clusters** (#1639). Snapshot-to-guest
  matching now uses each cluster's own view of its PBS storage and a
  submission-source learner that only trusts evidence when every candidate
  cluster is visible, so shared tokens or synced datastores cannot attribute
  one cluster's backups to another. Evidence survives partial poll failures
  instead of flapping. Backup-age alerts no longer cross-match a PBS
  connection name against a similarly named node.
- **Proxmox agent installation** (#1644). Install tokens minted from the main
  installer can now register the PVE or PBS source the installer detects —
  one registration per product type, bound to the first presenting hostname,
  and only within 24 hours of minting. A refused registration aborts the
  install visibly. The API setup script's closing message points at
  **Settings → Infrastructure** instead of a page that no longer exists, and
  its error branch reports invalid or expired setup tokens instead of
  claiming success. The post-install check waits for the agent's first report
  instead of warning two seconds in.
- **Patrol readiness checks** (#1640). The readiness endpoint streams
  keepalive bytes from the first moment of the run, so proxies with 30-second
  read timeouts see a live connection for the full multi-probe evaluation. A
  connection cut mid-run is classified as interrupted, keeps the evidence
  from completed scenarios, and shows a neutral "check did not complete"
  banner; readiness is only reported from a completed verdict, and an
  interrupted check never blocks Patrol from running. A crash inside a
  provider integration now fails the check instead of the whole process, and
  the browser client no longer prints raw non-JSON error bodies into the UI.
- **DNS and SSH churn** (#1638). The cluster endpoint discovery-policy check
  resolves hostnames through the shared cached resolver instead of a raw
  lookup per node per poll, and the link-local blocklist is enforced against
  the same resolver view the connection dial uses. Failed SSH temperature
  probes and host-key scans back off with decay instead of retrying every 10
  seconds, and the backoff clears when the SSH key changes or settings are
  saved.
- **Alert threshold editors** (#1642). Each metric now has an explicit On/Off
  toggle in the per-resource override editor, the global defaults row, and
  the bulk edit dialog — no more typing -1. Values disabled under older
  guidance that said to use 0 render consistently as Off everywhere, turning
  a metric back on always produces a working threshold, and the toggle is
  reachable by keyboard.
- **SSO callback URLs.** OIDC provider cards show the provider's callback /
  redirect URL with a copy button, and all SSO URLs are derived from the
  configured public URL or, when none is set, from the request that reached
  Pulse — never a `localhost` guess that fails at the identity provider.
- LXC memory metrics fall back to the cluster-resources listing again when
  per-guest RRD data is unavailable, restoring memory readings lost on some
  PVE configurations.
- Agent command execution honors the configuration gate at channel admission,
  closing a window where a disabled command channel could still admit an
  agent.

## Changed

- Platform table metric bars are colored from the configured alert thresholds
  rather than fixed cutoffs.
- The unused `autoUpdateTime` and `autoUpdateCheckInterval` settings were
  removed from the system settings API and UI; they were stored but never
  drove the update schedule. Existing configurations containing them load
  unchanged, and API clients that still send them are accepted.

## Docs

- New Microsoft Entra ID (Azure AD) SSO integration guide covering app
  registration, group claims by Object ID, role mapping, the `AADSTS650053`
  scope pitfall, and group-overage protection (#1635), contributed by
  @drgimpfen and expanded.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.2` only when you are
comfortable testing an RC. The rollback target is `v6.1.2`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

Installations already running the hardened unattended-update unit repair their
own timer and helper on the next update cycle, so no manual step is needed to
pick up the once-daily schedule. If that in-place migration cannot run, the
update itself still completes and the repair is retried on the following
cycle.

Everything `v6.2.0-rc.1` introduced is still in this candidate, including
External Probes for Pulse Pro. External Probes need the `external_probe` Pulse
Pro entitlement on the Pulse server, and ICMP probes use the system `ping`
binary, so a probe host running in a container or a hardened service unit needs
`CAP_NET_RAW`; prefer TCP or HTTP checks there, or grant the capability. See
`docs/UNIFIED_AGENT.md`.

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
