# Stable demo transaction and acceptance

This continues the existing update/recovery route. It was built for a single
demo workflow and first-success health; the actual host also runs Relay and
Caddy. An immediate healthy sample does not cover a delayed failure, and a
workflow queue does not exclude another repository's host deployment.

## Implemented boundary

- Both existing mutating workflows submit the same reviewed Python engine.
  Update still requires the immutable stable activation and customer-promotion
  lease and archive verification against the trusted release signing key. Only
  the signed Linux/amd64 server binary and VERSION are selected; no installer is
  executed. The demo does not serve enrolled agents, so updating its bundled
  agents/scripts or updater/service assets is not part of this runtime operation. Verification
  only never submits it. Recovery never installs a different executable.
- The independently owned systemd child holds the existing Relay deployment
  file lock through capture, atomic runtime/profile work, forward observation,
  complete restoration and recovery observation. It does not restart Relay or
  Caddy, upgrade a package, lift the Caddy hold or activate frozen commercial copy.
- Billing and `.encryption.key` are read-only to the profile helper. Before the
  first service stop or estate mutation, it requires regular, bounded,
  unambiguous billing JSON with an already provisioned `demo_fixtures`
  capability. Missing capability or unavailable/malformed state refuses the
  transaction with fixed diagnostics; it never grants the capability, drops
  integrity or rewrites secret-bearing JSON. It rechecks after quiescent capture
  before changing the environment. The application still owns HMAC/entitlement
  verification, encryption and legacy plaintext migration; this shape check
  does not authenticate a licence. Full-state snapshots/recovery preserve the
  original private billing/key estate. Native fixtures seed non-secret demo
  capability state only while provisioning their fresh empty disposable estate.
- The complete quiescent **chosen footprint**: Pulse executable, VERSION,
  unit/drop-in identity and demo data estate is retained
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
- Phase/terminal receipt I/O failure cannot gate owned stop, restoration or the
  full recovery watch. It retains captures and the original failure. Even an
  observed full restored window is **unverified** after evidence loss:
  `observation_failed`/`recovery_required` blocks a new mutation. A persistently
  unwritable receipt may leave only a nonterminal record; missing evidence is
  not converted to acceptance or retried as a new request.
- Terminal collection also observes the original systemd child's exit, without
  restarting or stopping it. `RemainAfterExit=yes` retains the closed result;
  failed units are not collected automatically. A favourable JSON replacement
  can become visible before its directory fsync fails, so it is provisional
  until the writer exits successfully. Missing, malformed or contradictory
  child evidence returns uncertainty, never a successful receipt or replay.
  The engine exits **2** on receipt loss, distinct from **1** for an observed
  failed operation; a stale `rolled_back` JSON cannot hide that loss. New
  attempts also reconcile the retained child of a prior source-bound terminal
  that requires this binding. Older receipts gain no missing exit evidence.
- Source-bound intent precedes systemd submission. An identical request reads
  its receipt and cannot start another child after a lost response. SSH loss
  does not terminate the child. TERM/INT/HUP cannot abort owned recovery. The
  45-minute runtime, 20-minute stop, 4,000-second remote observer, 4,100-second
  client and 90-minute workflow budgets cover both observation windows.

## Available validation and remaining result

`TestDemoTransactionConnectedRecoveryControls` executes the real engine and
bootstrap using a real private filesystem estate and virtual-clock command,
service, journal and HTTP adapters. Tests retain delayed 55/299-second failure,
changed executable/VERSION/data rollback, cancellation, failed and unhealthy
recovery, headroom refusal, shared-lock contention, source mismatch, journal
failure, cohost restart, source-bound repeated/lost submission and no-op proof.
Receipt faults after stop, after runtime replacement and during restoration
exercise the real filesystem engine, including persistent write loss,
post-replacement fsync loss, cancellation and failed-terminal non-replay. The
unchanged-parent control retains the adverse skipped restoration. Collection
controls wait for writer closure and reject favourable stale terminals.
The production window is not shortened. This is **not native acceptance**.

Connected billing controls also pass actual canonical encrypted and legacy
secret state through the real profile helper, retain every billing/key byte and
metadata, then use the canonical loader to verify encryption/migration and HMAC.
A tampered signature remains rejected by that loader, not laundered by the
helper. Malformed, missing, duplicate-key, nonregular and unprovisioned billing
refuse before service/binary/data mutation. Late failures retain full-state
recovery and sanitised evidence. These are synthetic source tests; they do not
establish production capability provisioning, a green CodeQL check or native
forward/reverse acceptance.

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
original and fixture hashes remain distinct. This exercises the actual narrow atomic runtime swaps with synthetic executables,
not a signed published archive or customer acceptance. Its source exists;
**no native pass is asserted by this document**. Read the exact-head terminal
job and artifact before judging that result.

Before treating this route as restored, the exact reviewed source must return
native disposable systemd/SSH-loss receipts and signed published-runtime
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

## Returned footprint defect and chosen correction (3 October 2026)

Review of ee451f8c reproduced late failure followed by `rolled_back/verified`
with the candidate `/opt/pulse/VERSION` left installed. The old tests restricted
the synthetic installer to PATHS and missed that distribution mutation.

The actual v6.4.5 installer at 99c8740785dcb6d66f7078112ae673ab45bf1b81
changes the server, VERSION, bundled agents and scripts, binary symlink,
`/usr/local/bin/update-pulse`, `pulse-auto-update.sh`, service/auto-update units,
existing timer assets/activation, config/marker state and sibling configuration
backups (including rotation). Missing-install paths can create a service account
and acquire OS dependencies. Snapshotting only a binary and /etc/pulse cannot
roll all that back; adding more partial snapshots would repeat the defect.

The existing installer and in-process updater were compared. Both include agent
distribution/helper provisioning unnecessary for this synthetic demo runtime;
the in-process updater also lacks this route's cohost exclusion and full-window
recovery. We keep existing signature verification and atomic sibling-swap
semantics but apply **only the two runtime files** and demo profile. Agents,
scripts, helpers, symlinks, backup rotation, service account, packages, unit and
timer provisioning/activation remain untouched. VERSION joins the private
snapshot and both runtime file identities are checked through forward/recovery
watches. Data rollback remains quiescent and retained; startup may naturally
write new data. This is not a claim to restore every general-installer effect.

The CI dispatcher refuses production SSH mutation until the newest matching
main native run passes. It checks ancestry and final content of the engine,
dispatcher, native driver and workflow; an unrelated later main commit does not
invalidate unchanged proof. A matching failure/pending run cannot select an
older favourable sample. Refused/malformed reads have no fallback. Only the
normal recent-first/history-fill inventory rule is used for successful reads.
Native and signed installed acceptance are still separate; absence fails closed.
