import { Component, createSignal, Show, For, onMount, createEffect, createMemo } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { unwrap } from 'solid-js/store';
import { useWebSocket } from '@/App';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { SearchField } from '@/components/shared/SearchField';
import Server from 'lucide-solid/icons/server';
import Users from 'lucide-solid/icons/users';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import { MonitoringAPI } from '@/api/monitoring';
import {
  AgentProfilesAPI,
  type AgentProfile,
  type AgentProfileAssignment,
} from '@/api/agentProfiles';
import { SecurityAPI } from '@/api/security';
import { notificationStore } from '@/stores/notifications';
import { useResources } from '@/hooks/useResources';
import type { SecurityStatus } from '@/types/config';
import type { AgentLookupResponse } from '@/types/api';
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
import {
  getPreferredNamedEntityLabel,
  getPreferredResourceDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import {
  getAgentCapabilityBadgeClass,
  getAgentCapabilityLabel,
  type AgentCapability,
} from '@/utils/agentCapabilityPresentation';
import {
  ALLOW_RECONNECT_LABEL,
  getUnifiedAgentStatusPresentation,
  MONITORING_STOPPED_STATUS_LABEL,
} from '@/utils/unifiedAgentStatusPresentation';
import {
  trackAgentInstallCommandCopied,
  trackAgentInstallProfileSelected,
  trackAgentInstallTokenGenerated,
} from '@/utils/upgradeMetrics';
import type { Resource } from '@/types/resource';
import {
  getAgentDiscoveryResourceId,
  isAppContainerDiscoveryResourceType,
} from '@/utils/discoveryTarget';

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

type AgentPlatform = 'linux' | 'macos' | 'freebsd' | 'windows';
type UnifiedAgentStatus = 'active' | 'removed';
type ScopeCategory = 'default' | 'profile' | 'ai-managed' | 'na';
type InstallProfile = 'auto' | 'docker' | 'kubernetes' | 'proxmox-pve' | 'proxmox-pbs' | 'truenas';

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
  scope: {
    label: string;
    detail?: string;
    category: ScopeCategory;
  };
  searchText: string;
  kubernetesInfo?: {
    server?: string;
    context?: string;
    tokenName?: string;
  };
};

const getInventorySubjectLabel = (name?: string, fallback?: string) =>
  name || fallback || 'this host';
const getRemovedItemLabel = (row: UnifiedAgentRow) => {
  if (row.capabilities.includes('kubernetes') && !row.capabilities.includes('agent')) {
    return 'Kubernetes cluster';
  }
  if (row.capabilities.includes('docker')) {
    return 'Docker runtime';
  }
  if (row.capabilities.includes('proxmox')) {
    return 'Proxmox node';
  }
  return 'Host agent';
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
    flags: ['--enable-docker'],
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
  url: string,
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
        command: `curl -fsSL ${url}/install.sh | bash -s -- --url ${url} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
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
        command: `curl -fsSL ${url}/install.sh | bash -s -- --url ${url} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
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
        command: `curl -fsSL ${url}/install.sh | bash -s -- --url ${url} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
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
        command: `irm ${url}/install.ps1 | iex`,
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
        command: `$env:PULSE_URL="${url}"; $env:PULSE_TOKEN="${TOKEN_PLACEHOLDER}"; irm ${url}/install.ps1 | iex`,
        note: (
          <span>
            Non-interactive installation. Set environment variables before running to skip prompts.
          </span>
        ),
      },
    ],
  },
});

interface UnifiedAgentsProps {
  embedded?: boolean;
  showInstaller?: boolean;
  showInventory?: boolean;
}

export const UnifiedAgents: Component<UnifiedAgentsProps> = (props) => {
  const { state } = useWebSocket();
  const { byType, resources, mutate: mutateResources, refetch: refetchResources } = useResources();
  const navigate = useNavigate();

  const pd = (r: Resource) =>
    r.platformData ? (unwrap(r.platformData) as Record<string, unknown>) : undefined;
  const asRecord = (value: unknown) =>
    value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;
  const asString = (value: unknown) =>
    typeof value === 'string' && value.trim() ? value.trim() : undefined;
  const asBoolean = (value: unknown) => (typeof value === 'boolean' ? value : undefined);
  const platformAgent = (r: Resource) => asRecord(pd(r)?.agent);
  const platformDocker = (r: Resource) => asRecord(pd(r)?.docker);
  const platformKubernetes = (r: Resource) => asRecord(pd(r)?.kubernetes);

  const getAgentId = (r: Resource) => {
    const platformData = pd(r);
    return (
      r.agent?.agentId ||
      asString(platformAgent(r)?.agentId) ||
      asString(platformData?.agentId) ||
      r.discoveryTarget?.resourceId ||
      r.discoveryTarget?.agentId
    );
  };

  const getAgentActionId = (r: Resource) => {
    const discoveryAgentId = getAgentDiscoveryResourceId(r.discoveryTarget);
    if (discoveryAgentId) return discoveryAgentId;
    if (r.discoveryTarget?.agentId) {
      return r.discoveryTarget.agentId;
    }
    return getAgentId(r);
  };

  const getDockerActionId = (r: Resource) => {
    const platformData = pd(r);
    if (
      isAppContainerDiscoveryResourceType(r.discoveryTarget?.resourceType) &&
      r.discoveryTarget.resourceId
    ) {
      return r.discoveryTarget.resourceId;
    }
    return (
      asString(platformDocker(r)?.hostSourceId) ||
      asString(platformData?.hostSourceId) ||
      (r.type === 'docker-host' ? r.discoveryTarget?.agentId || r.id : undefined)
    );
  };

  const getKubernetesActionId = (r: Resource) => {
    const platformData = pd(r);
    const kubernetes = platformKubernetes(r);
    if (r.discoveryTarget?.resourceType === 'k8s' && r.discoveryTarget.resourceId) {
      return r.discoveryTarget.resourceId;
    }
    return asString(kubernetes?.clusterId) || asString(platformData?.clusterId) || r.id;
  };

  const getKubernetesAgentId = (r: Resource) => {
    const kubernetes = platformKubernetes(r);
    return (
      asString(kubernetes?.agentId) || r.discoveryTarget?.agentId || asString(kubernetes?.clusterId)
    );
  };

  const getAgentVersion = (r: Resource) => {
    const platformData = pd(r);
    return (
      r.agent?.agentVersion ||
      asString(platformAgent(r)?.agentVersion) ||
      asString(platformData?.agentVersion)
    );
  };

  const getDockerVersion = (r: Resource) => {
    const platformData = pd(r);
    return (
      asString(platformDocker(r)?.agentVersion) ||
      asString(platformDocker(r)?.dockerVersion) ||
      asString(platformData?.dockerVersion)
    );
  };

  const getCommandsEnabled = (r: Resource) => {
    const platformData = pd(r);
    return (
      r.agent?.commandsEnabled ??
      asBoolean(platformAgent(r)?.commandsEnabled) ??
      asBoolean(platformData?.commandsEnabled)
    );
  };

  const getLinkedNodeId = (r: Resource) => asString(pd(r)?.linkedNodeId);
  const getIsOutdatedBinary = (r: Resource) => asBoolean(pd(r)?.isLegacy);
  const hasDockerSource = (r: Resource) =>
    r.type === 'docker-host' ||
    r.platformType === 'docker' ||
    Boolean(platformDocker(r)) ||
    Boolean(getDockerActionId(r));

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
  const [profiles, setProfiles] = createSignal<AgentProfile[]>([]);
  const [assignments, setAssignments] = createSignal<AgentProfileAssignment[]>([]);
  // Track pending command config changes: agentId -> { desired value, timestamp }
  const [pendingCommandConfig, setPendingCommandConfig] = createSignal<
    Record<string, { enabled: boolean; timestamp: number }>
  >({});
  const [pendingScopeUpdates, setPendingScopeUpdates] = createSignal<Record<string, boolean>>({});
  const [expandedRowKey, setExpandedRowKey] = createSignal<string | null>(null);
  const [filterCapability, setFilterCapability] = createSignal<'all' | AgentCapability>('all');
  const [filterScope, setFilterScope] = createSignal<'all' | Exclude<ScopeCategory, 'na'>>('all');
  const [filterSearch, setFilterSearch] = createSignal('');

  createEffect(() => {
    if (requiresToken()) {
      setConfirmedNoToken(false);
    } else {
      setCurrentToken(null);
      setLatestRecord(null);
    }
  });

  // Use agentUrl from API (PULSE_PUBLIC_URL) if configured, otherwise fall back to window.location
  const agentUrl = () => securityStatus()?.agentUrl || getPulseBaseUrl();

  const commandSections = createMemo(() => {
    const url = customAgentUrl() || agentUrl();
    const commands = buildCommandsByPlatform(url);
    return Object.entries(commands).map(([platform, meta]) => ({
      platform: platform as AgentPlatform,
      ...meta,
    }));
  });

  onMount(() => {
    if (typeof window === 'undefined') {
      return;
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

    const fetchAgentProfiles = async () => {
      try {
        const [profilesData, assignmentsData] = await Promise.all([
          AgentProfilesAPI.listProfiles(),
          AgentProfilesAPI.listAssignments(),
        ]);
        setProfiles(profilesData);
        setAssignments(assignmentsData);
      } catch (err) {
        logger.debug('Failed to load agent profiles', err);
        setProfiles([]);
        setAssignments([]);
      }
    };
    fetchAgentProfiles();
  });

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

  const resolvedToken = () => {
    if (requiresToken()) {
      return currentToken() || TOKEN_PLACEHOLDER;
    }
    return currentToken() || 'disabled';
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
    return command.replace(
      /\|\s*bash -s --\s+(.+)$/,
      '| { if [ "$(id -u)" -eq 0 ]; then bash -s -- $1; elif command -v sudo >/dev/null 2>&1; then sudo bash -s -- $1; else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi; }',
    );
  };
  const getInsecureFlag = () => (insecureMode() ? ' --insecure' : '');
  const getEnableCommandsFlag = () => (enableCommands() ? ' --enable-commands' : '');
  const getCurlFlags = () => (insecureMode() ? '-kfsSL' : '-fsSL');
  const getSelectedInstallProfile = () =>
    INSTALL_PROFILE_OPTIONS.find((option) => option.value === installProfile()) ??
    INSTALL_PROFILE_OPTIONS[0];
  const getInstallProfileFlags = () => getSelectedInstallProfile().flags;
  const handleInstallProfileChange = (profile: InstallProfile) => {
    setInstallProfile(profile);
    trackAgentInstallProfileSelected(UNIFIED_AGENT_TELEMETRY_SURFACE, profile);
  };

  const getUninstallCommand = () => {
    const url = customAgentUrl() || agentUrl();
    const token = currentToken() || latestRecord()?.id;
    const insecure = insecureMode() ? ' --insecure' : '';
    // Only include token if we have a real one - the uninstall script works without it
    // Avoid including <api-token> placeholder which causes shell syntax errors
    const baseArgs = token
      ? `--uninstall --url ${url} --token ${token}${insecure}`
      : `--uninstall --url ${url}${insecure}`;
    return withPrivilegeEscalation(
      `curl ${getCurlFlags()} ${url}/install.sh | bash -s -- ${baseArgs}`,
    );
  };

  const getWindowsUninstallCommand = () => {
    const url = customAgentUrl() || agentUrl();
    const token = currentToken() || latestRecord()?.id;
    // Include URL and token for server notification (removes agent from dashboard)
    if (token) {
      return `$env:PULSE_URL="${url}"; $env:PULSE_TOKEN="${token}"; $env:PULSE_UNINSTALL="true"; irm $env:PULSE_URL/install.ps1 | iex`;
    }
    return `$env:PULSE_UNINSTALL="true"; irm ${url}/install.ps1 | iex`;
  };

  /** Derive agent capabilities from a v6 unified resource. */
  const getCapabilities = (r: Resource): AgentCapability[] => {
    const caps: AgentCapability[] = [];
    if (r.agent) caps.push('agent');
    if (hasDockerSource(r)) caps.push('docker');
    if (r.type === 'agent' || r.type === 'pbs' || r.type === 'pmg' || r.proxmox)
      caps.push('proxmox');
    return caps;
  };

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
  ) => {
    mutateResources((prev) =>
      prev.filter((resource) => !matchesRemovedAgent(resource, ids, capabilities)),
    );
    void refetchResources().catch((err) => {
      logger.debug('Failed to refresh resources after agent removal', err);
    });
  };

  const reconcileRemovedKubernetesCluster = (clusterId: string) => {
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
      .filter((r) => r.agent != null || r.type === 'docker-host')
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
      if (!agentResource.agent) continue;
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
      notificationStore.error('Failed to update agent scope');
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
    setExpandedRowKey((prev) => (prev === rowKey ? null : rowKey));
  };

  const outdatedAgents = createMemo(() => agentResources().filter((r) => getIsOutdatedBinary(r)));
  const hasOutdatedAgents = createMemo(() => outdatedAgents().length > 0);

  const removedDockerHosts = createMemo(() => {
    const removed = state.removedDockerHosts || [];
    return removed.sort((a, b) => b.removedAt - a.removedAt);
  });

  const removedDockerHostIds = createMemo(() => {
    return new Set(
      removedDockerHosts()
        .map((runtime) => runtime.id?.trim())
        .filter((id): id is string => Boolean(id)),
    );
  });

  const kubernetesClusters = createMemo(() => {
    const map = new Map<
      string,
      {
        id: string;
        actionClusterId: string;
        name: string;
        displayName?: string;
        customDisplayName?: string;
        status?: string;
        lastSeen?: number;
        version?: string;
        agentVersion?: string;
        agentId?: string;
        server?: string;
        context?: string;
        tokenName?: string;
      }
    >();

    byType('k8s-cluster').forEach((cluster) => {
      const kubernetes = platformKubernetes(cluster);
      const actionClusterId = getKubernetesActionId(cluster);
      const name =
        cluster.displayName ||
        asString(kubernetes?.clusterName) ||
        cluster.name ||
        actionClusterId ||
        cluster.id;
      const agentVersion = getAgentVersion(cluster);
      const key = actionClusterId || cluster.id;
      map.set(key, {
        id: cluster.id,
        actionClusterId: actionClusterId || cluster.id,
        name,
        displayName: cluster.displayName,
        customDisplayName: cluster.displayName,
        status: cluster.status,
        lastSeen: cluster.lastSeen,
        version: agentVersion,
        agentVersion,
        agentId: getKubernetesAgentId(cluster),
        server: asString(kubernetes?.server),
        context: asString(kubernetes?.context),
        tokenName: asString(platformAgent(cluster)?.tokenName) || asString(pd(cluster)?.tokenName),
      });
    });

    return Array.from(map.values()).sort((a, b) =>
      getPreferredNamedEntityLabel({
        id: a.actionClusterId,
        displayName: a.displayName,
        name: a.name,
      }).localeCompare(
        getPreferredNamedEntityLabel({
          id: b.actionClusterId,
          displayName: b.displayName,
          name: b.name,
        }),
      ),
    );
  });

  const removedKubernetesClusters = createMemo(() => {
    const removed = state.removedKubernetesClusters || [];
    return removed.sort((a, b) => b.removedAt - a.removedAt);
  });

  // Agents linked to PVE nodes (shown separately with unlink option)
  const linkedAgents = createMemo(() => {
    return agentResources().flatMap((r) => {
      const linkedNodeId = getLinkedNodeId(r);
      if (!linkedNodeId) return [];

      const hostname = getPreferredResourceHostname(r) || 'Unknown';
      const version = getAgentVersion(r);
      return [
        {
          id: r.id,
          hostname,
          displayName: r.displayName,
          linkedNodeId,
          status: r.status,
          version: version || undefined,
          lastSeen: r.lastSeen ? new Date(r.lastSeen).getTime() : undefined,
        },
      ];
    });
  });
  const hasLinkedAgents = createMemo(() => linkedAgents().length > 0);
  const showInstaller = () => props.showInstaller ?? true;
  const showInventory = () => props.showInventory ?? true;

  const unifiedRows = createMemo<UnifiedAgentRow[]>(() => {
    const rows: UnifiedAgentRow[] = [];

    // Build rows directly from v6 unified resources that have agents
    agentResources().forEach((r) => {
      const hostname = getPreferredResourceHostname(r) || 'Unknown';
      const agentId = getAgentId(r);
      const resolvedAgentId = agentId || getAgentActionId(r);
      const scopeInfo = getScopeInfo(resolvedAgentId);
      const agentActionId = getAgentActionId(r);
      const dockerActionId = hasDockerSource(r) ? getDockerActionId(r) : undefined;
      if (dockerActionId && removedDockerHostIds().has(dockerActionId)) {
        return;
      }
      const name = getPreferredResourceDisplayName(r);
      const searchText = [name, hostname, r.id, resolvedAgentId, agentActionId, dockerActionId]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();

      rows.push({
        rowKey: `agent-${r.id}`,
        id: r.id,
        agentActionId,
        dockerActionId,
        name,
        hostname,
        displayName: r.displayName,
        capabilities: getCapabilities(r),
        status: 'active',
        healthStatus: r.status,
        lastSeen: r.lastSeen,
        version: getAgentVersion(r) || getDockerVersion(r),
        isOutdatedBinary: getIsOutdatedBinary(r),
        linkedNodeId: getLinkedNodeId(r),
        commandsEnabled: getCommandsEnabled(r),
        agentId: resolvedAgentId,
        scope: scopeInfo,
        searchText,
      });
    });

    kubernetesClusters().forEach((cluster) => {
      const name =
        cluster.customDisplayName || cluster.displayName || cluster.name || cluster.actionClusterId;
      const scopeAgentId = cluster.agentId;
      rows.push({
        rowKey: `k8s-${cluster.actionClusterId}`,
        id: cluster.id,
        kubernetesActionId: cluster.actionClusterId,
        name,
        capabilities: ['kubernetes'],
        status: 'active',
        healthStatus: cluster.status,
        lastSeen: cluster.lastSeen,
        version: cluster.version || cluster.agentVersion,
        agentId: scopeAgentId,
        scope: getScopeInfo(scopeAgentId),
        searchText: [
          name,
          cluster.name,
          cluster.displayName,
          cluster.id,
          cluster.actionClusterId,
          scopeAgentId,
          cluster.server,
          cluster.context,
        ]
          .filter(Boolean)
          .join(' ')
          .toLowerCase(),
        kubernetesInfo: {
          server: cluster.server,
          context: cluster.context,
          tokenName: cluster.tokenName,
        },
      });
    });

    removedDockerHosts().forEach((runtime) => {
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
        scope: getScopeInfo(undefined),
        searchText: [name, runtime.hostname, runtime.id].filter(Boolean).join(' ').toLowerCase(),
      });
    });

    removedKubernetesClusters().forEach((cluster) => {
      const name = getPreferredNamedEntityLabel(cluster);
      rows.push({
        rowKey: `removed-k8s-${cluster.id}`,
        id: cluster.id,
        kubernetesActionId: cluster.id,
        name,
        capabilities: ['kubernetes'],
        status: 'removed',
        removedAt: cluster.removedAt,
        scope: getScopeInfo(undefined),
        searchText: [name, cluster.name, cluster.id].filter(Boolean).join(' ').toLowerCase(),
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

  const filteredActiveRows = createMemo(() => activeRows().filter(matchesInventoryFilters));
  const filteredMonitoringStoppedRows = createMemo(() =>
    monitoringStoppedRows().filter(matchesInventoryFilters),
  );

  const hasFilters = createMemo(() => {
    return (
      filterCapability() !== 'all' || filterScope() !== 'all' || filterSearch().trim().length > 0
    );
  });

  const hasMonitoringStoppedRows = createMemo(() => monitoringStoppedRows().length > 0);
  const showMonitoringStoppedSection = createMemo(() => {
    return hasMonitoringStoppedRows() || hasFilters();
  });

  const resetFilters = () => {
    setFilterCapability('all');
    setFilterScope('all');
    setFilterSearch('');
  };

  const getUpgradeCommand = (_hostname: string) => {
    const token = resolvedToken();
    const url = customAgentUrl() || agentUrl();
    return withPrivilegeEscalation(
      `curl ${getCurlFlags()} ${url}/install.sh | bash -s -- --url ${url} --token ${token}${getInsecureFlag()}`,
    );
  };

  const handleRemoveAgent = async (
    ids: { agentId?: string; dockerId?: string },
    capabilities: AgentCapability[],
    subjectLabel?: string,
  ) => {
    const subject = getInventorySubjectLabel(subjectLabel, 'this host');
    if (
      !confirm(
        `Stop monitoring ${subject} in Pulse? The host will keep running remotely, but Pulse will ignore future reports until reconnect is allowed.`,
      )
    )
      return;

    try {
      let removed = false;
      // Remove the agent registration
      if (capabilities.includes('agent') && ids.agentId) {
        await MonitoringAPI.deleteAgent(ids.agentId);
        removed = true;
      }
      // Remove docker runtime registration if present
      if (capabilities.includes('docker') && ids.dockerId) {
        await MonitoringAPI.deleteDockerRuntime(ids.dockerId, { force: true });
        removed = true;
      }
      if (removed) {
        reconcileRemovedAgent(ids, capabilities);
        notificationStore.success(
          `Monitoring stopped for ${subject}. Pulse will ignore future reports until reconnect is allowed.`,
        );
      } else {
        notificationStore.error('No host identifiers are available to stop monitoring.');
      }
    } catch (err) {
      logger.error('Failed to stop monitoring host', err);
      notificationStore.error(`Failed to stop monitoring ${subject}.`);
    }
  };

  const handleAllowReconnect = async (agentId: string, hostname?: string) => {
    const subject = getInventorySubjectLabel(hostname, agentId);
    try {
      await MonitoringAPI.allowDockerRuntimeReenroll(agentId);
      notificationStore.success(
        `Reconnect allowed for ${subject}. Pulse will accept reports from it again.`,
      );
    } catch (err) {
      logger.error('Failed to allow reconnect for host', err);
      notificationStore.error(`Failed to allow reconnect for ${subject}.`);
    }
  };

  const handleRemoveKubernetesCluster = async (clusterId: string, clusterName?: string) => {
    const subject = getInventorySubjectLabel(clusterName, 'this cluster');
    if (
      !confirm(
        `Stop monitoring ${subject} in Pulse? The cluster will keep running, but Pulse will ignore future reports until reconnect is allowed.`,
      )
    )
      return;

    try {
      await MonitoringAPI.deleteKubernetesCluster(clusterId);
      reconcileRemovedKubernetesCluster(clusterId);
      notificationStore.success(
        `Monitoring stopped for ${subject}. Pulse will ignore future reports until reconnect is allowed.`,
      );
    } catch (err) {
      logger.error('Failed to stop monitoring kubernetes cluster', err);
      notificationStore.error(`Failed to stop monitoring ${subject}.`);
    }
  };

  const handleAllowKubernetesReconnect = async (clusterId: string, name?: string) => {
    const subject = getInventorySubjectLabel(name, clusterId);
    try {
      await MonitoringAPI.allowKubernetesClusterReenroll(clusterId);
      notificationStore.success(
        `Reconnect allowed for ${subject}. Pulse will accept reports from it again.`,
      );
    } catch (err) {
      logger.error('Failed to allow reconnect for kubernetes cluster', err);
      notificationStore.error(`Failed to allow reconnect for ${subject}.`);
    }
  };

  const handleToggleCommands = async (agentId: string | undefined, enabled: boolean) => {
    if (!agentId) {
      notificationStore.error('Agent ID unavailable for command configuration');
      return;
    }
    // Set optimistic/pending state immediately with timestamp
    setPendingCommandConfig((prev) => ({
      ...prev,
      [agentId]: { enabled, timestamp: Date.now() },
    }));

    try {
      await MonitoringAPI.updateAgentConfig(agentId, { commandsEnabled: enabled });
      notificationStore.success(
        `Pulse command execution ${enabled ? 'enabled' : 'disabled'}. Syncing with agent...`,
      );
    } catch (err) {
      // On error, clear the pending state so toggle reverts
      setPendingCommandConfig((prev) => {
        const next = { ...prev };
        delete next[agentId];
        return next;
      });
      logger.error('Failed to toggle AI commands', err);
      notificationStore.error('Failed to update agent configuration');
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

  return (
    <div class="space-y-6">
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
            <Show when={requiresToken()}>
              <div class="space-y-3">
                <div class="space-y-1">
                  <p class="text-sm font-semibold text-base-content">
                    <span class="inline-flex items-center justify-center w-5 h-5 mr-1.5 rounded-full bg-blue-600 text-white text-xs font-bold">
                      1
                    </span>
                    Generate API token
                  </p>
                  <p class="text-sm text-muted ml-6">
                    Create a fresh token scoped for Agent, Docker, and Kubernetes monitoring.
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
            </Show>

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

            {/* Show locked step 2 preview when token required but not yet generated */}
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
                              const copyCommand = () => {
                                let cmd = snippet.command.replace(
                                  TOKEN_PLACEHOLDER,
                                  resolvedToken(),
                                );
                                // Insert -k flag for curl if insecure mode enabled (issue #806)
                                if (insecureMode() && cmd.includes('curl -fsSL')) {
                                  cmd = cmd.replace('curl -fsSL', 'curl -kfsSL');
                                }
                                // For bash scripts (not PowerShell), append insecure flag
                                const isBashScript = !cmd.includes('$env:') && !cmd.includes('irm');
                                if (isBashScript) {
                                  const profileFlags = getInstallProfileFlags();
                                  if (profileFlags.length > 0) {
                                    cmd += ` ${profileFlags.join(' ')}`;
                                  }
                                  if (insecureMode()) {
                                    cmd += getInsecureFlag();
                                  }
                                  // Add --enable-commands flag if enabled (issue #903)
                                  if (enableCommands()) {
                                    cmd += getEnableCommandsFlag();
                                  }
                                  cmd = withPrivilegeEscalation(cmd);
                                }
                                return cmd;
                              };
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
                                          notificationStore.success('Copied to clipboard');
                                        } else {
                                          notificationStore.error('Failed to copy');
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
                      const statusBadgeClasses = () =>
                        agent().connected
                          ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
                          : 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200';
                      return (
                        <div class="space-y-1 rounded-md border border-blue-200 bg-surface px-3 py-2 text-xs text-blue-900 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100">
                          <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                            <div class="text-sm font-semibold">
                              {agent().displayName || agent().hostname}
                            </div>
                            <div class="flex items-center gap-2">
                              <span
                                class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold ${statusBadgeClasses()}`}
                              >
                                {agent().connected ? 'Connected' : 'Not reporting yet'}
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
                          <code class="bg-surface-hover px-1 rounded">--enable-proxmox</code> —
                          Force enable Proxmox integration (creates API token)
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

            {/* Uninstall section - always visible */}
            <div class="border-t border-border pt-4 mt-4">
              <div class="space-y-3">
                <h4 class="text-sm font-semibold text-base-content">Uninstall agent</h4>
                <p class="text-xs text-muted">
                  Run the appropriate command on your machine to remove the Pulse agent:
                </p>
                {/* Linux/macOS uninstall */}
                <div class="space-y-1">
                  <span class="text-xs font-medium text-muted">Linux / macOS / FreeBSD</span>
                  <div class="relative">
                    <button
                      type="button"
                      onClick={async () => {
                        const success = await copyToClipboard(getUninstallCommand());
                        if (success) {
                          notificationStore.success('Copied to clipboard');
                        } else {
                          notificationStore.error('Failed to copy');
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
                {/* Windows uninstall */}
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
                          notificationStore.success('Copied to clipboard');
                        } else {
                          notificationStore.error('Failed to copy');
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

      <Show when={showInventory()}>
        <div class="space-y-6">
          <div class="flex flex-wrap items-center gap-3 rounded-md border border-border bg-surface-alt px-4 py-3 text-sm">
            <span class="inline-flex items-center rounded-full bg-surface px-3 py-1 text-xs font-medium text-base-content">
              {filteredActiveRows().length} active
            </span>
            <span class="inline-flex items-center rounded-full bg-amber-100 px-3 py-1 text-xs font-medium text-amber-800 dark:bg-amber-900 dark:text-amber-200">
              {filteredMonitoringStoppedRows().length} monitoring stopped
            </span>
            <span class="text-muted">
              Stopping monitoring in Pulse does not uninstall software on the remote system.
            </span>
          </div>

          <SettingsPanel
            title="Connected infrastructure"
            description="Review infrastructure currently reporting to Pulse, including Docker and Kubernetes coverage."
            icon={<Users class="w-5 h-5" strokeWidth={2} />}
            bodyClass="space-y-4"
          >
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
                      {outdatedAgents().length} outdated agent binary
                      {outdatedAgents().length > 1 ? 'ies' : ''} detected
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
                  Search connected infrastructure
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
                  {filteredMonitoringStoppedRows().length} item(s) are in the recovery queue.
                </span>
              </Show>
            </div>

            <div>
              <PulseDataGrid
                data={filteredActiveRows()}
                emptyState={
                  hasFilters()
                    ? 'No connected infrastructure matches the current filters.'
                    : 'No infrastructure connected yet.'
                }
                desktopMinWidth="960px"
                columns={[
                  {
                    key: 'name',
                    label: 'Name',
                    render: (row: UnifiedAgentRow) => {
                      const expanded = () => expandedRowKey() === row.rowKey;
                      const agentName = row.displayName || row.hostname || row.name;
                      return (
                        <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
                          <div class="min-w-0 text-left">
                            <div class="truncate text-sm font-medium text-base-content">
                              {row.name}
                            </div>
                            <Show
                              when={
                                row.displayName && row.hostname && row.displayName !== row.hostname
                              }
                            >
                              <div class="truncate text-xs text-muted">{row.hostname}</div>
                            </Show>
                          </div>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              toggleAgentDetails(row.rowKey);
                            }}
                            class="inline-flex min-h-10 min-w-10 sm:min-h-9 sm:min-w-9 items-center justify-center rounded-md p-2 hover:text-base-content"
                            aria-label={`${expanded() ? 'Hide' : 'Show'} details for ${agentName}`}
                            aria-expanded={expanded()}
                            aria-controls={`agent-details-${row.rowKey}`}
                          >
                            <svg
                              class={`h-4 w-4 transition-transform ${expanded() ? 'rotate-180' : ''}`}
                              viewBox="0 0 20 20"
                              fill="currentColor"
                            >
                              <path
                                fill-rule="evenodd"
                                d="M5.23 7.21a.75.75 0 011.06.02L10 10.94l3.71-3.7a.75.75 0 111.06 1.06l-4.24 4.24a.75.75 0 01-1.06 0L5.21 8.29a.75.75 0 01.02-1.08z"
                                clip-rule="evenodd"
                              />
                            </svg>
                          </button>
                        </div>
                      );
                    },
                  },
                  {
                    key: 'capabilities',
                    label: 'Capabilities',
                    render: (row: UnifiedAgentRow) => (
                      <div class="flex flex-wrap items-center gap-2 text-xs">
                        <For each={row.capabilities}>
                          {(cap) => (
                            <span
                              class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${getAgentCapabilityBadgeClass(cap)}`}
                            >
                              {getAgentCapabilityLabel(cap)}
                            </span>
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
                    key: 'scope',
                    label: 'Scope',
                    render: (row: UnifiedAgentRow) => {
                      const isActive = () => row.status === 'active';
                      const isKubernetes = () =>
                        row.capabilities.includes('kubernetes') &&
                        !row.capabilities.includes('agent');
                      const resolvedAgentId = row.agentId || '';
                      const assignment = () =>
                        resolvedAgentId ? assignmentByAgent().get(resolvedAgentId) : undefined;
                      const isScopeUpdating = () =>
                        resolvedAgentId ? Boolean(pendingScopeUpdates()[resolvedAgentId]) : false;
                      const agentName = row.displayName || row.hostname || row.name;

                      return (
                        <Show
                          when={isActive() && resolvedAgentId}
                          fallback={<span class="text-xs text-muted">N/A</span>}
                        >
                          <Show
                            when={isKubernetes()}
                            fallback={
                              <Show
                                when={profiles().length > 0}
                                fallback={
                                  <span class="text-base-content" title={row.scope.detail}>
                                    {row.scope.label}
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
                                        resolvedAgentId,
                                        nextValue || null,
                                        agentName,
                                      );
                                    }}
                                    disabled={isScopeUpdating()}
                                    class="min-h-10 sm:min-h-9 rounded-md border border-border bg-surface px-2.5 py-1.5 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                                  >
                                    <option value="">Default (Auto-detect)</option>
                                    <For each={profiles()}>
                                      {(profile) => (
                                        <option value={profile.id}>
                                          {profile.name || profile.id}
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
                            <span class="text-base-content" title={row.scope.detail}>
                              {row.scope.label}
                            </span>
                          </Show>
                        </Show>
                      );
                    },
                  },
                  {
                    key: 'commandsEnabled',
                    label: 'Pulse Cmds',
                    render: (row: UnifiedAgentRow) => {
                      const isActive = () => row.status === 'active';
                      const configAgentId = row.agentActionId;

                      return (
                        <Show
                          when={isActive() && row.capabilities.includes('agent') && configAgentId}
                          fallback={<span class="text-xs text-muted">N/A</span>}
                        >
                          {(() => {
                            const pending = pendingCommandConfig();
                            const isPending = Boolean(configAgentId && configAgentId in pending);
                            const effectiveEnabled =
                              configAgentId && isPending
                                ? pending[configAgentId].enabled
                                : Boolean(row.commandsEnabled);

                            return (
                              <div class="flex items-center gap-2">
                                <button
                                  onClick={() =>
                                    handleToggleCommands(configAgentId, !effectiveEnabled)
                                  }
                                  disabled={isPending}
                                  class={`relative inline-flex h-8 w-12 sm:h-7 sm:w-12 flex-shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 ${
                                    isPending ? 'opacity-60 cursor-wait' : ''
                                  } ${effectiveEnabled ? 'bg-blue-600' : 'bg-surface-hover'}`}
                                  title={
                                    isPending
                                      ? 'Syncing with agent...'
                                      : effectiveEnabled
                                        ? 'Pulse command execution enabled'
                                        : 'Pulse command execution disabled'
                                  }
                                >
                                  <span
                                    class={`pointer-events-none inline-block h-6 w-6 sm:h-5 sm:w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
                                      effectiveEnabled
                                        ? 'translate-x-4 sm:translate-x-5'
                                        : 'translate-x-0'
                                    }`}
                                  />
                                </button>
                                <Show when={isPending}>
                                  <svg
                                    class="animate-spin h-4 w-4 text-blue-500"
                                    xmlns="http://www.w3.org/2000/svg"
                                    fill="none"
                                    viewBox="0 0 24 24"
                                  >
                                    <circle
                                      class="opacity-25"
                                      cx="12"
                                      cy="12"
                                      r="10"
                                      stroke="currentColor"
                                      stroke-width="4"
                                    ></circle>
                                    <path
                                      class="opacity-75"
                                      fill="currentColor"
                                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                                    ></path>
                                  </svg>
                                </Show>
                              </div>
                            );
                          })()}
                        </Show>
                      );
                    },
                  },
                  {
                    key: 'lastSeen',
                    label: 'Last Seen',
                    render: (row: UnifiedAgentRow) => {
                      const isRemoved = () => row.status === 'removed';
                      const lastSeenLabel = () => {
                        if (isRemoved()) {
                          return row.removedAt
                            ? `Monitoring stopped ${formatRelativeTime(row.removedAt)}`
                            : MONITORING_STOPPED_STATUS_LABEL;
                        }
                        return row.lastSeen ? formatRelativeTime(row.lastSeen) : '—';
                      };
                      return <span class="text-xs text-muted">{lastSeenLabel()}</span>;
                    },
                  },
                  {
                    key: 'version',
                    label: 'Version',
                    render: (row: UnifiedAgentRow) => (
                      <span class="text-xs text-muted">{row.version || '—'}</span>
                    ),
                  },
                  {
                    key: 'actions',
                    label: 'Actions',
                    align: 'right',
                    render: (row: UnifiedAgentRow) => {
                      const isRemoved = () => row.status === 'removed';
                      const isKubernetes = () =>
                        row.capabilities.includes('kubernetes') &&
                        !row.capabilities.includes('agent');
                      const canRemove = () => {
                        const needsAgent = row.capabilities.includes('agent') && !row.agentActionId;
                        const needsDocker =
                          row.capabilities.includes('docker') &&
                          !row.dockerActionId &&
                          !row.agentActionId;
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
                                  onClick={() =>
                                    handleRemoveAgent(
                                      { agentId: row.agentActionId, dockerId: row.dockerActionId },
                                      row.capabilities,
                                      row.name,
                                    )
                                  }
                                  disabled={!canRemove()}
                                  title={
                                    !canRemove() ? 'Agent ID unavailable for removal' : undefined
                                  }
                                  class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 hover:text-red-900 disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400 dark:hover:bg-red-900 dark:hover:text-red-300"
                                >
                                  Stop monitoring
                                </button>
                              }
                            >
                              <button
                                onClick={() =>
                                  handleRemoveKubernetesCluster(
                                    row.kubernetesActionId || row.id,
                                    row.name,
                                  )
                                }
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 hover:text-red-900 dark:text-red-400 dark:hover:bg-red-900 dark:hover:text-red-300"
                              >
                                Stop monitoring
                              </button>
                            </Show>
                          }
                        >
                          <Show
                            when={row.capabilities.includes('docker')}
                            fallback={
                              <button
                                onClick={() =>
                                  handleAllowKubernetesReconnect(
                                    row.kubernetesActionId || row.id,
                                    row.name,
                                  )
                                }
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-blue-600 hover:bg-blue-50 hover:text-blue-900 dark:text-blue-400 dark:hover:bg-blue-900 dark:hover:text-blue-300"
                              >
                                {ALLOW_RECONNECT_LABEL}
                              </button>
                            }
                          >
                            <button
                              onClick={() =>
                                handleAllowReconnect(
                                  row.dockerActionId || row.id,
                                  row.displayName || row.hostname || row.name,
                                )
                              }
                              class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2.5 py-1.5 text-sm font-medium text-blue-600 hover:bg-blue-50 hover:text-blue-900 dark:text-blue-400 dark:hover:bg-blue-900 dark:hover:text-blue-300"
                            >
                              {ALLOW_RECONNECT_LABEL}
                            </button>
                          </Show>
                        </Show>
                      );
                    },
                  },
                ]}
                keyExtractor={(row) => row.rowKey}
                onRowClick={(row) => toggleAgentDetails(row.rowKey)}
                isRowExpanded={(row) => expandedRowKey() === row.rowKey}
                expandedRender={(row: UnifiedAgentRow) => {
                  const isKubernetes = () =>
                    row.capabilities.includes('kubernetes') && !row.capabilities.includes('agent');
                  const resolvedAgentId = row.agentId || '';
                  const assignment = () =>
                    resolvedAgentId ? assignmentByAgent().get(resolvedAgentId) : undefined;
                  const agentName = row.displayName || row.hostname || row.name;

                  return (
                    <div id={`agent-details-${row.rowKey}`} class="px-4 py-4 text-sm text-muted">
                      <div class="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto]">
                        <div class="space-y-3">
                          <div class="flex flex-wrap items-center gap-2 text-xs">
                            <For each={row.capabilities}>
                              {(cap) => (
                                <span
                                  class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${getAgentCapabilityBadgeClass(cap)}`}
                                >
                                  {getAgentCapabilityLabel(cap)}
                                </span>
                              )}
                            </For>
                            <Show when={row.isOutdatedBinary}>
                              <span class="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                                Outdated
                              </span>
                            </Show>
                            <Show when={row.linkedNodeId}>
                              <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900 dark:text-blue-300">
                                Linked
                              </span>
                            </Show>
                          </div>
                          <div class="text-xs text-muted">
                            ID: <span class="font-mono text-base-content">{row.id}</span>
                          </div>
                          <Show when={row.agentActionId && row.agentActionId !== row.id}>
                            <div class="text-xs text-muted">
                              Agent ID:{' '}
                              <span class="font-mono text-base-content">{row.agentActionId}</span>
                            </div>
                          </Show>
                          <Show when={row.dockerActionId && row.dockerActionId !== row.id}>
                            <div class="text-xs text-muted">
                              Container Agent ID:{' '}
                              <span class="font-mono text-base-content">{row.dockerActionId}</span>
                            </div>
                          </Show>
                          <Show when={row.kubernetesActionId && row.kubernetesActionId !== row.id}>
                            <div class="text-xs text-muted">
                              Cluster ID:{' '}
                              <span class="font-mono text-base-content">
                                {row.kubernetesActionId}
                              </span>
                            </div>
                          </Show>
                          <Show when={row.agentId && row.agentId !== row.id}>
                            <div class="text-xs text-muted">
                              Agent ID:{' '}
                              <span class="font-mono text-base-content">{row.agentId}</span>
                            </div>
                          </Show>
                          <Show when={row.linkedNodeId}>
                            <div class="text-xs text-muted">
                              Linked node ID:{' '}
                              <span class="font-mono text-base-content">{row.linkedNodeId}</span>
                            </div>
                          </Show>
                          <Show when={row.status === 'active' && row.lastSeen}>
                            <div class="text-xs text-muted">
                              Last seen {formatRelativeTime(row.lastSeen)} (
                              {formatAbsoluteTime(row.lastSeen)})
                            </div>
                          </Show>
                          <Show when={row.status === 'removed' && row.removedAt}>
                            <div class="text-xs text-muted">
                              Monitoring stopped {formatRelativeTime(row.removedAt)} (
                              {formatAbsoluteTime(row.removedAt)})
                            </div>
                          </Show>
                          <Show when={row.status === 'removed'}>
                            <div class="text-xs text-muted">
                              Pulse is ignoring new reports from this item until reconnect is
                              allowed.
                            </div>
                          </Show>
                          <Show
                            when={
                              row.kubernetesInfo &&
                              (row.kubernetesInfo.server ||
                                row.kubernetesInfo.context ||
                                row.kubernetesInfo.tokenName)
                            }
                          >
                            <div class="space-y-1 text-xs text-muted">
                              <Show when={row.kubernetesInfo?.server}>
                                <div>
                                  Server:{' '}
                                  <span class="text-base-content">
                                    {row.kubernetesInfo?.server}
                                  </span>
                                </div>
                              </Show>
                              <Show when={row.kubernetesInfo?.context}>
                                <div>
                                  Context:{' '}
                                  <span class="text-base-content">
                                    {row.kubernetesInfo?.context}
                                  </span>
                                </div>
                              </Show>
                              <Show when={row.kubernetesInfo?.tokenName}>
                                <div>
                                  Token:{' '}
                                  <span class="text-base-content">
                                    {row.kubernetesInfo?.tokenName}
                                  </span>
                                </div>
                              </Show>
                            </div>
                          </Show>
                          <Show when={row.scope.category !== 'na'}>
                            <div class="text-xs text-muted">
                              Scope profile:{' '}
                              <span class="text-base-content">{row.scope.label}</span>
                              <Show when={row.scope.detail}>
                                <span class="ml-1 text-muted">{row.scope.detail}</span>
                              </Show>
                            </div>
                            <Show when={assignment()}>
                              <div class="text-[11px] text-amber-600 dark:text-amber-400">
                                Restart required to apply scope changes.
                              </div>
                              <button
                                type="button"
                                onClick={() =>
                                  handleResetScope(resolvedAgentId, agentName || resolvedAgentId)
                                }
                                class="text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300 text-left"
                              >
                                Reset to default
                              </button>
                            </Show>
                          </Show>
                        </div>
                        <div class="space-y-2 md:justify-self-end">
                          <div class="text-xs uppercase tracking-wide text-muted">Utilities</div>
                          <div class="flex flex-col gap-2">
                            <Show when={row.status === 'active' && !isKubernetes()}>
                              <button
                                type="button"
                                onClick={async () => {
                                  const cmd = getUninstallCommand();
                                  const success = await copyToClipboard(cmd);
                                  if (success) {
                                    notificationStore.success('Uninstall command copied');
                                  } else {
                                    notificationStore.error('Failed to copy');
                                  }
                                }}
                                class="text-xs text-slate-600 hover:text-base-content text-left"
                              >
                                Copy uninstall command
                              </button>
                            </Show>
                            <Show when={row.isOutdatedBinary}>
                              <button
                                type="button"
                                onClick={async () => {
                                  const success = await copyToClipboard(
                                    getUpgradeCommand(row.hostname || ''),
                                  );
                                  if (success) {
                                    notificationStore.success('Upgrade command copied');
                                  } else {
                                    notificationStore.error('Failed to copy');
                                  }
                                }}
                                class="text-xs text-amber-700 hover:text-amber-900 dark:text-amber-300 dark:hover:text-amber-200 text-left"
                              >
                                Copy upgrade command
                              </button>
                            </Show>
                          </div>
                        </div>
                      </div>
                    </div>
                  );
                }}
              />
            </div>
          </SettingsPanel>

          <Show when={showMonitoringStoppedSection()}>
            <SettingsPanel
              title="Recovery queue"
              description="Infrastructure with monitoring stopped stays out of active inventory until reconnect is allowed."
              icon={<Users class="w-5 h-5" strokeWidth={2} />}
              bodyClass="space-y-4"
            >
              <Show
                when={filteredMonitoringStoppedRows().length > 0}
                fallback={
                  <div class="rounded-md border border-dashed border-border px-4 py-6 text-sm text-muted">
                    {hasFilters()
                      ? 'No monitoring-stopped items match the current filters.'
                      : 'No infrastructure currently has monitoring stopped.'}
                  </div>
                }
              >
                <div class="rounded-lg border border-amber-200 bg-amber-50/70 dark:border-amber-800 dark:bg-amber-950/30">
                  <div class="border-b border-amber-200 px-4 py-3 text-xs text-amber-900 dark:border-amber-800 dark:text-amber-100">
                    Pulse is intentionally ignoring reports from these items. This does not
                    uninstall software on the remote system.
                  </div>
                  <div class="divide-y divide-amber-200/80 dark:divide-amber-800/80">
                    <For each={filteredMonitoringStoppedRows()}>
                      {(row) => (
                        <div class="flex flex-col gap-3 px-4 py-3 lg:flex-row lg:items-center lg:justify-between">
                          <div class="min-w-0 space-y-1">
                            <div class="flex flex-wrap items-center gap-2">
                              <h4 class="truncate text-sm font-semibold text-base-content">
                                {row.name}
                              </h4>
                              <span class="inline-flex items-center rounded-full bg-white/80 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide text-amber-800 dark:bg-amber-900/60 dark:text-amber-200">
                                {getRemovedItemLabel(row)}
                              </span>
                            </div>
                            <div class="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted">
                              <span>{row.capabilities.map(getAgentCapabilityLabel).join(', ')}</span>
                              <Show
                                when={
                                  row.displayName &&
                                  row.hostname &&
                                  row.displayName !== row.hostname
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
                          </div>

                          <div class="flex items-center gap-3 lg:flex-shrink-0">
                            <button
                              onClick={() =>
                                row.capabilities.includes('docker')
                                  ? handleAllowReconnect(
                                      row.dockerActionId || row.id,
                                      row.displayName || row.hostname || row.name,
                                    )
                                  : handleAllowKubernetesReconnect(
                                      row.kubernetesActionId || row.id,
                                      row.name,
                                    )
                              }
                              class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md bg-white px-3 py-1.5 text-sm font-medium text-blue-600 shadow-sm ring-1 ring-border hover:bg-blue-50 hover:text-blue-900 dark:bg-slate-900 dark:text-blue-400 dark:ring-slate-700 dark:hover:bg-blue-900 dark:hover:text-blue-300"
                            >
                              {ALLOW_RECONNECT_LABEL}
                            </button>
                            <span class="text-xs text-muted">
                              Ready to return to active monitoring
                            </span>
                          </div>
                        </div>
                      )}
                    </For>
                  </div>
                </div>
              </Show>
            </SettingsPanel>
          </Show>
        </div>
      </Show>
    </div>
  );
};
