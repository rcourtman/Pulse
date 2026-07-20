import type { Connection } from '@/api/connections';
import type { AgentFleetAgentDiagnostic, AgentFleetDiagnosticReason } from '@/api/agentDiagnostics';
import {
  compareAgentVersions,
  formatAgentVersionDisplay,
  parseAgentVersion,
} from '@/utils/agentVersion';
import type { AgentCommandPlatform } from '@/utils/agentInstallCommand';
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

export type InfrastructureAgentDoctorStatus =
  'healthy' | 'waiting' | 'warning' | 'critical' | 'removed' | 'unknown';

export type InfrastructureAgentDoctorTarget = Omit<
  InfrastructureAgentUpdateTarget,
  'connection'
> & {
  connectionId: string;
  connection?: Connection;
  diagnostic?: AgentFleetAgentDiagnostic;
  status: InfrastructureAgentDoctorStatus;
  reasons: AgentFleetDiagnosticReason[];
  evidence: string[];
  needsUpdate: boolean;
  commandPlatform: AgentCommandPlatform | null;
  commandBlockedReason?: string;
  updaterLabel?: string;
  profileLabel?: string;
  profileVersionLabel?: string;
  lastSeen?: number | string | null;
  source: 'diagnostics' | 'ledger-fallback' | 'removed';
};

export interface InfrastructureAgentDoctorOptions {
  rows: readonly InfrastructureSystemRow[];
  connections?: readonly Connection[];
  diagnostics?: readonly AgentFleetAgentDiagnostic[];
  diagnosticsAvailable: boolean;
  targetVersion?: string | null;
  scopedAgentIds?: readonly string[];
}

const maybeAdd = (flags: Set<string>, flag: string) => {
  if (flag.trim()) flags.add(flag);
};

export const normalizeAgentConnectionID = (value: string | null | undefined): string => {
  const trimmed = (value || '').trim();
  if (!trimmed) return '';
  return trimmed.startsWith('agent:') ? trimmed : `agent:${trimmed}`;
};

export const diagnosticConnectionID = (diagnostic: AgentFleetAgentDiagnostic): string => {
  const explicit = diagnostic.connectionId?.trim();
  if (explicit) return explicit;
  return normalizeAgentConnectionID(diagnostic.agentId || diagnostic.id);
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

const KNOWN_LINUX_PLATFORMS = new Set([
  'alpine',
  'almalinux',
  'amazon',
  'arch',
  'centos',
  'debian',
  'fedora',
  'gentoo',
  'linux',
  'manjaro',
  'nixos',
  'openwrt',
  'opensuse',
  'oracle',
  'proxmox',
  'qnap',
  'raspbian',
  'redhat',
  'rhel',
  'rocky',
  'sles',
  'suse',
  'synology',
  'ubuntu',
  'unraid',
]);

/**
 * Agent Doctor must never turn an unknown platform into an executable command.
 * The general installer defaults unknown values to Linux for legacy callers;
 * this stricter resolver is intentionally limited to host-local repair output.
 */
export const resolveKnownAgentCommandPlatform = (
  platform?: string | null,
): AgentCommandPlatform | null => {
  const normalized = platform?.trim().toLowerCase() ?? '';
  if (!normalized) return null;
  if (normalized.includes('windows')) return 'windows';
  if (
    normalized === 'darwin' ||
    normalized === 'mac' ||
    normalized === 'macos' ||
    normalized.includes('mac os') ||
    normalized.includes('os x')
  ) {
    return 'macos';
  }
  if (
    normalized.includes('freebsd') ||
    normalized.includes('pfsense') ||
    normalized.includes('opnsense')
  ) {
    return 'freebsd';
  }
  if (
    normalized.includes('linux') ||
    KNOWN_LINUX_PLATFORMS.has(normalized) ||
    Array.from(KNOWN_LINUX_PLATFORMS).some((candidate) => normalized.startsWith(`${candidate} `))
  ) {
    return 'linux';
  }
  return null;
};

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

type AgentConnectionBinding = {
  connection: Connection;
  displayName: string;
  contextLabel: string;
  installFlags: string[];
};

const collectAgentConnectionBindings = (
  rows: readonly InfrastructureSystemRow[],
  connections: readonly Connection[],
): Map<string, AgentConnectionBinding> => {
  const bindings = new Map<string, AgentConnectionBinding>();
  const add = (row: InfrastructureSystemRow | undefined, connection?: Connection) => {
    if (!connection || connection.type !== 'agent' || bindings.has(connection.id)) return;
    bindings.set(connection.id, {
      connection,
      displayName: connectionDisplayName(connection),
      contextLabel: row ? rowContextLabel(row) : 'Machine',
      installFlags: row ? updateInstallFlagsForRow(row) : [],
    });
  };

  for (const row of rows) {
    add(row, row.connection);
    for (const connection of row.attachedConnections) add(row, connection);
    for (const member of row.members) add(row, member.agentConnection);
  }
  // Rows deliberately suppress some duplicate physical hosts. The raw ledger
  // remains authoritative for fleet membership, so retain unrepresented agents.
  for (const connection of connections) add(undefined, connection);
  return bindings;
};

const fallbackReason = (
  code: string,
  severity: 'warning' | 'critical',
  message: string,
  evidence: string[] = [],
): AgentFleetDiagnosticReason => ({ code, severity, message, evidence });

const ledgerFallbackReasons = (
  connection: Connection,
  needsUpdate: boolean,
): AgentFleetDiagnosticReason[] => {
  const reasons: AgentFleetDiagnosticReason[] = [];
  switch (connection.state) {
    case 'unauthorized':
      reasons.push(
        fallbackReason(
          'ledger_unauthorized',
          'critical',
          'The agent connection is unauthorized.',
          connection.stateReason ? [connection.stateReason] : [],
        ),
      );
      break;
    case 'unreachable':
      reasons.push(
        fallbackReason(
          'ledger_unreachable',
          'critical',
          'The agent connection is unreachable.',
          connection.stateReason ? [connection.stateReason] : [],
        ),
      );
      break;
    case 'stale':
      reasons.push(
        fallbackReason(
          'ledger_stale',
          'warning',
          'The agent has stopped reporting recently.',
          connection.stateReason ? [connection.stateReason] : [],
        ),
      );
      break;
    case 'pending':
      reasons.push(fallbackReason('ledger_pending', 'warning', 'The agent has not reported yet.'));
      break;
    case 'paused':
      reasons.push(fallbackReason('ledger_paused', 'warning', 'The agent is paused.'));
      break;
  }

  if (connection.agentUpdate?.state === 'error') {
    reasons.push(
      fallbackReason(
        'ledger_update_error',
        'warning',
        'The last agent update attempt failed.',
        connection.agentUpdate.lastError ? [connection.agentUpdate.lastError] : [],
      ),
    );
  }
  for (const module of connection.agentModules ?? []) {
    if (module.enabled && module.state !== 'running') {
      reasons.push(
        fallbackReason(
          'ledger_module_degraded',
          'warning',
          `${module.name} is enabled but ${module.state}.`,
          module.lastError ? [module.lastError] : [],
        ),
      );
    }
  }
  if (needsUpdate) {
    reasons.push(
      fallbackReason(
        'agent_version_stale',
        'warning',
        'This agent is behind the supported target.',
      ),
    );
  }
  return reasons;
};

const fallbackStatus = (
  connection: Connection,
  reasons: readonly AgentFleetDiagnosticReason[],
): InfrastructureAgentDoctorStatus => {
  if (reasons.some((reason) => reason.severity === 'critical')) return 'critical';
  if (reasons.length > 0) return 'warning';
  if (connection.state === 'active' && connection.fleet?.versionDrift === 'current') {
    return 'healthy';
  }
  return 'unknown';
};

const evidenceFor = (
  connection: Connection | undefined,
  diagnostic: AgentFleetAgentDiagnostic | undefined,
): string[] => {
  const evidence = new Set<string>();
  if (connection?.id) evidence.add(`Connection: ${connection.id}`);
  if (connection?.agentIdentity?.hostname) {
    evidence.add(`Hostname: ${connection.agentIdentity.hostname}`);
  }
  if (connection?.agentIdentity?.platform) {
    const platform = [connection.agentIdentity.platform, connection.agentIdentity.architecture]
      .filter(Boolean)
      .join(' / ');
    evidence.add(`Platform: ${platform}`);
  }
  if (connection?.agentIdentity?.reportIp) {
    evidence.add(`Reported IP: ${connection.agentIdentity.reportIp}`);
  }
  const update = connection?.agentUpdate ?? diagnostic?.agentUpdate;
  if (update?.lastCheckedAt) evidence.add(`Last updater check: ${update.lastCheckedAt}`);
  if (update?.lastAttemptAt) evidence.add(`Last update attempt: ${update.lastAttemptAt}`);
  if (update?.lastSuccessAt) evidence.add(`Last successful update: ${update.lastSuccessAt}`);
  if (diagnostic?.machineIdFingerprint) {
    evidence.add(`Machine identity: ${diagnostic.machineIdFingerprint}`);
  }
  for (const address of diagnostic?.interfaceAddresses ?? []) {
    evidence.add(`Reported interface: ${address}`);
  }
  for (const reason of diagnostic?.reasons ?? []) {
    for (const item of reason.evidence ?? []) evidence.add(item);
  }
  return Array.from(evidence);
};

const updaterPresentation = (
  connection: Connection,
  diagnostic: AgentFleetAgentDiagnostic | undefined,
  needsUpdate: boolean,
): { label?: string; waiting: boolean } => {
  const update = connection.agentUpdate ?? diagnostic?.agentUpdate;
  if (!update) return { waiting: false };

  const state = update.state?.trim().toLowerCase();
  switch (state) {
    case 'updating':
      return { label: 'Updating automatically', waiting: needsUpdate };
    case 'checking':
      return {
        label: update.autoUpdate ? 'Checking for an automatic update' : 'Checking for an update',
        waiting: needsUpdate && update.autoUpdate,
      };
    case 'update-available':
      return {
        label: update.autoUpdate
          ? 'Update queued automatically'
          : 'Update available; manual action required',
        waiting: needsUpdate && update.autoUpdate,
      };
    case 'idle':
      return {
        label:
          needsUpdate && update.autoUpdate
            ? 'Waiting for the next automatic check'
            : update.autoUpdate
              ? 'Automatic updates ready'
              : 'Manual updates only',
        waiting: needsUpdate && update.autoUpdate,
      };
    case 'disabled':
      return { label: 'Automatic updates disabled', waiting: false };
    case 'error':
      return { label: 'Last update attempt failed', waiting: false };
    default:
      return state ? { label: `Updater state: ${state}`, waiting: false } : { waiting: false };
  }
};

const doctorTargetFromBinding = (
  binding: AgentConnectionBinding,
  diagnostic: AgentFleetAgentDiagnostic | undefined,
  diagnosticsAvailable: boolean,
  targetVersion?: string | null,
): InfrastructureAgentDoctorTarget => {
  const connection = binding.connection;
  const expectedVersion = expectedVersionFor(connection, targetVersion);
  const needsUpdate = connectionNeedsUpdate(connection, expectedVersion);
  const fallbackReasons = ledgerFallbackReasons(connection, needsUpdate);
  const reasons = diagnosticsAvailable && diagnostic ? (diagnostic.reasons ?? []) : fallbackReasons;
  const updater = updaterPresentation(connection, diagnostic, needsUpdate);
  let status: InfrastructureAgentDoctorStatus =
    diagnosticsAvailable && diagnostic
      ? diagnostic.status
      : fallbackStatus(connection, fallbackReasons);

  // The ledger can advance one poll ahead of diagnostics. Never hide a live
  // update/error signal while the structured endpoint catches up.
  if (status === 'healthy' && fallbackReasons.length > 0) status = 'warning';

  const nonVersionReasons = reasons.filter((reason) => reason.code !== 'agent_version_stale');
  if (
    updater.waiting &&
    connection.state === 'active' &&
    status !== 'critical' &&
    nonVersionReasons.length === 0
  ) {
    status = 'waiting';
  }

  const commandPlatform = resolveKnownAgentCommandPlatform(connection.agentIdentity?.platform);
  let commandBlockedReason: string | undefined;
  if (needsUpdate && !expectedVersion) {
    commandBlockedReason = 'No supported target version is available, so Pulse will not guess.';
  } else if (needsUpdate && !parseAgentVersion(expectedVersion)) {
    commandBlockedReason = 'The reported target version is not a supported release version.';
  } else if (needsUpdate && !commandPlatform) {
    commandBlockedReason =
      'The agent did not report a recognized platform, so Pulse will not guess an update command.';
  } else if (needsUpdate && commandPlatform === 'freebsd') {
    commandBlockedReason =
      'Pulse cannot verify saved FreeBSD or pfSense installer state yet. Open Install on a host for the reviewed manual path instead of running a guessed update command.';
  } else if (updater.waiting) {
    commandBlockedReason =
      'This eligible v6 agent is handling the update asynchronously. Wait for its updater result before using a manual command.';
  }

  const hasStructuredUpgradeAction = Boolean(
    diagnostic?.repairActions?.some(
      (action) => action.code === 'copy_upgrade_command' && action.supported,
    ),
  );
  if (
    needsUpdate &&
    diagnosticsAvailable &&
    diagnostic &&
    !hasStructuredUpgradeAction &&
    !commandBlockedReason
  ) {
    commandBlockedReason = 'The diagnostic service did not offer a supported update repair.';
  }

  const profileLabel =
    diagnostic?.profileName?.trim() || diagnostic?.profileId?.trim() || undefined;
  const profileVersionLabel = diagnostic?.profileVersion
    ? `Expected v${diagnostic.profileVersion} · deployed v${diagnostic.deployedProfileVersion || 0}`
    : undefined;

  return {
    key: connection.id,
    connectionId: connection.id,
    connection,
    diagnostic,
    displayName: binding.displayName,
    contextLabel: binding.contextLabel,
    currentVersion: connection.agentVersion?.trim() || diagnostic?.version?.trim() || undefined,
    expectedVersion,
    installFlags: binding.installFlags,
    status,
    reasons,
    evidence: evidenceFor(connection, diagnostic),
    needsUpdate,
    commandPlatform,
    commandBlockedReason,
    updaterLabel: updater.label,
    profileLabel,
    profileVersionLabel,
    lastSeen: connection.lastSeen ?? diagnostic?.lastSeen,
    source: diagnosticsAvailable && diagnostic ? 'diagnostics' : 'ledger-fallback',
  };
};

const removedDoctorTarget = (
  diagnostic: AgentFleetAgentDiagnostic,
): InfrastructureAgentDoctorTarget => ({
  key: `removed:${diagnostic.rowKey || diagnostic.id}`,
  connectionId: diagnosticConnectionID(diagnostic),
  diagnostic,
  displayName: diagnostic.name || diagnostic.hostname || diagnostic.id,
  contextLabel: diagnostic.types?.join(' + ') || 'Removed agent',
  currentVersion: diagnostic.version?.trim() || undefined,
  installFlags: [],
  status: 'removed',
  reasons: diagnostic.reasons ?? [],
  evidence: evidenceFor(undefined, diagnostic),
  needsUpdate: false,
  commandPlatform: null,
  profileLabel: diagnostic.profileName?.trim() || diagnostic.profileId?.trim() || undefined,
  profileVersionLabel: diagnostic.profileVersion
    ? `Expected v${diagnostic.profileVersion} · deployed v${diagnostic.deployedProfileVersion || 0}`
    : undefined,
  lastSeen: diagnostic.lastSeen,
  source: 'removed',
});

const DOCTOR_STATUS_RANK: Record<InfrastructureAgentDoctorStatus, number> = {
  critical: 6,
  warning: 5,
  waiting: 4,
  unknown: 3,
  removed: 2,
  healthy: 1,
};

export const collectInfrastructureAgentDoctorTargets = ({
  rows,
  connections = [],
  diagnostics = [],
  diagnosticsAvailable,
  targetVersion,
  scopedAgentIds = [],
}: InfrastructureAgentDoctorOptions): InfrastructureAgentDoctorTarget[] => {
  const bindings = collectAgentConnectionBindings(rows, connections);
  const diagnosticsByConnectionID = new Map(
    diagnostics
      .filter((diagnostic) => diagnostic.status !== 'removed')
      .map((diagnostic) => [diagnosticConnectionID(diagnostic), diagnostic]),
  );
  const scoped = new Set(scopedAgentIds.map(normalizeAgentConnectionID).filter(Boolean));
  const inScope = (connectionId: string) =>
    scoped.size === 0 || scoped.has(normalizeAgentConnectionID(connectionId));

  const targets = Array.from(bindings.values())
    .filter((binding) => inScope(binding.connection.id))
    .map((binding) =>
      doctorTargetFromBinding(
        binding,
        diagnosticsByConnectionID.get(binding.connection.id),
        diagnosticsAvailable,
        targetVersion,
      ),
    );

  if (diagnosticsAvailable && scoped.size === 0) {
    for (const diagnostic of diagnostics) {
      if (diagnostic.status === 'removed') targets.push(removedDoctorTarget(diagnostic));
    }
  }

  return targets.sort(
    (left, right) =>
      DOCTOR_STATUS_RANK[right.status] - DOCTOR_STATUS_RANK[left.status] ||
      left.displayName.localeCompare(right.displayName),
  );
};

export const summarizeInfrastructureAgentDoctorTargets = (
  targets: readonly InfrastructureAgentDoctorTarget[],
) => ({
  total: targets.length,
  healthy: targets.filter((target) => target.status === 'healthy').length,
  waiting: targets.filter((target) => target.status === 'waiting').length,
  warning: targets.filter((target) => target.status === 'warning').length,
  critical: targets.filter((target) => target.status === 'critical').length,
  unknown: targets.filter((target) => target.status === 'unknown').length,
  removed: targets.filter((target) => target.status === 'removed').length,
});
