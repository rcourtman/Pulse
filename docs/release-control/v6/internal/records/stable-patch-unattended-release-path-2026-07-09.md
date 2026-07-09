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
  same-version RC, stale rollback lineage, or a missing exact-SHA dry run.
- `scripts/trigger-stable-patch.sh` is the noninteractive operator entrypoint.

## Current Verdict

Blocked pending one pushed exact-SHA `Release Dry Run` that exercises the new
no-mutation demo path on GitHub-hosted infrastructure. This record must be
updated with that run URL and the gate promoted to `passed` only after the run
and local public-demo checks both succeed.
