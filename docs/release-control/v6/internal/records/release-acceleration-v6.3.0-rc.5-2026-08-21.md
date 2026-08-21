# v6.3.0-rc.5 Release Acceleration Qualification

## Candidate identity

- Initial dispatch SHA: `4217f72bdc92ef4ec31a74ad1056551fc2848711`
- Initial release run: `32493044910`
- Private staging child: `rcourtman/pulse-enterprise` run `32493076136`
- Code-backed correction: `ae27ad751174d2b2b3160a86f127d5c087a645d1`
- Version: `v6.3.0-rc.5`
- Rollback target: `v6.2.1`

## Failed qualification attempt

The release was dispatched at `2026-08-21T14:36:24Z`. Preparation completed in
9 seconds, the frontend embed bundle completed in 61 seconds, and the Windows
PowerShell installer smoke completed in 64 seconds. Backend qualification
started at `14:37:49Z` and failed at `14:40:12Z`, 3 minutes 48 seconds after
dispatch. The failure was therefore definitive while the public mutation
boundary was still closed.

The PVE worker had 8 virtual CPUs and 5,986 MiB available while the public,
private, and backend compile lanes overlapped. The memory-aware planner
correctly selected one API shard. Its count-only batch ceiling then placed all
3,736 top-level API tests in one `-test.run` regex. Linux rejected the exec
with `Argument list too long` before any API test ran. This was a release-harness
failure, not a product test failure.

Both the public release run and its inert private staging child were cancelled.
No `v6.3.0-rc.5` Git tag or GitHub release was created, and no customer
activation or convergence owner existed.

## Correction and proof

The canonical shard planner now caps each deterministic contiguous batch by
both test count and encoded regex byte length. The default 64 KiB ceiling stays
below Linux's per-argument exec limit without forcing a second memory-heavy
race process when the worker legitimately falls back to one shard.

Proof on the exact compiled PVE API test inventory:

- 3,736 top-level tests discovered
- one shard retained
- three ordered batches: 1,288, 1,327, and 1,121 tests
- largest encoded regex: 65,520 bytes
- reconstructed batch order exactly matched the compiled binary's emitted
  order

The focused Python release-preflight suite passed all nine tests with one
intentional skip. The Go release-asset contract test passed. Shell syntax,
contract, status, and registry audits passed. The complete repository
pre-commit gate passed, including secret and sensitivity scans, canonical
completion enforcement, governance guardrails, and release-control tests.

## Retry boundary

The next release dispatch must use an exact pushed `main` SHA that descends
from `ae27ad751174d2b2b3160a86f127d5c087a645d1`. It must build and qualify a
new immutable candidate and a new private staging packet; artifacts or prefixes
from the cancelled attempt are not retry inputs. The warm dispatch-to-definitive
convergence objective remains open until a complete release measures 15 minutes
or less without weakening exact-SHA qualification, signing, installer smoke,
public/private integrity, or convergence verification.
