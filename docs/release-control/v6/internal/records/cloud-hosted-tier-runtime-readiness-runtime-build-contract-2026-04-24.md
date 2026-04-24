# Cloud Hosted Tier Runtime Build Contract Record

- Date: `2026-04-24`
- Gate: `cloud-hosted-tier-runtime-readiness`
- Assertion: `RA11`
- Result: `passed`
- Evidence tier: `managed-runtime-exercise`

## Follow-Up Closed

The production storage remediation uncovered that ad hoc hosted tenant runtime
image builds were coupled to the release installer signing path. Building the
server `runtime` target from source without release signing material reached
installer rendering and failed with:

`installer-ssh-public-key is required for rendered installers`

That was the wrong build boundary for Pulse Cloud tenant hotfix images. Hosted
tenant runtime images need the Pulse server runtime, entrypoint, healthcheck,
and data ownership contract, but they do not need to embed generated installer
scripts, agent binaries, or installer signing material. Those public download
endpoints can continue to proxy canonical release assets when local image
artifacts are absent.

## Canonical Fix

`Dockerfile` now splits the build graph into these boundaries:

1. `backend-builder` builds only the Pulse server binaries and embedded
   frontend.
2. `release-assets-builder` derives from `backend-builder` and owns unified
   agent binaries, rendered installers, and detached signature sidecars.
3. `pulse-runtime-base` owns the shared Pulse server runtime filesystem,
   entrypoint, healthcheck, user, and data directories.
4. `hosted_runtime` derives from `pulse-runtime-base` and intentionally does
   not copy rendered installers or embedded agent binaries.
5. `runtime` derives from `pulse-runtime-base` and still copies signed
   release installer and agent assets from `release-assets-builder`.
6. `agent_runtime` still depends on `release-assets-builder`.

Release builds that declare `PULSE_UPDATE_SIGNING_PUBLIC_KEY` still fail closed
unless the matching update signing secret is mounted. Non-release smoke builds
that do not declare that expected public key can render unsigned local installer
placeholders for the full self-hosted `runtime` target, but hosted tenant
hotfix builds can avoid that release-asset path entirely with:

`DOCKER_BUILDKIT=1 docker build --target hosted_runtime -t pulse-hosted-runtime:<tag> .`

## Proof

- `go test ./scripts/installtests -run TestDockerAndDemoBuildsUseCanonicalReleaseLdflags -count=1`
- `python3 scripts/release_control/status_audit.py --pretty`
- Docker build proof for the isolated `hosted_runtime` target was run on
  `pulse-cloud` from a clean temporary context assembled from `HEAD` plus the
  staged Dockerfile patch, so unrelated local working-tree edits did not affect
  the result:
  - `DOCKER_BUILDKIT=1 docker build --progress=plain --target hosted_runtime -t pulse-hosted-runtime-contract:d5513d479-20260424T100117Z .`
  - image inspection passed: `/app/pulse`, `/docker-entrypoint.sh`, and
    `/docker-healthcheck.sh` existed; `/opt/pulse/scripts/install.sh` and
    `/opt/pulse/bin` did not exist.
- The full self-hosted `runtime` target was also built without signing secrets
  from the same clean-context model:
  - `DOCKER_BUILDKIT=1 docker build --progress=plain --target runtime -t pulse-runtime-contract:d5513d479-20260424T100740Z .`
  - the render-installer step used
    `--allow-empty-installer-ssh-public-key` because no expected signing public
    key was declared.
  - image inspection passed: `/app/pulse`,
    `/opt/pulse/scripts/install.sh`, signature sidecars, and
    `/opt/pulse/bin/pulse-agent-linux-amd64` existed.
- Both proof images, temporary source contexts, and BuildKit cache were removed
  from `pulse-cloud` after verification. The host returned to `13G` used,
  `142G` available, `9%` full, with `0B` Docker build cache.

## Conclusion

The hosted tenant runtime image build contract is no longer blocked by
installer signing material. The official self-hosted release image keeps the
signed installer/agent asset path, while production Pulse Cloud tenant runtime
hotfix images have a dedicated target that stays scoped to hosted runtime
execution.
