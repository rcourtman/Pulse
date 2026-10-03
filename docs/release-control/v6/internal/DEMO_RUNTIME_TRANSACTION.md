# Stable demo transaction and acceptance

This continues the existing update/recovery route. It was built for a single
demo workflow and first-success health; the actual host also runs Relay and
Caddy. An immediate healthy sample does not cover a delayed failure, and a
workflow queue does not exclude another repository's host deployment.

## Implemented boundary

- Both existing mutating workflows submit the same reviewed Python engine.
  Update still requires the immutable stable activation and customer-promotion
  lease, exact tagged installer and trusted release signing key. Verification
  only never submits it. Recovery never installs a different executable.
- The independently owned systemd child holds the existing Relay deployment
  file lock through capture, installation/profile work, forward observation,
  complete restoration and recovery observation. It does not restart Relay or
  Caddy, upgrade a package, lift the Caddy hold or activate frozen commercial copy.
- The complete quiescent Pulse executable/unit/drop-in/data estate is retained
  privately before mutation. No backup or database is deleted for space. The
  unhealthy demo-only route retains the original generated operational history
  before its existing bounded-profile reset. No snapshot is automatically erased.
- Readiness has a separate 60-second limit. Forward, healthy-noop and restored
  paths require 300 seconds of actual local/public/Relay HTTP 200, expected
  version, stable service PID/restarts and available new-journal observations.
  Transport errors, redirects, missing state/journals and new crashes fail.
  Recovery uses fresh cursors; a retained forward failure remains a failure.
- Failed/unhealthy/uncertain recovery remains owned and blocks subsequent
  mutation. A prior failed baseline is not a healthy rollback. No crash dump is
  induced. Receipt fields contain fixed diagnostics and identities, never raw
  journal/HTTP bodies, environment values or snapshot contents.
- Source-bound intent precedes systemd submission. An identical request reads
  its receipt and cannot start another child after a lost response. SSH loss
  does not terminate the child. TERM/INT/HUP cannot abort owned recovery. The
  45-minute runtime, 20-minute stop, 4,000-second remote observer, 4,100-second
  client and 90-minute workflow budgets cover both observation windows.

## Available validation and remaining result

`TestDemoTransactionConnectedRecoveryControls` executes the real engine and
bootstrap using a real private filesystem estate and virtual-clock command,
service, journal and HTTP adapters. Tests retain delayed 55/299-second failure,
changed executable/unit/data rollback, cancellation, failed and unhealthy
recovery, headroom refusal, shared-lock contention, source mismatch, journal
failure, cohost restart, source-bound repeated/lost submission and no-op proof.
The production window is not shortened. This is **not native acceptance**.

`demo-runtime-native.yml` now carries the next executable acceptance rather
than requiring a worker to obtain system mode: exact PR/main source, a fresh
secret-free public hosted runner, actual systemd/Caddy and disposable trusted
TLS. The successful changed-executable step observes the full 300 seconds. A
different executable and data then fail at 55 seconds; the original observer
is lost, the same retained request observes its child without replay, TERM is
sent during restoration, and the old executable/data must return through a
full real-clock recovery window. Sanitised artifacts bind control/driver/
engine/bootstrap identity, service PID/restart observations and cleanup. The
fixture rebinds only Relay's fixed health URL to its local Caddy endpoint; the
original and fixture hashes remain distinct. This is a synthetic installer,
not signed published-installer or customer acceptance. Its source exists;
**no native pass is asserted by this document**. Read the exact-head terminal
job and artifact before judging that result.

Before treating this route as restored, the exact reviewed source must return
native disposable systemd/SSH-loss receipts and signed published-installer
forward/reverse data/identity results. Then reconcile actual demo workflow and
public browser readback through the release owner; source tests do not establish
an installed recovery. Delivery owns that continuation and the remaining ingress
migration. Unrestricted root/Tailscale administration can still bypass a source
transaction; this candidate is not host enforcement. Workers neither install
policy/credentials nor replay a successful publication to test it.

Retained failed/unknown estates require the operational owner's reconciliation;
do not remove a lock, intent or snapshot merely to permit a fresh sample. A
later frontend/network failure is not authority for a second SSH session to
stop a committed or unrelated service. Keep the signed packet, failure and
native/browser acceptance distinct in the release judgment.
