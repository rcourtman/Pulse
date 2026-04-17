import type { Resource } from '@/types/resource';
import type { NodeConfigWithStatus } from '@/types/nodes';
import type { TrueNASConnection } from '@/api/truenas';
import type { VMwareConnection } from '@/api/vmware';

export type ConnectionKind = 'pve' | 'pbs' | 'pmg' | 'truenas' | 'vmware' | 'agent';
export type ConnectionMethod = 'api' | 'agent';
export type ConnectionStatus = 'reporting' | 'pending' | 'offline' | 'error' | 'unknown';

export interface ConnectionRow {
  id: string;
  kind: ConnectionKind;
  kindLabel: string;
  method: ConnectionMethod;
  methodLabel: string;
  name: string;
  host?: string;
  status: ConnectionStatus;
  statusLabel: string;
  lastReportedMs?: number;
}

export const CONNECTION_KIND_LABELS: Record<ConnectionKind, string> = {
  pve: 'Proxmox VE',
  pbs: 'PBS',
  pmg: 'PMG',
  truenas: 'TrueNAS',
  vmware: 'VMware',
  agent: 'Agent host',
};

export const CONNECTION_METHOD_LABELS: Record<ConnectionMethod, string> = {
  api: 'API',
  agent: 'Agent',
};

export const CONNECTION_STATUS_LABELS: Record<ConnectionStatus, string> = {
  reporting: 'Reporting',
  pending: 'Pending',
  offline: 'Offline',
  error: 'Error',
  unknown: 'Unknown',
};

function row(
  kind: ConnectionKind,
  id: string,
  name: string,
  method: ConnectionMethod,
  status: ConnectionStatus,
  host?: string,
  lastReportedMs?: number,
): ConnectionRow {
  return {
    id: `${kind}:${id}`,
    kind,
    kindLabel: CONNECTION_KIND_LABELS[kind],
    method,
    methodLabel: CONNECTION_METHOD_LABELS[method],
    name,
    host,
    status,
    statusLabel: CONNECTION_STATUS_LABELS[status],
    lastReportedMs,
  };
}

function mapNodeStatus(status: NodeConfigWithStatus['status']): ConnectionStatus {
  switch (status) {
    case 'connected':
      return 'reporting';
    case 'pending':
      return 'pending';
    case 'disconnected':
    case 'offline':
      return 'offline';
    case 'error':
      return 'error';
    default:
      return 'unknown';
  }
}

export function pveConnectionRow(node: NodeConfigWithStatus): ConnectionRow {
  return row('pve', node.id, node.displayName || node.name, 'api', mapNodeStatus(node.status), node.host);
}

export function pbsConnectionRow(node: NodeConfigWithStatus): ConnectionRow {
  return row('pbs', node.id, node.displayName || node.name, 'api', mapNodeStatus(node.status), node.host);
}

export function pmgConnectionRow(node: NodeConfigWithStatus): ConnectionRow {
  return row('pmg', node.id, node.displayName || node.name, 'api', mapNodeStatus(node.status), node.host);
}

function pollBackedStatus(
  enabled: boolean,
  lastSuccessAt: string | undefined,
  consecutiveFailures: number | undefined,
): { status: ConnectionStatus; lastReportedMs?: number } {
  if (!enabled) return { status: 'offline' };
  const lastReportedMs = lastSuccessAt ? Date.parse(lastSuccessAt) || undefined : undefined;
  const fails = consecutiveFailures ?? 0;
  if (fails > 2) return { status: 'error', lastReportedMs };
  if (fails > 0 && !lastReportedMs) return { status: 'error' };
  if (lastReportedMs) return { status: 'reporting', lastReportedMs };
  return { status: 'pending' };
}

export function truenasConnectionRow(conn: TrueNASConnection): ConnectionRow {
  const { status, lastReportedMs } = pollBackedStatus(
    conn.enabled,
    conn.poll?.lastSuccessAt,
    conn.poll?.consecutiveFailures,
  );
  return row('truenas', conn.id, conn.name || conn.host, 'api', status, conn.host, lastReportedMs);
}

export function vmwareConnectionRow(conn: VMwareConnection): ConnectionRow {
  const { status, lastReportedMs } = pollBackedStatus(
    conn.enabled,
    conn.poll?.lastSuccessAt,
    conn.poll?.consecutiveFailures,
  );
  return row('vmware', conn.id, conn.name || conn.host, 'api', status, conn.host, lastReportedMs);
}

function mapResourceStatus(status: Resource['status']): ConnectionStatus {
  switch (status) {
    case 'online':
    case 'running':
      return 'reporting';
    case 'offline':
    case 'stopped':
      return 'offline';
    case 'degraded':
      return 'error';
    default:
      return 'unknown';
  }
}

export function agentConnectionRow(resource: Resource): ConnectionRow {
  return row(
    'agent',
    resource.id,
    resource.displayName || resource.name,
    'agent',
    mapResourceStatus(resource.status),
    undefined,
    resource.lastSeen || undefined,
  );
}

export interface ConnectionsTableSources {
  pveNodes: readonly NodeConfigWithStatus[];
  pbsNodes: readonly NodeConfigWithStatus[];
  pmgNodes: readonly NodeConfigWithStatus[];
  truenasConnections: readonly TrueNASConnection[];
  vmwareConnections: readonly VMwareConnection[];
  agentResources: readonly Resource[];
}

export function buildConnectionRows(sources: ConnectionsTableSources): ConnectionRow[] {
  return [
    ...sources.pveNodes.map(pveConnectionRow),
    ...sources.pbsNodes.map(pbsConnectionRow),
    ...sources.pmgNodes.map(pmgConnectionRow),
    ...sources.truenasConnections.map(truenasConnectionRow),
    ...sources.vmwareConnections.map(vmwareConnectionRow),
    ...sources.agentResources.map(agentConnectionRow),
  ].sort((a, b) => a.name.localeCompare(b.name));
}
