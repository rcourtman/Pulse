/**
 * Specs that are deliberately not run. Quarantine is reserved for known,
 * unresolved product/spec incompatibilities and is never an automatic result
 * of an infrastructure failure.
 */
export const QUARANTINED_SPECS = [
  // Multi-tenant scenarios 6/7 have an unresolved org-resolution bug (see
  // core-e2e-failure-taxonomy): after switchOrg+reload the UI still resolves
  // the default org. CI runs a multi-tenant environment, so delisting this
  // spec turns that open bug into a permanent red.
  '**/03-multi-tenant.spec.ts',
  '**/47-inline-selection-scroll-stability.spec.ts',
  '**/48-summary-hover-selection.spec.ts',
];

/**
 * Specs that run on every push but do not gate the Core E2E verdict. The
 * stable tier is every non-quarantined spec not listed here.
 *
 * Audit (2026-08-11): the 10 most recent completed main workflow records at
 * 2026-08-11T20:22:40Z contained six executed runs and four concurrency
 * cancellations with no jobs. Only explicit Playwright `failed` or `flaky`
 * summaries in the 48 executed shard logs count as incidents; progress lines
 * and cancelled/non-executed runs do not. The audit found 13 stable-spec
 * incidents and no shard-wide environment collapse. Existing probation specs
 * remain because six evidence-bearing runs cannot prove the promotion rule.
 *
 * Promotion rule: a spec returns to stable after 10 consecutive executed Core
 * E2E runs on main with neither a failure nor a retry-pass (`flaky`). Demotion
 * rule: one genuine spec failure/flake on main returns a stable spec here. A
 * shared environment or runner failure is recorded separately and never
 * demotes every spec that happened to be scheduled on the affected shard.
 */
export const PROBATION_SPECS = [
  '**/01-core-e2e.spec.ts',
  '**/02-navigation-perf.spec.ts',
  '**/04-mobile.spec.ts',
  '**/05-settings-mobile-audit.spec.ts',
  '**/06-theme-visual.spec.ts',
  '**/11-first-session.spec.ts',
  '**/15-settings-shell-consistency.spec.ts',
  '**/17-proxmox-backups-layout.spec.ts',
  '**/20-local-doc-links.spec.ts',
  '**/21-truenas-connections-workspace.spec.ts',
  '**/28-truenas-alert-resource-links.spec.ts',
  '**/36-vmware-alert-history-resource-incidents.spec.ts',
  '**/38-vmware-ai-chat-mentions.spec.ts',
  '**/39-vmware-resource-detail-drawer.spec.ts',
  '**/40-vmware-storage-source-filter.spec.ts',
  '**/41-vmware-phase1-exclusion-integrity.spec.ts',
  '**/42-vmware-ai-chat-read-recovery.spec.ts',
  '**/43-platform-mock-runtime.spec.ts',
  '**/44-workloads-chart-spacing.spec.ts',
  '**/45-workloads-memory-tail.spec.ts',
  '**/46-storage-summary-continuity.spec.ts',
  '**/49-demo-scenario-curation.spec.ts',
  '**/50-storage-physical-disk-io-history.spec.ts',
  '**/52-ai-settings-provider-setup.spec.ts',
  '**/53-demo-mode-commercial-boundary.spec.ts',
  '**/55-self-hosted-upgrade-return.spec.ts',
  '**/56-pulse-account-upgrade-bootstrap.spec.ts',
  '**/59-self-hosted-plans-entitlement-summary.spec.ts',
  '**/59-workloads-column-layout.spec.ts',
  '**/62-storage-growth-column.spec.ts',
  '**/64-workloads-proxmox-refresh-stability.spec.ts',
  '**/68-infrastructure-onboarding.spec.ts',
  '**/68-platform-pages-shell.spec.ts',
  '**/70-self-hosted-manual-activation-success.spec.ts',
  '**/75-agent-integrations-surface-contract.spec.ts',
  '**/76-assistant-tool-output-preview.spec.ts',
  '**/77-msp-isolation.spec.ts',
  '**/78-monitor-first-patrol-workbench.spec.ts',
  '**/79-update-flow.spec.ts',
  '**/81-actions-inbox.spec.ts',
  '**/90-operational-trust-protection-posture.spec.ts',
  '**/91-operational-trust-attention-workbench.spec.ts',
  '**/91-proxmox-node-display-names.spec.ts',
  '**/92-operational-trust-availability-facet.spec.ts',
];
