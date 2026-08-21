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

## Definitive cut and recovery lineage

The definitive release used exact candidate SHA
`1327dddad5200f07271e16abdf4dd83fa1f2eb4f` in source run `32502098673` and
private staging child `32502128732`. Every immutable gate passed, including
native signing, exact-candidate container and Helm qualification, backend
tests, frontend checks, installer smoke, public image publication, private
packet staging, and release readiness. The release remained quarantined when
its first convergence owner, run `32503887753`, ended in GitHub
`startup_failure` before creating a job.

Activation-only recovery first exposed a stale duplicate list of historical
job display names. Commit `1ef8797d28746e102cae2ffe7ddb768d8cbfd38d`
replaced that parallel catalog with the canonical successful
`release_readiness` DAG join, retained the all-jobs failure verdict, and added
the `actions: read` permission required by the Helm Pages reusable workflow.
Recovery run `32504507283` then revalidated the unchanged candidate manifest,
published the exact candidate at `2026-08-21T16:45:10Z`, and uploaded the
irreversible activation marker.

Post-publication convergence exposed two further control-path defects without
changing the release candidate:

- Helm Pages ran from a nested `gh-pages` checkout, so repository-discovering
  `gh release` commands failed. Commit
  `b16b8e5242505b7b59d193b980e5c178a9737b2c` binds every read and write to
  `${GITHUB_REPOSITORY}` explicitly.
- The independent paid-runtime public-boundary proof called the tailnet-only
  license endpoint from a sibling hosted job that had not joined Tailscale.
  `pulse-pro` commit `33f96418cc9d0f8c6f6a16b4075767621845ccba`
  adds the same pinned tailnet setup as the broker mutation and enforces its
  placement in the private distribution validator.

Final convergence successor `32505105536` completed successfully at
`2026-08-21T16:54:19Z`. Docker aliases, Helm Pages, and private paid-runtime
promotion all passed; private child `rcourtman/pulse-pro` run `32505155126`
used the corrected private workflow and passed. The release has 221 assets,
including `release-activation.json` and immutable convergence-owner records.
The annotated `v6.3.0-rc.5` tag peels to the exact candidate SHA. The public
Helm index serves `6.3.0-rc.5`, and both Docker Hub `:rc` aliases match their
exact-version OCI index digests.

## Performance verdict

The definitive source run was dispatched at `2026-08-21T16:16:43Z`. Backend
qualification completed at `16:37:24Z`, publication committed at `16:45:10Z`,
and final customer convergence completed at `16:54:19Z`. That is 28 minutes
27 seconds to publication and 37 minutes 36 seconds to definitive convergence.
The 15-minute objective was not met.

Backend qualification remained the source-run critical path at 19 minutes
37 seconds. Its two exact disjoint API batches passed, but the larger batch
occupied roughly 13 minutes after compile and setup. Activation-only recovery
took 33 seconds and the final corrected convergence successor took 2 minutes
46 seconds, demonstrating that rebuild-free recovery and the post-publication
parallel fan-out are no longer the primary timing constraint. The warm
dispatch-to-definitive-convergence objective remains open until a complete
release measures 15 minutes or less without weakening exact-SHA
qualification, signing, installer smoke, public/private integrity, or
convergence verification.

## Remaining critical-path decomposition

The definitive run proves that backend work is not the only path that must be
shortened. Even with a zero-duration backend gate, the private staging child
finished 16 minutes 52 seconds after source dispatch and public exact-version
Docker publication finished 17 minutes 34 seconds after dispatch.

The private child spent 148 seconds uploading and 214 seconds downloading an
uncompressed 1.24 GB dual-repository compiled payload. That payload included
the complete public frontend, MCP, server, and control-plane matrix even though
Pro archive assembly consumes only the public Unified Agent matrix plus five
Pro server binaries. Its hosted job then spent another 99 seconds deleting
preinstalled SDKs and unrelated cached images despite starting with 12 GB free.
The corrected handoff requests the canonical Pro-packaging profile, omits those
unused public products, enables compressed artifact transfer, uses the current
artifact client, and performs heavyweight disk cleanup only below an explicit
8 GiB safety floor. Exact Pulse and pulse-enterprise SHA manifests remain the
authority on both sides of the credential boundary.

The first non-publishing exact-SHA proof of that lean profile failed before Pro
compilation because the public server packages require
`frontend-modern/dist` as an embed source even though Pro never transfers the
frontend as a standalone payload. The corrected profile builds that exact-SHA
embed prerequisite concurrently with the public-agent matrix, leaves it in the
source checkout for the five Pro builds, and still excludes it from the
manifest-covered cross-repository payload. A governed regression test preserves
the distinction between a required local compile input and an unused transfer
product.

The first public rc.6 dispatch then exposed the complementary warm-cache race:
agent and MCP compilation advanced far enough to launch a public server while
Vite was still producing the embed directory. The source run failed in 57
seconds, before candidate upload or publication. The final scheduler retains
frontend overlap with all agent and MCP targets but joins successful frontend
completion before launching the first server or control-plane target, covering
both the lean Pro profile and the full public matrix without serializing the
independent work.

The next exact-SHA run passed the full public compiler, both backend race
shards, private staging, candidate validation, container and installer smoke,
and both public Docker publications. It then refused activation because the
frontend hot-path source guard still opened the retired root
`internal/api/resources.go` path after the production resource service moved
to `internal/api/resourceapi/resources.go`. All 20,579 executable frontend
tests passed; only the stale source-location assertion failed. The guard now
follows the canonical production owner so future decomposition cannot leave a
false release blocker behind.

Public Docker publication took 5 minutes 1 second, but its prior dependency
shape did not make it eligible until 12 minutes 33 seconds after dispatch. The
corrected DAG makes exact-version Docker staging eligible as soon as the
immutable candidate exists, which was 8 minutes 56 seconds after dispatch in
the definitive run. Release-line validation accepts the anticipated tag only
when it is bound to the exact 40-character source SHA on the governed release
branch and rejects an existing tag at any other commit. Candidate container
qualification, draft validation, Docker staging, and every other immutable
gate still meet at `release_readiness`; only that join can open durable
convergence and activation. On the observed rc.5 timings, the dependency change
alone moves projected Docker completion to approximately 13 minutes 57 seconds.

These projections do not close the objective. The release-acceleration claim
remains open until the private payload reduction is measured, the production
API decomposition produces a controlled PVE improvement, and a fresh exact-SHA
release completes definitive customer convergence in 15 minutes or less.

The rc.5 runner telemetry also explains why the backend used the slow path:
its one-time planner sample occurred while both credential-free release
compilers were resident and only 5,986 MiB remained available. That sample
permanently selected one API shard even though the compilers were short-lived.
The corrected admission policy waits at most 120 seconds for that useful
sibling work to release memory, then requires 14 GiB of available headroom for
two observed-size race binaries and the concurrent non-root package graph. If
the worker still cannot supply that floor, the gate reports a capacity failure
at admission instead of entering a one-shard path already known to approach
the 20-minute job timeout.

The public Docker publisher also serialized two independent consumers of the
same verified container payload: the main server image followed by the provider
control-plane image and their attestations. The corrected publisher expresses
those products as a two-leg, fail-independent matrix. Both legs repeat the
exact-checkout, release-line, and candidate-manifest verification before using
their product-specific prebuilt target; the reusable workflow still joins both
results before `release_readiness`. This removes the observed control-plane
assembly and attestation steps from the server-image critical path without
changing any tag, registry, provenance, SBOM, or activation contract.
