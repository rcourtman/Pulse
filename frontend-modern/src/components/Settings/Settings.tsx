import {
  Component,
  createSignal,
  onMount,
  For,
  Show,
  createEffect,
  createMemo,
  onCleanup,
  on,
} from 'solid-js';
import { useNavigate, useLocation } from '@solidjs/router';
import { useWebSocket } from '@/App';
import { showSuccess, showError, showWarning } from '@/utils/toast';
import { copyToClipboard } from '@/utils/clipboard';
import { logger } from '@/utils/logger';
import {
  apiFetch,
  apiFetchJSON,
  clearApiToken as clearApiClientToken,
  getApiToken as getApiClientToken,
  setApiToken as setApiClientToken,
} from '@/utils/apiClient';
import { NodeModal } from './NodeModal';
import { ChangePasswordModal } from './ChangePasswordModal';
import { UnifiedAgents } from './UnifiedAgents';
import APITokenManager from './APITokenManager';
import { OIDCPanel } from './OIDCPanel';
import { AISettings } from './AISettings';
import { AICostDashboard } from '@/components/AI/AICostDashboard';
import { QuickSecuritySetup } from './QuickSecuritySetup';
import { SecurityPostureSummary } from './SecurityPostureSummary';
import { DiagnosticsPanel } from './DiagnosticsPanel';
import {
  PveNodesTable,
  PbsNodesTable,
  PmgNodesTable,
  type TemperatureTransportInfo,
} from './ConfiguredNodeTables';
import { SettingsSectionNav } from './SettingsSectionNav';
import { SettingsAPI } from '@/api/settings';
import { NodesAPI } from '@/api/nodes';
import { UpdatesAPI } from '@/api/updates';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import Server from 'lucide-solid/icons/server';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Mail from 'lucide-solid/icons/mail';
import Shield from 'lucide-solid/icons/shield';
import Lock from 'lucide-solid/icons/lock';
import Key from 'lucide-solid/icons/key';
import Activity from 'lucide-solid/icons/activity';
import Loader from 'lucide-solid/icons/loader';
import Network from 'lucide-solid/icons/network';
import Monitor from 'lucide-solid/icons/monitor';
import Sliders from 'lucide-solid/icons/sliders-horizontal';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Clock from 'lucide-solid/icons/clock';
import Sparkles from 'lucide-solid/icons/sparkles';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import type { NodeConfig } from '@/types/nodes';
import type { UpdateInfo, VersionInfo } from '@/api/updates';
import type { APITokenRecord } from '@/api/security';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { showTokenReveal } from '@/stores/tokenReveal';
import { updateStore } from '@/stores/updates';

const COMMON_DISCOVERY_SUBNETS = [
  '192.168.1.0/24',
  '192.168.0.0/24',
  '10.0.0.0/24',
  '172.16.0.0/24',
  '192.168.10.0/24',
];
// Type definitions
interface DiscoveredServer {
  ip: string;
  port: number;
  type: 'pve' | 'pbs' | 'pmg';
  version: string;
  hostname?: string;
  release?: string;
}

type RawDiscoveredServer = {
  ip?: string;
  port?: number;
  type?: string;
  version?: string;
  hostname?: string;
  name?: string;
  release?: string;
};

interface ClusterEndpoint {
  Host?: string;
  IP?: string;
}

interface DiagnosticsNode {
  id: string;
  name: string;
  host: string;
  type: string;
  authMethod: string;
  connected: boolean;
  error?: string;
  details?: Record<string, unknown>;
  lastPoll?: string;
  clusterInfo?: Record<string, unknown>;
}

interface DiagnosticsPBS {
  id: string;
  name: string;
  host: string;
  connected: boolean;
  error?: string;
  details?: Record<string, unknown>;
}

interface SystemDiagnostic {
  os: string;
  arch: string;
  goVersion: string;
  numCPU: number;
  numGoroutine: number;
  memoryMB: number;
}

interface TemperatureProxyHTTPStatus {
  node: string;
  url?: string;
  reachable: boolean;
  error?: string;
}

interface TemperatureProxyControlPlaneState {
  instance: string;
  lastSync?: string;
  refreshIntervalSeconds?: number;
  secondsBehind?: number;
  status?: string;
}

interface TemperatureProxySocketHost {
  node?: string;
  host?: string;
  cooldownUntil?: string;
  secondsRemaining?: number;
  lastError?: string;
}

type TemperatureSocketCooldownInfo = {
  secondsRemaining?: number;
  until?: string;
  lastError?: string;
};

interface HostProxySummary {
  requested?: boolean;
  installed?: boolean;
  hostSocketPresent?: boolean;
  containerSocketPresent?: boolean | null;
  lastUpdated?: string;
  ctid?: string;
}

interface HostProxyStatusResponse {
  hostSocketPresent?: boolean;
  containerSocketPresent?: boolean;
  summary?: HostProxySummary | null;
  reinstallCommand?: string;
  installerURL?: string;
  lastChecked?: string;
}

interface TemperatureProxyDiagnostic {
  legacySSHDetected: boolean;
  recommendProxyUpgrade: boolean;
  socketFound: boolean;
  socketPath?: string;
  socketPermissions?: string;
  socketOwner?: string;
  socketGroup?: string;
  proxyReachable?: boolean;
  proxyVersion?: string;
  proxyPublicKeySha256?: string;
  proxySshDirectory?: string;
  legacySshKeyCount?: number;
  proxyCapabilities?: string[];
  notes?: string[];
  httpProxies?: TemperatureProxyHTTPStatus[];
  controlPlaneEnabled?: boolean;
  controlPlaneStates?: TemperatureProxyControlPlaneState[];
  socketHostCooldowns?: TemperatureProxySocketHost[];
}

interface APITokenSummary {
  id: string;
  name: string;
  hint?: string;
  createdAt?: string;
  lastUsedAt?: string;
  source?: string;
}

interface APITokenUsage {
  tokenId: string;
  hostCount: number;
  hosts?: string[];
}

interface APITokenDiagnostic {
  enabled: boolean;
  tokenCount: number;
  hasEnvTokens: boolean;
  hasLegacyToken: boolean;
  recommendTokenSetup: boolean;
  recommendTokenRotation: boolean;
  legacyDockerHostCount?: number;
  unusedTokenCount?: number;
  notes?: string[];
  tokens?: APITokenSummary[];
  usage?: APITokenUsage[];
}

interface DockerAgentAttention {
  hostId: string;
  name: string;
  status: string;
  agentVersion?: string;
  tokenHint?: string;
  lastSeen?: string;
  issues: string[];
}

interface DockerAgentDiagnostic {
  hostsTotal: number;
  hostsOnline: number;
  hostsReportingVersion: number;
  hostsWithTokenBinding: number;
  hostsWithoutTokenBinding: number;
  hostsWithoutVersion?: number;
  hostsOutdatedVersion?: number;
  hostsWithStaleCommand?: number;
  hostsPendingUninstall?: number;
  hostsNeedingAttention: number;
  recommendedAgentVersion?: string;
  attention?: DockerAgentAttention[];
  notes?: string[];
}

interface DiscoveryDiagnostic {
  enabled: boolean;
  configuredSubnet?: string;
  activeSubnet?: string;
  environmentOverride?: string;
  subnetAllowlist?: string[];
  subnetBlocklist?: string[];
  scanning?: boolean;
  scanInterval?: string;
  lastScanStartedAt?: string;
  lastResultTimestamp?: string;
  lastResultServers?: number;
  lastResultErrors?: number;
}

interface AlertsDiagnostic {
  legacyThresholdsDetected: boolean;
  legacyThresholdSources?: string[];
  legacyScheduleSettings?: string[];
  missingCooldown: boolean;
  missingGroupingWindow: boolean;
  notes?: string[];
}

interface ProxyRegisterNode {
  name: string;
  sshReady: boolean;
  error?: string;
}

interface DockerMigrationResult {
  token: string;
  installCommand: string;
  systemdServiceSnippet: string;
  pulseURL: string;
  record: APITokenRecord;
}

interface DiagnosticsData {
  version: string;
  runtime: string;
  uptime: number;
  nodes: DiagnosticsNode[];
  pbs: DiagnosticsPBS[];
  system: SystemDiagnostic;
  temperatureProxy?: TemperatureProxyDiagnostic | null;
  apiTokens?: APITokenDiagnostic | null;
  dockerAgents?: DockerAgentDiagnostic | null;
  alerts?: AlertsDiagnostic | null;
  discovery?: DiscoveryDiagnostic | null;
  errors: string[];
}

interface DiscoveryScanStatus {
  scanning: boolean;
  subnet?: string;
  lastScanStartedAt?: number;
  lastResultAt?: number;
  errors?: string[];
}

type SettingsTab =
  | 'proxmox'
  | 'docker'
  | 'hosts'
  | 'agents'
  | 'system-general'
  | 'system-network'
  | 'system-updates'
  | 'system-backups'
  | 'system-ai'
  | 'api'
  | 'security-overview'
  | 'security-auth'
  | 'security-sso'
  | 'diagnostics'
  | 'updates';

type AgentKey = 'pve' | 'pbs' | 'pmg';

const SETTINGS_HEADER_META: Record<SettingsTab, { title: string; description: string }> = {
  proxmox: {
    title: 'Proxmox',
    description:
      'Monitor your Proxmox Virtual Environment, Backup Server, and Mail Gateway infrastructure.',
  },
  docker: {
    title: 'Docker',
    description:
      'Monitor Docker hosts, containers, images, and volumes across your infrastructure.',
  },
  hosts: {
    title: 'Hosts',
    description: 'Monitor Linux, macOS, and Windows machines—servers, desktops, and laptops.',
  },
  agents: {
    title: 'Agents',
    description: 'Install and manage the unified Pulse agent for host and Docker monitoring.',
  },
  'system-general': {
    title: 'General Settings',
    description: 'Configure appearance preferences and UI behavior.',
  },
  'system-network': {
    title: 'Network Settings',
    description: 'Manage CORS, embedding, and network discovery configuration.',
  },
  'system-updates': {
    title: 'Updates',
    description:
      'Check for updates, configure update channels, and manage automatic update checks.',
  },
  'system-backups': {
    title: 'Backup Polling',
    description: 'Control how often Pulse queries Proxmox for backup tasks and snapshots.',
  },
  'system-ai': {
    title: 'AI Assistant',
    description: 'Configure AI-powered infrastructure analysis and remediation suggestions.',
  },
  api: {
    title: 'API access',
    description:
      'Generate scoped tokens and manage automation credentials for agents and integrations.',
  },
  'security-overview': {
    title: 'Security Overview',
    description: 'View your security posture at a glance and monitor authentication status.',
  },
  'security-auth': {
    title: 'Authentication',
    description: 'Manage password-based authentication and credential rotation.',
  },
  'security-sso': {
    title: 'Single Sign-On',
    description: 'Configure OIDC providers for enterprise authentication.',
  },
  diagnostics: {
    title: 'Diagnostics',
    description:
      'Inspect discovery scans, connection health, and runtime metrics for troubleshooting.',
  },
  updates: {
    title: 'Update History',
    description: 'Review past software updates, rollback events, and upgrade audit logs.',
  },
};

const BACKUP_INTERVAL_OPTIONS = [
  { label: 'Default (~90 seconds)', value: 0 },
  { label: '15 minutes', value: 15 * 60 },
  { label: '30 minutes', value: 30 * 60 },
  { label: '1 hour', value: 60 * 60 },
  { label: '6 hours', value: 6 * 60 * 60 },
  { label: '12 hours', value: 12 * 60 * 60 },
  { label: '24 hours', value: 24 * 60 * 60 },
];

const BACKUP_INTERVAL_MAX_MINUTES = 7 * 24 * 60; // 7 days

const PVE_POLLING_MIN_SECONDS = 10;
const PVE_POLLING_MAX_SECONDS = 3600;
const PVE_POLLING_PRESETS = [
  { label: '10 seconds (default)', value: 10 },
  { label: '15 seconds', value: 15 },
  { label: '30 seconds', value: 30 },
  { label: '60 seconds', value: 60 },
  { label: '2 minutes', value: 120 },
  { label: '5 minutes', value: 300 },
];

// Node with UI-specific fields
type NodeConfigWithStatus = NodeConfig & {
  hasPassword?: boolean;
  hasToken?: boolean;
  status: 'connected' | 'disconnected' | 'offline' | 'error' | 'pending';
};

interface SettingsProps {
  darkMode: () => boolean;
  toggleDarkMode: () => void;
}

const Settings: Component<SettingsProps> = (props) => {
  const { state, connected: _connected } = useWebSocket();
  const navigate = useNavigate();
  const location = useLocation();

  const deriveTabFromPath = (path: string): SettingsTab => {
    if (path.includes('/settings/proxmox')) return 'proxmox';
    if (path.includes('/settings/agent-hub')) return 'proxmox';
    if (path.includes('/settings/docker')) return 'agents';
    if (path.includes('/settings/containers')) return 'agents';
    if (
      path.includes('/settings/hosts') ||
      path.includes('/settings/host-agents') ||
      path.includes('/settings/servers') ||
      path.includes('/settings/linuxServers') ||
      path.includes('/settings/windowsServers') ||
      path.includes('/settings/macServers') ||
      path.includes('/settings/agents')
    )
      return 'agents';
    if (path.includes('/settings/system-general')) return 'system-general';
    if (path.includes('/settings/system-network')) return 'system-network';
    if (path.includes('/settings/system-updates')) return 'system-updates';
    if (path.includes('/settings/system-backups')) return 'system-backups';
    if (path.includes('/settings/system-ai')) return 'system-ai';
    if (path.includes('/settings/system')) return 'system-general';
    if (path.includes('/settings/api')) return 'api';
    if (path.includes('/settings/security-overview')) return 'security-overview';
    if (path.includes('/settings/security-auth')) return 'security-auth';
    if (path.includes('/settings/security-sso')) return 'security-sso';
    if (path.includes('/settings/security')) return 'security-overview';
    if (path.includes('/settings/diagnostics')) return 'diagnostics';
    if (path.includes('/settings/updates')) return 'updates';
    // Legacy platform paths map to the Proxmox tab
    if (
      path.includes('/settings/pve') ||
      path.includes('/settings/pbs') ||
      path.includes('/settings/pmg') ||
      path.includes('/settings/docker') ||
      path.includes('/settings/linuxServers') ||
      path.includes('/settings/windowsServers') ||
      path.includes('/settings/macServers')
    ) {
      return 'proxmox';
    }
    return 'proxmox';
  };

  const deriveAgentFromPath = (path: string): AgentKey | null => {
    if (path.includes('/settings/pve')) return 'pve';
    if (path.includes('/settings/pbs')) return 'pbs';
    if (path.includes('/settings/pmg')) return 'pmg';
    return null;
  };

  const [currentTab, setCurrentTab] = createSignal<SettingsTab>(
    deriveTabFromPath(location.pathname),
  );
  const activeTab = () => currentTab();

  const [selectedAgent, setSelectedAgent] = createSignal<AgentKey>('pve');

  const agentPaths: Record<AgentKey, string> = {
    pve: '/settings/pve',
    pbs: '/settings/pbs',
    pmg: '/settings/pmg',
  };

  const handleSelectAgent = (agent: AgentKey) => {
    setSelectedAgent(agent);
    if (currentTab() !== 'proxmox') {
      setCurrentTab('proxmox');
    }
    const target = agentPaths[agent];
    if (target && location.pathname !== target) {
      navigate(target, { scroll: false });
    }
  };

  const setActiveTab = (tab: SettingsTab) => {
    if (tab === 'proxmox' && deriveAgentFromPath(location.pathname) === null) {
      setSelectedAgent('pve');
    }
    const targetPath = `/settings/${tab}`;
    if (location.pathname !== targetPath) {
      navigate(targetPath, { scroll: false });
      return;
    }
    if (currentTab() !== tab) {
      setCurrentTab(tab);
    }
  };

  const headerMeta = () =>
    SETTINGS_HEADER_META[activeTab()] ?? {
      title: 'Settings',
      description: 'Manage Pulse configuration.',
    };

  const _pveBackupsState = () => state.backups?.pve ?? state.pveBackups;
  const _pbsBackupsState = () => state.backups?.pbs ?? state.pbsBackups;

  // Keep tab state in sync with URL and handle /settings redirect without flicker
  createEffect(
    on(
      () => location.pathname,
      (path) => {
        if (path === '/settings' || path === '/settings/') {
          if (currentTab() !== 'proxmox') {
            setCurrentTab('proxmox');
          }
          setSelectedAgent('pve');
          return;
        }

        if (path.startsWith('/settings/agent-hub')) {
          navigate(path.replace('/settings/agent-hub', '/settings/proxmox'), {
            replace: true,
            scroll: false,
          });
          return;
        }

        if (path.startsWith('/settings/servers')) {
          navigate(path.replace('/settings/servers', '/settings/hosts'), {
            replace: true,
            scroll: false,
          });
          return;
        }

        if (path.startsWith('/settings/containers')) {
          navigate(path.replace('/settings/containers', '/settings/docker'), {
            replace: true,
            scroll: false,
          });
          return;
        }

        if (
          path.startsWith('/settings/linuxServers') ||
          path.startsWith('/settings/windowsServers') ||
          path.startsWith('/settings/macServers')
        ) {
          navigate('/settings/hosts', {
            replace: true,
            scroll: false,
          });
          return;
        }

        const resolved = deriveTabFromPath(path);
        if (resolved !== currentTab()) {
          setCurrentTab(resolved);
        }

        if (resolved === 'proxmox') {
          const agentFromPath = deriveAgentFromPath(path);
          setSelectedAgent(agentFromPath ?? 'pve');
        }
      },
    ),
  );

  const [hasUnsavedChanges, setHasUnsavedChanges] = createSignal(false);
  // Sidebar always starts expanded for discoverability (issue #764)
  // Users can collapse during session but it resets on page reload
  const [sidebarCollapsed, setSidebarCollapsed] = createSignal(false);
  const [nodes, setNodes] = createSignal<NodeConfigWithStatus[]>([]);
  const [discoveredNodes, setDiscoveredNodes] = createSignal<DiscoveredServer[]>([]);
  const [showNodeModal, setShowNodeModal] = createSignal(false);
  const [editingNode, setEditingNode] = createSignal<NodeConfigWithStatus | null>(null);
  const [currentNodeType, setCurrentNodeType] = createSignal<'pve' | 'pbs' | 'pmg'>('pve');
  const [modalResetKey, setModalResetKey] = createSignal(0);
  const [showPasswordModal, setShowPasswordModal] = createSignal(false);
  const [showDeleteNodeModal, setShowDeleteNodeModal] = createSignal(false);
  const [nodePendingDelete, setNodePendingDelete] = createSignal<NodeConfigWithStatus | null>(null);
  const [deleteNodeLoading, setDeleteNodeLoading] = createSignal(false);
  const isNodeModalVisible = (type: 'pve' | 'pbs' | 'pmg') =>
    Boolean(showNodeModal() && currentNodeType() === type);
  const resolveTemperatureMonitoringEnabled = (node?: NodeConfigWithStatus | null) => {
    if (node && typeof node.temperatureMonitoringEnabled === 'boolean') {
      return node.temperatureMonitoringEnabled;
    }
    return temperatureMonitoringEnabled();
  };
  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);
  const [discoveryScanStatus, setDiscoveryScanStatus] = createSignal<DiscoveryScanStatus>({
    scanning: false,
  });

  const pveNodes = createMemo(() => nodes().filter((n) => n.type === 'pve'));
  const pbsNodes = createMemo(() => nodes().filter((n) => n.type === 'pbs'));
  const pmgNodes = createMemo(() => nodes().filter((n) => n.type === 'pmg'));

  // System settings
  const [pvePollingInterval, setPVEPollingInterval] = createSignal<number>(PVE_POLLING_MIN_SECONDS);
  const [pvePollingSelection, setPVEPollingSelection] = createSignal<number | 'custom'>(
    PVE_POLLING_MIN_SECONDS,
  );
  const [pvePollingCustomSeconds, setPVEPollingCustomSeconds] = createSignal(30);
  const [allowedOrigins, setAllowedOrigins] = createSignal('*');
  const [discoveryEnabled, setDiscoveryEnabled] = createSignal(false);
  const [discoverySubnet, setDiscoverySubnet] = createSignal('auto');
  const [discoveryMode, setDiscoveryMode] = createSignal<'auto' | 'custom'>('auto');
  const [discoverySubnetDraft, setDiscoverySubnetDraft] = createSignal('');
  const [lastCustomSubnet, setLastCustomSubnet] = createSignal('');
  const [discoverySubnetError, setDiscoverySubnetError] = createSignal<string | undefined>();
  const [savingDiscoverySettings, setSavingDiscoverySettings] = createSignal(false);
  const [envOverrides, setEnvOverrides] = createSignal<Record<string, boolean>>({});
  const [temperatureMonitoringEnabled, setTemperatureMonitoringEnabled] = createSignal(true);
  const [savingTemperatureSetting, setSavingTemperatureSetting] = createSignal(false);
  const [_hostProxyStatus, setHostProxyStatus] = createSignal<HostProxyStatusResponse | null>(null);
  const [hideLocalLogin, setHideLocalLogin] = createSignal(false);
  const [savingHideLocalLogin, setSavingHideLocalLogin] = createSignal(false);

  const temperatureMonitoringLocked = () =>
    Boolean(
      envOverrides().temperatureMonitoringEnabled || envOverrides()['ENABLE_TEMPERATURE_MONITORING'],
    );
  const hideLocalLoginLocked = () =>
    Boolean(envOverrides().hideLocalLogin || envOverrides()['PULSE_AUTH_HIDE_LOCAL_LOGIN']);

  const pvePollingEnvLocked = () =>
    Boolean(envOverrides().pvePollingInterval || envOverrides().PVE_POLLING_INTERVAL);
  let discoverySubnetInputRef: HTMLInputElement | undefined;

  const parseSubnetList = (value: string) => {
    const seen = new Set<string>();
    return value
      .split(',')
      .map((token) => token.trim())
      .filter((token) => {
        if (!token || token.toLowerCase() === 'auto' || seen.has(token)) {
          return false;
        }
        seen.add(token);
        return true;
      });
  };

  const normalizeSubnetList = (value: string) => parseSubnetList(value).join(', ');

  const currentDraftSubnetValue = () => {
    if (discoveryMode() === 'custom') {
      return discoverySubnetDraft();
    }
    const draft = discoverySubnetDraft();
    if (draft.trim() !== '') {
      return draft;
    }
    const saved = discoverySubnet();
    return saved.toLowerCase() === 'auto' ? '' : saved;
  };

  const isValidCIDR = (value: string) => {
    const subnets = parseSubnetList(value);
    if (subnets.length === 0) {
      return false;
    }

    return subnets.every((token) => {
      const [network, prefix] = token.split('/');
      if (!network || typeof prefix === 'undefined') {
        return false;
      }

      const prefixNumber = Number(prefix);
      if (!Number.isInteger(prefixNumber) || prefixNumber < 0 || prefixNumber > 32) {
        return false;
      }

      const octets = network.split('.');
      if (octets.length !== 4) {
        return false;
      }

      return octets.every((octet) => {
        if (octet === '') return false;
        if (!/^\d+$/.test(octet)) return false;
        const valueNumber = Number(octet);
        return valueNumber >= 0 && valueNumber <= 255;
      });
    });
  };

  const handleHideLocalLoginChange = async (enabled: boolean): Promise<void> => {
    if (hideLocalLoginLocked() || savingHideLocalLogin()) {
      return;
    }

    const previous = hideLocalLogin();
    setHideLocalLogin(enabled);
    setSavingHideLocalLogin(true);

    try {
      await SettingsAPI.updateSystemSettings({ hideLocalLogin: enabled });
      if (enabled) {
        notificationStore.success('Local login hidden', 2000);
      } else {
        notificationStore.info('Local login visible', 2000);
      }
      // Reload security status to reflect changes
      await loadSecurityStatus();
    } catch (error) {
      logger.error('Failed to update hide local login setting', error);
      notificationStore.error(
        error instanceof Error ? error.message : 'Failed to update hide local login setting',
      );
      setHideLocalLogin(previous);
    } finally {
      setSavingHideLocalLogin(false);
    }
  };

  const applySavedDiscoverySubnet = (subnet?: string | null) => {
    const raw = typeof subnet === 'string' ? subnet.trim() : '';
    if (raw === '' || raw.toLowerCase() === 'auto') {
      setDiscoverySubnet('auto');
      setDiscoveryMode('auto');
      setDiscoverySubnetDraft('');
    } else {
      setDiscoveryMode('custom');
      const normalizedValue = normalizeSubnetList(raw);
      setDiscoverySubnet(normalizedValue);
      setDiscoverySubnetDraft(normalizedValue);
      setLastCustomSubnet(normalizedValue);
      setDiscoverySubnetError(undefined);
      return;
    }
    setDiscoverySubnetError(undefined);
  };
  // Connection timeout removed - backend-only setting

  // Iframe embedding settings
  const [allowEmbedding, setAllowEmbedding] = createSignal(false);
  const [allowedEmbedOrigins, setAllowedEmbedOrigins] = createSignal('');

  // Webhook security settings
  const [webhookAllowedPrivateCIDRs, setWebhookAllowedPrivateCIDRs] = createSignal('');

  // Update settings
  const [versionInfo, setVersionInfo] = createSignal<VersionInfo | null>(null);
  const [updateInfo, setUpdateInfo] = createSignal<UpdateInfo | null>(null);
  const [checkingForUpdates, setCheckingForUpdates] = createSignal(false);
  const [updateChannel, setUpdateChannel] = createSignal<'stable' | 'rc'>('stable');
  const [autoUpdateEnabled, setAutoUpdateEnabled] = createSignal(false);
  const [autoUpdateCheckInterval, setAutoUpdateCheckInterval] = createSignal(24);
  const [autoUpdateTime, setAutoUpdateTime] = createSignal('03:00');
  const [backupPollingEnabled, setBackupPollingEnabled] = createSignal(true);
  const [backupPollingInterval, setBackupPollingInterval] = createSignal(0);
  const [backupPollingCustomMinutes, setBackupPollingCustomMinutes] = createSignal(60);
  const [backupPollingUseCustom, setBackupPollingUseCustom] = createSignal(false);
  const backupPollingEnvLocked = () =>
    Boolean(envOverrides()['ENABLE_BACKUP_POLLING'] || envOverrides()['BACKUP_POLLING_INTERVAL']);
  const backupIntervalSelectValue = () => {
    if (backupPollingUseCustom()) {
      return 'custom';
    }
    const seconds = backupPollingInterval();
    return BACKUP_INTERVAL_OPTIONS.some((option) => option.value === seconds)
      ? String(seconds)
      : 'custom';
  };
  const backupIntervalSummary = () => {
    if (!backupPollingEnabled()) {
      return 'Backup polling is disabled.';
    }
    const seconds = backupPollingInterval();
    if (seconds <= 0) {
      return 'Pulse checks backups and snapshots at the default cadence (~every 90 seconds).';
    }
    if (seconds % 86400 === 0) {
      const days = seconds / 86400;
      return `Pulse checks backups every ${days === 1 ? 'day' : `${days} days`}.`;
    }
    if (seconds % 3600 === 0) {
      const hours = seconds / 3600;
      return `Pulse checks backups every ${hours === 1 ? 'hour' : `${hours} hours`}.`;
    }
    const minutes = Math.max(1, Math.round(seconds / 60));
    return `Pulse checks backups every ${minutes === 1 ? 'minute' : `${minutes} minutes`}.`;
  };

  // Diagnostics
  const [diagnosticsData, setDiagnosticsData] = createSignal<DiagnosticsData | null>(null);
  const [_runningDiagnostics, setRunningDiagnostics] = createSignal(false);
  const [proxyActionLoading, setProxyActionLoading] = createSignal<'register-nodes' | null>(null);
  const [_proxyRegisterSummary, setProxyRegisterSummary] = createSignal<ProxyRegisterNode[] | null>(
    null,
  );
  const [dockerActionLoading, setDockerActionLoading] = createSignal<string | null>(null);
  const [_dockerMigrationResults, setDockerMigrationResults] = createSignal<
    Record<string, DockerMigrationResult>
  >({});

  // Security
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatusInfo | null>(null);
  const [securityStatusLoading, setSecurityStatusLoading] = createSignal(true);
  const [exportPassphrase, setExportPassphrase] = createSignal('');
  const [useCustomPassphrase, setUseCustomPassphrase] = createSignal(false);
  const [importPassphrase, setImportPassphrase] = createSignal('');
  const [importFile, setImportFile] = createSignal<File | null>(null);
  const [showExportDialog, setShowExportDialog] = createSignal(false);
  const [showImportDialog, setShowImportDialog] = createSignal(false);
  const [showApiTokenModal, setShowApiTokenModal] = createSignal(false);
  const [apiTokenInput, setApiTokenInput] = createSignal('');
  const [apiTokenModalSource, setApiTokenModalSource] = createSignal<'export' | 'import' | null>(
    null,
  );
  const [showQuickSecuritySetup, setShowQuickSecuritySetup] = createSignal(false);
  const authDisabledByEnv = createMemo(() => Boolean(securityStatus()?.deprecatedDisableAuth));
  const [showQuickSecurityWizard, setShowQuickSecurityWizard] = createSignal(false);

  const formatRelativeTime = (timestamp?: number) => {
    if (!timestamp) {
      return '';
    }

    const delta = Date.now() - timestamp;
    if (delta < 0) {
      return 'just now';
    }

    const seconds = Math.round(delta / 1000);
    if (seconds < 60) {
      return `${seconds}s ago`;
    }

    const minutes = Math.round(seconds / 60);
    if (minutes < 60) {
      return `${minutes}m ago`;
    }

    const hours = Math.round(minutes / 60);
    if (hours < 24) {
      return `${hours}h ago`;
    }

    return new Date(timestamp).toLocaleString();
  };

  const formatIsoDateTime = (iso?: string) => {
    if (!iso) {
      return '';
    }
    const timestamp = Date.parse(iso);
    if (Number.isNaN(timestamp)) {
      return '';
    }
    return new Date(timestamp).toLocaleString();
  };

  const formatIsoRelativeTime = (iso?: string) => {
    if (!iso) {
      return '';
    }
    const timestamp = Date.parse(iso);
    if (Number.isNaN(timestamp)) {
      return '';
    }
    return formatRelativeTime(timestamp);
  };

  const controlPlaneStatusLabel = (status?: string) => {
    switch (status) {
      case 'healthy':
        return 'Healthy';
      case 'stale':
        return 'Behind';
      case 'offline':
        return 'Offline';
      case 'pending':
      default:
        return 'Pending';
    }
  };

  const controlPlaneStatusClass = (status?: string) => {
    switch (status) {
      case 'healthy':
        return 'bg-green-500';
      case 'stale':
        return 'bg-yellow-500';
      case 'offline':
        return 'bg-red-500';
      default:
        return 'bg-gray-500';
    }
  };

  const formatUptime = (seconds: number) => {
    if (!seconds || seconds <= 0) {
      return 'Unknown';
    }

    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);

    if (days > 0) {
      return `${days}d ${hours}h`;
    }
    if (hours > 0) {
      return `${hours}h ${minutes}m`;
    }
    if (minutes > 0) {
      return `${minutes}m`;
    }
    return `${Math.floor(seconds)}s`;
  };

  const normalizeHostKey = (value?: string | null) => {
    if (!value) {
      return '';
    }
    let result = value.trim().toLowerCase();
    if (!result) {
      return '';
    }
    result = result.replace(/^https?:\/\//, '');
    const slashIndex = result.indexOf('/');
    if (slashIndex !== -1) {
      result = result.slice(0, slashIndex);
    }
    const colonIndex = result.indexOf(':');
    if (colonIndex !== -1) {
      result = result.slice(0, colonIndex);
    }
    return result;
  };

  const emitTemperatureProxyWarnings = (diag: DiagnosticsData | null) => {
    if (!diag?.temperatureProxy) {
      return;
    }
    if (diag.temperatureProxy.httpProxies) {
      const failing = (diag.temperatureProxy.httpProxies as TemperatureProxyHTTPStatus[]).filter(
        (proxy) => proxy && proxy.node && !proxy.reachable,
      );
      if (failing.length > 0) {
        const nodes = failing.map((proxy) => proxy.node || 'Unknown').join(', ');
        showWarning(`Pulse cannot reach HTTPS temperature proxy on: ${nodes}`);
      }
    }
    if (diag.temperatureProxy.controlPlaneStates) {
      const stale = (diag.temperatureProxy.controlPlaneStates as TemperatureProxyControlPlaneState[]).filter(
        (state) => state && (state.status === 'stale' || state.status === 'offline'),
      );
      if (stale.length > 0) {
        const names = stale.map((state) => state.instance || 'Proxy').join(', ');
        showWarning(`Temperature proxy control plane is behind on: ${names}`);
      }
    }
    if (diag.temperatureProxy.socketHostCooldowns) {
      const cooling = (diag.temperatureProxy.socketHostCooldowns as TemperatureProxySocketHost[]).filter(
        (entry) => entry && (entry.node || entry.host),
      );
      if (cooling.length > 0) {
        const hosts = cooling.map((entry) => entry.node || entry.host || 'proxy').join(', ');
        showWarning(`Temperature proxy is cooling down the following hosts: ${hosts}`);
      }
    }
  };

  const temperatureTransportInfo = createMemo<TemperatureTransportInfo | null>(() => {
    const diag = diagnosticsData();
    if (!diag?.temperatureProxy) {
      return null;
    }
    const httpMap: TemperatureTransportInfo['httpMap'] = {};
    const proxies = diag.temperatureProxy.httpProxies || [];
    proxies.forEach((proxy) => {
      if (!proxy || !proxy.node) {
        return;
      }
      const key = proxy.node.trim().toLowerCase();
      if (!key) {
        return;
      }
      httpMap[key] = {
        reachable: Boolean(proxy.reachable),
        error: proxy.error || undefined,
        url: proxy.url || undefined,
      };
    });
    const socketStatus: TemperatureTransportInfo['socketStatus'] =
      diag.temperatureProxy.socketFound && diag.temperatureProxy.proxyReachable
        ? 'healthy'
        : diag.temperatureProxy.socketFound
          ? 'error'
          : 'missing';
    const cooldowns: Record<string, TemperatureSocketCooldownInfo> = {};
    const socketHosts = diag.temperatureProxy.socketHostCooldowns || [];
    (socketHosts as TemperatureProxySocketHost[]).forEach((entry) => {
      const key = normalizeHostKey(entry.node) || normalizeHostKey(entry.host);
      if (!key) {
        return;
      }
      cooldowns[key] = {
        secondsRemaining: entry.secondsRemaining,
        until: entry.cooldownUntil,
        lastError: entry.lastError || undefined,
      };
    });
    return { httpMap, socketStatus, socketCooldowns: cooldowns };
  });

  const proxyNodeChecksSupported = createMemo(() => {
    const caps = diagnosticsData()?.temperatureProxy?.proxyCapabilities;
    if (!caps || caps.length === 0) {
      return true;
    }
    return caps.some(
      (cap) => typeof cap === 'string' && cap.trim().toLowerCase() === 'admin',
    );
  });

  const runDiagnostics = async () => {
    setRunningDiagnostics(true);
    try {
      const response = await apiFetch('/api/diagnostics');
      const diag = await response.json();
      setDiagnosticsData(diag);
      emitTemperatureProxyWarnings(diag);
      setHostProxyStatus(
        diag?.temperatureProxy?.hostProxySummary
          ? {
            hostSocketPresent: Boolean(diag.temperatureProxy?.socketFound),
            containerSocketPresent:
              diag.temperatureProxy?.hostProxySummary?.containerSocketPresent ?? undefined,
            summary: diag.temperatureProxy?.hostProxySummary ?? undefined,
          }
          : null,
      );
    } catch (err) {
      logger.error('Failed to fetch diagnostics', err);
      showError('Failed to run diagnostics');
    } finally {
      setRunningDiagnostics(false);
    }
  };

  const refreshHostProxyStatus = async (notify = false) => {
    try {
      const status = (await apiFetchJSON(
        '/api/temperature-proxy/host-status',
      )) as HostProxyStatusResponse;
      setHostProxyStatus(status);
      if (notify) {
        showSuccess('Host proxy status refreshed', undefined, 2000);
      }
    } catch (err) {
      logger.error('Failed to refresh host proxy status', err);
      showError('Failed to refresh host proxy status');
    }
  };

  createEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    const shouldPoll = currentTab() === 'proxmox';
    if (!shouldPoll) {
      return;
    }
    void runDiagnostics();
    void refreshHostProxyStatus(false);
    const intervalId = window.setInterval(() => {
      void runDiagnostics();
      void refreshHostProxyStatus(false);
    }, 60000);
    onCleanup(() => {
      window.clearInterval(intervalId);
    });
  });

  const handleRegisterProxyNodes = async () => {
    if (proxyActionLoading()) return;
    setProxyActionLoading('register-nodes');
    try {
      const response = await apiFetch('/api/diagnostics/temperature-proxy/register-nodes', {
        method: 'POST',
      });
      const data = await response.json().catch(() => null);
      if (!response.ok || !data || data.success !== true) {
        const message =
          (data && typeof data.error === 'string' && data.error) ||
          (data && typeof data.message === 'string' && data.message) ||
          'Failed to query proxy nodes';
        if (response.status === 403) {
          showWarning(message);
        } else {
          showError(message);
        }
        return;
      }

      const nodes = Array.isArray(data.nodes)
        ? (data.nodes as Array<Record<string, unknown>>).map((node) => ({
          name: typeof node.name === 'string' ? node.name : 'unknown',
          sshReady: Boolean(node.ssh_ready),
          error: typeof node.error === 'string' ? node.error : undefined,
        }))
        : [];

      setProxyRegisterSummary(nodes);
      showSuccess('Queried proxy node registration state');
      await runDiagnostics();
    } catch (err) {
      logger.error('Failed to query proxy node registration state', err);
      showError('Failed to query proxy nodes');
    } finally {
      setProxyActionLoading(null);
    }
  };

  const handleDockerPrepareToken = async (hostId: string) => {
    if (dockerActionLoading()) return;
    setDockerActionLoading(hostId);
    try {
      const response = await apiFetch('/api/diagnostics/docker/prepare-token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ hostId }),
      });
      const data = await response.json().catch(() => null);
      if (!response.ok || !data || data.success !== true) {
        const message =
          data && typeof data.message === 'string'
            ? data.message
            : 'Failed to prepare Docker token';
        showError(message);
        return;
      }

      const recordPayload = data.record as APITokenRecord | undefined;
      if (!recordPayload || typeof recordPayload.id !== 'string') {
        showError('Server did not return a valid token record');
        return;
      }
      const tokenRecord: APITokenRecord = {
        id: recordPayload.id,
        name: recordPayload.name,
        prefix: recordPayload.prefix,
        suffix: recordPayload.suffix,
        createdAt: recordPayload.createdAt,
        lastUsedAt: recordPayload.lastUsedAt,
      };

      const migrationResult: DockerMigrationResult = {
        token: data.token as string,
        installCommand: data.installCommand as string,
        systemdServiceSnippet: data.systemdServiceSnippet as string,
        pulseURL: data.pulseURL as string,
        record: tokenRecord,
      };

      setDockerMigrationResults((prev) => ({
        ...prev,
        [hostId]: migrationResult,
      }));

      showTokenReveal({
        token: migrationResult.token,
        record: tokenRecord,
        source: 'docker',
        note: 'Copy this token into the install command shown in Diagnostics.',
      });

      const hostName = data.host && typeof data.host.name === 'string' ? data.host.name : hostId;
      showSuccess(`Generated dedicated token for ${hostName}`);
      await runDiagnostics();
    } catch (err) {
      logger.error('Failed to prepare Docker token', err);
      showError('Failed to prepare Docker token');
    } finally {
      setDockerActionLoading(null);
    }
  };

  const handleCopy = async (text: string, successMessage: string) => {
    const success = await copyToClipboard(text);
    if (success) {
      showSuccess(successMessage);
    } else {
      showError('Failed to copy to clipboard');
    }
  };

  const tabGroups: {
    id: 'platforms' | 'operations' | 'system' | 'security';
    label: string;
    items: {
      id: SettingsTab;
      label: string;
      icon: Component<{ class?: string; strokeWidth?: number }>;
      iconProps?: { strokeWidth?: number };
      disabled?: boolean;
    }[];
  }[] = [
      {
        id: 'platforms',
        label: 'Platforms',
        items: [
          { id: 'proxmox', label: 'Proxmox', icon: ProxmoxIcon },
          { id: 'agents', label: 'Agents', icon: Monitor, iconProps: { strokeWidth: 2 } },
        ],
      },
      {
        id: 'operations',
        label: 'Operations',
        items: [
          { id: 'api', label: 'API Tokens', icon: BadgeCheck },
          {
            id: 'diagnostics',
            label: 'Diagnostics',
            icon: Activity,
            iconProps: { strokeWidth: 2 },
          },
        ],
      },
      {
        id: 'system',
        label: 'System',
        items: [
          {
            id: 'system-general',
            label: 'General',
            icon: Sliders,
            iconProps: { strokeWidth: 2 },
          },
          {
            id: 'system-network',
            label: 'Network',
            icon: Network,
            iconProps: { strokeWidth: 2 },
          },
          {
            id: 'system-updates',
            label: 'Updates',
            icon: RefreshCw,
            iconProps: { strokeWidth: 2 },
          },
          {
            id: 'system-backups',
            label: 'Backups',
            icon: Clock,
            iconProps: { strokeWidth: 2 },
          },
          {
            id: 'system-ai',
            label: 'AI Assistant',
            icon: Sparkles,
            iconProps: { strokeWidth: 2 },
          },
        ],
      },
      {
        id: 'security',
        label: 'Security',
        items: [
          {
            id: 'security-overview',
            label: 'Overview',
            icon: Shield,
            iconProps: { strokeWidth: 2 },
          },
          {
            id: 'security-auth',
            label: 'Authentication',
            icon: Lock,
            iconProps: { strokeWidth: 2 },
          },
          {
            id: 'security-sso',
            label: 'Single Sign-On',
            icon: Key,
            iconProps: { strokeWidth: 2 },
          },
        ],
      },
    ];

  const flatTabs = tabGroups.flatMap((group) => group.items);

  // Function to load nodes
  const loadNodes = async () => {
    try {
      const nodesList = await NodesAPI.getNodes();
      // Merge temperature data from WebSocket state (if available)
      // state is a store object, not a function
      const stateNodes = state.nodes;
      const nodesWithStatus = nodesList.map((node) => {
        // Find matching node in state to get temperature data
        // State uses a unified 'nodes' array for all node types
        // Match nodes by ID or by name (handling .lan suffix variations)
        const stateNode = stateNodes?.find((n) => {
          // Try exact ID match first
          if (n.id === node.id) return true;
          // Try exact name match
          if (n.name === node.name) return true;
          // Try name with/without .lan suffix
          const nodeNameBase = node.name.replace(/\.lan$/, '');
          const stateNameBase = n.name.replace(/\.lan$/, '');
          if (nodeNameBase === stateNameBase) return true;
          // Also check if state node ID contains the config node name
          if (n.id.includes(node.name) || node.name.includes(n.name)) return true;
          return false;
        });

        const mergedNode = {
          ...node,
          // Use the hasPassword/hasToken from the API if available, otherwise check local fields
          hasPassword: node.hasPassword ?? !!node.password,
          hasToken: node.hasToken ?? !!node.tokenValue,
          status: node.status || ('pending' as const),
          // Merge temperature data from state
          temperature: stateNode?.temperature || node.temperature,
        };

        return mergedNode;
      });
      setNodes(nodesWithStatus);
    } catch (error) {
      logger.error('Failed to load nodes', error);
      // If we get a 429 or network error, retry after a delay
      if (
        error instanceof Error &&
        (error.message.includes('429') || error.message.includes('fetch'))
      ) {
        logger.info('Retrying node load after delay');
        setTimeout(() => loadNodes(), 3000);
      }
    }
  };

  // Function to load discovered nodes
  const loadSecurityStatus = async () => {
    setSecurityStatusLoading(true);
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/security/status');
      if (response.ok) {
        const status = await response.json();
        logger.debug('Security status loaded', status);
        setSecurityStatus(status);
      } else {
        logger.error('Failed to fetch security status', { status: response.status });
      }
    } catch (err) {
      logger.error('Failed to fetch security status', err);
    } finally {
      setSecurityStatusLoading(false);
    }
  };

  createEffect(() => {
    if (authDisabledByEnv() && showQuickSecuritySetup()) {
      setShowQuickSecuritySetup(false);
    }
  });

  const updateDiscoveredNodesFromServers = (
    servers: RawDiscoveredServer[] | undefined | null,
    options: { merge?: boolean } = {},
  ) => {
    const { merge = false } = options;

    if (!servers || servers.length === 0) {
      if (!merge) {
        setDiscoveredNodes([]);
      }
      return;
    }

    // Prepare sets of configured hosts and cluster member IPs to filter duplicates
    const configuredHosts = new Set<string>();
    const clusterMemberIPs = new Set<string>();

    nodes().forEach((n) => {
      const cleanedHost = n.host.replace(/^https?:\/\//, '').replace(/:\d+$/, '');
      configuredHosts.add(cleanedHost.toLowerCase());

      if (
        n.type === 'pve' &&
        'isCluster' in n &&
        n.isCluster &&
        'clusterEndpoints' in n &&
        n.clusterEndpoints
      ) {
        n.clusterEndpoints.forEach((endpoint: ClusterEndpoint) => {
          if (endpoint.IP) {
            clusterMemberIPs.add(endpoint.IP.toLowerCase());
          }
          if (endpoint.Host) {
            clusterMemberIPs.add(endpoint.Host.toLowerCase());
          }
        });
      }
    });

    const recognizedTypes = ['pve', 'pbs', 'pmg'] as const;
    type RecognizedType = (typeof recognizedTypes)[number];
    const isRecognizedType = (value: string): value is RecognizedType =>
      (recognizedTypes as readonly string[]).includes(value);

    const normalized = servers
      .map((server): DiscoveredServer | null => {
        const ip = (server.ip || '').trim();
        let type = (server.type || '').toLowerCase();
        const hostname = (server.hostname || server.name || '').trim();
        const version = (server.version || '').trim();
        const release = (server.release || '').trim();

        if (!isRecognizedType(type)) {
          const metadata = `${hostname} ${version} ${release}`.toLowerCase();
          if (metadata.includes('pmg') || metadata.includes('mail gateway')) {
            type = 'pmg';
          } else if (metadata.includes('pbs') || metadata.includes('backup server')) {
            type = 'pbs';
          } else if (metadata.includes('pve') || metadata.includes('virtual environment')) {
            type = 'pve';
          }
        }

        if (!ip || !isRecognizedType(type)) {
          return null;
        }

        const port = typeof server.port === 'number' ? server.port : type === 'pbs' ? 8007 : 8006;

        return {
          ip,
          port,
          type,
          version: version || 'Unknown',
          hostname: hostname || undefined,
          release: release || undefined,
        };
      })
      .filter((server): server is DiscoveredServer => server !== null);

    const filtered = normalized.filter((server) => {
      const serverIP = server.ip.toLowerCase();
      const serverHostname = server.hostname?.toLowerCase();

      if (
        configuredHosts.has(serverIP) ||
        (serverHostname && configuredHosts.has(serverHostname))
      ) {
        return false;
      }

      if (
        clusterMemberIPs.has(serverIP) ||
        (serverHostname && clusterMemberIPs.has(serverHostname))
      ) {
        return false;
      }

      return true;
    });

    if (merge) {
      setDiscoveredNodes((prev) => {
        const existingMap = new Map(prev.map((item) => [`${item.ip}:${item.port}`, item]));
        filtered.forEach((server) => {
          existingMap.set(`${server.ip}:${server.port}`, server);
        });
        return Array.from(existingMap.values());
      });
    } else {
      setDiscoveredNodes(filtered);
    }

    setDiscoveryScanStatus((prev) => ({
      ...prev,
      lastResultAt: Date.now(),
    }));
  };

  const loadDiscoveredNodes = async () => {
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/discover');
      if (response.ok) {
        const data = await response.json();
        if (Array.isArray(data.servers)) {
          updateDiscoveredNodesFromServers(data.servers as RawDiscoveredServer[]);
          setDiscoveryScanStatus((prev) => ({
            ...prev,
            lastResultAt: typeof data.timestamp === 'number' ? data.timestamp : Date.now(),
            errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
          }));
        } else {
          updateDiscoveredNodesFromServers([]);
          setDiscoveryScanStatus((prev) => ({
            ...prev,
            lastResultAt: typeof data?.timestamp === 'number' ? data.timestamp : prev.lastResultAt,
            errors: Array.isArray(data?.errors) && data.errors.length > 0 ? data.errors : undefined,
          }));
        }
      }
    } catch (error) {
      logger.error('Failed to load discovered nodes', error);
    }
  };

  const triggerDiscoveryScan = async (options: { quiet?: boolean } = {}) => {
    const { quiet = false } = options;

    setDiscoveryScanStatus((prev) => ({
      ...prev,
      scanning: true,
      subnet: discoverySubnet() || prev.subnet,
      lastScanStartedAt: Date.now(),
      errors: undefined,
    }));

    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/discover', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ subnet: discoverySubnet() || 'auto' }),
      });

      if (!response.ok) {
        const message = await response.text();
        throw new Error(message || 'Discovery request failed');
      }

      if (!quiet) {
        notificationStore.info('Discovery scan started', 2000);
      }
    } catch (error) {
      logger.error('Failed to start discovery scan', error);
      notificationStore.error('Failed to start discovery scan');
      setDiscoveryScanStatus((prev) => ({
        ...prev,
        scanning: false,
      }));
    }
  };

  const handleDiscoveryEnabledChange = async (enabled: boolean): Promise<boolean> => {
    if (envOverrides().discoveryEnabled || savingDiscoverySettings()) {
      return false;
    }

    const previousEnabled = discoveryEnabled();
    const previousSubnet = discoverySubnet();
    let subnetToSend = discoverySubnet();

    if (enabled) {
      if (discoveryMode() === 'custom') {
        const trimmedDraft = discoverySubnetDraft().trim();
        if (!trimmedDraft) {
          setDiscoverySubnetError('Enter at least one subnet before enabling discovery');
          notificationStore.error('Enter at least one subnet before enabling discovery');
          return false;
        }
        if (!isValidCIDR(trimmedDraft)) {
          setDiscoverySubnetError(
            'Use CIDR format such as 192.168.1.0/24 (comma-separated for multiple)',
          );
          notificationStore.error('Enter valid CIDR subnet values before enabling discovery');
          return false;
        }
        const normalizedDraft = normalizeSubnetList(trimmedDraft);
        setDiscoverySubnetDraft(normalizedDraft);
        setDiscoverySubnetError(undefined);
        subnetToSend = normalizedDraft;
      } else {
        subnetToSend = 'auto';
        setDiscoverySubnetError(undefined);
      }
    }

    setDiscoveryEnabled(enabled);
    setSavingDiscoverySettings(true);

    try {
      await SettingsAPI.updateSystemSettings({
        discoveryEnabled: enabled,
        discoverySubnet: subnetToSend,
      });
      applySavedDiscoverySubnet(subnetToSend);
      if (enabled && subnetToSend !== 'auto') {
        setLastCustomSubnet(subnetToSend);
      }

      if (enabled) {
        await triggerDiscoveryScan({ quiet: true });
        notificationStore.success('Discovery enabled — scanning network...', 2000);
      } else {
        notificationStore.info('Discovery disabled', 2000);
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: false,
        }));
      }

      return true;
    } catch (error) {
      logger.error('Failed to update discovery setting', error);
      notificationStore.error('Failed to update discovery setting');
      setDiscoveryEnabled(previousEnabled);
      applySavedDiscoverySubnet(previousSubnet);
      return false;
    } finally {
      setSavingDiscoverySettings(false);
      await loadDiscoveredNodes();
    }
  };

  const commitDiscoverySubnet = async (rawValue: string): Promise<boolean> => {
    if (envOverrides().discoverySubnet) {
      return false;
    }

    const value = rawValue.trim();
    if (!value) {
      setDiscoverySubnetError('Enter at least one subnet in CIDR format (e.g., 192.168.1.0/24)');
      return false;
    }
    if (!isValidCIDR(value)) {
      setDiscoverySubnetError(
        'Use CIDR format such as 192.168.1.0/24 (comma-separated for multiple)',
      );
      return false;
    }

    const normalizedValue = normalizeSubnetList(value);
    if (!normalizedValue) {
      setDiscoverySubnetError('Enter at least one valid subnet in CIDR format');
      return false;
    }

    const previousSubnet = discoverySubnet();
    const previousNormalized =
      previousSubnet.toLowerCase() === 'auto' ? '' : normalizeSubnetList(previousSubnet);

    if (normalizedValue === previousNormalized) {
      setDiscoverySubnetDraft(normalizedValue);
      setDiscoverySubnetError(undefined);
      setLastCustomSubnet(normalizedValue);
      return true;
    }

    setSavingDiscoverySettings(true);

    try {
      setDiscoverySubnetError(undefined);
      await SettingsAPI.updateSystemSettings({
        discoveryEnabled: discoveryEnabled(),
        discoverySubnet: normalizedValue,
      });
      setLastCustomSubnet(normalizedValue);
      applySavedDiscoverySubnet(normalizedValue);
      if (discoveryEnabled()) {
        await triggerDiscoveryScan({ quiet: true });
        notificationStore.success('Discovery subnet updated — scanning network...', 2000);
      } else {
        notificationStore.success('Discovery subnet saved', 2000);
      }
      return true;
    } catch (error) {
      logger.error('Failed to update discovery subnet', error);
      notificationStore.error('Failed to update discovery subnet');
      applySavedDiscoverySubnet(previousSubnet);
      setDiscoverySubnetDraft(previousSubnet === 'auto' ? '' : normalizeSubnetList(previousSubnet));
      return false;
    } finally {
      setDiscoverySubnetError(undefined);
      setSavingDiscoverySettings(false);
      await loadDiscoveredNodes();
    }
  };

  const handleTemperatureMonitoringChange = async (enabled: boolean): Promise<void> => {
    if (temperatureMonitoringLocked() || savingTemperatureSetting()) {
      return;
    }

    const previous = temperatureMonitoringEnabled();
    setTemperatureMonitoringEnabled(enabled);
    setSavingTemperatureSetting(true);

    try {
      await SettingsAPI.updateSystemSettings({ temperatureMonitoringEnabled: enabled });
      if (enabled) {
        notificationStore.success('Temperature monitoring enabled', 2000);
      } else {
        notificationStore.info('Temperature monitoring disabled', 2000);
      }
    } catch (error) {
      logger.error('Failed to update temperature monitoring setting', error);
      notificationStore.error(
        error instanceof Error
          ? error.message
          : 'Failed to update temperature monitoring setting',
      );
      setTemperatureMonitoringEnabled(previous);
    } finally {
      setSavingTemperatureSetting(false);
    }
  };

  const handleNodeTemperatureMonitoringChange = async (nodeId: string, enabled: boolean | null): Promise<void> => {
    if (savingTemperatureSetting()) {
      return;
    }

    const node = nodes().find((n) => n.id === nodeId);
    if (!node) {
      return;
    }

    const previous = node.temperatureMonitoringEnabled;
    setSavingTemperatureSetting(true);

    // Update local state optimistically
    setNodes(
      nodes().map((n) => (n.id === nodeId ? { ...n, temperatureMonitoringEnabled: enabled } : n)),
    );

    // Also update editingNode if this is the node being edited
    if (editingNode()?.id === nodeId) {
      setEditingNode({ ...editingNode()!, temperatureMonitoringEnabled: enabled });
    }

    try {
      await NodesAPI.updateNode(nodeId, { temperatureMonitoringEnabled: enabled } as any);
      if (enabled === true) {
        notificationStore.success('Temperature monitoring enabled for this node', 2000);
      } else if (enabled === false) {
        notificationStore.info('Temperature monitoring disabled for this node', 2000);
      } else {
        notificationStore.info('Using global temperature monitoring setting', 2000);
      }
    } catch (error) {
      logger.error('Failed to update node temperature monitoring setting', error);
      notificationStore.error(
        error instanceof Error
          ? error.message
          : 'Failed to update temperature monitoring setting',
      );
      // Revert on error
      setNodes(
        nodes().map((n) => (n.id === nodeId ? { ...n, temperatureMonitoringEnabled: previous } : n)),
      );
      // Also revert editingNode
      if (editingNode()?.id === nodeId) {
        setEditingNode({ ...editingNode()!, temperatureMonitoringEnabled: previous });
      }
    } finally {
      setSavingTemperatureSetting(false);
    }
  };

  const handleDiscoveryModeChange = async (mode: 'auto' | 'custom') => {
    if (envOverrides().discoverySubnet || savingDiscoverySettings()) {
      return;
    }
    if (mode === discoveryMode()) {
      return;
    }

    if (mode === 'auto') {
      const previousSubnet = discoverySubnet();
      setDiscoveryMode('auto');
      setDiscoverySubnetDraft('');
      setDiscoverySubnetError(undefined);
      setSavingDiscoverySettings(true);
      try {
        await SettingsAPI.updateSystemSettings({
          discoveryEnabled: discoveryEnabled(),
          discoverySubnet: 'auto',
        });
        applySavedDiscoverySubnet('auto');
        if (discoveryEnabled()) {
          await triggerDiscoveryScan({ quiet: true });
        }
        notificationStore.info(
          'Auto discovery scans each network phase. Large networks may take longer.',
          4000,
        );
      } catch (error) {
        logger.error('Failed to update discovery subnet', error);
        notificationStore.error('Failed to update discovery subnet');
        applySavedDiscoverySubnet(previousSubnet);
      } finally {
        setSavingDiscoverySettings(false);
        await loadDiscoveredNodes();
      }
      return;
    }

    setDiscoveryMode('custom');
    const rawDraft = discoverySubnet() !== 'auto' ? discoverySubnet() : lastCustomSubnet() || '';
    const normalizedDraft = normalizeSubnetList(rawDraft);
    setDiscoverySubnetDraft(normalizedDraft);
    setDiscoverySubnetError(undefined);
    queueMicrotask(() => {
      discoverySubnetInputRef?.focus();
      discoverySubnetInputRef?.select();
    });
  };

  // Load nodes and system settings on mount
  onMount(async () => {
    // Subscribe to events
    const unsubscribeAutoRegister = eventBus.on('node_auto_registered', () => {
      // Close any open modals
      setShowNodeModal(false);
      setEditingNode(null);
      // Reload nodes
      loadNodes();
      loadDiscoveredNodes();
    });

    const unsubscribeRefresh = eventBus.on('refresh_nodes', () => {
      loadNodes();
    });

    const unsubscribeDiscovery = eventBus.on('discovery_updated', (data) => {
      if (!data) {
        updateDiscoveredNodesFromServers([]);
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: false,
        }));
        return;
      }

      if (Array.isArray(data.servers)) {
        updateDiscoveredNodesFromServers(data.servers as RawDiscoveredServer[], {
          merge: !!data.immediate,
        });
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: data.scanning ?? prev.scanning,
          lastResultAt: data.timestamp ?? Date.now(),
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
        }));
      } else if (!data.immediate) {
        // Ensure we clear stale results when the update explicitly reports no servers
        updateDiscoveredNodesFromServers([]);
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: data.scanning ?? prev.scanning,
          lastResultAt: data.timestamp ?? prev.lastResultAt,
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
        }));
      } else {
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: data.scanning ?? prev.scanning,
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
        }));
      }
    });

    const unsubscribeDiscoveryStatus = eventBus.on('discovery_status', (data) => {
      if (!data) {
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: false,
        }));
        return;
      }

      setDiscoveryScanStatus((prev) => ({
        ...prev,
        scanning: !!data.scanning,
        subnet: data.subnet || prev.subnet,
        lastScanStartedAt: data.scanning ? (data.timestamp ?? Date.now()) : prev.lastScanStartedAt,
        lastResultAt: !data.scanning && data.timestamp ? data.timestamp : prev.lastResultAt,
      }));

      if (typeof data.subnet === 'string' && data.subnet !== discoverySubnet()) {
        applySavedDiscoverySubnet(data.subnet);
      }
    });

    // Poll for node updates when modal is open
    let pollInterval: ReturnType<typeof setInterval> | undefined;
    createEffect(() => {
      // Clear any existing interval first
      if (pollInterval) {
        clearInterval(pollInterval);
        pollInterval = undefined;
      }

      if (showNodeModal()) {
        // Start polling every 3 seconds when modal is open
        pollInterval = setInterval(() => {
          loadNodes();
          loadDiscoveredNodes();
        }, 3000);
      }
    });

    // Poll for discovered nodes every 30 seconds
    const discoveryInterval = setInterval(() => {
      loadDiscoveredNodes();
    }, 30000);

    // Clean up on unmount
    onCleanup(() => {
      unsubscribeAutoRegister();
      unsubscribeRefresh();
      unsubscribeDiscovery();
      unsubscribeDiscoveryStatus();
      if (pollInterval) {
        clearInterval(pollInterval);
      }
      clearInterval(discoveryInterval);
    });

    try {
      // Load data with small delays to prevent rate limit bursts
      // Load security status first as it's lightweight
      await loadSecurityStatus();

      // Small delay to prevent burst
      await new Promise((resolve) => setTimeout(resolve, 50));

      // Load nodes
      await loadNodes();

      // Another small delay
      await new Promise((resolve) => setTimeout(resolve, 50));

      // Load discovered nodes
      await loadDiscoveredNodes();

      // Load system settings
      try {
        const systemSettings = await SettingsAPI.getSystemSettings();
        const rawPVESecs =
          typeof systemSettings.pvePollingInterval === 'number'
            ? Math.round(systemSettings.pvePollingInterval)
            : PVE_POLLING_MIN_SECONDS;
        const clampedPVESecs = Math.min(
          PVE_POLLING_MAX_SECONDS,
          Math.max(PVE_POLLING_MIN_SECONDS, rawPVESecs),
        );
        setPVEPollingInterval(clampedPVESecs);
        const presetMatch = PVE_POLLING_PRESETS.find((opt) => opt.value === clampedPVESecs);
        if (presetMatch) {
          setPVEPollingSelection(presetMatch.value);
        } else {
          setPVEPollingSelection('custom');
          setPVEPollingCustomSeconds(clampedPVESecs);
        }
        setAllowedOrigins(systemSettings.allowedOrigins || '*');
        // Connection timeout is backend-only
        // Load discovery settings (default to false when unset)
        setDiscoveryEnabled(systemSettings.discoveryEnabled ?? false);
        applySavedDiscoverySubnet(systemSettings.discoverySubnet);
        // Load embedding settings
        setAllowEmbedding(systemSettings.allowEmbedding ?? false);
        setAllowedEmbedOrigins(systemSettings.allowedEmbedOrigins || '');
        // Load webhook security settings
        setWebhookAllowedPrivateCIDRs(systemSettings.webhookAllowedPrivateCIDRs || '');
        setTemperatureMonitoringEnabled(
          typeof systemSettings.temperatureMonitoringEnabled === 'boolean'
            ? systemSettings.temperatureMonitoringEnabled
            : true,
        );
        // Load hideLocalLogin setting
        setHideLocalLogin(systemSettings.hideLocalLogin ?? false);

        // Backup polling controls
        if (typeof systemSettings.backupPollingEnabled === 'boolean') {
          setBackupPollingEnabled(systemSettings.backupPollingEnabled);
        } else {
          setBackupPollingEnabled(true);
        }
        const intervalSeconds =
          typeof systemSettings.backupPollingInterval === 'number'
            ? Math.max(0, Math.floor(systemSettings.backupPollingInterval))
            : 0;
        setBackupPollingInterval(intervalSeconds);
        if (intervalSeconds > 0) {
          setBackupPollingCustomMinutes(Math.max(1, Math.round(intervalSeconds / 60)));
        }
        // Determine if the loaded interval is a custom value
        const isPresetInterval = BACKUP_INTERVAL_OPTIONS.some((opt) => opt.value === intervalSeconds);
        setBackupPollingUseCustom(!isPresetInterval && intervalSeconds > 0);
        // Load auto-update settings
        setAutoUpdateEnabled(systemSettings.autoUpdateEnabled || false);
        setAutoUpdateCheckInterval(systemSettings.autoUpdateCheckInterval || 24);
        setAutoUpdateTime(systemSettings.autoUpdateTime || '03:00');
        if (systemSettings.updateChannel) {
          setUpdateChannel(systemSettings.updateChannel as 'stable' | 'rc');
        }
        // Track environment variable overrides
        if (systemSettings.envOverrides) {
          setEnvOverrides(systemSettings.envOverrides);
        }
      } catch (error) {
        logger.error('Failed to load settings', error);
      }

      // Load version information
      try {
        const version = await UpdatesAPI.getVersion();
        setVersionInfo(version);
        // Also set it in the store so it's available globally
        updateStore.checkForUpdates(); // This will load version info too
        // Only use version.channel as fallback if user hasn't configured a preference
        // The user's saved updateChannel preference should take priority
        // Check the signal value since systemSettings is scoped to the previous try block
        if (version.channel && !updateChannel()) {
          setUpdateChannel(version.channel as 'stable' | 'rc');
        }
      } catch (error) {
        logger.error('Failed to load version', error);
      }
    } catch (error) {
      logger.error('Failed to load configuration', error);
    } finally {
      // Mark initial load as complete even if there were errors
      setInitialLoadComplete(true);
    }
  });

  // Re-merge temperature data from WebSocket state when it updates
  createEffect(
    on(
      () => state.nodes,
      (stateNodes) => {
        const currentNodes = nodes();

        // Only run if we have nodes loaded and state has data
        if (stateNodes && stateNodes.length > 0 && currentNodes.length > 0) {
          const updatedNodes = currentNodes.map((node) => {
            // Match nodes by ID or by name (handling .lan suffix variations)
            const stateNode = stateNodes.find((n) => {
              // Try exact ID match first
              if (n.id === node.id) return true;
              // Try exact name match
              if (n.name === node.name) return true;
              // Try name with/without .lan suffix
              const nodeNameBase = node.name.replace(/\.lan$/, '');
              const stateNameBase = n.name.replace(/\.lan$/, '');
              if (nodeNameBase === stateNameBase) return true;
              // Also check if state node ID contains the config node name
              if (n.id.includes(node.name) || node.name.includes(n.name)) return true;
              return false;
            });

            // Merge temperature data from state if available
            if (stateNode?.temperature) {
              return { ...node, temperature: stateNode.temperature };
            }
            return node;
          });
          setNodes(updatedNodes);
        }
      },
    ),
  );

  const saveSettings = async () => {
    try {
      if (
        activeTab() === 'system-general' ||
        activeTab() === 'system-network' ||
        activeTab() === 'system-updates' ||
        activeTab() === 'system-backups'
      ) {
        // Save system settings using typed API
        await SettingsAPI.updateSystemSettings({
          pvePollingInterval: pvePollingInterval(),
          allowedOrigins: allowedOrigins(),
          // Connection timeout is backend-only
          // Discovery settings are saved immediately on toggle
          updateChannel: updateChannel(),
          autoUpdateEnabled: autoUpdateEnabled(),
          autoUpdateCheckInterval: autoUpdateCheckInterval(),
          autoUpdateTime: autoUpdateTime(),
          backupPollingEnabled: backupPollingEnabled(),
          backupPollingInterval: backupPollingInterval(),
          allowEmbedding: allowEmbedding(),
          allowedEmbedOrigins: allowedEmbedOrigins(),
          webhookAllowedPrivateCIDRs: webhookAllowedPrivateCIDRs(),
        });
      }

      showSuccess('Settings saved successfully. Service restart may be required for port changes.');
      setHasUnsavedChanges(false);

      // Reload the page after a short delay to ensure the new settings are applied
      setTimeout(() => {
        window.location.reload();
      }, 3000);
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to save settings');
    }
  };

  const nodePendingDeleteLabel = () => {
    const node = nodePendingDelete();
    if (!node) return '';
    return node.displayName || node.name || node.host || node.id;
  };

  const nodePendingDeleteHost = () => nodePendingDelete()?.host || '';
  const nodePendingDeleteType = () => nodePendingDelete()?.type || '';
  const nodePendingDeleteTypeLabel = () => {
    switch (nodePendingDeleteType()) {
      case 'pve':
        return 'Proxmox VE node';
      case 'pbs':
        return 'Proxmox Backup Server';
      case 'pmg':
        return 'Proxmox Mail Gateway';
      default:
        return 'Pulse node';
    }
  };

  const requestDeleteNode = (node: NodeConfigWithStatus) => {
    setNodePendingDelete(node);
    setShowDeleteNodeModal(true);
  };

  const cancelDeleteNode = () => {
    if (deleteNodeLoading()) return;
    setShowDeleteNodeModal(false);
    setNodePendingDelete(null);
  };

  const deleteNode = async () => {
    const pending = nodePendingDelete();
    if (!pending) return;
    setDeleteNodeLoading(true);
    try {
      await NodesAPI.deleteNode(pending.id);
      setNodes(nodes().filter((n) => n.id !== pending.id));
      const label = pending.displayName || pending.name || pending.host || pending.id;
      showSuccess(`${label} removed successfully`);
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to delete node');
    } finally {
      setDeleteNodeLoading(false);
      setShowDeleteNodeModal(false);
      setNodePendingDelete(null);
    }
  };

  const testNodeConnection = async (nodeId: string) => {
    try {
      const node = nodes().find((n) => n.id === nodeId);
      if (!node) {
        throw new Error('Node not found');
      }

      // Use the existing node test endpoint which uses stored credentials
      const result = await NodesAPI.testExistingNode(nodeId);
      if (result.status === 'success') {
        // Check for warnings in the response
        if (result.warnings && Array.isArray(result.warnings) && result.warnings.length > 0) {
          const warningMessage = result.message + '\n\nWarnings:\n' + result.warnings.map((w: string) => '• ' + w).join('\n');
          showWarning(warningMessage);
        } else {
          showSuccess(result.message || 'Connection successful');
        }
      } else {
        throw new Error(result.message || 'Connection failed');
      }
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Connection test failed');
    }
  };

  const refreshClusterNodes = async (nodeId: string) => {
    try {
      notificationStore.info('Refreshing cluster membership...', 2000);
      const result = await NodesAPI.refreshClusterNodes(nodeId);
      if (result.status === 'success') {
        if (result.nodesAdded && result.nodesAdded > 0) {
          showSuccess(`Found ${result.nodesAdded} new node(s) in cluster "${result.clusterName}"`);
        } else {
          showSuccess(`Cluster "${result.clusterName}" membership verified (${result.newNodeCount} nodes)`);
        }
        // Refresh nodes list to show updated cluster info
        await loadNodes();
      } else {
        throw new Error('Failed to refresh cluster');
      }
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to refresh cluster membership');
    }
  };

  const checkForUpdates = async () => {
    setCheckingForUpdates(true);
    try {
      // Force check with current channel selection
      await updateStore.checkForUpdates(true);
      const info = updateStore.updateInfo();
      setUpdateInfo(info);

      // If update was dismissed, clear it so user can see it again
      if (info?.available && updateStore.isDismissed()) {
        updateStore.clearDismissed();
      }

      if (!info?.available) {
        showSuccess('You are running the latest version');
      }
    } catch (error) {
      showError('Failed to check for updates');
      logger.error('Update check error', error);
    } finally {
      setCheckingForUpdates(false);
    }
  };

  const handleExport = async () => {
    if (!exportPassphrase()) {
      const hasAuth = securityStatus()?.hasAuthentication;
      showError(
        hasAuth
          ? useCustomPassphrase()
            ? 'Please enter a passphrase'
            : 'Please enter your password'
          : 'Please enter a passphrase',
      );
      return;
    }

    // Backend requires at least 12 characters for encryption security
    if (exportPassphrase().length < 12) {
      const hasAuth = securityStatus()?.hasAuthentication;
      showError(
        hasAuth && !useCustomPassphrase()
          ? 'Your password must be at least 12 characters. Please use a custom passphrase instead.'
          : 'Passphrase must be at least 12 characters long',
      );
      return;
    }

    // Only check for API token if user is not authenticated via password
    // If user is logged in with password, session auth is sufficient
    const hasPasswordAuth = securityStatus()?.hasAuthentication;
    if (!hasPasswordAuth && securityStatus()?.apiTokenConfigured && !getApiClientToken()) {
      setApiTokenModalSource('export');
      setShowApiTokenModal(true);
      return;
    }

    try {
      // Get CSRF token from cookie
      const csrfCookie = document.cookie
        .split('; ')
        .find((row) => row.startsWith('pulse_csrf='));
      const csrfToken = csrfCookie
        ? decodeURIComponent(csrfCookie.split('=').slice(1).join('='))
        : undefined;

      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };

      // Add CSRF token if available
      if (csrfToken) {
        headers['X-CSRF-Token'] = csrfToken;
      }

      // Add API token if configured
      const apiToken = getApiClientToken();
      if (apiToken) {
        headers['X-API-Token'] = apiToken;
      }

      const response = await fetch('/api/config/export', {
        method: 'POST',
        headers,
        credentials: 'include', // Include cookies for session auth
        body: JSON.stringify({ passphrase: exportPassphrase() }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        // Handle authentication errors
        if (response.status === 401 || response.status === 403) {
          // Check if we're using API token auth (not password auth)
          const hasPasswordAuth = securityStatus()?.hasAuthentication;
          if (!hasPasswordAuth) {
            // Clear invalid token if we had one
            const hadToken = getApiClientToken();
            if (hadToken) {
              clearApiClientToken();
              showError('Invalid or expired API token. Please re-enter.');
              setApiTokenModalSource('export');
              setShowApiTokenModal(true);
              return;
            }
            if (errorText.includes('API_TOKEN') || errorText.includes('API_TOKENS')) {
              setApiTokenModalSource('export');
              setShowApiTokenModal(true);
              return;
            }
          }
          throw new Error('Export requires authentication');
        }
        throw new Error(errorText || 'Export failed');
      }

      const data = await response.json();

      // Create and download file
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `pulse-config-${new Date().toISOString().split('T')[0]}.json`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);

      showSuccess('Configuration exported successfully');
      setShowExportDialog(false);
      setExportPassphrase('');
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : 'Failed to export configuration';
      showError(errorMessage);
      logger.error('Export error', error);
    }
  };

  const handleImport = async () => {
    if (!importPassphrase()) {
      showError('Please enter the password');
      return;
    }

    if (!importFile()) {
      showError('Please select a file to import');
      return;
    }

    // Only check for API token if user is not authenticated via password
    // If user is logged in with password, session auth is sufficient
    const hasPasswordAuth = securityStatus()?.hasAuthentication;
    if (!hasPasswordAuth && securityStatus()?.apiTokenConfigured && !getApiClientToken()) {
      setApiTokenModalSource('import');
      setShowApiTokenModal(true);
      return;
    }

    try {
      const fileContent = await importFile()!.text();

      // Support three formats:
      // 1. UI export: {status: "success", data: "base64string"}
      // 2. Legacy format: {data: "base64string"}
      // 3. CLI export: raw base64 string (no JSON wrapper)
      let encryptedData: string;

      // Try to parse as JSON first
      try {
        const exportData = JSON.parse(fileContent);

        if (typeof exportData === 'string') {
          // Raw base64 string wrapped in JSON (edge case)
          encryptedData = exportData;
        } else if (exportData.data) {
          // Standard format with data field
          encryptedData = exportData.data;
        } else {
          showError('Invalid backup file format. Expected encrypted data in "data" field.');
          return;
        }
      } catch (_parseError) {
        // Not JSON - treat entire contents as raw base64 from CLI export
        encryptedData = fileContent.trim();
      }

      // Get CSRF token from cookie
      const csrfCookie = document.cookie
        .split('; ')
        .find((row) => row.startsWith('pulse_csrf='));
      const csrfToken = csrfCookie
        ? decodeURIComponent(csrfCookie.split('=').slice(1).join('='))
        : undefined;

      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };

      // Add CSRF token if available
      if (csrfToken) {
        headers['X-CSRF-Token'] = csrfToken;
      }

      // Add API token if configured
      const apiToken = getApiClientToken();
      if (apiToken) {
        headers['X-API-Token'] = apiToken;
      }

      const response = await fetch('/api/config/import', {
        method: 'POST',
        headers,
        credentials: 'include', // Include cookies for session auth
        body: JSON.stringify({
          passphrase: importPassphrase(),
          data: encryptedData,
        }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        // Handle authentication errors
        if (response.status === 401 || response.status === 403) {
          // Check if we're using API token auth (not password auth)
          const hasPasswordAuth = securityStatus()?.hasAuthentication;
          if (!hasPasswordAuth) {
            // Clear invalid token if we had one
            const hadToken = getApiClientToken();
            if (hadToken) {
              clearApiClientToken();
              showError('Invalid or expired API token. Please re-enter.');
              setApiTokenModalSource('import');
              setShowApiTokenModal(true);
              return;
            }
            if (errorText.includes('API_TOKEN') || errorText.includes('API_TOKENS')) {
              setApiTokenModalSource('import');
              setShowApiTokenModal(true);
              return;
            }
          }
          throw new Error('Import requires authentication');
        }
        throw new Error(errorText || 'Import failed');
      }

      showSuccess('Configuration imported successfully. Reloading...');
      setShowImportDialog(false);
      setImportPassphrase('');
      setImportFile(null);

      // Reload page to apply new configuration
      setTimeout(() => window.location.reload(), 2000);
    } catch (error) {
      showError('Failed to import configuration');
      logger.error('Import error', error);
    }
  };

  return (
    <>
      <div class="space-y-6">
        {/* Page header - no card wrapper for cleaner hierarchy */}
        <div class="px-1">
          <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-2">
            {headerMeta().title}
          </h1>
          <p class="text-base text-gray-600 dark:text-gray-400">{headerMeta().description}</p>
        </div>

        {/* Save notification bar - only show when there are unsaved changes */}
        <Show
          when={
            hasUnsavedChanges() &&
            (activeTab() === 'proxmox' ||
              activeTab() === 'system-general' ||
              activeTab() === 'system-network' ||
              activeTab() === 'system-updates' ||
              activeTab() === 'system-backups')
          }
        >
          <div class="bg-amber-50 dark:bg-amber-900/30 border-l-4 border-amber-500 dark:border-amber-400 rounded-r-lg shadow-sm p-4">
            <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
              <div class="flex items-start gap-3">
                <svg
                  class="w-5 h-5 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <div>
                  <p class="font-semibold text-amber-900 dark:text-amber-100">Unsaved changes</p>
                  <p class="text-sm text-amber-700 dark:text-amber-200 mt-0.5">
                    Your changes will be lost if you navigate away
                  </p>
                </div>
              </div>
              <div class="flex w-full sm:w-auto gap-3">
                <button
                  type="button"
                  class="flex-1 sm:flex-initial px-5 py-2.5 text-sm font-medium bg-amber-600 text-white rounded-lg hover:bg-amber-700 shadow-sm transition-colors"
                  onClick={saveSettings}
                >
                  Save Changes
                </button>
                <button
                  type="button"
                  class="px-4 py-2.5 text-sm font-medium text-amber-700 dark:text-amber-200 hover:underline transition-colors"
                  onClick={() => {
                    window.location.reload();
                  }}
                >
                  Discard
                </button>
              </div>
            </div>
          </div>
        </Show>

        <Card padding="none" class="relative lg:flex overflow-hidden">
          <div
            class={`hidden lg:flex lg:flex-col ${sidebarCollapsed() ? 'w-16' : 'w-72'} ${sidebarCollapsed() ? 'lg:min-w-[4rem] lg:max-w-[4rem] lg:basis-[4rem]' : 'lg:min-w-[18rem] lg:max-w-[18rem] lg:basis-[18rem]'} relative border-b border-gray-200 dark:border-gray-700 lg:border-b-0 lg:border-r lg:border-gray-200 dark:lg:border-gray-700 lg:align-top flex-shrink-0 transition-all duration-200`}
            aria-label="Settings navigation"
            aria-expanded={!sidebarCollapsed()}
          >
            <div
              class={`sticky top-0 ${sidebarCollapsed() ? 'px-2' : 'px-4'} py-5 space-y-5 transition-all duration-200`}
            >
              <Show when={!sidebarCollapsed()}>
                <div class="flex items-center justify-between pb-2 border-b border-gray-200 dark:border-gray-700">
                  <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Settings</h2>
                  <button
                    type="button"
                    onClick={() => setSidebarCollapsed(true)}
                    class="p-1 rounded-md text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                    aria-label="Collapse sidebar"
                  >
                    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M11 19l-7-7 7-7m8 14l-7-7 7-7"
                      />
                    </svg>
                  </button>
                </div>
              </Show>
              <Show when={sidebarCollapsed()}>
                <button
                  type="button"
                  onClick={() => setSidebarCollapsed(false)}
                  class="w-full p-2 rounded-md text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                  aria-label="Expand sidebar"
                >
                  <svg
                    class="w-5 h-5 mx-auto"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M13 5l7 7-7 7M5 5l7 7-7 7"
                    />
                  </svg>
                </button>
              </Show>
              <div id="settings-sidebar-menu" class="space-y-5">
                <For each={tabGroups}>
                  {(group) => (
                    <div class="space-y-2">
                      <Show when={!sidebarCollapsed()}>
                        <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {group.label}
                        </p>
                      </Show>
                      <div class="space-y-1.5">
                        <For each={group.items}>
                          {(item) => {
                            const isActive = () => activeTab() === item.id;
                            return (
                              <button
                                type="button"
                                aria-current={isActive() ? 'page' : undefined}
                                disabled={item.disabled}
                                class={`flex w-full items-center ${sidebarCollapsed() ? 'justify-center' : 'gap-2.5'} rounded-md ${sidebarCollapsed() ? 'px-2 py-2.5' : 'px-3 py-2'
                                  } text-sm font-medium transition-colors ${item.disabled
                                    ? 'opacity-60 cursor-not-allowed text-gray-400 dark:text-gray-600'
                                    : isActive()
                                      ? 'bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-200'
                                      : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700/60 dark:hover:text-gray-100'
                                  }`}
                                onClick={() => {
                                  if (item.disabled) return;
                                  setActiveTab(item.id);
                                }}
                                title={sidebarCollapsed() ? item.label : undefined}
                              >
                                <item.icon class="w-4 h-4" {...(item.iconProps || {})} />
                                <Show when={!sidebarCollapsed()}>
                                  <span class="truncate">{item.label}</span>
                                </Show>
                              </button>
                            );
                          }}
                        </For>
                      </div>
                    </div>
                  )}
                </For>
              </div>
            </div>
          </div>

          <div class="flex-1 overflow-hidden">
            <Show when={flatTabs.length > 0}>
              <div class="lg:hidden border-b border-gray-200 dark:border-gray-700">
                <div
                  class="flex gap-1 px-2 py-1 w-full overflow-x-auto scrollbar-hide"
                  style="-webkit-overflow-scrolling: touch;"
                >
                  <For each={flatTabs}>
                    {(tab) => {
                      const isActive = activeTab() === tab.id;
                      const disabled = tab.disabled;
                      return (
                        <button
                          type="button"
                          disabled={disabled}
                          class={`px-3 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap ${disabled
                            ? 'opacity-60 cursor-not-allowed text-gray-400 dark:text-gray-600 border-transparent'
                            : isActive
                              ? 'text-blue-600 dark:text-blue-300 border-blue-500 dark:border-blue-400'
                              : 'text-gray-600 dark:text-gray-400 border-transparent hover:text-blue-500 dark:hover:text-blue-300 hover:border-blue-300/70 dark:hover:border-blue-500/50'
                            }`}
                          onClick={() => {
                            if (disabled) return;
                            setActiveTab(tab.id);
                          }}
                        >
                          {tab.label}
                        </button>
                      );
                    }}
                  </For>
                </div>
              </div>
            </Show>

            <div class="p-6 lg:p-8">
              <Show when={activeTab() === 'proxmox'}>
                <SettingsSectionNav
                  current={selectedAgent()}
                  onSelect={handleSelectAgent}
                  class="mb-6"
                />
              </Show>

              {/* Recommendation banner for Proxmox tab */}
              <Show when={activeTab() === 'proxmox'}>
                <div class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 mb-6 dark:border-blue-800 dark:bg-blue-900/20">
                  <div class="flex items-start gap-3">
                    <svg
                      class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                    <div class="flex-1">
                      <p class="text-sm text-blue-800 dark:text-blue-200">
                        <strong>Recommended:</strong> Install the Pulse agent on your Proxmox nodes for automatic setup, temperature monitoring, and AI features.
                      </p>
                      <button
                        type="button"
                        onClick={() => navigate('/settings/agents')}
                        class="mt-2 text-sm font-medium text-blue-700 hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200 underline"
                      >
                        Go to Agents tab →
                      </button>
                    </div>
                  </div>
                </div>
              </Show>

              {/* PVE Nodes Tab */}
              <Show when={activeTab() === 'proxmox' && selectedAgent() === 'pve'}>
                <div class="space-y-6 mt-6">
                  <div class="space-y-4">
                    <Show when={!initialLoadComplete()}>
                      <div class="flex items-center justify-center rounded-lg border border-dashed border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/40 py-12 text-sm text-gray-500 dark:text-gray-400">
                        Loading configuration...
                      </div>
                    </Show>
                    <Show when={initialLoadComplete()}>
                      <Card padding="lg">
                        <div class="space-y-4">
                          <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                            <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                              Proxmox VE nodes
                            </h4>
                            <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end">
                              {/* Discovery toggle */}
                              <div
                                class="flex items-center gap-2 sm:gap-3"
                                title="Enable automatic discovery of Proxmox servers on your network"
                              >
                                <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">
                                  Discovery
                                </span>
                                <Toggle
                                  checked={discoveryEnabled()}
                                  onChange={async (e: ToggleChangeEvent) => {
                                    if (
                                      envOverrides().discoveryEnabled ||
                                      savingDiscoverySettings()
                                    ) {
                                      e.preventDefault();
                                      return;
                                    }
                                    const success = await handleDiscoveryEnabledChange(
                                      e.currentTarget.checked,
                                    );
                                    if (!success) {
                                      e.currentTarget.checked = discoveryEnabled();
                                    }
                                  }}
                                  disabled={
                                    envOverrides().discoveryEnabled || savingDiscoverySettings()
                                  }
                                  containerClass="gap-2"
                                  label={
                                    <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                                      {discoveryEnabled() ? 'On' : 'Off'}
                                    </span>
                                  }
                                />
                              </div>

                              <Show when={discoveryEnabled()}>
                                <button
                                  type="button"
                                  onClick={async () => {
                                    notificationStore.info('Refreshing discovery...', 2000);
                                    try {
                                      await triggerDiscoveryScan({ quiet: true });
                                    } finally {
                                      await loadDiscoveredNodes();
                                    }
                                  }}
                                  class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-1"
                                  title="Refresh discovered servers"
                                >
                                  <svg
                                    width="16"
                                    height="16"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
                                    <polyline points="23 4 23 10 17 10"></polyline>
                                    <polyline points="1 20 1 14 7 14"></polyline>
                                    <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                                  </svg>
                                  <span class="hidden sm:inline">Refresh</span>
                                </button>
                              </Show>

                              <button
                                type="button"
                                onClick={() => {
                                  setEditingNode(null);
                                  setCurrentNodeType('pve');
                                  setModalResetKey((prev) => prev + 1);
                                  setShowNodeModal(true);
                                }}
                                class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
                              >
                                <svg
                                  width="16"
                                  height="16"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  stroke="currentColor"
                                  stroke-width="2"
                                >
                                  <line x1="12" y1="5" x2="12" y2="19"></line>
                                  <line x1="5" y1="12" x2="19" y2="12"></line>
                                </svg>
                                <span class="sm:hidden">Add</span>
                                <span class="hidden sm:inline">Add PVE Node</span>
                              </button>
                            </div>
                          </div>

                          <Show when={pveNodes().length > 0}>
                            <PveNodesTable
                              nodes={pveNodes()}
                              stateNodes={state.nodes ?? []}
                              stateHosts={state.hosts ?? []}
                              globalTemperatureMonitoringEnabled={temperatureMonitoringEnabled()}
                              temperatureTransports={temperatureTransportInfo()}
                              onTestConnection={testNodeConnection}
                              onEdit={(node) => {
                                setEditingNode(node);
                                setCurrentNodeType('pve');
                                setShowNodeModal(true);
                              }}
                              onDelete={requestDeleteNode}
                              onRefreshCluster={refreshClusterNodes}
                            />
                          </Show>

                          <Show
                            when={
                              pveNodes().length === 0 &&
                              discoveredNodes().filter((n) => n.type === 'pve').length === 0
                            }
                          >
                            <div class="flex flex-col items-center justify-center py-12 px-4 text-center">
                              <div class="rounded-full bg-gray-100 dark:bg-gray-800 p-4 mb-4">
                                <Server class="h-8 w-8 text-gray-400 dark:text-gray-500" />
                              </div>
                              <p class="text-base font-medium text-gray-900 dark:text-gray-100 mb-1">
                                No PVE nodes configured
                              </p>
                              <p class="text-sm text-gray-500 dark:text-gray-400">
                                Add a Proxmox VE node to start monitoring your infrastructure
                              </p>
                            </div>
                          </Show>
                        </div>
                      </Card>
                    </Show>

                    {/* Discovered PVE nodes - only show when discovery is enabled */}
                    <Show when={discoveryEnabled()}>
                      <div class="space-y-3">
                        <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
                          <Show when={discoveryScanStatus().scanning}>
                            <span class="flex items-center gap-2">
                              <Loader class="h-4 w-4 animate-spin" />
                              <span>Scanning your network for Proxmox VE servers…</span>
                            </span>
                          </Show>
                          <Show
                            when={
                              !discoveryScanStatus().scanning &&
                              (discoveryScanStatus().lastResultAt ||
                                discoveryScanStatus().lastScanStartedAt)
                            }
                          >
                            <span>
                              Last scan{' '}
                              {formatRelativeTime(
                                discoveryScanStatus().lastResultAt ??
                                discoveryScanStatus().lastScanStartedAt,
                              )}
                            </span>
                          </Show>
                        </div>
                        <Show
                          when={
                            discoveryScanStatus().errors && discoveryScanStatus().errors!.length
                          }
                        >
                          <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-2">
                            <span class="font-medium">Discovery issues:</span>
                            <ul class="list-disc ml-4 mt-1 space-y-0.5">
                              <For each={discoveryScanStatus().errors || []}>
                                {(err) => <li>{err}</li>}
                              </For>
                            </ul>
                            <Show
                              when={
                                discoveryMode() === 'auto' &&
                                (discoveryScanStatus().errors || []).some((err) =>
                                  /timed out|timeout/i.test(err),
                                )
                              }
                            >
                              <p class="mt-2 text-[0.7rem] font-medium text-amber-700 dark:text-amber-300">
                                Large networks can time out in auto mode. Switch to a custom subnet
                                for faster, targeted scans.
                              </p>
                            </Show>
                          </div>
                        </Show>
                        <Show
                          when={
                            discoveryScanStatus().scanning &&
                            discoveredNodes().filter((n) => n.type === 'pve').length === 0
                          }
                        >
                          <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                            <Loader class="h-4 w-4 animate-spin" />
                            <span>
                              Waiting for responses… this can take up to a minute depending on your
                              network size.
                            </span>
                          </div>
                        </Show>
                        <For each={discoveredNodes().filter((n) => n.type === 'pve')}>
                          {(server) => (
                            <div
                              class="bg-gray-50/50 dark:bg-gray-700/30 rounded-lg p-4 border border-gray-200/50 dark:border-gray-600/50 opacity-75 hover:opacity-100 transition-opacity cursor-pointer"
                              onClick={() => {
                                // Pre-fill the modal with discovered server info
                                setEditingNode({
                                  id: '',
                                  type: 'pve',
                                  name: server.hostname || `pve-${server.ip}`,
                                  host: `https://${server.ip}:${server.port}`,
                                  user: '',
                                  tokenName: '',
                                  tokenValue: '',
                                  verifySSL: false,
                                  monitorVMs: true,
                                  monitorContainers: true,
                                  monitorStorage: true,
                                  monitorBackups: true,
                                  monitorPhysicalDisks: false,
                                  status: 'pending',
                                } as NodeConfigWithStatus);
                                setCurrentNodeType('pve');
                                setShowNodeModal(true);
                              }}
                            >
                              <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                                <div class="flex-1 min-w-0">
                                  <div class="flex items-start gap-3">
                                    <div class="flex-shrink-0 w-3 h-3 mt-1.5 rounded-full bg-gray-400 animate-pulse"></div>
                                    <div class="flex-1 min-w-0">
                                      <h4 class="font-medium text-gray-700 dark:text-gray-300">
                                        {server.hostname || `Proxmox VE at ${server.ip}`}
                                      </h4>
                                      <p class="text-sm text-gray-500 dark:text-gray-500 mt-1">
                                        {server.ip}:{server.port}
                                      </p>
                                      <div class="flex items-center gap-2 mt-2">
                                        <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">
                                          Discovered
                                        </span>
                                        <span class="text-xs text-gray-500 dark:text-gray-400">
                                          Click to configure
                                        </span>
                                      </div>
                                    </div>
                                  </div>
                                </div>
                                <svg
                                  width="20"
                                  height="20"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  class="text-gray-400 mt-1"
                                >
                                  <path
                                    d="M12 5v14m-7-7h14"
                                    stroke="currentColor"
                                    stroke-width="2"
                                    stroke-linecap="round"
                                  />
                                </svg>
                              </div>
                            </div>
                          )}
                        </For>
                      </div>
                    </Show>
                  </div>
                </div>
              </Show>

              {/* PBS Nodes Tab */}
              <Show when={activeTab() === 'proxmox' && selectedAgent() === 'pbs'}>
                <div class="space-y-6 mt-6">
                  <div class="space-y-4">
                    <Show when={!initialLoadComplete()}>
                      <div class="flex items-center justify-center rounded-lg border border-dashed border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/40 py-12 text-sm text-gray-500 dark:text-gray-400">
                        Loading configuration...
                      </div>
                    </Show>
                    <Show when={initialLoadComplete()}>
                      <Card padding="lg">
                        <div class="space-y-4">
                          <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                            <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                              Proxmox Backup Server nodes
                            </h4>
                            <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end">
                              {/* Discovery toggle */}
                              <div
                                class="flex items-center gap-2 sm:gap-3"
                                title="Enable automatic discovery of PBS servers on your network"
                              >
                                <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">
                                  Discovery
                                </span>
                                <Toggle
                                  checked={discoveryEnabled()}
                                  onChange={async (e: ToggleChangeEvent) => {
                                    if (
                                      envOverrides().discoveryEnabled ||
                                      savingDiscoverySettings()
                                    ) {
                                      e.preventDefault();
                                      return;
                                    }
                                    const success = await handleDiscoveryEnabledChange(
                                      e.currentTarget.checked,
                                    );
                                    if (!success) {
                                      e.currentTarget.checked = discoveryEnabled();
                                    }
                                  }}
                                  disabled={
                                    envOverrides().discoveryEnabled || savingDiscoverySettings()
                                  }
                                  containerClass="gap-2"
                                  label={
                                    <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                                      {discoveryEnabled() ? 'On' : 'Off'}
                                    </span>
                                  }
                                />
                              </div>

                              <Show when={discoveryEnabled()}>
                                <button
                                  type="button"
                                  onClick={async () => {
                                    notificationStore.info('Refreshing discovery...', 2000);
                                    try {
                                      await triggerDiscoveryScan({ quiet: true });
                                    } finally {
                                      await loadDiscoveredNodes();
                                    }
                                  }}
                                  class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-1"
                                  title="Refresh discovered servers"
                                >
                                  <svg
                                    width="16"
                                    height="16"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
                                    <polyline points="23 4 23 10 17 10"></polyline>
                                    <polyline points="1 20 1 14 7 14"></polyline>
                                    <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                                  </svg>
                                  <span class="hidden sm:inline">Refresh</span>
                                </button>
                              </Show>

                              <button
                                type="button"
                                onClick={() => {
                                  setEditingNode(null);
                                  setCurrentNodeType('pbs');
                                  setModalResetKey((prev) => prev + 1);
                                  setShowNodeModal(true);
                                }}
                                class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
                              >
                                <svg
                                  width="16"
                                  height="16"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  stroke="currentColor"
                                  stroke-width="2"
                                >
                                  <line x1="12" y1="5" x2="12" y2="19"></line>
                                  <line x1="5" y1="12" x2="19" y2="12"></line>
                                </svg>
                                <span class="sm:hidden">Add</span>
                                <span class="hidden sm:inline">Add PBS Node</span>
                              </button>
                            </div>
                          </div>

                          <Show when={pbsNodes().length > 0}>
                            <PbsNodesTable
                              nodes={pbsNodes()}
                              statePbs={state.pbs ?? []}
                              globalTemperatureMonitoringEnabled={temperatureMonitoringEnabled()}
                              onTestConnection={testNodeConnection}
                              onEdit={(node) => {
                                setEditingNode(node);
                                setCurrentNodeType('pbs');
                                setShowNodeModal(true);
                              }}
                              onDelete={requestDeleteNode}
                            />
                          </Show>

                          <Show
                            when={
                              pbsNodes().length === 0 &&
                              discoveredNodes().filter((n) => n.type === 'pbs').length === 0
                            }
                          >
                            <div class="flex flex-col items-center justify-center py-12 px-4 text-center">
                              <div class="rounded-full bg-gray-100 dark:bg-gray-800 p-4 mb-4">
                                <HardDrive class="h-8 w-8 text-gray-400 dark:text-gray-500" />
                              </div>
                              <p class="text-base font-medium text-gray-900 dark:text-gray-100 mb-1">
                                No PBS nodes configured
                              </p>
                              <p class="text-sm text-gray-500 dark:text-gray-400">
                                Add a Proxmox Backup Server to monitor your backup infrastructure
                              </p>
                            </div>
                          </Show>
                        </div>
                      </Card>
                    </Show>

                    {/* Discovered PBS nodes - only show when discovery is enabled */}
                    <Show when={discoveryEnabled()}>
                      <div class="space-y-3">
                        <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
                          <Show when={discoveryScanStatus().scanning}>
                            <span class="flex items-center gap-2">
                              <Loader class="h-4 w-4 animate-spin" />
                              <span>Scanning your network for Proxmox Backup Servers…</span>
                            </span>
                          </Show>
                          <Show
                            when={
                              !discoveryScanStatus().scanning &&
                              (discoveryScanStatus().lastResultAt ||
                                discoveryScanStatus().lastScanStartedAt)
                            }
                          >
                            <span>
                              Last scan{' '}
                              {formatRelativeTime(
                                discoveryScanStatus().lastResultAt ??
                                discoveryScanStatus().lastScanStartedAt,
                              )}
                            </span>
                          </Show>
                        </div>
                        <Show
                          when={
                            discoveryScanStatus().errors && discoveryScanStatus().errors!.length
                          }
                        >
                          <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-2">
                            <span class="font-medium">Discovery issues:</span>
                            <ul class="list-disc ml-4 mt-1 space-y-0.5">
                              <For each={discoveryScanStatus().errors || []}>
                                {(err) => <li>{err}</li>}
                              </For>
                            </ul>
                            <Show
                              when={
                                discoveryMode() === 'auto' &&
                                (discoveryScanStatus().errors || []).some((err) =>
                                  /timed out|timeout/i.test(err),
                                )
                              }
                            >
                              <p class="mt-2 text-[0.7rem] font-medium text-amber-700 dark:text-amber-300">
                                Large networks can time out in auto mode. Switch to a custom subnet
                                for faster, targeted scans.
                              </p>
                            </Show>
                          </div>
                        </Show>
                        <Show
                          when={
                            discoveryScanStatus().scanning &&
                            discoveredNodes().filter((n) => n.type === 'pbs').length === 0
                          }
                        >
                          <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                            <Loader class="h-4 w-4 animate-spin" />
                            <span>
                              Waiting for responses… this can take up to a minute depending on your
                              network size.
                            </span>
                          </div>
                        </Show>
                        <For each={discoveredNodes().filter((n) => n.type === 'pbs')}>
                          {(server) => (
                            <div
                              class="bg-gray-50/50 dark:bg-gray-700/30 rounded-lg p-4 border border-gray-200/50 dark:border-gray-600/50 opacity-75 hover:opacity-100 transition-opacity cursor-pointer"
                              onClick={() => {
                                // Pre-fill the modal with discovered server info
                                setEditingNode({
                                  id: '',
                                  type: 'pbs',
                                  name: server.hostname || `pbs-${server.ip}`,
                                  host: `https://${server.ip}:${server.port}`,
                                  user: '',
                                  tokenName: '',
                                  tokenValue: '',
                                  verifySSL: false,
                                  monitorDatastores: true,
                                  monitorSyncJobs: true,
                                  monitorVerifyJobs: true,
                                  monitorPruneJobs: true,
                                  monitorGarbageJobs: true,
                                  status: 'pending',
                                } as NodeConfigWithStatus);
                                setCurrentNodeType('pbs');
                                setShowNodeModal(true);
                              }}
                            >
                              <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                                <div class="flex-1 min-w-0">
                                  <div class="flex items-start gap-3">
                                    <div class="flex-shrink-0 w-3 h-3 mt-1.5 rounded-full bg-gray-400 animate-pulse"></div>
                                    <div class="flex-1 min-w-0">
                                      <h4 class="font-medium text-gray-700 dark:text-gray-300">
                                        {server.hostname || `Backup Server at ${server.ip}`}
                                      </h4>
                                      <p class="text-sm text-gray-500 dark:text-gray-500 mt-1">
                                        {server.ip}:{server.port}
                                      </p>
                                      <div class="flex items-center gap-2 mt-2">
                                        <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">
                                          Discovered
                                        </span>
                                        <span class="text-xs text-gray-500 dark:text-gray-400">
                                          Click to configure
                                        </span>
                                      </div>
                                    </div>
                                  </div>
                                </div>
                                <svg
                                  width="20"
                                  height="20"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  class="text-gray-400 mt-1"
                                >
                                  <path
                                    d="M12 5v14m-7-7h14"
                                    stroke="currentColor"
                                    stroke-width="2"
                                    stroke-linecap="round"
                                  />
                                </svg>
                              </div>
                            </div>
                          )}
                        </For>
                      </div>
                    </Show>
                  </div>
                </div>
              </Show>
              {/* PMG Nodes Tab */}
              <Show when={activeTab() === 'proxmox' && selectedAgent() === 'pmg'}>
                <div class="space-y-6 mt-6">
                  <div class="space-y-4">
                    <Show when={!initialLoadComplete()}>
                      <div class="flex items-center justify-center rounded-lg border border-dashed border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/40 py-12 text-sm text-gray-500 dark:text-gray-400">
                        Loading configuration...
                      </div>
                    </Show>

                    <Show when={initialLoadComplete()}>
                      <Card padding="lg">
                        <div class="space-y-4">
                          <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                            <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                              Proxmox Mail Gateway nodes
                            </h4>
                            <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end">
                              {/* Discovery toggle */}
                              <div
                                class="flex items-center gap-2 sm:gap-3"
                                title="Enable automatic discovery of PMG servers on your network"
                              >
                                <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">
                                  Discovery
                                </span>
                                <Toggle
                                  checked={discoveryEnabled()}
                                  onChange={async (e: ToggleChangeEvent) => {
                                    if (
                                      envOverrides().discoveryEnabled ||
                                      savingDiscoverySettings()
                                    ) {
                                      e.preventDefault();
                                      return;
                                    }
                                    const success = await handleDiscoveryEnabledChange(
                                      e.currentTarget.checked,
                                    );
                                    if (!success) {
                                      e.currentTarget.checked = discoveryEnabled();
                                    }
                                  }}
                                  disabled={
                                    envOverrides().discoveryEnabled || savingDiscoverySettings()
                                  }
                                  containerClass="gap-2"
                                  label={
                                    <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                                      {discoveryEnabled() ? 'On' : 'Off'}
                                    </span>
                                  }
                                />
                              </div>

                              <Show when={discoveryEnabled()}>
                                <button
                                  type="button"
                                  onClick={async () => {
                                    notificationStore.info('Refreshing discovery...', 2000);
                                    try {
                                      await triggerDiscoveryScan({ quiet: true });
                                    } finally {
                                      await loadDiscoveredNodes();
                                    }
                                  }}
                                  class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-1"
                                  title="Refresh discovered servers"
                                >
                                  <svg
                                    width="16"
                                    height="16"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
                                    <polyline points="23 4 23 10 17 10"></polyline>
                                    <polyline points="1 20 1 14 7 14"></polyline>
                                    <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
                                  </svg>
                                  <span class="hidden sm:inline">Refresh</span>
                                </button>
                              </Show>

                              <button
                                type="button"
                                onClick={() => {
                                  setEditingNode(null);
                                  setCurrentNodeType('pmg');
                                  setModalResetKey((prev) => prev + 1);
                                  setShowNodeModal(true);
                                }}
                                class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
                              >
                                <svg
                                  width="16"
                                  height="16"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  stroke="currentColor"
                                  stroke-width="2"
                                >
                                  <line x1="12" y1="5" x2="12" y2="19"></line>
                                  <line x1="5" y1="12" x2="19" y2="12"></line>
                                </svg>
                                <span class="sm:hidden">Add</span>
                                <span class="hidden sm:inline">Add PMG Node</span>
                              </button>
                            </div>
                          </div>

                          <Show when={pmgNodes().length > 0}>
                            <PmgNodesTable
                              nodes={pmgNodes()}
                              statePmg={state.pmg ?? []}
                              globalTemperatureMonitoringEnabled={temperatureMonitoringEnabled()}
                              onTestConnection={testNodeConnection}
                              onEdit={(node) => {
                                setEditingNode(nodes().find((n) => n.id === node.id) ?? null);
                                setCurrentNodeType('pmg');
                                setModalResetKey((prev) => prev + 1);
                                setShowNodeModal(true);
                              }}
                              onDelete={requestDeleteNode}
                            />
                          </Show>

                          <Show when={pmgNodes().length === 0}>
                            <div class="flex flex-col items-center justify-center py-12 px-4 text-center">
                              <div class="rounded-full bg-gray-100 dark:bg-gray-800 p-4 mb-4">
                                <Mail class="h-8 w-8 text-gray-400 dark:text-gray-500" />
                              </div>
                              <p class="text-base font-medium text-gray-900 dark:text-gray-100 mb-1">
                                No PMG nodes configured
                              </p>
                              <p class="text-sm text-gray-500 dark:text-gray-400">
                                Add a Proxmox Mail Gateway to monitor mail queue and quarantine
                                metrics
                              </p>
                            </div>
                          </Show>
                        </div>
                      </Card>
                    </Show>

                    {/* Discovered PMG nodes - only show when discovery is enabled */}
                    <Show when={discoveryEnabled()}>
                      <div class="space-y-3">
                        <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
                          <Show when={discoveryScanStatus().scanning}>
                            <span class="flex items-center gap-2">
                              <Loader class="h-4 w-4 animate-spin" />
                              Scanning network...
                            </span>
                          </Show>
                          <Show
                            when={
                              !discoveryScanStatus().scanning &&
                              (discoveryScanStatus().lastResultAt ||
                                discoveryScanStatus().lastScanStartedAt)
                            }
                          >
                            <span>
                              Last scan{' '}
                              {formatRelativeTime(
                                discoveryScanStatus().lastResultAt ??
                                discoveryScanStatus().lastScanStartedAt,
                              )}
                            </span>
                          </Show>
                        </div>
                        <Show
                          when={
                            discoveryScanStatus().errors && discoveryScanStatus().errors!.length
                          }
                        >
                          <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-2">
                            <span class="font-medium">Discovery issues:</span>
                            <ul class="list-disc ml-4 mt-1 space-y-0.5">
                              <For each={discoveryScanStatus().errors || []}>
                                {(err) => <li>{err}</li>}
                              </For>
                            </ul>
                            <Show
                              when={
                                discoveryMode() === 'auto' &&
                                (discoveryScanStatus().errors || []).some((err) =>
                                  /timed out|timeout/i.test(err),
                                )
                              }
                            >
                              <p class="mt-2 text-[0.7rem] font-medium text-amber-700 dark:text-amber-300">
                                Large networks can time out in auto mode. Switch to a custom subnet
                                for faster, targeted scans.
                              </p>
                            </Show>
                          </div>
                        </Show>
                        <Show
                          when={
                            discoveryScanStatus().scanning &&
                            discoveredNodes().filter((n) => n.type === 'pmg').length === 0
                          }
                        >
                          <div class="text-center py-6 text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800/50 rounded-lg border-2 border-dashed border-gray-300 dark:border-gray-600">
                            <svg
                              class="h-8 w-8 mx-auto mb-2 animate-pulse text-purple-500"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2"
                            >
                              <circle cx="11" cy="11" r="8" />
                              <path d="m21 21-4.35-4.35" />
                            </svg>
                            <p class="text-sm">Scanning for PMG servers...</p>
                          </div>
                        </Show>
                        <For each={discoveredNodes().filter((n) => n.type === 'pmg')}>
                          {(server) => (
                            <div
                              class="bg-gradient-to-r from-purple-50 to-transparent dark:from-purple-900/20 dark:to-transparent border-l-4 border-purple-500 rounded-lg p-4 cursor-pointer hover:shadow-md transition-all"
                              onClick={() => {
                                setEditingNode(null);
                                setCurrentNodeType('pmg');
                                setModalResetKey((prev) => prev + 1);
                                setShowNodeModal(true);
                                setTimeout(() => {
                                  const hostInput = document.querySelector(
                                    'input[placeholder*="192.168"]',
                                  ) as HTMLInputElement;
                                  if (hostInput) {
                                    hostInput.value = server.ip;
                                    hostInput.dispatchEvent(new Event('input', { bubbles: true }));
                                  }
                                }, 50);
                              }}
                            >
                              <div class="flex items-start justify-between">
                                <div class="flex items-start gap-3 flex-1 min-w-0">
                                  <svg
                                    width="24"
                                    height="24"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                    class="text-purple-500 flex-shrink-0 mt-0.5"
                                  >
                                    <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path>
                                    <polyline points="22,6 12,13 2,6"></polyline>
                                  </svg>
                                  <div class="flex-1 min-w-0">
                                    <h4 class="font-medium text-gray-900 dark:text-gray-100 truncate">
                                      {server.hostname || `PMG at ${server.ip}`}
                                    </h4>
                                    <p class="text-sm text-gray-500 dark:text-gray-500 mt-1">
                                      {server.ip}:{server.port}
                                    </p>
                                    <div class="flex items-center gap-2 mt-2">
                                      <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">
                                        Discovered
                                      </span>
                                      <span class="text-xs text-gray-500 dark:text-gray-400">
                                        Click to configure
                                      </span>
                                    </div>
                                  </div>
                                </div>
                                <svg
                                  width="20"
                                  height="20"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  class="text-gray-400 mt-1"
                                >
                                  <path
                                    d="M12 5v14m-7-7h14"
                                    stroke="currentColor"
                                    stroke-width="2"
                                    stroke-linecap="round"
                                  />
                                </svg>
                              </div>
                            </div>
                          )}
                        </For>
                      </div>
                    </Show>
                  </div>
                </div>
              </Show>
              {/* Unified Agents Tab */}
              <Show when={activeTab() === 'agents'}>
                <UnifiedAgents />
              </Show>

              {/* System General Tab */}
              <Show when={activeTab() === 'system-general'}>
                <div class="space-y-6">
                  <Card
                    padding="none"
                    class="overflow-hidden border border-gray-200 dark:border-gray-700"
                    border={false}
                  >
                    <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                      <div class="flex items-center gap-3">
                        <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
                          <Sliders
                            class="w-5 h-5 text-blue-600 dark:text-blue-300"
                            strokeWidth={2}
                          />
                        </div>
                        <SectionHeader
                          title="General"
                          description="Appearance and display preferences"
                          size="sm"
                          class="flex-1"
                        />
                      </div>
                    </div>
                    <div class="p-6 space-y-5">
                      <div class="flex items-center justify-between gap-3">
                        <div class="text-sm text-gray-600 dark:text-gray-400">
                          <p class="font-medium text-gray-900 dark:text-gray-100">Dark mode</p>
                          <p class="text-xs text-gray-500 dark:text-gray-400">
                            Toggle to match your environment. Pulse remembers this preference on
                            each browser.
                          </p>
                        </div>
                        <Toggle
                          checked={props.darkMode()}
                          onChange={(event) => {
                            const desired = (event.currentTarget as HTMLInputElement).checked;
                            if (desired !== props.darkMode()) {
                              props.toggleDarkMode();
                            }
                          }}
                        />
                      </div>
                    </div>
                  </Card>
                  <Card
                    padding="none"
                    class="overflow-hidden border border-gray-200 dark:border-gray-700"
                    border={false}
                  >
                    <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                      <div class="flex items-center gap-3">
                        <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
                          <Activity class="w-5 h-5 text-blue-600 dark:text-blue-300" strokeWidth={2} />
                        </div>
                        <SectionHeader
                          title="Monitoring cadence"
                          description="Control how frequently Pulse polls Proxmox VE nodes."
                          size="sm"
                          class="flex-1"
                        />
                      </div>
                    </div>
                    <div class="p-6 space-y-5">
                      <div class="space-y-2">
                        <p class="text-sm text-gray-600 dark:text-gray-400">
                          Shorter intervals provide near-real-time updates at the cost of higher API
                          and CPU usage on each node. Set a longer interval to reduce load on busy
                          clusters.
                        </p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          Current cadence: {pvePollingInterval()} seconds (
                          {pvePollingInterval() >= 60
                            ? `${(pvePollingInterval() / 60).toFixed(
                              pvePollingInterval() % 60 === 0 ? 0 : 1,
                            )} minute${pvePollingInterval() / 60 === 1 ? '' : 's'}`
                            : 'under a minute'}
                          ).
                        </p>
                      </div>
                      <div class="space-y-4">
                        <div class="grid gap-2 sm:grid-cols-3">
                          <For each={PVE_POLLING_PRESETS}>
                            {(option) => (
                              <button
                                type="button"
                                class={`rounded-lg border px-3 py-2 text-left text-sm transition-colors ${pvePollingSelection() === option.value
                                  ? 'border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900/30 dark:text-blue-100'
                                  : 'border-gray-200 bg-white text-gray-700 hover:border-blue-400 hover:text-blue-600 dark:border-gray-600 dark:bg-gray-900/50 dark:text-gray-200'
                                  } ${pvePollingEnvLocked() ? 'opacity-60 cursor-not-allowed' : ''}`}
                                disabled={pvePollingEnvLocked()}
                                onClick={() => {
                                  if (pvePollingEnvLocked()) return;
                                  setPVEPollingSelection(option.value);
                                  setPVEPollingInterval(option.value);
                                  setHasUnsavedChanges(true);
                                }}
                              >
                                {option.label}
                              </button>
                            )}
                          </For>
                          <button
                            type="button"
                            class={`rounded-lg border px-3 py-2 text-left text-sm transition-colors ${pvePollingSelection() === 'custom'
                              ? 'border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900/30 dark:text-blue-100'
                              : 'border-gray-200 bg-white text-gray-700 hover:border-blue-400 hover:text-blue-600 dark:border-gray-600 dark:bg-gray-900/50 dark:text-gray-200'
                              } ${pvePollingEnvLocked() ? 'opacity-60 cursor-not-allowed' : ''}`}
                            disabled={pvePollingEnvLocked()}
                            onClick={() => {
                              if (pvePollingEnvLocked()) return;
                              setPVEPollingSelection('custom');
                              setPVEPollingInterval(pvePollingCustomSeconds());
                              setHasUnsavedChanges(true);
                            }}
                          >
                            Custom interval
                          </button>
                        </div>
                        <Show when={pvePollingSelection() === 'custom'}>
                          <div class="space-y-2 rounded-md border border-dashed border-gray-300 p-4 dark:border-gray-600">
                            <label class="text-xs font-medium text-gray-700 dark:text-gray-200">
                              Custom polling interval (10–3600 seconds)
                            </label>
                            <input
                              type="number"
                              min={PVE_POLLING_MIN_SECONDS}
                              max={PVE_POLLING_MAX_SECONDS}
                              value={pvePollingCustomSeconds()}
                              class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-900/60"
                              disabled={pvePollingEnvLocked()}
                              onInput={(e) => {
                                if (pvePollingEnvLocked()) return;
                                const parsed = Math.floor(Number(e.currentTarget.value));
                                if (Number.isNaN(parsed)) {
                                  return;
                                }
                                const clamped = Math.min(
                                  PVE_POLLING_MAX_SECONDS,
                                  Math.max(PVE_POLLING_MIN_SECONDS, parsed),
                                );
                                setPVEPollingCustomSeconds(clamped);
                                setPVEPollingInterval(clamped);
                                setHasUnsavedChanges(true);
                              }}
                            />
                            <p class="text-[0.68rem] text-gray-500 dark:text-gray-400">
                              Applies to all PVE clusters and standalone nodes.
                            </p>
                          </div>
                        </Show>
                        <Show when={pvePollingEnvLocked()}>
                          <div class="flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-800 dark:bg-amber-900/30 dark:text-amber-200">
                            <svg
                              class="h-4 w-4 flex-shrink-0"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2"
                            >
                              <circle cx="12" cy="12" r="10" />
                              <line x1="12" y1="8" x2="12" y2="12" />
                              <circle cx="12" cy="16" r="0.5" />
                            </svg>
                            <span>Managed via environment variable PVE_POLLING_INTERVAL.</span>
                          </div>
                        </Show>
                      </div>
                    </div>
                  </Card>
                </div>
              </Show>

              {/* System Network Tab */}
              <Show when={activeTab() === 'system-network'}>
                <div class="space-y-6">
                  <Card
                    tone="info"
                    padding="md"
                    border={false}
                    class="border border-blue-200 dark:border-blue-800"
                  >
                    <div class="flex items-start gap-3">
                      <svg
                        class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                        />
                      </svg>
                      <div class="text-sm text-blue-800 dark:text-blue-200">
                        <p class="font-medium mb-1">Configuration Priority</p>
                        <ul class="space-y-1">
                          <li>
                            • Some env vars override settings (API_TOKENS, legacy API_TOKEN, PORTS,
                            AUTH)
                          </li>
                          <li>• Changes made here are saved to system.json immediately</li>
                          <li>• Settings persist unless overridden by env vars</li>
                        </ul>
                      </div>
                    </div>
                  </Card>
                  <Card
                    padding="none"
                    class="overflow-hidden border border-gray-200 dark:border-gray-700"
                    border={false}
                  >
                    <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                      <div class="flex items-center gap-3">
                        <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
                          <Network
                            class="w-5 h-5 text-blue-600 dark:text-blue-300"
                            strokeWidth={2}
                          />
                        </div>
                        <SectionHeader
                          title="Network"
                          description="Discovery, CORS, and embedding settings"
                          size="sm"
                          class="flex-1"
                        />
                      </div>
                    </div>
                    <div class="p-6 space-y-8">
                      <section class="space-y-5">
                        <SectionHeader
                          title="Network discovery"
                          description="Control how Pulse scans for Proxmox services on your network."
                          size="sm"
                          align="left"
                        />
                        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                          <div class="text-sm text-gray-600 dark:text-gray-400">
                            <p class="font-medium text-gray-900 dark:text-gray-100">
                              Automatic scanning
                            </p>
                            <p class="text-xs text-gray-500 dark:text-gray-400">
                              Enable discovery to surface Proxmox VE, PBS, and PMG endpoints
                              automatically.
                            </p>
                          </div>
                          <Toggle
                            checked={discoveryEnabled()}
                            onChange={async (e: ToggleChangeEvent) => {
                              if (envOverrides().discoveryEnabled || savingDiscoverySettings()) {
                                e.preventDefault();
                                return;
                              }
                              const success = await handleDiscoveryEnabledChange(
                                e.currentTarget.checked,
                              );
                              if (!success) {
                                e.currentTarget.checked = discoveryEnabled();
                              }
                            }}
                            disabled={envOverrides().discoveryEnabled || savingDiscoverySettings()}
                            containerClass="gap-2"
                            label={
                              <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                                {discoveryEnabled() ? 'On' : 'Off'}
                              </span>
                            }
                          />
                        </div>

                        <Show when={discoveryEnabled()}>
                          <div class="space-y-4 rounded-lg border border-gray-200 bg-white/40 p-3 dark:border-gray-600 dark:bg-gray-800/40">
                            <fieldset class="space-y-2">
                              <legend class="text-xs font-medium text-gray-700 dark:text-gray-300">
                                Scan scope
                              </legend>
                              <div class="space-y-2">
                                <label
                                  class={`flex items-start gap-3 rounded-lg border p-2 transition-colors ${discoveryMode() === 'auto'
                                    ? 'border-blue-200 bg-blue-50/80 dark:border-blue-700 dark:bg-blue-900/20'
                                    : 'border-transparent hover:border-gray-200 dark:hover:border-gray-600'
                                    }`}
                                >
                                  <input
                                    type="radio"
                                    name="discoveryMode"
                                    value="auto"
                                    checked={discoveryMode() === 'auto'}
                                    onChange={async () => {
                                      if (discoveryMode() !== 'auto') {
                                        await handleDiscoveryModeChange('auto');
                                      }
                                    }}
                                    disabled={
                                      envOverrides().discoverySubnet || savingDiscoverySettings()
                                    }
                                    class="mt-1 h-4 w-4 border-gray-300 text-blue-600 focus:ring-blue-500"
                                  />
                                  <div class="space-y-1">
                                    <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                      Auto (slower, full scan)
                                    </p>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">
                                      Scans container, local, and gateway networks. Large networks
                                      may time out after two minutes.
                                    </p>
                                  </div>
                                </label>

                                <label
                                  class={`flex items-start gap-3 rounded-lg border p-2 transition-colors ${discoveryMode() === 'custom'
                                    ? 'border-blue-200 bg-blue-50/80 dark:border-blue-700 dark:bg-blue-900/20'
                                    : 'border-transparent hover:border-gray-200 dark:hover:border-gray-600'
                                    }`}
                                >
                                  <input
                                    type="radio"
                                    name="discoveryMode"
                                    value="custom"
                                    checked={discoveryMode() === 'custom'}
                                    onChange={() => {
                                      if (discoveryMode() !== 'custom') {
                                        handleDiscoveryModeChange('custom');
                                      }
                                    }}
                                    disabled={
                                      envOverrides().discoverySubnet || savingDiscoverySettings()
                                    }
                                    class="mt-1 h-4 w-4 border-gray-300 text-blue-600 focus:ring-blue-500"
                                  />
                                  <div class="space-y-1">
                                    <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                      Custom subnet (faster)
                                    </p>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">
                                      Limit discovery to one or more CIDR ranges to finish faster on
                                      large networks.
                                    </p>
                                  </div>
                                </label>
                                <Show when={discoveryMode() === 'custom'}>
                                  <div class="flex flex-wrap items-center gap-2 pl-9 pr-2">
                                    <span class="text-[0.68rem] uppercase tracking-wide text-gray-500 dark:text-gray-400">
                                      Common networks:
                                    </span>
                                    <For each={COMMON_DISCOVERY_SUBNETS}>
                                      {(preset) => {
                                        const baseValue = currentDraftSubnetValue();
                                        const currentSelections = parseSubnetList(baseValue);
                                        const isActive = currentSelections.includes(preset);
                                        return (
                                          <button
                                            type="button"
                                            class={`rounded border px-2.5 py-1 text-[0.7rem] transition-colors ${isActive
                                              ? 'border-blue-500 bg-blue-600 text-white dark:border-blue-400 dark:bg-blue-500'
                                              : 'border-gray-300 text-gray-700 hover:border-blue-400 hover:bg-blue-50 dark:border-gray-600 dark:text-gray-300 dark:hover:border-blue-500 dark:hover:bg-blue-900/30'
                                              }`}
                                            onClick={async () => {
                                              if (envOverrides().discoverySubnet) {
                                                return;
                                              }
                                              let selections = [...currentSelections];
                                              if (isActive) {
                                                selections = selections.filter(
                                                  (item) => item !== preset,
                                                );
                                              } else {
                                                selections.push(preset);
                                              }

                                              if (selections.length === 0) {
                                                setDiscoverySubnetDraft('');
                                                setLastCustomSubnet('');
                                                setDiscoverySubnetError(
                                                  'Enter at least one subnet in CIDR format (e.g., 192.168.1.0/24)',
                                                );
                                                return;
                                              }

                                              const updatedValue = normalizeSubnetList(
                                                selections.join(', '),
                                              );
                                              setDiscoveryMode('custom');
                                              setDiscoverySubnetDraft(updatedValue);
                                              setLastCustomSubnet(updatedValue);
                                              setDiscoverySubnetError(undefined);
                                              await commitDiscoverySubnet(updatedValue);
                                            }}
                                            disabled={envOverrides().discoverySubnet}
                                            classList={{
                                              'cursor-not-allowed opacity-60':
                                                envOverrides().discoverySubnet,
                                            }}
                                          >
                                            {preset}
                                          </button>
                                        );
                                      }}
                                    </For>
                                  </div>
                                </Show>
                              </div>
                            </fieldset>

                            <div class="space-y-2">
                              <div class="flex items-center justify-between gap-2">
                                <label
                                  for="discoverySubnetInput"
                                  class="text-xs font-medium text-gray-700 dark:text-gray-300"
                                >
                                  Discovery subnet
                                </label>
                                <span
                                  class="text-gray-400 hover:text-gray-500 dark:text-gray-500 dark:hover:text-gray-300"
                                  title="Use CIDR notation (comma-separated for multiple), e.g. 192.168.1.0/24, 10.0.0.0/24. Smaller ranges keep scans quick."
                                >
                                  <svg
                                    class="h-4 w-4"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
                                    <circle cx="12" cy="12" r="10"></circle>
                                    <path d="M12 16v-4"></path>
                                    <path d="M12 8h.01"></path>
                                  </svg>
                                </span>
                              </div>
                              <input
                                id="discoverySubnetInput"
                                ref={(el) => {
                                  discoverySubnetInputRef = el;
                                }}
                                type="text"
                                value={discoverySubnetDraft()}
                                placeholder={
                                  discoveryMode() === 'auto'
                                    ? 'auto (scan every network phase)'
                                    : '192.168.1.0/24, 10.0.0.0/24'
                                }
                                class={`w-full rounded-lg border px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 ${envOverrides().discoverySubnet
                                  ? 'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-600 dark:bg-amber-900/20 dark:text-amber-200 cursor-not-allowed opacity-60'
                                  : 'border-gray-300 bg-white dark:border-gray-600 dark:bg-gray-900/70'
                                  }`}
                                disabled={envOverrides().discoverySubnet}
                                onInput={(e) => {
                                  if (envOverrides().discoverySubnet) {
                                    return;
                                  }
                                  const rawValue = e.currentTarget.value;
                                  setDiscoverySubnetDraft(rawValue);
                                  if (discoveryMode() !== 'custom') {
                                    setDiscoveryMode('custom');
                                  }
                                  setLastCustomSubnet(rawValue);
                                  const trimmed = rawValue.trim();
                                  if (!trimmed) {
                                    setDiscoverySubnetError(undefined);
                                    return;
                                  }
                                  if (!isValidCIDR(trimmed)) {
                                    setDiscoverySubnetError(
                                      'Use CIDR format such as 192.168.1.0/24 (comma-separated for multiple)',
                                    );
                                  } else {
                                    setDiscoverySubnetError(undefined);
                                  }
                                }}
                                onBlur={async (e) => {
                                  if (
                                    envOverrides().discoverySubnet ||
                                    discoveryMode() !== 'custom'
                                  ) {
                                    return;
                                  }
                                  const rawValue = e.currentTarget.value;
                                  setDiscoverySubnetDraft(rawValue);
                                  const trimmed = rawValue.trim();
                                  if (!trimmed) {
                                    setDiscoverySubnetError(
                                      'Enter at least one subnet in CIDR format (e.g., 192.168.1.0/24)',
                                    );
                                    return;
                                  }
                                  if (!isValidCIDR(trimmed)) {
                                    setDiscoverySubnetError(
                                      'Use CIDR format such as 192.168.1.0/24 (comma-separated for multiple)',
                                    );
                                    return;
                                  }
                                  setDiscoverySubnetError(undefined);
                                  await commitDiscoverySubnet(rawValue);
                                }}
                                onKeyDown={(e) => {
                                  if (e.key === 'Enter') {
                                    e.preventDefault();
                                    (e.currentTarget as HTMLInputElement).blur();
                                  }
                                }}
                              />
                              <Show when={discoverySubnetError()}>
                                <p class="text-xs text-red-600 dark:text-red-400">
                                  {discoverySubnetError()}
                                </p>
                              </Show>
                              <Show when={!discoverySubnetError() && discoveryMode() === 'auto'}>
                                <p class="text-xs text-gray-500 dark:text-gray-400">
                                  Auto scans every reachable network phase. Large networks may time
                                  out — switch to custom subnets to narrow the search.
                                </p>
                              </Show>
                              <Show when={!discoverySubnetError() && discoveryMode() === 'custom'}>
                                <p class="text-xs text-gray-500 dark:text-gray-400">
                                  Example: 192.168.1.0/24, 10.0.0.0/24 (comma-separated). Smaller
                                  ranges finish faster and avoid timeouts.
                                </p>
                              </Show>
                            </div>
                          </div>
                        </Show>

                        <Show
                          when={envOverrides().discoveryEnabled || envOverrides().discoverySubnet}
                        >
                          <div class="rounded-lg border border-amber-200 bg-amber-100/80 p-3 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-900/20 dark:text-amber-200">
                            Discovery settings are locked by environment variables. Update the
                            service configuration and restart Pulse to change them here.
                          </div>
                        </Show>
                      </section>

                      <section class="space-y-3">
                        <h4 class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <circle cx="12" cy="12" r="10"></circle>
                            <path d="M2 12h20M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z"></path>
                          </svg>
                          Network Settings
                        </h4>
                        <div class="space-y-2">
                          <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                            CORS Allowed Origins
                          </label>
                          <p class="text-xs text-gray-600 dark:text-gray-400">
                            For reverse proxy setups (* = allow all, empty = same-origin only)
                          </p>
                          <div class="relative">
                            <input
                              type="text"
                              value={allowedOrigins()}
                              onChange={(e) => {
                                if (!envOverrides().allowedOrigins) {
                                  setAllowedOrigins(e.currentTarget.value);
                                  setHasUnsavedChanges(true);
                                }
                              }}
                              disabled={envOverrides().allowedOrigins}
                              placeholder="* or https://example.com"
                              class={`w-full px-3 py-1.5 text-sm border rounded-lg ${envOverrides().allowedOrigins
                                ? 'border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/20 cursor-not-allowed opacity-75'
                                : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800'
                                }`}
                            />
                            {envOverrides().allowedOrigins && (
                              <div class="mt-2 p-2 bg-amber-100 dark:bg-amber-900/30 border border-amber-300 dark:border-amber-700 rounded text-xs text-amber-800 dark:text-amber-200">
                                <div class="flex items-center gap-1">
                                  <svg
                                    class="w-4 h-4"
                                    fill="none"
                                    stroke="currentColor"
                                    viewBox="0 0 24 24"
                                  >
                                    <path
                                      stroke-linecap="round"
                                      stroke-linejoin="round"
                                      stroke-width="2"
                                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                                    />
                                  </svg>
                                  <span>Overridden by ALLOWED_ORIGINS environment variable</span>
                                </div>
                                <div class="mt-1 text-amber-700 dark:text-amber-300">
                                  Remove the env var and restart to enable UI configuration
                                </div>
                              </div>
                            )}
                          </div>
                        </div>
                      </section>

                      <section class="space-y-3">
                        <h4 class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <rect x="3" y="4" width="18" height="14" rx="2"></rect>
                            <path d="M7 20h10"></path>
                          </svg>
                          Embedding
                        </h4>
                        <p class="text-xs text-gray-600 dark:text-gray-400">
                          Allow Pulse to be embedded in iframes (e.g., Homepage dashboard)
                        </p>
                        <div class="space-y-3">
                          <div class="flex items-center gap-2">
                            <input
                              type="checkbox"
                              id="allowEmbedding"
                              checked={allowEmbedding()}
                              onChange={(e) => {
                                setAllowEmbedding(e.currentTarget.checked);
                                setHasUnsavedChanges(true);
                              }}
                              class="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                            />
                            <label
                              for="allowEmbedding"
                              class="text-sm text-gray-700 dark:text-gray-300"
                            >
                              Allow iframe embedding
                            </label>
                          </div>

                          <Show when={allowEmbedding()}>
                            <div class="space-y-2">
                              <label class="text-xs font-medium text-gray-700 dark:text-gray-300">
                                Allowed Embed Origins (optional)
                              </label>
                              <p class="text-xs text-gray-600 dark:text-gray-400">
                                Comma-separated list of origins that can embed Pulse (leave empty
                                for same-origin only)
                              </p>
                              <input
                                type="text"
                                value={allowedEmbedOrigins()}
                                onChange={(e) => {
                                  setAllowedEmbedOrigins(e.currentTarget.value);
                                  setHasUnsavedChanges(true);
                                }}
                                placeholder="https://my.domain, https://dashboard.example.com"
                                class="w-full px-3 py-1.5 text-sm border rounded-lg border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800"
                              />
                              <p class="text-xs text-gray-500 dark:text-gray-400">
                                Example: If Pulse is at <code>pulse.my.domain</code> and your
                                dashboard is at <code>my.domain</code>, add{' '}
                                <code>https://my.domain</code> here.
                              </p>
                            </div>
                          </Show>
                        </div>
                      </section>

                      {/* Webhook Security Settings */}
                      <section class="space-y-3">
                        <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2">
                          <svg
                            xmlns="http://www.w3.org/2000/svg"
                            class="h-4 w-4"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width={2}
                              d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                            />
                          </svg>
                          Webhook Security
                        </h3>
                        <div class="space-y-3">
                          <div>
                            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                              Allowed Private IP Ranges for Webhooks
                            </label>
                            <p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
                              By default, webhooks to private IP addresses are blocked for
                              security. Enter trusted CIDR ranges to allow webhooks to internal
                              services (leave empty to block all private IPs).
                            </p>
                            <input
                              type="text"
                              value={webhookAllowedPrivateCIDRs()}
                              onChange={(e) => {
                                setWebhookAllowedPrivateCIDRs(e.currentTarget.value);
                                setHasUnsavedChanges(true);
                              }}
                              placeholder="192.168.1.0/24, 10.0.0.0/8"
                              class="w-full px-3 py-1.5 text-sm border rounded-lg border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800"
                            />
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                              Example: <code>192.168.1.0/24,10.0.0.0/8</code> allows webhooks to
                              these private networks. Localhost and cloud metadata services
                              remain blocked.
                            </p>
                          </div>
                        </div>
                      </section>

                      <Card
                        tone="warning"
                        padding="sm"
                        border={false}
                        class="border border-amber-200 dark:border-amber-800"
                      >
                        <p class="text-xs text-amber-800 dark:text-amber-200 mb-2">
                          <strong>Port Configuration:</strong> Use{' '}
                          <code class="font-mono bg-amber-100 dark:bg-amber-800 px-1 rounded">
                            systemctl edit pulse
                          </code>
                        </p>
                        <p class="text-xs text-amber-700 dark:text-amber-300 font-mono">
                          [Service]
                          <br />
                          Environment="FRONTEND_PORT=8080"
                          <br />
                          <span class="text-xs text-amber-600 dark:text-amber-400">
                            Then restart: sudo systemctl restart pulse
                          </span>
                        </p>
                      </Card>
                    </div>
                  </Card>
                </div>
              </Show>

              {/* System Updates Tab */}
              <Show when={activeTab() === 'system-updates'}>
                <div class="space-y-6">
                  <Card
                    padding="none"
                    class="overflow-hidden border border-gray-200 dark:border-gray-700"
                    border={false}
                  >
                    <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                      <div class="flex items-center gap-3">
                        <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
                          <RefreshCw
                            class="w-5 h-5 text-blue-600 dark:text-blue-300"
                            strokeWidth={2}
                          />
                        </div>
                        <SectionHeader
                          title="Updates"
                          description="Version checking and automatic update configuration"
                          size="sm"
                          class="flex-1"
                        />
                      </div>
                    </div>
                    <div class="p-6 space-y-6">
                      <section class="space-y-4">
                        <div class="space-y-4">
                          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                            <div>
                              <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                Current Version
                              </label>
                              <p class="text-xs text-gray-600 dark:text-gray-400">
                                {versionInfo()?.version || 'Loading...'}
                                {versionInfo()?.isDevelopment && ' (Development)'}
                                {versionInfo()?.isDocker && ' - Docker'}
                              </p>
                            </div>
                            <button
                              type="button"
                              onClick={checkForUpdates}
                              disabled={
                                checkingForUpdates() ||
                                versionInfo()?.isDocker ||
                                versionInfo()?.isSourceBuild
                              }
                              class={`px-4 py-2 text-sm rounded-lg transition-colors flex items-center gap-2 ${versionInfo()?.isDocker || versionInfo()?.isSourceBuild
                                ? 'bg-gray-100 dark:bg-gray-700 text-gray-400 dark:text-gray-500 cursor-not-allowed'
                                : 'bg-blue-600 text-white hover:bg-blue-700'
                                }`}
                            >
                              {checkingForUpdates() ? (
                                <>
                                  <div class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></div>
                                  Checking...
                                </>
                              ) : (
                                <>Check for Updates</>
                              )}
                            </button>
                          </div>

                          <Show when={versionInfo()?.isDocker && !updateInfo()?.available}>
                            <div class="p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                              <p class="text-xs text-blue-800 dark:text-blue-200">
                                <strong>Docker Installation:</strong> Updates are managed through
                                Docker. Pull the latest image to update.
                              </p>
                            </div>
                          </Show>

                          <Show when={versionInfo()?.isSourceBuild}>
                            <div class="p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                              <p class="text-xs text-blue-800 dark:text-blue-200">
                                <strong>Built from source:</strong> Pull the latest code from git
                                and rebuild to update.
                              </p>
                            </div>
                          </Show>

                          <Show when={Boolean(updateInfo()?.warning)}>
                            <div class="p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg">
                              <p class="text-xs text-amber-800 dark:text-amber-200">
                                {updateInfo()?.warning}
                              </p>
                            </div>
                          </Show>

                          <Show when={updateInfo()?.available}>
                            <div class="p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg space-y-3">
                              <div>
                                <p class="text-sm font-medium text-green-800 dark:text-green-200">
                                  Update Available: {updateInfo()?.latestVersion}
                                </p>
                                <p class="text-xs text-green-700 dark:text-green-300 mt-1">
                                  Released:{' '}
                                  {updateInfo()?.releaseDate
                                    ? new Date(updateInfo()!.releaseDate).toLocaleDateString()
                                    : 'Unknown'}
                                </p>
                              </div>

                              <div class="p-2 bg-green-100 dark:bg-green-900/40 rounded space-y-2">
                                <p class="text-xs font-medium text-green-800 dark:text-green-200">
                                  How to update:
                                </p>
                                <Show when={versionInfo()?.deploymentType === 'proxmoxve'}>
                                  <p class="text-xs text-green-700 dark:text-green-300">
                                    Type{' '}
                                    <code class="px-1 py-0.5 bg-green-200 dark:bg-green-800 rounded">
                                      update
                                    </code>{' '}
                                    in the LXC console
                                  </p>
                                </Show>
                                <Show when={versionInfo()?.deploymentType === 'docker'}>
                                  <div class="text-xs text-green-700 dark:text-green-300 space-y-1">
                                    <p>Run these commands:</p>
                                    <code class="block p-1 bg-green-200 dark:bg-green-800 rounded text-xs">
                                      docker pull rcourtman/pulse:latest
                                      <br />
                                      docker restart pulse
                                    </code>
                                  </div>
                                </Show>
                                <Show
                                  when={
                                    versionInfo()?.deploymentType === 'systemd' ||
                                    versionInfo()?.deploymentType === 'manual'
                                  }
                                >
                                  <div class="text-xs text-green-700 dark:text-green-300 space-y-1">
                                    <p>
                                      Click the "Install Update" button below, or download and install manually:
                                    </p>
                                    <code class="block p-1 bg-green-200 dark:bg-green-800 rounded text-xs">
                                      curl -LO https://github.com/rcourtman/Pulse/releases/download/{updateInfo()?.latestVersion}/pulse-{updateInfo()?.latestVersion}-linux-amd64.tar.gz
                                      <br />
                                      sudo systemctl stop pulse
                                      <br />
                                      sudo tar -xzf pulse-{updateInfo()?.latestVersion}-linux-amd64.tar.gz -C /usr/local/bin pulse
                                      <br />
                                      sudo systemctl start pulse
                                    </code>
                                  </div>
                                </Show>
                                <Show when={versionInfo()?.deploymentType === 'development'}>
                                  <p class="text-xs text-green-700 dark:text-green-300">
                                    Pull latest changes and rebuild
                                  </p>
                                </Show>
                                <Show
                                  when={!versionInfo()?.deploymentType && versionInfo()?.isDocker}
                                >
                                  <p class="text-xs text-green-700 dark:text-green-300">
                                    Pull the latest Pulse Docker image and recreate your container.
                                  </p>
                                </Show>
                              </div>

                              <Show when={updateInfo()?.releaseNotes}>
                                <details class="mt-1">
                                  <summary class="text-xs text-green-700 dark:text-green-300 cursor-pointer">
                                    Release Notes
                                  </summary>
                                  <pre class="mt-2 text-xs text-green-600 dark:text-green-400 whitespace-pre-wrap font-mono bg-green-100 dark:bg-green-900/30 p-2 rounded">
                                    {updateInfo()?.releaseNotes}
                                  </pre>
                                </details>
                              </Show>
                            </div>
                          </Show>

                          <div class="border-t border-gray-200 dark:border-gray-600 pt-4 space-y-4">
                            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                              <div>
                                <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                  Update Channel
                                </label>
                                <p class="text-xs text-gray-600 dark:text-gray-400">
                                  Choose between stable and release candidate versions
                                </p>
                              </div>
                              <select
                                value={updateChannel()}
                                onChange={(e) => {
                                  setUpdateChannel(e.currentTarget.value as 'stable' | 'rc');
                                  setHasUnsavedChanges(true);
                                }}
                                disabled={versionInfo()?.isDocker}
                                class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 disabled:opacity-50"
                              >
                                <option value="stable">Stable</option>
                                <option value="rc">Release Candidate</option>
                              </select>
                            </div>

                            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                              <div>
                                <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                  Update Checks
                                </label>
                                <p class="text-xs text-gray-600 dark:text-gray-400">
                                  Automatically check for updates (installation is manual)
                                </p>
                              </div>
                              <label class="relative inline-flex items-center cursor-pointer">
                                <input
                                  type="checkbox"
                                  checked={autoUpdateEnabled()}
                                  onChange={(e) => {
                                    setAutoUpdateEnabled(e.currentTarget.checked);
                                    setHasUnsavedChanges(true);
                                  }}
                                  disabled={versionInfo()?.isDocker}
                                  class="sr-only peer"
                                />
                                <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
                              </label>
                            </div>

                            <Show when={autoUpdateEnabled()}>
                              <div class="space-y-4 rounded-md border border-gray-200 dark:border-gray-600 p-3">
                                <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                                  <div>
                                    <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                      Check Interval
                                    </label>
                                    <p class="text-xs text-gray-600 dark:text-gray-400">
                                      How often to check for updates
                                    </p>
                                  </div>
                                  <select
                                    value={autoUpdateCheckInterval()}
                                    onChange={(e) => {
                                      setAutoUpdateCheckInterval(parseInt(e.currentTarget.value));
                                      setHasUnsavedChanges(true);
                                    }}
                                    class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                                  >
                                    <option value="6">Every 6 hours</option>
                                    <option value="12">Every 12 hours</option>
                                    <option value="24">Daily</option>
                                    <option value="168">Weekly</option>
                                  </select>
                                </div>

                                <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                                  <div>
                                    <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                      Check Time
                                    </label>
                                    <p class="text-xs text-gray-600 dark:text-gray-400">
                                      Preferred time to check for updates
                                    </p>
                                  </div>
                                  <input
                                    type="time"
                                    value={autoUpdateTime()}
                                    onChange={(e) => {
                                      setAutoUpdateTime(e.currentTarget.value);
                                      setHasUnsavedChanges(true);
                                    }}
                                    class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                                  />
                                </div>
                              </div>
                            </Show>
                          </div>
                        </div>
                      </section>
                    </div>
                  </Card>
                </div>
              </Show>

              {/* System Backups Tab */}
              <Show when={activeTab() === 'system-backups'}>
                <div class="space-y-6">
                  <Card
                    padding="none"
                    class="overflow-hidden border border-gray-200 dark:border-gray-700"
                    border={false}
                  >
                    <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                      <div class="flex items-center gap-3">
                        <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
                          <Clock class="w-5 h-5 text-blue-600 dark:text-blue-300" strokeWidth={2} />
                        </div>
                        <SectionHeader
                          title="Backups"
                          description="Backup polling and configuration management"
                          size="sm"
                          class="flex-1"
                        />
                      </div>
                    </div>
                    <div class="p-6 space-y-8">
                      <section class="space-y-3">
                        <h4 class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                          >
                            <circle cx="12" cy="12" r="9" stroke-width="2" />
                            <path
                              d="M12 7v5l3 3"
                              stroke-width="2"
                              stroke-linecap="round"
                              stroke-linejoin="round"
                            />
                          </svg>
                          Backup polling
                        </h4>
                        <p class="text-xs text-gray-600 dark:text-gray-400">
                          Control how often Pulse queries Proxmox backup tasks, datastore contents,
                          and guest snapshots. Longer intervals reduce disk activity and API load.
                        </p>
                        <div class="space-y-3">
                          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                            <div>
                              <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                Enable backup polling
                              </p>
                              <p class="text-xs text-gray-600 dark:text-gray-400">
                                Required for dashboard backup status, storage snapshots, and
                                alerting.
                              </p>
                            </div>
                            <label class="relative inline-flex items-center cursor-pointer">
                              <input
                                type="checkbox"
                                class="sr-only peer"
                                checked={backupPollingEnabled()}
                                disabled={backupPollingEnvLocked()}
                                onChange={(e) => {
                                  setBackupPollingEnabled(e.currentTarget.checked);
                                  if (!backupPollingEnvLocked()) {
                                    setHasUnsavedChanges(true);
                                  }
                                }}
                              />
                              <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
                            </label>
                          </div>

                          <Show when={backupPollingEnabled()}>
                            <div class="space-y-3 rounded-md border border-gray-200 dark:border-gray-600 p-3">
                              <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                                <div>
                                  <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                    Polling interval
                                  </label>
                                  <p class="text-xs text-gray-600 dark:text-gray-400">
                                    {backupIntervalSummary()}
                                  </p>
                                </div>
                                <select
                                  value={backupIntervalSelectValue()}
                                  disabled={backupPollingEnvLocked()}
                                  onChange={(e) => {
                                    const value = e.currentTarget.value;
                                    if (value === 'custom') {
                                      setBackupPollingUseCustom(true);
                                      const minutes = Math.max(1, backupPollingCustomMinutes());
                                      setBackupPollingInterval(minutes * 60);
                                    } else {
                                      setBackupPollingUseCustom(false);
                                      const seconds = parseInt(value, 10);
                                      if (!Number.isNaN(seconds)) {
                                        setBackupPollingInterval(seconds);
                                        if (seconds > 0) {
                                          setBackupPollingCustomMinutes(
                                            Math.max(1, Math.round(seconds / 60)),
                                          );
                                        }
                                      }
                                    }
                                    if (!backupPollingEnvLocked()) {
                                      setHasUnsavedChanges(true);
                                    }
                                  }}
                                  class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 disabled:opacity-50"
                                >
                                  <For each={BACKUP_INTERVAL_OPTIONS}>
                                    {(option) => (
                                      <option value={String(option.value)}>{option.label}</option>
                                    )}
                                  </For>
                                  <option value="custom">Custom interval…</option>
                                </select>
                              </div>

                              <Show when={backupIntervalSelectValue() === 'custom'}>
                                <div class="space-y-2">
                                  <label class="text-xs font-medium text-gray-700 dark:text-gray-300">
                                    Custom interval (minutes)
                                  </label>
                                  <div class="flex items-center gap-3">
                                    <input
                                      type="number"
                                      min="1"
                                      max={BACKUP_INTERVAL_MAX_MINUTES}
                                      value={backupPollingCustomMinutes()}
                                      disabled={backupPollingEnvLocked()}
                                      onInput={(e) => {
                                        const value = Number(e.currentTarget.value);
                                        if (Number.isNaN(value)) {
                                          return;
                                        }
                                        const clamped = Math.max(
                                          1,
                                          Math.min(BACKUP_INTERVAL_MAX_MINUTES, Math.floor(value)),
                                        );
                                        setBackupPollingCustomMinutes(clamped);
                                        setBackupPollingInterval(clamped * 60);
                                        if (!backupPollingEnvLocked()) {
                                          setHasUnsavedChanges(true);
                                        }
                                      }}
                                      class="w-24 px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 disabled:opacity-50"
                                    />
                                    <span class="text-xs text-gray-500 dark:text-gray-400">
                                      1 – {BACKUP_INTERVAL_MAX_MINUTES} minutes (≈7 days max)
                                    </span>
                                  </div>
                                </div>
                              </Show>
                            </div>
                          </Show>

                          <Show when={backupPollingEnvLocked()}>
                            <div class="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 dark:border-amber-700 dark:bg-amber-900/20 p-3 text-xs text-amber-700 dark:text-amber-200">
                              <svg
                                class="w-4 h-4 flex-shrink-0 mt-0.5"
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                              >
                                <path
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                  stroke-width="2"
                                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                                />
                              </svg>
                              <div>
                                <p class="font-medium">Environment override detected</p>
                                <p class="mt-1">
                                  The <code class="font-mono">ENABLE_BACKUP_POLLING</code> or{' '}
                                  <code class="font-mono">BACKUP_POLLING_INTERVAL</code> environment
                                  variables are set. Remove them and restart Pulse to manage backup
                                  polling here.
                                </p>
                              </div>
                            </div>
                          </Show>
                        </div>
                      </section>

                      <SectionHeader
                        title="Backup & restore"
                        description="Backup your node configurations and credentials or restore from a previous backup."
                        size="md"
                        class="mb-4"
                      />

                      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                        {/* Export Section */}
                        <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                          <div class="flex items-start gap-3">
                            <div class="flex-shrink-0 w-10 h-10 bg-blue-100 dark:bg-blue-900/30 rounded-lg flex items-center justify-center">
                              <svg
                                class="w-5 h-5 text-blue-600 dark:text-blue-400"
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                              >
                                <path
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                  stroke-width="2"
                                  d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10"
                                />
                              </svg>
                            </div>
                            <div class="flex-1">
                              <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-1">
                                Export Configuration
                              </h4>
                              <p class="text-xs text-gray-600 dark:text-gray-400 mb-3">
                                Download an encrypted backup of all nodes and settings
                              </p>
                              <button
                                type="button"
                                onClick={() => {
                                  // Default to custom passphrase if no auth is configured
                                  setUseCustomPassphrase(!securityStatus()?.hasAuthentication);
                                  setShowExportDialog(true);
                                }}
                                class="px-3 py-1.5 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 transition-colors inline-flex items-center gap-2"
                              >
                                <svg
                                  class="w-4 h-4"
                                  fill="none"
                                  stroke="currentColor"
                                  viewBox="0 0 24 24"
                                >
                                  <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
                                  />
                                </svg>
                                Export Backup
                              </button>
                            </div>
                          </div>
                        </div>

                        {/* Import Section */}
                        <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                          <div class="flex items-start gap-3">
                            <div class="flex-shrink-0 w-10 h-10 bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center">
                              <svg
                                class="w-5 h-5 text-gray-600 dark:text-gray-400"
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                              >
                                <path
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                  stroke-width="2"
                                  d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
                                />
                              </svg>
                            </div>
                            <div class="flex-1">
                              <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-1">
                                Restore Configuration
                              </h4>
                              <p class="text-xs text-gray-600 dark:text-gray-400 mb-3">
                                Upload a backup file to restore nodes and settings
                              </p>
                              <button
                                type="button"
                                onClick={() => setShowImportDialog(true)}
                                class="px-3 py-1.5 bg-gray-600 text-white text-sm rounded-md hover:bg-gray-700 transition-colors inline-flex items-center gap-2"
                              >
                                <svg
                                  class="w-4 h-4"
                                  fill="none"
                                  stroke="currentColor"
                                  viewBox="0 0 24 24"
                                >
                                  <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
                                  />
                                </svg>
                                Restore Backup
                              </button>
                            </div>
                          </div>
                        </div>
                      </div>

                      <div class="mt-4 p-3 bg-amber-50 dark:bg-amber-900/20 rounded-lg border border-amber-200 dark:border-amber-800">
                        <div class="flex gap-2">
                          <svg
                            class="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                            />
                          </svg>
                          <div class="text-xs text-amber-700 dark:text-amber-300">
                            <p class="font-medium mb-1">Important Notes</p>
                            <ul class="space-y-0.5 text-amber-600 dark:text-amber-400">
                              <li>• Backups contain encrypted credentials and sensitive data</li>
                              <li>• Use a strong passphrase to protect your backup</li>
                              <li>• Store backup files securely and never share the passphrase</li>
                            </ul>
                          </div>
                        </div>
                      </div>
                    </div>
                  </Card>
                </div>
              </Show>

              {/* AI Assistant Tab */}
              <Show when={activeTab() === 'system-ai'}>
                <div class="space-y-6">
                  <AISettings />
                  <AICostDashboard />
                </div>
              </Show>

              {/* API Access */}
              <Show when={activeTab() === 'api'}>
                <div class="space-y-6">
                  <Card
                    padding="none"
                    class="overflow-hidden border border-gray-200 dark:border-gray-700"
                    border={false}
                  >
                    <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                      <div class="flex items-center gap-3">
                        <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
                          <BadgeCheck class="w-5 h-5 text-blue-600 dark:text-blue-300" />
                        </div>
                        <SectionHeader
                          title="API Access"
                          description="Generate scoped tokens for agents and automation"
                          size="sm"
                          class="flex-1"
                        />
                      </div>
                    </div>
                    <div class="p-6 space-y-3">
                      <p class="text-sm text-gray-600 dark:text-gray-400">
                        Generate scoped tokens for Docker agents, host agents, and automation
                        pipelines. Tokens are shown once—store them securely and rotate when
                        infrastructure changes.
                      </p>
                      <a
                        href="https://github.com/rcourtman/Pulse/blob/main/docs/CONFIGURATION.md#token-scopes"
                        target="_blank"
                        rel="noreferrer"
                        class="inline-flex w-fit items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-1.5 text-xs font-semibold text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-200"
                      >
                        View scope reference
                      </a>
                    </div>
                  </Card>

                  <APITokenManager
                    currentTokenHint={securityStatus()?.apiTokenHint}
                    onTokensChanged={() => {
                      void loadSecurityStatus();
                    }}
                    refreshing={securityStatusLoading()}
                  />
                </div>
              </Show>

              {/* Security Overview Tab */}
              <Show when={activeTab() === 'security-overview'}>
                <div class="space-y-6">
                  <Show when={!securityStatusLoading() && securityStatus()}>
                    <SecurityPostureSummary status={securityStatus()!} />
                  </Show>

                  <Show when={!securityStatusLoading() && securityStatus()?.hasProxyAuth}>
                    <Card
                      padding="sm"
                      class="border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/20"
                    >
                      <div class="flex flex-col gap-2 text-xs text-blue-800 dark:text-blue-200">
                        <div class="flex items-center gap-2">
                          <svg
                            class="w-4 h-4 flex-shrink-0"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                            />
                          </svg>
                          <span class="font-semibold text-blue-900 dark:text-blue-100">
                            Proxy authentication detected
                          </span>
                        </div>
                        <p>
                          Requests are validated by an upstream proxy. The current proxied user is
                          {securityStatus()?.proxyAuthUsername
                            ? ` ${securityStatus()?.proxyAuthUsername}`
                            : ' available once a request is received'}
                          .
                          {securityStatus()?.proxyAuthIsAdmin ? ' Admin privileges confirmed.' : ''}
                          <Show when={securityStatus()?.proxyAuthLogoutURL}>
                            {' '}
                            <a
                              class="underline font-medium"
                              href={securityStatus()?.proxyAuthLogoutURL}
                            >
                              Proxy logout
                            </a>
                          </Show>
                        </p>
                        <p>
                          Need configuration tips? Review the proxy auth guide in the docs.{' '}
                          <a
                            class="underline font-medium"
                            href="https://github.com/rcourtman/Pulse/blob/main/docs/PROXY_AUTH.md"
                            target="_blank"
                            rel="noreferrer"
                          >
                            Read proxy auth guide →
                          </a>
                        </p>
                      </div>
                    </Card>
                  </Show>
                </div>
              </Show>

              {/* Security Authentication Tab */}
              <Show when={activeTab() === 'security-auth'}>
                <div class="space-y-6">
                  {/* Show message when auth is disabled */}
                  <Show when={!securityStatus()?.hasAuthentication}>
                    <Card
                      padding="none"
                      class="overflow-hidden border border-amber-200 dark:border-amber-800"
                      border={false}
                    >
                      {/* Header */}
                      <div class="bg-gradient-to-r from-amber-50 to-amber-50 dark:from-amber-900/20 dark:to-amber-900/20 px-6 py-4 border-b border-amber-200 dark:border-amber-700">
                        <div class="flex items-center gap-3">
                          <div class="p-2 bg-amber-100 dark:bg-amber-900/50 rounded-lg">
                            <svg
                              class="w-5 h-5 text-amber-600 dark:text-amber-400"
                              fill="none"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                            >
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                              />
                            </svg>
                          </div>
                          <SectionHeader title="Authentication disabled" size="sm" class="flex-1" />
                          <Show
                            when={!authDisabledByEnv()}
                            fallback={
                              <span class="px-3 py-1.5 text-xs font-semibold rounded-lg border border-amber-300 text-amber-800 bg-amber-100/60 dark:border-amber-700 dark:text-amber-100 dark:bg-amber-900/40 whitespace-nowrap">
                                Controlled by DISABLE_AUTH
                              </span>
                            }
                          >
                            <button
                              type="button"
                              onClick={() => setShowQuickSecuritySetup(!showQuickSecuritySetup())}
                              class="px-3 py-1.5 text-xs font-medium rounded-lg border border-amber-300 text-amber-800 bg-amber-100/50 hover:bg-amber-100 transition-colors dark:border-amber-700 dark:text-amber-200 dark:bg-amber-900/30 dark:hover:bg-amber-900/40 whitespace-nowrap"
                            >
                              Setup
                            </button>
                          </Show>
                        </div>
                      </div>

                      {/* Content */}
                      <div class="p-6">
                        <p class="text-sm text-amber-700 dark:text-amber-300 mb-4">
                          <Show
                            when={authDisabledByEnv()}
                            fallback={
                              <>
                                Authentication is currently disabled. Set up password authentication
                                to protect your Pulse instance.
                              </>
                            }
                          >
                            Authentication settings are locked by the legacy{' '}
                            <code class="font-mono text-xs text-amber-800 dark:text-amber-200">
                              DISABLE_AUTH
                            </code>{' '}
                            environment variable. Remove it from your deployment and restart Pulse
                            before enabling security from this page.
                          </Show>
                        </p>

                        <Show when={showQuickSecuritySetup() && !authDisabledByEnv()}>
                          <QuickSecuritySetup
                            onConfigured={() => {
                              setShowQuickSecuritySetup(false);
                              loadSecurityStatus();
                            }}
                          />
                        </Show>
                      </div>
                    </Card>
                  </Show>

                  {/* Authentication */}
                  <Show
                    when={
                      !securityStatusLoading() &&
                      (securityStatus()?.hasAuthentication || securityStatus()?.apiTokenConfigured)
                    }
                  >
                    <Card
                      padding="none"
                      class="overflow-hidden border border-gray-200 dark:border-gray-700"
                      border={false}
                    >
                      {/* Header */}
                      <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                        <div class="flex items-center gap-3">
                          <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
                            <Lock
                              class="w-5 h-5 text-blue-600 dark:text-blue-300"
                              strokeWidth={2}
                            />
                          </div>
                          <SectionHeader
                            title="Authentication"
                            description="Password management and credential rotation"
                            size="sm"
                            class="flex-1"
                          />
                        </div>
                      </div>

                      {/* Content */}
                      <div class="p-6">
                        <div class="flex flex-wrap items-center gap-3">
                          <button
                            type="button"
                            onClick={(e) => {
                              e.preventDefault();
                              e.stopPropagation();
                              setShowPasswordModal(true);
                            }}
                            class="px-4 py-2 text-sm font-medium bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                          >
                            Change password
                          </button>
                          <Show
                            when={!authDisabledByEnv()}
                            fallback={
                              <span class="px-4 py-2 text-sm font-semibold border border-amber-300 text-amber-800 bg-amber-50 dark:border-amber-700 dark:text-amber-200 dark:bg-amber-900/30 rounded-lg">
                                Remove DISABLE_AUTH to rotate credentials
                              </span>
                            }
                          >
                            <button
                              type="button"
                              onClick={() => setShowQuickSecurityWizard(!showQuickSecurityWizard())}
                              class="px-4 py-2 text-sm font-medium border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
                            >
                              Rotate credentials
                            </button>
                          </Show>
                          <div class="flex-1"></div>
                          <div class="text-xs text-gray-600 dark:text-gray-400">
                            <span class="font-medium text-gray-800 dark:text-gray-200">User:</span>{' '}
                            {securityStatus()?.authUsername || 'Not configured'}
                          </div>
                        </div>

                        <div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                          <Toggle
                            label="Hide local login form"
                            description="Hide the username/password form on the login page. Users will only see SSO options unless ?show_local=true is used."
                            checked={hideLocalLogin()}
                            onChange={(e: ToggleChangeEvent) => handleHideLocalLoginChange(e.currentTarget.checked)}
                            disabled={hideLocalLoginLocked() || savingHideLocalLogin()}
                            locked={hideLocalLoginLocked()}
                            lockedMessage="This setting is managed by the PULSE_AUTH_HIDE_LOCAL_LOGIN environment variable"
                          />
                        </div>

                        <Show when={!authDisabledByEnv() && showQuickSecurityWizard()}>
                          <div class="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                            <QuickSecuritySetup
                              mode="rotate"
                              defaultUsername={securityStatus()?.authUsername || 'admin'}
                              onConfigured={() => {
                                setShowQuickSecurityWizard(false);
                                loadSecurityStatus();
                              }}
                            />
                          </div>
                        </Show>
                      </div>
                    </Card>
                  </Show>

                  {/* Show pending restart message if configured but not loaded */}
                  <Show when={securityStatus()?.configuredButPendingRestart}>
                    <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-4">
                      <div class="flex items-start space-x-3">
                        <div class="flex-shrink-0">
                          <svg
                            class="h-6 w-6 text-amber-600 dark:text-amber-400"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                            />
                          </svg>
                        </div>
                        <div class="flex-1">
                          <h4 class="text-sm font-semibold text-amber-900 dark:text-amber-100">
                            Security Configured - Restart Required
                          </h4>
                          <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                            Security settings have been configured but the service needs to be
                            restarted to activate them.
                          </p>
                          <p class="text-xs text-amber-600 dark:text-amber-400 mt-2">
                            After restarting, you'll need to log in with your saved credentials.
                          </p>

                          <div class="mt-4 bg-white dark:bg-gray-800 rounded-lg p-3 border border-amber-200 dark:border-amber-700">
                            <p class="text-xs font-semibold text-gray-900 dark:text-gray-100 mb-2">
                              How to restart Pulse:
                            </p>

                            <Show when={versionInfo()?.deploymentType === 'proxmoxve'}>
                              <div class="space-y-2">
                                <p class="text-xs text-gray-700 dark:text-gray-300">
                                  Type{' '}
                                  <code class="px-1 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">
                                    update
                                  </code>{' '}
                                  in your ProxmoxVE console
                                </p>
                                <p class="text-xs text-gray-600 dark:text-gray-400 italic">
                                  Or restart manually with:{' '}
                                  <code class="text-xs">systemctl restart pulse</code>
                                </p>
                              </div>
                            </Show>

                            <Show when={versionInfo()?.deploymentType === 'docker'}>
                              <div class="space-y-1">
                                <p class="text-xs text-gray-700 dark:text-gray-300">
                                  Restart your Docker container:
                                </p>
                                <code class="block text-xs bg-gray-100 dark:bg-gray-700 p-2 rounded mt-1">
                                  docker restart pulse
                                </code>
                              </div>
                            </Show>

                            <Show
                              when={
                                versionInfo()?.deploymentType === 'systemd' ||
                                versionInfo()?.deploymentType === 'manual'
                              }
                            >
                              <div class="space-y-1">
                                <p class="text-xs text-gray-700 dark:text-gray-300">
                                  Restart the service:
                                </p>
                                <code class="block text-xs bg-gray-100 dark:bg-gray-700 p-2 rounded mt-1">
                                  sudo systemctl restart pulse
                                </code>
                              </div>
                            </Show>

                            <Show when={versionInfo()?.deploymentType === 'development'}>
                              <div class="space-y-1">
                                <p class="text-xs text-gray-700 dark:text-gray-300">
                                  Restart the development server:
                                </p>
                                <code class="block text-xs bg-gray-100 dark:bg-gray-700 p-2 rounded mt-1">
                                  sudo systemctl restart pulse-hot-dev
                                </code>
                              </div>
                            </Show>

                            <Show when={!versionInfo()?.deploymentType}>
                              <div class="space-y-1">
                                <p class="text-xs text-gray-700 dark:text-gray-300">
                                  Restart Pulse using your deployment method
                                </p>
                              </div>
                            </Show>
                          </div>

                          <div class="mt-3 p-2 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded">
                            <p class="text-xs text-green-700 dark:text-green-300">
                              <strong>Tip:</strong> Make sure you've saved your credentials
                              before restarting!
                            </p>
                          </div>
                        </div>
                      </div>
                    </div>
                  </Show>
                </div>
              </Show>

              {/* Security Single Sign-On Tab */}
              <Show when={activeTab() === 'security-sso'}>
                <div class="space-y-6">
                  <OIDCPanel onConfigUpdated={loadSecurityStatus} />
                </div>
              </Show>

              {/* Diagnostics Tab */}
              <Show when={activeTab() === 'diagnostics'}>
                <DiagnosticsPanel />
              </Show>
            </div>
          </div>
        </Card >
      </div >

      {/* Delete Node Modal */}
      < Show when={showDeleteNodeModal()} >
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <Card padding="lg" class="w-full max-w-lg space-y-5">
            <SectionHeader title={`Remove ${nodePendingDeleteLabel()}`} size="md" class="mb-1" />
            <div class="space-y-3 text-sm text-gray-600 dark:text-gray-300">
              <p>
                Removing this {nodePendingDeleteTypeLabel().toLowerCase()} also scrubs the Pulse
                footprint on the host — the proxy service, SSH key, API token, and bind mount are
                all cleaned up automatically.
              </p>
              <div class="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm leading-relaxed dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-100">
                <p class="font-medium text-blue-900 dark:text-blue-100">What happens next</p>
                <ul class="mt-2 list-disc space-y-1 pl-4 text-blue-800 dark:text-blue-200 text-sm">
                  <li>Pulse removes the node entry and clears related alerts.</li>
                  <li>
                    {nodePendingDeleteHost() ? (
                      <>
                        The host <span class="font-semibold">{nodePendingDeleteHost()}</span> loses
                        the proxy service, SSH key, and API token.
                      </>
                    ) : (
                      'The host loses the proxy service, SSH key, and API token.'
                    )}
                  </li>
                  <li>
                    If the host comes back later, rerunning the setup script reinstalls everything
                    with a fresh key.
                  </li>
                  <Show when={nodePendingDeleteType() === 'pbs'}>
                    <li>
                      Backup user tokens on the PBS are removed, so jobs referencing them will no
                      longer authenticate until the node is re-added.
                    </li>
                  </Show>
                  <Show when={nodePendingDeleteType() === 'pmg'}>
                    <li>
                      Mail gateway tokens are removed as part of the cleanup; re-enroll to restore
                      outbound telemetry.
                    </li>
                  </Show>
                </ul>
              </div>
            </div>

            <div class="flex items-center justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={cancelDeleteNode}
                class="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
                disabled={deleteNodeLoading()}
              >
                Keep node
              </button>
              <button
                type="button"
                onClick={deleteNode}
                disabled={deleteNodeLoading()}
                class="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-red-500 dark:hover:bg-red-400"
              >
                {deleteNodeLoading() ? 'Removing…' : 'Remove node'}
              </button>
            </div>
          </Card>
        </div>
      </Show >

      {/* Node Modal - Use separate modals for PVE and PBS to ensure clean state */}
      < Show when={isNodeModalVisible('pve')} >
        <NodeModal
          isOpen={true}
          resetKey={modalResetKey()}
          onClose={() => {
            setShowNodeModal(false);
            setEditingNode(null);
            // Increment resetKey to force form reset on next open
            setModalResetKey((prev) => prev + 1);
          }}
          nodeType="pve"
          editingNode={editingNode()?.type === 'pve' ? (editingNode() ?? undefined) : undefined}
          securityStatus={securityStatus() ?? undefined}
          temperatureMonitoringEnabled={resolveTemperatureMonitoringEnabled(
            editingNode()?.type === 'pve' ? editingNode() : null,
          )}
          temperatureMonitoringLocked={temperatureMonitoringLocked()}
          savingTemperatureSetting={savingTemperatureSetting()}
          onToggleTemperatureMonitoring={
            editingNode()?.id
              ? (enabled: boolean) => handleNodeTemperatureMonitoringChange(editingNode()!.id, enabled)
              : handleTemperatureMonitoringChange
          }
          onSave={async (nodeData) => {
            try {
              if (editingNode() && editingNode()!.id) {
                // Update existing node (only if it has a valid ID)
                await NodesAPI.updateNode(editingNode()!.id, nodeData as NodeConfig);

                // Update local state
                setNodes(
                  nodes().map((n) =>
                    n.id === editingNode()!.id
                      ? {
                        ...n,
                        ...nodeData,
                        // Update hasPassword/hasToken based on whether credentials were provided
                        hasPassword: nodeData.password ? true : n.hasPassword,
                        hasToken: nodeData.tokenValue ? true : n.hasToken,
                        status: 'pending',
                      }
                      : n,
                  ),
                );
                showSuccess('Node updated successfully');
              } else {
                // Add new node
                await NodesAPI.addNode(nodeData as NodeConfig);

                // Reload nodes to get the new ID
                const nodesList = await NodesAPI.getNodes();
                const nodesWithStatus = nodesList.map((node) => ({
                  ...node,
                  // Use the hasPassword/hasToken from the API if available, otherwise check local fields
                  hasPassword: node.hasPassword ?? !!node.password,
                  hasToken: node.hasToken ?? !!node.tokenValue,
                  status: node.status || ('pending' as const),
                }));
                setNodes(nodesWithStatus);
                showSuccess('Node added successfully');
              }

              setShowNodeModal(false);
              setEditingNode(null);
            } catch (error) {
              showError(error instanceof Error ? error.message : 'Operation failed');
            }
          }}
        />
      </Show >

      {/* PBS Node Modal - Separate instance to prevent contamination */}
      < Show when={isNodeModalVisible('pbs')} >
        <NodeModal
          isOpen={true}
          resetKey={modalResetKey()}
          onClose={() => {
            setShowNodeModal(false);
            setEditingNode(null);
            // Increment resetKey to force form reset on next open
            setModalResetKey((prev) => prev + 1);
          }}
          nodeType="pbs"
          editingNode={editingNode()?.type === 'pbs' ? (editingNode() ?? undefined) : undefined}
          securityStatus={securityStatus() ?? undefined}
          temperatureMonitoringEnabled={resolveTemperatureMonitoringEnabled(
            editingNode()?.type === 'pbs' ? editingNode() : null,
          )}
          temperatureMonitoringLocked={temperatureMonitoringLocked()}
          savingTemperatureSetting={savingTemperatureSetting()}
          onToggleTemperatureMonitoring={
            editingNode()?.id
              ? (enabled: boolean) => handleNodeTemperatureMonitoringChange(editingNode()!.id, enabled)
              : handleTemperatureMonitoringChange
          }
          onSave={async (nodeData) => {
            try {
              if (editingNode() && editingNode()!.id) {
                // Update existing node (only if it has a valid ID)
                await NodesAPI.updateNode(editingNode()!.id, nodeData as NodeConfig);

                // Update local state
                setNodes(
                  nodes().map((n) =>
                    n.id === editingNode()!.id
                      ? {
                        ...n,
                        ...nodeData,
                        hasPassword: nodeData.password ? true : n.hasPassword,
                        hasToken: nodeData.tokenValue ? true : n.hasToken,
                        status: 'pending',
                      }
                      : n,
                  ),
                );
                showSuccess('Node updated successfully');
              } else {
                // Add new node
                await NodesAPI.addNode(nodeData as NodeConfig);

                // Reload the nodes list to get the latest state
                const nodesList = await NodesAPI.getNodes();
                const nodesWithStatus = nodesList.map((node) => ({
                  ...node,
                  // Use the hasPassword/hasToken from the API if available, otherwise check local fields
                  hasPassword: node.hasPassword ?? !!node.password,
                  hasToken: node.hasToken ?? !!node.tokenValue,
                  status: node.status || ('pending' as const),
                }));
                setNodes(nodesWithStatus);
                showSuccess('Node added successfully');
              }

              setShowNodeModal(false);
              setEditingNode(null);
            } catch (error) {
              showError(error instanceof Error ? error.message : 'Operation failed');
            }
          }}
        />
      </Show >

      {/* PMG Node Modal */}
      < Show when={isNodeModalVisible('pmg')} >
        <NodeModal
          isOpen={true}
          resetKey={modalResetKey()}
          onClose={() => {
            setShowNodeModal(false);
            setEditingNode(null);
            setModalResetKey((prev) => prev + 1);
          }}
          nodeType="pmg"
          editingNode={editingNode()?.type === 'pmg' ? (editingNode() ?? undefined) : undefined}
          securityStatus={securityStatus() ?? undefined}
          temperatureMonitoringEnabled={resolveTemperatureMonitoringEnabled(
            editingNode()?.type === 'pmg' ? editingNode() : null,
          )}
          temperatureMonitoringLocked={temperatureMonitoringLocked()}
          savingTemperatureSetting={savingTemperatureSetting()}
          onToggleTemperatureMonitoring={
            editingNode()?.id
              ? (enabled: boolean) => handleNodeTemperatureMonitoringChange(editingNode()!.id, enabled)
              : handleTemperatureMonitoringChange
          }
          onSave={async (nodeData) => {
            try {
              if (editingNode() && editingNode()!.id) {
                await NodesAPI.updateNode(editingNode()!.id, nodeData as NodeConfig);
                setNodes(
                  nodes().map((n) =>
                    n.id === editingNode()!.id
                      ? {
                        ...n,
                        ...nodeData,
                        hasPassword: nodeData.password ? true : n.hasPassword,
                        hasToken: nodeData.tokenValue ? true : n.hasToken,
                        status: 'pending',
                      }
                      : n,
                  ),
                );
                showSuccess('Node updated successfully');
              } else {
                await NodesAPI.addNode(nodeData as NodeConfig);
                const nodesList = await NodesAPI.getNodes();
                const nodesWithStatus = nodesList.map((node) => ({
                  ...node,
                  hasPassword: node.hasPassword ?? !!node.password,
                  hasToken: node.hasToken ?? !!node.tokenValue,
                  status: node.status || ('pending' as const),
                }));
                setNodes(nodesWithStatus);
                showSuccess('Node added successfully');
              }

              setShowNodeModal(false);
              setEditingNode(null);
            } catch (error) {
              showError(error instanceof Error ? error.message : 'Operation failed');
            }
          }}
        />
      </Show >
      {/* Export Dialog */}
      < Show when={showExportDialog()} >
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="Export configuration" size="md" class="mb-4" />

            <div class="space-y-4">
              {/* Password Choice Section - Only show if auth is enabled */}
              <Show when={securityStatus()?.hasAuthentication}>
                <div class="bg-gray-50 dark:bg-gray-900/50 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
                  <div class="space-y-3">
                    <label class="flex items-start gap-3 cursor-pointer">
                      <input
                        type="radio"
                        checked={!useCustomPassphrase()}
                        onChange={() => {
                          setUseCustomPassphrase(false);
                          setExportPassphrase('');
                        }}
                        class="mt-1 text-blue-600 focus:ring-blue-500"
                      />
                      <div class="flex-1">
                        <div class="text-sm font-medium text-gray-700 dark:text-gray-300">
                          Use your login password
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                          Use the same password you use to log into Pulse (recommended)
                        </div>
                      </div>
                    </label>

                    <label class="flex items-start gap-3 cursor-pointer">
                      <input
                        type="radio"
                        checked={useCustomPassphrase()}
                        onChange={() => setUseCustomPassphrase(true)}
                        class="mt-1 text-blue-600 focus:ring-blue-500"
                      />
                      <div class="flex-1">
                        <div class="text-sm font-medium text-gray-700 dark:text-gray-300">
                          Use a custom passphrase
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                          Create a different passphrase for this backup
                        </div>
                      </div>
                    </label>
                  </div>
                </div>
              </Show>

              {/* Show password input based on selection */}
              <div class={formField}>
                <label class={labelClass()}>
                  {securityStatus()?.hasAuthentication
                    ? useCustomPassphrase()
                      ? 'Custom Passphrase'
                      : 'Enter Your Login Password'
                    : 'Encryption Passphrase'}
                </label>
                <input
                  type="password"
                  value={exportPassphrase()}
                  onInput={(e) => setExportPassphrase(e.currentTarget.value)}
                  placeholder={
                    securityStatus()?.hasAuthentication
                      ? useCustomPassphrase()
                        ? 'Enter a strong passphrase'
                        : 'Enter your Pulse login password'
                      : 'Enter a strong passphrase for encryption'
                  }
                  class={controlClass()}
                />
                <Show when={!securityStatus()?.hasAuthentication || useCustomPassphrase()}>
                  <p class={`${formHelpText} mt-1`}>
                    You'll need this passphrase to restore the backup.
                  </p>
                </Show>
                <Show when={securityStatus()?.hasAuthentication && !useCustomPassphrase()}>
                  <p class={`${formHelpText} mt-1`}>
                    You'll use this same password when restoring the backup
                  </p>
                </Show>
              </div>

              <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
                <div class="flex gap-2">
                  <svg
                    class="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                    />
                  </svg>
                  <div class="text-xs text-amber-700 dark:text-amber-300">
                    <strong>Important:</strong> The backup contains node credentials but NOT
                    authentication settings. Each Pulse instance should configure its own login
                    credentials for security. Remember your{' '}
                    {useCustomPassphrase() || !securityStatus()?.hasAuthentication
                      ? 'passphrase'
                      : 'password'}{' '}
                    for restoring.
                  </div>
                </div>
              </div>

              <div class="flex justify-end space-x-3">
                <button
                  type="button"
                  onClick={() => {
                    setShowExportDialog(false);
                    setExportPassphrase('');
                    setUseCustomPassphrase(false);
                  }}
                  class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleExport}
                  disabled={
                    !exportPassphrase() || (useCustomPassphrase() && exportPassphrase().length < 12)
                  }
                  class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Export
                </button>
              </div>
            </div>
          </Card>
        </div>
      </Show >

      {/* API Token Modal */}
      < Show when={showApiTokenModal()} >
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="API token required" size="md" class="mb-4" />

            <div class="space-y-4">
              <p class="text-sm text-gray-600 dark:text-gray-400">
                This Pulse instance requires an API token for export/import operations. Please enter
                the API token configured on the server.
              </p>

              <div class={formField}>
                <label class={labelClass()}>API Token</label>
                <input
                  type="password"
                  value={apiTokenInput()}
                  onInput={(e) => setApiTokenInput(e.currentTarget.value)}
                  placeholder="Enter API token"
                  class={controlClass()}
                />
              </div>

              <div class="text-xs text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 rounded p-2">
                <p class="font-semibold mb-1">The API token is set as an environment variable:</p>
                <code class="block">API_TOKENS=token-for-export,token-for-automation</code>
              </div>
            </div>

            <div class="flex justify-end space-x-2 mt-6">
              <button
                type="button"
                onClick={() => {
                  setShowApiTokenModal(false);
                  setApiTokenInput('');
                }}
                class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => {
                  if (apiTokenInput()) {
                    const tokenValue = apiTokenInput()!;
                    setApiClientToken(tokenValue);
                    const source = apiTokenModalSource();
                    setShowApiTokenModal(false);
                    setApiTokenInput('');
                    setApiTokenModalSource(null);
                    // Retry the operation that triggered the modal
                    if (source === 'export') {
                      handleExport();
                    } else if (source === 'import') {
                      handleImport();
                    }
                  } else {
                    showError('Please enter the API token');
                  }
                }}
                disabled={!apiTokenInput()}
                class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Authenticate
              </button>
            </div>
          </Card>
        </div>
      </Show >

      {/* Import Dialog */}
      < Show when={showImportDialog()} >
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="Import configuration" size="md" class="mb-4" />

            <div class="space-y-4">
              <div class={formField}>
                <label class={labelClass()}>Configuration File</label>
                <input
                  type="file"
                  accept=".json"
                  onChange={(e) => {
                    const file = e.currentTarget.files?.[0];
                    if (file) setImportFile(file);
                  }}
                  class={controlClass('cursor-pointer')}
                />
              </div>

              <div class={formField}>
                <label class={labelClass()}>Backup Password</label>
                <input
                  type="password"
                  value={importPassphrase()}
                  onInput={(e) => setImportPassphrase(e.currentTarget.value)}
                  placeholder="Enter the password used when creating this backup"
                  class={controlClass()}
                />
                <p class={`${formHelpText} mt-1`}>
                  This is usually your Pulse login password, unless you used a custom passphrase
                </p>
              </div>

              <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded p-3">
                <p class="text-xs text-yellow-700 dark:text-yellow-300">
                  <strong>Warning:</strong> Importing will replace all current configuration. This
                  action cannot be undone.
                </p>
              </div>

              <div class="flex justify-end space-x-3">
                <button
                  type="button"
                  onClick={() => {
                    setShowImportDialog(false);
                    setImportPassphrase('');
                    setImportFile(null);
                  }}
                  class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleImport}
                  disabled={!importPassphrase() || !importFile()}
                  class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Import
                </button>
              </div>
            </div>
          </Card>
        </div>
      </Show >

      <ChangePasswordModal
        isOpen={showPasswordModal()}
        onClose={() => {
          setShowPasswordModal(false);
          // Refresh security status after password change
          loadSecurityStatus();
        }}
      />
    </>
  );
};

export default Settings;
