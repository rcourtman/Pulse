import type { Connection } from '@/api/connections';
import type { ConnectionType } from '@/api/connections';
import type {
  ConnectionFleetAdapterHealth,
  ConnectionFleetConfigRollout,
  ConnectionFleetCredentialStatus,
  ConnectionFleetEnrollmentState,
  ConnectionFleetGovernance,
  ConnectionFleetLivenessState,
  ConnectionFleetRemoteControl,
  ConnectionFleetUpdateStatus,
  ConnectionFleetVersionDrift,
} from '@/api/connections';

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

const prettifyPlatform = (platform?: string | null): string | null => {
  const normalized = platform?.trim().toLowerCase();
  if (!normalized) return null;
  switch (normalized) {
    case 'linux':
      return 'Linux';
    case 'windows':
      return 'Windows';
    case 'darwin':
    case 'macos':
      return 'macOS';
    case 'freebsd':
      return 'FreeBSD';
    case 'unraid':
      return 'Unraid';
    default:
      return platform?.trim() ?? null;
  }
};

const isIPv4Literal = (value: string): boolean => /^\d{1,3}(?:\.\d{1,3}){3}$/.test(value);

const firstDistinctAgentIPv4Alias = (connection: Connection): string | null => {
  const excluded = new Set(
    [
      connection.name,
      connection.address,
      connection.agentIdentity?.hostname,
      connection.agentIdentity?.reportIp,
    ]
      .map((value) => value?.trim().toLowerCase())
      .filter((value): value is string => Boolean(value)),
  );

  for (const candidate of connection.hostAliases ?? []) {
    const trimmed = candidate.trim();
    if (!trimmed || !isIPv4Literal(trimmed)) continue;
    if (excluded.has(trimmed.toLowerCase())) continue;
    return trimmed;
  }
  return null;
};

export const connectionAgentIdentitySummary = (connection: Connection): string | null => {
  const osName = connection.agentIdentity?.osName?.trim();
  const osVersion = connection.agentIdentity?.osVersion?.trim();
  if (osName && osVersion) return `${osName} ${osVersion}`;
  if (osName) return osName;
  return prettifyPlatform(connection.agentIdentity?.platform);
};

export const connectionAgentEndpointDisplay = (connection: Connection): string | null => {
  const normalizedName = connection.name.trim().toLowerCase();
  const reportIp = connection.agentIdentity?.reportIp?.trim();
  if (reportIp) return reportIp;

  const aliasIPv4 = firstDistinctAgentIPv4Alias(connection);
  if (aliasIPv4) return aliasIPv4;

  const hostname = connection.agentIdentity?.hostname?.trim();
  if (hostname && hostname.toLowerCase() !== normalizedName) {
    return hostname;
  }

  const address = connection.address?.trim();
  if (!address) return null;
  return address.toLowerCase() !== normalizedName ? address : null;
};

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
  availability: 'Availability',
};

export const surfaceLabel = (key: string): string => SURFACE_LABELS[key] ?? key;

export type InfrastructureSourceKind = 'api' | 'agent' | 'both' | 'probe' | 'unknown';

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
  probe: {
    label: 'Probe',
    badgeClassName:
      'inline-flex items-center rounded-full bg-cyan-100 px-2 py-0.5 text-[11px] font-medium text-cyan-800 dark:bg-cyan-950/40 dark:text-cyan-200',
    title: 'Data collected by an agentless availability probe',
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

export type FleetGovernanceSignalKey =
  | 'enrollment'
  | 'liveness'
  | 'version'
  | 'adapter'
  | 'config'
  | 'credentials'
  | 'updates'
  | 'remote-control';

export type FleetGovernanceSignalTone = 'ok' | 'info' | 'warning' | 'critical' | 'muted';

export interface FleetGovernanceSignal {
  key: FleetGovernanceSignalKey;
  label: string;
  detail: string;
  tone: FleetGovernanceSignalTone;
}

const DEFAULT_FLEET_GOVERNANCE: ConnectionFleetGovernance = {
  enrollmentState: 'pending',
  livenessState: 'pending',
  versionDrift: 'unknown',
  adapterHealth: 'unknown',
  configRollout: 'unknown',
  credentialStatus: 'unknown',
  updateStatus: 'unknown',
  remoteControl: 'not-applicable',
};

export const fleetGovernanceForConnection = (connection: Connection): ConnectionFleetGovernance =>
  connection.fleet ?? DEFAULT_FLEET_GOVERNANCE;

const fleetSignalClassNameByTone: Record<FleetGovernanceSignalTone, string> = {
  ok: 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200',
  info: 'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-200',
  warning:
    'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200',
  critical:
    'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-200',
  muted: 'border-border bg-surface-alt text-muted',
};

export const fleetSignalClassName = (tone: FleetGovernanceSignalTone): string =>
  `inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-medium whitespace-nowrap ${fleetSignalClassNameByTone[tone]}`;

const enrollmentSignal = (
  state: ConnectionFleetEnrollmentState,
  connection: Connection,
): FleetGovernanceSignal => {
  switch (state) {
    case 'configured':
      return {
        key: 'enrollment',
        label: 'Configured',
        detail: 'This source is configured in the infrastructure ledger.',
        tone: 'ok',
      };
    case 'enrolled':
      return {
        key: 'enrollment',
        label: 'Enrolled',
        detail: 'This agent has reported into the fleet ledger.',
        tone: 'ok',
      };
    case 'paused':
      return {
        key: 'enrollment',
        label: 'Paused',
        detail: 'This source is present but currently paused.',
        tone: 'muted',
      };
    case 'pending':
      return {
        key: 'enrollment',
        label: 'Enrollment pending',
        detail:
          connection.type === 'agent'
            ? 'Pulse has not received the first agent report yet.'
            : 'Pulse has not confirmed this source yet.',
        tone: 'warning',
      };
  }
};

const livenessSignal = (state: ConnectionFleetLivenessState): FleetGovernanceSignal => {
  switch (state) {
    case 'active':
      return {
        key: 'liveness',
        label: 'Live',
        detail: 'Pulse has recent activity for this source.',
        tone: 'ok',
      };
    case 'paused':
      return {
        key: 'liveness',
        label: 'Paused',
        detail: 'Pulse is not polling this source while it is paused.',
        tone: 'muted',
      };
    case 'stale':
      return {
        key: 'liveness',
        label: 'Stale',
        detail: 'Pulse has not seen recent activity from this source.',
        tone: 'warning',
      };
    case 'pending':
      return {
        key: 'liveness',
        label: 'Pending',
        detail: 'Pulse is waiting for first activity from this source.',
        tone: 'warning',
      };
    case 'unauthorized':
      return {
        key: 'liveness',
        label: 'Unauthorized',
        detail: 'The configured credential is being rejected.',
        tone: 'critical',
      };
    case 'unreachable':
      return {
        key: 'liveness',
        label: 'Unreachable',
        detail: 'Pulse cannot currently reach this source.',
        tone: 'critical',
      };
  }
};

const versionSignal = (state: ConnectionFleetVersionDrift): FleetGovernanceSignal => {
  switch (state) {
    case 'behind':
      return {
        key: 'version',
        label: 'Version behind',
        detail: 'This agent is behind the current Pulse Agent target.',
        tone: 'warning',
      };
    case 'current':
      return {
        key: 'version',
        label: 'Version current',
        detail: 'This agent matches the current Pulse Agent target.',
        tone: 'ok',
      };
    case 'unknown':
      return {
        key: 'version',
        label: 'Version unknown',
        detail: 'Pulse does not yet have enough version data for this agent.',
        tone: 'muted',
      };
    case 'not-applicable':
      return {
        key: 'version',
        label: 'No agent version',
        detail: 'This source is not governed by the Pulse Agent binary version.',
        tone: 'muted',
      };
  }
};

const adapterSignal = (state: ConnectionFleetAdapterHealth): FleetGovernanceSignal => {
  switch (state) {
    case 'healthy':
      return {
        key: 'adapter',
        label: 'Adapter healthy',
        detail: 'The collection adapter is operating normally.',
        tone: 'ok',
      };
    case 'degraded':
      return {
        key: 'adapter',
        label: 'Adapter degraded',
        detail: 'The collection adapter needs attention or first confirmation.',
        tone: 'warning',
      };
    case 'blocked':
      return {
        key: 'adapter',
        label: 'Adapter blocked',
        detail: 'The collection adapter cannot complete its current read path.',
        tone: 'critical',
      };
    case 'paused':
      return {
        key: 'adapter',
        label: 'Adapter paused',
        detail: 'Collection is paused for this source.',
        tone: 'muted',
      };
    case 'unknown':
      return {
        key: 'adapter',
        label: 'Adapter unknown',
        detail: 'Pulse has not classified adapter health yet.',
        tone: 'muted',
      };
  }
};

const configSignal = (state: ConnectionFleetConfigRollout): FleetGovernanceSignal => {
  switch (state) {
    case 'configured':
      return {
        key: 'config',
        label: 'Config set',
        detail: 'This source has configured collection scope.',
        tone: 'ok',
      };
    case 'reported':
      return {
        key: 'config',
        label: 'Config reported',
        detail: 'This agent is reporting its applied runtime configuration.',
        tone: 'ok',
      };
    case 'paused':
      return {
        key: 'config',
        label: 'Config paused',
        detail: 'Config changes are not active while this source is paused.',
        tone: 'muted',
      };
    case 'unknown':
      return {
        key: 'config',
        label: 'Config unknown',
        detail: 'Pulse has not received enough runtime configuration state yet.',
        tone: 'warning',
      };
  }
};

const credentialSignal = (state: ConnectionFleetCredentialStatus): FleetGovernanceSignal => {
  switch (state) {
    case 'verified':
      return {
        key: 'credentials',
        label: 'Credentials verified',
        detail: 'The current credential path is accepted.',
        tone: 'ok',
      };
    case 'invalid':
      return {
        key: 'credentials',
        label: 'Credentials invalid',
        detail: 'The current credential path is rejected by the source.',
        tone: 'critical',
      };
    case 'paused':
      return {
        key: 'credentials',
        label: 'Credentials paused',
        detail: 'Credential checks are paused with this source.',
        tone: 'muted',
      };
    case 'unknown':
      return {
        key: 'credentials',
        label: 'Credentials unknown',
        detail: 'Pulse has not verified this credential path yet.',
        tone: 'warning',
      };
  }
};

const updateSignal = (state: ConnectionFleetUpdateStatus): FleetGovernanceSignal => {
  switch (state) {
    case 'update-available':
      return {
        key: 'updates',
        label: 'Update available',
        detail: 'A newer Pulse Agent binary is available for this system.',
        tone: 'warning',
      };
    case 'current':
      return {
        key: 'updates',
        label: 'Update current',
        detail: 'This agent is already on the current target version.',
        tone: 'ok',
      };
    case 'unknown':
      return {
        key: 'updates',
        label: 'Update unknown',
        detail: 'Pulse cannot yet compare this agent with the target version.',
        tone: 'muted',
      };
    case 'not-applicable':
      return {
        key: 'updates',
        label: 'No agent update',
        detail: 'This source is not updated through the Pulse Agent binary rollout.',
        tone: 'muted',
      };
  }
};

const remoteControlSignal = (state: ConnectionFleetRemoteControl): FleetGovernanceSignal => {
  switch (state) {
    case 'enabled':
      return {
        key: 'remote-control',
        label: 'Remote control enabled',
        detail: 'Pulse command execution is enabled for this agent.',
        tone: 'info',
      };
    case 'disabled':
      return {
        key: 'remote-control',
        label: 'Remote control off',
        detail: 'Pulse command execution is disabled for this agent.',
        tone: 'muted',
      };
    case 'not-applicable':
      return {
        key: 'remote-control',
        label: 'No remote control',
        detail: 'This source does not use Pulse Agent command execution.',
        tone: 'muted',
      };
  }
};

export const fleetGovernanceSignalsForConnection = (
  connection: Connection,
): FleetGovernanceSignal[] => {
  const fleet = fleetGovernanceForConnection(connection);
  return [
    enrollmentSignal(fleet.enrollmentState, connection),
    livenessSignal(fleet.livenessState),
    versionSignal(fleet.versionDrift),
    adapterSignal(fleet.adapterHealth),
    configSignal(fleet.configRollout),
    credentialSignal(fleet.credentialStatus),
    updateSignal(fleet.updateStatus),
    remoteControlSignal(fleet.remoteControl),
  ];
};

export const visibleFleetGovernanceSignals = (
  signals: readonly FleetGovernanceSignal[],
): FleetGovernanceSignal[] => {
  const attention = signals.filter(
    (signal) => signal.tone === 'critical' || signal.tone === 'warning',
  );
  const control = signals.filter((signal) => signal.tone === 'info');
  if (attention.length + control.length > 0) {
    return [...attention, ...control].slice(0, 3);
  }
  return [
    {
      key: 'liveness',
      label: 'Fleet OK',
      detail:
        'Enrollment, liveness, adapter health, credentials, and rollout state have no current warnings.',
      tone: 'ok',
    },
  ];
};

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
  fleetSignals: FleetGovernanceSignal[];
  fleetHighlights: FleetGovernanceSignal[];
  primary: boolean;
  agentConnection?: Connection;
}

export interface InfrastructureSystemRow {
  id: string;
  ownerType: ConnectionType;
  name: string;
  subtitle?: string;
  identitySubtitle?: string;
  source: InfrastructureSourceKind;
  host?: string;
  coverageLabels: string[];
  statusLabel: string;
  statusClassName: string;
  agentUpdateCount: number;
  lastActivityText: string;
  lastErrorMessage?: string;
  fleetSignals: FleetGovernanceSignal[];
  fleetHighlights: FleetGovernanceSignal[];
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
