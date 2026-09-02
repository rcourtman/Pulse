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
2. `preview` (stored as `rc` in the existing update-channel API and persisted
   settings for backward compatibility)
   - Opt-in channel for internal use, staging-like environments, and explicitly
     willing preview users.
   - Publishes governed prerelease tags such as `6.5.0-alpha.1`,
     `6.5.0-beta.1`, and `6.5.0-rc.1`.
   - Must never be the default channel.
   - In v6, the legacy `rc` wire value affects manual and in-app preview update
     selection; unattended
     systemd auto-updates remain `stable`-only.
3. Source builds
   - Are not a customer-facing release channel.
   - Remain reserved for development, debugging, and branch validation.

## Reporter Test Image Contract

A reporter test image is an issue-scoped diagnostic source build for one or a
few explicitly participating Docker users. It exists to validate a reviewed
fix quickly without publishing a prerelease or exposing the broader preview
cohort to an unqualified candidate.

It is not a fourth release channel. It does not count as alpha, beta, RC,
stable, a published prerelease, release qualification, or stable-promotion
lineage.

### Eligibility

Use a reporter test image only when all of these conditions hold:

1. the reported behavior has a concrete diagnosis or reproduction, an exact
   reviewed fix commit, and targeted automated regression proof
2. the reporter runs Docker or an equivalent OCI deployment and the affected
   behavior can be validated without an installer, updater, Helm, mobile,
   private paid-runtime, or other release-only surface
3. the validation cohort is one or a few named issue participants rather than
   a broad preview audience
4. the reporter can pin an exact image and return to a known release image
5. the test does not require destructive data migration, production secret
   handling, or a widened authentication, authorization, tenant, billing,
   licensing, relay, or other trust boundary

If any condition is false or uncertain, use the governed prerelease path. Use
beta while user validation or planned product changes remain. Use RC only when
the build is believed capable of becoming stable without product changes.

### Build And Publication

1. Build from one exact reviewed commit in the canonical Pulse repository.
   Never build arbitrary contributor code on a machine or builder that can
   access registry credentials, signing material, production data, workspace
   secrets, or unrelated Docker workloads.
2. Use an isolated builder and clean source state. Do not pass host secrets,
   production mounts, privileged runtime access, or unrelated Docker sockets
   into the build. The workstation's current default builder for reporter
   test images, with fallback and traps, is documented in the workspace
   `LOCAL_CAPABILITIES.md` ("Remote amd64 Builder on longonk442").
   The server-only diagnostic image (no embedded installer or agent-download
   artifacts) is the `hosted_runtime` Dockerfile target. Setting
   `BUILD_AGENT=0` does NOT achieve this: the default `runtime` target always
   builds the full agent artifact set regardless of that arg.
3. Publish a new immutable tag shaped as `test-<issue>-<short-sha>`, for
   example `rcourtman/pulse:test-1437-44a3f194`. Never overwrite or reuse the
   tag.
4. Stamp OCI source and revision labels. The running Pulse version must identify
   the released base plus test issue and commit metadata, for example
   `v6.2.1+test.1437.44a3f194`. It must not impersonate a stable or RC version.
5. Build only the reporter's required platform unless more than one platform
   is part of the named validation cohort. Record every published platform.
6. Run the targeted regression proof plus a clean-container start, health
   check, and version check before sharing the image. Pull the published image
   by digest for the final smoke so registry publication itself is exercised.
7. Record the source commit, image tag, registry digest, platform, proof
   commands, smoke result, exact pull or Compose override, and exact rollback
   image in the task handoff. Preserve the tag, digest, test steps, and rollback
   image in the approved issue reply so later triage can recover the artifact
   identity without relying on chat history.

### Reporter Guidance And Claims

1. Do not offer the image until it is published, pullable by digest, and has
   passed the required smoke.
2. The approved reporter reply must include the exact tag and digest, describe
   it as a temporary diagnostic build, name the focused checks, recommend a
   backup and non-production use where practical, provide the prior stable
   rollback image, and request fresh diagnostics from the same reproduction
   window if validation fails.
3. Keep the issue open until the named user-visible behavior is confirmed or
   conservatively superseded by another active issue.
4. Reporter confirmation is narrow live evidence for the named issue,
   environment, commit, image digest, and exercised behavior. It does not
   qualify installers, updater behavior, Helm, other architectures, private
   paid runtime, cross-repo compatibility, migrations, security boundaries,
   or the wider release.
5. A successful test does not force an immediate patch release. Schedule the
   fix by severity and active customer harm. Any later prerelease, stable, or patch
   must rebuild and qualify through the canonical release workflow rather
   than promote the reporter image.

### Forbidden Release Mutations And Lifecycle

1. A reporter test image must never create a GitHub release or git tag, update
   release notes or update feeds, publish Helm, move Docker aliases, update the
   demo, stage or promote private paid artifacts, or satisfy a prerelease or stable
   release gate.
2. Never attach `latest`, a stable version, an alpha, beta, or RC version, or another mutable
   alias to the image.
3. Failed validation is fixed forward from a new reviewed commit and published
   under a new commit-derived tag. Shared tags and digests are immutable.
4. After a stable release containing the fix ships and the issue is resolved,
   the test image becomes eligible for removal only with explicit maintainer
   approval. The old tag remains retired and must never be reused.

## Development Model

1. Use short-lived feature branches and feature flags for incomplete or risky
   work.
2. Do not move directly from "issue fixed" to "all customers updated".
3. Channel promotion is the primary customer-safety boundary.
4. Branch topology may change over time; the `stable` versus `preview` customer
   contract must not.
5. The active release profile in `docs/release-control/control_plane.json`
   owns the governed prerelease and stable release branches for the current
   line; release automation must resolve branch requirements from that file
   instead of assuming `main`.

## Runtime Verification Claim Levels

Release status language is an evidence contract. Use these three levels and do
not promote a claim beyond the evidence that exists:

1. `implemented`
   - The source change and targeted regression proof exist.
   - This does not mean a release artifact contains the change or that a live
     installation is running it.
2. `release-validated`
   - The immutable release artifact containing the change passed its governed
     build, artifact, install, and release-pipeline checks.
   - Publication, a successful installer, and a displayed version number do
     not by themselves prove the affected behavior on real hardware.
3. `live-verified`
   - The exact release is running on the named target and a fresh,
     machine-readable assertion against the affected live API passed.
   - A saved receipt must be independently verified for the expected target,
     exact version, assertion, and allowed age before anyone describes the
     issue as fixed on hardware, fixed in production, or live verified.

For the Proxmox protection-posture persistence regression, the canonical
collector and verifier are `scripts/release_control/live_runtime_proof.py`.
The collector must query `/api/version` and every page of
`/api/recovery/postures`, require a non-empty successful-posture cohort, and
fail while any workload with `lastSuccessfulPointAt` remains `unknown`. It must
record the target origin, TLS-verification state, exact expected and observed
versions, packaged-versus-development runtime state, UTC collection time,
posture counts, failing resource IDs, and raw response SHA-256 values in a
sealed JSON receipt. Authentication values must be read from named environment
variables rather than command-line values or receipt content.

Example post-install proof:

```bash
export PULSE_LIVE_PROOF_AUTHORIZATION='Bearer <monitoring-read-token>'
python3 scripts/release_control/live_runtime_proof.py collect \
  --base-url https://pulse.example.net \
  --expected-version 6.2.0-rc.8 \
  --authorization-env PULSE_LIVE_PROOF_AUTHORIZATION \
  --minimum-postures 1 \
  --minimum-successful-postures 1 \
  --output /secure/release-evidence/pulse-6.2.0-rc.8-live.json
python3 scripts/release_control/live_runtime_proof.py verify \
  --receipt /secure/release-evidence/pulse-6.2.0-rc.8-live.json \
  --expected-origin https://pulse.example.net \
  --expected-version 6.2.0-rc.8 \
  --max-age-seconds 3600
```

The receipt contains operational target identity and may contain resource IDs,
so retain it in the restricted release-evidence location rather than a public
release body. Missing, stale, failed, edited, wrong-target, wrong-version, or
TLS-unverified receipts leave the claim at `implemented` or
`release-validated`; an operator statement cannot substitute for the receipt.

## Prerelease Rules

1. Published prereleases use one of three maturity stages:
   - `alpha.N` is incomplete or experimental and intended for internal or
     tightly controlled evaluation.
   - `beta.N` is the normal user-testing stage. It may contain known gaps and
     planned product changes and is not represented as a possible stable build.
   - `rc.N` is reserved for a build the release owner believes can become
     stable without product changes. RC publication runs the stable-depth
     integration gate in addition to the common release checks.
2. Stable promotion lineage must come from a published `rc.N`. Alpha and beta
   evidence inform the release, but neither can be promoted directly to stable.
3. Each published prerelease must have:
   - Targeted automated checks for touched release surfaces.
   - A smoke install on a fresh or staging-like environment.
   - Release notes plus the rollback target and exact reinstall command recorded before publish.
   - At least one live run of the release pipeline for the prerelease tag itself, not
     only structural workflow validation.
   - A governed prerelease publication record; an accidental git tag by itself
     does not count as a shipped prerelease.
4. A published alpha, beta, or RC is a cohort checkpoint, not a delivery
   vehicle for each individual fix. After the first checkpoint at a maturity
   stage on a version line, at least 24 hours of public observation must elapse
   before another checkpoint at that same stage may be published. Moving from
   alpha to beta or beta to RC is a maturity decision and starts a new stage.
   Accumulate compatible fixes and release-note outcomes during the window.
   Candidate preparation, draft creation, and Release Dry Run remain available
   throughout it because they do not replace the public cohort.
5. Use an immutable issue-and-commit reporter test image for narrow confirmation
   that cannot justify a new public cohort. Broad release qualification remains
   on the governed prerelease path, but successful narrow validation does not
   bypass the 24-hour publication boundary.
6. Failed prereleases are fixed forward and replaced with a new prerelease. They are never
   promoted as-is to `stable`.

## Release Train

Adopted 2026-09-01 under the delivery contract
(`pulse-dev-infra/docs/delivery-contract.md`). The train exists so that a
stable release is an exact soaked candidate rather than the tip of a branch
that keeps moving, and so that the always-running maintainer can release
without the other lanes changing the candidate underneath it.

1. A minor train runs every two weeks. The first candidate is cut on a
   fixed day and time, Tuesday 09:00 Europe/London, and general availability
   is the Tuesday one week later; the next train's candidate is cut the
   Tuesday after that. The first train is v6.5.0: `v6.5.0-rc.1` on
   2026-09-08, general availability on 2026-09-15, then `v6.6.0-rc.1` on
   2026-09-22. The cadence follows measured velocity, not preference: at the
   70 to 150 commits a day `main` received in the week before adoption, a
   four-week train would put two to four thousand commits into every user
   upgrade, and a two-week train halves that while keeping a full seven day
   soak. The release steward reviews the cadence against velocity after
   every third train and records the decision here.
2. Each train has its own branch, `release/v6.N`, created from `main` at cut
   time and declared in `docs/release-control/control_plane.json` so the
   release workflow refuses a dispatch from any other branch. `main` is never
   frozen. A fix for something found in the candidate is backported to the
   release branch through a pull request; each backport produces the next
   `rc.N` and restarts the soak. After general availability the branch is
   the patch line for that train.
3. General availability promotes the candidate's content. The resolver
   refuses a stable promotion whose tree differs from the promoted candidate
   in anything but release metadata (version, chart, compose, release notes,
   upgrade guide, release records) unless `hotfix_exception` names active
   customer harm. The v6.4.0 promotion, which shipped 64 changed files that
   `v6.4.0-rc.12` had not soaked, is the case this rule prevents.
4. A minor release (`X.Y.0`) requires a seven day soak of the promoted
   candidate; patch releases keep the 72 hour minimum. Patch releases are for
   a named regression or security issue only.
5. "Soaked clean" means all of: the soak has elapsed since the candidate's
   release was published; no open issue labelled `affects-<candidate
   version>` is at high or critical severity; the maintainer's dogfood
   instance and the demo server ran the candidate for the whole soak without
   an incident; and preview-channel telemetry, where it exists, shows no
   elevated failure rate. The release steward names this evidence in the
   packet.
6. Version-bound owner exceptions waived the soak for v6.0.0, v6.1.0,
   v6.2.0, v6.3.0, and v6.4.0. They remain recorded and bounded; the train
   does not continue the practice. An exception requires active customer
   harm and is recorded in the release notes.
7. The `v6.4.3` patch line predates the first train and is the first line
   released under the train's branch rule. `release/v6.4` was created from
   `main` at the exact-SHA-qualified commit `56e51e622e` on 2026-09-02 and is
   declared in `control_plane.json` with the version prefix `6.4.3`, so the
   release workflow refuses a `v6.4.3` dispatch from any other branch and a
   moving `main` can no longer invalidate the compiler's exact-SHA binding
   between dispatch and compilation, which is what failed run 33579042375.
   Earlier `6.4.x` versions keep their historical `main` mapping.

## Paid Pro Artifact Lineage

1. Customer-facing private Pulse Pro archives and private Pulse Pro Docker images
   must track the same immutable release checkpoint as the public Pulse release
   they support.
2. During a v6 prerelease phase, private Pro artifacts must be built from the
   exact public alpha, beta, or RC tag, use that same version, and publish under
   matching artifact names, R2 prefixes, and Docker tags such as
   `6.5.0-beta.1`.
3. Do not build or advertise `license.pulserelay.pro/pulse-pro:6.0.0`, a
   `pulse-pro-v6.0.0-...` private archive, or a GA-shaped private R2 prefix
   until the intentional v6 GA publish.
4. A private Pro build from a moving branch is valid only as an internal proof
   artifact. It is not valid customer guidance and must not update the live
   paid-download manifest or private Docker customer tag.
5. Customer-facing private Pro archive and exact-version Docker staging is part
   of the public v6 release pipeline. After the governed tag and unpublished
   draft exist, `create-release.yml` must dispatch
   `rcourtman/pulse-enterprise` `Build Pro Release` against the exact public
   tag with `upload_to_r2=true`, `publish_docker_image=true`, and an R2 prefix
   derived by the release run, then wait for that workflow to succeed. Every
   cross-repository dispatch must request GitHub's returned workflow-run details
   and poll that exact run ID. Selecting the newest run by workflow, branch, or
   dispatch timestamp is forbidden because different release versions and manual
   dispatches may run concurrently.
6. The public v6 release pipeline must durably dispatch a dedicated convergence
   run before it publishes GitHub, then publish and publicly verify the GitHub
   release before that run may dispatch `rcourtman/pulse-pro` `Promote Paid
   Runtime Release` with the same version and R2 prefix. The live paid-download
   broker is a mutable customer pointer and must never advance before the
   verified `release-activation.json` commit marker exists. A failed private
   build prevents that commit. A failed live promotion is post-commit
   convergence debt: it fails and retries the separate convergence run without
   claiming that the already-public release rolled back.
   The activation marker must bind the exact staged R2 prefix. The dispatch must
   pass that prefix with the exact Pulse lease SHA and convergence run ID.
   `pulse-pro` must reject an unleased dispatch, verify the public Pulse lock ref,
   lease commit owner, active convergence run, and activation marker without
   relying on its repository-scoped `GITHUB_TOKEN`, then repeat that check
   immediately before live mutation. `pulse-pro` may serialize only the same
   validated lease correlation, never all releases or arbitrary R2 prefixes.
   Exact duplicates are therefore sequential and target-identical, while
   invalid lease values cannot replace a legitimate pending child.
7. Customer-facing private Pro archive or Docker promotion must use the generated
   paid-runtime proof packet from the Pro release workflow. The canonical command
   is `scripts/promote_paid_runtime_release_packet.sh --release-dir <proof-packet-dir> --admin-token-file <explicit-token-file> --execute-live`
   from `repos/pulse-pro`; GA promotions also require `--allow-ga-prefix`.
8. The promotion command is the release gate for the live paid-download broker:
   it validates the proof packet signatures, installs the exact manifest on
   `pulse-license`, runs the live customer-path proof, and restores the previous
   remote manifest if the gate fails. Do not send customer instructions from a
   customer-facing private Pro prerelease or stable release until that command
   passes.

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
   that has already been exercised as a published RC.
2. An RC git tag counts as stable-promotion lineage only if that RC was actually
   published through the governed prerelease path. Alpha tags, beta tags, and
   accidental or abandoned RC tags do not satisfy the stable-promotion
   requirement.
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
     Unknown Publisher state.
   - On 2026-07-23, the release owner separately approved a `v6.1.1`-only
     Windows Authenticode exception for the emergency patch addressing active
     customer update harm. This is a new, version-bound decision rather than a
     reuse of the `v6.1.0` exception. The same exact-SHA candidate, checksum,
     detached-signature, manifest, published-digest, owner-reason, and public
     Unknown Publisher disclosure controls remain mandatory. Stable `v6.1.2`
     and later restore the Authenticode requirement unless another explicit
     version-bound owner decision is recorded in policy.
   - Unproved self-service commercial transitions remain unavailable and
     unadvertised under the exposure-safety gate. This exception does not
     authorize enabling that feature or running a production billing proof.
11. v6.2.0 release-cutoff exception:
   - On 2026-08-09, the release owner declared current `main` at
     `b9811cdf538224e7f2870718744300ef8f80afa0` the v6.2.0 content cutoff and
     explicitly directed promotion from `v6.2.0-rc.11` without completing the
     normal 72-hour soak. The dated decision record is
     `docs/release-control/v6/internal/records/v6.2.0-stable-cutoff-owner-approval-2026-08-09.md`.
   - This is a bounded v6.2.0 owner-risk acceptance, not soak evidence and not
     a standing exception for later releases. The workflow input
     `hotfix_exception=true` transports the approved waiver through the shared
     resolver; it does not reclassify v6.2.0 as a patch hotfix.
   - The final release-preparation commit may change only governed version,
     release-note, qualification, test-guardrail, and release-control metadata.
     The exact pushed SHA must pass the no-publication `Release Dry Run` before
     the same SHA is dispatched through the single-build publication workflow.
   - Exact-SHA dry run `31306697834` failed closed before candidate assembly or
     public mutation because the SignPath `release-signing` policy was invalid:
     its `Release certificate 2026` CSR remained pending. After that failure,
     the release owner separately approved a `v6.2.0`-only unsigned-Windows
     exception. This signing exception is independent of the soak waiver and is
     not a standing decision for later releases.
   - The unsigned Windows artifacts remain bound by the exact-SHA candidate
     manifest, checksums, detached `.sig`/`.sshsig` signatures, and published
     digests. Public notes must disclose that they are not Authenticode-signed
     and may display an Unknown Publisher warning. Stable `v6.2.1` and later
   restore mandatory Windows Authenticode unless another explicit,
   version-bound owner decision is recorded.
12. v6.2.1 unsigned-Windows exception:
   - On 2026-08-10, release run `31343128024` failed closed before candidate
     assembly or public mutation because the SignPath `release-signing` policy
     remained invalid while its `Release certificate 2026` CSR was pending.
     The release owner then explicitly authorized unsigned Windows artifacts
     for `v6.2.1`. The dated decision record is
     `docs/release-control/v6/internal/records/v6.2.1-unsigned-windows-owner-approval-2026-08-10.md`.
   - This is a bounded `v6.2.1` decision, not a standing exception. Stable
     `v6.2.2` and later restore mandatory Authenticode unless another explicit,
     version-bound owner decision is recorded.
   - Windows artifacts remain bound by the exact-SHA candidate manifest,
     checksums, detached `.sig`/`.sshsig` signatures, and published digests.
     Public notes must disclose that the binaries are not Authenticode-signed
     and may display an Unknown Publisher warning.
13. v6.3.0 release-cutoff exception:
   - On 2026-08-22, the release owner reviewed the privacy-safe production
     telemetry posture for `v6.3.0-rc.5` and `v6.3.0-rc.6`, declared current
     `main` at `53ba9786c5522a6839f9cbd3d01c02402556f9eb` the v6.3.0 content
     cutoff, and explicitly directed stable promotion from `v6.3.0-rc.6`
     without completing its normal 72-hour soak. The dated decision record is
     `docs/release-control/v6/internal/records/v6.3.0-stable-cutoff-owner-approval-2026-08-22.md`.
   - This is a bounded v6.3.0 owner-risk acceptance, not soak evidence and not
     a standing exception for later releases. It explicitly accepts the modest
     telemetry cohort, the young `rc.6` follow-up window, and the bounded
     runtime and release-path changes between the promoted tag and the cutoff.
     The workflow input `hotfix_exception=true` transports the approved waiver
     through the shared resolver; it does not reclassify v6.3.0 as a patch
     hotfix.
   - The final release-preparation commit may change only governed version,
     release-note, qualification, test-guardrail, and release-control metadata.
     The exact pushed SHA must pass the no-publication `Release Dry Run` before
     the same SHA is dispatched through the single-build publication workflow.
   - This cutoff decision did not itself waive Windows signing. The release
     owner subsequently recorded a separate version-bound v6.3.0
     unsigned-Windows decision; neither exception broadens the other.
14. v6.3.0 unsigned-Windows exception:
   - On 2026-08-22, the release owner explicitly authorized unsigned Windows
     Unified Agent artifacts for stable `v6.3.0` because Authenticode signing
     is not yet available for this release. The dated decision record is
     `docs/release-control/v6/internal/records/v6.3.0-unsigned-windows-owner-approval-2026-08-22.md`.
   - This is a bounded `v6.3.0` decision, not evidence that Authenticode
     succeeded and not a standing exception. Stable `v6.3.1` and later restore
     mandatory Authenticode unless another explicit, version-bound owner
     decision is recorded.
   - Windows artifacts remain bound by the exact release SHA, immutable
     candidate manifest, SHA-256 checksums, detached `.sig` and `.sshsig`
     signatures, and published-digest verification. Public notes must disclose
     that the binaries are not Authenticode-signed and may display an Unknown
     Publisher warning.
15. v6.3.1 unsigned-Windows exception:
   - On 2026-08-23, the release owner explicitly authorized unsigned Windows
     Unified Agent artifacts for stable `v6.3.1` after exact-SHA rehearsal
     `32634435531` proved that the SignPath production certificate remains
     `CSR PENDING` and the `release-signing` policy is invalid. The dated
     decision record is
     `docs/release-control/v6/internal/records/v6.3.1-unsigned-windows-owner-approval-2026-08-23.md`.
   - This is a bounded `v6.3.1` decision, not evidence that Authenticode
     succeeded and not a standing exception. Stable `v6.3.2` and later restore
     mandatory Authenticode unless another explicit, version-bound owner
     decision is recorded.
   - Windows artifacts remain bound by the exact release SHA, immutable
   candidate manifest, SHA-256 checksums, detached `.sig` and `.sshsig`
   signatures, and published-digest verification. Public notes must disclose
   that the binaries are not Authenticode-signed and may display an Unknown
   Publisher warning.
16. Windows Authenticode unavailable policy from v6.3.2:
   - On 2026-08-25, release run `32896554952` failed closed when the SignPath
     production submission returned `Invalid request to SignPath API` before a
     signing-request record was created. The release owner explicitly directed
     stable releases from `v6.3.2` onward to skip Windows Authenticode while
     production credentials and certificate authorization remain unavailable.
     The dated decision record is
     `docs/release-control/v6/internal/records/windows-authenticode-unavailable-owner-policy-2026-08-25.md`.
   - This standing unavailable state remains active until the release owner
     explicitly confirms that SignPath production credentials and certificate
     authorization are ready. New stable versions must not require per-version
     unsigned allowlist updates while this state is active. Restoring signing
     requires an explicit policy/code change; credential arrival alone must not
     silently change release behavior.
   - Windows artifacts remain bound by the exact release SHA, immutable
     candidate manifest, SHA-256 checksums, detached `.sig` and `.sshsig`
   signatures, and published-digest verification. Public notes must disclose
   that the binaries are not Authenticode-signed and may display an Unknown
   Publisher warning.
17. v6.4.0 expedited stable-cutoff exception:
   - On 2026-08-28, the release owner explicitly directed stable `v6.4.0`
     publication from runtime cutoff
     `18b22d1ebbfe542484652e419320fc7643a792f0`, promoted from published
     `v6.4.0-rc.12`, without another public candidate or the remainder of the
     normal 72-hour soak. The dated decision record is
     `docs/release-control/v6/internal/records/v6.4.0-stable-cutoff-owner-approval-2026-08-28.md`.
   - The accepted reason is active customer harm from incorrect same-name
     Proxmox agent links, repeated inspection load on stopped-container-heavy
     Docker hosts, incomplete Proxmox backup and TrueNAS SMART detail, and
     unstable rolling metric history. The decision explicitly includes the
     bounded product fixes after `v6.4.0-rc.12`.
   - This is a version-bound owner-risk acceptance, not soak evidence and not
     a standing exception for later releases. The workflow input
     `hotfix_exception=true` transports the approved waiver through the shared
     resolver and does not reclassify v6.4.0 as a patch hotfix.
   - The final release-preparation commit may change only governed version,
     release-note, qualification, test-guardrail, and release-control metadata.
     The integrated single-build workflow must pass its exact-SHA preflight and
     immutable readiness gates before any publication boundary. Under the
     current single-build policy, a separate Release Dry Run is optional.
   - The standing Windows Authenticode-unavailable policy from v6.3.2 applies
     independently. It retains checksum, detached-signature, immutable-manifest,
     published-digest, and public Unknown Publisher disclosure controls.

## Single-Build Release Path

1. Every normal alpha, beta, RC, stable, and patch release is initiated once through
   `create-release.yml`. The workflow builds one exact-SHA candidate with the
   native signing lanes required by that version's governed policy while
   frontend, backend, Docker, Helm, and integration checks run in parallel. No
   tag, draft, or public release mutation occurs until those checks and the
   candidate build pass.
2. The release candidate is uploaded as a one-day Actions artifact with a
   machine-readable manifest that pins source SHA, version, filename, size, and
   SHA-256 for every release asset. Publication downloads and verifies that
   exact candidate; it must not rebuild release binaries or installers.
3. Standard staged asset verification compares the candidate
   manifest with GitHub's server-side release-asset SHA-256 digests. It must
   not re-download the multi-gigabyte release packet merely to recompute hashes
   already proven before upload. Manual and release-edit repair validation may
   retain the full-download fallback when no same-run candidate manifest exists.
4. Exact-version Docker publication, staged release-asset verification, exact
   Helm OCI publication, staged install smoke, and the exact-version private
   Pro build begin from the unpublished draft and converge at one immutable
   readiness gate. The pipeline must durably dispatch the release convergence
   run before publication. `activate_release` then publishes the GitHub release,
   verifies the public checksums, installer, provider-MSP bundle, and canonical
   Linux archive URLs, and returns the release to draft quarantine on any
   failure before `release-activation.json` is successfully uploaded. The upload
   is the irreversible release commit point because convergence may observe it
   immediately; activation-side read-back and the final verdict are post-commit
   public proof and may never return the release to draft.
5. Mutable Docker aliases, the live paid-runtime broker, the additive public
   Helm Pages index, and stable demo deployment belong to the separately visible Release
   Convergence run after the commit point. A global repository-ref lease must
   serialize those surfaces across every release version while exact-version
   staging remains version-parallel. Under the lease, every committed release
   must merge its chart into Helm Pages. If a newer committed activation marker
   exists, the superseded run must skip only Docker aliases, the paid-runtime
   broker, and the stable demo rather than move those targets backward.
   Any surface failure leaves the GitHub release committed, fails the convergence
   verdict, releases or stale-recovers the lease, and re-runs the complete
   idempotent convergence workflow. It must not fail a so-called definitive
   release verdict or return the committed release to draft.
   The three reusable mutation workflows expose `workflow_call` only; direct
   dispatch of aliases, Pages, or demo deployment would bypass the lease and is
   prohibited. Non-mutating demo verification remains part of Release Dry Run.
   The activation marker keeps the original convergence owner immutable. After
   that owner completes, a fresh workflow dispatch from repaired `main` may
   adopt the same exact release, source run, target commit, release ID, and R2
   lineage. It must first acquire the global lease, then publish a unique
   immutable owner asset named with its run ID, attempt, and lease SHA. Every
   child receives that exact asset name and digest. A clobbered constant owner
   asset is forbidden because stale CDN bytes could authorize the prior owner.
6. `Release Dry Run` remains the no-public-release rehearsal surface. It calls
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
   harm. It does not remove the integrated exact-SHA candidate and release
   checks, and the reason is recorded in the release metadata.
   A version-bound unsigned Windows decision must additionally be supplied
   through `--unsigned-windows-exception-reason`; it never follows implicitly
   from emergency patch status.
5. The release workflow must await every immutable readiness gate, durably
   enqueue the exact convergence run, and verify the public activation marker.
   `Release Activation Commit Verdict` is the release result. Docker aliases,
   stable demo deployment and browser proof, Helm Pages, and private Pro live
   promotion are post-commit convergence outcomes in the linked workflow run.
   They remain blocking convergence debt until green, but their failure must
   not misreport the committed GitHub release as an atomic rollback.

## Rollout Rules

1. Default installs stay on `stable`.
2. Broad customer announcements and unattended updates target `stable` only.
3. Preview enrollment must be explicit and reversible. The persisted and API
   wire value remains `rc` until a separately governed compatibility migration.
4. Paid production tenants should remain on `stable` unless they are knowingly
   participating in preview validation.

## Rollback Rules

1. Never delete or rewrite shipped tags to hide a bad release; supersede them
   with a newer release and explicit guidance.
2. If a prerelease is bad, hold it in the Preview channel, fix forward, and cut
   the next prerelease. Do not promote it.
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
11. Attach the sanitized historical-credential containment closure packet.
    Every declared Pulse and Pulse Pro redacted subject must have typed
    provider/control-plane closure plus replacement-deployment or
    verified-retirement evidence at the governed minimum tier. A raw gate
    status, narrative approval, current-value difference, or optional history
    rewrite cannot substitute for those records.

## Authority

If conflicts appear:

1. `SOURCE_OF_TRUTH.md` owns the locked decision that this policy is mandatory.
2. `status.json` owns whether the decision is open or resolved and whether the
   active target is release-ready.
3. `PRE_RELEASE_CHECKLIST.md` and
   `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` own execution proof for a
   specific promotion.
