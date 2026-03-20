import type { JSX } from 'solid-js';
import type {
  ConnectedInfrastructureItem,
  ConnectedInfrastructureSurface,
} from '@/types/api';
import {
  getAgentCapabilityLabel,
  type AgentCapability,
} from '@/utils/agentCapabilityPresentation';

export const TOKEN_PLACEHOLDER = '<api-token>';
export const UNIFIED_AGENT_TELEMETRY_SURFACE = 'settings_unified_agents';

export type AgentPlatform = 'linux' | 'macos' | 'freebsd' | 'windows';
export type UnifiedAgentStatus = 'active' | 'removed';
export type ScopeCategory = 'default' | 'profile' | 'ai-managed' | 'na';
export type InstallProfile =
  | 'auto'
  | 'docker'
  | 'kubernetes'
  | 'proxmox-pve'
  | 'proxmox-pbs'
  | 'truenas';

export type SetupHandoffState = {
  username: string;
  password: string;
  apiToken: string;
  createdAt?: string;
};

export type UnifiedAgentSurface = {
  key: string;
  kind: AgentCapability;
  label: string;
  detail: string;
  idLabel?: string;
  idValue?: string;
  action?: 'stop-monitoring' | 'allow-reconnect';
  controlId?: string;
};

export type UnifiedAgentRow = {
  rowKey: string;
  id: string;
  agentActionId?: string;
  dockerActionId?: string;
  kubernetesActionId?: string;
  name: string;
  hostname?: string;
  displayName?: string;
  capabilities: AgentCapability[];
  status: UnifiedAgentStatus;
  healthStatus?: string;
  lastSeen?: number;
  removedAt?: number;
  version?: string;
  isOutdatedBinary?: boolean;
  linkedNodeId?: string;
  commandsEnabled?: boolean;
  agentId?: string;
  upgradePlatform: AgentPlatform;
  scope: {
    label: string;
    detail?: string;
    category: ScopeCategory;
  };
  installFlags: string[];
  searchText: string;
  kubernetesInfo?: {
    server?: string;
    context?: string;
    tokenName?: string;
  };
  surfaces: UnifiedAgentSurface[];
};

export type InventoryActionType = 'stop-monitoring' | 'allow-reconnect';

export type InventoryActionNotice = {
  tone: 'success' | 'info';
  title: string;
  detail: string;
  showRecoveryQueueLink?: boolean;
};

export type StopMonitoringDialogState = {
  row: UnifiedAgentRow;
  subject: string;
  scopeLabel: string;
};

export type InstallProfileOption = {
  value: InstallProfile;
  label: string;
  description: string;
  flags: string[];
};

export type InfrastructureCommandSnippet = {
  label: string;
  command: string;
  note?: JSX.Element | string;
};

export type InfrastructureCommandSection = {
  title: string;
  description: string;
  snippets: InfrastructureCommandSnippet[];
};

export const buildDefaultTokenName = () => {
  const now = new Date();
  const iso = now.toISOString().slice(0, 16); // YYYY-MM-DDTHH:MM
  const stamp = iso.replace('T', ' ').replace(/:/g, '-');
  return `Agent ${stamp}`;
};

export const normalizeTelemetryPart = (value: string) =>
  value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '');

export const shellQuoteArg = (value: string) => `'${value.replace(/'/g, `'\"'\"'`)}'`;

export const getCapabilitySurfaceLabel = (capability: AgentCapability) => {
  switch (capability) {
    case 'agent':
      return 'Host telemetry';
    case 'docker':
      return 'Docker runtime data';
    case 'kubernetes':
      return 'Kubernetes cluster data';
    case 'proxmox':
      return 'Proxmox data';
    case 'pbs':
      return 'PBS data';
    case 'pmg':
      return 'PMG data';
    default:
      return getAgentCapabilityLabel(capability);
  }
};

export const getReconnectActionLabel = (row: UnifiedAgentRow) => {
  if (row.capabilities.includes('docker')) {
    return 'Allow Docker reconnect';
  }
  if (row.capabilities.includes('kubernetes')) {
    return 'Allow Kubernetes reconnect';
  }
  return 'Allow host reconnect';
};

export const joinHumanList = (parts: string[]) => {
  if (parts.length === 0) return '';
  if (parts.length === 1) return parts[0];
  if (parts.length === 2) return `${parts[0]} and ${parts[1]}`;
  return `${parts.slice(0, -1).join(', ')}, and ${parts.at(-1)}`;
};

const sentenceCaseSurfaceLabel = (label: string, index: number) => {
  if (index !== 0 || label.length === 0) {
    return label;
  }
  return `${label.slice(0, 1).toLowerCase()}${label.slice(1)}`;
};

export const getRowReportingSummary = (row: UnifiedAgentRow) => {
  const reported = row.surfaces.map((surface, index) =>
    sentenceCaseSurfaceLabel(surface.label, index),
  );
  if (reported.length === 0) {
    return '';
  }
  return `Pulse is receiving ${joinHumanList(reported)} from this item.`;
};

export const getRowSurfaceBreakdown = (row: UnifiedAgentRow) => row.surfaces;

export const getStopMonitoringSurfaces = (row: UnifiedAgentRow) => {
  const surfaces = getRowSurfaceBreakdown(row);
  const stopMonitoringSurfaces = surfaces.filter((surface) => surface.action === 'stop-monitoring');
  const hostManagedStopApplies = stopMonitoringSurfaces.some((surface) => surface.kind === 'agent');
  if (!hostManagedStopApplies) {
    return stopMonitoringSurfaces;
  }
  return surfaces.filter((surface) =>
    ['agent', 'docker', 'kubernetes', 'proxmox', 'pbs', 'pmg'].includes(surface.kind),
  );
};

export const getStopMonitoringScopeLabel = (row: UnifiedAgentRow) => {
  const surfaceLabels = getStopMonitoringSurfaces(row).map((surface) => surface.label);
  if (surfaceLabels.length === 0) {
    return 'Reporting for this item';
  }
  return joinHumanList(surfaceLabels);
};

export const createSurfaceScopedRow = (
  row: UnifiedAgentRow,
  surfaceKey: 'agent' | 'docker' | 'kubernetes' | 'proxmox' | 'pbs' | 'pmg',
): UnifiedAgentRow => {
  if (surfaceKey === 'docker') {
    return {
      ...row,
      rowKey: `${row.rowKey}-docker-surface`,
      capabilities: ['docker'],
      agentActionId: undefined,
      kubernetesActionId: undefined,
      linkedNodeId: undefined,
      surfaces: row.surfaces.filter((surface) => surface.kind === 'docker'),
    };
  }

  if (surfaceKey === 'kubernetes') {
    return {
      ...row,
      rowKey: `${row.rowKey}-kubernetes-surface`,
      capabilities: ['kubernetes'],
      agentActionId: undefined,
      dockerActionId: undefined,
      linkedNodeId: undefined,
      surfaces: row.surfaces.filter((surface) => surface.kind === 'kubernetes'),
    };
  }

  if (surfaceKey === 'agent') {
    const hostManagedCapabilities: AgentCapability[] = ['agent'];
    if (row.capabilities.includes('proxmox')) hostManagedCapabilities.push('proxmox');
    if (row.capabilities.includes('pbs')) hostManagedCapabilities.push('pbs');
    if (row.capabilities.includes('pmg')) hostManagedCapabilities.push('pmg');
    return {
      ...row,
      rowKey: `${row.rowKey}-agent-surface`,
      capabilities: hostManagedCapabilities,
      dockerActionId: undefined,
      kubernetesActionId: undefined,
      surfaces: row.surfaces.filter((surface) =>
        ['agent', 'proxmox', 'pbs', 'pmg'].includes(surface.kind),
      ),
    };
  }

  if (surfaceKey === 'pbs') {
    return {
      ...row,
      rowKey: `${row.rowKey}-pbs-surface`,
      capabilities: ['pbs'],
      dockerActionId: undefined,
      kubernetesActionId: undefined,
      surfaces: row.surfaces.filter((surface) => surface.kind === 'pbs'),
    };
  }

  if (surfaceKey === 'pmg') {
    return {
      ...row,
      rowKey: `${row.rowKey}-pmg-surface`,
      capabilities: ['pmg'],
      dockerActionId: undefined,
      kubernetesActionId: undefined,
      surfaces: row.surfaces.filter((surface) => surface.kind === 'pmg'),
    };
  }

  return {
    ...row,
    rowKey: `${row.rowKey}-proxmox-surface`,
    capabilities: ['proxmox'],
    dockerActionId: undefined,
    kubernetesActionId: undefined,
    surfaces: row.surfaces.filter((surface) => surface.kind === 'proxmox'),
  };
};

export const INSTALL_PROFILE_OPTIONS: InstallProfileOption[] = [
  {
    value: 'auto',
    label: 'Auto-detect (recommended)',
    description:
      'Let the installer detect Docker, Kubernetes, Proxmox, and agent capabilities automatically.',
    flags: [],
  },
  {
    value: 'docker',
    label: 'Docker / Podman runtime',
    description: 'Force container runtime monitoring even when detection is restricted.',
    flags: ['--enable-docker', '--disable-host'],
  },
  {
    value: 'kubernetes',
    label: 'Kubernetes node',
    description: 'Force Kubernetes monitoring on cluster nodes.',
    flags: ['--enable-kubernetes'],
  },
  {
    value: 'proxmox-pve',
    label: 'Proxmox VE node',
    description: 'Force Proxmox integration and register as a PVE node.',
    flags: ['--enable-proxmox', '--proxmox-type pve'],
  },
  {
    value: 'proxmox-pbs',
    label: 'Proxmox Backup node',
    description: 'Force Proxmox integration and register as a PBS node.',
    flags: ['--enable-proxmox', '--proxmox-type pbs'],
  },
  {
    value: 'truenas',
    label: 'TrueNAS SCALE agent',
    description:
      'Use default auto-detection; installer applies TrueNAS-safe service handling automatically.',
    flags: [],
  },
];

export const buildCommandsByPlatform = (
  unixCommand: string,
  windowsInteractiveCommand: string,
  windowsParameterizedCommand: string,
): Record<AgentPlatform, InfrastructureCommandSection> => ({
  linux: {
    title: 'Install on Linux',
    description:
      'The unified installer downloads the agent binary and configures the appropriate service for your system.',
    snippets: [
      {
        label: 'Install',
        command: unixCommand,
        note: (
          <span>
            Command auto-escalates with <code>sudo</code> when available; otherwise run from a root
            shell (for example <code>su -</code>). Auto-detects your init system and works on
            Debian, Ubuntu, Proxmox, Fedora, Alpine, Unraid, Synology, and more.
          </span>
        ),
      },
    ],
  },
  macos: {
    title: 'Install on macOS',
    description:
      'The unified installer downloads the universal binary and sets up a launchd service for background monitoring.',
    snippets: [
      {
        label: 'Install with launchd',
        command: unixCommand,
        note: (
          <span>
            Command auto-escalates with <code>sudo</code> when available; otherwise run from a root
            shell. Creates <code>/Library/LaunchDaemons/com.pulse.agent.plist</code> and starts the
            agent automatically.
          </span>
        ),
      },
    ],
  },
  freebsd: {
    title: 'Install on FreeBSD / pfSense / OPNsense',
    description:
      'The unified installer downloads the FreeBSD binary and sets up an rc.d service for background monitoring.',
    snippets: [
      {
        label: 'Install with rc.d',
        command: unixCommand,
        note: (
          <span>
            Run as root. <strong>Note:</strong> pfSense/OPNsense don't include bash by default.
            Install it first: <code>pkg install bash</code>. Creates{' '}
            <code>/usr/local/etc/rc.d/pulse-agent</code> and starts the agent automatically.
          </span>
        ),
      },
    ],
  },
  windows: {
    title: 'Install on Windows',
    description:
      'Run the PowerShell script to install and configure the unified agent as a Windows service with automatic startup.',
    snippets: [
      {
        label: 'Install as Windows Service (PowerShell)',
        command: windowsInteractiveCommand,
        note: (
          <span>
            Run in PowerShell as Administrator. The script will prompt for the Pulse URL and API
            token, download the agent binary, and install it as a Windows service with automatic
            startup.
          </span>
        ),
      },
      {
        label: 'Install with parameters (PowerShell)',
        command: windowsParameterizedCommand,
        note: (
          <span>
            Non-interactive installation. Set environment variables before running to skip prompts.
          </span>
        ),
      },
    ],
  },
});

const agentCapabilityFromSurfaceKind = (kind: ConnectedInfrastructureSurface['kind']): AgentCapability => {
  switch (kind) {
    case 'agent':
    case 'docker':
    case 'kubernetes':
    case 'proxmox':
    case 'pbs':
    case 'pmg':
      return kind;
    default:
      return 'agent';
  }
};

const installFlagsForCapabilities = (capabilities: AgentCapability[]) => {
  const flags = new Set<string>();
  if (capabilities.includes('docker')) {
    flags.add('--enable-docker');
    flags.add('--disable-host');
  }
  if (capabilities.includes('kubernetes')) {
    flags.add('--enable-kubernetes');
  }
  if (capabilities.includes('proxmox')) {
    flags.add('--enable-proxmox');
    flags.add('--proxmox-type pve');
  } else if (capabilities.includes('pbs')) {
    flags.add('--enable-proxmox');
    flags.add('--proxmox-type pbs');
  }
  return Array.from(flags);
};

const surfaceBreakdownFromConnectedSurface = (
  surface: ConnectedInfrastructureSurface,
): UnifiedAgentSurface => ({
  key: surface.kind,
  kind: agentCapabilityFromSurfaceKind(surface.kind),
  label: surface.label || getCapabilitySurfaceLabel(agentCapabilityFromSurfaceKind(surface.kind)),
  detail: surface.detail || '',
  idLabel: surface.idLabel,
  idValue: surface.idValue,
  action: surface.action,
  controlId: surface.controlId,
});

export const rowFromConnectedInfrastructureItem = (
  item: ConnectedInfrastructureItem,
  scope: UnifiedAgentRow['scope'],
): UnifiedAgentRow => {
  const surfaces = item.surfaces.map(surfaceBreakdownFromConnectedSurface);
  const capabilities = Array.from(new Set(surfaces.map((surface) => surface.kind)));
  const agentSurface = item.surfaces.find((surface) => surface.kind === 'agent');
  const dockerSurface = item.surfaces.find((surface) => surface.kind === 'docker');
  const kubernetesSurface = item.surfaces.find((surface) => surface.kind === 'kubernetes');
  const name = item.name || item.displayName || item.hostname || item.id;
  const rowKey =
    item.status === 'ignored'
      ? item.surfaces[0]?.kind === 'docker'
        ? `removed-docker-${item.id}`
        : item.surfaces[0]?.kind === 'kubernetes'
          ? `removed-k8s-${item.id}`
          : `removed-host-${item.id}`
      : kubernetesSurface && !agentSurface && !dockerSurface
        ? `k8s-${kubernetesSurface.controlId || item.id}`
        : `agent-${item.id}`;
  return {
    rowKey,
    id: item.id,
    agentActionId: item.uninstallAgentId || agentSurface?.controlId,
    dockerActionId: dockerSurface?.controlId,
    kubernetesActionId: kubernetesSurface?.controlId,
    name,
    hostname: item.hostname,
    displayName: item.displayName,
    capabilities,
    status: item.status === 'ignored' ? 'removed' : 'active',
    healthStatus: item.healthStatus,
    lastSeen: item.lastSeen,
    removedAt: item.removedAt,
    version: item.version,
    isOutdatedBinary: item.isOutdatedBinary,
    linkedNodeId: item.linkedNodeId,
    commandsEnabled: item.commandsEnabled,
    agentId: item.scopeAgentId || item.uninstallAgentId,
    upgradePlatform: item.upgradePlatform || 'linux',
    scope,
    installFlags: installFlagsForCapabilities(capabilities),
    searchText: [
      name,
      item.displayName,
      item.hostname,
      item.id,
      item.scopeAgentId,
      item.uninstallAgentId,
      agentSurface?.controlId,
      dockerSurface?.controlId,
      kubernetesSurface?.controlId,
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase(),
    kubernetesInfo:
      kubernetesSurface || capabilities.includes('kubernetes')
        ? {
            server: undefined,
            context: undefined,
            tokenName: undefined,
          }
        : undefined,
    surfaces,
  };
};

export const getPowerShellInstallProfileEnvFromFlags = (flags: string[]) => {
  const envAssignments: string[] = [];
  for (let index = 0; index < flags.length; index += 1) {
    const flag = flags[index];
    switch (flag) {
      case '--enable-docker':
        envAssignments.push(`$env:PULSE_ENABLE_DOCKER="true"`);
        break;
      case '--disable-host':
        envAssignments.push(`$env:PULSE_ENABLE_HOST="false"`);
        break;
      case '--enable-kubernetes':
        envAssignments.push(`$env:PULSE_ENABLE_KUBERNETES="true"`);
        break;
      case '--enable-proxmox':
        envAssignments.push(`$env:PULSE_ENABLE_PROXMOX="true"`);
        break;
      case '--proxmox-type':
        if (typeof flags[index + 1] === 'string' && flags[index + 1].trim()) {
          envAssignments.push(`$env:PULSE_PROXMOX_TYPE="${flags[index + 1].trim()}"`);
          index += 1;
        }
        break;
      default:
        if (flag.startsWith('--proxmox-type ')) {
          const proxmoxType = flag.slice('--proxmox-type '.length).trim();
          if (proxmoxType) {
            envAssignments.push(`$env:PULSE_PROXMOX_TYPE="${proxmoxType}"`);
          }
        }
        break;
    }
  }
  return envAssignments;
};
