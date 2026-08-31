# Pulse v6.4.2

_This changelog describes stable `v6.4.2` compared with stable `v6.4.1`._

## Fixed

- The OSS default authorizer no longer treats ordinary authenticated
  organization membership as permission to use the infrastructure action
  control plane. Browser and proxy sessions must satisfy the canonical
  administrator boundary, while explicitly scoped API tokens and real RBAC
  action grants retain their governed paths.
- Route and lifecycle guardrails cover planning, review, approval, execution,
  refresh, and recovery action surfaces so the permission check cannot be
  bypassed through a sibling action endpoint.
- SSO authentication no longer becomes instance-administrator authority when
  no local administrator is configured. SSO browser sessions must carry an
  effective RBAC `admin` grant on `*` before they can use administrator-only
  settings, discovery, configuration transfer, platform, or infrastructure
  action routes.
- SAML domain and email allowlists now reject assertions without an email
  claim when either allowlist is configured.
- Security-sensitive bootstrap, setup, repair, and recovery request bodies now
  have explicit size limits and reject oversized JSON before decoding.
- Pulse now distinguishes actively written PBS snapshots from terminally
  incomplete artifacts by correlating incomplete snapshots with live PBS data
  tasks. Failed backups and completed PBS-to-PBS sync copies no longer pin a
  guest in Backup Running, and incomplete artifacts are excluded from
  recoverable latest-backup pointers.
- The Alerts overview now exposes the same retry and dismiss actions as the
  Notifications view for retained delivery failures, so operators can clear a
  warning without deleting delivery history.
- Assistant command help now uses the canonical responsive dialog boundary,
  including focus containment, background isolation, Escape and backdrop
  dismissal, and focus return to the invoking control.
- The shipped migration guide now documents rerunning the agent installer with
  a new URL when the Pulse server address changes.
- Update settings now label the prerelease channel as Preview and explain beta
  user-testing builds separately from release candidates, while retaining the
  existing `rc` wire value for compatibility.
- Generated systemd services now preserve Pulse log severity as native journal
  priorities, enabling `journalctl -p` and downstream syslog filtering without
  changing the structured message or other logging sinks.
- Durable Proxmox identity recovery now scopes pins to qualified provider
  endpoints and refuses ambiguous short names or IP addresses, keeping
  independent same-name estates distinct during provider-first restart windows.
- Infrastructure settings now evaluates Proxmox cluster agent coverage per
  node, names uncovered hosts, and offers node-level installer actions so one
  reporting member cannot make a partially instrumented cluster look complete.
- The Unified Agent now reports typed privilege-helper operation health through
  its bounded module status. Agent Doctor exposes a dedicated degradation reason
  when privileged telemetry is omitted, without granting broader collector
  authority or exposing raw helper errors.

## Release integrity

- Published release validation now authenticates every Unified Agent download
  endpoint by checking its checksum and detached-signature headers against the
  exact served bytes before activation.
- The release candidate verifier now binds the requested version explicitly in
  the exact-SHA compiled-payload step, preventing step-local environment state
  from aborting candidate assembly.
- Helm OCI publication now authenticates both Helm and the provenance
  attestation client against GHCR before pushing the exact-version chart and
  its attestation.
- Release automation now uses the protected `actions/checkout` baseline across
  build, qualification, publication, recovery, and deployment workflows, with
  workflow-trust checks that reject privileged fork checkout paths.

## Upgrade requirement

- An SSO-only installation must map at least one trusted IdP group to the
  built-in `admin` role before upgrading. Sessions mapped only to `operator`,
  `viewer`, or no role intentionally lose the administrator access that older
  versions granted implicitly.

## Release Metadata

- Version: `v6.4.2`
- Previous stable: `v6.4.1`
- Rollback target: `v6.4.1`
- Rollback command: `sudo /bin/update --version v6.4.1`
- Promotion path: emergency stable patch from `main`, using the single-build
  release workflow after exact-SHA qualification
- Emergency reason: current stable releases contain two authorization
  boundary failures. A non-administrator organization member may reach the
  infrastructure action control plane, and every authenticated IdP user on an
  SSO-only instance may inherit instance-administrator authority without an
  explicit grant
- Windows signing decision: the standing SignPath-unavailable policy publishes
  unsigned Windows Unified Agent binaries. They may display an Unknown
  Publisher warning while exact-SHA candidate binding, checksums, detached
  signatures, immutable-manifest verification, and published-digest
  verification remain mandatory
- Mobile decision: `no-mobile-impact`. The patch changes no governed mobile
  route, payload, Relay, pairing, approval, push, or onboarding contract, so no
  companion mobile build or store rollout is required.
