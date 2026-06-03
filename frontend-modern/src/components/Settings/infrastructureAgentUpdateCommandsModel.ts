import type { Connection } from '@/api/connections';
import { compareAgentVersions, formatAgentVersionDisplay } from '@/utils/agentVersion';
import type { InfrastructureSystemRow } from './connectionsTableModel';

export type InfrastructureAgentUpdateTarget = {
  key: string;
  connection: Connection;
  displayName: string;
  contextLabel: string;
  currentVersion?: string;
  expectedVersion?: string;
  installFlags: string[];
};

const maybeAdd = (flags: Set<string>, flag: string) => {
  if (flag.trim()) flags.add(flag);
};

const normalizeAgentConnectionID = (value: string | null | undefined): string => {
  const trimmed = (value || '').trim();
  if (!trimmed) return '';
  return trimmed.startsWith('agent:') ? trimmed : `agent:${trimmed}`;
};

const updateInstallFlagsForRow = (row: InfrastructureSystemRow): string[] => {
  const flags = new Set<string>();

  switch (row.ownerType) {
    case 'pve':
      maybeAdd(flags, '--enable-proxmox');
      maybeAdd(flags, '--proxmox-type pve');
      break;
    case 'pbs':
      maybeAdd(flags, '--enable-proxmox');
      maybeAdd(flags, '--proxmox-type pbs');
      break;
    case 'docker':
      maybeAdd(flags, '--enable-docker');
      break;
    case 'kubernetes':
      maybeAdd(flags, '--enable-kubernetes');
      break;
  }

  return Array.from(flags);
};

const connectionDisplayName = (connection: Connection): string =>
  connection.agentIdentity?.hostname?.trim() ||
  connection.name?.trim() ||
  connection.address?.trim() ||
  connection.id;

const rowContextLabel = (row: InfrastructureSystemRow): string => {
  if (row.isCluster && row.name.trim()) return row.name;
  if (row.ownerType === 'agent') return 'Machine';
  return row.name.trim() || row.connection.name || row.ownerType;
};

const connectionNeedsUpdate = (connection: Connection, targetVersion?: string | null): boolean => {
  if (connection.agentUpdateAvailable) return true;
  const currentVersion = connection.agentVersion?.trim();
  if (!currentVersion || !targetVersion) return false;
  const comparison = compareAgentVersions(currentVersion, targetVersion);
  return comparison !== null && comparison < 0;
};

const expectedVersionFor = (connection: Connection, targetVersion?: string | null) =>
  connection.expectedAgentVersion?.trim() || formatAgentVersionDisplay(targetVersion) || undefined;

const pushTarget = (
  targetsByID: Map<string, InfrastructureAgentUpdateTarget>,
  row: InfrastructureSystemRow,
  connection?: Connection,
  targetVersion?: string | null,
) => {
  if (
    !connection ||
    connection.type !== 'agent' ||
    !connectionNeedsUpdate(connection, targetVersion)
  )
    return;

  const key = connection.id;
  if (targetsByID.has(key)) return;

  targetsByID.set(key, {
    key,
    connection,
    displayName: connectionDisplayName(connection),
    contextLabel: rowContextLabel(row),
    currentVersion: connection.agentVersion?.trim() || undefined,
    expectedVersion: expectedVersionFor(connection, targetVersion),
    installFlags: updateInstallFlagsForRow(row),
  });
};

export const collectInfrastructureAgentUpdateTargets = (
  rows: readonly InfrastructureSystemRow[],
  targetVersion?: string | null,
  scopedAgentIds: readonly string[] = [],
): InfrastructureAgentUpdateTarget[] => {
  const targetsByID = new Map<string, InfrastructureAgentUpdateTarget>();
  const scopedAgentIDSet = new Set(
    scopedAgentIds.map(normalizeAgentConnectionID).filter((value) => value.length > 0),
  );

  for (const row of rows) {
    pushTarget(targetsByID, row, row.connection, targetVersion);
    for (const connection of row.attachedConnections) {
      pushTarget(targetsByID, row, connection, targetVersion);
    }
    for (const member of row.members) {
      pushTarget(targetsByID, row, member.agentConnection, targetVersion);
    }
  }

  return Array.from(targetsByID.values())
    .filter(
      (target) =>
        scopedAgentIDSet.size === 0 || scopedAgentIDSet.has(normalizeAgentConnectionID(target.key)),
    )
    .sort((left, right) => left.displayName.localeCompare(right.displayName));
};
