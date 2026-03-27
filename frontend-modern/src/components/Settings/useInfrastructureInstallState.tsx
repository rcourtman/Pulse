import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { MonitoringAPI } from '@/api/monitoring';
import { SecurityAPI, type APITokenRecord } from '@/api/security';
import { notificationStore } from '@/stores/notifications';
import type { AgentLookupResponse, ConnectedInfrastructureItem } from '@/types/api';
import type { SecurityStatus } from '@/types/config';
import {
  AGENT_CONFIG_READ_SCOPE,
  AGENT_EXEC_SCOPE,
  AGENT_REPORT_SCOPE,
  DOCKER_REPORT_SCOPE,
  KUBERNETES_REPORT_SCOPE,
} from '@/constants/apiScopes';
import { copyToClipboard } from '@/utils/clipboard';
import { getPulseBaseUrl } from '@/utils/url';
import { logger } from '@/utils/logger';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { buildInfrastructureWorkspacePath } from './infrastructureWorkspaceModel';
import {
  buildUnixAgentInstallCommand,
  buildWindowsAgentInstallCommand,
  resolveInstallerBaseUrl,
} from '@/utils/agentInstallCommand';
import { getUnifiedAgentClipboardCopyErrorMessage } from '@/utils/unifiedAgentInventoryPresentation';
import {
  trackAgentInstallProfileSelected,
  trackAgentInstallTokenGenerated,
} from '@/utils/upgradeMetrics';
import {
  buildCommandsByPlatform,
  buildDefaultTokenName,
  getPowerShellInstallProfileEnvFromFlags,
  INSTALL_PROFILE_OPTIONS,
  TOKEN_PLACEHOLDER,
  type AgentPlatform,
  type InstallProfile,
  type SetupHandoffState,
  UNIFIED_AGENT_TELEMETRY_SURFACE,
} from './infrastructureOperationsModel';

export interface InfrastructureInstallStateOptions {
  embedded?: boolean;
}

const FIRST_HOST_AUTODETECT_POLL_MS = 3000;

const isActiveInfrastructureItem = (item: ConnectedInfrastructureItem) => item.status === 'active';

const pickMostRecentInfrastructureItem = (
  items: ConnectedInfrastructureItem[],
): ConnectedInfrastructureItem | null => {
  if (items.length === 0) {
    return null;
  }

  return [...items].sort((left, right) => (right.lastSeen || 0) - (left.lastSeen || 0))[0];
};

const toLookupResponseFromInfrastructureItem = (
  item: ConnectedInfrastructureItem,
): AgentLookupResponse => ({
  success: true,
  agent: {
    id: item.scopeAgentId?.trim() || item.uninstallAgentId?.trim() || item.id,
    hostname: item.hostname?.trim() || item.displayName?.trim() || item.name.trim() || item.id,
    displayName: item.displayName?.trim() || item.name.trim() || undefined,
    status: item.healthStatus?.trim() || item.status,
    connected: true,
    lastSeen: item.lastSeen || Date.now(),
    agentVersion: item.version?.trim() || undefined,
  },
});

export const useInfrastructureInstallState = (
  options: InfrastructureInstallStateOptions = {},
) => {
  const navigate = useNavigate();

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
  const [autoLookupActive, setAutoLookupActive] = createSignal(false);
  const [lookupWasAutoDetected, setLookupWasAutoDetected] = createSignal(false);
  const [insecureMode, setInsecureMode] = createSignal(false);
  const [enableCommands, setEnableCommands] = createSignal(false);
  const [installProfile, setInstallProfile] = createSignal<InstallProfile>('auto');
  const [customAgentUrl, setCustomAgentUrl] = createSignal('');
  const [customCaPath, setCustomCaPath] = createSignal('');
  const [setupHandoff, setSetupHandoff] = createSignal<SetupHandoffState | null>(null);

  createEffect(() => {
    if (requiresToken()) {
      setConfirmedNoToken(false);
    }
  });

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

    void fetchSecurityStatus();
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

  createEffect(() => {
    const shouldAutoDetectFirstHost =
      commandsUnlocked() && !lookupValue().trim() && !lookupResult()?.agent?.connected;

    if (!shouldAutoDetectFirstHost) {
      setAutoLookupActive(false);
      return;
    }

    let baselineActiveCount: number | null = null;
    let pollInterval: number | undefined;
    let cancelled = false;

    const pollForFirstHost = async () => {
      try {
        const state = await MonitoringAPI.getState();
        if (cancelled) {
          return;
        }

        const activeItems = (state.connectedInfrastructure || []).filter(isActiveInfrastructureItem);
        if (baselineActiveCount === null) {
          baselineActiveCount = activeItems.length;
        }

        const shouldKeepWatching = baselineActiveCount === 0 && activeItems.length === 0;
        setAutoLookupActive(shouldKeepWatching);

        if (baselineActiveCount !== 0 || activeItems.length === 0) {
          return;
        }

        const detectedItem = pickMostRecentInfrastructureItem(activeItems);
        if (!detectedItem) {
          return;
        }

        const detectedLookup = toLookupResponseFromInfrastructureItem(detectedItem);
        setLookupResult(detectedLookup);
        setLookupError(null);
        setLookupWasAutoDetected(true);
        setLookupValue(detectedLookup.agent?.hostname || detectedLookup.agent?.id || '');
        setAutoLookupActive(false);

        if (pollInterval) {
          window.clearInterval(pollInterval);
          pollInterval = undefined;
        }
      } catch (err) {
        if (!cancelled) {
          logger.warn('Failed to auto-detect first connected host', err);
        }
      }
    };

    void pollForFirstHost();
    pollInterval = window.setInterval(pollForFirstHost, FIRST_HOST_AUTODETECT_POLL_MS);

    onCleanup(() => {
      cancelled = true;
      setAutoLookupActive(false);
      if (pollInterval) {
        window.clearInterval(pollInterval);
      }
    });
  });

  const handleLookup = async () => {
    const query = lookupValue().trim();
    setLookupError(null);
    setLookupWasAutoDetected(false);

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

  const clearLookupState = () => {
    setLookupError(null);
    setLookupResult(null);
    setLookupWasAutoDetected(false);
  };

  const selectedCustomCaPath = () => customCaPath().trim();
  const getSelectedInstallProfile = () =>
    INSTALL_PROFILE_OPTIONS.find((option) => option.value === installProfile()) ??
    INSTALL_PROFILE_OPTIONS[0];
  const getInstallProfileFlags = () => getSelectedInstallProfile().flags;
  const getPowerShellInstallEnv = () => {
    const envAssignments = getPowerShellInstallProfileEnvFromFlags(getInstallProfileFlags());
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

  const openDirectProxmoxSetup = () => {
    navigate('/settings/infrastructure/proxmox');
  };

  const openDashboard = () => {
    navigate('/dashboard');
  };

  const openInfrastructureInventory = () => {
    navigate(buildInfrastructureWorkspacePath('inventory'));
  };

  const isEmbedded = () => options.embedded ?? false;

  return {
    acknowledgeNoToken,
    agentUrl,
    autoLookupActive,
    clearLookupState,
    clearSetupHandoff,
    commandSections,
    commandsUnlocked,
    confirmedNoToken,
    copySetupHandoffField,
    currentToken,
    customAgentUrl,
    customCaPath,
    downloadSetupHandoff,
    enableCommands,
    getInstallProfileFlags,
    getSelectedInstallProfile,
    handleGenerateToken,
    handleInstallProfileChange,
    handleLookup,
    hasToken,
    insecureMode,
    installProfile,
    isEmbedded,
    isGeneratingToken,
    latestRecord,
    lookupError,
    lookupLoading,
    lookupResult,
    lookupValue,
    lookupWasAutoDetected,
    openDashboard,
    openDirectProxmoxSetup,
    openInfrastructureInventory,
    requiresToken,
    selectedAgentUrl,
    setCustomAgentUrl,
    setCustomCaPath,
    setEnableCommands,
    setInsecureMode,
    setLookupValue,
    setTokenName,
    setupHandoff,
    tokenName,
  };
};

export type InfrastructureInstallState = ReturnType<typeof useInfrastructureInstallState>;
