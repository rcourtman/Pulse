import { createEffect, createMemo, createResource, onCleanup } from 'solid-js';
import {
  ConnectionsAPI,
  type Connection,
  type ConnectionState,
  type ConnectionType,
} from '@/api/connections';
import type { InfrastructureSystemRow } from './connectionsTableModel';

const POLL_INTERVAL_MS = 15000;

export const CONNECTION_TYPE_LABELS: Record<ConnectionType, string> = {
  pve: 'Proxmox VE',
  pbs: 'Proxmox Backup Server',
  pmg: 'Proxmox Mail Gateway',
  vmware: 'VMware vCenter',
  truenas: 'TrueNAS',
  agent: 'Pulse Unified Agent',
  docker: 'Docker',
  kubernetes: 'Kubernetes',
};

const STATE_PRESENTATION: Record<ConnectionState, { label: string; badgeClass: string }> = {
  active: {
    label: 'Active',
    badgeClass: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
  },
  paused: {
    label: 'Paused',
    badgeClass: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
  },
  unauthorized: {
    label: 'Unauthorized',
    badgeClass: 'bg-rose-100 text-rose-800 dark:bg-rose-900 dark:text-rose-200',
  },
  unreachable: {
    label: 'Unreachable',
    badgeClass: 'bg-rose-100 text-rose-800 dark:bg-rose-900 dark:text-rose-200',
  },
  stale: {
    label: 'Stale',
    badgeClass: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  },
  pending: {
    label: 'Pending',
    badgeClass: 'bg-surface-alt text-base-content',
  },
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
  host: 'Host',
  hosts: 'Hosts',
  datasets: 'Datasets',
  pools: 'Pools',
  replication: 'Replication',
};

export const surfaceLabel = (key: string): string => SURFACE_LABELS[key] ?? key;

// The ledger subtitle speaks the explainer's vocabulary so the user reads the
// same "Platform API vs Pulse Unified Agent" split here that the explainer
// card above just taught them. API-backed types get a "Platform API · {product}"
// prefix; agent/docker/k8s rows use the product label alone (the product name
// already carries the source — "Pulse Unified Agent" / "Docker" / "Kubernetes").
const PLATFORM_API_TYPES: ReadonlySet<ConnectionType> = new Set([
  'pve',
  'pbs',
  'pmg',
  'vmware',
  'truenas',
]);

const lastActivityText = (connection: Connection): string => {
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

const subtitleFor = (connection: Connection): string | undefined => {
  if (connection.stateReason) return connection.stateReason;
  const productLabel = CONNECTION_TYPE_LABELS[connection.type] ?? connection.type;
  return PLATFORM_API_TYPES.has(connection.type)
    ? `Platform API · ${productLabel}`
    : productLabel;
};

const EDITABLE_CONNECTION_TYPES: readonly ConnectionType[] = [
  'pve',
  'pbs',
  'pmg',
  'vmware',
  'truenas',
];

export const connectionToRow = (connection: Connection): InfrastructureSystemRow => {
  const presentation = STATE_PRESENTATION[connection.state] ?? STATE_PRESENTATION.pending;
  const activeScopeKeys = Object.keys(connection.scope ?? {}).filter(
    (key) => connection.scope?.[key],
  );
  const coverage =
    activeScopeKeys.length > 0
      ? activeScopeKeys.map(surfaceLabel)
      : connection.surfaces.map(surfaceLabel);
  const name = connection.name || connection.address || connection.id;
  const host =
    connection.address && connection.address !== name ? connection.address : undefined;
  return {
    id: connection.id,
    name,
    subtitle: subtitleFor(connection),
    host,
    coverageLabels: coverage,
    statusLabel: presentation.label,
    statusClassName: presentation.badgeClass,
    lastActivityText: lastActivityText(connection),
    lastErrorMessage: connection.lastError?.message,
    enabled: connection.enabled,
    canEdit: EDITABLE_CONNECTION_TYPES.includes(connection.type),
    canPause: connection.capabilities.supportsPause,
    canRemove: connection.type !== 'docker' && connection.type !== 'kubernetes',
    isAgent: connection.type === 'agent',
    connection,
  };
};

export interface ConnectionsLedger {
  connections: () => Connection[];
  rows: () => InfrastructureSystemRow[];
  findById: (id: string) => Connection | undefined;
  reload: () => void;
  loading: () => boolean;
  error: () => unknown;
}

export const useConnectionsLedger = (): ConnectionsLedger => {
  const [resource, { refetch }] = createResource<Connection[]>(() => ConnectionsAPI.list(), {
    initialValue: [],
  });

  createEffect(() => {
    const handle = window.setInterval(() => {
      void refetch();
    }, POLL_INTERVAL_MS);
    onCleanup(() => window.clearInterval(handle));
  });

  const connections = () => resource() ?? [];
  const rows = createMemo<InfrastructureSystemRow[]>(() => connections().map(connectionToRow));
  const findById = (id: string) => connections().find((conn) => conn.id === id);

  return {
    connections,
    rows,
    findById,
    reload: () => {
      void refetch();
    },
    loading: () => resource.loading,
    error: () => resource.error,
  };
};
