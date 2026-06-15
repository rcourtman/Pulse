# Pulse v6 Release Promotion Policy

This document defines how Pulse v6 and later releases move from development to
customer-facing availability. It is the release-trust contract for Pulse Pro,
Cloud, and self-hosted production users.

## Goals

1. Stable customers must not become the first test cohort for new changes.
2. Development speed must stay decoupled from customer exposure.
3. Every broad rollout must have explicit validation and rollback rules.

## Channel Contract

1. `stable`
   - Default for new installs.
   - The only recommended channel for paid and production environments.
   - Publishes only non-prerelease tags.
   - The only channel eligible for unattended broad rollout.
2. `rc`
   - Opt-in preview channel for internal use, staging-like environments, and
     explicitly willing preview users.
   - Publishes prerelease tags such as `6.0.0-rc.1`.
   - Must never be the default channel.
   - In v6, `rc` affects manual and in-app update selection; unattended
     systemd auto-updates remain `stable`-only.
3. Source builds
   - Are not a customer-facing release channel.
   - Remain reserved for development, debugging, and branch validation.

## Development Model

1. Use short-lived feature branches and feature flags for incomplete or risky
   work.
2. Do not move directly from "issue fixed" to "all customers updated".
3. Channel promotion is the primary customer-safety boundary.
4. Branch topology may change over time; the `stable` versus `rc` customer
   contract must not.
5. The active release profile in `docs/release-control/control_plane.json`
   owns the governed prerelease and stable release branches for the current
   line; release automation must resolve branch requirements from that file
   instead of assuming `main`.

## Prerelease Rules

1. Every candidate intended for broad customer use must ship to `rc` before it
   is eligible for `stable`.
2. Each published prerelease must have:
   - Targeted automated checks for touched release surfaces.
   - A smoke install on a fresh or staging-like environment.
   - Release notes plus the rollback target and exact reinstall command recorded before publish.
   - At least one live run of the release pipeline for the prerelease tag itself, not
     only structural workflow validation.
   - A governed prerelease publication record; an accidental git tag by itself
     does not count as a shipped prerelease.
3. Failed prereleases are fixed forward and replaced with a new prerelease. They are never
   promoted as-is to `stable`.

## Paid Pro Artifact Lineage

1. Customer-facing private Pulse Pro archives and private Pulse Pro Docker images
   must track the same immutable release checkpoint as the public Pulse release
   they support.
2. During the v6 RC phase, private Pro artifacts must be built from the exact
   public RC tag, use the same RC version, and publish under RC-shaped artifact
   names, R2 prefixes, and Docker tags such as `6.0.0-rc.5`.
3. Do not build or advertise `license.pulserelay.pro/pulse-pro:6.0.0`, a
   `pulse-pro-v6.0.0-...` private archive, or a GA-shaped private R2 prefix
   until the intentional v6 GA publish.
4. A private Pro build from a moving branch is valid only as an internal proof
   artifact. It is not valid customer guidance and must not update the live
   paid-download manifest or private Docker customer tag.
5. Customer-facing private Pro archive and Docker publication is part of the
   public v6 release pipeline. After `validate-release-assets.yml` succeeds for
   a non-draft v6 release, `create-release.yml` must dispatch
   `rcourtman/pulse-enterprise` `Build Pro Release` against the exact public
   tag with `upload_to_r2=true`, `publish_docker_image=true`, and an R2 prefix
   derived by the release run, then wait for that workflow to succeed.
6. The public v6 release pipeline must then dispatch `rcourtman/pulse-pro`
   `Promote Paid Runtime Release` with the same version and R2 prefix, and
   wait for the signed packet to promote the live paid-download broker. A failed
   private build or failed live promotion fails the public release workflow;
   private Pro RC/GA advancement must not depend on an operator noticing a
   checklist item after the public RC has shipped.
7. Customer-facing private Pro archive or Docker promotion must use the generated
   paid-runtime proof packet from the Pro release workflow. The canonical command
   is `scripts/promote_paid_runtime_release_packet.sh --release-dir <proof-packet-dir> --admin-token-file <explicit-token-file> --execute-live`
   from `repos/pulse-pro`; GA promotions also require `--allow-ga-prefix`.
8. The promotion command is the release gate for the live paid-download broker:
   it validates the proof packet signatures, installs the exact manifest on
   `pulse-license`, runs the live customer-path proof, and restores the previous
   remote manifest if the gate fails. Do not send customer instructions from a
   customer-facing private Pro RC/GA release until that command passes.

## v5 Maintenance Policy

1. When Pulse v6 reaches `stable`, Pulse v5 immediately enters
   maintenance-only support.
2. The maintenance-only window lasts 90 calendar days from the v6 GA or stable
   release date.
3. During that window, v5 fixes are limited to:
   - critical security issues
   - critical correctness or data-loss issues
   - migration blockers that prevent customers from reaching a safe v6 path
4. v5 will not receive:
   - new features
   - normal bug-fix backports
   - pricing-model exceptions
   - entitlement-model parity work introduced for v6
5. After the 90-day window ends, v5 may continue running for users who choose
   to stay on it, but it is unsupported.
6. The v6 GA announcement must publish the exact v5 end-of-support date
   calculated from the GA publication date.
7. Before GA promotion is actually cleared, release notes may keep placeholder
   dates for the GA notice; those placeholders do not satisfy the promotion
   gate by themselves.
8. `V5_MAINTENANCE_SUPPORT_POLICY.md` is the canonical source for this policy
   and the required GA release notice.

## Stable Promotion Rules

1. A stable tag must be promoted from a commit that has already been exercised
   as a published prerelease.
2. A prerelease git tag counts as stable-promotion lineage only if that prerelease was
   actually published through the governed prerelease path; accidental or abandoned git
   tags do not satisfy the stable-promotion requirement.
3. For v6 GA, do not promote to `stable` until the active control-plane target
   is the GA-promotion target and satisfies its `release_ready` completion
   rule.
4. Every stable promotion requires:
   - Applicable items in `PRE_RELEASE_CHECKLIST.md` complete.
   - Applicable entries in `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` cleared.
   - No known unresolved RC-era user-visible issues intended for the v6 GA
     scope remain open. Each one must be fixed in the candidate, proven
     invalid with evidence, or conservatively superseded with the original
     failure resolved or explicitly narrowed.
   - The previous stable rollback target and exact reinstall command recorded.
   - A live release-pipeline exercise already completed for the promoted prerelease tag,
     not only YAML lint or static workflow validation.
   - The locked 90-day v5 maintenance-only policy in
     `V5_MAINTENANCE_SUPPORT_POLICY.md` and the exact end-of-support notice
     ready to publish with the promotion.
5. Normal stable promotions require a minimum 72-hour prerelease soak after the
   candidate is available to internal or staging-like users.
6. Hotfix exception:
   - A shorter soak is allowed only for narrowly scoped fixes to active
     customer harm.
   - The exception plus the rollback target and exact reinstall command must be
     recorded in the release notes or release ticket before promotion.

## Rollout Rules

1. Default installs stay on `stable`.
2. Broad customer announcements and unattended updates target `stable` only.
3. `rc` enrollment must be explicit and reversible.
4. Paid production tenants should remain on `stable` unless they are knowingly
   participating in preview validation.

## Rollback Rules

1. Never delete or rewrite shipped tags to hide a bad release; supersede them
   with a newer release and explicit guidance.
2. If a prerelease is bad, hold it in `rc`, fix forward, and cut the next prerelease. Do not
   promote it.
3. If a stable release is bad:
   - Pause further promotion or auto-update exposure.
   - Direct affected users to the prior stable pin.
   - Cut and validate a hotfix or rollback release.
4. The previous stable version must remain installable by exact version pin
   until the replacement stable release is trusted.

## Required Release Artifacts

1. Release notes.
2. Rollback target version and exact pin command.
3. Checklist evidence and gate status.
4. Staging or internal validation note.
5. v5 maintenance-only support policy and end-of-support note for the GA cutover.
6. Exact v6 GA and v5 end-of-support dates locked before GA publish and then
   published in the GA release notes.
7. Prerelease-to-GA rehearsal record plus the machine-generated
   `rc-to-ga-rehearsal-summary` artifact, including the GitHub Actions run URL
   for the non-publish dry run and the canonical promotion metadata envelope:
   candidate stable tag, promotion channel, promoted prerelease tag, rollback target,
   exact rollback command, planned GA date, and planned v5 end-of-support
   date. Materialize that dated record with
   `python3 scripts/release_control/record_rc_to_ga_rehearsal.py --run-id <run-id>`
   unless an explicitly different output path is needed.
8. The pushed governed release-branch copy of `.github/workflows/release-dry-run.yml`
   must already accept that stable rehearsal metadata envelope through
   `workflow_dispatch`, and the local release branch must match `origin` before
   dispatch, because GitHub executes the selected remote ref and does not see
   local-only governance state.
9. For v6 GA, the exact self-hosted public forward and rollback packet must be
   locked in the launch ticket before promotion: preview deploy/audit commands,
   production deploy/audit commands, and the explicit rollback deploy/audit
   commands that return `pulserelay.pro` to the approved v5 posture. Preview
   proof, readiness records, and internal target completion do not authorize the
   production public checkout flip by themselves; until the owner-approved GA
   packet is actively executed, production public checkout remains
   `PULSE_PUBLIC_RELEASE_TRACK=v5` with `PULSE_V6_RELEASE_APPROVED=0`.
10. For v6 GA, attach the dated RC issue-closure record for the candidate so
    the final issue set and its dispositions are explicit in the promotion
    packet rather than implied.

## Authority

If conflicts appear:

1. `SOURCE_OF_TRUTH.md` owns the locked decision that this policy is mandatory.
2. `status.json` owns whether the decision is open or resolved and whether the
   active target is release-ready.
3. `PRE_RELEASE_CHECKLIST.md` and
   `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` own execution proof for a
   specific promotion.
