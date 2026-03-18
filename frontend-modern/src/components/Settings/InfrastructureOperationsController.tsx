import { Component, createSignal, Show, For, onMount, createEffect, createMemo } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { unwrap } from 'solid-js/store';
import { useWebSocket } from '@/App';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Dialog } from '@/components/shared/Dialog';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { SearchField } from '@/components/shared/SearchField';
import Server from 'lucide-solid/icons/server';
import Users from 'lucide-solid/icons/users';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import {
  MonitoringAPI,
  type RemovedDockerHost,
  type RemovedHostAgent,
  type RemovedKubernetesCluster,
} from '@/api/monitoring';
import {
  AgentProfilesAPI,
  MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE,
  type AgentProfile,
  type AgentProfileAssignment,
} from '@/api/agentProfiles';
import { SecurityAPI } from '@/api/security';
import { notificationStore } from '@/stores/notifications';
import { useResources } from '@/hooks/useResources';
import type { SecurityStatus } from '@/types/config';
import type {
  AgentLookupResponse,
  ConnectedInfrastructureItem,
  ConnectedInfrastructureSurface,
} from '@/types/api';
import type { APITokenRecord } from '@/api/security';
import {
  AGENT_REPORT_SCOPE,
  AGENT_CONFIG_READ_SCOPE,
  DOCKER_REPORT_SCOPE,
  KUBERNETES_REPORT_SCOPE,
  AGENT_EXEC_SCOPE,
} from '@/constants/apiScopes';
import { copyToClipboard } from '@/utils/clipboard';
import { getPulseBaseUrl } from '@/utils/url';
import { logger } from '@/utils/logger';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  buildPowerShellInstallScriptBootstrap,
  buildUnixAgentInstallCommand,
  buildWindowsAgentInstallCommand,
  resolveInstallerBaseUrl,
  powerShellQuote,
} from '@/utils/agentInstallCommand';
import {
  getPreferredNamedEntityLabel,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import {
  getActionableAgentIdFromResource,
  getActionableDockerRuntimeIdFromResource,
  getActionableKubernetesClusterIdFromResource,
  getExplicitAgentIdFromResource,
  getPlatformAgentRecord,
  getPlatformDataRecord,
  hasAgentFacet,
  hasDockerWorkloadsScope,
} from '@/utils/agentResources';
import {
  getAgentCapabilityBadgeClass,
  getAgentCapabilityLabel,
  type AgentCapability,
} from '@/utils/agentCapabilityPresentation';
import {
  ALLOW_RECONNECT_LABEL,
  getUnifiedAgentLookupStatusPresentation,
  getUnifiedAgentStatusPresentation,
  MONITORING_STOPPED_STATUS_LABEL,
} from '@/utils/unifiedAgentStatusPresentation';
import {
  getUnifiedAgentAllowReconnectErrorMessage,
  getUnifiedAgentAllowReconnectSuccessMessage,
  getUnifiedAgentClipboardCopyErrorMessage,
  getUnifiedAgentClipboardCopySuccessMessage,
  getInventorySubjectLabel,
  getMonitoringStoppedEmptyState,
  getRemovedUnifiedAgentItemLabel,
  getUnifiedAgentStopMonitoringErrorMessage,
  getUnifiedAgentStopMonitoringSuccessMessage,
  getUnifiedAgentStopMonitoringUnavailableMessage,
  getUnifiedAgentLastSeenLabel,
  getUnifiedAgentUninstallCommandCopiedMessage,
  getUnifiedAgentUpgradeCommandCopiedMessage,
} from '@/utils/unifiedAgentInventoryPresentation';
import {
  trackAgentInstallCommandCopied,
  trackAgentInstallProfileSelected,
  trackAgentInstallTokenGenerated,
} from '@/utils/upgradeMetrics';
import type { Resource } from '@/types/resource';

const TOKEN_PLACEHOLDER = '<api-token>';
const UNIFIED_AGENT_TELEMETRY_SURFACE = 'settings_unified_agents';

const buildDefaultTokenName = () => {
  const now = new Date();
  const iso = now.toISOString().slice(0, 16); // YYYY-MM-DDTHH:MM
  const stamp = iso.replace('T', ' ').replace(/:/g, '-');
  return `Agent ${stamp}`;
};

const normalizeTelemetryPart = (value: string) =>
  value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '');

const shellQuoteArg = (value: string) => `'${value.replace(/'/g, `'\"'\"'`)}'`;
type AgentPlatform = 'linux' | 'macos' | 'freebsd' | 'windows';
type UnifiedAgentStatus = 'active' | 'removed';
type ScopeCategory = 'default' | 'profile' | 'ai-managed' | 'na';
type InstallProfile = 'auto' | 'docker' | 'kubernetes' | 'proxmox-pve' | 'proxmox-pbs' | 'truenas';

type SetupHandoffState = {
  username: string;
  password: string;
  apiToken: string;
  createdAt?: string;
};

type UnifiedAgentRow = {
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
  surfaces: Array<{
    key: string;
    kind: AgentCapability;
    label: string;
    detail: string;
    idLabel?: string;
    idValue?: string;
    action?: 'stop-monitoring' | 'allow-reconnect';
    controlId?: string;
  }>;
};

type InventoryActionType = 'stop-monitoring' | 'allow-reconnect';

type InventoryActionNotice = {
  tone: 'success' | 'info';
  title: string;
  detail: string;
  showRecoveryQueueLink?: boolean;
};

type StopMonitoringDialogState = {
  row: UnifiedAgentRow;
  subject: string;
  scopeLabel: string;
};

const getCapabilitySurfaceLabel = (capability: AgentCapability) => {
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

const getReconnectActionLabel = (row: UnifiedAgentRow) => {
  if (row.capabilities.includes('docker')) {
    return 'Allow Docker reconnect';
  }
  if (row.capabilities.includes('kubernetes')) {
    return 'Allow Kubernetes reconnect';
  }
  return 'Allow host reconnect';
};

const joinHumanList = (parts: string[]) => {
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

const getRowReportingSummary = (row: UnifiedAgentRow) => {
  const reported = row.surfaces.map((surface, index) => sentenceCaseSurfaceLabel(surface.label, index));
  if (reported.length === 0) {
    return '';
  }
  return `Pulse is receiving ${joinHumanList(reported)} from this item.`;
};

const getRowSurfaceBreakdown = (row: UnifiedAgentRow) => {
  return row.surfaces;
};

const getStopMonitoringSurfaces = (row: UnifiedAgentRow) => {
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

const getStopMonitoringScopeLabel = (row: UnifiedAgentRow) => {
  const surfaceLabels = getStopMonitoringSurfaces(row).map((surface) => surface.label);
  if (surfaceLabels.length === 0) {
    return 'Reporting for this item';
  }
  return joinHumanList(surfaceLabels);
};

const createSurfaceScopedRow = (
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

const INSTALL_PROFILE_OPTIONS: {
  value: InstallProfile;
  label: string;
  description: string;
  flags: string[];
}[] = [
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

// Generate platform-specific commands with the appropriate Pulse URL
// Uses agentUrl from API (PULSE_PUBLIC_URL) if configured, otherwise falls back to window.location
const buildCommandsByPlatform = (
  unixCommand: string,
  windowsInteractiveCommand: string,
  windowsParameterizedCommand: string,
): Record<
  AgentPlatform,
  {
    title: string;
    description: string;
    snippets: { label: string; command: string; note?: string | any }[];
  }
> => ({
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

const surfaceBreakdownFromConnectedSurface = (surface: ConnectedInfrastructureSurface) => ({
  key: surface.kind,
  kind: agentCapabilityFromSurfaceKind(surface.kind),
  label: surface.label || getCapabilitySurfaceLabel(agentCapabilityFromSurfaceKind(surface.kind)),
  detail: surface.detail || '',
  idLabel: surface.idLabel,
  idValue: surface.idValue,
  action: surface.action,
  controlId: surface.controlId,
});

const rowFromConnectedInfrastructureItem = (
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

export interface InfrastructureOperationsControllerProps {
  embedded?: boolean;
  showInstaller?: boolean;
  showInventory?: boolean;
}

export const InfrastructureOperationsController: Component<
  InfrastructureOperationsControllerProps
> = (props) => {
  const { state } = useWebSocket();
  const { resources, mutate: mutateResources, refetch: refetchResources } = useResources();
  const navigate = useNavigate();

  const pd = (r: Resource) => {
    const platformData = getPlatformDataRecord(r);
    return platformData ? unwrap(platformData) : undefined;
  };
  const asRecord = (value: unknown) =>
    value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;
  const asBoolean = (value: unknown) => (typeof value === 'boolean' ? value : undefined);
  const platformAgent = (r: Resource) => asRecord(getPlatformAgentRecord(r));

  const getAgentId = (r: Resource) => getExplicitAgentIdFromResource(r);

  const getAgentActionId = (r: Resource) => getActionableAgentIdFromResource(r);

  const getDockerActionId = (r: Resource) => getActionableDockerRuntimeIdFromResource(r);

  const getKubernetesActionId = (r: Resource) => getActionableKubernetesClusterIdFromResource(r);

  const getCommandsEnabled = (r: Resource) => {
    const platformData = pd(r);
    return (
      r.agent?.commandsEnabled ??
      asBoolean(platformAgent(r)?.commandsEnabled) ??
      asBoolean(platformData?.commandsEnabled)
    );
  };

  let hasLoggedSecurityStatusError = false;

  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [latestRecord, setLatestRecord] = createSignal<APITokenRecord | null>(null);
  const [tokenName, setTokenName] = createSignal('');
  const [confirmedNoToken, setConfirmedNoToken] = createSignal(false);
  const [currentToken, setCurrentToken] = createSignal<string | null>(null);
  const [isGeneratingToken, setIsGeneratingToken] = createSignal(false);
  const [lookupValue, setLookupValue] = createSignal('');
  const [lookupResult, setLookupResult] = createSignal<AgentLookupResponse | null>(null);
  const [lookupError, setLookupError] = createSignal<string | null>(null);
  const [lookupLoading, setLookupLoading] = createSignal(false);
  const [insecureMode, setInsecureMode] = createSignal(false); // For self-signed certificates (issue #806)
  const [enableCommands, setEnableCommands] = createSignal(false); // Enable Pulse command execution (issue #903)
  const [installProfile, setInstallProfile] = createSignal<InstallProfile>('auto');
  const [customAgentUrl, setCustomAgentUrl] = createSignal('');
  const [customCaPath, setCustomCaPath] = createSignal('');
  const [setupHandoff, setSetupHandoff] = createSignal<SetupHandoffState | null>(null);
  const [profiles, setProfiles] = createSignal<AgentProfile[]>([]);
  const [assignments, setAssignments] = createSignal<AgentProfileAssignment[]>([]);
  // Track pending command config changes: agentId -> { desired value, timestamp }
  const [pendingCommandConfig, setPendingCommandConfig] = createSignal<
    Record<string, { enabled: boolean; timestamp: number }>
  >({});
  const [pendingScopeUpdates, setPendingScopeUpdates] = createSignal<Record<string, boolean>>({});
  const [pendingInventoryActions, setPendingInventoryActions] = createSignal<
    Record<string, InventoryActionType>
  >({});
  const [inventoryActionNotice, setInventoryActionNotice] =
    createSignal<InventoryActionNotice | null>(null);
  const [optimisticRemovedHostAgents, setOptimisticRemovedHostAgents] = createSignal<
    RemovedHostAgent[]
  >([]);
  const [optimisticRemovedDockerHosts, setOptimisticRemovedDockerHosts] = createSignal<
    RemovedDockerHost[]
  >([]);
  const [optimisticRemovedKubernetesClusters, setOptimisticRemovedKubernetesClusters] =
    createSignal<RemovedKubernetesCluster[]>([]);
  const [stopMonitoringDialog, setStopMonitoringDialog] =
    createSignal<StopMonitoringDialogState | null>(null);
  const [expandedRowKey, setExpandedRowKey] = createSignal<string | null>(null);
  const [selectedIgnoredRowKey, setSelectedIgnoredRowKey] = createSignal<string | null>(null);
  const [filterCapability, setFilterCapability] = createSignal<'all' | AgentCapability>('all');
  const [filterScope, setFilterScope] = createSignal<'all' | Exclude<ScopeCategory, 'na'>>('all');
  const [filterSearch, setFilterSearch] = createSignal('');
  let recoveryQueueSectionRef: HTMLDivElement | undefined;

  const loadAgentProfiles = async () => {
    try {
      const [profilesData, assignmentsData] = await Promise.all([
        AgentProfilesAPI.listProfiles(),
        AgentProfilesAPI.listAssignments(),
      ]);
      setProfiles(profilesData);
      setAssignments(assignmentsData);
    } catch (err) {
      logger.debug('Failed to load agent profiles', err);
      notificationStore.error(err instanceof Error ? err.message : 'Failed to load agent profiles');
    }
  };

  createEffect(() => {
    if (requiresToken()) {
      setConfirmedNoToken(false);
    }
  });

  // Use agentUrl from API (PULSE_PUBLIC_URL) if configured, otherwise fall back to window.location
  const agentUrl = () => securityStatus()?.agentUrl || getPulseBaseUrl();
  const selectedAgentUrl = () => resolveInstallerBaseUrl(customAgentUrl(), agentUrl());

  onMount(() => {
    if (typeof window === 'undefined') {
      return;
    }

    const rawSetupHandoff = sessionStorage.getItem(STORAGE_KEYS.SETUP_HANDOFF);
    if (rawSetupHandoff) {
      try {
        const parsed = JSON.parse(rawSetupHandoff) as Partial<SetupHandoffState>;
        if (parsed.username && parsed.password && parsed.apiToken) {
          setSetupHandoff({
            username: parsed.username,
            password: parsed.password,
            apiToken: parsed.apiToken,
            createdAt: parsed.createdAt,
          });
        } else {
          sessionStorage.removeItem(STORAGE_KEYS.SETUP_HANDOFF);
        }
      } catch (_err) {
        sessionStorage.removeItem(STORAGE_KEYS.SETUP_HANDOFF);
      }
    }

    const fetchSecurityStatus = async () => {
      try {
        const data = await SecurityAPI.getStatus();
        setSecurityStatus(data);
      } catch (err) {
        if (!hasLoggedSecurityStatusError) {
          hasLoggedSecurityStatusError = true;
          logger.error('Failed to load security status', err);
        }
      }
    };
    fetchSecurityStatus();
    void loadAgentProfiles();
  });

  const clearSetupHandoff = () => {
    if (typeof window !== 'undefined') {
      try {
        sessionStorage.removeItem(STORAGE_KEYS.SETUP_HANDOFF);
      } catch (_err) {
        // Ignore storage cleanup failures.
      }
    }
    setSetupHandoff(null);
  };

  const copySetupHandoffField = async (value: string, successMessage: string) => {
    const copied = await copyToClipboard(value);
    if (copied) {
      notificationStore.success(successMessage);
    } else {
      notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
    }
  };

  const downloadSetupHandoff = () => {
    const handoff = setupHandoff();
    if (!handoff) return;

    const baseUrl = getPulseBaseUrl();
    const content = `Pulse First-Run Credentials
============================
Generated: ${handoff.createdAt || new Date().toISOString()}

Web Login:
----------
URL: ${baseUrl}
Username: ${handoff.username}
Password: ${handoff.password}

Admin API Token:
----------------
${handoff.apiToken}

Canonical install workspace:
----------------------------
${baseUrl.replace(/\/$/, '')}/settings/infrastructure/install

Generate a scoped install token below before copying Unified Agent install commands.
`;

    const blob = new Blob([content], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `pulse-first-run-credentials-${Date.now()}.txt`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  const requiresToken = () => {
    const status = securityStatus();
    if (status) {
      return status.requiresAuth || status.apiTokenConfigured;
    }
    return true;
  };

  const hasToken = () => Boolean(currentToken());
  const commandsUnlocked = () => (requiresToken() ? hasToken() : hasToken() || confirmedNoToken());

  const acknowledgeNoToken = () => {
    if (requiresToken()) {
      notificationStore.info('Generate or select a token before continuing.', 4000);
      return;
    }
    setCurrentToken(null);
    setLatestRecord(null);
    setConfirmedNoToken(true);
    notificationStore.success('Confirmed install commands without an API token.', 3500);
  };

  const handleGenerateToken = async () => {
    if (isGeneratingToken()) return;

    setIsGeneratingToken(true);
    try {
      const desiredName = tokenName().trim() || buildDefaultTokenName();
      // Generate token with unified agent reporting scopes
      const scopes = [
        AGENT_REPORT_SCOPE,
        AGENT_CONFIG_READ_SCOPE,
        DOCKER_REPORT_SCOPE,
        KUBERNETES_REPORT_SCOPE,
        AGENT_EXEC_SCOPE,
      ];
      const { token, record } = await SecurityAPI.createToken(desiredName, scopes);

      setCurrentToken(token);
      setLatestRecord(record);
      setTokenName('');
      setConfirmedNoToken(false);
      trackAgentInstallTokenGenerated(UNIFIED_AGENT_TELEMETRY_SURFACE, 'manual');
      notificationStore.success(
        'Token generated with Agent config + reporting, Docker, and Kubernetes permissions.',
        4000,
      );
    } catch (err) {
      logger.error('Failed to generate agent token', err);
      notificationStore.error(
        'Failed to generate agent token. Confirm you are signed in as an administrator.',
        6000,
      );
    } finally {
      setIsGeneratingToken(false);
    }
  };

  const resolvedCommandToken = () => {
    if (requiresToken()) {
      return currentToken() || TOKEN_PLACEHOLDER;
    }
    return currentToken();
  };

  const handleLookup = async () => {
    const query = lookupValue().trim();
    setLookupError(null);

    if (!query) {
      setLookupResult(null);
      setLookupError('Enter a hostname or agent ID to check.');
      return;
    }

    setLookupLoading(true);
    try {
      const result = await MonitoringAPI.lookupAgent({ id: query, hostname: query });
      if (!result) {
        setLookupResult(null);
        setLookupError(`No agent has reported with "${query}" yet. Try again in a few seconds.`);
      } else {
        setLookupResult(result);
        setLookupError(null);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Agent lookup failed.';
      setLookupResult(null);
      setLookupError(message);
    } finally {
      setLookupLoading(false);
    }
  };

  const withPrivilegeEscalation = (command: string) => {
    if (!command.includes('| bash -s --')) return command;
    return command.replace(/\|\s*bash -s --([\s\S]*)$/, (_match, args: string) => {
      return `| { if [ "$(id -u)" -eq 0 ]; then bash -s --${args}; elif command -v sudo >/dev/null 2>&1; then sudo bash -s --${args}; else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi; }`;
    });
  };
  const selectedCustomCaPath = () => customCaPath().trim();
  const urlRequiresInstallerInsecure = (url: string) =>
    insecureMode() || url.trim().toLowerCase().startsWith('http://');
  const getInsecureFlag = (url: string) => (urlRequiresInstallerInsecure(url) ? ' --insecure' : '');
  const getCurlFlags = () => (insecureMode() ? '-kfsSL' : '-fsSL');
  const getShellCustomCaCurlFlag = () => {
    const caPath = selectedCustomCaPath();
    return caPath ? ` --cacert ${shellQuoteArg(caPath)}` : '';
  };
  const getShellCustomCaInstallerFlag = () => {
    const caPath = selectedCustomCaPath();
    return caPath ? ` --cacert ${shellQuoteArg(caPath)}` : '';
  };
  const getSelectedInstallProfile = () =>
    INSTALL_PROFILE_OPTIONS.find((option) => option.value === installProfile()) ??
    INSTALL_PROFILE_OPTIONS[0];
  const getInstallProfileFlags = () => getSelectedInstallProfile().flags;
  const getPowerShellInstallProfileEnvFromFlags = (flags: string[]) => {
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
  const getPowerShellInstallProfileEnv = () =>
    getPowerShellInstallProfileEnvFromFlags(getInstallProfileFlags());
  const getPowerShellTransportEnv = () => {
    const envAssignments: string[] = [];
    if (insecureMode()) {
      envAssignments.push(`$env:PULSE_INSECURE_SKIP_VERIFY="true"`);
    }
    if (selectedCustomCaPath()) {
      envAssignments.push(`$env:PULSE_CACERT="${powerShellQuote(selectedCustomCaPath())}"`);
    }
    return envAssignments;
  };
  const getPowerShellModeEnv = () => {
    const envAssignments = getPowerShellTransportEnv();
    if (enableCommands()) {
      envAssignments.push(`$env:PULSE_ENABLE_COMMANDS="true"`);
    }
    return envAssignments;
  };
  const getPowerShellInstallEnv = () => {
    const envAssignments = getPowerShellInstallProfileEnv();
    if (enableCommands()) {
      envAssignments.push(`$env:PULSE_ENABLE_COMMANDS="true"`);
    }
    return envAssignments;
  };
  const getInstallerExtraArgs = () => [
    ...getInstallProfileFlags(),
    ...(enableCommands() ? ['--enable-commands'] : []),
  ];
  const handleInstallProfileChange = (profile: InstallProfile) => {
    setInstallProfile(profile);
    trackAgentInstallProfileSelected(UNIFIED_AGENT_TELEMETRY_SURFACE, profile);
  };

  const getCanonicalUninstallAgentId = (row?: UnifiedAgentRow) =>
    row?.agentActionId?.trim() || row?.agentId?.trim() || '';
  const getCanonicalUninstallHostname = (row?: UnifiedAgentRow) => row?.hostname?.trim() || '';

  const getUninstallCommand = (row?: UnifiedAgentRow) => {
    const url = selectedAgentUrl();
    const token = resolvedCommandToken();
    const insecure = getInsecureFlag(url);
    const agentId = getCanonicalUninstallAgentId(row);
    const hostname = getCanonicalUninstallHostname(row);
    const baseArgs = token
      ? `--uninstall --url ${shellQuoteArg(url)} --token ${shellQuoteArg(token)}${insecure}${getShellCustomCaInstallerFlag()}`
      : `--uninstall --url ${shellQuoteArg(url)}${insecure}${getShellCustomCaInstallerFlag()}`;
    const identityArgs = `${agentId ? ` --agent-id ${shellQuoteArg(agentId)}` : ''}${hostname ? ` --hostname ${shellQuoteArg(hostname)}` : ''}`;
    return withPrivilegeEscalation(
      `curl ${getCurlFlags()}${getShellCustomCaCurlFlag()} ${shellQuoteArg(`${url}/install.sh`)} | bash -s -- ${baseArgs}${identityArgs}`,
    );
  };

  const getWindowsUninstallCommand = (row?: UnifiedAgentRow) => {
    const url = selectedAgentUrl();
    const token = resolvedCommandToken();
    const transportEnv = getPowerShellTransportEnv();
    const agentId = getCanonicalUninstallAgentId(row);
    const hostname = getCanonicalUninstallHostname(row);
    const identityEnv: string[] = [];
    if (agentId) {
      identityEnv.push(`$env:PULSE_AGENT_ID="${powerShellQuote(agentId)}"`);
    }
    if (hostname) {
      identityEnv.push(`$env:PULSE_HOSTNAME="${powerShellQuote(hostname)}"`);
    }
    const prefixParts = [...transportEnv, ...identityEnv];
    const prefix = prefixParts.length > 0 ? `${prefixParts.join('; ')}; ` : '';
    // Include URL and token for server notification (removes agent from dashboard)
    if (token) {
      return `${prefix}$env:PULSE_URL="${powerShellQuote(url)}"; $env:PULSE_TOKEN="${powerShellQuote(token)}"; $env:PULSE_UNINSTALL="true"; ${buildPowerShellInstallScriptBootstrap(url)}`;
    }
    return `${prefix}$env:PULSE_URL="${powerShellQuote(url)}"; $env:PULSE_UNINSTALL="true"; ${buildPowerShellInstallScriptBootstrap(url)}`;
  };

  const getPlatformUninstallCommand = (platform: AgentPlatform, row?: UnifiedAgentRow) => {
    if (platform === 'windows') {
      return getWindowsUninstallCommand(row);
    }
    return getUninstallCommand(row);
  };

  const commandSections = createMemo(() => {
    const url = selectedAgentUrl();
    const token = resolvedCommandToken();
    const unixCommand = buildUnixAgentInstallCommand({
      baseUrl: url,
      token,
      insecure: insecureMode(),
      caCertPath: selectedCustomCaPath(),
      extraArgs: getInstallerExtraArgs(),
    });
    const windowsInteractiveCommand = buildWindowsAgentInstallCommand({
      baseUrl: url,
      token: currentToken(),
      insecure: insecureMode(),
      caCertPath: selectedCustomCaPath(),
      extraEnvAssignments: getPowerShellInstallEnv(),
    });
    const windowsParameterizedCommand = buildWindowsAgentInstallCommand({
      baseUrl: url,
      token,
      insecure: insecureMode(),
      caCertPath: selectedCustomCaPath(),
      extraEnvAssignments: getPowerShellInstallEnv(),
    });
    const commands = buildCommandsByPlatform(
      unixCommand,
      windowsInteractiveCommand,
      windowsParameterizedCommand,
    );
    return Object.entries(commands).map(([platform, meta]) => ({
      platform: platform as AgentPlatform,
      ...meta,
    }));
  });

  const matchesRemovedAgent = (
    resource: Resource,
    ids: { agentId?: string; dockerId?: string },
    capabilities: AgentCapability[],
  ) => {
    if (capabilities.includes('agent') && ids.agentId) {
      const agentId = ids.agentId;
      if (
        resource.id === agentId ||
        getAgentId(resource) === agentId ||
        getAgentActionId(resource) === agentId
      ) {
        return true;
      }
    }

    if (capabilities.includes('docker') && ids.dockerId) {
      const dockerId = ids.dockerId;
      if (resource.id === dockerId || getDockerActionId(resource) === dockerId) {
        return true;
      }
    }

    return false;
  };

  const reconcileRemovedAgent = (
    ids: { agentId?: string; dockerId?: string },
    capabilities: AgentCapability[],
    row: UnifiedAgentRow,
  ) => {
    const removedAt = Date.now();
    if (capabilities.includes('agent') && ids.agentId) {
      setOptimisticRemovedHostAgents((prev) => [
        {
          id: ids.agentId!,
          hostname: row.hostname,
          displayName: row.displayName || row.name,
          removedAt,
        },
        ...prev.filter((item) => item.id !== ids.agentId),
      ]);
    }
    if (capabilities.includes('docker') && ids.dockerId) {
      setOptimisticRemovedDockerHosts((prev) => [
        {
          id: ids.dockerId!,
          hostname: row.hostname,
          displayName: row.displayName || row.name,
          removedAt,
        },
        ...prev.filter((item) => item.id !== ids.dockerId),
      ]);
    }
    mutateResources((prev) =>
      prev.filter((resource) => !matchesRemovedAgent(resource, ids, capabilities)),
    );
    void refetchResources().catch((err) => {
      logger.debug('Failed to refresh resources after agent removal', err);
    });
  };

  const reconcileRemovedKubernetesCluster = (clusterId: string, clusterName?: string) => {
    setOptimisticRemovedKubernetesClusters((prev) => [
      {
        id: clusterId,
        name: clusterName || clusterId,
        displayName: clusterName,
        removedAt: Date.now(),
      },
      ...prev.filter((item) => item.id !== clusterId),
    ]);
    mutateResources((prev) =>
      prev.filter(
        (resource) =>
          !(
            resource.type === 'k8s-cluster' &&
            (getKubernetesActionId(resource) === clusterId || resource.id === clusterId)
          ),
      ),
    );
    void refetchResources().catch((err) => {
      logger.debug('Failed to refresh resources after kubernetes removal', err);
    });
  };

  /**
   * All resources managed by an agent.
   * In v6, the backend already merges resources by identity — a PVE node with a
   * linked agent is a single resource of type "agent" with agent + proxmox data.
   * No frontend merge logic or type-flapping prevention needed.
   *
   * Includes docker-host resources that may lack the `agent` facet when the
   * agent's docker data is represented as a separate resource type.
   */
  const agentResources = createMemo(() => {
    return resources()
      .filter((r) => r.type !== 'k8s-cluster' && (hasAgentFacet(r) || hasDockerWorkloadsScope(r)))
      .sort((a, b) =>
        (getPreferredResourceHostname(a) || '').localeCompare(
          getPreferredResourceHostname(b) || '',
        ),
      );
  });

  const agentByActionId = createMemo(() => {
    const map = new Map<string, Resource>();
    // Only include resources with an agent facet (not docker-only resources)
    // to avoid polluting the command config sync lookup.
    for (const agentResource of agentResources()) {
      if (!hasAgentFacet(agentResource)) continue;
      const actionId = getAgentActionId(agentResource);
      if (!actionId || map.has(actionId)) continue;
      map.set(actionId, agentResource);
    }
    return map;
  });

  const profileById = createMemo(() => {
    const map = new Map<string, AgentProfile>();
    for (const profile of profiles()) {
      map.set(profile.id, profile);
    }
    return map;
  });

  const assignmentByAgent = createMemo(() => {
    const map = new Map<string, AgentProfileAssignment>();
    for (const assignment of assignments()) {
      map.set(assignment.agent_id, assignment);
    }
    return map;
  });

  const getScopeInfo = (agentId: string | undefined) => {
    if (!agentId) {
      return { label: 'N/A', detail: '', category: 'na' as const };
    }
    const assignment = assignmentByAgent().get(agentId);
    if (!assignment) {
      return { label: 'Default', detail: 'Auto-detect', category: 'default' as const };
    }
    const profile = profileById().get(assignment.profile_id);
    if (!profile) {
      return {
        label: 'Profile assigned',
        detail: assignment.profile_id,
        category: 'profile' as const,
      };
    }
    const name = profile.name || assignment.profile_id;
    const isAIManaged =
      profile.description?.toLowerCase().includes('pulse ai') ||
      name.toLowerCase().startsWith('ai scope');
    return isAIManaged
      ? { label: 'Patrol-managed', detail: name, category: 'ai-managed' as const }
      : { label: name, detail: 'Assigned profile', category: 'profile' as const };
  };

  const getProfileOptionLabel = (profileId: string) => {
    const profile = profileById().get(profileId);
    if (profile) {
      return profile.name || profile.id;
    }
    return `Missing profile (${profileId})`;
  };

  const updateScopeAssignment = async (
    agentId: string,
    profileId: string | null,
    agentName: string,
  ) => {
    if (!agentId) {
      return;
    }
    if (pendingScopeUpdates()[agentId]) {
      return;
    }

    setPendingScopeUpdates((prev) => ({ ...prev, [agentId]: true }));
    try {
      if (profileId) {
        await AgentProfilesAPI.assignProfile(agentId, profileId);
        setAssignments((prev) => {
          const updatedAt = new Date().toISOString();
          const next = prev.filter((a) => a.agent_id !== agentId);
          next.push({ agent_id: agentId, profile_id: profileId, updated_at: updatedAt });
          return next;
        });
        notificationStore.success(
          `Scope updated for ${agentName}. Restart the agent to apply changes.`,
        );
      } else {
        await AgentProfilesAPI.unassignProfile(agentId);
        setAssignments((prev) => prev.filter((a) => a.agent_id !== agentId));
        notificationStore.success(
          `Scope reset for ${agentName}. Restart the agent to apply changes.`,
        );
      }
    } catch (err) {
      logger.error('Failed to update agent scope', err);
      if (err instanceof Error && err.message === MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE) {
        await loadAgentProfiles();
      }
      notificationStore.error(
        err instanceof Error && err.message ? err.message : 'Failed to update agent scope',
      );
    } finally {
      setPendingScopeUpdates((prev) => {
        const next = { ...prev };
        delete next[agentId];
        return next;
      });
    }
  };

  const handleResetScope = async (agentId: string, agentName: string) => {
    if (
      !confirm(
        `Reset scope for ${agentName}? This removes any assigned profile and reverts to auto-detect.`,
      )
    )
      return;
    await updateScopeAssignment(agentId, null, agentName);
  };

  const toggleAgentDetails = (rowKey: string) => {
    setExpandedRowKey(rowKey);
  };

  const connectedInfrastructureItems = createMemo<ConnectedInfrastructureItem[]>(
    () => state.connectedInfrastructure,
  );
  const showInstaller = () => props.showInstaller ?? true;
  const showInventory = () => props.showInventory ?? true;

  const unifiedRows = createMemo<UnifiedAgentRow[]>(() => {
    const rows: UnifiedAgentRow[] = [];
    const optimisticHostIDs = new Set(
      optimisticRemovedHostAgents()
        .map((item) => item.id?.trim())
        .filter((id): id is string => Boolean(id)),
    );
    const optimisticDockerIDs = new Set(
      optimisticRemovedDockerHosts()
        .map((item) => item.id?.trim())
        .filter((id): id is string => Boolean(id)),
    );
    const optimisticKubernetesIDs = new Set(
      optimisticRemovedKubernetesClusters()
        .map((item) => item.id?.trim())
        .filter((id): id is string => Boolean(id)),
    );

    connectedInfrastructureItems().forEach((item) => {
      const optimisticFilteredItem: ConnectedInfrastructureItem =
        item.status === 'active'
          ? {
              ...item,
              surfaces: item.surfaces.filter((surface) => {
                const controlId = surface.controlId?.trim();
                if (!controlId) return true;
                if (surface.kind === 'agent') return !optimisticHostIDs.has(controlId);
                if (surface.kind === 'docker') return !optimisticDockerIDs.has(controlId);
                if (surface.kind === 'kubernetes') return !optimisticKubernetesIDs.has(controlId);
                return true;
              }),
            }
          : item;

      if (optimisticFilteredItem.status === 'active' && optimisticFilteredItem.surfaces.length === 0) {
        return;
      }

      rows.push(
        rowFromConnectedInfrastructureItem(
          optimisticFilteredItem,
          getScopeInfo(optimisticFilteredItem.scopeAgentId),
        ),
      );
    });

    const backendIgnoredSurfaceKeys = new Set(
      rows
        .filter((row) => row.status === 'removed')
        .flatMap((row) =>
          row.surfaces.map((surface) => `${surface.kind}:${surface.controlId || surface.idValue || row.id}`),
        ),
    );

    optimisticRemovedDockerHosts().forEach((runtime) => {
      const key = `docker:${runtime.id}`;
      if (backendIgnoredSurfaceKeys.has(key)) return;
      const name = getPreferredNamedEntityLabel(runtime);
      rows.push({
        rowKey: `removed-docker-${runtime.id}`,
        id: runtime.id,
        dockerActionId: runtime.id,
        name,
        hostname: runtime.hostname,
        displayName: runtime.displayName,
        capabilities: ['docker'],
        status: 'removed',
        removedAt: runtime.removedAt,
        upgradePlatform: 'linux',
        scope: getScopeInfo(undefined),
        installFlags: ['--enable-docker', '--disable-host'],
        searchText: [name, runtime.hostname, runtime.id].filter(Boolean).join(' ').toLowerCase(),
        surfaces: [
          {
            key: 'docker',
            kind: 'docker',
            label: 'Docker runtime data',
            detail: 'Pulse is blocking Docker runtime reports from this machine.',
            idLabel: 'Docker runtime ID',
            idValue: runtime.id,
            action: 'allow-reconnect',
            controlId: runtime.id,
          },
        ],
      });
    });

    optimisticRemovedHostAgents().forEach((host) => {
      const key = `agent:${host.id}`;
      if (backendIgnoredSurfaceKeys.has(key)) return;
      const name = getPreferredNamedEntityLabel(host);
      rows.push({
        rowKey: `removed-host-${host.id}`,
        id: host.id,
        agentActionId: host.id,
        name,
        hostname: host.hostname,
        displayName: host.displayName,
        capabilities: ['agent'],
        status: 'removed',
        removedAt: host.removedAt,
        upgradePlatform: 'linux',
        scope: getScopeInfo(undefined),
        installFlags: [],
        searchText: [name, host.hostname, host.id].filter(Boolean).join(' ').toLowerCase(),
        surfaces: [
          {
            key: 'agent',
            kind: 'agent',
            label: 'Host telemetry',
            detail: 'Pulse is blocking host telemetry from this machine.',
            idLabel: 'Agent ID',
            idValue: host.id,
            action: 'allow-reconnect',
            controlId: host.id,
          },
        ],
      });
    });

    optimisticRemovedKubernetesClusters().forEach((cluster) => {
      const key = `kubernetes:${cluster.id}`;
      if (backendIgnoredSurfaceKeys.has(key)) return;
      const name = getPreferredNamedEntityLabel(cluster);
      rows.push({
        rowKey: `removed-k8s-${cluster.id}`,
        id: cluster.id,
        kubernetesActionId: cluster.id,
        name,
        capabilities: ['kubernetes'],
        status: 'removed',
        removedAt: cluster.removedAt,
        upgradePlatform: 'linux',
        scope: getScopeInfo(undefined),
        installFlags: ['--enable-kubernetes'],
        searchText: [name, cluster.name, cluster.id].filter(Boolean).join(' ').toLowerCase(),
        surfaces: [
          {
            key: 'kubernetes',
            kind: 'kubernetes',
            label: 'Kubernetes cluster data',
            detail: 'Pulse is blocking Kubernetes telemetry for this cluster.',
            idLabel: 'Cluster ID',
            idValue: cluster.id,
            action: 'allow-reconnect',
            controlId: cluster.id,
          },
        ],
      });
    });

    rows.sort((a, b) => {
      if (a.status !== b.status) {
        return a.status === 'active' ? -1 : 1;
      }
      return a.name.localeCompare(b.name);
    });

    return rows;
  });

  const matchesInventoryFilters = (row: UnifiedAgentRow) => {
    const query = filterSearch().trim().toLowerCase();
    if (
      filterCapability() !== 'all' &&
      !row.capabilities.includes(filterCapability() as AgentCapability)
    ) {
      return false;
    }
    if (filterScope() !== 'all' && row.scope.category !== filterScope()) {
      return false;
    }
    if (query && !row.searchText.includes(query)) {
      return false;
    }
    return true;
  };

  const activeRows = createMemo(() => unifiedRows().filter((row) => row.status === 'active'));
  const monitoringStoppedRows = createMemo(() =>
    unifiedRows().filter((row) => row.status === 'removed'),
  );
  const linkedAgents = createMemo(() =>
    activeRows()
      .filter((row) => Boolean(row.linkedNodeId))
      .map((row) => ({
        id: row.id,
        hostname: row.hostname || row.name,
        displayName: row.displayName,
        linkedNodeId: row.linkedNodeId!,
        status: row.healthStatus || 'online',
        version: row.version,
        lastSeen: row.lastSeen,
      })),
  );
  const hasLinkedAgents = createMemo(() => linkedAgents().length > 0);
  const outdatedAgents = createMemo(() => activeRows().filter((row) => row.isOutdatedBinary));
  const hasOutdatedAgents = createMemo(() => outdatedAgents().length > 0);

  const filteredActiveRows = createMemo(() => activeRows().filter(matchesInventoryFilters));
  const filteredMonitoringStoppedRows = createMemo(() =>
    monitoringStoppedRows().filter(matchesInventoryFilters),
  );
  const selectedActiveRow = createMemo(() => {
    const selectedKey = expandedRowKey();
    return filteredActiveRows().find((row) => row.rowKey === selectedKey) || null;
  });
  const selectedIgnoredRow = createMemo(() => {
    const selectedKey = selectedIgnoredRowKey();
    return filteredMonitoringStoppedRows().find((row) => row.rowKey === selectedKey) || null;
  });

  const hasFilters = createMemo(() => {
    return (
      filterCapability() !== 'all' || filterScope() !== 'all' || filterSearch().trim().length > 0
    );
  });

  const hasMonitoringStoppedRows = createMemo(() => monitoringStoppedRows().length > 0);
  const showMonitoringStoppedSection = createMemo(() => {
    return hasMonitoringStoppedRows() || hasFilters();
  });
  const coverageSummary = createMemo(() => {
    const active = activeRows();
    return {
      agent: active.filter((row) => row.capabilities.includes('agent')).length,
      docker: active.filter((row) => row.capabilities.includes('docker')).length,
      kubernetes: active.filter((row) => row.capabilities.includes('kubernetes')).length,
      proxmox: active.filter((row) => row.capabilities.includes('proxmox')).length,
      pbs: active.filter((row) => row.capabilities.includes('pbs')).length,
      pmg: active.filter((row) => row.capabilities.includes('pmg')).length,
    };
  });

  const reportingCoverageSummaryText = createMemo(() => {
    const summary = coverageSummary();
    const activeClauses = [
      summary.agent > 0 ? `${summary.agent} host${summary.agent === 1 ? '' : 's'}` : null,
      summary.docker > 0
        ? `${summary.docker} Docker runtime${summary.docker === 1 ? '' : 's'}`
        : null,
      summary.kubernetes > 0
        ? `${summary.kubernetes} Kubernetes cluster${summary.kubernetes === 1 ? '' : 's'}`
        : null,
      summary.proxmox > 0
        ? `${summary.proxmox} Proxmox node${summary.proxmox === 1 ? '' : 's'}`
        : null,
      summary.pbs > 0 ? `${summary.pbs} PBS server${summary.pbs === 1 ? '' : 's'}` : null,
      summary.pmg > 0 ? `${summary.pmg} PMG server${summary.pmg === 1 ? '' : 's'}` : null,
    ].filter((value): value is string => Boolean(value));

    if (activeClauses.length === 0) {
      return 'Pulse is not currently receiving live infrastructure reports.';
    }

    return `Pulse is currently receiving live reports from ${joinHumanList(activeClauses)}.`;
  });

  const inventoryStatusSummaryText = createMemo(() => {
    const activeCount = filteredActiveRows().length;
    const recoveryCount = filteredMonitoringStoppedRows().length;
    const recoveryClause =
      recoveryCount > 0
        ? `${recoveryCount} item${recoveryCount === 1 ? ' is' : 's are'} currently ignored by Pulse`
        : 'nothing is currently ignored by Pulse';
    return `${activeCount} item${activeCount === 1 ? ' is' : 's are'} actively reporting right now, and ${recoveryClause}. Stopping monitoring in Pulse does not uninstall software on the remote system.`;
  });

  const resetFilters = () => {
    setFilterCapability('all');
    setFilterScope('all');
    setFilterSearch('');
  };

  const setInventoryActionPending = (
    rowKey: string,
    action: InventoryActionType,
    pending: boolean,
  ) => {
    setPendingInventoryActions((prev) => {
      const next = { ...prev };
      if (pending) {
        next[rowKey] = action;
      } else {
        delete next[rowKey];
      }
      return next;
    });
  };

  const getPendingInventoryAction = (rowKey: string) => pendingInventoryActions()[rowKey];

  const scrollToRecoveryQueue = () => {
    recoveryQueueSectionRef?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };

  const openStopMonitoringDialog = (row: UnifiedAgentRow) => {
    setStopMonitoringDialog({
      row,
      subject: getInventorySubjectLabel(row.name, 'this item'),
      scopeLabel: getStopMonitoringScopeLabel(row),
    });
  };

  const getUpgradeCommand = (row: UnifiedAgentRow) => {
    const token = resolvedCommandToken();
    const url = selectedAgentUrl();
    const agentId = getCanonicalUninstallAgentId(row);
    const hostname = getCanonicalUninstallHostname(row);
    if (row.upgradePlatform === 'windows') {
      const envAssignments = [
        ...getPowerShellInstallProfileEnvFromFlags(row.installFlags),
        ...getPowerShellModeEnv(),
      ];
      if (agentId) {
        envAssignments.push(`$env:PULSE_AGENT_ID="${powerShellQuote(agentId)}"`);
      }
      if (hostname) {
        envAssignments.push(`$env:PULSE_HOSTNAME="${powerShellQuote(hostname)}"`);
      }
      const prefix = envAssignments.length > 0 ? `${envAssignments.join('; ')}; ` : '';
      const tokenEnv = token ? `$env:PULSE_TOKEN="${powerShellQuote(token)}"; ` : '';
      return `${prefix}$env:PULSE_URL="${powerShellQuote(url)}"; ${tokenEnv}${buildPowerShellInstallScriptBootstrap(url)}`;
    }
    let command = `curl ${getCurlFlags()}${getShellCustomCaCurlFlag()} ${shellQuoteArg(`${url}/install.sh`)} | bash -s -- --url ${shellQuoteArg(url)}`;
    if (token) {
      command += ` --token ${shellQuoteArg(token)}`;
    }
    if (row.installFlags.length > 0) {
      command += ` ${row.installFlags.join(' ')}`;
    }
    if (urlRequiresInstallerInsecure(url)) {
      command += getInsecureFlag(url);
    }
    command += getShellCustomCaInstallerFlag();
    if (agentId) {
      command += ` --agent-id ${shellQuoteArg(agentId)}`;
    }
    if (hostname) {
      command += ` --hostname ${shellQuoteArg(hostname)}`;
    }
    return withPrivilegeEscalation(command);
  };

  const handleRemoveAgent = async (row: UnifiedAgentRow) => {
    const subject = getInventorySubjectLabel(row.name, 'this host');

    setInventoryActionNotice(null);
    setInventoryActionPending(row.rowKey, 'stop-monitoring', true);
    try {
      let removed = false;
      // Remove the agent registration
      if (row.capabilities.includes('agent') && row.agentActionId) {
        await MonitoringAPI.deleteAgent(row.agentActionId);
        removed = true;
      }
      // Remove docker runtime registration if present
      if (row.capabilities.includes('docker') && row.dockerActionId) {
        await MonitoringAPI.deleteDockerRuntime(row.dockerActionId, { force: true });
        removed = true;
      }
      if (removed) {
        reconcileRemovedAgent(
          { agentId: row.agentActionId, dockerId: row.dockerActionId },
          row.capabilities,
          row,
        );
        setInventoryActionNotice({
          tone: 'success',
          title: `Monitoring stopped for ${subject}`,
          detail:
            'Pulse removed it from active reporting and will ignore new reports until you allow reconnect. You can review it in Ignored by Pulse below.',
          showRecoveryQueueLink: true,
        });
        notificationStore.success(getUnifiedAgentStopMonitoringSuccessMessage(subject));
      } else {
        notificationStore.error(getUnifiedAgentStopMonitoringUnavailableMessage());
      }
    } catch (err) {
      logger.error('Failed to stop monitoring host', err);
      notificationStore.error(getUnifiedAgentStopMonitoringErrorMessage(subject));
    } finally {
      setInventoryActionPending(row.rowKey, 'stop-monitoring', false);
      setStopMonitoringDialog(null);
    }
  };

  const handleAllowHostReconnect = async (row: UnifiedAgentRow) => {
    const agentId = row.agentActionId || row.id;
    const subject = getInventorySubjectLabel(row.displayName || row.hostname || row.name, agentId);
    setInventoryActionNotice(null);
    setInventoryActionPending(row.rowKey, 'allow-reconnect', true);
    try {
      await MonitoringAPI.allowHostAgentReenroll(agentId);
      setOptimisticRemovedHostAgents((prev) => prev.filter((item) => item.id !== agentId));
      setInventoryActionNotice({
        tone: 'info',
        title: `Reconnect allowed for ${subject}`,
        detail: 'Pulse will accept reports from it again the next time it checks in.',
      });
      notificationStore.success(getUnifiedAgentAllowReconnectSuccessMessage(subject));
    } catch (err) {
      logger.error('Failed to allow reconnect for host agent', err);
      notificationStore.error(getUnifiedAgentAllowReconnectErrorMessage(subject));
    } finally {
      setInventoryActionPending(row.rowKey, 'allow-reconnect', false);
    }
  };

  const handleAllowDockerReconnect = async (row: UnifiedAgentRow) => {
    const agentId = row.dockerActionId || row.id;
    const subject = getInventorySubjectLabel(row.displayName || row.hostname || row.name, agentId);
    setInventoryActionNotice(null);
    setInventoryActionPending(row.rowKey, 'allow-reconnect', true);
    try {
      await MonitoringAPI.allowDockerRuntimeReenroll(agentId);
      setOptimisticRemovedDockerHosts((prev) => prev.filter((item) => item.id !== agentId));
      setInventoryActionNotice({
        tone: 'info',
        title: `Reconnect allowed for ${subject}`,
        detail: 'Pulse will accept reports from it again the next time it checks in.',
      });
      notificationStore.success(getUnifiedAgentAllowReconnectSuccessMessage(subject));
    } catch (err) {
      logger.error('Failed to allow reconnect for docker runtime', err);
      notificationStore.error(getUnifiedAgentAllowReconnectErrorMessage(subject));
    } finally {
      setInventoryActionPending(row.rowKey, 'allow-reconnect', false);
    }
  };

  const handleRemoveKubernetesCluster = async (row: UnifiedAgentRow) => {
    const clusterId = row.kubernetesActionId || row.id;
    const subject = getInventorySubjectLabel(row.name, 'this cluster');

    setInventoryActionNotice(null);
    setInventoryActionPending(row.rowKey, 'stop-monitoring', true);
    try {
      await MonitoringAPI.deleteKubernetesCluster(clusterId);
      reconcileRemovedKubernetesCluster(clusterId, row.name);
      setInventoryActionNotice({
        tone: 'success',
        title: `Monitoring stopped for ${subject}`,
        detail:
          'Pulse removed it from active reporting and will ignore new reports until you allow reconnect. You can review it in Ignored by Pulse below.',
        showRecoveryQueueLink: true,
      });
      notificationStore.success(getUnifiedAgentStopMonitoringSuccessMessage(subject));
    } catch (err) {
      logger.error('Failed to stop monitoring kubernetes cluster', err);
      notificationStore.error(getUnifiedAgentStopMonitoringErrorMessage(subject));
    } finally {
      setInventoryActionPending(row.rowKey, 'stop-monitoring', false);
      setStopMonitoringDialog(null);
    }
  };

  const handleAllowKubernetesReconnect = async (row: UnifiedAgentRow) => {
    const clusterId = row.kubernetesActionId || row.id;
    const subject = getInventorySubjectLabel(row.name, clusterId);
    setInventoryActionNotice(null);
    setInventoryActionPending(row.rowKey, 'allow-reconnect', true);
    try {
      await MonitoringAPI.allowKubernetesClusterReenroll(clusterId);
      setOptimisticRemovedKubernetesClusters((prev) =>
        prev.filter((item) => item.id !== clusterId),
      );
      setInventoryActionNotice({
        tone: 'info',
        title: `Reconnect allowed for ${subject}`,
        detail: 'Pulse will accept reports from it again the next time it checks in.',
      });
      notificationStore.success(getUnifiedAgentAllowReconnectSuccessMessage(subject));
    } catch (err) {
      logger.error('Failed to allow reconnect for kubernetes cluster', err);
      notificationStore.error(getUnifiedAgentAllowReconnectErrorMessage(subject));
    } finally {
      setInventoryActionPending(row.rowKey, 'allow-reconnect', false);
    }
  };

  // Clear pending state when agent reports matching the expected value, or after timeout
  createEffect(() => {
    const pending = pendingCommandConfig();
    const agents = agentByActionId();
    const now = Date.now();
    const TIMEOUT_MS = 2 * 60 * 1000; // 2 minutes

    // Check if any pending config now matches the reported state or has timed out
    let updated = false;
    const newPending = { ...pending };
    const timedOut: string[] = [];

    for (const agentId of Object.keys(pending)) {
      const entry = pending[agentId];
      const agent = agents.get(agentId);

      const agentCommandsEnabled = agent ? getCommandsEnabled(agent) : undefined;
      if (
        agent &&
        typeof agentCommandsEnabled === 'boolean' &&
        agentCommandsEnabled === entry.enabled
      ) {
        // Agent confirmed the change
        delete newPending[agentId];
        updated = true;
      } else if (now - entry.timestamp > TIMEOUT_MS) {
        // Timed out waiting for agent
        delete newPending[agentId];
        const agentLabel = agent ? agent.identity?.hostname || agent.name || agentId : agentId;
        timedOut.push(agentLabel);
        updated = true;
      }
    }

    if (updated) {
      setPendingCommandConfig(newPending);
      if (timedOut.length > 0) {
        notificationStore.warning(
          `Config sync timed out for ${timedOut.join(', ')}. Agent may be offline.`,
        );
      }
    }
  });

  createEffect(() => {
    const rows = filteredActiveRows();
    const selectedKey = expandedRowKey();

    if (rows.length === 0) {
      if (selectedKey !== null) {
        setExpandedRowKey(null);
      }
      return;
    }

    if (selectedKey && !rows.some((row) => row.rowKey === selectedKey)) {
      setExpandedRowKey(null);
    }
  });

  createEffect(() => {
    const rows = filteredMonitoringStoppedRows();
    const selectedKey = selectedIgnoredRowKey();

    if (rows.length === 0) {
      if (selectedKey !== null) {
        setSelectedIgnoredRowKey(null);
      }
      return;
    }

    if (selectedKey && !rows.some((row) => row.rowKey === selectedKey)) {
      setSelectedIgnoredRowKey(null);
    }
  });

  const reportingColumns = [
    {
      key: 'name',
      label: 'Name',
      render: (row: UnifiedAgentRow) => {
        const selected = () => expandedRowKey() === row.rowKey;
        const agentName = row.displayName || row.hostname || row.name;
        const reportingSummary = getRowReportingSummary(row);
        return (
          <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
            <div class="min-w-0 text-left">
              <div class="truncate text-sm font-medium text-base-content">{row.name}</div>
              <Show when={row.displayName && row.hostname && row.displayName !== row.hostname}>
                <div class="truncate text-xs text-muted">{row.hostname}</div>
              </Show>
              <Show when={reportingSummary}>
                <div class="mt-1 line-clamp-2 text-xs text-muted">{reportingSummary}</div>
              </Show>
            </div>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                toggleAgentDetails(row.rowKey);
              }}
              class={`inline-flex min-h-10 items-center justify-center rounded-md px-2.5 py-1.5 text-xs font-medium sm:min-h-9 ${
                selected()
                  ? 'bg-blue-600 text-white shadow-sm hover:bg-blue-700'
                  : 'text-muted hover:bg-surface hover:text-base-content'
              }`}
              aria-label={`View details for ${agentName}`}
              aria-pressed={selected()}
              aria-controls={`agent-details-${row.rowKey}`}
            >
              {selected() ? 'Open details' : 'View details'}
            </button>
          </div>
        );
      },
    },
    {
      key: 'capabilities',
      label: 'Reporting surfaces',
      render: (row: UnifiedAgentRow) => (
        <div class="space-y-1 text-xs">
          <For each={row.capabilities}>
            {(cap) => (
              <div class="flex items-center gap-2">
                <span
                  class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${getAgentCapabilityBadgeClass(cap)}`}
                >
                  {getCapabilitySurfaceLabel(cap)}
                </span>
              </div>
            )}
          </For>
        </div>
      ),
    },
    {
      key: 'status',
      label: 'Status',
      render: (row: UnifiedAgentRow) => {
        const statusPresentation = () =>
          getUnifiedAgentStatusPresentation(row.status, row.healthStatus);
        return (
          <span
            class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${statusPresentation().badgeClass}`}
          >
            {statusPresentation().label}
          </span>
        );
      },
    },
    {
      key: 'lastSeen',
      label: 'Last Seen',
      render: (row: UnifiedAgentRow) => (
        <span class="text-xs text-muted">
          {getUnifiedAgentLastSeenLabel(row, MONITORING_STOPPED_STATUS_LABEL)}
        </span>
      ),
    },
    {
      key: 'actions',
      label: 'Actions',
      align: 'right' as const,
      render: (row: UnifiedAgentRow) => {
        const isRemoved = () => row.status === 'removed';
        const isKubernetes = () =>
          row.capabilities.includes('kubernetes') && !row.capabilities.includes('agent');
        const pendingAction = () => getPendingInventoryAction(row.rowKey);
        const isStopping = () => pendingAction() === 'stop-monitoring';
        const isAllowingReconnect = () => pendingAction() === 'allow-reconnect';
        const canRemove = () => {
          const needsAgent = row.capabilities.includes('agent') && !row.agentActionId;
          const needsDocker =
            row.capabilities.includes('docker') && !row.dockerActionId && !row.agentActionId;
          return !needsAgent && !needsDocker;
        };
        return (
          <Show
            when={isRemoved()}
            fallback={
              <Show
                when={isKubernetes()}
                fallback={
                  <button
                    onClick={() => openStopMonitoringDialog(row)}
                    disabled={!canRemove() || Boolean(pendingAction())}
                    title={!canRemove() ? 'Agent ID unavailable for removal' : undefined}
                    class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 hover:text-red-900 disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400 dark:hover:bg-red-900 dark:hover:text-red-300"
                  >
                    {isStopping() ? 'Stopping…' : 'Stop monitoring'}
                  </button>
                }
              >
                <button
                  onClick={() => openStopMonitoringDialog(row)}
                  disabled={Boolean(pendingAction())}
                  class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 hover:text-red-900 dark:text-red-400 dark:hover:bg-red-900 dark:hover:text-red-300"
                >
                  {isStopping() ? 'Stopping…' : 'Stop monitoring'}
                </button>
              </Show>
            }
          >
            <Show
              when={row.capabilities.includes('docker')}
              fallback={
                <Show
                  when={row.capabilities.includes('kubernetes')}
                  fallback={
                    <button
                      onClick={() => handleAllowHostReconnect(row)}
                      disabled={Boolean(pendingAction())}
                      class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-blue-600 hover:bg-blue-50 hover:text-blue-900 dark:text-blue-400 dark:hover:bg-blue-900 dark:hover:text-blue-300"
                    >
                      {isAllowingReconnect()
                        ? 'Allowing reconnect…'
                        : ALLOW_RECONNECT_LABEL}
                    </button>
                  }
                >
                  <button
                    onClick={() => handleAllowKubernetesReconnect(row)}
                    disabled={Boolean(pendingAction())}
                    class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-blue-600 hover:bg-blue-50 hover:text-blue-900 dark:text-blue-400 dark:hover:bg-blue-900 dark:hover:text-blue-300"
                  >
                    {isAllowingReconnect()
                      ? 'Allowing reconnect…'
                      : ALLOW_RECONNECT_LABEL}
                  </button>
                </Show>
              }
            >
              <button
                onClick={() => handleAllowDockerReconnect(row)}
                disabled={Boolean(pendingAction())}
                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-blue-600 hover:bg-blue-50 hover:text-blue-900 dark:text-blue-400 dark:hover:bg-blue-900 dark:hover:text-blue-300"
              >
                {isAllowingReconnect() ? 'Allowing reconnect…' : ALLOW_RECONNECT_LABEL}
              </button>
            </Show>
          </Show>
        );
      },
    },
  ];

  const renderSelectedActiveRowDetails = (
    rowAccessor: () => UnifiedAgentRow,
  ) => {
    const row = () => rowAccessor();
    const isKubernetes = () =>
      row().capabilities.includes('kubernetes') && !row().capabilities.includes('agent');
    const resolvedAgentId = () => row().agentId || '';
    const assignment = () =>
      resolvedAgentId() ? assignmentByAgent().get(resolvedAgentId()) : undefined;
    const isScopeUpdating = () =>
      resolvedAgentId() ? Boolean(pendingScopeUpdates()[resolvedAgentId()]) : false;
    const agentName = () => row().displayName || row().hostname || row().name;
    const surfaces = () => getRowSurfaceBreakdown(row());

    const renderHeader = () => (
      <div class="border-b border-border bg-surface-alt px-4 py-4">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0 space-y-3">
            <div>
              <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                Selected reporting item
              </div>
              <div class="mt-2 text-lg font-semibold text-base-content">{row().name}</div>
              <Show
                when={
                  row().displayName && row().hostname && row().displayName !== row().hostname
                }
              >
                <div class="mt-1 text-xs text-muted">{row().hostname}</div>
              </Show>
              <div class="mt-2 text-sm text-base-content">
                Use surface controls to stop specific reporting without removing the machine.
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-xs">
              <For each={row().capabilities}>
                {(cap) => (
                  <span
                    class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${getAgentCapabilityBadgeClass(cap)}`}
                  >
                    {getAgentCapabilityLabel(cap)}
                  </span>
                )}
              </For>
              <Show when={row().isOutdatedBinary}>
                <span class="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                  Outdated
                </span>
              </Show>
              <Show when={row().linkedNodeId}>
                <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900 dark:text-blue-300">
                  Linked
                </span>
              </Show>
            </div>
          </div>
          <button
            type="button"
            onClick={() => setExpandedRowKey(null)}
            class="rounded-md p-1 hover:bg-surface-hover hover:text-base-content"
            aria-label="Close"
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>
      </div>
    );

    const renderMachineOverview = () => (
      <div class="rounded-lg border border-border bg-surface-alt px-4 py-4">
        <div class="text-xs font-semibold uppercase tracking-wide text-muted">Machine overview</div>
        <div class="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-1">
          <div class="space-y-2 text-xs text-muted">
            <div>
              Item ID: <span class="font-mono text-base-content">{row().id}</span>
            </div>
            <Show when={row().agentActionId && row().agentActionId !== row().id}>
              <div>
                Agent ID: <span class="font-mono text-base-content">{row().agentActionId}</span>
              </div>
            </Show>
            <Show when={row().dockerActionId && row().dockerActionId !== row().id}>
              <div>
                Container Agent ID:{' '}
                <span class="font-mono text-base-content">{row().dockerActionId}</span>
              </div>
            </Show>
            <Show when={row().kubernetesActionId && row().kubernetesActionId !== row().id}>
              <div>
                Cluster ID:{' '}
                <span class="font-mono text-base-content">{row().kubernetesActionId}</span>
              </div>
            </Show>
            <Show when={row().agentId && row().agentId !== row().id}>
              <div>
                Reporting agent ID:{' '}
                <span class="font-mono text-base-content">{row().agentId}</span>
              </div>
            </Show>
            <Show when={row().linkedNodeId}>
              <div>
                Linked node ID:{' '}
                <span class="font-mono text-base-content">{row().linkedNodeId}</span>
              </div>
            </Show>
          </div>
          <div class="space-y-2 text-xs text-muted">
            <Show when={row().lastSeen}>
              <div>
                Last seen {formatRelativeTime(row().lastSeen!)} (
                {formatAbsoluteTime(row().lastSeen!)})
              </div>
            </Show>
            <Show when={row().scope.category !== 'na'}>
              <div>
                <div class="mb-1">Scope profile</div>
                <Show
                  when={resolvedAgentId()}
                  fallback={
                    <span class="text-base-content" title={row().scope.detail}>
                      {row().scope.label}
                    </span>
                  }
                >
                  <Show
                    when={isKubernetes()}
                    fallback={
                      <Show
                        when={profiles().length > 0}
                        fallback={
                          <span class="text-base-content" title={row().scope.detail}>
                            {row().scope.label}
                          </span>
                        }
                      >
                        <div class="flex items-center gap-2">
                          <select
                            value={assignment()?.profile_id || ''}
                            onChange={(event) => {
                              const nextValue = event.currentTarget.value;
                              const currentValue = assignment()?.profile_id || '';
                              if (nextValue === currentValue) {
                                return;
                              }
                              void updateScopeAssignment(
                                resolvedAgentId(),
                                nextValue || null,
                                agentName(),
                              );
                            }}
                            disabled={isScopeUpdating()}
                            class="min-h-10 sm:min-h-9 rounded-md border border-border bg-surface px-2.5 py-1.5 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                          >
                            <option value="">Default (Auto-detect)</option>
                            <Show
                              when={
                                assignment()?.profile_id &&
                                !profileById().has(assignment()!.profile_id)
                              }
                            >
                              <option value={assignment()!.profile_id}>
                                {getProfileOptionLabel(assignment()!.profile_id)}
                              </option>
                            </Show>
                            <For each={profiles()}>
                              {(profile) => (
                                <option value={profile.id}>
                                  {getProfileOptionLabel(profile.id)}
                                </option>
                              )}
                            </For>
                          </select>
                          <Show when={isScopeUpdating()}>
                            <span class="text-[10px] text-muted">Updating…</span>
                          </Show>
                        </div>
                      </Show>
                    }
                  >
                    <span class="text-base-content" title={row().scope.detail}>
                      {row().scope.label}
                    </span>
                  </Show>
                </Show>
              </div>
            </Show>
            <Show
              when={
                row().kubernetesInfo &&
                (row().kubernetesInfo?.server ||
                  row().kubernetesInfo?.context ||
                  row().kubernetesInfo?.tokenName)
              }
            >
              <div class="space-y-1 pt-1">
                <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                  Kubernetes connection
                </div>
                <Show when={row().kubernetesInfo?.server}>
                  <div>
                    Server:{' '}
                    <span class="text-base-content">{row().kubernetesInfo?.server}</span>
                  </div>
                </Show>
                <Show when={row().kubernetesInfo?.context}>
                  <div>
                    Context:{' '}
                    <span class="text-base-content">{row().kubernetesInfo?.context}</span>
                  </div>
                </Show>
                <Show when={row().kubernetesInfo?.tokenName}>
                  <div>
                    Token:{' '}
                    <span class="text-base-content">{row().kubernetesInfo?.tokenName}</span>
                  </div>
                </Show>
              </div>
            </Show>
          </div>
        </div>
        <Show when={assignment()}>
          <div class="mt-3 border-t border-border pt-3">
            <div class="text-[11px] text-amber-600 dark:text-amber-400">
              Restart required to apply scope changes.
            </div>
            <button
              type="button"
              onClick={() => handleResetScope(resolvedAgentId(), agentName() || resolvedAgentId())}
              class="mt-2 text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300 text-left"
            >
              Reset to default
            </button>
          </div>
        </Show>
      </div>
    );

    const renderSurfaceControls = () => (
      <Show when={surfaces().length > 0}>
        <div class="rounded-lg border border-border bg-surface-alt px-4 py-4">
          <div class="flex flex-col gap-1">
            <div class="text-xs font-semibold uppercase tracking-wide text-muted">
              Surface controls
            </div>
            <div class="text-xs text-muted">
              Stop a specific surface. Other surfaces keep reporting.
            </div>
          </div>
          <div class="mt-3 overflow-hidden rounded-md border border-border">
            <div class="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1.35fr)_minmax(0,0.9fr)_auto] gap-0 border-b border-border bg-surface px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
              <div>Surface</div>
              <div>What Pulse receives</div>
              <div>ID</div>
              <div class="text-right">Control</div>
            </div>
            <For each={surfaces()}>
              {(surface) => (
                <div class="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1.35fr)_minmax(0,0.9fr)_auto] gap-0 border-b border-border bg-surface-alt px-3 py-2 text-xs last:border-b-0">
                  <div class="pr-3 font-medium text-base-content">{surface.label}</div>
                  <div class="pr-3 text-muted">{surface.detail}</div>
                  <div class="text-muted">
                    <Show
                      when={surface.idLabel && surface.idValue}
                      fallback={<span class="text-muted">Not separately addressed</span>}
                    >
                      <div class="space-y-1">
                        <div class="text-[11px]">{surface.idLabel}</div>
                        <div class="font-mono text-base-content">{surface.idValue}</div>
                      </div>
                    </Show>
                  </div>
                  <div class="pl-3 text-right">
                    <Show
                      when={
                        surface.key === 'docker' ||
                        surface.key === 'agent' ||
                        surface.key === 'kubernetes'
                      }
                      fallback={
                        <span class="text-[11px] text-muted">Managed with host telemetry</span>
                      }
                    >
                      <button
                        type="button"
                        data-row-action
                        onClick={(event) => {
                          event.stopPropagation();
                          openStopMonitoringDialog(
                            createSurfaceScopedRow(
                              row(),
                              surface.key as 'agent' | 'docker' | 'kubernetes',
                            ),
                          );
                        }}
                        class="inline-flex min-h-9 items-center rounded-md px-2.5 py-1.5 text-xs font-medium text-red-600 hover:bg-red-50 hover:text-red-900 dark:text-red-400 dark:hover:bg-red-900 dark:hover:text-red-300"
                      >
                        Stop this surface
                      </button>
                    </Show>
                  </div>
                </div>
              )}
            </For>
          </div>
        </div>
      </Show>
    );

    const renderMachineActions = () => (
      <div class="rounded-lg border border-border bg-surface-alt px-4 py-4">
        <div class="text-xs font-semibold uppercase tracking-wide text-muted">Machine actions</div>
        <div class="mt-2 text-xs text-muted">Machine-level utilities.</div>
        <div class="mt-4 flex flex-col gap-2">
          <Show when={!isKubernetes()}>
            <button
              type="button"
              onClick={async () => {
                const cmd = getPlatformUninstallCommand(row().upgradePlatform, row());
                const success = await copyToClipboard(cmd);
                if (success) {
                  notificationStore.success(getUnifiedAgentUninstallCommandCopiedMessage());
                } else {
                  notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
                }
              }}
              class="rounded-md border border-border px-3 py-2 text-left text-xs text-slate-600 hover:bg-surface hover:text-base-content"
            >
              Copy uninstall command
            </button>
          </Show>
          <Show when={row().isOutdatedBinary}>
            <button
              type="button"
              onClick={async () => {
                const success = await copyToClipboard(getUpgradeCommand(row()));
                if (success) {
                  notificationStore.success(getUnifiedAgentUpgradeCommandCopiedMessage());
                } else {
                  notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
                }
              }}
              class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-left text-xs text-amber-700 hover:bg-amber-100 hover:text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300 dark:hover:bg-amber-900/60 dark:hover:text-amber-200"
            >
              Copy upgrade command
            </button>
          </Show>
          <div class="rounded-md border border-blue-200 bg-blue-50 px-3 py-3 text-xs text-blue-900 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-100">
            Use surface controls above to stop reporting without uninstalling.
          </div>
        </div>
      </div>
    );

    return (
      <div id={`agent-details-${row().rowKey}`} class="flex h-full flex-col overflow-y-auto">
        {renderHeader()}
        <div class="space-y-4 p-4 text-sm text-muted">
          {renderMachineOverview()}
          {renderSurfaceControls()}
          {renderMachineActions()}
        </div>
      </div>
    );
  };

  const renderSelectedIgnoredRowDetails = (
    rowAccessor: () => UnifiedAgentRow,
  ) => {
    const row = () => rowAccessor();
    const pendingAction = () => getPendingInventoryAction(row().rowKey);
    const isAllowingReconnect = () => pendingAction() === 'allow-reconnect';
    const reconnectLabel = () => getReconnectActionLabel(row());
    const blockedId = () =>
      row().dockerActionId || row().kubernetesActionId || row().agentActionId || row().id;

    const renderHeader = () => (
      <div class="border-b border-amber-200 bg-amber-100/80 px-4 py-4 dark:border-amber-800 dark:bg-amber-900/30">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-900 dark:text-amber-100">
              Selected ignored item
            </div>
            <div class="mt-2 text-lg font-semibold text-base-content">{row().name}</div>
            <div class="mt-2 text-xs font-medium uppercase tracking-wide text-amber-800 dark:text-amber-200">
              Ignored by Pulse
            </div>
            <div class="mt-2 text-sm text-amber-950 dark:text-amber-100">
              Pulse is blocking reports from this surface.
            </div>
          </div>
          <button
            type="button"
            onClick={() => setSelectedIgnoredRowKey(null)}
            class="rounded-md p-1 hover:bg-amber-200/70 hover:text-base-content dark:hover:bg-amber-800/50"
            aria-label="Close"
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>
      </div>
    );

    const renderIgnoredSurface = () => (
      <div class="rounded-lg border border-amber-200/80 bg-white/70 px-4 py-4 dark:border-amber-800/80 dark:bg-amber-950/20">
        <div class="text-xs font-semibold uppercase tracking-wide text-muted">Ignored surface</div>
        <div class="mt-3 overflow-hidden rounded-md border border-amber-200/80 dark:border-amber-800/80">
          <div class="grid grid-cols-[minmax(0,1.2fr)_minmax(0,1.5fr)_minmax(0,1fr)_auto] gap-0 border-b border-amber-200/80 bg-white/80 px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-amber-900 dark:border-amber-800/80 dark:bg-amber-950/30 dark:text-amber-100">
            <div>Ignored surface</div>
            <div>What Pulse is ignoring</div>
            <div>ID</div>
            <div class="text-right">Recovery</div>
          </div>
          <div class="grid grid-cols-[minmax(0,1.2fr)_minmax(0,1.5fr)_minmax(0,1fr)_auto] gap-0 bg-transparent px-3 py-2 text-xs">
            <div class="pr-3 font-medium text-base-content">
              {row().capabilities.map(getCapabilitySurfaceLabel).join(', ')}
            </div>
            <div class="pr-3 text-muted">
              Pulse will ignore new reports for this surface until reconnect is allowed.
            </div>
            <div class="pr-3 text-muted">
              <span class="font-mono text-base-content">{blockedId()}</span>
            </div>
            <div class="text-right text-[11px] text-muted">Ready to return</div>
          </div>
        </div>
      </div>
    );

    const renderRecoveryAction = () => (
      <div class="rounded-lg border border-amber-200/80 bg-white/70 px-4 py-4 dark:border-amber-800/80 dark:bg-amber-950/20">
        <div class="text-xs font-semibold uppercase tracking-wide text-muted">Recovery action</div>
        <div class="mt-2 text-xs text-muted">Allow this blocked ID to report again.</div>
        <div class="mt-4 flex flex-col gap-3">
          <button
            onClick={() =>
              row().capabilities.includes('docker')
                ? handleAllowDockerReconnect(row())
                : row().capabilities.includes('kubernetes')
                  ? handleAllowKubernetesReconnect(row())
                  : handleAllowHostReconnect(row())
            }
            disabled={Boolean(pendingAction())}
            class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-white px-3 py-2 text-sm font-medium text-blue-600 shadow-sm ring-1 ring-border hover:bg-blue-50 hover:text-blue-900 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-slate-900 dark:text-blue-400 dark:ring-slate-700 dark:hover:bg-blue-900 dark:hover:text-blue-300"
          >
            {isAllowingReconnect() ? 'Allowing reconnect…' : reconnectLabel()}
          </button>
          <div class="text-xs text-muted">
            This only changes Pulse. It does not reinstall software.
          </div>
        </div>
      </div>
    );

    return (
      <div
        id={`ignored-details-${row().rowKey}`}
        class="flex h-full flex-col overflow-y-auto bg-amber-50/70 dark:bg-amber-950/30"
      >
        {renderHeader()}
        <div class="space-y-4 p-4 text-sm text-muted">
          {renderIgnoredSurface()}
          {renderRecoveryAction()}
        </div>
      </div>
    );
  };

  const renderStopMonitoringDialog = () => (
    <Dialog
      isOpen={Boolean(stopMonitoringDialog())}
      onClose={() => {
        if (!stopMonitoringDialog()) return;
        const row = stopMonitoringDialog()!.row;
        if (getPendingInventoryAction(row.rowKey)) return;
        setStopMonitoringDialog(null);
      }}
      panelClass="max-w-lg"
      closeOnBackdrop={
        !stopMonitoringDialog() || !getPendingInventoryAction(stopMonitoringDialog()!.row.rowKey)
      }
      ariaLabel="Confirm stop monitoring"
    >
      <Show when={stopMonitoringDialog()}>
        {(dialog) => {
          const row = () => dialog().row;
          const pending = () => getPendingInventoryAction(row().rowKey) === 'stop-monitoring';
          const isKubernetes = () =>
            row().capabilities.includes('kubernetes') && !row().capabilities.includes('agent');
          const affectedSurfaces = () => getStopMonitoringSurfaces(row());
          return (
            <div class="flex max-h-[90vh] flex-col">
              <div class="border-b border-border px-6 py-4">
                <h2 class="text-lg font-semibold text-base-content">Stop monitoring?</h2>
                <p class="mt-1 text-sm text-muted">
                  Pulse will remove{' '}
                  <span class="font-medium text-base-content">{dialog().subject}</span> from
                  active reporting.
                </p>
              </div>
              <div class="space-y-4 px-6 py-4">
                <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-100">
                  <p class="font-medium">{dialog().scopeLabel} will stop in Pulse.</p>
                  <p class="mt-1 text-xs opacity-90">
                    The remote system keeps running. Pulse will ignore future reports and move this
                    item into Ignored by Pulse until you allow reconnect.
                  </p>
                </div>
                <Show when={affectedSurfaces().length > 0}>
                  <div class="rounded-md border border-border bg-surface-hover px-4 py-3 text-sm text-muted">
                    <p class="font-medium text-base-content">
                      Pulse will stop these reporting surfaces
                    </p>
                    <div class="mt-3 grid gap-2">
                      <For each={affectedSurfaces()}>
                        {(surface) => (
                          <div class="rounded-md border border-border bg-surface px-3 py-2">
                            <div class="text-sm font-medium text-base-content">{surface.label}</div>
                            <div class="mt-1 text-xs text-muted">{surface.detail}</div>
                            <Show when={surface.idLabel && surface.idValue}>
                              <div class="mt-2 text-[11px] text-muted">
                                {surface.idLabel}:{' '}
                                <span class="font-mono text-base-content">{surface.idValue}</span>
                              </div>
                            </Show>
                          </div>
                        )}
                      </For>
                    </div>
                  </div>
                </Show>
                <div class="rounded-md border border-border bg-surface-hover px-4 py-3 text-sm text-muted">
                  <p class="font-medium text-base-content">What stays unchanged</p>
                  <p class="mt-1 text-xs">
                    {isKubernetes()
                      ? 'The cluster itself is not uninstalled or shut down.'
                      : 'The host, containers, and installed agent binaries are not uninstalled or shut down.'}
                  </p>
                </div>
              </div>
              <div class="flex flex-col-reverse gap-2 border-t border-border px-6 py-4 sm:flex-row sm:justify-end">
                <button
                  type="button"
                  onClick={() => setStopMonitoringDialog(null)}
                  disabled={pending()}
                  class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={() =>
                    isKubernetes()
                      ? handleRemoveKubernetesCluster(row())
                      : handleRemoveAgent(row())
                  }
                  disabled={pending()}
                  class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {pending() ? 'Stopping…' : 'Confirm stop monitoring'}
                </button>
              </div>
            </div>
          );
        }}
      </Show>
    </Dialog>
  );

  const renderInstallerSection = () => (
    <Show when={showInstaller()}>
      <SettingsPanel
        title={props.embedded ? 'Install on a host' : 'Infrastructure'}
        description={
          props.embedded
            ? 'Use the unified agent as the default path for hosts, Docker, Kubernetes, and agent-managed Proxmox.'
            : 'Primary setup hub for unified agents across systems, Docker, Kubernetes, Proxmox, and related infrastructure.'
        }
        icon={<Server class="w-5 h-5" strokeWidth={2} />}
        bodyClass="space-y-5"
      >
        <Show when={setupHandoff()}>
          {(handoff) => (
            <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-950 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-50">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div class="space-y-2">
                  <p class="font-semibold">Security configured. Save these first-run credentials now.</p>
                  <p class="text-xs text-emerald-800 dark:text-emerald-200">
                    This is the canonical handoff from first-run setup into Infrastructure Install.
                    Generate a scoped install token below before copying agent commands.
                  </p>
                  <div class="grid gap-3 sm:grid-cols-3">
                    <div class="rounded-md border border-emerald-200 bg-white px-3 py-2 dark:border-emerald-800 dark:bg-emerald-950">
                      <div class="text-[11px] font-medium uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                        Username
                      </div>
                      <div class="mt-1 font-mono text-sm text-base-content">{handoff().username}</div>
                    </div>
                    <div class="rounded-md border border-emerald-200 bg-white px-3 py-2 dark:border-emerald-800 dark:bg-emerald-950">
                      <div class="text-[11px] font-medium uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                        Password
                      </div>
                      <div class="mt-1 font-mono text-sm text-base-content break-all">
                        {handoff().password}
                      </div>
                    </div>
                    <div class="rounded-md border border-emerald-200 bg-white px-3 py-2 dark:border-emerald-800 dark:bg-emerald-950">
                      <div class="text-[11px] font-medium uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                        Admin API Token
                      </div>
                      <div class="mt-1 font-mono text-sm text-base-content break-all">
                        {handoff().apiToken}
                      </div>
                    </div>
                  </div>
                </div>
                <div class="flex flex-wrap gap-2 lg:w-64 lg:flex-col">
                  <button
                    type="button"
                    onClick={() =>
                      void copySetupHandoffField(handoff().password, 'Copied first-run password.')
                    }
                    class="inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-100 dark:hover:bg-emerald-800"
                  >
                    Copy password
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      void copySetupHandoffField(
                        handoff().apiToken,
                        'Copied first-run admin API token.',
                      )
                    }
                    class="inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-100 dark:hover:bg-emerald-800"
                  >
                    Copy admin token
                  </button>
                  <button
                    type="button"
                    onClick={downloadSetupHandoff}
                    class="inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-100 dark:hover:bg-emerald-800"
                  >
                    Download credentials
                  </button>
                  <button
                    type="button"
                    onClick={clearSetupHandoff}
                    class="inline-flex items-center justify-center rounded-md px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100 dark:text-emerald-100 dark:hover:bg-emerald-800"
                  >
                    Dismiss
                  </button>
                </div>
              </div>
            </div>
          )}
        </Show>

        <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100">
          <p class="font-semibold">Unified Agent is the default monitoring gateway.</p>
          <p class="mt-1 text-xs text-blue-800 dark:text-blue-200">
            Install it on each system you want Pulse to monitor. The installer auto-detects
            available platforms on that machine and enables the right integrations.
          </p>
        </div>

        <Show when={!props.embedded}>
          <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-100">
            <div class="flex items-start gap-3">
              <ProxmoxIcon class="w-5 h-5 text-amber-500 mt-0.5 shrink-0" />
              <div class="flex-1">
                <p class="text-sm">
                  Proxmox nodes can be added here with the unified agent for extra capabilities
                  like temperature monitoring and Pulse Patrol automation (auto-creates the
                  required token and links the node).
                </p>
                <button
                  type="button"
                  onClick={() => navigate('/settings/infrastructure/proxmox')}
                  class="mt-2 inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2 py-1.5 text-sm font-medium text-emerald-800 hover:bg-emerald-100 hover:text-emerald-900 dark:text-emerald-200 dark:hover:bg-emerald-900 dark:hover:text-emerald-100 underline"
                >
                  Need direct setup instead? Open Proxmox →
                </button>
              </div>
            </div>
          </div>
        </Show>

        <div class="space-y-5">
          <div class="space-y-3">
            <div class="space-y-1">
              <p class="text-sm font-semibold text-base-content">
                <span class="inline-flex items-center justify-center w-5 h-5 mr-1.5 rounded-full bg-blue-600 text-white text-xs font-bold">
                  1
                </span>
                Generate API token
              </p>
              <p class="text-sm text-muted ml-6">
                {requiresToken()
                  ? 'Create a fresh token scoped for Agent, Docker, and Kubernetes monitoring.'
                  : 'Tokens are optional on this Pulse instance. Generate one if you want copied commands to preserve explicit credentialed transport.'}
              </p>
            </div>

            <div class="flex gap-2">
              <input
                type="text"
                value={tokenName()}
                onInput={(event) => setTokenName(event.currentTarget.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && !isGeneratingToken()) {
                    handleGenerateToken();
                  }
                }}
                placeholder="Token name (optional)"
                class="flex-1 rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:focus:border-blue-400 dark:focus:ring-blue-900"
              />
              <button
                type="button"
                onClick={handleGenerateToken}
                disabled={isGeneratingToken()}
                class="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isGeneratingToken()
                  ? 'Generating…'
                  : hasToken()
                    ? 'Generate another'
                    : 'Generate token'}
              </button>
            </div>

            <Show when={latestRecord()}>
              <div class="flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-4 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200">
                <svg
                  class="w-4 h-4"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                </svg>
                <span>
                  Token <strong>{latestRecord()?.name}</strong> created. Commands below now
                  include this credential.
                </span>
              </div>
            </Show>
          </div>

          <Show when={!requiresToken()}>
            <div class="space-y-3">
              <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
                Tokens are optional on this Pulse instance. Confirm to generate commands without
                embedding a token.
              </div>
              <button
                type="button"
                onClick={acknowledgeNoToken}
                disabled={confirmedNoToken()}
                class={`inline-flex items-center justify-center rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                  confirmedNoToken()
                    ? 'bg-green-600 text-white cursor-default'
                    : 'bg-surface text-base-content border border-border hover:bg-surface-hover'
                }`}
              >
                {confirmedNoToken() ? 'No token confirmed' : 'Confirm without token'}
              </button>
            </div>
          </Show>

          <Show when={requiresToken() && !commandsUnlocked()}>
            <div class="space-y-3 opacity-60 pointer-events-none select-none">
              <div class="flex items-center justify-between">
                <div>
                  <h4 class="text-sm font-semibold text-base-content">
                    <span class="inline-flex items-center justify-center w-5 h-5 mr-1.5 rounded-full bg-slate-400 text-white text-xs font-bold">
                      2
                    </span>
                    Installation commands
                  </h4>
                  <p class="text-xs text-muted mt-0.5 ml-6">
                    Generate a token above to unlock installation commands.
                  </p>
                </div>
              </div>
              <div class="rounded-md border border-border bg-surface-hover px-4 py-6 text-center">
                <svg
                  class="w-8 h-8 mx-auto text-muted mb-2"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="1.5"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"
                  />
                </svg>
                <p class="text-sm text-muted">
                  Click "Generate token" above to see installation commands
                </p>
              </div>
            </div>
          </Show>

          <Show when={commandsUnlocked()}>
            <div class="space-y-3">
              <div class="space-y-3">
                <div class="flex items-center justify-between">
                  <div>
                    <h4 class="text-sm font-semibold text-base-content">
                      <Show when={requiresToken()}>
                        <span class="inline-flex items-center justify-center w-5 h-5 mr-1.5 rounded-full bg-green-600 text-white text-xs font-bold">
                          2
                        </span>
                      </Show>
                      Installation commands
                    </h4>
                    <p class={`text-xs text-muted mt-0.5 ${requiresToken() ? 'ml-6' : ''}`}>
                      The installer auto-detects Docker, Kubernetes, and Proxmox on the target
                      machine.
                    </p>
                  </div>
                </div>

                <div class="rounded-md border border-border bg-surface-hover px-4 py-3">
                  <label class="block text-xs font-medium text-base-content mb-1.5">
                    Connection URL (Agent → Pulse)
                  </label>
                  <div class="flex gap-2">
                    <input
                      type="text"
                      value={customAgentUrl()}
                      onInput={(e) => setCustomAgentUrl(e.currentTarget.value)}
                      placeholder={agentUrl()}
                      class="flex-1 rounded-md border bg-surface px-3 py-1.5 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                    />
                  </div>
                  <p class="mt-1.5 text-xs text-muted">
                    Override the address agents use to connect to this server (e.g., use IP
                    address <code>http://192.0.2.50:7655</code> if DNS fails).
                    <Show when={!customAgentUrl()}>
                      <span class="ml-1 opacity-75">
                        Currently using auto-detected: {agentUrl()}
                      </span>
                    </Show>
                  </p>
                </div>
                <div class="rounded-md border border-border bg-surface-hover px-4 py-3">
                  <label
                    for="custom-ca-certificate-path"
                    class="block text-xs font-medium text-base-content mb-1.5"
                  >
                    Custom CA certificate path (optional)
                  </label>
                  <div class="flex gap-2">
                    <input
                      id="custom-ca-certificate-path"
                      type="text"
                      value={customCaPath()}
                      onInput={(e) => setCustomCaPath(e.currentTarget.value)}
                      placeholder={
                        selectedAgentUrl().startsWith('http://')
                          ? 'Not needed for plain HTTP'
                          : 'Examples: /etc/pulse/ca.pem or C:\\Pulse\\ca.cer'
                      }
                      class="flex-1 rounded-md border bg-surface px-3 py-1.5 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                    />
                  </div>
                  <p class="mt-1.5 text-xs text-muted">
                    Preserves custom trust for copied install, upgrade, and uninstall commands.
                    Shell commands pass <code>--cacert</code> to both the download and the
                    installer. Windows commands set <code>PULSE_CACERT</code> and use a
                    transport-aware PowerShell bootstrap for the initial script fetch.
                  </p>
                </div>
                <Show when={insecureMode()}>
                  <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
                    <span class="font-medium">TLS verification disabled</span> — skip cert checks
                    for self-signed setups. Not recommended for production.
                  </div>
                </Show>
                <label
                  class="inline-flex items-center gap-2 text-sm text-base-content cursor-pointer"
                  title="Skip TLS certificate verification (for self-signed certificates)"
                >
                  <input
                    type="checkbox"
                    checked={insecureMode()}
                    onChange={(e) => setInsecureMode(e.currentTarget.checked)}
                    class="rounded text-blue-600 focus:ring-blue-500"
                  />
                  Skip TLS certificate verification (self-signed certs; not recommended)
                </label>
                <label
                  class="inline-flex items-center gap-2 text-sm text-base-content cursor-pointer"
                  title="Allow Pulse Patrol to execute diagnostic and fix commands on this agent (auto-fix requires Pulse Pro)"
                >
                  <input
                    type="checkbox"
                    checked={enableCommands()}
                    onChange={(e) => setEnableCommands(e.currentTarget.checked)}
                    class="rounded text-blue-600 focus:ring-blue-500"
                  />
                  Enable Pulse command execution (for Patrol auto-fix)
                </label>
                <Show when={enableCommands()}>
                  <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-2 text-sm text-blue-800 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200">
                    <span class="font-medium">Pulse commands enabled</span> — The agent will
                    accept diagnostic and fix commands from Pulse Patrol features.
                  </div>
                </Show>
                <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-2 text-sm text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-100">
                  <span class="font-medium">Config signing (optional)</span> — Require signed
                  remote config payloads with{' '}
                  <code>PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED=true</code>. Provide keys via{' '}
                  <code>PULSE_AGENT_CONFIG_SIGNING_KEY</code> (Pulse) and{' '}
                  <code>PULSE_AGENT_CONFIG_PUBLIC_KEYS</code> (agents).
                </div>
                <div class="rounded-md border border-border bg-surface-hover px-4 py-3">
                  <label
                    for="install-profile-select"
                    class="block text-xs font-medium text-base-content mb-1.5"
                  >
                    Target profile (optional)
                  </label>
                  <select
                    id="install-profile-select"
                    value={installProfile()}
                    onChange={(event) =>
                      handleInstallProfileChange(event.currentTarget.value as InstallProfile)
                    }
                    class="w-full rounded-md border bg-surface px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                  >
                    <For each={INSTALL_PROFILE_OPTIONS}>
                      {(option) => <option value={option.value}>{option.label}</option>}
                    </For>
                  </select>
                  <p class="mt-1.5 text-xs text-muted">
                    {getSelectedInstallProfile().description}
                  </p>
                  <Show when={getInstallProfileFlags().length > 0}>
                    <p class="mt-1.5 text-xs text-muted">
                      Adds flags to shell-based install commands:{' '}
                      <code>{getInstallProfileFlags().join(' ')}</code>
                    </p>
                  </Show>
                </div>
              </div>

              <div class="space-y-4">
                <For each={commandSections()}>
                  {(section) => (
                    <div class="space-y-3 rounded-md border border-border p-4">
                      <div class="space-y-1">
                        <h5 class="text-sm font-semibold text-base-content">{section.title}</h5>
                        <p class="text-xs text-muted">{section.description}</p>
                      </div>
                      <div class="space-y-3">
                        <For each={section.snippets}>
                          {(snippet) => {
                            const copyCommand = () => snippet.command;
                            const commandTelemetryCapability = () => {
                              const label = normalizeTelemetryPart(snippet.label) || 'install';
                              return `${section.platform}:${installProfile()}:${label}`;
                            };

                            return (
                              <div class="space-y-2">
                                <h6 class="text-xs font-semibold uppercase tracking-wide text-muted">
                                  {snippet.label}
                                </h6>
                                <div class="relative">
                                  <button
                                    type="button"
                                    onClick={async () => {
                                      const success = await copyToClipboard(copyCommand());
                                      if (success) {
                                        trackAgentInstallCommandCopied(
                                          UNIFIED_AGENT_TELEMETRY_SURFACE,
                                          commandTelemetryCapability(),
                                        );
                                        notificationStore.success(
                                          getUnifiedAgentClipboardCopySuccessMessage(),
                                        );
                                      } else {
                                        notificationStore.error(
                                          getUnifiedAgentClipboardCopyErrorMessage(),
                                        );
                                      }
                                    }}
                                    class="absolute right-2 top-2 inline-flex min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 items-center justify-center rounded-md bg-surface-hover p-2 transition-colors hover:text-slate-200"
                                    title="Copy command"
                                  >
                                    <svg
                                      width="16"
                                      height="16"
                                      viewBox="0 0 24 24"
                                      fill="none"
                                      stroke="currentColor"
                                      stroke-width="2"
                                    >
                                      <rect
                                        x="9"
                                        y="9"
                                        width="13"
                                        height="13"
                                        rx="2"
                                        ry="2"
                                      ></rect>
                                      <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                    </svg>
                                  </button>
                                  <pre class="overflow-x-auto rounded-md bg-base p-3 pr-12 text-xs text-base-content">
                                    <code>{copyCommand()}</code>
                                  </pre>
                                </div>
                                <Show when={snippet.note}>
                                  <p class="text-xs text-muted">{snippet.note}</p>
                                </Show>
                              </div>
                            );
                          }}
                        </For>
                      </div>
                    </div>
                  )}
                </For>
              </div>

              <div class="space-y-3 rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-100">
                <div class="flex items-center justify-between gap-3">
                  <h5 class="text-sm font-semibold">Check installation status</h5>
                  <button
                    type="button"
                    onClick={handleLookup}
                    disabled={lookupLoading()}
                    class="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {lookupLoading() ? 'Checking…' : 'Check status'}
                  </button>
                </div>
                <p class="text-xs text-blue-800 dark:text-blue-200">
                  Enter the hostname (or agent ID) from the machine you just installed. Pulse
                  returns the latest status instantly.
                </p>
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                  <input
                    type="text"
                    value={lookupValue()}
                    onInput={(event) => {
                      setLookupValue(event.currentTarget.value);
                      setLookupError(null);
                      setLookupResult(null);
                    }}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') {
                        event.preventDefault();
                        void handleLookup();
                      }
                    }}
                    placeholder="Hostname or agent ID"
                    class="flex-1 rounded-md border border-blue-200 bg-surface px-3 py-2 text-sm text-blue-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100 dark:focus:border-blue-300 dark:focus:ring-blue-800"
                  />
                </div>
                <Show when={lookupError()}>
                  <p class="text-xs font-medium text-red-600 dark:text-red-300">
                    {lookupError()}
                  </p>
                </Show>
                <Show when={lookupResult()}>
                  {(result) => {
                    const agent = () => result().agent!;
                    const lookupStatusPresentation = () =>
                      getUnifiedAgentLookupStatusPresentation(agent().connected);
                    return (
                      <div class="space-y-1 rounded-md border border-blue-200 bg-surface px-3 py-2 text-xs text-blue-900 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100">
                        <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                          <div class="text-sm font-semibold">
                            {agent().displayName || agent().hostname}
                          </div>
                          <div class="flex items-center gap-2">
                            <span
                              class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold ${lookupStatusPresentation().badgeClass}`}
                            >
                              {lookupStatusPresentation().label}
                            </span>
                            <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium text-blue-700 dark:bg-blue-900 dark:text-blue-200">
                              {agent().status || 'unknown'}
                            </span>
                          </div>
                        </div>
                        <div>
                          Last seen {formatRelativeTime(agent().lastSeen)} (
                          {formatAbsoluteTime(agent().lastSeen)})
                        </div>
                        <Show when={agent().agentVersion}>
                          <div class="text-xs text-blue-700 dark:text-blue-200">
                            Agent version {agent().agentVersion}
                          </div>
                        </Show>
                      </div>
                    );
                  }}
                </Show>
              </div>
              <details class="rounded-md border border-border bg-surface-hover px-4 py-3 text-sm">
                <summary class="cursor-pointer text-sm font-medium text-base-content">
                  Troubleshooting
                </summary>
                <div class="mt-3 space-y-4">
                  <div>
                    <p class="text-xs uppercase tracking-wide text-muted">
                      Auto-detection not working?
                    </p>
                    <p class="mt-1 text-xs text-muted">
                      If Docker, Kubernetes, or Proxmox isn't detected automatically, add these
                      flags to the install command:
                    </p>
                    <ul class="mt-2 text-xs text-muted list-disc list-inside space-y-1">
                      <li>
                        <code class="bg-surface-hover px-1 rounded">--enable-docker</code> — Force
                        enable Docker/Podman monitoring
                      </li>
                      <li>
                        <code class="bg-surface-hover px-1 rounded">--enable-kubernetes</code> —
                        Force enable Kubernetes monitoring
                      </li>
                      <li>
                        <code class="bg-surface-hover px-1 rounded">--enable-proxmox</code> — Force
                        enable Proxmox integration (creates API token)
                      </li>
                      <li>
                        <code class="bg-surface-hover px-1 rounded">--proxmox-type pve|pbs</code>{' '}
                        — Set Proxmox node mode explicitly
                      </li>
                      <li>
                        <code class="bg-surface-hover px-1 rounded">--disable-docker</code> — Skip
                        Docker even if detected
                      </li>
                    </ul>
                  </div>
                </div>
              </details>
            </div>
          </Show>

          <div class="border-t border-border pt-4 mt-4">
            <div class="space-y-3">
              <h4 class="text-sm font-semibold text-base-content">Uninstall agent</h4>
              <p class="text-xs text-muted">
                Run the appropriate command on your machine to remove the Pulse agent:
              </p>
              <div class="space-y-1">
                <span class="text-xs font-medium text-muted">Linux / macOS / FreeBSD</span>
                <div class="relative">
                  <button
                    type="button"
                    onClick={async () => {
                      const success = await copyToClipboard(getUninstallCommand());
                      if (success) {
                        notificationStore.success(getUnifiedAgentClipboardCopySuccessMessage());
                      } else {
                        notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
                      }
                    }}
                    class="absolute right-2 top-2 inline-flex min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 items-center justify-center rounded-md bg-surface-hover p-2 text-slate-400 transition-colors hover:bg-slate-700 hover:text-slate-200"
                    title="Copy command"
                  >
                    <svg
                      width="16"
                      height="16"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                    </svg>
                  </button>
                  <pre class="overflow-x-auto rounded-md bg-slate-950 p-3 pr-12 font-mono text-xs text-red-400">
                    <code>{getUninstallCommand()}</code>
                  </pre>
                </div>
              </div>
              <p class="text-xs text-muted italic">
                If the agent can't reach this server, run directly on the machine:{' '}
                <code class="bg-surface-hover px-1 rounded not-italic">
                  sudo bash /var/lib/pulse-agent/install.sh --uninstall
                </code>{' '}
                (TrueNAS:{' '}
                <code class="bg-surface-hover px-1 rounded not-italic">
                  /data/pulse-agent/install.sh
                </code>
                , Unraid:{' '}
                <code class="bg-surface-hover px-1 rounded not-italic">
                  /boot/config/plugins/pulse-agent/install.sh
                </code>
                )
              </p>
              <div class="space-y-1">
                <span class="text-xs font-medium text-muted">
                  Windows (PowerShell as Administrator)
                </span>
                <div class="relative">
                  <button
                    type="button"
                    onClick={async () => {
                      const success = await copyToClipboard(getWindowsUninstallCommand());
                      if (success) {
                        notificationStore.success(getUnifiedAgentClipboardCopySuccessMessage());
                      } else {
                        notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
                      }
                    }}
                    class="absolute right-2 top-2 inline-flex min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 items-center justify-center rounded-md bg-surface-hover p-2 text-slate-400 transition-colors hover:bg-slate-700 hover:text-slate-200"
                    title="Copy command"
                  >
                    <svg
                      width="16"
                      height="16"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                    </svg>
                  </button>
                  <pre class="overflow-x-auto rounded-md bg-slate-950 p-3 pr-12 font-mono text-xs text-red-400">
                    <code>{getWindowsUninstallCommand()}</code>
                  </pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </SettingsPanel>
    </Show>
  );

  const renderIgnoredSection = () => (
    <Show when={showMonitoringStoppedSection()}>
      <div ref={recoveryQueueSectionRef}>
        <SettingsPanel
          title="Ignored by Pulse"
          description="Items you explicitly told Pulse to ignore stay out of live reporting until reconnect is allowed."
          icon={<Users class="w-5 h-5" strokeWidth={2} />}
          bodyClass="space-y-4"
        >
          <Show
            when={filteredMonitoringStoppedRows().length > 0}
            fallback={
              <div class="rounded-md border border-dashed border-border px-4 py-6 text-sm text-muted">
                {getMonitoringStoppedEmptyState(hasFilters())}
              </div>
            }
          >
            <div class="grid gap-4">
              <div class="overflow-hidden rounded-lg border border-amber-200 bg-amber-50/70 dark:border-amber-800 dark:bg-amber-950/30">
                <div class="border-b border-amber-200 px-4 py-3 dark:border-amber-800">
                  <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-900 dark:text-amber-100">
                    Browse ignored items
                  </div>
                  <div class="mt-2 text-sm text-amber-950 dark:text-amber-100">
                    Select an ignored item to open its recovery drawer.
                  </div>
                </div>
                <div class="divide-y divide-amber-200/80 dark:divide-amber-800/80">
                  <For each={filteredMonitoringStoppedRows()}>
                    {(row) => {
                      const pendingAction = () => getPendingInventoryAction(row.rowKey);
                      const isSelected = () => selectedIgnoredRowKey() === row.rowKey;
                      return (
                        <button
                          type="button"
                          onClick={() => setSelectedIgnoredRowKey(row.rowKey)}
                          class={`flex w-full flex-col gap-2 px-4 py-3 text-left transition-colors ${
                            isSelected()
                              ? 'bg-amber-100/80 ring-1 ring-inset ring-amber-300 dark:bg-amber-900/40 dark:ring-amber-700'
                              : 'hover:bg-amber-100/50 dark:hover:bg-amber-900/20'
                          }`}
                        >
                          <div class="flex flex-wrap items-center justify-between gap-3">
                            <div class="min-w-0">
                              <div class="flex flex-wrap items-center gap-2">
                                <h4 class="truncate text-sm font-semibold text-base-content">
                                  {row.name}
                                </h4>
                                <span class="inline-flex items-center rounded-full bg-white/80 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide text-amber-800 dark:bg-amber-900/60 dark:text-amber-200">
                                  {getRemovedUnifiedAgentItemLabel(row)}
                                </span>
                              </div>
                              <div class="mt-1 text-xs text-muted">
                                {row.capabilities.map(getCapabilitySurfaceLabel).join(', ')}
                              </div>
                            </div>
                            <div class="text-[11px] text-muted">
                              {pendingAction() === 'allow-reconnect'
                                ? 'Reconnect in progress'
                                : 'Select to review'}
                            </div>
                          </div>
                          <div class="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted">
                            <Show
                              when={
                                row.displayName && row.hostname && row.displayName !== row.hostname
                              }
                            >
                              <span>Hostname: {row.hostname}</span>
                            </Show>
                            <span>
                              Stopped{' '}
                              {row.removedAt
                                ? `${formatRelativeTime(row.removedAt)} (${formatAbsoluteTime(row.removedAt)})`
                                : 'at an unknown time'}
                            </span>
                          </div>
                        </button>
                      );
                    }}
                  </For>
                </div>
              </div>

              <Dialog
                isOpen={Boolean(selectedIgnoredRow())}
                onClose={() => setSelectedIgnoredRowKey(null)}
                layout="drawer-right"
                panelClass="max-w-[720px]"
                ariaLabel="Ignored item details"
              >
                <Show when={selectedIgnoredRow()}>
                  {(rowAccessor) => renderSelectedIgnoredRowDetails(rowAccessor)}
                </Show>
              </Dialog>
            </div>
          </Show>
        </SettingsPanel>
      </div>
    </Show>
  );

  return (
    <div class="space-y-6">
      {renderStopMonitoringDialog()}
      {renderInstallerSection()}

      <Show when={showInventory()}>
        <div class="space-y-6">
          <div class="rounded-md border border-border bg-surface-alt px-4 py-3 text-sm">
            <p class="text-base-content">{inventoryStatusSummaryText()}</p>
          </div>

          <SettingsPanel
            title="Reporting now"
            description="Hosts and runtimes currently checking in to Pulse."
            icon={<Users class="w-5 h-5" strokeWidth={2} />}
            bodyClass="space-y-4"
          >
            <div class="rounded-md border border-border bg-surface-alt px-4 py-3">
              <p class="text-sm font-medium text-base-content">{reportingCoverageSummaryText()}</p>
              <p class="mt-2 text-xs text-muted">
                This workspace does not list every asset Pulse has discovered. It focuses on
                systems and runtimes that are actively checking in right now.
              </p>
            </div>

            <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-4 dark:border-emerald-800 dark:bg-emerald-950/40">
              <p class="text-xs font-semibold uppercase tracking-wide text-emerald-800 dark:text-emerald-300">
                Active reporting
              </p>
              <p class="mt-2 text-2xl font-semibold text-emerald-900 dark:text-emerald-100">
                {filteredActiveRows().length}
              </p>
              <p class="mt-2 text-sm text-emerald-900 dark:text-emerald-200">
                Item{filteredActiveRows().length === 1 ? '' : 's'} actively checking in to Pulse.
              </p>
            </div>

            <Show when={hasLinkedAgents()}>
              <div class="flex items-start gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 dark:border-blue-800 dark:bg-blue-900">
                <svg
                  class="h-4 w-4 mt-0.5 flex-shrink-0 text-blue-500 dark:text-blue-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <p class="text-xs text-blue-700 dark:text-blue-300">
                  <span class="font-medium">{linkedAgents().length}</span> agent
                  {linkedAgents().length > 1 ? 's are' : ' is'} linked to Proxmox node
                  {linkedAgents().length > 1 ? 's' : ''} and flagged with a{' '}
                  <span class="font-medium text-blue-700 dark:text-blue-300">Linked</span> badge.
                </p>
              </div>
            </Show>

            <Show when={hasOutdatedAgents()}>
              <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-700 dark:bg-amber-900">
                <div class="flex items-start gap-3">
                  <svg
                    class="h-5 w-5 flex-shrink-0 text-amber-500 dark:text-amber-400 mt-0.5"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                  >
                    <path
                      fill-rule="evenodd"
                      d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z"
                      clip-rule="evenodd"
                    />
                  </svg>
                  <div class="flex-1 space-y-1">
                    <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
                      {outdatedAgents().length} outdated agent {outdatedAgents().length > 1 ? 'binaries' : 'binary'} detected
                    </p>
                    <p class="text-sm text-amber-700 dark:text-amber-300">
                      Older standalone agent binaries are deprecated. Expand a row to copy the
                      upgrade command.
                    </p>
                  </div>
                </div>
              </div>
            </Show>

            <div class="space-y-3">
              <div class="min-w-[220px] space-y-1">
                <label for="agent-filter-search" class="text-xs font-medium text-muted">
                  Search reporting items
                </label>
                <SearchField
                  placeholder="Search name, hostname, or ID"
                  value={filterSearch()}
                  onChange={setFilterSearch}
                  class="w-full"
                  inputClass="min-h-10 sm:min-h-9 px-3 py-2 sm:py-1.5 shadow-sm focus:ring-1"
                />
              </div>

              <div class="flex flex-wrap items-end gap-3">
                <div class="pb-2 text-xs font-medium uppercase tracking-wide text-muted">
                  Refine results
                </div>
                <div class="space-y-1">
                  <label for="agent-filter-capability" class="text-xs font-medium text-muted">
                    Capability
                  </label>
                  <select
                    id="agent-filter-capability"
                    value={filterCapability()}
                    onChange={(event) =>
                      setFilterCapability(event.currentTarget.value as 'all' | AgentCapability)
                    }
                    class="min-h-10 sm:min-h-9 rounded-md border border-border bg-surface px-2.5 py-2 sm:py-1.5 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                  >
                    <option value="all">All capabilities</option>
                    <option value="agent">Agent</option>
                    <option value="docker">Docker</option>
                    <option value="kubernetes">Kubernetes</option>
                    <option value="proxmox">Proxmox</option>
                    <option value="pbs">PBS</option>
                    <option value="pmg">PMG</option>
                  </select>
                </div>
                <div class="space-y-1">
                  <label for="agent-filter-scope" class="text-xs font-medium text-muted">
                    Scope
                  </label>
                  <select
                    id="agent-filter-scope"
                    value={filterScope()}
                    onChange={(event) =>
                      setFilterScope(
                        event.currentTarget.value as 'all' | Exclude<ScopeCategory, 'na'>,
                      )
                    }
                    class="min-h-10 sm:min-h-9 rounded-md border border-border bg-surface px-2.5 py-2 sm:py-1.5 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                  >
                    <option value="all">All scopes</option>
                    <option value="default">Default</option>
                    <option value="profile">Profile assigned</option>
                    <option value="ai-managed">Patrol-managed</option>
                  </select>
                </div>
                <button
                  type="button"
                  onClick={resetFilters}
                  disabled={!hasFilters()}
                  class={`min-h-10 sm:min-h-9 rounded-md px-3 py-2 text-sm font-medium transition-colors ${hasFilters() ? ' text-base-content hover:bg-surface-alt' : ' text-slate-400 cursor-not-allowed '}`}
                >
                  Clear
                </button>
              </div>
            </div>

            <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-muted">
              <span>
                Showing {filteredActiveRows().length} of {activeRows().length} active records.
              </span>
              <Show when={filteredMonitoringStoppedRows().length > 0}>
                <span>
                  {filteredMonitoringStoppedRows().length} item(s) are currently ignored by Pulse.
                </span>
              </Show>
            </div>

            <div class="rounded-md border border-border bg-surface-hover px-4 py-3 text-sm text-muted">
              Stop monitoring removes an item from active reporting and moves it into the Ignored by
              Pulse list. The remote system keeps running; Pulse simply ignores new reports until
              you allow reconnect.
            </div>

            <Show when={inventoryActionNotice()}>
              {(notice) => (
                <div
                  class={`rounded-md border px-4 py-3 text-sm ${
                    notice().tone === 'success'
                      ? 'border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-100'
                      : 'border-blue-200 bg-blue-50 text-blue-900 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100'
                  }`}
                >
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div class="space-y-1">
                      <p class="font-semibold">{notice().title}</p>
                      <p class="text-xs opacity-90">{notice().detail}</p>
                    </div>
                    <div class="flex items-center gap-2">
                      <Show when={notice().showRecoveryQueueLink}>
                        <button
                          type="button"
                          onClick={scrollToRecoveryQueue}
                          class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-xs font-medium underline"
                        >
                          View ignored items
                        </button>
                      </Show>
                      <button
                        type="button"
                        onClick={() => setInventoryActionNotice(null)}
                        class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-xs font-medium underline"
                        aria-label="Dismiss inventory action message"
                      >
                        Dismiss
                      </button>
                    </div>
                  </div>
                </div>
              )}
            </Show>

            <div class="grid gap-4">
              <div class="overflow-hidden rounded-xl border border-border bg-surface shadow-sm">
                <div class="border-b border-border bg-surface-alt px-4 py-3">
                  <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                    Browse reporting items
                  </div>
                  <div class="mt-2 text-sm text-base-content">
                    Select a reporting item to open its details drawer.
                  </div>
                </div>
                <PulseDataGrid
                  data={filteredActiveRows()}
                  emptyState={
                    hasFilters()
                      ? 'No reporting items match the current filters.'
                      : 'Nothing is actively reporting to Pulse yet.'
                  }
                  desktopMinWidth="960px"
                  columns={reportingColumns}
                  keyExtractor={(row) => row.rowKey}
                  onRowClick={(row) => toggleAgentDetails(row.rowKey)}
                />
              </div>

              <Dialog
                isOpen={Boolean(selectedActiveRow())}
                onClose={() => setExpandedRowKey(null)}
                layout="drawer-right"
                panelClass="max-w-[760px]"
                ariaLabel="Reporting item details"
              >
                <Show when={selectedActiveRow()}>
                  {(rowAccessor) => renderSelectedActiveRowDetails(rowAccessor)}
                </Show>
              </Dialog>
            </div>
          </SettingsPanel>

          {renderIgnoredSection()}
        </div>
      </Show>
    </div>
  );
};

export default InfrastructureOperationsController;
