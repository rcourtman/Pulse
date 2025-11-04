# Installer v2 Rollout Playbook

This playbook captures the agreed rollout process for major installer updates
(`install.sh`, host/agent installers, and supporting bundles). Use it when
cutting a new installer generation or deprecating legacy flows.

---

## 1. Define Objectives

- Document the functional changes (new flags, platforms, security updates).
- Outline compatibility expectations (supported OS versions, container
  runtimes, Proxmox releases).
- Decide which behaviours remain behind feature flags or environment toggles.

Create a tracking issue with:
- Summary of the change
- Owners for implementation, docs, and QA
- Planned release milestone

---

## 2. Build in Parallel

While the existing installer continues shipping:

1. Implement changes under `scripts/install-*-v2.sh` (keep the original script
   untouched).
2. Gate sensitive logic behind `PULSE_INSTALLER_V2` or similar flags so you can
   exercise new code paths without breaking production users.
3. Add migration scaffolding to handle legacy configs—record details in
   `MIGRATION_SCAFFOLDING.md`.

---

## 3. Testing Matrix

Cover these scenarios before merging:

| Scenario | Notes |
|----------|-------|
| Fresh install on Debian/Ubuntu | Include root + non-root invocation, interactive + `--force`. |
| Proxmox LXC creation | Validate resource sizing, storage defaults, and rollback on failure. |
| Docker / Compose flow | Ensure bind mounts, auto-hash, and setup wizard paths remain intact. |
| Upgrade in place | Re-run installer on an existing deployment; preserve data and services. |
| Air-gapped mode | Exercise `--offline` or pre-downloaded assets if supported. |
| Uninstall / cleanup | Confirm services and temp files are removed where promised. |

Automate what you can with `scripts/tests/integration/`. Record any manual
checks in the tracking issue.

---

## 4. Documentation Updates

- Update end-user docs (`docs/INSTALL.md`, Docker/Kubernetes guides) with new
  flags and workflows.
- Refresh quick-start snippets in `README.md`.
- Note behavioural changes in `docs/RELEASE_NOTES.md` under the relevant
  version once the rollout ships.
- For script contributors, update `docs/CONTRIBUTING-SCRIPTS.md` and
  `docs/script-library-guide.md` with new patterns.

---

## 5. Staged Rollout

1. Merge v2 into `main` behind a feature flag or version guard.
2. Ask beta testers to opt-in via `--use-installer-v2` (or similar).
3. Monitor GitHub issues / Discord for feedback.
4. Iterate quickly on regressions—keep the old installer intact until v2 is
   stable.

When metrics show low regression risk, switch the default path to v2 but keep
the v1 flag available for at least one stable release.

---

## 6. Sunset Legacy Paths

- Announce deprecation in release notes at least one version before removal.
- Remove the legacy flag after the deprecation window.
- Delete scaffolding and update `MIGRATION_SCAFFOLDING.md` to reflect the
  cleanup.

---

## 7. Post-Rollout Checklist

- Audit bundled artifacts (`dist/*.sh`) for unexpected diffs.
- Regenerate checksums and publish updated hashes if distributing binaries.
- Close the tracking issue with a summary of lessons learned and follow-up
  tasks (e.g., telemetry improvements).

Keeping this playbook up to date ensures future installer iterations follow the
same predictable rollout process and reduces duplicate tribal knowledge.

