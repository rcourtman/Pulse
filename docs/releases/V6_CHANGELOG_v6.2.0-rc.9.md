# Pulse v6.2.0-rc.9

_This changelog describes the changes since `v6.2.0-rc.8`.
`v6.2.0-rc.9` remains a prerelease and rolls back to stable `v6.1.2`._

## Added

- Certificate-validity monitoring and alerting.
- Real VMware vCenter tags in collected resource metadata.
- Wildcard forms for Docker ignored-container alert patterns.

## Improved

- Shared generation-bound resource views across API requests and mock-mode
  consumers while preserving request-local presentation behavior.
- Responsive tables, filters, navigation, settings, alerts, and interaction
  targets across desktop, tablet, and phone layouts.
- Canonical agent auto-registration identity matching and platform-scoped alert
  threshold host handling.
- Multi-architecture update digest reporting and release signing-policy links.

## Fixed

- Preserved configured ntfy metadata, provider incidents, and acknowledged
  state across live delivery and notification-settings changes.
- Honored disabled alert grouping and represented every grouped alert in
  provider payloads.
- Released tenant resource-store handles during offboarding and shutdown, and
  allowed accepted guest metadata writes to finish before monitor teardown.
- Sent resource WebSocket deltas after initial state and kept mock-mode sources
  isolated from configured infrastructure.
- Refused to serve agent binaries older than the server, corrected the
  already-current version warning, and made wrapper, watchdog, and sibling-agent
  process handling safer.
- Corrected QNAP RAID role bitmap parsing and Docker container override identity.
- Closed the outstanding CodeQL findings, bounded provider MSP restore writes,
  and upgraded `js-yaml` to address GHSA-5p4m-2wfm-xmqj.
- Restored focus after inline details and token dialogs, localized displayed
  dates and times, and removed narrow-layout clipping and hidden controls.

## Release Metadata

- Version: `v6.2.0-rc.9`
- Previous candidate: `v6.2.0-rc.8`
- Previous stable: `v6.1.2`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease that does not move stable or latest
  install pointers
- Windows signing decision: Authenticode through SignPath is the mandatory
  signing backend and no unsigned-Windows exception applies to any `v6.2.0`
  release
- Mobile decision: `existing-mobile-build-compatible`; Pulse Mobile 1.0.0 iOS
  build 12 is distributed through the TestFlight public beta link and Android
  versionCode 9 remains on Play open testing, both using runtime version 2. The
  changes since RC8 do not alter mobile relay payloads, pairing, approvals,
  authentication, or onboarding contracts. No public store rollout is part of
  this candidate
