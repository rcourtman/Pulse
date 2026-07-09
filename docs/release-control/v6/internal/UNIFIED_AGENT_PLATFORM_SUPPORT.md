# Unified Agent Platform Support Evidence

This matrix separates a binary that can be cross-compiled from a lifecycle
that has been exercised on the operating system which will run it. A release
must not use cross-compilation alone as proof of native support.

## Evidence levels

- `native-ci`: tests and the built binary execute on a native GitHub-hosted OS
  and architecture runner.
- `native-lab`: install, service start, report, update, restart/reboot, and
  uninstall have been exercised on a maintained native lab target.
- `cross-build`: the release build produces the target and validates its file
  format, but no native runtime claim follows from that evidence.
- `installer-contract`: the installer parser, state recovery, service
  rendering, and teardown contracts have executable tests without a native
  installed lifecycle.

## Current matrix

| Runtime target | Release artifact | Automated evidence | Remaining release-grade evidence |
| --- | --- | --- | --- |
| Linux amd64 | yes | native-ci gate | scheduled installed-service reboot/update rehearsal |
| Linux arm64 | yes | native-ci gate | scheduled installed-service reboot/update rehearsal |
| Linux armv7 | yes | cross-build | native lab lifecycle |
| Linux armv6 | yes | cross-build | native lab lifecycle |
| Linux 386 | yes | cross-build | native lab lifecycle |
| macOS arm64 | yes | native-ci gate; prior v5-to-v6 native-lab upgrade record | signed and notarized release candidate; recurring installed-service lifecycle |
| macOS amd64 | yes | native-ci gate | signed and notarized release candidate; recurring installed-service lifecycle |
| Windows amd64 | yes | native-ci gate | Authenticode-signed release candidate; recurring installed-service lifecycle |
| Windows arm64 | yes | cross-build | native lab lifecycle and Authenticode signing |
| Windows 386 | yes | cross-build | native lab lifecycle and Authenticode signing |
| FreeBSD amd64 | yes | cross-build with fail-closed ELF validation; installer-contract | native rc.d install/update/reboot/uninstall lifecycle |
| FreeBSD arm64 | yes | cross-build with fail-closed ELF validation; installer-contract | native rc.d install/update/reboot/uninstall lifecycle |

`native-ci gate` means `.github/workflows/unified-agent-native.yml` now requires
the proof on relevant agent changes. The first successful workflow run is the
evidence event; the presence of the workflow file alone is not a green run.

Appliance families such as Unraid, Synology, QNAP, TrueNAS, pfSense, and
OPNsense additionally require their own persistence and service-manager lab
proof. Reusing a Linux or FreeBSD binary does not establish appliance lifecycle
support.

## Release trust boundary

All release artifacts continue to require Pulse checksum, SSHSIG, and Ed25519
verification. Those controls protect download integrity and the self-updater,
but they do not replace platform-native identity:

- macOS distribution still requires Developer ID signing and Apple
  notarization before the normal-user experience can be called release-grade.
- Windows distribution still requires Authenticode signing before the
  normal-user experience can be called release-grade.

Immutable candidate assembly now has native macOS and Windows signing jobs,
and normal release promotion requires them. The remaining external evidence is
a successful run with the repository's Developer ID/notary and Authenticode
credentials configured; missing credentials fail the release instead of
silently publishing unsigned desktop binaries.
