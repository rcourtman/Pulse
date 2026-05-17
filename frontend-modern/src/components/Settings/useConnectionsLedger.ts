import { createMemo } from 'solid-js';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { formatConnectionErrorMessage } from '@/utils/connectionErrorPresentation';
import {
  ConnectionsAPI,
  type Connection,
  type ConnectionState,
  type ConnectionSystemMember,
  type ConnectionSystem,
  type ConnectionType,
} from '@/api/connections';
import {
  agentAttachmentSignal,
  connectionAgentEndpointDisplay,
  connectionAgentIdentitySummary,
  connectionLastActivityText,
  fleetGovernanceSignalsForConnection,
  lastActivityTextFromLastSeen,
  surfaceLabel,
  type FleetGovernanceSignal,
  type InfrastructureSourceKind,
  type InfrastructureSystemMemberRow,
  type InfrastructureSystemRow,
  visibleFleetGovernanceSignals,
} from './connectionsTableModel';

export { surfaceLabel };

const POLL_INTERVAL_MS = 15000;
const CONNECTIONS_LEDGER_QUERY_KEY = 'settings-infrastructure-connections';

export const CONNECTION_TYPE_LABELS: Record<ConnectionType, string> = {
  pve: 'Proxmox VE',
  pbs: 'Proxmox Backup Server',
  pmg: 'Proxmox Mail Gateway',
  vmware: 'VMware vCenter',
  truenas: 'TrueNAS SCALE',
  availability: 'Network Endpoint',
  agent: 'Pulse Agent',
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
  'availability',
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
  const hasAvailabilityProbe = connections.some((connection) => connection.type === 'availability');

  if (hasPlatformAPI && hasPulseAgent) {
    return 'via platform API and Pulse Agent';
  }

  if (hasPlatformAPI) {
    return 'via platform API';
  }

  if (hasPulseAgent) {
    return 'via Pulse Agent';
  }
  if (hasAvailabilityProbe) {
    return 'via availability probe';
  }

  const productLabel = CONNECTION_TYPE_LABELS[primaryConnection.type] ?? primaryConnection.type;
  return `via ${productLabel}`;
};

// A connection is "contributing" if it is currently delivering data — not
// merely configured. Otherwise the source badge claims "API + Agent" when
// the agent has been silent for hours, which directly contradicts the
// "Agent offline" chip on the same row.
const isContributing = (connection: Connection): boolean =>
  connection.state === 'active' || connection.state === 'paused';

const sourceFor = (connections: readonly Connection[]): InfrastructureSourceKind => {
  const contributors = connections.filter(isContributing);
  const hasPlatformAPI = contributors.some((connection) => PLATFORM_API_TYPES.has(connection.type));
  const hasPulseAgent = contributors.some((connection) => connection.type === 'agent');
  const hasAvailabilityProbe = contributors.some(
    (connection) => connection.type === 'availability',
  );
  if (hasPlatformAPI && hasPulseAgent) return 'both';
  if (hasPlatformAPI) return 'api';
  if (hasPulseAgent) return 'agent';
  if (hasAvailabilityProbe) return 'probe';
  // Fall back to configured presence when no path is currently active, so
  // the row still names what was set up even if everything is offline.
  if (connections.some((connection) => PLATFORM_API_TYPES.has(connection.type))) return 'api';
  if (connections.some((connection) => connection.type === 'agent')) return 'agent';
  if (connections.some((connection) => connection.type === 'availability')) return 'probe';
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

interface CachedInfrastructureSystemRow {
  signature: string;
  row: InfrastructureSystemRow;
}

const stableRecordEntries = (record?: Record<string, boolean> | null): [string, boolean][] =>
  Object.entries(record ?? {}).sort(([left], [right]) => left.localeCompare(right));

const connectionRowSignature = (connection: Connection): string =>
  JSON.stringify({
    id: connection.id,
    type: connection.type,
    name: connection.name,
    address: connection.address,
    state: connection.state,
    stateReason: connection.stateReason,
    enabled: connection.enabled,
    surfaces: connection.surfaces,
    scope: stableRecordEntries(connection.scope),
    lastSeen: connection.lastSeen,
    lastActivityText: connectionLastActivityText(connection),
    lastError: connection.lastError,
    source: connection.source,
    capabilities: connection.capabilities,
    agentVersion: connection.agentVersion,
    expectedAgentVersion: connection.expectedAgentVersion,
    agentUpdateAvailable: connection.agentUpdateAvailable,
    agentIdentity: connection.agentIdentity,
    hostAliases: connection.hostAliases,
    fleet: connection.fleet,
  });

const connectionRowSignatureForID = (
  connectionsByID: Map<string, Connection>,
  id?: string | null,
): string | null => {
  if (!id) return null;
  const connection = connectionsByID.get(id);
  return connection ? connectionRowSignature(connection) : null;
};

const systemRowSignature = (
  system: ConnectionSystem,
  connectionsByID: Map<string, Connection>,
): string =>
  JSON.stringify({
    id: system.id,
    type: system.type,
    clusterName: system.clusterName,
    components: system.components,
    members: (system.members ?? []).map((member) => ({
      id: member.id,
      name: member.name,
      endpoint: member.endpoint,
      hostAliases: member.hostAliases,
      state: member.state,
      lastSeen: member.lastSeen,
      lastActivityText: lastActivityTextFromLastSeen(member.lastSeen),
      primary: member.primary,
      agentConnectionId: member.agentConnectionId,
      agentConnectionSignature: connectionRowSignatureForID(
        connectionsByID,
        member.agentConnectionId,
      ),
    })),
    componentConnections: system.components.map((component) => ({
      connectionId: component.connectionId,
      signature: connectionRowSignatureForID(connectionsByID, component.connectionId),
    })),
    primaryConnectionSignature: connectionRowSignatureForID(connectionsByID, system.id),
  });

const memberSubtitleFor = (member: ConnectionSystemMember): string =>
  member.primary ? 'Primary node' : 'Cluster member';

const buildMemberRow = (
  member: ConnectionSystemMember,
  connectionsByID: Map<string, Connection>,
): InfrastructureSystemMemberRow | null => {
  const name = member.name?.trim();
  if (!name) return null;

  const agentConnection = member.agentConnectionId
    ? connectionsByID.get(member.agentConnectionId)
    : undefined;
  // Cluster members always have an implicit API path via the cluster's
  // primary connection. When a paired agent is also contributing, both
  // sources apply; otherwise the row is API-only.
  const source: InfrastructureSourceKind =
    agentConnection && isContributing(agentConnection) ? 'both' : 'api';
  // Member status reflects the Proxmox-side view of the node. An optional
  // paired agent being silent is reported as a separate "Agent offline" chip
  // so the headline state doesn't make a working node look down.
  const state = member.state;
  const presentation = STATE_PRESENTATION[state] ?? STATE_PRESENTATION.pending;
  const lastSeen = member.lastSeen ?? agentConnection?.lastSeen;
  const attachmentSignal = agentConnection ? agentAttachmentSignal(agentConnection) : null;
  const fleetSignals: FleetGovernanceSignal[] = [
    ...(attachmentSignal ? [attachmentSignal] : []),
    ...(agentConnection ? fleetGovernanceSignalsForConnection(agentConnection) : []),
  ];

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
  // Run raw backend error chains through the user-facing presenter so the
  // connection table doesn't surface implementation strings like 'context
  // deadline exceeded' or 'Client.Timeout exceeded while awaiting headers'.
  const rawLastErrorMessage =
    primaryConnection.lastError?.message ??
    attachedConnections.find((connection) => connection.lastError?.message)?.lastError?.message;
  const lastErrorMessage = formatConnectionErrorMessage(rawLastErrorMessage) ?? undefined;
  const hostTelemetryLabel = surfaceLabel('host');
  const coverageLabels = isCluster
    ? coverageLabelsFor(componentConnections).filter((label) => label !== hostTelemetryLabel)
    : coverageLabelsFor(componentConnections);
  const source: InfrastructureSourceKind = sourceFor(componentConnections);
  const subtitle = isCluster
    ? `Cluster · ${members.length} ${members.length === 1 ? 'node' : 'nodes'}`
    : subtitleFor(componentConnections, primaryConnection);
  const identitySubtitle = isStandaloneAgent
    ? (connectionAgentIdentitySummary(primaryConnection) ?? undefined)
    : undefined;
  const memberAgentConnectionIds = new Set(
    members
      .map((member) => member.agentConnection?.id)
      .filter((id): id is string => typeof id === 'string' && id.length > 0),
  );
  const rowFleetSignalConnections = isCluster
    ? componentConnections.filter(
        (connection) =>
          connection.id === primaryConnection.id || !memberAgentConnectionIds.has(connection.id),
      )
    : componentConnections;
  // For non-agent primaries (PVE, PBS, etc.), surface a clearly-named chip
  // when an attached agent is not reporting. We avoid double-injecting the
  // chip for cluster member agents — those are tracked on the member rows.
  const attachmentSignals: FleetGovernanceSignal[] =
    primaryConnection.type === 'agent'
      ? []
      : rowFleetSignalConnections
          .filter(
            (connection) =>
              connection.id !== primaryConnection.id &&
              connection.type === 'agent' &&
              !memberAgentConnectionIds.has(connection.id),
          )
          .map((connection) => agentAttachmentSignal(connection))
          .filter((signal): signal is FleetGovernanceSignal => Boolean(signal));
  const fleetSignals: FleetGovernanceSignal[] = [
    ...attachmentSignals,
    ...rowFleetSignalConnections.flatMap((connection) =>
      fleetGovernanceSignalsForConnection(connection),
    ),
  ];

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
  // Clusters roll up across members so a single unreachable node surfaces on
  // the parent row. We deliberately exclude attached *agents* here: a stale
  // agent means optional host telemetry is missing, not that the system has
  // stopped reporting. That gets its own named chip below.
  let rollup: ClusterRollup | undefined;
  if (isCluster) {
    const states: ConnectionState[] = [
      primaryConnection.state,
      ...rawMembers.map((member) => member.state),
    ];
    const worstState = states.reduce<ConnectionState>(
      (acc, state) => moreSevereState(acc, state),
      'active',
    );
    const lastSeens: Array<string | null | undefined> = [
      primaryConnection.lastSeen,
      ...rawMembers.map((member) => member.lastSeen),
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

const EMPTY_CONNECTIONS_LEDGER_SNAPSHOT: ConnectionsLedgerSnapshot = {
  connections: [],
  systems: [],
};

export const useConnectionsLedger = (): ConnectionsLedger => {
  const rowCache = new Map<string, CachedInfrastructureSystemRow>();

  const cacheRow = (
    activeKeys: Set<string>,
    key: string,
    signature: string,
    buildRow: () => InfrastructureSystemRow | null,
  ): InfrastructureSystemRow | null => {
    activeKeys.add(key);
    const cached = rowCache.get(key);
    if (cached?.signature === signature) {
      return cached.row;
    }
    const row = buildRow();
    if (!row) {
      rowCache.delete(key);
      return null;
    }
    rowCache.set(key, { signature, row });
    return row;
  };

  const pruneRowCache = (activeKeys: Set<string>) => {
    for (const key of rowCache.keys()) {
      if (!activeKeys.has(key)) {
        rowCache.delete(key);
      }
    }
  };

  const ledgerSnapshot = createNonSuspendingQuery<ConnectionsLedgerSnapshot, string>({
    source: () => CONNECTIONS_LEDGER_QUERY_KEY,
    fetcher: async () => {
      const response = await ConnectionsAPI.list();
      return {
        connections: response.connections ?? [],
        systems: response.systems ?? [],
      };
    },
    initialValue: EMPTY_CONNECTIONS_LEDGER_SNAPSHOT,
    cacheKey: (key) => key,
    pollMs: POLL_INTERVAL_MS,
  });

  const snapshot = () => ledgerSnapshot.value() ?? EMPTY_CONNECTIONS_LEDGER_SNAPSHOT;
  const connections = () => snapshot().connections ?? [];
  const rows = createMemo<InfrastructureSystemRow[]>(() => {
    const allConnections = connections();
    const systems = snapshot().systems ?? [];
    const activeCacheKeys = new Set<string>();
    const standaloneRows = () =>
      allConnections
        .map((connection) =>
          cacheRow(
            activeCacheKeys,
            `connection:${connection.id}`,
            connectionRowSignature(connection),
            () => connectionToRow(connection),
          ),
        )
        .filter((row): row is InfrastructureSystemRow => Boolean(row));

    if (systems.length === 0) {
      const rows = standaloneRows();
      pruneRowCache(activeCacheKeys);
      return rows;
    }

    const byID = new Map(allConnections.map((connection) => [connection.id, connection]));
    // A loose Pulse Agent that runs on a host that is already a cluster
    // member duplicates the same physical machine in two table rows (once
    // via Proxmox API, once via the agent). Collect every cluster-member
    // host name and alias so we can drop the redundant standalone row.
    const clusterMemberHosts = new Set<string>();
    for (const system of systems) {
      if (system.type !== 'pve' || !system.clusterName?.trim()) continue;
      for (const member of system.members ?? []) {
        const name = member.name?.trim().toLowerCase();
        if (name) clusterMemberHosts.add(name);
        for (const alias of member.hostAliases ?? []) {
          const value = alias?.trim().toLowerCase();
          if (value) clusterMemberHosts.add(value);
        }
      }
    }
    const groupedRows = systems
      .map((system) => {
        // Skip a standalone-agent system if its host is already represented
        // as a cluster member elsewhere on the page.
        if (system.type === 'agent' && (system.components?.length ?? 0) <= 1) {
          const primary = byID.get(system.id);
          const candidates = [
            primary?.name,
            primary?.address,
            primary?.agentIdentity?.hostname,
            ...(primary?.hostAliases ?? []),
          ];
          if (
            candidates.some(
              (value) => value && clusterMemberHosts.has(value.trim().toLowerCase()),
            )
          ) {
            return null;
          }
        }
        return cacheRow(
          activeCacheKeys,
          `system:${system.id}`,
          systemRowSignature(system, byID),
          () => systemToRow(system, byID),
        );
      })
      .filter((row): row is InfrastructureSystemRow => Boolean(row));

    if (groupedRows.length === 0) {
      const rows = standaloneRows();
      pruneRowCache(activeCacheKeys);
      return rows;
    }

    pruneRowCache(activeCacheKeys);
    return groupedRows;
  });
  const findById = (id: string) => connections().find((conn) => conn.id === id);

  return {
    connections,
    rows,
    findById,
    reload: () => {
      void ledgerSnapshot.refetch();
    },
    loading: ledgerSnapshot.loading,
    error: ledgerSnapshot.error,
  };
};
