# Pulse v6.2.0-rc.10 Release Notes

`v6.2.0-rc.10` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2` and supersedes `v6.2.0-rc.9`. This candidate focuses on
security boundaries, role-correct settings and resource access, resilient live
state recovery, and safer release operations.

## Highlights

- Installer, diagnostic, and sign-in URLs reject untrusted origins; proxy
  cleanup verifies SSH hosts; historical credentials are contained.
- Settings, resource links, update polling, health, and oversized WebSocket
  recovery preserve role boundaries and clean live state.
- Agent/PBS lifecycle fixes, opt-in commerce, secret scanning, worktree
  isolation, and convergent promotion improve operations.

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
- Corrected responsive Settings panel clipping and restored architecture tests
  that keep desktop, tablet, and phone navigation coherent.
- Restored release staging before publication, exact-version paid-customer
  promotion order, verifiable MSP evaluation delivery, and dependency-audit
  enforcement for the shipped frontend.

## Release Qualification

- The v6 control plane reports all 44 readiness assertions and all 26 release
  gates passed, including complete provider closure and replacement or
  retirement evidence for every historically reachable credential identity.
- The post-RC9 code-backed risk range contains 61 commits across 226 files and
  ends at `5ff0855882cdbcfc9d4c8f8d87a1ffa3972db818`; the containment record and
  release-packet commits are metadata-only successors.
- Targeted proof covers request-origin validation, SSH host-key enforcement,
  settings RBAC and responsive layout, WebSocket recovery, resource deltas,
  PBS polling, agent update convergence, release promotion, dependency audits,
  and mobile API compatibility.
- Release publication builds and validates one immutable `main` SHA before
  publishing the GitHub prerelease, Docker image, Helm chart, and private Pro
  packet.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.10` only when you are
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
