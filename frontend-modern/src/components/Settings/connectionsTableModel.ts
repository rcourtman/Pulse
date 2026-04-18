import type { TrueNASConnection } from '@/api/truenas';
import type { VMwareConnection } from '@/api/vmware';
import type { NodeConfigWithStatus } from '@/types/nodes';
import { formatRelativeTime } from '@/utils/format';
import { getUnifiedAgentLastSeenLabel } from '@/utils/unifiedAgentInventoryPresentation';
import {
  getUnifiedAgentStatusPresentation,
  MONITORING_STOPPED_STATUS_LABEL,
} from '@/utils/unifiedAgentStatusPresentation';
import type { UnifiedAgentRow } from './infrastructureOperationsModel';

type ManagedNodeKind = 'pve' | 'pbs' | 'pmg';

export type ConnectionManageAction =
  | { kind: 'inventory-active'; rowKey: string }
  | { kind: 'inventory-ignored'; rowKey: string }
  | { kind: 'proxmox-node'; nodeKind: ManagedNodeKind; nodeId: string }
  | { kind: 'truenas-connection'; connectionId: string }
  | { kind: 'vmware-connection'; connectionId: string };

export interface ConnectionRow {
  id: string;
  name: string;
  subtitle: string;
  host?: string;
  coverageLabels: string[];
  collectionLabel: string;
  statusLabel: string;
  statusClassName: string;
  lastActivityText: string;
  manageLabel: string;
  manage: ConnectionManageAction;
}

const SUCCESS_BADGE_CLASS =
  'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300';
const WARNING_BADGE_CLASS =
  'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200';
const DANGER_BADGE_CLASS = 'bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-300';
const MUTED_BADGE_CLASS = 'bg-surface text-muted';
const DEFAULT_BADGE_CLASS = 'bg-surface-alt text-base-content';

const PROXMOX_CAPABILITY_KEYS = ['proxmox', 'pbs', 'pmg'] as const;
const AGENT_CAPABILITY_KEYS = ['agent', 'docker', 'kubernetes'] as const;

const formatActivity = (value?: number | string | null) =>
  value ? formatRelativeTime(value, { emptyText: '—' }) : '—';

const normalizeConfigKey = (value?: string | null) => value?.trim().toLowerCase() ?? '';

const buildConfiguredRow = (params: {
  id: string;
  name: string;
  host?: string;
  subtitle: string;
  coverageLabels: string[];
  collectionLabel: string;
  statusLabel: string;
  statusClassName: string;
  lastActivityText: string;
  manageLabel: string;
  manage: ConnectionManageAction;
}) => ({
  id: params.id,
  name: params.name,
  subtitle: params.subtitle,
  host: params.host && params.host !== params.name ? params.host : undefined,
  coverageLabels: params.coverageLabels,
  collectionLabel: params.collectionLabel,
  statusLabel: params.statusLabel,
  statusClassName: params.statusClassName,
  lastActivityText: params.lastActivityText,
  manageLabel: params.manageLabel,
  manage: params.manage,
});

const collectionLabelFromCapabilities = (capabilities: UnifiedAgentRow['capabilities']) => {
  const hasAgent = capabilities.some((capability) =>
    AGENT_CAPABILITY_KEYS.includes(capability as (typeof AGENT_CAPABILITY_KEYS)[number]),
  );
  const hasApi = capabilities.some((capability) =>
    PROXMOX_CAPABILITY_KEYS.includes(capability as (typeof PROXMOX_CAPABILITY_KEYS)[number]) ||
    capability === 'truenas',
  );

  if (hasAgent && hasApi) {
    return 'Agent + API';
  }
  if (hasApi) {
    return 'API';
  }
  if (hasAgent) {
    return 'Agent';
  }
  return 'Runtime';
};

const reportingRow = (row: UnifiedAgentRow): ConnectionRow => {
  const statusPresentation = getUnifiedAgentStatusPresentation(row.status, row.healthStatus);

  return {
    id: row.rowKey,
    name: row.name,
    subtitle: row.status === 'removed' ? 'Ignored by Pulse' : 'Live reporting item',
    host:
      row.hostname && row.hostname !== row.name && row.hostname !== row.displayName
        ? row.hostname
        : undefined,
    coverageLabels: row.surfaces.map((surface) => surface.label),
    collectionLabel: collectionLabelFromCapabilities(row.capabilities),
    statusLabel: statusPresentation.label,
    statusClassName: statusPresentation.badgeClass,
    lastActivityText: getUnifiedAgentLastSeenLabel(row, MONITORING_STOPPED_STATUS_LABEL),
    manageLabel: row.status === 'removed' ? 'Review ignored' : 'View details',
    manage:
      row.status === 'removed'
        ? { kind: 'inventory-ignored', rowKey: row.rowKey }
        : { kind: 'inventory-active', rowKey: row.rowKey },
  };
};

const nodeStatusPresentation = (status: NodeConfigWithStatus['status']) => {
  switch (status) {
    case 'connected':
      return { label: 'Connected', className: SUCCESS_BADGE_CLASS };
    case 'pending':
      return { label: 'Pending', className: WARNING_BADGE_CLASS };
    case 'disconnected':
    case 'offline':
      return { label: 'Offline', className: MUTED_BADGE_CLASS };
    case 'error':
      return { label: 'Error', className: DANGER_BADGE_CLASS };
    default:
      return { label: 'Unknown', className: DEFAULT_BADGE_CLASS };
  }
};

const proxmoxNodeRow = (
  node: NodeConfigWithStatus,
  kindLabel: string,
  nodeKind: ManagedNodeKind,
): ConnectionRow => {
  const status = nodeStatusPresentation(node.status);

  return buildConfiguredRow({
    id: `${nodeKind}:${node.id}`,
    name: node.displayName || node.name,
    host: node.host,
    subtitle: 'Configured platform connection',
    coverageLabels: [`${kindLabel} data`],
    collectionLabel: 'API',
    statusLabel: status.label,
    statusClassName: status.className,
    lastActivityText: '—',
    manageLabel: 'Edit connection',
    manage: { kind: 'proxmox-node', nodeKind, nodeId: node.id },
  });
};

const pollBackedPresentation = (
  enabled: boolean,
  lastSuccessAt: string | undefined,
  consecutiveFailures: number | undefined,
) => {
  const lastActivityText = formatActivity(lastSuccessAt);
  const failures = consecutiveFailures ?? 0;

  if (!enabled) {
    return {
      label: 'Disabled',
      className: MUTED_BADGE_CLASS,
      lastActivityText,
    };
  }

  if (failures > 2) {
    return {
      label: 'Sync failing',
      className: DANGER_BADGE_CLASS,
      lastActivityText,
    };
  }

  if (failures > 0 && !lastSuccessAt) {
    return {
      label: 'Error',
      className: DANGER_BADGE_CLASS,
      lastActivityText: 'No successful sync yet',
    };
  }

  if (lastSuccessAt) {
    return {
      label: 'Healthy',
      className: SUCCESS_BADGE_CLASS,
      lastActivityText,
    };
  }

  return {
    label: 'Awaiting first sync',
    className: WARNING_BADGE_CLASS,
    lastActivityText: '—',
  };
};

const truenasRow = (connection: TrueNASConnection): ConnectionRow => {
  const health = pollBackedPresentation(
    connection.enabled,
    connection.poll?.lastSuccessAt,
    connection.poll?.consecutiveFailures,
  );

  return buildConfiguredRow({
    id: `truenas:${connection.id}`,
    name: connection.name || connection.host,
    host: connection.host,
    subtitle: 'Configured platform connection',
    coverageLabels: ['TrueNAS data'],
    collectionLabel: 'API',
    statusLabel: health.label,
    statusClassName: health.className,
    lastActivityText: health.lastActivityText,
    manageLabel: 'Edit connection',
    manage: { kind: 'truenas-connection', connectionId: connection.id },
  });
};

const vmwareRow = (connection: VMwareConnection): ConnectionRow => {
  const health = pollBackedPresentation(
    connection.enabled,
    connection.poll?.lastSuccessAt,
    connection.poll?.consecutiveFailures,
  );

  return buildConfiguredRow({
    id: `vmware:${connection.id}`,
    name: connection.name || connection.host,
    host: connection.host,
    subtitle: 'Configured platform connection',
    coverageLabels: ['VMware data'],
    collectionLabel: 'API',
    statusLabel: health.label,
    statusClassName: health.className,
    lastActivityText: health.lastActivityText,
    manageLabel: 'Edit connection',
    manage: { kind: 'vmware-connection', connectionId: connection.id },
  });
};

const isTopLevelInfrastructureRow = (row: UnifiedAgentRow) =>
  !row.linkedVmId?.trim() && !row.linkedContainerId?.trim();

const reportingConfigKeys = (rows: UnifiedAgentRow[]) => {
  const keys = new Set<string>();

  for (const row of rows) {
    for (const surface of row.surfaces) {
      switch (surface.kind) {
        case 'proxmox':
        case 'pbs':
        case 'pmg': {
          const rawId = surface.idValue || surface.controlId || row.id;
          if (rawId) {
            keys.add(`${surface.kind}:${normalizeConfigKey(rawId)}`);
          }
          break;
        }
        case 'truenas': {
          const rawId = surface.idValue || row.hostname || row.id;
          if (rawId) {
            keys.add(`truenas:${normalizeConfigKey(rawId)}`);
          }
          break;
        }
        default:
          break;
      }
    }
  }

  return keys;
};

const compareRows = (left: ConnectionRow, right: ConnectionRow) => {
  const leftName = left.name.trim().toLowerCase();
  const rightName = right.name.trim().toLowerCase();
  if (leftName === rightName) {
    return left.id.localeCompare(right.id);
  }
  return leftName.localeCompare(rightName);
};

export interface ConnectionsTableSources {
  activeRows: readonly UnifiedAgentRow[];
  monitoringStoppedRows: readonly UnifiedAgentRow[];
  pveNodes: readonly NodeConfigWithStatus[];
  pbsNodes: readonly NodeConfigWithStatus[];
  pmgNodes: readonly NodeConfigWithStatus[];
  truenasConnections: readonly TrueNASConnection[];
  vmwareConnections: readonly VMwareConnection[];
  includeConfigurationRows?: boolean;
}

export function buildConnectionRows(sources: ConnectionsTableSources): ConnectionRow[] {
  const infrastructureRows = [
    ...sources.activeRows.filter(isTopLevelInfrastructureRow),
    ...sources.monitoringStoppedRows.filter(isTopLevelInfrastructureRow),
  ];
  const reportingRows = [
    ...infrastructureRows.map(reportingRow),
  ];

  if (sources.includeConfigurationRows === false) {
    return reportingRows.sort(compareRows);
  }

  const seenConfigKeys = reportingConfigKeys(infrastructureRows);

  const configuredRows: ConnectionRow[] = [
    ...sources.pveNodes
      .filter((node) => !seenConfigKeys.has(`proxmox:${normalizeConfigKey(node.id)}`))
      .map((node) => proxmoxNodeRow(node, 'Proxmox VE', 'pve')),
    ...sources.pbsNodes
      .filter((node) => !seenConfigKeys.has(`pbs:${normalizeConfigKey(node.id)}`))
      .map((node) => proxmoxNodeRow(node, 'PBS', 'pbs')),
    ...sources.pmgNodes
      .filter((node) => !seenConfigKeys.has(`pmg:${normalizeConfigKey(node.id)}`))
      .map((node) => proxmoxNodeRow(node, 'PMG', 'pmg')),
    ...sources.truenasConnections
      .filter((connection) => !seenConfigKeys.has(`truenas:${normalizeConfigKey(connection.host)}`))
      .map(truenasRow),
    ...sources.vmwareConnections.map(vmwareRow),
  ];

  return [...reportingRows, ...configuredRows].sort(compareRows);
}
