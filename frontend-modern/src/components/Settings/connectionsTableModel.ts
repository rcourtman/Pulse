import type { Connection, ConnectionAgentIdentity } from '@/api/connections';
import type { ConnectionType } from '@/api/connections';
import type {
  ConnectionFleetAdapterHealth,
  ConnectionFleetCommandPolicy,
  ConnectionFleetConfigDrift,
  ConnectionFleetCredentialHealth,
  ConnectionFleetEnrollmentState,
  ConnectionFleetGovernance,
  ConnectionFleetLivenessState,
  ConnectionFleetRolloutState,
  ConnectionFleetUpdateStatus,
  ConnectionFleetVersionDrift,
} from '@/api/connections';
import { getAgentHostProfileFamily } from '@/utils/platformSupportManifest';

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

type ConnectionAgentIdentityPresentation = ConnectionAgentIdentity & {
  hostProfile?: string | null;
};

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
    default:
      return platform?.trim() ?? null;
  }
};

const connectionAgentHostProfileLabel = (
  identity?: ConnectionAgentIdentityPresentation | null,
): string | null => {
  const hostProfile = identity?.hostProfile?.trim();
  if (hostProfile) {
    return getAgentHostProfileFamily(hostProfile) ?? prettifyPlatform(hostProfile);
  }
  // Prefer osName over platform: osName carries the specific platform identity
  // ("Proxmox VE", "Unraid", "TrueNAS SCALE") while platform is the broader OS
  // family ("debian", "linux") that loses that identity on display.
  const osName = identity?.osName?.trim();
  const platform = identity?.platform?.trim();
  return prettifyPlatform(osName || platform);
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
  const hostProfile = connectionAgentHostProfileLabel(connection.agentIdentity);
  const osName = connection.agentIdentity?.osName?.trim();
  const osVersion = connection.agentIdentity?.osVersion?.trim();
  if (hostProfile && osVersion) return `${hostProfile} ${osVersion}`;
  if (osName && osVersion) return `${osName} ${osVersion}`;
  if (osName) return osName;
  return hostProfile;
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
      'inline-flex items-center rounded-full bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-base-content',
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
  | 'module-health'
  | 'config'
  | 'config-drift'
  | 'credentials'
  | 'rollout'
  | 'credential-health'
  | 'updates'
  | 'remote-control'
  | 'command-policy';

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

const configDriftFromFleet = (fleet: ConnectionFleetGovernance): ConnectionFleetConfigDrift => {
  if (fleet.configDrift) return fleet.configDrift;

  switch (fleet.configRollout) {
    case 'configured':
    case 'reported':
      return { status: 'current', reason: 'Configuration is current.' };
    case 'paused':
      return { status: 'paused', reason: 'Configuration rollout is paused.' };
    case 'unknown':
      return { status: 'unknown', reason: 'Configuration drift state is unknown.' };
  }
};

const rolloutFromFleet = (fleet: ConnectionFleetGovernance): ConnectionFleetRolloutState => {
  if (fleet.rollout) return fleet.rollout;

  switch (fleet.configRollout) {
    case 'configured':
    case 'reported':
      return { status: 'current', stage: 'applied' };
    case 'paused':
      return { status: 'paused', stage: 'paused' };
    case 'unknown':
      return { status: 'unknown' };
  }
};

const credentialHealthFromFleet = (
  fleet: ConnectionFleetGovernance,
): ConnectionFleetCredentialHealth => {
  if (fleet.credentialHealth) return fleet.credentialHealth;

  switch (fleet.credentialStatus) {
    case 'verified':
      return { status: 'verified' };
    case 'invalid':
      return { status: 'invalid' };
    case 'paused':
      return { status: 'paused' };
    case 'unknown':
      return { status: 'unknown' };
  }
};

const commandPolicyFromFleet = (fleet: ConnectionFleetGovernance): ConnectionFleetCommandPolicy => {
  if (fleet.commandPolicy) return fleet.commandPolicy;

  switch (fleet.remoteControl) {
    case 'enabled':
      return {
        status: 'enabled',
        desired: 'unknown',
        applied: 'enabled',
        enforcement: 'not-applicable',
      };
    case 'disabled':
      return {
        status: 'disabled',
        desired: 'unknown',
        applied: 'disabled',
        enforcement: 'not-applicable',
      };
    case 'not-applicable':
      return { status: 'not-applicable', enforcement: 'not-applicable' };
    case 'unknown':
      return {
        status: 'unknown',
        desired: 'unknown',
        applied: 'unknown',
        enforcement: 'pending',
      };
  }
};

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

const configDriftSignal = (state: ConnectionFleetConfigDrift): FleetGovernanceSignal => {
  switch (state.status) {
    case 'current':
      return {
        key: 'config-drift',
        label: 'Config current',
        detail: state.reason || 'Desired and applied configuration fingerprints match.',
        tone: 'ok',
      };
    case 'drifted':
      return {
        key: 'config-drift',
        label: 'Config drift',
        detail: state.reason || 'Desired and applied configuration fingerprints do not match.',
        tone: 'warning',
      };
    case 'pending':
      return {
        key: 'config-drift',
        label: 'Config pending',
        detail: state.reason || 'Pulse is waiting for applied configuration confirmation.',
        tone: 'warning',
      };
    case 'paused':
      return {
        key: 'config-drift',
        label: 'Config paused',
        detail: state.reason || 'Configuration rollout is paused for this source.',
        tone: 'muted',
      };
    case 'unknown':
      return {
        key: 'config-drift',
        label: 'Config unknown',
        detail: state.reason || 'Pulse does not yet have enough config fingerprint data.',
        tone: 'warning',
      };
    case 'not-applicable':
      return {
        key: 'config-drift',
        label: 'No config drift',
        detail: state.reason || 'This source is not governed by desired/applied config rollout.',
        tone: 'muted',
      };
  }
};

const rolloutSignal = (state: ConnectionFleetRolloutState): FleetGovernanceSignal => {
  switch (state.status) {
    case 'current':
      return {
        key: 'rollout',
        label: 'Rollout current',
        detail: state.reason || 'The rollout state is current.',
        tone: 'ok',
      };
    case 'pending':
      return {
        key: 'rollout',
        label: 'Rollout pending',
        detail: state.reason || 'The staged rollout is waiting for confirmation.',
        tone: 'warning',
      };
    case 'paused':
      return {
        key: 'rollout',
        label: 'Rollout paused',
        detail: state.reason || 'The staged rollout is paused.',
        tone: 'warning',
      };
    case 'blocked':
      return {
        key: 'rollout',
        label: 'Rollout blocked',
        detail: state.reason || 'The rollout is blocked by the current connection state.',
        tone: 'critical',
      };
    case 'unknown':
      return {
        key: 'rollout',
        label: 'Rollout unknown',
        detail: state.reason || 'Pulse has not classified staged rollout state yet.',
        tone: 'warning',
      };
    case 'not-applicable':
      return {
        key: 'rollout',
        label: 'No rollout',
        detail: state.reason || 'This source does not use staged rollout control.',
        tone: 'muted',
      };
  }
};

const credentialHealthSignal = (state: ConnectionFleetCredentialHealth): FleetGovernanceSignal => {
  switch (state.status) {
    case 'verified':
      return {
        key: 'credential-health',
        label: 'Credentials verified',
        detail: 'The current credential path is accepted.',
        tone: 'ok',
      };
    case 'invalid':
      return {
        key: 'credential-health',
        label: 'Credentials invalid',
        detail: 'The current credential path is rejected by the source.',
        tone: 'critical',
      };
    case 'expired':
      return {
        key: 'credential-health',
        label: 'Credentials expired',
        detail: 'The credential has passed its configured expiration.',
        tone: 'critical',
      };
    case 'expiring':
      return {
        key: 'credential-health',
        label: 'Credentials expiring',
        detail: 'The credential is approaching its configured expiration.',
        tone: 'warning',
      };
    case 'paused':
      return {
        key: 'credential-health',
        label: 'Credentials paused',
        detail: 'Credential checks are paused with this source.',
        tone: 'muted',
      };
    case 'unknown':
      return {
        key: 'credential-health',
        label: 'Credentials unknown',
        detail: 'Pulse has not verified this credential path yet.',
        tone: 'warning',
      };
    case 'not-applicable':
      return {
        key: 'credential-health',
        label: 'No credentials',
        detail: 'This source does not use a stored credential path.',
        tone: 'muted',
      };
  }
};

const updateSignal = (
  state: ConnectionFleetUpdateStatus,
  update?: Connection['agentUpdate'],
): FleetGovernanceSignal => {
  switch (state) {
    case 'update-available':
      return {
        key: 'updates',
        label: 'Update available',
        detail: 'A newer Pulse Agent binary is available for this system.',
        tone: 'warning',
      };
    case 'checking':
      return {
        key: 'updates',
        label: 'Checking for updates',
        detail: 'This agent is checking the Pulse server for an updated binary.',
        tone: 'info',
      };
    case 'updating':
      return {
        key: 'updates',
        label: 'Agent updating',
        detail: 'This agent is verifying and applying an updated binary.',
        tone: 'info',
      };
    case 'failed':
      return {
        key: 'updates',
        label: 'Agent update failed',
        detail:
          update?.lastError ||
          'The agent could not complete its latest update check or update attempt.',
        tone: 'critical',
      };
    case 'disabled':
      return {
        key: 'updates',
        label: 'Auto-update off',
        detail: 'Automatic Pulse Agent updates are disabled on this system.',
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

const moduleHealthSignal = (connection: Connection): FleetGovernanceSignal | undefined => {
  const module = connection.agentModules?.find(
    (candidate) => candidate.enabled && candidate.state !== 'running',
  );
  if (!module) return undefined;

  const displayName =
    module.name === 'docker'
      ? 'Docker'
      : module.name === 'kubernetes'
        ? 'Kubernetes'
        : module.name === 'host'
          ? 'Host'
          : module.name;
  return {
    key: 'module-health',
    label: `${displayName} module ${module.state}`,
    detail:
      module.lastError || `${displayName} monitoring is enabled but the module is not running yet.`,
    tone: 'warning',
  };
};

const commandPolicySignal = (state: ConnectionFleetCommandPolicy): FleetGovernanceSignal => {
  if (state.enforcement === 'drifted') {
    const desiredDisabledAppliedEnabled =
      state.desired === 'disabled' && state.applied === 'enabled';
    return {
      key: 'command-policy',
      label: 'Command policy mismatch',
      detail: state.reason || 'Desired and applied command-policy states do not match.',
      tone: desiredDisabledAppliedEnabled ? 'critical' : 'warning',
    };
  }
  if (state.enforcement === 'pending') {
    return {
      key: 'command-policy',
      label: 'Command policy pending',
      detail: state.reason || 'Pulse is waiting for applied command-policy confirmation.',
      tone: 'warning',
    };
  }

  switch (state.status) {
    case 'enabled':
      return {
        key: 'command-policy',
        label: 'Remote control enabled',
        detail: state.reason || 'Pulse command execution is enabled for this agent.',
        tone: 'info',
      };
    case 'disabled':
      return {
        key: 'command-policy',
        label: 'Remote control disabled',
        detail: state.reason || 'Pulse command execution is disabled for this agent.',
        tone: 'info',
      };
    case 'blocked':
      return {
        key: 'command-policy',
        label: 'Remote control blocked',
        detail: state.reason || 'Pulse command execution is blocked by policy.',
        tone: 'critical',
      };
    case 'unknown':
      return {
        key: 'command-policy',
        label: 'Remote control unknown',
        detail: state.reason || 'Pulse has not confirmed command-policy state yet.',
        tone: 'warning',
      };
    case 'not-applicable':
      return {
        key: 'command-policy',
        label: 'No remote control',
        detail: state.reason || 'This source does not use Pulse Agent command execution.',
        tone: 'muted',
      };
  }
};

// Builds the row-level problem line when an attached Pulse Agent isn't
// reporting. Derived directly from connection state — not laundered
// through the fleet-signal pipeline — so the row builder owns the user
// story for "API works, agent doesn't" without a synthesized chip hack.
export const agentAttachmentProblem = (agent: Connection): InfrastructureRowProblem | undefined => {
  switch (agent.state) {
    case 'stale': {
      const ago = lastActivityTextFromLastSeen(agent.lastSeen);
      const suffix = ago && ago !== 'No activity yet' && ago !== 'Unknown' ? ` · ${ago}` : '';
      return {
        label: `Agent offline${suffix}`,
        detail:
          agent.stateReason ||
          'The Pulse Agent on this host has not reported recently. Proxmox API metrics are unaffected.',
        tone: 'warning',
      };
    }
    case 'unreachable':
      return {
        label: 'Agent unreachable',
        detail: agent.stateReason || 'Pulse cannot currently reach the agent on this host.',
        tone: 'critical',
      };
    case 'unauthorized':
      return {
        label: 'Agent unauthorized',
        detail: agent.stateReason || 'The Pulse Agent token is being rejected.',
        tone: 'critical',
      };
    case 'pending':
      return {
        label: 'Agent pending first report',
        detail: agent.stateReason || 'The Pulse Agent has not reported yet.',
        tone: 'warning',
      };
    default:
      return undefined;
  }
};

// Connections that run a Pulse Agent / collector binary. Agent-binary and
// managed-config governance (version drift, config drift, staged rollout,
// binary updates, remote command policy) only describes these — a pull-based
// API source (PVE, PBS, PMG, vSphere, TrueNAS, availability probe) has no
// agent, so the backend's echoed rollout/config state must not be surfaced as
// the row's problem. Without this gate an unreachable PBS reads "Rollout
// blocked" instead of plainly reflecting that it is unreachable.
const AGENT_FLEET_CONNECTION_TYPES: ReadonlySet<ConnectionType> = new Set([
  'agent',
  'docker',
  'kubernetes',
]);

const connectionRunsAgentFleet = (connection: Connection): boolean =>
  AGENT_FLEET_CONNECTION_TYPES.has(connection.type);

export const fleetGovernanceSignalsForConnection = (
  connection: Connection,
): FleetGovernanceSignal[] => {
  const fleet = fleetGovernanceForConnection(connection);
  const runsAgentFleet = connectionRunsAgentFleet(connection);
  const signals: FleetGovernanceSignal[] = [
    enrollmentSignal(fleet.enrollmentState, connection),
    livenessSignal(fleet.livenessState),
    credentialHealthSignal(credentialHealthFromFleet(fleet)),
  ];
  if (runsAgentFleet) {
    const moduleSignal = moduleHealthSignal(connection);
    if (moduleSignal) signals.push(moduleSignal);
    signals.push(
      configDriftSignal(configDriftFromFleet(fleet)),
      rolloutSignal(rolloutFromFleet(fleet)),
      versionSignal(fleet.versionDrift),
      updateSignal(fleet.updateStatus, connection.agentUpdate),
    );
  }
  signals.push(adapterSignal(fleet.adapterHealth));
  if (runsAgentFleet) {
    signals.push(commandPolicySignal(commandPolicyFromFleet(fleet)));
  }
  return signals;
};

export const visibleFleetGovernanceSignals = (
  signals: readonly FleetGovernanceSignal[],
): FleetGovernanceSignal[] => {
  const hasPassiveAgentConfigConfirmation = signals.some(isPassiveAgentConfigConfirmationSignal);
  const visibleSignals = signals.filter((signal) => {
    if (isPassiveAgentConfigConfirmationSignal(signal)) return false;
    if (isPassiveAgentRolloutConfirmationFallbackSignal(signal, hasPassiveAgentConfigConfirmation))
      return false;
    // Liveness duplicates the row's status badge column; skip it here so a
    // stale connection doesn't get "Stale" rendered twice.
    if (signal.key === 'liveness') return false;
    // "Adapter degraded" is internal collection-health terminology that
    // doesn't name a user-actionable problem. The agent-attachment chip
    // names the specific failure ("Agent offline · 4h ago") when it
    // matters; everything else this signal raised was redundant noise.
    if (signal.key === 'adapter') return false;
    // Default-disabled remote control is the unconfigured state on every
    // fresh agent. Surface only when policy is actively wrong
    // (blocked/mismatched/pending/unknown). The Manage drawer is the right
    // place to read informational policy state.
    if (signal.key === 'command-policy' && signal.tone === 'info') return false;
    return true;
  });
  const attention = visibleSignals.filter(
    (signal) => signal.tone === 'critical' || signal.tone === 'warning',
  );
  const control = visibleSignals.filter((signal) => signal.tone === 'info');
  return [...attention, ...control].slice(0, 3);
};

const isPassiveAgentConfigConfirmationSignal = (signal: FleetGovernanceSignal): boolean => {
  if (signal.tone !== 'warning') return false;
  if (signal.key !== 'config-drift' && signal.key !== 'rollout') return false;

  const detail = signal.detail.toLowerCase();
  return (
    detail.includes('comparable applied agent config fingerprint') ||
    detail.includes('comparable applied agent configuration fingerprint') ||
    detail.includes('agent to report an applied configuration fingerprint')
  );
};

const isPassiveAgentRolloutConfirmationFallbackSignal = (
  signal: FleetGovernanceSignal,
  hasPassiveAgentConfigConfirmation: boolean,
): boolean => {
  if (!hasPassiveAgentConfigConfirmation) return false;
  if (signal.tone !== 'warning' || signal.key !== 'rollout') return false;

  const detail = signal.detail.toLowerCase();
  return (
    detail.includes('staged rollout is waiting for confirmation') ||
    detail.includes(
      'rollout state cannot be confirmed without comparable desired and applied agent config fingerprints',
    )
  );
};

// A row-level operational problem expressed as one plain-English sentence
// — rendered below the status badge instead of as a parallel coloured chip.
// `fleetHighlights` is retained on the row model for downstream consumers
// (Manage drawer, audit views) that need the full structured signal list.
export interface InfrastructureRowProblem {
  label: string;
  detail: string;
  tone: 'warning' | 'critical';
}

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
  problem?: InfrastructureRowProblem;
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
  problem?: InfrastructureRowProblem;
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

// Pick the single most important problem to surface in the row's status
// column. Critical beats warning; otherwise first wins — the upstream
// builder has already prioritised highlights (attention before info).
export const primaryRowProblem = (
  signals: readonly FleetGovernanceSignal[],
): InfrastructureRowProblem | undefined => {
  const critical = signals.find((signal) => signal.tone === 'critical');
  if (critical) {
    return { label: critical.label, detail: critical.detail, tone: 'critical' };
  }
  const warning = signals.find((signal) => signal.tone === 'warning');
  if (warning) {
    return { label: warning.label, detail: warning.detail, tone: 'warning' };
  }
  return undefined;
};
