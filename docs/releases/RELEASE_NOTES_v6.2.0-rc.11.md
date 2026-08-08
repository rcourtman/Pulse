# Pulse v6.2.0-rc.11 Release Notes

`v6.2.0-rc.11` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2`, replaces the abandoned partial `v6.2.0-rc.10`
candidate, and supersedes `v6.2.0-rc.9` for public prerelease testing. This
candidate focuses on security boundaries, role-correct settings and resource
access, resilient live state recovery, accurate Proxmox availability, and safer
release operations.

## Highlights

- Security hardening blocks untrusted installer, diagnostic, sign-in, and SSH cleanup origins.
- Role-correct Settings and resilient recovery keep viewer access and live resource state coherent.
- Proxmox availability retries GET when a server rejects HEAD with HTTP 405 or 501.

## Fixed

- Rejected hostile request-host, forwarded-host, and configured-public-URL
  values before they can enter copied installer commands, hosted diagnostics,
  or magic-link responses.
- Hid administrator-only System and infrastructure settings from viewer
  sessions, stopped viewer polling of privileged endpoints, and kept
  viewer-safe workload health available without offering inaccessible routes.
- Recovered from WebSocket frames above the inbound guard without accepting an
  oversized baseline, and applied subsequent resource deltas to canonical raw
  server state.
- Prevented update-status polling outside the routes that own update authority
  and reduced routine authorization-denial log noise without weakening abuse
  signals.
- Stopped repeated PBS node-name fetches, corrected retry classification, and
  preserved configured alert intent for agents merged with Proxmox nodes.
- Retried HTTP and HTTPS availability probes with GET when a target reports
  that HEAD is unsupported through HTTP 405 or 501, while retaining failures
  for other server errors.
- Corrected responsive Settings panel clipping and restored architecture tests
  that keep desktop, tablet, and phone navigation coherent.
- Restored authenticated draft-asset access for release install smoke and gave
  the Windows TLS fixture a bounded setup window on cold hosted runners.
- Restored release staging before publication, exact-version paid-customer
  promotion order, verifiable MSP evaluation delivery, and dependency-audit
  enforcement for the shipped frontend.

## Release Qualification

- The v6 control plane reports all 44 readiness assertions and all 26 release
  gates passed, including complete provider closure and replacement or
  retirement evidence for every historically reachable credential identity.
- The post-RC9 code-backed validation-risk range contains 68 commits across 241
  files and ends at `2018aa8a9a965d693982e260f525f6cc4f49aa41`; the monitoring
  contract and RC11 packet commits are metadata-only successors.
- RC10 is not reused: an earlier failed attempt staged exact-version registry
  artifacts from `76e07be290892ed8453bbed942855c1e7f673232`, so RC11 provides a
  new immutable version identity for the corrected candidate.
- Targeted proof covers request-origin validation, SSH host-key enforcement,
  settings RBAC and responsive layout, WebSocket recovery, resource deltas,
  PBS polling, Proxmox HTTP probe fallback, agent update convergence, release
  promotion, dependency audits, Windows install-command execution, and mobile
  API compatibility.
- Release publication builds and validates one immutable `main` SHA before
  publishing the GitHub prerelease, Docker image, Helm chart, and private Pro
  packet.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.11` only when you are
comfortable testing an RC. The rollback target is stable `v6.1.2`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

Existing configurations remain valid and no manual data migration is required.

This server candidate is compatible with the current Pulse Mobile 1.0.0 beta
candidates. iOS build 12 is distributed through the TestFlight public beta link,
and Android versionCode 9 remains available through Play open testing; both use
runtime version 2. The changes since RC9 preserve the checked-in mobile API,
Relay, pairing, approval, push, authentication, and onboarding contracts. No
public mobile-store rollout is part of this RC.

Windows Unified Agent binaries in this candidate keep checksum and
detached-signature verification, but they are not yet Authenticode-signed and
Windows may show an unknown-publisher warning. No unsigned-Windows exception
applies to any `v6.2.0` release. Stable `v6.2.0` must publish Windows agents
through the mandatory SignPath Authenticode path.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
