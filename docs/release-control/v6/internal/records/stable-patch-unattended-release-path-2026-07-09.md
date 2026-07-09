# Stable Patch Unattended Release Path - 2026-07-09

## Scope

Establish the durable gate that a routine stable patch requires minutes of
operator attention, one exact-SHA preflight, one publish dispatch, awaited demo
deployment, and one definitive release verdict.

## v6.0.5 Evidence

- Three release runs (`29013413583`, `29016263854`, and `29019195340`) each
  spent approximately 37 to 46 minutes before failing integration tests. The
  exact candidate SHA had no mandatory release preflight, so the operator paid
  that cost inside the public-release operation on every attempt.
- The successful public `v6.0.5` release pipeline run `29022145812` took
  approximately two hours. Docker publication and demo deployment were
  dispatched asynchronously with `continue-on-error`, so its green result was
  not a definitive release verdict.
- Demo update run `29032637236` joined the business tailnet as an ephemeral
  `tag:infra` node but used Tailscale `1.42.0`; all 18 `ssh-keyscan` attempts
  then timed out without a Tailscale peer-propagation ping or TCP/22 diagnosis.
- The active business-tailnet policy was inspected on 2026-07-09. The OAuth
  credential `github-actions-infra` is active with all scopes, `tag:infra` is
  owned by `autogroup:admin`, and the ACL explicitly allows `tag:infra`
  sources to reach `tag:infra` destinations on TCP 22 and 443. The demo host
  is online at its governed Tailscale address and local TCP/22 succeeds. The
  external OAuth/tag/ACL configuration is therefore not the failing boundary.
- Post-correction verification run `29043832292` joined the tailnet on commit
  `910418c3b2a165ef5cfb136b144e8ccd95b5d264`, but the configured demo target
  was absent from the runner peer map for the full bounded wait. The
  `demo-stable` environment's `DEMO_SERVER_HOST` secret was refreshed at
  `2026-07-09T19:28:27Z`, between that failure and the next run. Repository
  commit `f8bbae2f34d087887336c0c4c065d071d016821f` added only identity
  diagnostics to the reachability helper; it did not change connection or
  authorization behavior. Repository-scoped Tailscale OAuth secrets were
  unchanged during this A/B interval.
- Verification-only run `29044914999` then succeeded. Its GitHub-hosted runner
  joined the expected tailnet with `tag:infra`, saw the demo peer online and
  active, established a direct Tailscale ping and TCP/22 connection, verified
  the SSH host identity, and completed runtime, frontend, public-health, and
  browser checks. This isolates the external correction to the environment's
  demo-target configuration, not OAuth scopes, tags, ACLs, DNS, or `sshd`.
  GitHub does not expose the superseded secret value, so the evidence proves
  the configuration boundary and change event without claiming a redacted
  historical value.
- The workflow exposed no peer-map, Tailscale ping, or TCP/22 evidence. The
  operator had to infer a network failure from blind host-key retries, inspect
  Tailscale policy separately, and deploy a signed installer over local SSH.
- `https://demo.pulserelay.pro/api/version` reported stable `6.0.5`, and
  `/api/health` reported healthy before the workflow correction.

## Avoidable Delay and Intervention Inventory

1. Release tests ran after the operator crossed the publication boundary
   instead of as a recent exact-SHA prerequisite.
2. The stable resolver required RC lineage for every normal patch, forcing
   routine low-risk fixes through RC ceremony or a misleading hotfix exception.
3. The general release helper prompted interactively for metadata already
   derivable from the repository and release packet.
4. Docker publication and demo deployment were detached follow-on dispatches;
   the top-level result could not answer whether the release was operational.
5. The demo path used an obsolete Tailscale client without waiting for
   eventually consistent peer propagation.
6. Eighteen blind SSH host-key retries consumed about six minutes while hiding
   whether the failing layer was tailnet policy, peer visibility, TCP/22, or
   `sshd`.
7. Demo recovery required manual SSH and a locally materialized signed
   installer after the release workflow had already reported success.

## Repository Correction

- Both demo workflows use the pinned Tailscale GitHub Action v4 client and its
  target `ping` readiness input.
- Shared diagnostics distinguish tailnet reachability from TCP/22 and SSH host
  key failures without printing credentials.
- `Update Demo Server` supports an awaited `workflow_call` and a no-mutation
  verification mode used by `Release Dry Run`.
- The release workflow awaits Docker publication and stable demo deployment,
  then emits a terminal `Definitive Release Verdict`.
- Routine stable patch release resolution no longer fabricates RC ceremony,
  but it fails closed for documented high-risk runtime changes, an existing
  same-version RC, stale rollback lineage, or failed integrated exact-SHA
  candidate checks.
- `scripts/trigger-stable-patch.sh` is the noninteractive operator entrypoint.

## End-to-End Rehearsal

- Exact-SHA `Release Dry Run` `29046383139` succeeded for
  `0d89be1c915af7bf5980637d9a050e847e74d663`. The 35-minute preflight passed
  frontend, backend, release-policy, binary-build, container-build, and
  integration checks, then awaited the reusable no-mutation demo workflow.
- The chained demo verification succeeded in 66 seconds. The runner joined
  `tawny-powan.ts.net` with `tag:infra`, observed the demo peer online and
  active, reached it directly over Tailscale and TCP/22, verified SSH identity,
  confirmed runtime version `6.0.5`, checked frontend parity and public health,
  and passed the public browser smoke test.
- Mutation-only steps were skipped. Release `v6.0.5` remained published at
  `2026-07-09T14:36:38Z` with 213 assets and no asset newer than
  `2026-07-09T14:36:37Z`; no new public Pulse release was created.
- Independent post-run checks confirmed
  `https://demo.pulserelay.pro/api/version` still reports stable `6.0.5` and
  `https://demo.pulserelay.pro/api/health` reports `healthy` with all declared
  dependencies healthy.

## Required External Configuration

Keep the `demo-stable` environment's `DEMO_SERVER_HOST` secret set to the
current Tailscale IPv4 assigned to `pulse-relay` in the business-tailnet admin
console. The repository now proves this mapping before SSH and fails with
peer-map, ping, and TCP/22 diagnostics if it drifts. No OAuth, tag-owner, or ACL
change is required for the currently verified `tag:infra` path.

## Current Verdict

Passed. A routine stable patch now uses one noninteractive publish dispatch
whose exact-SHA candidate checks run before publication. `Release Dry Run`
remains available for explicit no-public-release rehearsal. The publish DAG
awaits release promotion, Docker publication, stable demo deployment,
definitive public verification, and its terminal verdict; manual SSH is not
part of the standard path. The release-wide single-build timing contract is
tracked in
`docs/release-control/v6/internal/records/single-build-release-promotion-path-2026-07-09.md`.
