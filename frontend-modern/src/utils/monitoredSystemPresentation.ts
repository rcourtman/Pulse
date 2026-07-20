const normalizeMonitoredSystemValue = (value: string | null | undefined): string =>
  value?.trim().toLowerCase() ?? '';

const titleCaseWords = (value: string): string =>
  value
    .split(/\s+/)
    .filter((part) => part.length > 0)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const MONITORED_SYSTEM_LEDGER_PRESENTATION = {
  briefSummary:
    'Pulse counts top-level monitored systems. Child resources underneath them are included.',
  sectionTitle: 'Monitored Systems',
  panelTitle: 'Monitored System Ledger',
  disclosureButtonLabel: 'View counting rules',
  disclosureHideLabel: 'Hide counting rules',
  disclosureDefinition:
    'A monitored system is a top-level monitored root such as a Docker host, Kubernetes cluster, Proxmox node, standalone host, or TrueNAS system. Each root counts once no matter how Pulse collects it. Child resources like VMs, containers, pods, disks, backups, and services underneath that root are included.',
  ledgerDescription:
    'Review the top-level monitored systems Pulse has identified for reporting and support context.',
  tableNameLabel: 'Name',
  tableStatusLabel: 'Status',
  tableLatestIncludedSignalLabel: 'Latest Included Signal',
  countedSystemBadgeLabel: 'Counts as 1 monitored system',
  groupedSourcesHeading: 'Grouped sources',
  countingExplanationHeading: 'Why this counts',
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
      'Pulse waits for the session visibility state before loading monitored-system usage details.',
  },
  hiddenState: {
    title: 'Monitored-system usage is hidden in demo mode',
    message:
      'The public demo uses sample infrastructure data, so Pulse hides counted-system totals and billing actions instead of creating a demo license.',
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
  impactPreview: {
    fallbackTitle: 'Monitored-system impact',
    addsSystemsTitle: 'This change adds monitored systems',
    removesSystemsTitle: 'This change removes monitored systems',
    unchangedTitle: 'This change keeps monitored-system count unchanged',
    unavailableTitle: 'Monitored-system verification is temporarily unavailable',
    unavailableFallbackMessage:
      'Pulse cannot verify monitored-system impact right now. You can still save the connection and review the impact after inventory refreshes.',
    unavailableUnsettledMessage:
      'Pulse is still settling provider-owned inventory for this platform connection. You can still save the connection and review the impact after the first baseline finishes.',
    unavailableRebuildPendingMessage:
      'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view. You can still save the connection and review the impact in a moment.',
  },
} as const;

const MONITORED_SYSTEM_USAGE_UNAVAILABLE_ERROR_CODE = 'monitored_system_usage_unavailable';

export type MonitoredSystemImpactPreviewUnavailableState = {
  reason: string | null;
  title: string;
  message: string;
};

export type MonitoredSystemImpactPreviewTitleInput = {
  current_count?: number | null;
  projected_count?: number | null;
};

export type MonitoredSystemImpactPreviewSummaryInput = MonitoredSystemImpactPreviewTitleInput;

const normalizeImpactPreviewCount = (count: number | null | undefined): number =>
  typeof count === 'number' && Number.isFinite(count) ? Math.max(0, count) : 0;

const formatImpactPreviewCount = (count: number): string =>
  `${count} monitored ${count === 1 ? 'system' : 'systems'}`;

const formatImpactPreviewDelta = (delta: number): string => (delta > 0 ? `+${delta}` : `${delta}`);

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

export function getMonitoredSystemImpactPreviewUnavailableTitle(): string {
  return MONITORED_SYSTEM_LEDGER_PRESENTATION.impactPreview.unavailableTitle;
}

export function getMonitoredSystemImpactPreviewTitle(
  preview: MonitoredSystemImpactPreviewTitleInput | null | undefined,
): string {
  const presentation = MONITORED_SYSTEM_LEDGER_PRESENTATION.impactPreview;
  if (!preview) return presentation.fallbackTitle;

  const current = normalizeImpactPreviewCount(preview.current_count);
  const projected =
    typeof preview.projected_count === 'number' && Number.isFinite(preview.projected_count)
      ? normalizeImpactPreviewCount(preview.projected_count)
      : current;
  const delta = projected - current;

  if (delta > 0) return presentation.addsSystemsTitle;
  if (delta < 0) return presentation.removesSystemsTitle;
  return presentation.unchangedTitle;
}

export function formatMonitoredSystemImpactPreviewSummary(
  preview: MonitoredSystemImpactPreviewSummaryInput,
): string {
  const current = normalizeImpactPreviewCount(preview.current_count);
  const projected =
    typeof preview.projected_count === 'number' && Number.isFinite(preview.projected_count)
      ? normalizeImpactPreviewCount(preview.projected_count)
      : current;
  const delta = projected - current;
  const currentSummary = `Pulse currently counts ${formatImpactPreviewCount(current)}.`;

  if (delta !== 0) {
    return `${currentSummary} Saving this change would bring the count to ${formatImpactPreviewCount(
      projected,
    )} (${formatImpactPreviewDelta(delta)}).`;
  }

  return `${currentSummary} Saving this change would keep the count at ${formatImpactPreviewCount(
    projected,
  )}.`;
}

export function formatMonitoredSystemImpactPreviewUnavailableMessage(reason?: string): string {
  switch (normalizeMonitoredSystemValue(reason)) {
    case 'supplemental_inventory_unsettled':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.impactPreview.unavailableUnsettledMessage;
    case 'supplemental_inventory_rebuild_pending':
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.impactPreview.unavailableRebuildPendingMessage;
    default:
      return MONITORED_SYSTEM_LEDGER_PRESENTATION.impactPreview.unavailableFallbackMessage;
  }
}

export function buildMonitoredSystemImpactPreviewUnavailableState(input: {
  code?: string | null;
  reason?: string | null;
}): MonitoredSystemImpactPreviewUnavailableState | null {
  if (normalizeMonitoredSystemValue(input.code) !== MONITORED_SYSTEM_USAGE_UNAVAILABLE_ERROR_CODE) {
    return null;
  }

  const reason = input.reason?.trim() || null;
  return {
    reason,
    title: getMonitoredSystemImpactPreviewUnavailableTitle(),
    message: formatMonitoredSystemImpactPreviewUnavailableMessage(reason ?? undefined),
  };
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
