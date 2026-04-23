import { createEffect, createMemo, createResource, onCleanup } from 'solid-js';
import {
  ConnectionsAPI,
  type Connection,
  type ConnectionState,
  type ConnectionSystem,
  type ConnectionType,
} from '@/api/connections';
import { connectionLastActivityText, type InfrastructureSystemRow } from './connectionsTableModel';

const POLL_INTERVAL_MS = 15000;

export const CONNECTION_TYPE_LABELS: Record<ConnectionType, string> = {
  pve: 'Proxmox VE',
  pbs: 'Proxmox Backup Server',
  pmg: 'Proxmox Mail Gateway',
  vmware: 'VMware vCenter',
  truenas: 'TrueNAS SCALE',
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
  host: 'Host telemetry',
  hosts: 'Hosts',
  datasets: 'Datasets',
  pools: 'Pools',
  replication: 'Replication',
};

export const surfaceLabel = (key: string): string => SURFACE_LABELS[key] ?? key;

const PLATFORM_API_TYPES: ReadonlySet<ConnectionType> = new Set([
  'pve',
  'pbs',
  'pmg',
  'vmware',
  'truenas',
]);

const EDITABLE_CONNECTION_TYPES: readonly ConnectionType[] = [
  'pve',
  'pbs',
  'pmg',
  'vmware',
  'truenas',
];

const coverageLabelsFor = (connections: readonly Connection[]): string[] => {
  const seen = new Set<string>();
  for (const connection of connections) {
    const activeScopeKeys = Object.keys(connection.scope ?? {}).filter(
      (key) => connection.scope?.[key],
    );
    const coverage =
      activeScopeKeys.length > 0
        ? activeScopeKeys.map(surfaceLabel)
        : connection.surfaces.map(surfaceLabel);
    for (const label of coverage) {
      seen.add(label);
    }
  }
  return [...seen];
};

const subtitleFor = (connections: readonly Connection[], primaryConnection: Connection): string => {
  const hasPlatformAPI = connections.some((connection) => PLATFORM_API_TYPES.has(connection.type));
  const hasPulseAgent = connections.some((connection) => connection.type === 'agent');

  if (hasPlatformAPI && hasPulseAgent) {
    return 'via platform API and Pulse Agent';
  }

  if (hasPlatformAPI) {
    return 'via platform API';
  }

  if (hasPulseAgent) {
    return 'via Pulse Agent';
  }

  const productLabel = CONNECTION_TYPE_LABELS[primaryConnection.type] ?? primaryConnection.type;
  return `via ${productLabel}`;
};

const agentUpdateCountFor = (connections: readonly Connection[]): number =>
  connections.filter(
    (connection) => connection.type === 'agent' && Boolean(connection.agentUpdateAvailable),
  ).length;

const buildRow = (
  ownerType: ConnectionType,
  primaryConnection: Connection,
  componentConnections: readonly Connection[],
): InfrastructureSystemRow => {
  const presentation = STATE_PRESENTATION[primaryConnection.state] ?? STATE_PRESENTATION.pending;
  const name = primaryConnection.name || primaryConnection.address || primaryConnection.id;
  const host =
    primaryConnection.address && primaryConnection.address !== name
      ? primaryConnection.address
      : undefined;
  const attachedConnections = componentConnections.filter(
    (connection) => connection.id !== primaryConnection.id,
  );
  const lastErrorMessage =
    primaryConnection.lastError?.message ??
    attachedConnections.find((connection) => connection.lastError?.message)?.lastError?.message;

  return {
    id: primaryConnection.id,
    ownerType,
    name,
    subtitle: subtitleFor(componentConnections, primaryConnection),
    host,
    coverageLabels: coverageLabelsFor(componentConnections),
    statusLabel: presentation.label,
    statusClassName: presentation.badgeClass,
    agentUpdateCount: agentUpdateCountFor(componentConnections),
    lastActivityText: connectionLastActivityText(primaryConnection),
    lastErrorMessage,
    enabled: primaryConnection.enabled,
    canEdit: EDITABLE_CONNECTION_TYPES.includes(primaryConnection.type),
    canPause: primaryConnection.capabilities.supportsPause,
    canRemove: primaryConnection.type !== 'docker' && primaryConnection.type !== 'kubernetes',
    isAgent: primaryConnection.type === 'agent',
    attachedConnections,
    connection: primaryConnection,
  };
};

export const connectionToRow = (connection: Connection): InfrastructureSystemRow =>
  buildRow(connection.type, connection, [connection]);

const systemToRow = (
  system: ConnectionSystem,
  connectionsByID: Map<string, Connection>,
): InfrastructureSystemRow | null => {
  const primaryConnection = connectionsByID.get(system.id);
  if (!primaryConnection) return null;

  const componentConnections = system.components
    .map((component) => connectionsByID.get(component.connectionId))
    .filter((connection): connection is Connection => Boolean(connection));

  if (componentConnections.length === 0) {
    componentConnections.push(primaryConnection);
  }

  return buildRow(system.type, primaryConnection, componentConnections);
};

export interface ConnectionsLedger {
  connections: () => Connection[];
  rows: () => InfrastructureSystemRow[];
  findById: (id: string) => Connection | undefined;
  reload: () => void;
  loading: () => boolean;
  error: () => unknown;
}

interface ConnectionsLedgerSnapshot {
  connections: Connection[];
  systems: ConnectionSystem[];
}

export const useConnectionsLedger = (): ConnectionsLedger => {
  const [resource, { refetch }] = createResource<ConnectionsLedgerSnapshot>(
    async () => {
      const response = await ConnectionsAPI.list();
      return {
        connections: response.connections ?? [],
        systems: response.systems ?? [],
      };
    },
    {
      initialValue: {
        connections: [],
        systems: [],
      },
    },
  );

  createEffect(() => {
    const handle = window.setInterval(() => {
      void refetch();
    }, POLL_INTERVAL_MS);
    onCleanup(() => window.clearInterval(handle));
  });

  const snapshot = () => resource() ?? { connections: [], systems: [] };
  const connections = () => snapshot().connections ?? [];
  const rows = createMemo<InfrastructureSystemRow[]>(() => {
    const allConnections = connections();
    const systems = snapshot().systems ?? [];
    if (systems.length === 0) {
      return allConnections.map(connectionToRow);
    }

    const byID = new Map(allConnections.map((connection) => [connection.id, connection]));
    const groupedRows = systems
      .map((system) => systemToRow(system, byID))
      .filter((row): row is InfrastructureSystemRow => Boolean(row));

    if (groupedRows.length === 0) {
      return allConnections.map(connectionToRow);
    }

    return groupedRows;
  });
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
