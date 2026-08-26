# Pulse v6.4.0-rc.5 Release Notes

`v6.4.0-rc.5` is a release candidate for the next v6 minor release. It makes API-token removal durable, exposes notification decisions more clearly, and moves alert lifecycle handling onto a deterministic transition core.

## What's improved

- **Clearer notification evidence** — Active alerts show delivery status, and the delivery activity view includes held notifications and their suppression reasons.
- **More reliable alert lifecycles** — Metric, availability, health, posture, backup, snapshot, storage, and update alerts now share a deterministic transition core with pinned confirmation, recovery, acknowledgement, and re-fire behavior.
- **Safer agent setup** — Agent install commands reveal their newly generated token separately, making it easier to copy the credential without mixing it with the shell command.
- **Better filesystem feedback** — Filesystem history drawers again show loading progress while a longer range is fetched.

## Fixes

- API-token deletion now persists the reduced inventory atomically and restores the complete prior live inventory if saving fails, instead of reporting success with inconsistent credentials ([#1783](https://github.com/rcourtman/Pulse/issues/1783)).
- Alert delivery diagnosis now distinguishes pending, sent, failed, and suppressed outcomes without requiring operators to infer delivery from an active alert alone.
- Confirmation-based alerts retain the first matched observation as their occurrence start, and new confirmation runs no longer inherit stale timestamps.
- Canonically keyed metric alerts resolve through the same identity used to create them instead of remaining stale after recovery.
- Stateful alert manual clears preserve acknowledgement and recent-resolution behavior so a quick refire does not duplicate history.
- Guest and host memory percentages use the same cache-aware basis in operator-facing explanations.

## Before you upgrade

- This is a release candidate. Stable installations remain on v6.3.2 unless an operator explicitly selects this version.
- Existing configurations remain valid and no manual data migration is required.
- Pulse Mobile does not consume the changed alert transition internals or browser delivery evidence. Existing mobile, Relay, onboarding, pairing, push, and mobile-facing API contracts are unchanged, so no companion update is required.
- Windows Unified Agent binaries are checksum- and detached-signature-verified but are not Authenticode-signed, so Windows may show an Unknown Publisher warning.

## Known issues

- Windows Authenticode signing remains unavailable for this candidate; use the published checksum and detached signature when verifying Windows agent downloads.
