# Windows Authenticode Unavailable Owner Policy

- Decision date: `2026-08-25`
- Effective from: `v6.3.2`
- Failed release run: `32896554952`
- Restoration owner: release owner

## Owner Decision

Release run `32896554952` reached the SignPath production `release-signing`
submission after building and uploading the exact-SHA unsigned Windows agent
artifact. SignPath rejected the submission with `Invalid request to SignPath
API` before returning a signing request ID, so no request record exists to
approve or collect on a failed-job rerun.

On 2026-08-25, the release owner directed stable releases from `v6.3.2` onward
to skip Windows Authenticode while SignPath production credentials and
certificate authorization remain unavailable. New stable versions do not need
individual unsigned-Windows allowlist changes while this standing unavailable
state remains active.

Signing must not be restored automatically or merely because external account
state changes. The release owner will explicitly confirm when production
credentials and certificate authorization are ready; restoring Authenticode
then requires a reviewed policy and code change.

## Required Integrity Controls

The unavailable state changes only the Authenticode requirement. Windows
artifacts must still be built from the exact release SHA and bound to the
immutable candidate manifest, SHA-256 checksums, detached `.sig` and `.sshsig`
signatures, and published-digest verification. Public release notes must state
that the Windows binaries are not Authenticode-signed and may display an
Unknown Publisher warning.

The normal integrated frontend, backend, installer, Docker, Helm, macOS signing
and notarization, candidate validation, publication, activation, and convergence
controls remain unchanged.
