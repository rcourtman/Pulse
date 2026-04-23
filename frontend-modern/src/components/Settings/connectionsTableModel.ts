import type { Connection } from '@/api/connections';
import type { ConnectionType } from '@/api/connections';

export const lastActivityTextFromLastSeen = (lastSeen?: string | null): string => {
  if (!lastSeen) return 'No activity yet';
  const ts = Date.parse(lastSeen);
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

export const connectionLastActivityText = (connection: Connection): string =>
  lastActivityTextFromLastSeen(connection.lastSeen);

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

const SURFACE_LABELS: Record<string, string> = {
  vms: 'VMs',
  containers: 'Containers',
  storage: 'Storage',
  backups: 'Backups',
  datastores: 'Datastores',
  syncJobs: 'Sync jobs',
  verifyJobs: 'Verify jobs',
  pruneJobs: 'Prune jobs',
  garbageJobs: 'GC jobs',
  mailStats: 'Mail stats',
  queues: 'Queues',
  quarantine: 'Quarantine',
  domainStats: 'Domain stats',
  host: 'Host telemetry',
  hosts: 'Hosts',
  datasets: 'Datasets',
  pools: 'Pools',
  replication: 'Replication',
};

export const surfaceLabel = (key: string): string => SURFACE_LABELS[key] ?? key;

export type InfrastructureSourceKind = 'api' | 'agent' | 'both' | 'unknown';

export interface InfrastructureSourcePresentation {
  label: string;
  badgeClassName: string;
  title: string;
}

const SOURCE_PRESENTATION: Record<InfrastructureSourceKind, InfrastructureSourcePresentation> = {
  api: {
    label: 'API',
    badgeClassName:
      'inline-flex items-center rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-700 dark:bg-slate-800/60 dark:text-slate-200',
    title: 'Data collected via the platform API',
  },
  agent: {
    label: 'Agent',
    badgeClassName:
      'inline-flex items-center rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-medium text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200',
    title: 'Data collected via Pulse Agent',
  },
  both: {
    label: 'API + Agent',
    badgeClassName:
      'inline-flex items-center rounded-full bg-indigo-100 px-2 py-0.5 text-[11px] font-medium text-indigo-800 dark:bg-indigo-950/40 dark:text-indigo-200',
    title: 'Data collected via the platform API and Pulse Agent',
  },
  unknown: {
    label: '—',
    badgeClassName: 'text-[12px] text-muted',
    title: 'No source attached',
  },
};

export const infrastructureSourcePresentation = (
  source: InfrastructureSourceKind,
): InfrastructureSourcePresentation => SOURCE_PRESENTATION[source];

export interface InfrastructureSystemMemberRow {
  id: string;
  name: string;
  subtitle: string;
  source: InfrastructureSourceKind;
  host?: string;
  hostAliases?: string[];
  coverageLabels: string[];
  statusLabel: string;
  statusClassName: string;
  lastActivityText: string;
  primary: boolean;
  agentConnection?: Connection;
}

export interface InfrastructureSystemRow {
  id: string;
  ownerType: ConnectionType;
  name: string;
  subtitle?: string;
  source: InfrastructureSourceKind;
  host?: string;
  coverageLabels: string[];
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
  isCluster: boolean;
  attachedConnections: Connection[];
  members: InfrastructureSystemMemberRow[];
  connection: Connection;
}
