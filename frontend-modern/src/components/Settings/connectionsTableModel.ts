import { getUnifiedAgentLastSeenLabel } from '@/utils/unifiedAgentInventoryPresentation';
import {
  getUnifiedAgentStatusPresentation,
  MONITORING_STOPPED_STATUS_LABEL,
} from '@/utils/unifiedAgentStatusPresentation';
import type { UnifiedAgentRow } from './infrastructureOperationsModel';

export type SystemManageAction =
  | { kind: 'connection'; connectionId: string }
  | { kind: 'inventory-active'; rowKey: string }
  | { kind: 'inventory-ignored'; rowKey: string };

export interface InfrastructureSystemRow {
  id: string;
  name: string;
  subtitle?: string;
  host?: string;
  coverageLabels: string[];
  collectionLabel: string;
  statusLabel: string;
  statusClassName: string;
  lastActivityText: string;
  manageLabel: string;
  manage: SystemManageAction;
}

const PROXMOX_CAPABILITY_KEYS = ['proxmox', 'pbs', 'pmg'] as const;
const AGENT_CAPABILITY_KEYS = ['agent', 'docker', 'kubernetes'] as const;

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

const isTopLevelInfrastructureRow = (row: UnifiedAgentRow) =>
  !row.linkedVmId?.trim() && !row.linkedContainerId?.trim();

const compareRows = (left: InfrastructureSystemRow, right: InfrastructureSystemRow) => {
  const leftName = left.name.trim().toLowerCase();
  const rightName = right.name.trim().toLowerCase();
  if (leftName === rightName) {
    return left.id.localeCompare(right.id);
  }
  return leftName.localeCompare(rightName);
};

const reportingRow = (row: UnifiedAgentRow): InfrastructureSystemRow => {
  const statusPresentation = getUnifiedAgentStatusPresentation(row.status, row.healthStatus);

  return {
    id: row.rowKey,
    name: row.name,
    subtitle: row.status === 'removed' ? 'Ignored by Pulse' : undefined,
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

export interface InfrastructureSystemsTableSources {
  activeRows: readonly UnifiedAgentRow[];
  monitoringStoppedRows: readonly UnifiedAgentRow[];
}

export function buildInfrastructureSystemRows(
  sources: InfrastructureSystemsTableSources,
): InfrastructureSystemRow[] {
  return [
    ...sources.activeRows.filter(isTopLevelInfrastructureRow).map(reportingRow),
    ...sources.monitoringStoppedRows.filter(isTopLevelInfrastructureRow).map(reportingRow),
  ].sort(compareRows);
}
