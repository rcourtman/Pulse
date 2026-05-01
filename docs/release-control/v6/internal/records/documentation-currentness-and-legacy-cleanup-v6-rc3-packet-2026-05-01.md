# Documentation Currentness And Legacy Cleanup v6 RC3 Packet Record

- Date: `2026-05-01`
- Gate: `documentation-currentness-and-legacy-cleanup`
- Scope:
  - `pulse`
  - lane `L9`

## Context

The late RC3 audit compared recent v5.1.29 maintenance fixes, open GitHub
issues, and v6 release-branch evidence. The branch now needs a current RC3
packet so operator guidance matches the candidate being prepared instead of
leaving the top-level release note pointer on the older RC2 draft.

## Review Surface

1. `VERSION`
2. `docs/RELEASE_NOTES.md`
3. `docs/releases/RELEASE_NOTES_v6_RC3_DRAFT.md`
4. `docs/releases/V6_CHANGELOG_RC3_DRAFT.md`
5. `docs/releases/V6_RC3_OPERATOR_SUPPORT_PACK_DRAFT.md`
6. `docs/releases/V6_PRERELEASE_RUNBOOK.md`
7. `docs/UPGRADE_v6.md`
8. `docs/release-control/v6/internal/subsystems/deployment-installability.md`
9. `docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-blocked-2026-04-04.md`
10. `.github/workflows/create-release.yml`
11. `scripts/release_control/resolve_release_promotion.py`
12. `scripts/release_control/release_promotion_policy_test.py`

## Outcome

The in-repo release packet now reflects the current `v6.0.0-rc.3` candidate:

- `VERSION` is set to `6.0.0-rc.3`, matching the release workflow version
  preflight requirement.
- `docs/RELEASE_NOTES.md` points current v6 prerelease readers at the RC3
  packet while keeping the RC2 draft as historical context.
- The RC3 release notes, changelog, and operator support pack describe the
  late v5.1.29 maintenance ports, the current RC issue fixes, and the areas
  operators should re-test.
- The packet records `v5.1.29` as the stable rollback target with
  `./scripts/install.sh --version v5.1.29`.
- The packet explicitly warns that systems pinned to the historical `rc.2`
  update trust root should use manual reinstall or explicit trust migration
  rather than assuming unattended continuity into later prerelease or GA builds.
- `docs/UPGRADE_v6.md` and the prerelease runbook now point current prerelease
  operators at the RC3 packet instead of leaving the proof surface on RC2.
- The deployment-installability contract records that later corrective RC
  packets must carry the live rollback target and any trust-root continuity
  caveat before the release workflow is dispatched.
- The RC-to-GA blocked record now reflects that the current line is preparing
  `VERSION=6.0.0-rc.3`, not a `6.0.0` GA candidate.

No GitHub release, public issue comment, retitle, closure, or other external
state change was made as part of this packet preparation.

## Proof

- `python3 scripts/release_control/status_audit.py --pretty`
  - `repo_ready=True`
  - `rc_ready=True`
  - `release_ready=True`
  - `current_version=6.0.0-rc.3`
- `python3 scripts/release_control/documentation_currentness_test.py`
  - passed, 6 tests
- `python3 scripts/release_control/release_promotion_policy_test.py`
  - passed, 17 tests
- `python3 scripts/release_control/render_release_body_test.py`
  - passed, 3 tests
- `python3 scripts/release_control/resolve_release_promotion.py --version 6.0.0-rc.3 --rollback-version v5.1.29`
  - `rollback_tag=v5.1.29`
  - `rollback_command=./scripts/install.sh --version v5.1.29`
