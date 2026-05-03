# Documentation Currentness And Legacy Cleanup RC3 Commit Audit Record

- Date: `2026-05-03`
- Gate: `documentation-currentness-and-legacy-cleanup`
- Scope:
  - `pulse`
  - lane `L9`

## Context

Before publishing `v6.0.0-rc.3`, the release packet needed to be checked
against the actual post-`rc.2` release range rather than only the late RC3
maintenance commits. The existing draft changelog existed, but it primarily
described the v5.1.29 maintenance ports and current RC issue follow-up. That
was accurate but under-scoped for the full `rc.2` to `rc.3` delta.

The packet was refreshed again after the later RC3 candidate commits that made
self-hosted SSO a Community-tier capability, audited SSO provider settings,
and fixed stable installer prerelease selection.

## Reviewed Range

- From tag: `v6.0.0-rc.2`
- From commit: `2868b44cf91b59bca85cd886711d78cd3c376fab`
- To candidate commit: `c27814d1901ec59fad510dfb5c57358dfa6525b1`
- Git range: `v6.0.0-rc.2..c27814d1901ec59fad510dfb5c57358dfa6525b1`
- Commit count: `603`
- Date span in the range: `2026-04-16` through `2026-05-03`
- Changed scope: `1765` files, `113745` insertions, `72725` deletions

## Review Method

The audit read the full commit subject range in chronological order, checked
the tag boundaries, reviewed the aggregate diff scope, and grouped the commits
by changed surface and release risk. The range was not treated as only a v5
maintenance port because many post-`rc.2` commits changed release packaging,
security, commercial posture, hosted/mobile proof, governance surfaces, and
frontend layout behavior.

Commands used for the coverage pass:

- `git rev-parse v6.0.0-rc.2^{}`
- `git rev-parse HEAD`
- `git rev-list --count v6.0.0-rc.2..HEAD`
- `git log --reverse --format='%h%x09%an%x09%ad%x09%s' --date=short v6.0.0-rc.2..HEAD`
- `git diff --stat v6.0.0-rc.2..HEAD`
- `git log --format='%h%x09%s' --name-only v6.0.0-rc.2..HEAD`

## Commit Coverage Summary

The 603 commits were covered by these release-note buckets:

- release packaging, release validation, signed assets, installer resolution,
  stable-channel prerelease filtering, update signer continuity, rollback
  posture, Helm, Docker, and workflow hardening
- security, auth, token handling, setup/bootstrap state, transport validation,
  trusted proxy, websocket origin, workflow permission, webhook, and outbound
  HTTP hardening
- commercial, licensing, Relay, Pro, self-hosted plan, Community-tier SSO,
  hosted signup, Pulse Account, tenant/workspace, MSP, and Cloud readiness
  cleanup
- infrastructure, connections, Unified Agent, Proxmox, PBS, PMG, TrueNAS,
  VMware, Docker, Podman, platform admission, and fleet governance
- monitoring, alerts, metrics history, Workloads, Storage, Recovery, backup,
  snapshot, Ceph, ZFS, RAID, filters, tables, charts, and summary layout
- AI, Patrol, action governance, approval/audit history, command policy, and
  Ollama local-runtime behavior
- mobile companion-role, hosted mobile proof, approval routing, and device
  readiness evidence
- documentation, release-control evidence, contribution policy, support-pack,
  upgrade-guide, and public-copy cleanup
- frontend layout, Dashboard retirement, Infrastructure-first routing,
  Settings surfaces, first-session/setup flow, shared table primitives, filter
  decks, saved views, sticky summaries, and responsive controls

## Packet Updates

The public RC3 packet was expanded so it no longer describes `rc.3` as only a
corrective maintenance RC:

- `docs/releases/V6_CHANGELOG_RC3_DRAFT.md`
  - records the exact commit range and count
  - adds release packaging, security/auth, hosted/mobile, governance, latest
    storage, skip-auth, SSO entitlement, provider-settings, stable installer
    selection, and artifact-validation coverage
- `docs/releases/RELEASE_NOTES_v6_RC3_DRAFT.md`
  - expands the release intent from a narrow corrective RC to a broad
    hardening RC with corrective maintenance at its core
  - adds release packaging, security/auth, hosted/mobile, governance, storage
    summary, skip-auth, SSO entitlement, provider-settings, stable installer
    selection, and artifact-validation re-test notes
- `docs/releases/V6_RC3_OPERATOR_SUPPORT_PACK_DRAFT.md`
  - aligns maintainer-facing support language with the broader audited delta
  - adds the newest storage, skip-auth, SSO, stable installer selection, and
    release-asset validation notes
- `docs/release-control/v6/internal/subsystems/deployment-installability.md`
  - records that post-draft packet changes must carry exact commit coverage,
    artifact/release-pipeline evidence, and a refreshed draft before
    publication
- `scripts/release_control/render_release_body_test.py`
  - verifies that the RC3 packet retains commit coverage and release artifact
    hardening language

## Outcome

The audit did not identify a new unhandled code blocker from the commit range.
It did identify a documentation gap: the existing RC3 packet under-described
the full post-`rc.2` release delta. That gap is now recorded and addressed in
the draft packet before publication.

No GitHub release publication, Docker publication, public issue comment,
retitle, closure, or other external state change was made as part of this
audit.

## Verification

- `git diff --check -- docs/releases/V6_CHANGELOG_RC3_DRAFT.md docs/releases/RELEASE_NOTES_v6_RC3_DRAFT.md docs/releases/V6_RC3_OPERATOR_SUPPORT_PACK_DRAFT.md docs/release-control/v6/internal/records/documentation-currentness-and-legacy-cleanup-rc3-commit-audit-2026-05-03.md docs/release-control/v6/internal/subsystems/deployment-installability.md scripts/release_control/render_release_body_test.py`
  - passed
- `python3 scripts/release_control/documentation_currentness_test.py`
  - passed, 6 tests
- `python3 scripts/release_control/render_release_body_test.py`
  - passed, 4 tests
- `PYTHONPATH=scripts/release_control python3 -m unittest scripts.release_control.release_promotion_policy_test.ReleasePromotionPolicyTest.test_release_notes_index_points_at_current_rc_packet scripts.release_control.release_promotion_policy_test.ReleasePromotionPolicyTest.test_operator_support_packs_keep_free_first_paid_continuity_wording scripts.release_control.release_promotion_policy_test.ReleasePromotionPolicyTest.test_version_file_matches_current_rc_packet -q`
  - passed, 3 tests
- `python3 scripts/release_control/status_audit.py --pretty`
  - `repo_ready=True`
  - `rc_ready=True`
  - `release_ready=True`
