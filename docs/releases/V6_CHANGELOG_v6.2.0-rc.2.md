# Pulse v6.2.0-rc.2

_This changelog describes the changes since `v6.2.0-rc.1`.
`v6.2.0-rc.2` remains a prerelease and rolls back to stable `v6.1.2`._

## Added

- Explicit per-metric off toggles in the per-resource override editor, the
  global defaults row, and the bulk edit dialog, replacing the `-1` sentinel
  operators had to type by hand (#1642).
- OIDC provider cards show the provider's callback / redirect URL with a copy
  button so it can be registered at the identity provider without guesswork.
- Recorded PVE RRD fixtures and decode-alignment tests pin the guest RRD
  decode path against real Proxmox payloads.

## Changed

- Platform table metric bars are colored from the configured alert thresholds
  rather than fixed cutoffs.
- SSO callback, SP metadata, and ACS URLs are derived from the configured
  public URL or, when none is set, from the request that reached Pulse,
  instead of a hardcoded `localhost` base that fails at the identity provider.
  When neither source resolves a host the fields are omitted and the panel
  points at the public URL setting.
- The unused `autoUpdateTime` and `autoUpdateCheckInterval` system settings
  were removed from the settings API and UI; stored values load unchanged and
  API clients that still send them are accepted.
- Dead cache-aware RRD fields were removed from the guest RRD path.

## Fixed

- The generated `pulse-update.timer` carried two `OnCalendar` lines and
  attempted two updates per day; it now runs once daily in the 02:00-06:00
  window.
- Auto-update asset staging is guarded, so a failed copy or write can no
  longer install a broken helper or a truncated systemd unit while reporting
  success.
- Deployed installations whose hardened update unit cannot rewrite its own
  files migrate through a transient `systemd-run` unit on the next update
  cycle, and the update still completes when that migration cannot run.
- PBS backup attribution for VMIDs shared across clusters uses each cluster's
  own view of its PBS storage, so a guest no longer shows "no backup" or a
  stale backup age when its real backups sit in the root namespace (#1639).
- PBS submission-source learning only trusts evidence when every candidate
  cluster is visible, survives partial poll failures instead of flapping, and
  backup-age alerts no longer cross-match a PBS connection name against a
  similarly named node (#1639).
- Proxmox host-token installs register the source the installer detects, with
  one registration per canonical product type, a shared first-use hostname
  bind, and the 24-hour mint-age bound, so combined PVE and PBS hosts complete
  both legs from a single install token (#1644).
- A refused Proxmox registration aborts the install visibly, the API setup
  script points at Settings then Infrastructure instead of a page that no
  longer exists, its error branch reports invalid or expired setup tokens, and
  the post-install check waits for the agent's first report (#1644).
- The Patrol readiness endpoint commits its response header and streams
  keepalive bytes from the start of the run, so proxies with short read
  timeouts keep the connection open for the full multi-probe evaluation
  (#1640).
- A Patrol readiness run cut mid-flight is classified as interrupted, keeps
  evidence from completed scenarios, and renders a neutral banner instead of
  blaming the model (#1640).
- Patrol readiness is reported only from a completed overall verdict, an
  unassessed snapshot falls back to the base-config classifier capped at a
  warning, and an interrupted check never blocks Patrol from running (#1640).
- A panic inside provider streaming or validation is recovered into an
  ordinary readiness result carrying an internal-error cause instead of
  taking the process down, and the browser client no longer renders raw
  non-JSON error bodies (#1640).
- The cluster endpoint discovery-policy check resolves hostnames through the
  shared cached resolver instead of a raw lookup per node per poll, and the
  link-local blocklist is enforced against the same resolver view the
  connection dial uses (#1638).
- Failed SSH temperature probes and host-key scans back off with decay instead
  of retrying every poll cycle, and the backoff clears when the SSH key
  changes or system settings are saved (#1638).
- Thresholds disabled under the older guidance that said to use 0 render
  consistently as off, turning a metric back on always produces a working
  threshold, and the toggle is reachable by keyboard (#1642).
- LXC memory metrics fall back to the cluster-resources listing when per-guest
  RRD data is unavailable, restoring memory readings lost on some PVE
  configurations.
- Agent command execution honors the configuration gate at channel admission,
  closing a window where a disabled command channel could still admit an
  agent.

## Docs

- New Microsoft Entra ID (Azure AD) SSO integration guide covering app
  registration, group claims by Object ID, role mapping, the `AADSTS650053`
  scope pitfall, and group-overage protection (#1635).

## Release Metadata

- Version: `v6.2.0-rc.2`
- Previous candidate: `v6.2.0-rc.1`
- Previous stable: `v6.1.2`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease that does not move stable or latest
  install pointers
- Windows signing decision: Authenticode through SignPath is the mandatory
  signing backend and no unsigned-Windows exception applies to any `v6.2.0`
  release; this candidate publishes Windows agents under the standing
  prerelease path with exact-SHA, checksum, and detached-signature
  verification
- Mobile decision: `no-mobile-impact`; no companion build upload or public
  store rollout is part of this candidate
