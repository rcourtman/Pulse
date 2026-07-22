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

1. A first stable release, a stable minor release, and every patch that crosses
   one of the RC-required risk boundaries below must be promoted from a commit
   that has already been exercised as a published prerelease.
2. A prerelease git tag counts as stable-promotion lineage only if that prerelease was
   actually published through the governed prerelease path; accidental or abandoned git
   tags do not satisfy the stable-promotion requirement.
3. For v6 GA, do not promote to `stable` until the active control-plane target
   is the GA-promotion target and satisfies its `release_ready` completion
   rule.
4. Every stable promotion requires:
   - Applicable items in `PRE_RELEASE_CHECKLIST.md` complete.
   - Applicable entries in `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` cleared.
   - The previous stable rollback target and exact reinstall command recorded.
5. A first stable release or RC-required stable promotion additionally requires:
   - No known unresolved RC-era user-visible issues intended for the v6 GA
     scope remain open. Each one must be fixed in the candidate, proven
     invalid with evidence, or conservatively superseded with the original
     failure resolved or explicitly narrowed.
   - A live release-pipeline exercise already completed for the promoted prerelease tag,
     not only YAML lint or static workflow validation.
6. The first v6 GA promotion additionally requires:
   - The locked 90-day v5 maintenance-only policy in
     `V5_MAINTENANCE_SUPPORT_POLICY.md` and the exact end-of-support notice
     ready to publish with the promotion.
7. RC-derived stable promotions require a minimum 72-hour prerelease soak after
   the candidate is available to internal or staging-like users.
8. Hotfix exception:
   - Bypassing an RC requirement or shortening an RC soak is allowed only for
     narrowly scoped fixes to active customer harm.
   - The exception plus the rollback target and exact reinstall command must be
     recorded in the release notes or release ticket before promotion.
9. v6.0.0 owner-risk exception:
   - On 2026-07-02, after seven v6 release candidates, the release owner
     explicitly approved promoting the current `pulse/v6-release` branch with
     accumulated post-RC7 changes without RC8, another soak, or additional
     current-branch validation before GA.
   - This is a bounded v6.0.0 release-owner risk acceptance, not validation
     evidence for the post-RC7 changes and not a standing policy for later
     stable releases.
   - The release packet must describe the GA candidate honestly as the current
     branch after the RC line, keep rollback and v5 maintenance dates explicit,
     and retain the prior governed release-pipeline rehearsal evidence as
     automation lineage rather than claiming the post-RC7 changes were RC-tested.
10. v6.1.0 release-cutoff exception:
   - On 2026-07-22, the release owner ended the moving RC feedback loop and
     declared current `main` the v6.1.0 feature cutoff.
     The dated decision record is
     `docs/release-control/v6/internal/records/v6.1.0-stable-cutoff-owner-approval-2026-07-22.md`.
   - v6.1.0 may publish stable without another RC publication or the normal
     72-hour soak. This is an explicit owner-risk acceptance for this version,
     not reusable evidence and not a standing exception for later releases.
   - The exact stable SHA must still pass the no-public-release `Release Dry
     Run` and the normal single-build publication workflow. Only a security,
     data-loss, upgrade-breaking, startup-blocking, or release-pipeline defect
     may reopen the cutoff; ordinary findings move to v6.1.1.
   - After exact-SHA rehearsal `29927692302` exposed unavailable external
     SignPath configuration, the release owner approved a `v6.1.0`-only
     Windows Authenticode exception. The Windows artifacts must remain bound
     by the exact-SHA candidate manifest, checksums, detached `.sig`/`.sshsig`
     signatures, and published digests, and the public notes must disclose the
     Unknown Publisher state. Later stable releases restore the Authenticode
     requirement; this flag cannot be reused for another version.
   - Unproved self-service commercial transitions remain unavailable and
     unadvertised under the exposure-safety gate. This exception does not
     authorize enabling that feature or running a production billing proof.

## Single-Build Release Path

1. Every normal RC, stable, and patch release is initiated once through
   `create-release.yml`. The workflow builds one exact-SHA candidate with the
   native signing lanes required by that version's governed policy while
   frontend, backend, Docker, Helm, and integration checks run in parallel. No
   tag, draft, or public release mutation occurs until those checks and the
   candidate build pass.
2. The release candidate is uploaded as a one-day Actions artifact with a
   machine-readable manifest that pins source SHA, version, filename, size, and
   SHA-256 for every release asset. Publication downloads and verifies that
   exact candidate; it must not rebuild release binaries or installers.
3. Standard post-publication asset verification compares the candidate
   manifest with GitHub's server-side release-asset SHA-256 digests. It must
   not re-download the multi-gigabyte release packet merely to recompute hashes
   already proven before upload. Manual and release-edit repair validation may
   retain the full-download fallback when no same-run candidate manifest exists.
4. Docker publication, release-asset verification, and the private Pro build
   begin independently as soon as the release exists. Helm, floating tags,
   install smoke, stable demo deployment, and private paid-runtime promotion
   retain their required dependencies, and `Definitive Release Verdict` still
   fails unless every applicable terminal result passes.
5. `Release Dry Run` remains the no-public-release rehearsal surface. It calls
   the same candidate builder and no-mutation demo verification, but a separate
   dry run is not required before a normal release because the single publish
   workflow performs the exact-SHA preflight before crossing the publication
   boundary.

## Routine Stable Patch Path

1. A normal stable patch may omit a same-version RC only when all of these are
   true:
   - the rollback target is the latest preceding stable tag and the candidate
     descends from it;
   - no same-version RC tag already exists;
   - the diff does not touch authentication/authorization/tenant isolation,
     licensing/entitlement/billing authority, persisted data/schema/migration,
     relay/mobile trust protocol, or installer/updater/rollback execution;
   - the canonical stable release-notes packet exists;
   - the mobile-impact gate either proves no mobile-facing change or records
     current candidate evidence; and
   - the integrated exact-SHA candidate build and release checks pass before
     the workflow creates or publishes the release.
2. `scripts/trigger-stable-patch.sh` is the standard operator entrypoint. Run
   it once without `--dry-run`; it derives rollback and release notes, refuses
   local-only or dirty state, and supplies workflow metadata without
   interactive prompts. `--dry-run` is optional and exists only for an explicit
   no-public-release rehearsal.
3. Creating a same-version RC or touching an RC-required path moves the patch
   onto the RC promotion path. The resolver enforces that boundary. Do not use
   the routine helper to relabel a risky patch as routine.
4. `--emergency-hotfix-reason` is the narrow escape hatch for active customer
   harm. It does not remove the exact-SHA dry-run requirement, and the reason is
   recorded in the release metadata.
5. The release workflow must await Docker publication, stable demo deployment,
   public health/browser verification, install smoke, Helm publication,
   floating-tag promotion, and private Pro promotion where applicable. The
   terminal `Definitive Release Verdict` job is the one release result; an
   asynchronously dispatched demo workflow is not release completion.

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
