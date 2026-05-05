# Documentation Currentness And Legacy Cleanup v6 RC4 Packet Record

- Date: `2026-05-05`
- Gate: `documentation-currentness-and-legacy-cleanup`
- Scope:
  - `pulse`
  - lane `L9`

## Context

Before publishing `v6.0.0-rc.4`, the current release packet needed to move from
the shipped `rc.3` packet to a new packet that describes the exact
post-`rc.3` release-branch delta. The local `v6.0.0-rc.3` tag was stale during
the initial check, so the local tag was first refreshed from origin and the
packet audit was based on the published GitHub prerelease tag target.

## Reviewed Range

- From tag: `v6.0.0-rc.3`
- From commit: `f1744d36d0bde3c8735ae75a190af45c35087841`
- To validation commit: `1a3e5ec27d7d1c59f8b19e4a48c4ce83cac31bb9`
- Git range: `v6.0.0-rc.3..1a3e5ec27d7d1c59f8b19e4a48c4ce83cac31bb9`
- Commit count: `54`
- Date span in the range: `2026-05-03` through `2026-05-05`
- Changed scope: `343` files, `16760` insertions, `11434` deletions

## Review Method

The audit checked the remote `v6.0.0-rc.3` tag target, corrected the stale local
tag reference, read the full commit subject range in chronological order,
reviewed the aggregate diff scope, and grouped the commits by changed surface
and release risk.

Commands used for the coverage pass:

- `git ls-remote origin 'refs/tags/v6.0.0-rc.3*'`
- `git fetch origin +refs/tags/v6.0.0-rc.3:refs/tags/v6.0.0-rc.3`
- `git rev-list -n1 v6.0.0-rc.3`
- `git rev-list --count v6.0.0-rc.3..HEAD`
- `git log --reverse --format='%h%x09%H%x09%ad%x09%s' --date=short v6.0.0-rc.3..HEAD`
- `git diff --shortstat v6.0.0-rc.3..HEAD`
- `git diff --name-only v6.0.0-rc.3..HEAD`

## Commit Coverage Summary

The 54 commits were covered by these release-note buckets:

- hosted tenant identity keys, hosted signup owner IDs, hosted handoff
  identity, SSO stable principals, checkout magic-link principals, blank
  magic-link handling, ambiguous email principal handling, contact-email
  takeover prevention, API token owner metadata, organization runtime access,
  workspace-owner proof, Stripe webhook principal fixtures, and strict
  organization identity invariants
- API-first action planning, action-decision API, CLI action planning, CLI
  action capability discovery, CLI action audit reads, CLI fleet connection
  reads, persisted action plans, action execution safety contract, AI action
  audit lifecycle alignment, and fail-closed dry-run execution
- self-hosted licensing continuity, removal of monitored-system volume caps,
  and prevention of raw-cap writes in continuity state
- root-agent service defaults, API-first Proxmox onboarding, Proxmox setup and
  runtime token ACLs, Proxmox snapshot polling, Proxmox guest memory fallback,
  TrueNAS CORE agent supervisor restart, mdadm fallback discovery, Ceph pool
  threshold identity, storage primary issue impact handling, and metrics
  rollup write amplification
- Workloads empty-state source detection, Patrol mobile header controls, public
  demo admin read hiding, mock-mode legacy sidecar cleanup, mobile Relay docs
  label cleanup, release key helper module path, and Agent Security docs
  currentness
- RC4 packet preparation, RC4 Docker default correction, release-validation
  test alignment for retired monitored-system caps, and tenant monitor
  WebSocket-hub nil handling

## Packet Updates

The current RC packet now points at `v6.0.0-rc.4`:

- `VERSION`
  - records `6.0.0-rc.4`
- `docs/RELEASE_NOTES.md`
  - points current v6 prerelease readers at the RC4 packet
  - keeps RC3 as historical in-repo packet context
- `docs/UPGRADE_v6.md`
  - points upgrade readers at the RC4 release notes, changelog, and operator
    support pack
- `docs/releases/RELEASE_NOTES_v6_RC4_DRAFT.md`
  - records the exact post-`rc.3` commit range and candidate head
  - summarizes identity, API/CLI, action governance, self-hosted licensing,
    agent setup, monitoring, storage, Patrol, and mock-mode changes
- `docs/releases/V6_CHANGELOG_RC4_DRAFT.md`
  - groups the 54-commit release range into release-risk buckets
- `docs/releases/V6_RC4_OPERATOR_SUPPORT_PACK_DRAFT.md`
  - gives support staff the current rollback, trust-root, identity, action,
    Proxmox, and escalation language
- `docs/releases/V6_PRERELEASE_RUNBOOK.md`
  - updates the active RC example to `6.0.0-rc.4`
- `docs/release-control/v6/internal/subsystems/deployment-installability.md`
  - records that RC4 follows the later-corrective-RC packet requirements for
    rollback, trust-root continuity, and release-control evidence
- `docker-compose.yml`
  - pins the default Pulse image tag to `6.0.0-rc.4`
- `scripts/install-docker.sh`
  - pins the standalone installer fallback image version to `6.0.0-rc.4`
- `internal/config/billing_state_test.go`
  - asserts that billing-state normalization scrubs retired monitored-system
    limit metadata instead of preserving it as entitlement state
- `tests/migration/v5_full_upgrade_test.go` and
  `tests/migration/v5_real_exchange_upgrade_test.go`
  - align v5-to-v6 exchange proof with the canonical self-hosted contract:
    migrated self-hosted plans preserve plan identity and paid continuity
    without reintroducing monitored-system caps

## Outcome

The audit did not identify a new unhandled code blocker from the feature/runtime
commit range. It did identify a release-packet currentness gap because the repo
still pointed operators at `rc.3` while the branch had moved beyond the
published `rc.3` tag. The initial draft release workflow also exposed two
release-validation gaps before publication:

- Docker Compose and turnkey Docker install defaults still pinned the RC3 image
  tag.
- Backend migration and billing-state tests still expected the retired
  `max_monitored_systems` contract to survive in self-hosted entitlement state.

Both gaps are addressed before publishing RC4. The corrected tests assert the
canonical root contract: self-hosted v6 plans preserve paid continuity without
monitored-system volume caps, and `max_monitored_systems` remains only as
legacy metadata to scrub.

The second draft release workflow exposed one more backend release-validation
blocker before publication: tenant monitor startup can run in headless test
contexts without a WebSocket hub, but the state broadcast helper only checked a
plain nil interface and still dereferenced a typed nil `*websocket.Hub`. The
monitor broadcast contract now treats typed nil broadcasters as absent, and the
regression proof covers the tenant-scoped broadcast path.

No public issue comment, retitle, closure, or customer-facing message was made
as part of this packet update.

## Verification

- `git diff --check -- VERSION docs/RELEASE_NOTES.md docs/UPGRADE_v6.md docs/releases/V6_PRERELEASE_RUNBOOK.md docs/releases/RELEASE_NOTES_v6_RC4_DRAFT.md docs/releases/V6_CHANGELOG_RC4_DRAFT.md docs/releases/V6_RC4_OPERATOR_SUPPORT_PACK_DRAFT.md docs/release-control/v6/internal/records/documentation-currentness-and-legacy-cleanup-v6-rc4-packet-2026-05-05.md docs/release-control/v6/internal/subsystems/deployment-installability.md`
- `python3 scripts/release_control/documentation_currentness_test.py`
- `python3 scripts/release_control/render_release_body_test.py`
- `PYTHONPATH=scripts/release_control python3 -m unittest scripts.release_control.release_promotion_policy_test.ReleasePromotionPolicyTest.test_release_notes_index_points_at_current_rc_packet scripts.release_control.release_promotion_policy_test.ReleasePromotionPolicyTest.test_operator_support_packs_keep_free_first_paid_continuity_wording scripts.release_control.release_promotion_policy_test.ReleasePromotionPolicyTest.test_version_file_matches_current_rc_packet scripts.release_control.release_promotion_policy_test.ReleasePromotionPolicyTest.test_upgrade_guide_points_at_current_rc_support_pack -q`
- `python3 scripts/release_control/resolve_release_promotion.py --version 6.0.0-rc.4 --rollback-version v5.1.29 --release-notes-file docs/releases/RELEASE_NOTES_v6_RC4_DRAFT.md`
- `python3 scripts/release_control/status_audit.py --pretty`
- Draft release workflow `25382802275`:
  - `prepare`, `frontend_checks`, `helm_smoke`, and `docker_build` passed
  - `backend_tests` failed on stale RC3 Docker defaults and stale retired
    monitored-system cap expectations
- Draft release workflow `25384246365`:
  - `prepare`, `frontend_checks`, `helm_smoke`, and `docker_build` passed
  - `backend_tests` failed on a tenant monitor typed-nil WebSocket hub panic in
    `internal/api`
- `go test -race ./internal/config ./tests/migration ./scripts/installtests`
- `go test -race ./internal/monitoring ./internal/api`
