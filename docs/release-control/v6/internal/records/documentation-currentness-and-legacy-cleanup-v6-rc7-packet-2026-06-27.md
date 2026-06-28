# RC7 Release Packet Currentness Record

## Scope

- From tag: `v6.0.0-rc.6`
- From commit: `c25e95cb2b071551df95c8add62773905ba0628b`
- To validation-risk commit: `d796928969b0b557ef5ed2d48e0e6f5e5a197df3`
- Git range: `v6.0.0-rc.6..d796928969b0b557ef5ed2d48e0e6f5e5a197df3`
- Commit count: `979`
- Date span in the range: `2026-05-27` through `2026-06-28`
- Changed scope: `2003` files, `239767` insertions, `47168` deletions

The final release workflow dispatch may use a later metadata-only packet
refresh commit. That refresh is not counted as a new validation-risk commit
when it only updates the packet to name the last code-backed release fix.

## Outcome

The RC7 packet refresh keeps v6 on the opt-in prerelease channel and preserves
the stable rollback target as `v5.1.35`.

The branch had accumulated follow-up CI fixes after the initial RC7 packet:
Discovery disabled-state test copy, frontend bundle-size baseline drift,
Patrol-control telemetry disclosure wording, and RC7 Docker install defaults.
`Build and Test` run `28284309278` then exposed the release-installability
blocker: root `docker-compose.yml` and `scripts/install-docker.sh` still
defaulted to the stable `6.0.0` image while `VERSION` was `6.0.0-rc.7`.

The corrected packet pins the repo-root Docker Compose default and Docker
bootstrap installer fallback to `6.0.0-rc.7`. The installer proof now keeps the
stable-promotion guard version-aware: prerelease defaults are valid only when
the governed `VERSION` is prerelease, and leftover `-rc.` defaults remain a
blocker when the governed `VERSION` is stable.

The 2026-06-28 packet refresh moves the validation-risk head from the earlier
Docker-default correction to the current code-backed branch head. The newly
included commits surface deterministic capacity forecasts as finding signals,
query capacity forecast history through the metrics target ID, and sanitize
Patrol runtime failures in history.

The final RC7 release attempt exposed three release-gate issues after that
packet refresh:

1. GitHub CI runners with real `/sys/block` devices could make the hostagent
   SMART no-device test observe a real disk instead of the forced no-device
   fallback.
2. Alert manager tests could leave asynchronous active-alert/history saves
   writing into `t.TempDir()` while Go test cleanup removed the directory.
3. The published `install.sh` end-to-end smoke on a minimal Debian systemd
   container failed with `ssh-keygen is required to verify signed Pulse release
   assets` because the root installer did not bootstrap `openssh-client` before
   signed archive verification.

The corrective commits harden the hostagent no-device proof, track alert save
workers during shutdown, and make the root installer install `ca-certificates`
and `openssh-client` alongside `curl` and `wget` while treating `jq` as the
only optional install-time helper. The Pulse repository `WORKFLOW_PAT` Actions
secret was also refreshed on 2026-06-28 after the downstream private Pro
publication job failed to dispatch the existing `rcourtman/pulse-enterprise`
`Build Pro Release` workflow with the previous token.

No public issue comment, retitle, closure, or customer message was made as part
of this packet update. A failed `v6.0.0-rc.7` release workflow had already
published the earlier draft assets before the downstream install smoke and
private Pro handoff failures surfaced; final publication must therefore replace
that failed public release state with assets built from the refreshed branch
head.

## Verification

- `go test ./scripts/installtests -count=1`
- `go test -race -timeout 25m ./...`
- `python3 scripts/release_control/contract_audit.py --check`
- `git diff --check`
- No-attribution preflight for the RC7 Docker install default commit message
  and changed files.
- `Build and Test` run `28284309278`:
  - `Secret Scan` passed
  - `Frontend unit tests`, `Type-check frontend`, frontend bundle build,
    bundle-size check, and script smoke tests passed
  - `Go unit tests` failed on stale `6.0.0` Docker install defaults before the
    RC7 Docker-default correction
- Release packet head refresh:
  - validation-risk commit
    `d796928969b0b557ef5ed2d48e0e6f5e5a197df3`
  - `979` commits from `v6.0.0-rc.6`
  - `2003` files changed, `239767` insertions, `47168` deletions
- Final installability proof:
  - `go test ./scripts/installtests -run 'TestRootInstallScript(ArchiveSupportContract|InstallsSignatureVerificationDependencies)|TestInstallShSmokeWorkflowPresent|TestCreateReleasePublishesPrivateProRuntime' -count=1`
  - `python3 scripts/release_control/contract_audit.py --check`
