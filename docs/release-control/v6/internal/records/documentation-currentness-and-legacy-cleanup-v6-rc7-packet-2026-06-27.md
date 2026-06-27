# RC7 Release Packet Currentness Record

## Scope

- From tag: `v6.0.0-rc.6`
- From commit: `c25e95cb2b071551df95c8add62773905ba0628b`
- To validation-risk commit: `fc10de9b5477613316473267b72b05b6b2b7aaff`
- Git range: `v6.0.0-rc.6..fc10de9b5477613316473267b72b05b6b2b7aaff`
- Commit count: `975`
- Date span in the range: `2026-05-27` through `2026-06-27`
- Changed scope: `1997` files, `239625` insertions, `47030` deletions

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

No public issue comment, retitle, closure, release publication, or customer
message was made as part of this packet update.

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
    `fc10de9b5477613316473267b72b05b6b2b7aaff`
  - `975` commits from `v6.0.0-rc.6`
  - `1997` files changed, `239625` insertions, `47030` deletions
