import { createEffect, createMemo, createResource, onCleanup } from 'solid-js';
import {
  ConnectionsAPI,
  type Connection,
  type ConnectionState,
  type ConnectionSystemMember,
  type ConnectionSystem,
  type ConnectionType,
} from '@/api/connections';
import {
  connectionAgentEndpointDisplay,
  connectionAgentIdentitySummary,
  connectionLastActivityText,
  fleetGovernanceSignalsForConnection,
  lastActivityTextFromLastSeen,
  surfaceLabel,
  type InfrastructureSourceKind,
  type InfrastructureSystemMemberRow,
  type InfrastructureSystemRow,
  visibleFleetGovernanceSignals,
} from './connectionsTableModel';

export { surfaceLabel };

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

const CONNECTION_STATE_SEVERITY: Record<ConnectionState, number> = {
  active: 0,
  paused: 1,
  pending: 2,
  stale: 3,
  unauthorized: 4,
  unreachable: 5,
};

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

const sourceFor = (connections: readonly Connection[]): InfrastructureSourceKind => {
  const hasPlatformAPI = connections.some((connection) => PLATFORM_API_TYPES.has(connection.type));
  const hasPulseAgent = connections.some((connection) => connection.type === 'agent');
  if (hasPlatformAPI && hasPulseAgent) return 'both';
  if (hasPlatformAPI) return 'api';
  if (hasPulseAgent) return 'agent';
  return 'unknown';
};

const agentUpdateCountFor = (connections: readonly Connection[]): number =>
  connections.filter(
    (connection) => connection.type === 'agent' && Boolean(connection.agentUpdateAvailable),
  ).length;

const moreSevereState = (
  left: ConnectionState,
  right?: ConnectionState | null,
): ConnectionState => {
  if (!right) return left;
  return CONNECTION_STATE_SEVERITY[right] > CONNECTION_STATE_SEVERITY[left] ? right : left;
};

const oldestTimestamp = (values: ReadonlyArray<string | null | undefined>): string | undefined => {
  let oldestMs: number | undefined;
  let oldestRaw: string | undefined;
  for (const value of values) {
    if (!value) continue;
    const ms = Date.parse(value);
    if (Number.isNaN(ms)) continue;
    if (oldestMs === undefined || ms < oldestMs) {
      oldestMs = ms;
      oldestRaw = value;
    }
  }
  return oldestRaw;
};

interface ClusterRollup {
  state: ConnectionState;
  lastSeen?: string;
}

const memberSubtitleFor = (member: ConnectionSystemMember): string =>
  member.primary ? 'API contact' : 'Cluster member';

const buildMemberRow = (
  member: ConnectionSystemMember,
  connectionsByID: Map<string, Connection>,
): InfrastructureSystemMemberRow | null => {
  const name = member.name?.trim();
  if (!name) return null;

  const agentConnection = member.agentConnectionId
    ? connectionsByID.get(member.agentConnectionId)
    : undefined;
  const source: InfrastructureSourceKind = agentConnection ? 'agent' : 'unknown';
  const state = moreSevereState(member.state, agentConnection?.state);
  const presentation = STATE_PRESENTATION[state] ?? STATE_PRESENTATION.pending;
  const lastSeen =
    agentConnection && state === agentConnection.state
      ? agentConnection.lastSeen
      : (member.lastSeen ?? agentConnection?.lastSeen);
  const fleetSignals = agentConnection ? fleetGovernanceSignalsForConnection(agentConnection) : [];

  return {
    id: member.id,
    name,
    subtitle: memberSubtitleFor(member),
    source,
    host: member.endpoint?.trim() || undefined,
    hostAliases: member.hostAliases?.filter((alias) => alias.trim().length > 0) ?? [],
    coverageLabels: agentConnection ? coverageLabelsFor([agentConnection]) : [],
    statusLabel: presentation.label,
    statusClassName: presentation.badgeClass,
    lastActivityText: lastActivityTextFromLastSeen(lastSeen),
    fleetSignals,
    fleetHighlights: visibleFleetGovernanceSignals(fleetSignals),
    primary: Boolean(member.primary),
    agentConnection,
  };
};

const buildRow = (
  ownerType: ConnectionType,
  primaryConnection: Connection,
  componentConnections: readonly Connection[],
  system?: ConnectionSystem | null,
  members: InfrastructureSystemMemberRow[] = [],
  rollup?: ClusterRollup,
): InfrastructureSystemRow => {
  const rolledState = rollup?.state ?? primaryConnection.state;
  const presentation = STATE_PRESENTATION[rolledState] ?? STATE_PRESENTATION.pending;
  const clusterName = system?.clusterName?.trim();
  const isCluster = ownerType === 'pve' && Boolean(clusterName) && members.length > 0;
  const isStandaloneAgent = !isCluster && primaryConnection.type === 'agent';
  const name =
    ownerType === 'pve' && clusterName
      ? clusterName
      : primaryConnection.name || primaryConnection.address || primaryConnection.id;
  const host = isCluster
    ? undefined
    : isStandaloneAgent
      ? (connectionAgentEndpointDisplay(primaryConnection) ?? undefined)
      : primaryConnection.address && primaryConnection.address !== name
        ? primaryConnection.address
        : undefined;
  const attachedConnections = componentConnections.filter(
    (connection) => connection.id !== primaryConnection.id,
  );
  const lastErrorMessage =
    primaryConnection.lastError?.message ??
    attachedConnections.find((connection) => connection.lastError?.message)?.lastError?.message;
  const hostTelemetryLabel = surfaceLabel('host');
  const coverageLabels = isCluster
    ? coverageLabelsFor(componentConnections).filter((label) => label !== hostTelemetryLabel)
    : coverageLabelsFor(componentConnections);
  const source: InfrastructureSourceKind = isCluster ? 'api' : sourceFor(componentConnections);
  const subtitle = isCluster
    ? `Cluster · ${members.length} ${members.length === 1 ? 'node' : 'nodes'}`
    : subtitleFor(componentConnections, primaryConnection);
  const identitySubtitle = isStandaloneAgent
    ? (connectionAgentIdentitySummary(primaryConnection) ?? undefined)
    : undefined;
  const fleetSignals = componentConnections.flatMap((connection) =>
    fleetGovernanceSignalsForConnection(connection),
  );

  return {
    id: primaryConnection.id,
    ownerType,
    name,
    subtitle,
    identitySubtitle,
    source,
    host,
    coverageLabels,
    statusLabel: presentation.label,
    statusClassName: presentation.badgeClass,
    agentUpdateCount: agentUpdateCountFor(componentConnections),
    lastActivityText: rollup
      ? lastActivityTextFromLastSeen(rollup.lastSeen)
      : connectionLastActivityText(primaryConnection),
    lastErrorMessage,
    fleetSignals,
    fleetHighlights: visibleFleetGovernanceSignals(fleetSignals),
    enabled: primaryConnection.enabled,
    canEdit: EDITABLE_CONNECTION_TYPES.includes(primaryConnection.type),
    canPause: primaryConnection.capabilities.supportsPause,
    canRemove: primaryConnection.type !== 'docker' && primaryConnection.type !== 'kubernetes',
    isAgent: primaryConnection.type === 'agent',
    isCluster,
    attachedConnections,
    members,
    connection: primaryConnection,
  };
};

export const connectionToRow = (connection: Connection): InfrastructureSystemRow =>
  buildRow(connection.type, connection, [connection], null, []);

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

  const rawMembers = system.members ?? [];
  const members = rawMembers
    .map((member) => buildMemberRow(member, connectionsByID))
    .filter((member): member is InfrastructureSystemMemberRow => Boolean(member));

  const isCluster =
    system.type === 'pve' && Boolean(system.clusterName?.trim()) && members.length > 0;
  let rollup: ClusterRollup | undefined;
  if (isCluster) {
    const memberAgents = rawMembers
      .map((member) =>
        member.agentConnectionId ? connectionsByID.get(member.agentConnectionId) : undefined,
      )
      .filter((connection): connection is Connection => Boolean(connection));
    const states: ConnectionState[] = [
      primaryConnection.state,
      ...rawMembers.map((member) => member.state),
      ...memberAgents.map((connection) => connection.state),
    ];
    const worstState = states.reduce<ConnectionState>(
      (acc, state) => moreSevereState(acc, state),
      'active',
    );
    const lastSeens: Array<string | null | undefined> = [
      primaryConnection.lastSeen,
      ...rawMembers.map((member) => member.lastSeen),
      ...memberAgents.map((connection) => connection.lastSeen),
    ];
    rollup = { state: worstState, lastSeen: oldestTimestamp(lastSeens) };
  }

  return buildRow(system.type, primaryConnection, componentConnections, system, members, rollup);
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
