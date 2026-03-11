import { formatRelativeTime } from '@/utils/format';

export interface UnifiedAgentInventoryLike {
  capabilities: string[];
}

export interface UnifiedAgentLastSeenLike {
  status: string;
  lastSeen?: number;
  removedAt?: number;
}

export function getInventorySubjectLabel(name?: string, fallback?: string): string {
  return name || fallback || 'this host';
}

export function getRemovedUnifiedAgentItemLabel(row: UnifiedAgentInventoryLike): string {
  if (row.capabilities.includes('kubernetes') && !row.capabilities.includes('agent')) {
    return 'Kubernetes cluster';
  }
  if (row.capabilities.includes('docker')) {
    return 'Docker runtime';
  }
  if (row.capabilities.includes('proxmox')) {
    return 'Proxmox node';
  }
  return 'Host agent';
}

export function getUnifiedAgentLastSeenLabel(
  row: UnifiedAgentLastSeenLike,
  monitoringStoppedLabel: string,
): string {
  if (row.status === 'removed') {
    return row.removedAt
      ? `Monitoring stopped ${formatRelativeTime(row.removedAt)}`
      : monitoringStoppedLabel;
  }
  return row.lastSeen ? formatRelativeTime(row.lastSeen) : '—';
}

export function getMonitoringStoppedEmptyState(hasFilters: boolean): string {
  return hasFilters
    ? 'No monitoring-stopped items match the current filters.'
    : 'No infrastructure currently has monitoring stopped.';
}

export function getAgentLedgerLoadingState() {
  return {
    text: 'Loading agent ledger...',
  } as const;
}

export function getAgentLedgerErrorState() {
  return {
    title: 'Failed to load agent ledger.',
    retryingLabel: 'Retrying…',
    retryLabel: 'Retry',
  } as const;
}

export function getUnifiedAgentStopMonitoringUnavailableMessage(): string {
  return 'No host identifiers are available to stop monitoring.';
}

export function getUnifiedAgentStopMonitoringSuccessMessage(subject: string): string {
  return `Monitoring stopped for ${subject}. Pulse will ignore future reports until reconnect is allowed.`;
}

export function getUnifiedAgentStopMonitoringErrorMessage(subject: string): string {
  return `Failed to stop monitoring ${subject}.`;
}

export function getUnifiedAgentAllowReconnectSuccessMessage(subject: string): string {
  return `Reconnect allowed for ${subject}. Pulse will accept reports from it again.`;
}

export function getUnifiedAgentAllowReconnectErrorMessage(subject: string): string {
  return `Failed to allow reconnect for ${subject}.`;
}

export function getUnifiedAgentCommandsUnavailableMessage(): string {
  return 'Agent ID unavailable for command configuration';
}

export function getUnifiedAgentConfigUpdateSuccessMessage(enabled: boolean): string {
  return `Pulse command execution ${enabled ? 'enabled' : 'disabled'}. Syncing with agent...`;
}

export function getUnifiedAgentConfigUpdateErrorMessage(): string {
  return 'Failed to update agent configuration';
}

export function getUnifiedAgentClipboardCopySuccessMessage(): string {
  return 'Copied to clipboard';
}

export function getUnifiedAgentClipboardCopyErrorMessage(): string {
  return 'Failed to copy';
}

export function getUnifiedAgentUninstallCommandCopiedMessage(): string {
  return 'Uninstall command copied';
}

export function getUnifiedAgentUpgradeCommandCopiedMessage(): string {
  return 'Upgrade command copied';
}
