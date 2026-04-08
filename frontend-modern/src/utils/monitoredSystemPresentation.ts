const normalizeMonitoredSystemValue = (value: string | undefined): string =>
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
  loadingState: {
    text: 'Loading monitored system usage…',
  },
  errorState: {
    title: 'Monitored system usage is temporarily unavailable.',
    retryingLabel: 'Trying again…',
    retryLabel: 'Try again',
  },
  countingDetailsCollapsedLabel: 'View counting details',
  countingDetailsExpandedLabel: 'Hide counting details',
  currentStatusHeading: 'Current status',
  latestIncludedSignalSummaryLabel: 'Latest included signal',
  includedCollectionPathsHeading: 'Included collection paths',
  emptyState: 'No monitored systems counted.',
  noIncludedSignalLabel: 'No included signal yet.',
  fallbackExplanationSummary: 'Pulse counts this top-level collection path as one monitored system.',
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
} as const;

export type MonitoredSystemLegacyConnectionCounts = {
  proxmox_nodes: number;
  docker_hosts: number;
  kubernetes_clusters: number;
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
    parts.push(
      `${counts.proxmox_nodes} Proxmox ${counts.proxmox_nodes === 1 ? 'node' : 'nodes'}`,
    );
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

export function formatMonitoredSystemOverflowSummary(
  daysRemaining: number | undefined,
): string {
  if (!daysRemaining) return '';
  return `${MONITORED_SYSTEM_LEDGER_PRESENTATION.limitBanner.overflowSummaryPrefix} (${daysRemaining}d remaining)`;
}

export function formatMonitoredSystemLatestIncludedSignalSentence(signal: {
  attribution: string;
  relative: string;
}): string {
  return `${MONITORED_SYSTEM_LEDGER_PRESENTATION.latestIncludedSignalSummaryLabel}: ${signal.attribution}, reported ${signal.relative}.`;
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
