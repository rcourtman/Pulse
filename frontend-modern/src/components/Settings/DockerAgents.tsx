import { Component, createSignal, Show, For, onMount, createEffect, createMemo } from 'solid-js';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import { MonitoringAPI } from '@/api/monitoring';
import { SecurityAPI } from '@/api/security';
import { notificationStore } from '@/stores/notifications';
import type { SecurityStatus } from '@/types/config';
import type { DockerHost } from '@/types/api';
import type { APITokenRecord } from '@/api/security';
import { DOCKER_REPORT_SCOPE } from '@/constants/apiScopes';
import { resolveHostRuntime } from '@/components/Docker/runtimeDisplay';
import { copyToClipboard } from '@/utils/clipboard';
import { getPulseBaseUrl } from '@/utils/url';
import { logger } from '@/utils/logger';


export const DockerAgents: Component = () => {
  const { state } = useWebSocket();

  let hasLoggedSecurityStatusError = false;

  const [showHidden, setShowHidden] = createSignal(false);

  const allDockerHosts = () => state.dockerHosts || [];

  const dockerHosts = createMemo(() => {
    const all = allDockerHosts();
    const includeHidden = showHidden();
    let filtered = includeHidden ? all : all.filter(host => !host.hidden);

    if (!includeHidden) {
      filtered = filtered.filter(host => {
        if (!host.pendingUninstall) {
          return true;
        }
        const status = host.command?.status;
        return status === 'failed' || status === 'expired';
      });
    }

    return filtered;
  });

  const hiddenCount = () => allDockerHosts().filter(host => host.hidden).length;

  const pendingHosts = createMemo(() =>
    allDockerHosts().filter(host => {
      if (host.pendingUninstall) return true;
      const status = host.command?.status;
      return status === 'queued' || status === 'dispatched' || status === 'acknowledged' || status === 'completed';
    }),
  );

  const removedHosts = () => state.removedDockerHosts ?? [];
  const hasRemovedHosts = () => removedHosts().length > 0;

  const [removingHostId, setRemovingHostId] = createSignal<string | null>(null);
  const [showRemoveModal, setShowRemoveModal] = createSignal(false);
  const [hostToRemoveId, setHostToRemoveId] = createSignal<string | null>(null);
  const [uninstallCommandCopied, setUninstallCommandCopied] = createSignal(false);
  const [removeActionLoading, setRemoveActionLoading] = createSignal<
    'queue' | 'force' | 'hide' | 'awaitingCommand' | null
  >(null);
  const [showAdvancedOptions, setShowAdvancedOptions] = createSignal(false);
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [isGeneratingToken, setIsGeneratingToken] = createSignal(false);
  const [currentToken, setCurrentToken] = createSignal<string | null>(null);
  const [latestRecord, setLatestRecord] = createSignal<APITokenRecord | null>(null);
  const [tokenName, setTokenName] = createSignal('');
  const [commandQueuedTime, setCommandQueuedTime] = createSignal<Date | null>(null);
  const [elapsedSeconds, setElapsedSeconds] = createSignal(0);
  const [editingHostId, setEditingHostId] = createSignal<string | null>(null);
  const [editingDisplayName, setEditingDisplayName] = createSignal('');
  const [savingDisplayName, setSavingDisplayName] = createSignal(false);

  const pulseUrl = () => getPulseBaseUrl();

  const TOKEN_PLACEHOLDER = '<api-token>';

  const hostToRemove = createMemo(() => {
    const id = hostToRemoveId();
    if (!id) return null;
    return (state.dockerHosts || []).find(host => host.id === id) ?? null;
  });

  const getDisplayName = (host: DockerHost | { id: string; displayName?: string | null; hostname?: string | null; customDisplayName?: string | null }) => {
    if ('customDisplayName' in host && host.customDisplayName) {
      return host.customDisplayName;
    }
    return host.displayName || host.hostname || host.id;
  };

  const describeRuntime = (host: DockerHost) => {
    const runtimeInfo = resolveHostRuntime(host);
    const version = host.runtimeVersion || host.dockerVersion || '';
    return version ? `${runtimeInfo.label} ${version}` : runtimeInfo.label;
  };

  const modalDisplayName = () => {
    const host = hostToRemove();
    return host ? getDisplayName(host) : '';
  };

  const modalHostname = () => {
    const host = hostToRemove();
    return host?.hostname || host?.id || '';
  };

  const modalHostStatus = () => {
    const host = hostToRemove();
    return host?.status || 'unknown';
  };

  const modalHostIsOnline = () => modalHostStatus().toLowerCase() === 'online';
  const modalHostHidden = () => Boolean(hostToRemove()?.hidden);
  const modalCommand = createMemo(() => hostToRemove()?.command ?? null);
  const modalCommandStatus = createMemo(() => modalCommand()?.status ?? null);
  const modalCommandInProgress = createMemo(() => {
    const status = modalCommandStatus();
    return status === 'queued' || status === 'dispatched' || status === 'acknowledged';
  });
  const modalCommandFailed = createMemo(() => modalCommandStatus() === 'failed');
  const modalCommandCompleted = createMemo(() => modalCommandStatus() === 'completed');
  const modalCommandProgress = createMemo(() => {
    const cmd = modalCommand();
    if (!cmd) return [];

    const statusOrder: Record<string, number> = {
      queued: 0,
      dispatched: 1,
      acknowledged: 2,
      completed: 3,
      failed: 4,
      expired: 5,
    };
    const currentIndex = statusOrder[cmd.status] ?? 0;
    const steps = [
      { key: 'queued', label: 'Stop command queued' },
      { key: 'dispatched', label: 'Instruction delivered to the agent' },
      { key: 'acknowledged', label: 'Agent acknowledged the stop request' },
      { key: 'completed', label: 'Agent disabled the service and removed autostart' },
    ];

    return steps.map((step) => {
      const stepIndex = statusOrder[step.key] ?? 0;
      return {
        label: step.label,
        done: currentIndex > stepIndex,
        active: currentIndex === stepIndex,
      };
    });
  });

  const modalCommandTimedOut = createMemo(() => {
    return modalCommandInProgress() && elapsedSeconds() > 120; // 2 minutes
  });

  const modalLastHeartbeat = createMemo(() => {
    const host = hostToRemove();
    return host?.lastSeen ? formatRelativeTime(host.lastSeen) : null;
  });

  const modalHostPendingUninstall = createMemo(() => Boolean(hostToRemove()?.pendingUninstall));
  const modalHasCommand = createMemo(() => Boolean(modalCommand()));
  const [hasShownCommandCompletion, setHasShownCommandCompletion] = createSignal(false);

  const formatElapsedTime = (seconds: number) => {
    if (seconds < 60) {
      return `${seconds}s`;
    }
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}m ${secs}s`;
  };

  type RemovalStatusTone = 'info' | 'success' | 'danger';

  const removalBadgeClassMap: Record<RemovalStatusTone, string> = {
    info: 'inline-flex items-center gap-1 rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-blue-700 dark:bg-blue-900/40 dark:text-blue-200',
    success:
      'inline-flex items-center gap-1 rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200',
    danger:
      'inline-flex items-center gap-1 rounded-full bg-red-100 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-red-600 dark:bg-red-900/40 dark:text-red-200',
  };

  const removalTextClassMap: Record<RemovalStatusTone, string> = {
    info: 'text-blue-700 dark:text-blue-300',
    success: 'text-emerald-700 dark:text-emerald-300',
    danger: 'text-red-600 dark:text-red-300',
  };

  const getRemovalStatusInfo = (host: DockerHost): { label: string; tone: RemovalStatusTone } | null => {
    const status = host.command?.status ?? null;

    switch (status) {
      case 'failed':
        return {
          label: host.command?.failureReason || 'Pulse could not stop the agent automatically.',
          tone: 'danger',
        };
      case 'expired':
        return {
          label: 'Stop command expired before the agent responded.',
          tone: 'danger',
        };
      case 'completed':
        return {
          label: 'Agent stopped. Pulse will hide this host after the next missed heartbeat.',
          tone: 'success',
        };
      case 'acknowledged':
        return { label: 'Agent acknowledged the stop command—waiting for shutdown.', tone: 'info' };
      case 'dispatched':
        return { label: 'Instruction delivered to the agent.', tone: 'info' };
      case 'queued':
        return { label: 'Stop command queued; waiting to reach the agent.', tone: 'info' };
      default:
        if (host.pendingUninstall) {
          return { label: 'Marked for uninstall; waiting for agent confirmation.', tone: 'info' };
        }
        return null;
    }
  };

  createEffect(() => {
    if (!showRemoveModal()) return;
    const id = hostToRemoveId();
    const host = hostToRemove();
    if (id && !host) {
      closeRemoveModal();
    }
  });

  createEffect(() => {
    if (!showRemoveModal()) {
      return;
    }
    if (removeActionLoading() === 'awaitingCommand') {
      if (modalHasCommand() || modalHostPendingUninstall() || modalCommandFailed()) {
        setRemoveActionLoading(null);
      }
    }
  });

  // Track elapsed time for command execution
  createEffect(() => {
    const cmd = modalCommand();
    if (!cmd) {
      setCommandQueuedTime(null);
      setElapsedSeconds(0);
      return;
    }

    // Set queued time when command first appears
    if (cmd.createdAt && !commandQueuedTime()) {
      setCommandQueuedTime(new Date(cmd.createdAt));
    }

    // Update elapsed time every second while command is in progress
    if (modalCommandInProgress()) {
      const interval = setInterval(() => {
        const queuedTime = commandQueuedTime();
        if (queuedTime) {
          const now = new Date();
          const elapsed = Math.floor((now.getTime() - queuedTime.getTime()) / 1000);
          setElapsedSeconds(elapsed);
        }
      }, 1000);

      return () => clearInterval(interval);
    }
  });

  createEffect(() => {
    if (!showRemoveModal()) {
      return;
    }
    if (modalCommandCompleted() && !hasShownCommandCompletion()) {
      setHasShownCommandCompletion(true);
      notificationStore.success('Agent stopped. Pulse will hide this host after the next heartbeat.', 5000);
      if (typeof window !== 'undefined') {
        window.setTimeout(() => {
          closeRemoveModal();
        }, 1200);
      } else {
        closeRemoveModal();
      }
    }
  });

  onMount(() => {
    if (typeof window === 'undefined') {
      return;
    }

    const fetchSecurityStatus = async () => {
      try {
        const response = await fetch('/api/security/status', { credentials: 'include' });
        if (response.ok) {
          const data = (await response.json()) as SecurityStatus;
          setSecurityStatus(data);
        }
      } catch (err) {
        if (!hasLoggedSecurityStatusError) {
          hasLoggedSecurityStatusError = true;
          logger.error('Failed to load security status', err);
        }
      }
    };
    fetchSecurityStatus();
  });

  const showInstallCommand = () => !requiresToken() || Boolean(currentToken());

  const handleGenerateToken = async () => {
    if (isGeneratingToken()) return;
    setIsGeneratingToken(true);
    try {
      const name = tokenName().trim() || `Container host ${new Date().toISOString().slice(0, 10)}`;
      const { token, record } = await SecurityAPI.createToken(name, [DOCKER_REPORT_SCOPE]);

      setCurrentToken(token);
      setLatestRecord(record);
      setTokenName('');
      notificationStore.success('Token generated and inserted into the command below.', 4000);
    } catch (err) {
      logger.error('Failed to generate API token', err);
      const errorMsg = err instanceof Error ? err.message : 'Unknown error';
      notificationStore.error(`Failed to generate API token: ${errorMsg}`, 8000);
    } finally {
      setIsGeneratingToken(false);
    }
  };

  const requiresToken = () => {
    const status = securityStatus();
    if (status) {
      return status.requiresAuth || status.apiTokenConfigured;
    }
    return true;
  };

  // Always return command template with placeholder; the UI replaces it with the selected token.
  const getInstallCommandTemplate = () => {
    const url = pulseUrl();
    const tokenValue = requiresToken() ? TOKEN_PLACEHOLDER : 'disabled';
    const tokenSegment = `--token '${tokenValue}'`;
    return `curl -fsSL '${url}/install.sh' | bash -s -- --url '${url}' ${tokenSegment} --enable-docker`;
  };

  const getUninstallCommand = () => {
    const url = pulseUrl();
    return `curl -fsSL '${url}/install.sh' | bash -s -- --uninstall`;
  };

  const getSystemdService = () => {
    const token = requiresToken() ? TOKEN_PLACEHOLDER : 'disabled';
    return `[Unit]
Description=Pulse Unified Agent
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pulse-agent --url ${pulseUrl()} --token ${token} --interval 30s --enable-docker
Restart=always
RestartSec=5s
User=root

[Install]
WantedBy=multi-user.target`;
  };

  const getAllowReenrollCommand = (hostId: string) => {
    const url = pulseUrl();
    return `curl -X POST -H "X-API-Token: <token-with-docker:manage>" ${url}/api/agents/docker/hosts/${hostId}/allow-reenroll`;
  };

  const handleAllowReenroll = async (hostId: string, label: string) => {
    try {
      await MonitoringAPI.allowDockerHostReenroll(hostId);
      notificationStore.success(`Allowed ${label} to report again`, 4000);
    } catch (error) {
      logger.error('Failed to allow host re-enroll', error);
      const message =
        error instanceof Error
          ? error.message
          : 'Failed to clear the removal block. Confirm your account has docker:manage access.';
      notificationStore.error(message, 8000);
    }
  };

  const handleCopyAllowCommand = async (hostId: string, label: string) => {
    const command = getAllowReenrollCommand(hostId);
    const copied = await copyToClipboard(command);
    if (copied) {
      notificationStore.success(`Command copied for ${label}`, 3500);
    } else {
      notificationStore.error('Copy failed. You can still manually copy the snippet.', 4000);
    }
  };

  const isRemovingHost = (hostId: string) => removingHostId() === hostId;

  const openRemoveModal = (host: DockerHost) => {
    setHostToRemoveId(host.id);
    setUninstallCommandCopied(false);
    setRemoveActionLoading(null);
    setShowAdvancedOptions(false);
    setShowRemoveModal(true);
    setHasShownCommandCompletion(false);
  };

  const closeRemoveModal = () => {
    setShowRemoveModal(false);
    setHostToRemoveId(null);
    setUninstallCommandCopied(false);
    setRemoveActionLoading(null);
    setShowAdvancedOptions(false);
    setHasShownCommandCompletion(false);
  };

  const handleQueueStopCommand = async () => {
    const host = hostToRemove();
    if (!host || removeActionLoading()) return;

    const displayName = getDisplayName(host);
    setRemovingHostId(host.id);
    setRemoveActionLoading('queue');

    try {
      await MonitoringAPI.deleteDockerHost(host.id);
      notificationStore.success(`Stop command sent to ${displayName}`, 3500);
      setRemoveActionLoading('awaitingCommand');
    } catch (error) {
      logger.error('Failed to queue host stop command', error);
      const message = error instanceof Error ? error.message : 'Failed to send stop command';
      notificationStore.error(message, 8000);
      setRemoveActionLoading(null);
    } finally {
      setRemovingHostId(null);
    }
  };

  const handleHideHostFromModal = async () => {
    const host = hostToRemove();
    if (!host || (removeActionLoading() && removeActionLoading() !== 'awaitingCommand')) return;

    const displayName = getDisplayName(host);
    setRemovingHostId(host.id);
    setRemoveActionLoading('hide');

    try {
      await MonitoringAPI.deleteDockerHost(host.id, { hide: true });
      notificationStore.success(`Hidden host ${displayName}`, 3500);
      closeRemoveModal();
    } catch (error) {
      logger.error('Failed to hide host', error);
      const message = error instanceof Error ? error.message : 'Failed to hide host';
      notificationStore.error(message, 8000);
    } finally {
      setRemovingHostId(null);
      setRemoveActionLoading(null);
    }
  };

  const handleRemoveHostNow = async () => {
    const host = hostToRemove();
    if (!host || (removeActionLoading() && removeActionLoading() !== 'awaitingCommand')) return;

    const displayName = getDisplayName(host);
    setRemovingHostId(host.id);
    setRemoveActionLoading('force');

    try {
      await MonitoringAPI.deleteDockerHost(host.id, { force: true });
      notificationStore.success(`Removed host ${displayName}`, 3500);
      closeRemoveModal();
    } catch (error) {
      logger.error('Failed to remove host', error);
      const message = error instanceof Error ? error.message : 'Failed to remove host';
      notificationStore.error(message, 8000);
    } finally {
      setRemovingHostId(null);
      setRemoveActionLoading(null);
    }
  };

  const handleCleanupOfflineHost = async (hostId: string, displayName: string) => {
    if (isRemovingHost(hostId)) return;

    setRemovingHostId(hostId);

    try {
      await MonitoringAPI.deleteDockerHost(hostId, { force: true });
      notificationStore.success(`Removed host ${displayName}`, 3500);
      if (hostToRemoveId() === hostId) {
        closeRemoveModal();
      }
    } catch (error) {
      logger.error('Failed to remove host', error);
      const message = error instanceof Error ? error.message : 'Failed to remove host';
      notificationStore.error(message, 8000);
    } finally {
      setRemovingHostId(null);
    }
  };

  const handleUnhideHost = async (hostId: string, displayName: string) => {
    if (isRemovingHost(hostId)) return;

    setRemovingHostId(hostId);

    try {
      await MonitoringAPI.unhideDockerHost(hostId);
      notificationStore.success(`Unhidden host ${displayName}`, 3500);
    } catch (error) {
      logger.error('Failed to unhide host', error);
      const message = error instanceof Error ? error.message : 'Failed to unhide host';
      notificationStore.error(message, 8000);
    } finally {
      setRemovingHostId(null);
    }
  };

  const startEditingDisplayName = (host: DockerHost) => {
    setEditingHostId(host.id);
    setEditingDisplayName(host.customDisplayName || '');
  };

  const cancelEditingDisplayName = () => {
    setEditingHostId(null);
    setEditingDisplayName('');
  };

  const saveDisplayName = async (hostId: string, originalName: string) => {
    const newName = editingDisplayName().trim();

    setSavingDisplayName(true);
    try {
      await MonitoringAPI.setDockerHostDisplayName(hostId, newName);
      notificationStore.success(`Updated display name for ${originalName}`, 3500);
      setEditingHostId(null);
      setEditingDisplayName('');
    } catch (error) {
      logger.error('Failed to update display name', error);
      const message = error instanceof Error ? error.message : 'Failed to update display name';
      notificationStore.error(message, 8000);
    } finally {
      setSavingDisplayName(false);
    }
  };

  return (
    <div class="space-y-6">
      <Show when={hasRemovedHosts()}>
        <Card
          padding="lg"
          class="space-y-4 border border-amber-300 bg-amber-50 text-amber-900 shadow-sm dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-100"
        >
          <div class="space-y-1">
            <h3 class="text-sm font-semibold">Recently removed container hosts</h3>
            <p class="text-sm text-amber-800 dark:text-amber-200">
              Pulse is currently blocking these hosts because they were explicitly removed. Allow them to re-enroll or
              copy the command below and run it with a token that includes the <code>docker:manage</code> scope.
            </p>
          </div>

          <div class="space-y-3">
            <For each={removedHosts()}>
              {(entry) => {
                const label = entry.displayName || entry.hostname || entry.id;
                return (
                  <div class="rounded-lg border border-amber-200 bg-white/80 p-4 shadow-sm dark:border-amber-500/40 dark:bg-amber-950/20">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">{label}</p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">Host ID: {entry.id}</p>
                      </div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">
                        Removed {formatRelativeTime(entry.removedAt)}
                      </div>
                    </div>

                    <div class="mt-3 flex flex-wrap gap-2">
                      <button
                        type="button"
                        onClick={() => handleAllowReenroll(entry.id, label)}
                        class="inline-flex items-center justify-center gap-2 rounded-md bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-1 dark:focus:ring-offset-gray-900"
                      >
                        Allow re-enroll
                      </button>
                      <button
                        type="button"
                        onClick={() => handleCopyAllowCommand(entry.id, label)}
                        class="inline-flex items-center justify-center gap-2 rounded-md border border-emerald-600/50 px-3 py-1.5 text-xs font-medium text-emerald-700 transition hover:bg-emerald-50 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-1 dark:border-emerald-400/60 dark:text-emerald-200 dark:hover:bg-emerald-500/20 dark:focus:ring-offset-gray-900"
                      >
                        Copy curl command
                      </button>
                    </div>
                  </div>
                );
              }}
            </For>
            <p class="text-xs text-amber-800 dark:text-amber-200">
              If you removed a host intentionally, you can simply ignore it—entries expire automatically after 24 hours.
            </p>
          </div>
        </Card>
      </Show>

      <Card padding="lg" class="space-y-5">
        <div class="space-y-2">
          <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Enroll a container runtime</h3>
          <p class="text-sm text-gray-600 dark:text-gray-400">
            Run the command below on any host running Docker or Podman. The installer will automatically detect your container runtime.
          </p>
        </div>

        <div class="space-y-5">
          <Show when={requiresToken()}>
            <div class="space-y-3">
              <div class="space-y-1">
                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">Generate API token</p>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  Create a fresh token scoped to <code>{DOCKER_REPORT_SCOPE}</code>
                </p>
              </div>

              <div class="flex gap-2">
                <input
                  type="text"
                  value={tokenName()}
                  onInput={(e) => setTokenName(e.currentTarget.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !isGeneratingToken()) {
                      handleGenerateToken();
                    }
                  }}
                  placeholder="Token name (optional)"
                  class="flex-1 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-900/60"
                />
                <button
                  type="button"
                  onClick={handleGenerateToken}
                  disabled={isGeneratingToken()}
                  class="inline-flex items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isGeneratingToken() ? 'Generating…' : currentToken() ? 'Generate another' : 'Generate token'}
                </button>
              </div>

              <Show when={latestRecord()}>
                <div class="flex items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-4 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
                  <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                  </svg>
                  <span>
                    Token <strong>{latestRecord()?.name}</strong> created and inserted into the command below.
                  </span>
                </div>
              </Show>
            </div>
          </Show>

          <Show when={showInstallCommand()}>
            <div class="space-y-3">
              <div class="flex items-center justify-between">
                <label class="text-sm font-semibold text-gray-900 dark:text-gray-100">Install command</label>
                <button
                  type="button"
                  onClick={async () => {
                    const command = getInstallCommandTemplate().replace(TOKEN_PLACEHOLDER, currentToken() || TOKEN_PLACEHOLDER);
                    const success = await copyToClipboard(command);
                    if (typeof window !== 'undefined' && window.showToast) {
                      window.showToast(success ? 'success' : 'error', success ? 'Copied!' : 'Failed to copy');
                    }
                  }}
                  class="px-3 py-1.5 text-xs font-medium rounded-lg transition-colors bg-blue-600 text-white hover:bg-blue-700"
                >
                  Copy command
                </button>
              </div>
              <pre class="overflow-x-auto rounded-md bg-gray-900/90 p-3 text-xs text-gray-100">
                <code>{getInstallCommandTemplate().replace(TOKEN_PLACEHOLDER, currentToken() || TOKEN_PLACEHOLDER)}</code>
              </pre>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                The unified installer downloads the agent, detects your container runtime, configures a systemd service, and starts reporting automatically.
              </p>
            </div>
          </Show>

          <Show when={requiresToken() && !currentToken()}>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              Generate a token to see the install command.
            </p>
          </Show>
        </div>

        <details class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-700 dark:border-gray-700 dark:bg-gray-800/50 dark:text-gray-300">
          <summary class="cursor-pointer text-sm font-medium text-gray-900 dark:text-gray-100">
            Advanced options (uninstall & manual install)
          </summary>
          <div class="mt-3 space-y-4">
            <div>
              <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Uninstall</p>
              <div class="mt-2 flex items-center gap-2">
                <code class="flex-1 break-all rounded bg-gray-900 px-3 py-2 font-mono text-xs text-red-400 dark:bg-gray-950">
                  {getUninstallCommand()}
                </code>
                <button
                  type="button"
                  onClick={async () => {
                    const success = await copyToClipboard(getUninstallCommand());
                    if (typeof window !== 'undefined' && window.showToast) {
                      window.showToast(success ? 'success' : 'error', success ? 'Copied to clipboard' : 'Failed to copy to clipboard');
                    }
                  }}
                  class="rounded-lg bg-red-50 px-3 py-1.5 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 dark:bg-red-900/30 dark:text-red-300 dark:hover:bg-red-900/50"
                >
                  Copy
                </button>
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                Stops the agent, removes the binary, the systemd unit, and related files.
              </p>
            </div>

            <div>
              <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Manual installation</p>
              <div class="mt-2 space-y-3 rounded-lg border border-gray-200 bg-white p-3 text-xs dark:border-gray-700 dark:bg-gray-900">
                <p class="font-medium text-gray-900 dark:text-gray-100">1. Build the binary</p>
                <div class="rounded bg-gray-900 p-3 font-mono text-xs text-gray-100 dark:bg-gray-950">
                  <code>
                    cd /opt/pulse
                    <br />
                    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pulse-agent ./cmd/pulse-agent
                  </code>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  Building with <code class="font-mono text-[11px]">CGO_ENABLED=0</code> keeps the binary fully static so it runs on hosts with older glibc (e.g. Debian 11).
                </p>
                <p class="font-medium text-gray-900 dark:text-gray-100">2. Copy to host</p>
                <div class="rounded bg-gray-900 p-3 font-mono text-xs text-gray-100 dark:bg-gray-950">
                  <code>
                    scp pulse-agent user@docker-host:/usr/local/bin/
                    <br />
                    ssh user@docker-host chmod +x /usr/local/bin/pulse-agent
                  </code>
                </div>
                <p class="font-medium text-gray-900 dark:text-gray-100">3. Systemd template</p>
                <div class="relative">
                  <button
                    type="button"
                    onClick={async () => {
                      const success = await copyToClipboard(getSystemdService());
                      if (typeof window !== 'undefined' && window.showToast) {
                        window.showToast(success ? 'success' : 'error', success ? 'Copied to clipboard' : 'Failed to copy to clipboard');
                      }
                    }}
                    class="absolute right-2 top-2 rounded-lg bg-gray-700 px-3 py-1.5 text-xs font-medium text-gray-200 transition-colors hover:bg-gray-600"
                  >
                    Copy
                  </button>
                  <div class="rounded bg-gray-900 p-3 font-mono text-xs text-gray-100 dark:bg-gray-950">
                    <pre>{getSystemdService()}</pre>
                  </div>
                </div>
                <p class="font-medium text-gray-900 dark:text-gray-100">4. Enable & start</p>
                <div class="rounded bg-gray-900 p-3 font-mono text-xs text-gray-100 dark:bg-gray-950">
                  <code>
                    systemctl daemon-reload
                    <br />
                    systemctl enable --now pulse-agent
                  </code>
                </div>
              </div>
            </div>
          </div>
        </details>
      </Card>

      {/* Remove Container Host Modal */}
      <Show when={showRemoveModal()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div class="w-full max-w-2xl rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800">
            <div class="space-y-2">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                Remove container host "{modalDisplayName()}"
              </h3>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                Pulse guides you through uninstalling the agent and safely cleaning up the host entry.
              </p>
            </div>

            <div class="mt-4 space-y-4">
              <div class="rounded-lg border border-blue-200 bg-blue-50 p-4 space-y-3 dark:border-blue-800 dark:bg-blue-900/20">
                <div class="flex items-start gap-3">
                  <svg class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <div class="flex-1 space-y-2">
                    <h4 class="text-sm font-semibold text-blue-900 dark:text-blue-100">Step 1 · Pulse stops the agent</h4>
                    <p class="text-sm text-blue-800 dark:text-blue-200">
                      Pulse will queue a stop command with this host. The agent shuts down its system service (or Unraid autostart hook if present), confirms back to Pulse, and the row disappears as soon as that acknowledgement arrives—or after the next missed heartbeat.
                    </p>
                    <Show when={modalCommand()}>
                      <div class="rounded border border-blue-200 bg-white px-3 py-2 text-xs text-blue-800 dark:border-blue-700 dark:bg-blue-800/20 dark:text-blue-200">
                        <div class="flex items-center justify-between gap-3">
                          <span class="font-semibold uppercase tracking-wide text-[11px] text-blue-600 dark:text-blue-300">Command status</span>
                          <span class="rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium uppercase text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                            {modalCommandStatus()}
                          </span>
                        </div>
                        <Show when={modalCommand()?.message}>
                          <p class="mt-2 leading-snug">{modalCommand()?.message}</p>
                        </Show>
                        <Show when={modalCommandFailed()}>
                          <p class="mt-2 font-medium text-red-600 dark:text-red-300">
                            {modalCommand()?.failureReason || 'Pulse could not stop the agent automatically.'}
                          </p>
                        </Show>
                      </div>
                    </Show>
                    <button
                      type="button"
                      onClick={handleQueueStopCommand}
                      disabled={
                        removeActionLoading() !== null ||
                        modalCommandInProgress() ||
                        modalCommandStatus() === 'completed' ||
                        (modalHostPendingUninstall() && !modalHasCommand())
                      }
                      class={`inline-flex items-center justify-center rounded px-4 py-2 text-sm font-medium text-white transition-colors ${modalCommandStatus() === 'completed'
                        ? 'bg-emerald-600 dark:bg-emerald-500'
                        : 'bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-400'
                        } disabled:cursor-not-allowed disabled:opacity-60`}
                    >
                      {(() => {
                        if (removeActionLoading() === 'queue') return 'Sending…';
                        if (removeActionLoading() === 'awaitingCommand') return 'Waiting for agent…';
                        if (modalCommandInProgress()) return 'Waiting for agent…';
                        if (modalCommandStatus() === 'completed') return 'Agent stopped';
                        if (!modalHasCommand() && modalHostPendingUninstall()) return 'Waiting for host…';
                        if (modalCommandFailed()) return 'Retry stop command';
                        return 'Stop agent now';
                      })()}
                    </button>
                    <Show
                      when={
                        removeActionLoading() === 'awaitingCommand' &&
                        !modalHasCommand() &&
                        !modalHostPendingUninstall()
                      }
                    >
                      <div class="rounded border border-blue-200 bg-white p-3 dark:border-blue-700 dark:bg-blue-800/20">
                        <div class="flex items-start gap-2 text-xs text-blue-700 dark:text-blue-200">
                          <svg class="mt-0.5 h-4 w-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                          </svg>
                          <div>
                            <p class="font-semibold">Stop command sent.</p>
                            <p class="mt-1 leading-snug">
                              Pulse is waiting for <span class="font-medium">{modalHostname()}</span> to pick up the shutdown instruction. This usually finishes within 30-60 seconds.
                            </p>
                          </div>
                        </div>
                      </div>
                    </Show>
                    <Show
                      when={
                        modalCommandInProgress() ||
                        modalCommandCompleted() ||
                        (!modalHasCommand() && modalHostPendingUninstall())
                      }
                    >
                      <div class="space-y-3">
                        <Show when={modalHasCommand()}>
                          <div class="rounded border border-blue-200 bg-white p-3 dark:border-blue-700 dark:bg-blue-800/20">
                            <div class="mb-2 flex items-center justify-between">
                              <span class="text-xs font-semibold uppercase tracking-wide text-blue-700 dark:text-blue-300">Progress</span>
                              <Show
                                when={!modalCommandCompleted()}
                                fallback={<span class="text-xs font-semibold text-emerald-600 dark:text-emerald-300">Completed</span>}
                              >
                                <span class="text-xs text-blue-600 dark:text-blue-400">{formatElapsedTime(elapsedSeconds())} elapsed</span>
                              </Show>
                            </div>
                            <ul class="space-y-1.5">
                              <For each={modalCommandProgress()}>
                                {(step) => (
                                  <li
                                    class={`${step.done || step.active ? 'text-blue-700 dark:text-blue-200' : 'text-gray-500 dark:text-gray-400'} flex items-center gap-2 text-xs`}
                                  >
                                    <span
                                      class={`relative h-2 w-2 flex-shrink-0 rounded-full ${step.done
                                        ? 'bg-blue-500'
                                        : step.active
                                          ? 'bg-blue-400 animate-pulse'
                                          : 'bg-gray-300 dark:bg-gray-600'
                                        } ${modalCommandCompleted() && step.done ? 'after:absolute after:-inset-1 after:rounded-full after:border after:border-emerald-400/40 after:animate-pulse' : ''}`}
                                    />
                                    {step.label}
                                  </li>
                                )}
                              </For>
                            </ul>
                          </div>
                        </Show>

                        <Show when={!modalHasCommand() && modalHostPendingUninstall()}>
                          <div class="rounded border border-blue-200 bg-white p-3 text-xs text-blue-700 dark:border-blue-700 dark:bg-blue-800/20 dark:text-blue-200">
                            <p class="font-semibold">Agent already stopped.</p>
                            <p class="mt-1 leading-snug">
                              Pulse is waiting for <span class="font-medium">{modalHostname()}</span> to miss its next heartbeat so the host can be removed automatically. No further action is required—this usually finishes within 60 seconds.
                            </p>
                            <Show when={modalLastHeartbeat()}>
                              <p class="mt-2 text-[11px]">
                                Last heartbeat: {modalLastHeartbeat()}. Pulse will clear the entry after the next missed report.
                              </p>
                            </Show>
                          </div>
                        </Show>

                        <Show when={modalHasCommand() && !modalCommandCompleted()}>
                          <div class="flex items-start gap-2 text-xs text-blue-700 dark:text-blue-300">
                            <svg class="w-4 h-4 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                            <div>
                              <p>
                                <Show when={!modalCommandTimedOut()} fallback="This is taking longer than expected.">
                                  This usually takes 30-60 seconds.
                                </Show>
                                <Show when={modalLastHeartbeat()}>
                                  {' '}Last heartbeat: {modalLastHeartbeat()}.
                                </Show>
                              </p>
                            </div>
                          </div>
                        </Show>

                        <Show when={modalHasCommand() && modalCommandTimedOut() && !modalCommandCompleted()}>
                          <div class="rounded border border-yellow-200 bg-yellow-50 p-3 dark:border-yellow-700 dark:bg-yellow-900/20">
                            <div class="flex items-start gap-2">
                              <svg class="w-4 h-4 mt-0.5 flex-shrink-0 text-yellow-600 dark:text-yellow-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                              </svg>
                              <div class="flex-1 text-xs">
                                <p class="font-semibold text-yellow-900 dark:text-yellow-100">Command taking longer than expected</p>
                                <p class="mt-1 text-yellow-800 dark:text-yellow-200">
                                  The agent may be offline or experiencing issues. Consider using "Force remove now" below to skip the agent stop and remove the host immediately.
                                </p>
                              </div>
                            </div>
                          </div>
                        </Show>
                      </div>
                    </Show>
                    <Show when={modalCommandCompleted()}>
                      <div class="rounded border border-emerald-200 bg-white p-3 text-xs text-emerald-800 dark:border-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-100">
                        <p class="font-medium">
                          Agent confirmed the stop. Pulse has already cleaned up everything it controls:
                        </p>
                        <ul class="mt-2 space-y-1 leading-snug">
                          <li>• Terminated the running <code class="font-mono text-[11px]">pulse-agent</code> process</li>
                          <li>• Disabled future auto-start (stops the systemd unit or removes the Unraid autostart script if one exists)</li>
                          <li>• Cleared the host from the dashboard so new reports won’t appear unexpectedly</li>
                        </ul>
                        <p class="mt-2">
                          The binary remains at <code class="font-mono text-[11px]">/usr/local/bin/pulse-agent</code> for quick reinstalls. Use the uninstall command below if you prefer to remove it too.
                        </p>
                      </div>
                    </Show>
                    <Show when={modalCommandFailed() && modalCommand()?.failureReason}>
                      <p class="text-xs text-red-600 dark:text-red-300">
                        {modalCommand()?.failureReason}
                      </p>
                    </Show>
                    <Show when={modalCommand()}>
                      <details class="rounded border border-blue-200 bg-white p-3 text-xs text-gray-700 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
                        <summary class="flex cursor-pointer items-center justify-between gap-2">
                          <span class="font-semibold uppercase tracking-wide text-[11px] text-blue-700 dark:text-blue-300">Behind the scenes</span>
                          <code class="rounded bg-blue-100 px-2 py-0.5 font-mono text-[11px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                            {modalCommand()?.id}
                          </code>
                        </summary>
                        <div class="mt-2 space-y-2">
                          <ul class="space-y-1">
                            <For each={modalCommandProgress()}>
                              {(step) => (
                                <li
                                  class={`${step.done || step.active ? 'text-blue-700 dark:text-blue-200' : 'text-gray-500 dark:text-gray-400'} flex items-center gap-2`}
                                >
                                  <span
                                    class={`relative h-2 w-2 rounded-full ${step.done
                                      ? 'bg-blue-500'
                                      : step.active
                                        ? 'bg-blue-400 animate-pulse'
                                        : 'bg-gray-300 dark:bg-gray-600'
                                      } ${modalCommandCompleted() && step.done ? 'after:absolute after:-inset-1 after:rounded-full after:border after:border-emerald-400/40 after:animate-pulse' : ''}`}
                                  />
                                  {step.label}
                                </li>
                              )}
                            </For>
                          </ul>
                          <p class="leading-snug">
                            Pulse responds to the agent's <code class="font-mono text-[11px]">/api/agents/docker/report</code> call with a stop command. The agent disables its service, removes
                            <code class="font-mono text-[11px]">/boot/config/go.d/pulse-agent.sh</code>, and posts back to
                            <code class="font-mono text-[11px]">/api/agents/docker/commands/&lt;id&gt;/ack</code> so Pulse knows it can remove the row.
                          </p>
                        </div>
                      </details>
                    </Show>
                  </div>
                </div>
              </div>

              {/* Force remove option when command times out */}
              <Show when={modalCommandTimedOut()}>
                <div class="rounded-lg border border-orange-200 bg-orange-50 p-4 dark:border-orange-800 dark:bg-orange-900/20">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div class="flex items-start gap-3">
                      <svg class="w-5 h-5 text-orange-600 dark:text-orange-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                      </svg>
                      <div>
                        <h4 class="text-sm font-semibold text-orange-900 dark:text-orange-100">Skip waiting and remove now</h4>
                        <p class="text-sm text-orange-800 dark:text-orange-200">
                          Still waiting after {formatElapsedTime(elapsedSeconds())}. Remove the host entry immediately without waiting for the agent.
                        </p>
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={handleRemoveHostNow}
                      disabled={removeActionLoading() !== null && removeActionLoading() !== 'awaitingCommand'}
                      class="self-start rounded bg-orange-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-orange-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-orange-500 dark:hover:bg-orange-400 whitespace-nowrap"
                    >
                      {removeActionLoading() === 'force' ? 'Removing…' : 'Force remove now'}
                    </button>
                  </div>
                </div>
              </Show>

              <Show when={!modalHostIsOnline()}>
                <div class="rounded-lg border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-800 dark:bg-emerald-900/20">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div>
                      <h4 class="text-sm font-semibold text-emerald-900 dark:text-emerald-100">Host is offline</h4>
                      <p class="text-sm text-emerald-800 dark:text-emerald-200">
                        Tower stopped reporting. Remove it now to finish the cleanup.
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={handleRemoveHostNow}
                      disabled={removeActionLoading() !== null && removeActionLoading() !== 'awaitingCommand'}
                      class="self-start rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-emerald-500 dark:hover:bg-emerald-400"
                    >
                      {removeActionLoading() === 'force' ? 'Removing…' : 'Remove host'}
                    </button>
                  </div>
                </div>
              </Show>

              <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-900">
                <button
                  type="button"
                  onClick={() => setShowAdvancedOptions((prev) => !prev)}
                  class="flex w-full items-center justify-between text-sm font-semibold text-gray-900 transition-colors hover:text-blue-600 dark:text-gray-100 dark:hover:text-blue-300"
                >
                  <span>Need something else?</span>
                  <svg class={`h-4 w-4 transition-transform ${showAdvancedOptions() ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                <Show when={showAdvancedOptions()}>
                  <div class="mt-3 space-y-3 text-sm">
                    <div class="flex flex-col gap-2 rounded border border-gray-200 p-3 dark:border-gray-700">
                      <div>
                        <p class="font-semibold text-gray-900 dark:text-gray-100">Manual uninstall command</p>
                        <p class="text-xs text-gray-600 dark:text-gray-400">
                          Prefer to remove everything manually? Run this full uninstall on <code class="font-mono text-[11px]">{modalHostname()}</code>. It removes the service, startup script, log, and binary.
                        </p>
                      </div>
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                        <code class="flex-1 rounded bg-gray-900 px-3 py-2 font-mono text-xs text-gray-100 dark:bg-gray-950 overflow-x-auto">
                          {getUninstallCommand()}
                        </code>
                        <button
                          type="button"
                          onClick={async () => {
                            const success = await copyToClipboard(getUninstallCommand());
                            if (success) {
                              setUninstallCommandCopied(true);
                            }
                            if (typeof window !== 'undefined' && window.showToast) {
                              window.showToast(success ? 'success' : 'error', success ? 'Copied!' : 'Failed to copy');
                            }
                          }}
                          class="self-start rounded-lg bg-gray-800 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-gray-700"
                        >
                          Copy command
                        </button>
                      </div>
                      <Show when={uninstallCommandCopied()}>
                        <p class="text-[11px] font-medium text-gray-600 dark:text-gray-300">Command copied to clipboard.</p>
                      </Show>
                      <p class="text-[11px] text-gray-500 dark:text-gray-400">
                        This command stops the agent, removes the systemd service (or Unraid autostart hook), deletes <code class="font-mono text-[11px]">/var/log/pulse-agent.log</code>, and uninstalls the binary. Pulse will notice the host is gone after the next heartbeat (≈2 minutes) and clean up the row automatically.
                      </p>
                    </div>
                    <div class="flex flex-col gap-2 rounded border border-gray-200 p-3 dark:border-gray-700">
                      <div>
                        <p class="font-semibold text-gray-900 dark:text-gray-100">Force remove immediately</p>
                        <p class="text-xs text-gray-600 dark:text-gray-400">
                          Skips the stop command and removes the entry right away. Any new report from this host will be rejected until you reinstall.
                        </p>
                      </div>
                      <button
                        type="button"
                        onClick={handleRemoveHostNow}
                        disabled={removeActionLoading() !== null && removeActionLoading() !== 'awaitingCommand'}
                        class="self-start rounded bg-red-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-red-500 dark:hover:bg-red-400"
                      >
                        {removeActionLoading() === 'force' ? 'Removing…' : 'Force remove now'}
                      </button>
                    </div>
                    <div class="flex flex-col gap-2 rounded border border-gray-200 p-3 dark:border-gray-700">
                      <div>
                        <p class="font-semibold text-gray-900 dark:text-gray-100">Hide the host instead</p>
                        <p class="text-xs text-gray-600 dark:text-gray-400">
                          Remove it from the default view without uninstalling the agent. It will reappear if the agent reports again.
                        </p>
                        <Show when={modalHostHidden()}>
                          <p class="mt-1 text-xs text-blue-700 dark:text-blue-300">Already hidden.</p>
                        </Show>
                      </div>
                      <button
                        type="button"
                        onClick={handleHideHostFromModal}
                        disabled={(removeActionLoading() !== null && removeActionLoading() !== 'awaitingCommand') || modalHostHidden()}
                        class="self-start rounded bg-gray-200 px-3 py-1.5 text-xs font-medium text-gray-800 transition-colors hover:bg-gray-300 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600"
                      >
                        {removeActionLoading() === 'hide' ? 'Hiding...' : modalHostHidden() ? 'Already hidden' : 'Hide host'}
                      </button>
                    </div>
                  </div>
                </Show>
              </div>
            </div>

            <div class="mt-6 flex justify-end">
              <button
                type="button"
                onClick={closeRemoveModal}
                class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      </Show>

      {/* Active container hosts */}
      <Card>
        <div class="space-y-4">
          {/* Pending hosts banner */}
          <Show when={pendingHosts().length > 0}>
            <div class="rounded-lg border border-yellow-200 bg-yellow-50 px-4 py-3 dark:border-yellow-700 dark:bg-yellow-900/30">
              <div class="flex items-start gap-3">
                <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div class="flex-1 space-y-3">
                  <div>
                    <h4 class="text-sm font-semibold text-yellow-900 dark:text-yellow-100">
                      Stopping {pendingHosts().length} host{pendingHosts().length !== 1 ? 's' : ''}
                    </h4>
                    <p class="mt-1 text-sm text-yellow-800 dark:text-yellow-200">
                      Pulse has the stop command in flight. You can keep working—these hosts will disappear automatically once the agent shuts down or misses its next heartbeat.
                    </p>
                  </div>

                  <div class="space-y-2">
                    <For each={pendingHosts()}>
                      {(host) => {
                        const label = getDisplayName(host);
                        const statusInfo = getRemovalStatusInfo(host) ?? {
                          label: 'Marked for uninstall; waiting for agent confirmation.',
                          tone: 'info' as RemovalStatusTone,
                        };
                        const status = host.command?.status ?? (host.pendingUninstall ? 'pending' : 'unknown');
                        const isOnline = host.status?.toLowerCase() === 'online';
                        const lastSeenLabel = host.lastSeen ? formatRelativeTime(host.lastSeen) : 'Awaiting first report';
                        const badgeText =
                          status === 'completed'
                            ? 'Agent stopped'
                            : status === 'acknowledged'
                              ? 'Acknowledged'
                              : status === 'dispatched'
                                ? 'Dispatched'
                                : status === 'queued'
                                  ? 'Queued'
                                  : 'Pending';

                        return (
                          <div class="rounded-lg border border-yellow-200 bg-white/80 p-3 shadow-sm dark:border-yellow-700/40 dark:bg-yellow-900/20">
                            <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                              <div>
                                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">{label}</p>
                                <p class="text-xs text-gray-500 dark:text-gray-400">{host.hostname || host.id}</p>
                                <span class={removalBadgeClassMap[statusInfo.tone]}>{badgeText}</span>
                              </div>
                              <div class="text-[11px] text-gray-500 dark:text-gray-400 sm:text-right">
                                Last seen {lastSeenLabel}
                              </div>
                            </div>
                            <p class={`mt-2 text-xs ${removalTextClassMap[statusInfo.tone]}`}>{statusInfo.label}</p>
                            <div class="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                              <button
                                type="button"
                                class="inline-flex items-center justify-center gap-2 rounded bg-blue-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-blue-500 dark:hover:bg-blue-400"
                                onClick={() => openRemoveModal(host)}
                              >
                                View progress
                              </button>
                              <Show when={!isOnline || status === 'failed' || status === 'expired'}>
                                <button
                                  type="button"
                                  class="inline-flex items-center justify-center gap-2 rounded border border-blue-500/40 px-3 py-1.5 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-50 dark:border-blue-400/40 dark:text-blue-200 dark:hover:bg-blue-500/20"
                                  onClick={() => handleCleanupOfflineHost(host.id, label)}
                                  disabled={isRemovingHost(host.id)}
                                >
                                  {isRemovingHost(host.id) ? 'Cleaning up…' : 'Force remove now'}
                                </button>
                              </Show>
                            </div>
                          </div>
                        );
                      }}
                    </For>
                  </div>
                </div>
              </div>
            </div>
          </Show>

          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                Reporting container hosts ({dockerHosts().length})
              </h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Use this list to enroll or retire agents. For live health and troubleshooting, open the container monitoring view.
              </p>
            </div>
            <div class="flex flex-wrap items-center justify-end gap-2">
              <a
                href="/containers"
                class="inline-flex items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-1.5 text-xs font-semibold text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-600/60 dark:bg-blue-900/30 dark:text-blue-200 dark:hover:bg-blue-900/50"
              >
                <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
                </svg>
                Open container monitoring
              </a>
              <Show when={hiddenCount() > 0}>
                <button
                  type="button"
                  onClick={() => setShowHidden(!showHidden())}
                  class="px-3 py-1.5 text-xs font-medium rounded transition-colors bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
                >
                  {showHidden() ? 'Hide' : 'Show'} hidden ({hiddenCount()})
                </button>
              </Show>
            </div>
          </div>

          <Show
            when={dockerHosts().length > 0}
            fallback={
              <div class="text-center py-8">
                <div class="text-gray-400 dark:text-gray-500 mb-2">
                  <svg class="w-12 h-12 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
                    />
                  </svg>
                </div>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  No container agents are currently reporting.
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-500 mt-1">
                  Click "Show deployment instructions" above to get started.
                </p>
              </div>
            }
          >
            <div class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-gray-200 dark:border-gray-700">
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Host</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Status</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Agent &amp; runtime</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Last Seen</th>
                    <th class="text-right py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Actions</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={dockerHosts()}>
                    {(host) => {
                      const isOnline = host.status?.toLowerCase() === 'online';
                      const displayName = getDisplayName(host);
                      const commandStatus = host.command?.status ?? null;
                      const removalStatusInfo = getRemovalStatusInfo(host);
                      const runtimeInfo = resolveHostRuntime(host);
                      const commandInProgress =
                        commandStatus === 'queued' ||
                        commandStatus === 'dispatched' ||
                        commandStatus === 'acknowledged';
                      const commandFailed = commandStatus === 'failed';
                      const commandCompleted = commandStatus === 'completed';
                      const offlineActionLabel =
                        commandFailed || commandStatus === 'expired'
                          ? 'Force remove host'
                          : removalStatusInfo?.tone === 'success'
                            ? 'Remove host'
                            : host.pendingUninstall
                              ? 'Skip wait and remove now'
                              : 'Remove offline host';
                      const tokenRevoked = typeof host.tokenRevokedAt === 'number';
                      const tokenRevokedRelative = tokenRevoked ? formatRelativeTime(host.tokenRevokedAt!) : '';

                      return (
                        <tr class={`${isOnline ? 'bg-white dark:bg-gray-900' : 'bg-gray-50 dark:bg-gray-800/50 opacity-60'}`}>
                          <td class="py-3 px-4 align-top">
                            <Show
                              when={editingHostId() === host.id}
                              fallback={
                                <div class="flex items-center gap-2">
                                  <div class="flex-1">
                                    <div class="font-medium text-gray-900 dark:text-gray-100">{displayName}</div>
                                    <Show when={host.customDisplayName && host.customDisplayName !== host.displayName}>
                                      <div class="text-xs text-gray-400 dark:text-gray-500">
                                        Original: {host.displayName || host.hostname}
                                      </div>
                                    </Show>
                                  </div>
                                  <button
                                    type="button"
                                    class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                                    onClick={() => startEditingDisplayName(host)}
                                    title="Edit display name"
                                  >
                                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                                    </svg>
                                  </button>
                                </div>
                              }
                            >
                              <div class="flex items-center gap-2">
                                <input
                                  type="text"
                                  class="flex-1 px-2 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
                                  value={editingDisplayName()}
                                  onInput={(e) => setEditingDisplayName(e.currentTarget.value)}
                                  onKeyDown={(e) => {
                                    if (e.key === 'Enter') {
                                      saveDisplayName(host.id, displayName);
                                    } else if (e.key === 'Escape') {
                                      cancelEditingDisplayName();
                                    }
                                  }}
                                  placeholder="Custom display name"
                                  disabled={savingDisplayName()}
                                />
                                <button
                                  type="button"
                                  class="text-green-600 hover:text-green-700 disabled:opacity-50"
                                  onClick={() => saveDisplayName(host.id, displayName)}
                                  disabled={savingDisplayName()}
                                  title="Save"
                                >
                                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                  </svg>
                                </button>
                                <button
                                  type="button"
                                  class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 disabled:opacity-50"
                                  onClick={cancelEditingDisplayName}
                                  disabled={savingDisplayName()}
                                  title="Cancel"
                                >
                                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                  </svg>
                                </button>
                              </div>
                            </Show>
                            <div class="text-xs text-gray-500 dark:text-gray-400">{host.hostname}</div>
                            <div class="mt-1 text-[10px] uppercase tracking-wide">
                              <span
                                class={`inline-flex items-center rounded-full px-2 py-0.5 font-semibold ${runtimeInfo.badgeClass}`}
                                title={runtimeInfo.raw || runtimeInfo.label}
                              >
                                {runtimeInfo.label}
                              </span>
                            </div>
                            <Show when={host.os || host.architecture}>
                              <div class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                                {host.os}
                                <Show when={host.os && host.architecture}>
                                  <span class="mx-1">•</span>
                                </Show>
                                {host.architecture}
                              </div>
                            </Show>
                            <Show when={host.hidden}>
                              <span class="mt-2 inline-flex items-center rounded-full bg-gray-200 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-gray-700 dark:bg-gray-700 dark:text-gray-200">
                                Hidden
                              </span>
                            </Show>
                            <Show when={tokenRevoked}>
                              <div class="mt-2 text-xs text-amber-600 dark:text-amber-300">
                                Token revoked {tokenRevokedRelative}
                              </div>
                            </Show>
                          </td>
                          <td class="py-3 px-4 align-top">
                            <div class="flex flex-wrap items-center gap-2">
                              <span
                                class={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${isOnline
                                  ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                  : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
                                  }`}
                              >
                                {host.status || 'unknown'}
                              </span>
                              <Show when={commandInProgress}>
                                <span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300">
                                  <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                                  </svg>
                                  Stopping
                                </span>
                              </Show>
                              <Show when={commandFailed}>
                                <span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300">
                                  <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 5.636l-12.728 12.728M5.636 5.636l12.728 12.728" />
                                  </svg>
                                  Failed
                                </span>
                              </Show>
                              <Show when={commandCompleted}>
                                <span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">
                                  <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                  </svg>
                                  Stopped
                                </span>
                              </Show>
                            </div>
                            <Show when={removalStatusInfo}>
                              {(info) => {
                                const details = info();
                                return (
                                  <div class={`mt-2 text-xs ${removalTextClassMap[details.tone]}`}>
                                    {details.label}
                                  </div>
                                );
                              }}
                            </Show>
                          </td>
                          <td class="py-3 px-4 align-top">
                            <div class="text-sm font-medium text-gray-900 dark:text-gray-100">
                              {describeRuntime(host)}
                            </div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">
                              Agent {host.agentVersion ? `v${host.agentVersion}` : 'not reporting'}
                              <Show when={host.intervalSeconds}>
                                <span> (every {host.intervalSeconds}s)</span>
                              </Show>
                            </div>
                          </td>
                          <td class="py-3 px-4 align-top">
                            <div class="text-gray-900 dark:text-gray-100">
                              {host.lastSeen ? formatRelativeTime(host.lastSeen) : '—'}
                            </div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">
                              {host.lastSeen ? formatAbsoluteTime(host.lastSeen) : 'Awaiting first report'}
                            </div>
                          </td>
                          <td class="py-3 px-4 text-right align-top">
                            <Show
                              when={host.hidden}
                              fallback={
                                <>
                                  <Show when={commandInProgress || commandCompleted}>
                                    <button
                                      type="button"
                                      class="text-xs font-semibold text-blue-600 disabled:cursor-not-allowed disabled:opacity-50"
                                      disabled
                                    >
                                      {commandCompleted ? 'Cleaning up…' : 'Stopping…'}
                                    </button>
                                  </Show>
                                  <Show
                                    when={commandFailed}
                                    fallback={
                                      <Show
                                        when={!isOnline}
                                        fallback={
                                          <button
                                            type="button"
                                            class="text-xs font-semibold text-red-600 hover:text-red-700 disabled:cursor-not-allowed disabled:opacity-50"
                                            onClick={() => openRemoveModal(host)}
                                            disabled={isRemovingHost(host.id)}
                                          >
                                            {isRemovingHost(host.id) ? 'Working…' : 'Remove'}
                                          </button>
                                        }
                                      >
                                        <button
                                          type="button"
                                          class="text-xs font-semibold text-blue-600 hover:text-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
                                          onClick={() => handleCleanupOfflineHost(host.id, displayName)}
                                          disabled={isRemovingHost(host.id)}
                                        >
                                          {isRemovingHost(host.id) ? 'Cleaning up…' : offlineActionLabel}
                                        </button>
                                      </Show>
                                    }
                                  >
                                    <button
                                      type="button"
                                      class="text-xs font-semibold text-red-600 hover:text-red-700 disabled:cursor-not-allowed disabled:opacity-50"
                                      onClick={() => handleCleanupOfflineHost(host.id, displayName)}
                                      disabled={isRemovingHost(host.id)}
                                    >
                                      {isRemovingHost(host.id) ? 'Removing…' : 'Force remove'}
                                    </button>
                                  </Show>
                                </>
                              }
                            >
                              <button
                                type="button"
                                class="text-xs font-semibold text-blue-600 hover:text-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
                                onClick={() => handleUnhideHost(host.id, displayName)}
                                disabled={isRemovingHost(host.id)}
                              >
                                {isRemovingHost(host.id) ? 'Unhiding…' : 'Unhide'}
                              </button>
                            </Show>
                          </td>
                        </tr>
                      );
                    }}
                  </For>
                </tbody>
              </table>
            </div>
          </Show>
        </div>
      </Card>
    </div>
  );
};
