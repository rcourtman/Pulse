import type { Connection } from '@/api/connections';
import type { ConnectionType } from '@/api/connections';

export const connectionLastActivityText = (connection: Connection): string => {
  if (!connection.lastSeen) return 'No activity yet';
  const ts = Date.parse(connection.lastSeen);
  if (Number.isNaN(ts)) return 'Unknown';
  const diff = Math.max(0, Date.now() - ts);
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return `${sec}s ago`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const days = Math.floor(hr / 24);
  return `${days}d ago`;
};

export interface ConnectionAgentVersionPresentation {
  badgeClassName: string;
  badgeLabel: string;
  detail: string;
  title: string;
}

export const connectionAgentVersionPresentation = (
  connection: Connection,
): ConnectionAgentVersionPresentation | null => {
  const currentVersion = connection.agentVersion?.trim() ?? '';
  const expectedVersion = connection.expectedAgentVersion?.trim() ?? '';

  if (connection.agentUpdateAvailable && currentVersion && expectedVersion) {
    return {
      badgeClassName:
        'inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-900 dark:text-amber-200',
      badgeLabel: 'Update available',
      detail: `${currentVersion} -> ${expectedVersion}`,
      title: `Pulse Agent update available: ${currentVersion} -> ${expectedVersion}`,
    };
  }

  if (currentVersion) {
    return {
      badgeClassName:
        'inline-flex items-center rounded-full border border-border bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-base-content',
      badgeLabel: 'Agent version',
      detail: currentVersion,
      title: `Pulse Agent version ${currentVersion}`,
    };
  }

  if (expectedVersion) {
    return {
      badgeClassName:
        'inline-flex items-center rounded-full border border-border bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-base-content',
      badgeLabel: 'Version target',
      detail: expectedVersion,
      title: `Pulse Agent target version ${expectedVersion}`,
    };
  }

  return null;
};

export interface InfrastructureSystemRow {
  id: string;
  ownerType: ConnectionType;
  name: string;
  subtitle?: string;
  host?: string;
  coverageLabels: string[];
  sourceBadges: string[];
  statusLabel: string;
  statusClassName: string;
  agentUpdateCount: number;
  lastActivityText: string;
  lastErrorMessage?: string;
  enabled: boolean;
  canEdit: boolean;
  canPause: boolean;
  canRemove: boolean;
  isAgent: boolean;
  attachedConnections: Connection[];
  connection: Connection;
}
