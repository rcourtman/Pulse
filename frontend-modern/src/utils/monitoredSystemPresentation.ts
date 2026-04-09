const normalizeMonitoredSystemValue = (value: string | null | undefined): string =>
  value?.trim().toLowerCase() ?? '';

const titleCaseWords = (value: string): string =>
  value
    .split(/\s+/)
    .filter((part) => part.length > 0)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const MONITORED_SYSTEM_LEDGER_PRESENTATION = {
  briefSummary: 'Billing is based on monitored systems. Child resources are included.',
  sectionTitle: 'Monitored Systems',
  panelTitle: 'Monitored System Ledger',
  disclosureButtonLabel: 'View counting rules',
  disclosureHideLabel: 'Hide counting rules',
  disclosureDefinition:
    'A monitored system is a top-level machine or cluster Pulse actively monitors. Each system counts once no matter how Pulse collects it. Child resources like VMs, containers, pods, disks, backups, and services are included.',
  ledgerDescription:
    'Review the monitored systems currently counted against your Pulse Pro plan limit.',
  tableNameLabel: 'Name',
  tableStatusLabel: 'Status',
  tableLatestIncludedSignalLabel: 'Latest Included Signal',
  countedSystemBadgeLabel: 'Counts as 1 monitored system',
  groupedSourcesHeading: 'Grouped sources',
  countingExplanationHeading: 'Why this counts',
  continuityHeading: 'Plan continuity',
  continuityPlanLimitLabel: 'Plan limit',
  continuityEffectiveLimitLabel: 'Effective limit',
  continuityGrandfatheredFloorLabel: 'Grandfathered floor',
  continuityCaptureLabel: 'Continuity capture',
  continuityCapturePendingLabel: 'Pending',
  continuityCaptureCapturedLabel: 'Captured',
  usageVerifyingLabel: 'Verifying…',
  remainingCapacityUnavailableLabel: 'Unavailable',
  unlimitedLimitLabel: 'Unlimited',
  loadingState: {
    text: 'Loading monitored system usage…',
  },
  errorState: {
    title: 'Monitored system usage is temporarily unavailable.',
    retryingLabel: 'Trying again…',
    retryLabel: 'Try again',
  },
  unavailableState: {
    title: 'Verifying monitored-system inventory',
    fallbackMessage:
      'Pulse cannot currently verify monitored-system usage for this installation. Refresh after the monitoring runtime settles.',
    unsettledMessage:
      'Pulse is still collecting the first provider-owned inventory baseline. The monitored-system ledger will appear after that baseline completes.',
    rebuildPendingMessage:
      'Pulse has collected provider-owned inventory and is rebuilding the canonical monitored-system ledger. Usage will appear when that rebuild finishes.',
  },
  policyLoadingState: {
    title: 'Checking monitored-system visibility',
    message:
      'Pulse waits for the session presentation policy before loading usage or plan-limit data.',
  },
  hiddenState: {
    title: 'Monitored-system usage is hidden in demo mode',
    message:
      'The public demo uses sample infrastructure data, so Pulse hides counted-system totals, plan limits, and upgrade actions instead of creating a demo license.',
  },
  countingDetailsCollapsedLabel: 'View counting details',
  countingDetailsExpandedLabel: 'Hide counting details',
  currentStatusHeading: 'Current status',
  latestIncludedSignalSummaryLabel: 'Latest included signal',
  includedCollectionPathsHeading: 'Included collection paths',
  emptyState: 'No monitored systems counted.',
  noIncludedSignalLabel: 'No included signal yet.',
  fallbackExplanationSummary:
    'Pulse counts this top-level collection path as one monitored system.',
  statusSummaryByStatus: {
    online: 'All included top-level collection paths currently report online status.',
    warning:
      'At least one included top-level collection path is degraded, so Pulse marks this monitored system as warning.',
    offline:
      'At least one included source is offline or disconnected, so Pulse marks this monitored system as offline.',
    unknown: 'Pulse cannot determine a canonical runtime status for this monitored system yet.',
  },
  limitBanner: {
    learnMoreLabel: 'Learn more',
    installCollectorsLabel: 'Install v6 collectors',
    upgradeLabel: 'Upgrade to add more',
    overflowSummaryPrefix: 'Includes 1 temporary onboarding slot',
    legacyConnectionSuffix:
      'that count once toward your monitored-system cap when the same top-level system is discovered canonically.',
  },
  admissionPreview: {
    requiredTitle: 'Preview monitored-system impact before saving',
    requiredMessage:
      'Pulse must verify monitored-system capacity for this platform connection before it can be saved.',
    unavailableTitle: 'Monitored-system capacity is temporarily unavailable',
    unavailableFallbackMessage:
      'Pulse cannot verify monitored-system capacity right now, so this connection cannot be saved yet. Retry preview in a moment.',
    unavailableUnsettledMessage:
      'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
    unavailableRebuildPendingMessage:
      'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view, so this connection cannot be saved yet. Retry preview in a moment.',
    saveBlockedLimitMessage: 'This change would exceed your monitored-system limit',
    saveBlockedLoadingMessage: 'Wait for the monitored-system impact preview to finish',
  },
} as const;

const MONITORED_SYSTEM_USAGE_UNAVAILABLE_ERROR_CODE = 'monitored_system_usage_unavailable';

export type MonitoredSystemLegacyConnectionCounts = {
  proxmox_nodes: number;
  docker_hosts: number;
  kubernetes_clusters: number;
};

export type MonitoredSystemLimitUsageStatus = {
  current?: number | null;
  limit?: number | null;
  current_available?: boolean | null;
  current_unavailable_reason?: string | null;
  state?: string | null;
};

export type MonitoredSystemAdmissionPreviewUnavailableState = {
  reason: string | null;
  title: string;
  message: string;
};

export type MonitoredSystemAdmissionPreviewSaveState = {
  preview?: { would_exceed_limit?: boolean | null } | null;
  unavailableState?: MonitoredSystemAdmissionPreviewUnavailableState | null;
  error?: string | null;
  loading?: boolean | null;
};

export function getMonitoredSystemLedgerPresentation() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION;
}

export function getMonitoredSystemBriefSummary(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.briefSummary;
}

export function getMonitoredSystemDisclosureToggleLabel(open: boolean): string {
  return open
    ? MONITORED_SYSTEM_LEDGER_PRESENTATION.disclosureHideLabel
    : MONITORED_SYSTEM_LEDGER_PRESENTATION.disclosureButtonLabel;
}

export function getMonitoredSystemDisclosureDefinition(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.disclosureDefinition;
}

export function getMonitoredSystemLedgerDescription(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.ledgerDescription;
}

export function getMonitoredSystemLedgerLoadingState() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.loadingState;
}

export function getMonitoredSystemLedgerErrorState() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.errorState;
}

export function getMonitoredSystemLedgerUnavailableState() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.unavailableState;
}

export function getMonitoredSystemLedgerPolicyLoadingState() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.policyLoadingState;
}

export function getMonitoredSystemLedgerHiddenState() {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.hiddenState;
}

export function formatMonitoredSystemUsageUnavailableMessage(reason?: string | null): string {
  switch (normalizeMonitoredSystemValue(reason)) {
    case 'supplemental_inventory_unsettled':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.unavailableState.unsettledMessage;
    case 'supplemental_inventory_rebuild_pending':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.unavailableState.rebuildPendingMessage;
    default:
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.unavailableState.fallbackMessage;
  }
}

export function formatMonitoredSystemLedgerUnavailableMessage(reason?: string | null): string {
  return formatMonitoredSystemUsageUnavailableMessage(reason);
}

export function isMonitoredSystemLimitUsageAvailable(
  limit?: MonitoredSystemLimitUsageStatus | null,
): boolean {
  return limit?.current_available !== false;
}

export function getMonitoredSystemLimitUnavailableReason(
  limit?: MonitoredSystemLimitUsageStatus | null,
): string | undefined {
  if (isMonitoredSystemLimitUsageAvailable(limit)) return undefined;
  return limit?.current_unavailable_reason?.trim() || undefined;
}

export function getMonitoredSystemLimitUsageSummary(
  limit?: MonitoredSystemLimitUsageStatus | null,
): string | number {
  if (!isMonitoredSystemLimitUsageAvailable(limit)) {
    return MONITORED_SYSTEM_LEDGER_PRESENTATION.usageVerifyingLabel;
  }

  const current = typeof limit?.current === 'number' ? limit.current : 0;
  const planLimit = typeof limit?.limit === 'number' ? limit.limit : 0;
  if (planLimit > 0) {
    return `${current} / ${planLimit}`;
  }
  return current;
}

export function getMonitoredSystemLimitRemainingCapacity(
  limit?: MonitoredSystemLimitUsageStatus | null,
): string | number {
  if (!isMonitoredSystemLimitUsageAvailable(limit)) {
    return MONITORED_SYSTEM_LEDGER_PRESENTATION.remainingCapacityUnavailableLabel;
  }

  const current = typeof limit?.current === 'number' ? limit.current : 0;
  const planLimit = typeof limit?.limit === 'number' ? limit.limit : 0;
  if (planLimit <= 0) {
    return MONITORED_SYSTEM_LEDGER_PRESENTATION.unlimitedLimitLabel;
  }
  return Math.max(planLimit - current, 0);
}

export function isMonitoredSystemLimitUrgent(
  limit?: MonitoredSystemLimitUsageStatus | null,
): boolean {
  if (!limit || !isMonitoredSystemLimitUsageAvailable(limit)) return false;
  const state = normalizeMonitoredSystemValue(limit.state ?? undefined);
  return state === 'warning' || state === 'enforced';
}

export function getMonitoredSystemCountingDetailsToggleLabel(expanded: boolean): string {
  return expanded
    ? MONITORED_SYSTEM_LEDGER_PRESENTATION.countingDetailsExpandedLabel
    : MONITORED_SYSTEM_LEDGER_PRESENTATION.countingDetailsCollapsedLabel;
}

export function getMonitoredSystemExplanationFallbackSummary(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.fallbackExplanationSummary;
}

export function getMonitoredSystemStatusFallbackSummary(
  status: 'online' | 'warning' | 'offline' | 'unknown' = 'unknown',
): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.statusSummaryByStatus[status];
}

export function getMonitoredSystemLimitLearnMoreLabel(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.limitBanner.learnMoreLabel;
}

export function getMonitoredSystemLimitInstallCollectorsLabel(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.limitBanner.installCollectorsLabel;
}

export function getMonitoredSystemLimitUpgradeLabel(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.limitBanner.upgradeLabel;
}

export function getMonitoredSystemAdmissionPreviewUnavailableTitle(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.unavailableTitle;
}

export function getMonitoredSystemAdmissionPreviewRequiredState(): {
  title: string;
  message: string;
} {
  return {
    title: MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.requiredTitle,
    message: MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.requiredMessage,
  };
}

export function formatMonitoredSystemAdmissionPreviewUnavailableMessage(reason?: string): string {
  switch (normalizeMonitoredSystemValue(reason)) {
    case 'supplemental_inventory_unsettled':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.unavailableUnsettledMessage;
    case 'supplemental_inventory_rebuild_pending':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.unavailableRebuildPendingMessage;
    default:
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.unavailableFallbackMessage;
  }
}

export function buildMonitoredSystemAdmissionPreviewUnavailableState(input: {
  code?: string | null;
  reason?: string | null;
}): MonitoredSystemAdmissionPreviewUnavailableState | null {
  if (normalizeMonitoredSystemValue(input.code) !== MONITORED_SYSTEM_USAGE_UNAVAILABLE_ERROR_CODE) {
    return null;
  }

  const reason = input.reason?.trim() || null;
  return {
    reason,
    title: getMonitoredSystemAdmissionPreviewUnavailableTitle(),
    message: formatMonitoredSystemAdmissionPreviewUnavailableMessage(reason ?? undefined),
  };
}

export function isMonitoredSystemAdmissionPreviewResolvedSafely(
  state: MonitoredSystemAdmissionPreviewSaveState,
): boolean {
  return (
    !state.loading &&
    Boolean(state.preview) &&
    state.preview?.would_exceed_limit !== true &&
    !state.unavailableState &&
    !state.error?.trim()
  );
}

export function getMonitoredSystemAdmissionPreviewSaveBlockedMessage(
  state: MonitoredSystemAdmissionPreviewSaveState,
): string | null {
  if (isMonitoredSystemAdmissionPreviewResolvedSafely(state)) {
    return null;
  }

  if (state.loading) {
    return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.saveBlockedLoadingMessage;
  }
  if (state.unavailableState) {
    return state.unavailableState.message;
  }
  if (state.preview?.would_exceed_limit) {
    return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.saveBlockedLimitMessage;
  }

  const error = state.error?.trim();
  if (error) {
    return error;
  }

  return MONITORED_SYSTEM_LEDGER_PRESENTATION.admissionPreview.requiredMessage;
}

export function formatMonitoredSystemLimitSummary(limit: {
  current: number;
  limit: number;
}): string {
  return `Monitored systems: ${limit.current}/${limit.limit}`;
}

export function formatMonitoredSystemLegacyConnectionBreakdown(
  counts: MonitoredSystemLegacyConnectionCounts,
): string {
  const parts: string[] = [];

  if (counts.proxmox_nodes > 0) {
    parts.push(`${counts.proxmox_nodes} Proxmox ${counts.proxmox_nodes === 1 ? 'node' : 'nodes'}`);
  }
  if (counts.docker_hosts > 0) {
    parts.push(`${counts.docker_hosts} Docker ${counts.docker_hosts === 1 ? 'host' : 'hosts'}`);
  }
  if (counts.kubernetes_clusters > 0) {
    parts.push(
      `${counts.kubernetes_clusters} Kubernetes ${
        counts.kubernetes_clusters === 1 ? 'cluster' : 'clusters'
      }`,
    );
  }

  return parts.join(', ');
}

export function formatMonitoredSystemMigrationMessage(
  counts: MonitoredSystemLegacyConnectionCounts,
): string {
  const total = counts.proxmox_nodes + counts.docker_hosts + counts.kubernetes_clusters;
  if (total <= 0) return '';

  const noun = total === 1 ? 'resource' : 'resources';
  const breakdown = formatMonitoredSystemLegacyConnectionBreakdown(counts);
  return `You also have ${total} ${noun} connected via API or legacy collectors${
    breakdown ? ` (${breakdown})` : ''
  } ${MONITORED_SYSTEM_LEDGER_PRESENTATION.limitBanner.legacyConnectionSuffix}`;
}

export function formatMonitoredSystemOverflowSummary(daysRemaining: number | undefined): string {
  if (!daysRemaining) return '';
  return `${MONITORED_SYSTEM_LEDGER_PRESENTATION.limitBanner.overflowSummaryPrefix} (${daysRemaining}d remaining)`;
}

export function formatMonitoredSystemLatestIncludedSignalSentence(signal: {
  attribution: string;
  relative: string;
}): string {
  return `${MONITORED_SYSTEM_LEDGER_PRESENTATION.latestIncludedSignalSummaryLabel}: ${signal.attribution}, reported ${signal.relative}.`;
}

export function formatMonitoredSystemGroupedSourcesLabel(count: number): string {
  return `${count} grouped ${count === 1 ? 'source' : 'sources'}`;
}

export function getMonitoredSystemSourceLabel(source: string | undefined): string {
  switch (normalizeMonitoredSystemValue(source)) {
    case 'agent':
      return 'Agent';
    case 'docker':
      return 'Docker';
    case 'kubernetes':
      return 'Kubernetes';
    case 'multiple':
      return 'Multiple Sources';
    case 'pbs':
      return 'PBS';
    case 'pmg':
      return 'PMG';
    case 'proxmox':
      return 'Proxmox';
    case 'vmware':
      return 'VMware';
    case 'truenas':
      return 'TrueNAS';
    case '':
    case 'unknown':
      return '';
    default:
      return source?.trim() ?? '';
  }
}

export function getMonitoredSystemSurfaceTypeLabel(type: string | undefined): string {
  switch (normalizeMonitoredSystemValue(type)) {
    case 'agent':
      return 'Host';
    case 'docker-host':
      return 'Docker Host';
    case 'host':
      return 'Host';
    case 'kubernetes-cluster':
      return 'Kubernetes Cluster';
    case 'pbs-server':
      return 'PBS Server';
    case 'pmg-server':
      return 'PMG Server';
    case 'proxmox-node':
      return 'Proxmox Node';
    case 'truenas-system':
      return 'TrueNAS System';
    case '':
      return 'System';
    default:
      return titleCaseWords((type ?? '').trim().replace(/[-_]+/g, ' '));
  }
}

export function formatMonitoredSystemSurfaceAttribution(surface: {
  name: string;
  type?: string;
  source?: string;
}): string {
  const name = surface.name?.trim() || 'Unnamed source';
  const typeLabel = getMonitoredSystemSurfaceTypeLabel(surface.type);
  const sourceLabel = getMonitoredSystemSourceLabel(surface.source);
  if (sourceLabel === '' || sourceLabel.toLowerCase() === typeLabel.toLowerCase()) {
    return `${name} (${typeLabel})`;
  }
  return `${name} (${typeLabel} via ${sourceLabel})`;
}
