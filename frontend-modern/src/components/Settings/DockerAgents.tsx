import { Component, createSignal, Show, For, onCleanup, onMount, createEffect, on, createMemo } from 'solid-js';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import { MonitoringAPI } from '@/api/monitoring';
import { notificationStore } from '@/stores/notifications';
import type { SecurityStatus } from '@/types/config';
import { SecurityAPI, type APITokenRecord } from '@/api/security';
import type { DockerHost } from '@/types/api';
import { showTokenReveal } from '@/stores/tokenReveal';

export const DockerAgents: Component = () => {
  const { state } = useWebSocket();
  const [showInstructions, setShowInstructions] = createSignal(true);

  let hasLoggedSecurityStatusError = false;
  let hasLoggedTokenAuthWarning = false;
  let hasNotifiedTokenLoadError = false;
  let hasNotifiedTokenAuthFailure = false;
  let previousTokenStrategy: 'existing' | 'generate' | null = null;

  const [showHidden, setShowHidden] = createSignal(false);

  const dockerHosts = () => {
    const all = state.dockerHosts || [];
    return showHidden() ? all : all.filter(host => !host.hidden);
  };

  const hiddenCount = () => (state.dockerHosts || []).filter(host => host.hidden).length;

  const pendingHosts = () =>
    dockerHosts().filter(host => {
      const status = host.command?.status;
      if (status === 'queued' || status === 'dispatched' || status === 'acknowledged') return true;
      return Boolean(host.pendingUninstall);
    });

  const [removingHostId, setRemovingHostId] = createSignal<string | null>(null);
  const [showRemoveModal, setShowRemoveModal] = createSignal(false);
  const [hostToRemoveId, setHostToRemoveId] = createSignal<string | null>(null);
  const [uninstallCommandCopied, setUninstallCommandCopied] = createSignal(false);
  const [removeActionLoading, setRemoveActionLoading] = createSignal<'queue' | 'force' | 'hide' | null>(null);
  const [showAdvancedOptions, setShowAdvancedOptions] = createSignal(false);
  const [apiToken, setApiToken] = createSignal<string | null>(null);
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [availableTokens, setAvailableTokens] = createSignal<APITokenRecord[]>([]);
  const [loadingTokens, setLoadingTokens] = createSignal(false);
  const [tokensLoaded, setTokensLoaded] = createSignal(false);
  const [tokensError, setTokensError] = createSignal(false);
  const [isGeneratingToken, setIsGeneratingToken] = createSignal(false);
  const [tokenAccessDenied, setTokenAccessDenied] = createSignal(false);
  const [selectedTokenStrategy, setSelectedTokenStrategy] = createSignal<'existing' | 'generate' | null>(null);
  const [showGenerateTokenModal, setShowGenerateTokenModal] = createSignal(false);
  const [newTokenName, setNewTokenName] = createSignal('');

  const pulseUrl = () => {
    if (typeof window !== 'undefined') {
      const protocol = window.location.protocol;
      const hostname = window.location.hostname;
      const port = window.location.port;
      return `${protocol}//${hostname}${port ? `:${port}` : ''}`;
    }
    return 'http://localhost:7655';
  };

  const TOKEN_PLACEHOLDER = '<api-token>';

  const hostToRemove = createMemo(() => {
    const id = hostToRemoveId();
    if (!id) return null;
    return (state.dockerHosts || []).find(host => host.id === id) ?? null;
  });

  const getDisplayName = (host: DockerHost | { id: string; displayName?: string | null; hostname?: string | null }) => {
    return host.displayName || host.hostname || host.id;
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

  createEffect(() => {
    if (!showRemoveModal()) return;
    const id = hostToRemoveId();
    const host = hostToRemove();
    if (id && !host) {
      closeRemoveModal();
    }
  });

  onMount(() => {
    if (typeof window === 'undefined') {
      return;
    }

    const readToken = () => window.localStorage.getItem('apiToken');
    const currentToken = readToken();
    if (currentToken) {
      setApiToken(currentToken);
    }

    const handleStorage = (event: StorageEvent) => {
      if (event.key === 'apiToken') {
        setApiToken(event.newValue);
      }
    };

    window.addEventListener('storage', handleStorage);
    onCleanup(() => window.removeEventListener('storage', handleStorage));

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
          console.error('Failed to load security status', err);
        }
      }
    };
    fetchSecurityStatus();
  });

  const loadTokens = async () => {
    if (loadingTokens()) return;
    setLoadingTokens(true);
    setTokensError(false);
    try {
      const tokens = await SecurityAPI.listTokens();
      const sorted = [...tokens].sort(
        (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
      );
      setAvailableTokens(sorted);
      setTokenAccessDenied(false);
      hasNotifiedTokenAuthFailure = false;
      hasNotifiedTokenLoadError = false;
      setTokensLoaded(true);
    } catch (err) {
      setTokensError(true);
      if (err instanceof Error && /authentication required/i.test(err.message)) {
        setTokenAccessDenied(true);
        if (!hasLoggedTokenAuthWarning) {
          hasLoggedTokenAuthWarning = true;
          console.debug('API token listing requires authentication.');
        }
        if (!hasNotifiedTokenAuthFailure) {
          hasNotifiedTokenAuthFailure = true;
          notificationStore.error('Authentication required to list API tokens', 6000);
        }
      } else {
        if (!hasNotifiedTokenLoadError) {
          hasNotifiedTokenLoadError = true;
          console.error('Failed to load API tokens', err);
          notificationStore.error('Failed to load API tokens', 6000);
        }
      }
    } finally {
      setLoadingTokens(false);
    }
  };

  const retryTokenLoad = () => {
    if (loadingTokens()) return;
    notificationStore.info('Retrying API token load…', 4000);
    setTokensLoaded(false);
    void loadTokens();
  };

  createEffect(on(
    () => showInstructions(),
    (isShowing) => {
      if (isShowing && !tokensLoaded() && !loadingTokens()) {
        void loadTokens();
      }
    }
  ));

  const hasStoredToken = () => Boolean(apiToken());
  const canGenerateToken = () => !tokenAccessDenied();
  const commandReady = () => !requiresToken() || (!!apiToken() && selectedTokenStrategy() !== null);

  // Find the token record that matches the currently stored token
  const storedTokenRecord = () => {
    const hint = securityStatus()?.apiTokenHint;
    if (!hint) return null;

    return availableTokens().find(token => {
      const tokenHint = `${token.prefix}...${token.suffix}`;
      return tokenHint === hint;
    });
  };

  createEffect(on(
    () => [requiresToken(), apiToken(), selectedTokenStrategy()],
    ([requiresToken, hasToken, strategy]) => {
      if (!requiresToken) {
        setSelectedTokenStrategy('existing');
        return;
      }

      if (!hasToken && strategy === 'existing') {
        setSelectedTokenStrategy(null);
      }
    }
  ));

  const copyToClipboard = async (text: string): Promise<boolean> => {
    try {
      if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
        return true;
      }

      if (typeof document === 'undefined') {
        return false;
      }

      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.style.position = 'fixed';
      textarea.style.left = '-999999px';
      textarea.style.top = '-999999px';
      document.body.appendChild(textarea);
      textarea.focus();
      textarea.select();

      try {
        return document.execCommand('copy');
      } finally {
        document.body.removeChild(textarea);
      }
    } catch (err) {
      console.error('Failed to copy to clipboard', err);
      return false;
    }
  };

  const handleSelectExistingToken = () => {
    if (!hasStoredToken()) {
      if (typeof window !== 'undefined' && window.showToast) {
        window.showToast('warning', 'No saved token found in this browser. Generate a new token instead.');
      }
      return;
    }

    setSelectedTokenStrategy('existing');
    if (typeof window !== 'undefined' && window.showToast) {
      window.showToast('success', 'Install command updated with your saved token.');
    }
  };

  const openGenerateTokenFlow = () => {
    if (!canGenerateToken()) {
      if (typeof window !== 'undefined' && window.showToast) {
        window.showToast('error', 'Sign in with an administrator account to generate tokens here.');
      }
      return;
    }

    const currentTokens = availableTokens();
    const defaultName = `Docker host ${currentTokens.length + 1}`;
    previousTokenStrategy = selectedTokenStrategy();
    setSelectedTokenStrategy('generate');
    setNewTokenName(defaultName);
    setShowGenerateTokenModal(true);
  };

  const handleCreateToken = async () => {
    if (isGeneratingToken()) return;
    if (!canGenerateToken()) {
      notificationStore.error('You need administrator access to create API tokens from here.', 6000);
      return;
    }

    setIsGeneratingToken(true);
    try {
      const currentTokens = availableTokens();
      const defaultName = `Docker host ${currentTokens.length + 1}`;
      const desiredName = newTokenName().trim() || defaultName;
      const { token, record } = await SecurityAPI.createToken(desiredName);

      // Update the tokens list with the new token
      const filtered = currentTokens.filter((t) => t.id !== record.id);
      setAvailableTokens([record, ...filtered]);

      setApiToken(token);
      setSelectedTokenStrategy('generate');
      previousTokenStrategy = 'generate';
      setShowGenerateTokenModal(false);
      setNewTokenName('');
      showTokenReveal({
        token,
        record,
        source: 'docker',
        note: 'Copy this token into the install command for your Docker agent or other automation.',
      });
      if (typeof window !== 'undefined') {
        try {
          window.localStorage.setItem('apiToken', token);
          window.dispatchEvent(new StorageEvent('storage', { key: 'apiToken', newValue: token }));
        } catch (err) {
          console.warn('Unable to persist API token in localStorage', err);
        }
      }
      notificationStore.success('New API token generated and added to the install command.', 6000);
    } catch (err) {
      console.error('Failed to generate API token', err);
      notificationStore.error('Failed to generate API token', 6000);
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
    if (!requiresToken()) {
      return `curl -fsSL ${url}/install-docker-agent.sh | bash -s -- --url ${url} --token disabled`;
    }
    return `curl -fsSL ${url}/install-docker-agent.sh | bash -s -- --url ${url} --token ${TOKEN_PLACEHOLDER}`;
  };

  const getUninstallCommand = () => {
    const url = pulseUrl();
    return `curl -fsSL ${url}/install-docker-agent.sh | bash -s -- --uninstall`;
  };

  const getSystemdService = () => {
    const token = requiresToken() ? TOKEN_PLACEHOLDER : 'disabled';
    return `[Unit]
Description=Pulse Docker Agent
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pulse-docker-agent --url ${pulseUrl()} --token ${token} --interval 30s
Restart=always
RestartSec=5s
User=root

[Install]
WantedBy=multi-user.target`;
  };

  const isRemovingHost = (hostId: string) => removingHostId() === hostId;

  const openRemoveModal = (host: DockerHost) => {
    setHostToRemoveId(host.id);
    setUninstallCommandCopied(false);
    setRemoveActionLoading(null);
    setShowAdvancedOptions(false);
    setShowRemoveModal(true);
  };

  const closeRemoveModal = () => {
    setShowRemoveModal(false);
    setHostToRemoveId(null);
    setUninstallCommandCopied(false);
    setRemoveActionLoading(null);
    setShowAdvancedOptions(false);
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
    } catch (error) {
      console.error('Failed to queue Docker host stop command', error);
      const message = error instanceof Error ? error.message : 'Failed to send stop command';
      notificationStore.error(message, 8000);
    } finally {
      setRemovingHostId(null);
      setRemoveActionLoading(null);
    }
  };

  const handleHideHostFromModal = async () => {
    const host = hostToRemove();
    if (!host || removeActionLoading()) return;

    const displayName = getDisplayName(host);
    setRemovingHostId(host.id);
    setRemoveActionLoading('hide');

    try {
      await MonitoringAPI.deleteDockerHost(host.id, { hide: true });
      notificationStore.success(`Hidden Docker host ${displayName}`, 3500);
      closeRemoveModal();
    } catch (error) {
      console.error('Failed to hide Docker host', error);
      const message = error instanceof Error ? error.message : 'Failed to hide Docker host';
      notificationStore.error(message, 8000);
    } finally {
      setRemovingHostId(null);
      setRemoveActionLoading(null);
    }
  };

  const handleRemoveHostNow = async () => {
    const host = hostToRemove();
    if (!host || removeActionLoading()) return;

    const displayName = getDisplayName(host);
    setRemovingHostId(host.id);
    setRemoveActionLoading('force');

    try {
      await MonitoringAPI.deleteDockerHost(host.id, { force: true });
      notificationStore.success(`Removed Docker host ${displayName}`, 3500);
      closeRemoveModal();
    } catch (error) {
      console.error('Failed to remove Docker host', error);
      const message = error instanceof Error ? error.message : 'Failed to remove Docker host';
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
      notificationStore.success(`Removed Docker host ${displayName}`, 3500);
      if (hostToRemoveId() === hostId) {
        closeRemoveModal();
      }
    } catch (error) {
      console.error('Failed to remove Docker host', error);
      const message = error instanceof Error ? error.message : 'Failed to remove Docker host';
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
      notificationStore.success(`Unhidden Docker host ${displayName}`, 3500);
    } catch (error) {
      console.error('Failed to unhide Docker host', error);
      const message = error instanceof Error ? error.message : 'Failed to unhide Docker host';
      notificationStore.error(message, 8000);
    } finally {
      setRemovingHostId(null);
    }
  };

  return (
    <div class="space-y-6">
      <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <SectionHeader title="Docker agent monitoring" size="md" class="flex-1" />
        <button
          type="button"
          onClick={() => setShowInstructions(!showInstructions())}
          class="px-4 py-2 text-sm font-medium text-blue-700 dark:text-blue-300 bg-blue-50 dark:bg-blue-900/30 rounded-lg hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors"
        >
          {showInstructions() ? 'Hide' : 'Show'} deployment instructions
        </button>
      </div>

      {/* Deployment Instructions */}
      <Show when={showInstructions()}>
        <Card class="space-y-5">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Add a Docker host</h3>
            <p class="text-sm text-gray-600 dark:text-gray-400">
              Run this command as root on your Docker host to start monitoring.
            </p>
          </div>

          <details class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-700 dark:border-gray-700 dark:bg-gray-900/40 dark:text-gray-300">
            <summary class="flex cursor-pointer items-center justify-between font-semibold text-gray-800 dark:text-gray-100">
              <span>What exactly gets installed?</span>
              <svg class="h-4 w-4 text-gray-500 transition-transform group-open:-rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
              </svg>
            </summary>
            <div class="mt-3 space-y-2">
              <ul class="list-disc space-y-1 pl-5 leading-snug">
                <li>A single self-contained Go binary (<code class="font-mono text-[11px]">pulse-docker-agent</code>, ~7&nbsp;MB)</li>
                <li>A systemd unit on Linux or Unraid startup script so the agent restarts after reboots</li>
                <li>No extra dependencies: it talks directly to <code class="font-mono text-[11px]">/var/run/docker.sock</code> and sends metrics over HTTPS</li>
                <li>Every report includes a control handshake so Pulse can issue constrained commands (e.g. stop) without running arbitrary shell</li>
              </ul>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                Removing a host tears down the service and autostart hook automatically; keeping the binary for quick reinstalls is optional and called out in the dialog.
              </p>
            </div>
          </details>

          <Show when={requiresToken()}>
            <div class="space-y-4">
              <div class="space-y-1">
                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">Step 1 · Choose an API token</p>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  Use the token saved in this browser or create a new credential just for this Docker host. The choice will populate the install command automatically.
                </p>
              </div>

              <Show when={loadingTokens()}>
                <div class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
                  Loading API tokens…
                </div>
              </Show>

              <Show when={tokensError()}>
                <div class="space-y-2 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-900/30 dark:text-red-200">
                  <p>
                    {tokenAccessDenied()
                      ? 'Authentication required to list API tokens. Sign in with an administrator account, then try again.'
                      : 'Failed to load API tokens. Please try again.'}
                  </p>
                  <div class="flex flex-wrap items-center gap-3">
                    <button
                      type="button"
                      onClick={retryTokenLoad}
                      class="inline-flex items-center rounded bg-red-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-60 disabled:cursor-not-allowed"
                      disabled={loadingTokens()}
                    >
                      Retry
                    </button>
                    <Show when={!tokenAccessDenied()}>
                      <span class="text-xs text-red-700 dark:text-red-300">
                        Still failing? Check network connectivity and server logs.
                      </span>
                    </Show>
                  </div>
                </div>
              </Show>

              <div class="grid gap-3 sm:grid-cols-2">
                <button
                  type="button"
                  onClick={handleSelectExistingToken}
                  disabled={!hasStoredToken()}
                  class={`relative flex flex-col gap-2 p-4 text-left rounded-lg border transition shadow-sm ${
                    selectedTokenStrategy() === 'existing'
                      ? 'border-blue-500 ring-2 ring-blue-200 dark:ring-blue-400/40 bg-blue-50/40 dark:bg-blue-900/10'
                      : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 hover:border-blue-300 dark:hover:border-blue-500 hover:bg-blue-50/20 dark:hover:bg-blue-900/10'
                  } disabled:opacity-60 disabled:cursor-not-allowed`}
                >
                  <div class="flex items-start justify-between gap-3">
                    <div>
                      <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">Option 1</p>
                      <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">Use saved token</h4>
                    </div>
                    <svg class="w-5 h-5 text-gray-400 dark:text-gray-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                    </svg>
                  </div>
                  <p class="text-sm text-gray-600 dark:text-gray-400">
                    Fill the install command with the API token already stored in this browser.
                  </p>
                  <Show when={hasStoredToken()}>
                    <div class="mt-2 space-y-1.5">
                      <div class="flex items-center gap-2 text-xs text-green-700 dark:text-green-300">
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <span>Saved token detected</span>
                      </div>
                      <Show when={storedTokenRecord()}>
                        <div class="pl-6 text-xs text-gray-700 dark:text-gray-300">
                          <span class="font-medium">{storedTokenRecord()?.name}</span>
                          <Show when={securityStatus()?.apiTokenHint}>
                            {' '}·{' '}
                            <code class="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 rounded font-mono text-[11px] text-gray-600 dark:text-gray-400">
                              {securityStatus()?.apiTokenHint}
                            </code>
                          </Show>
                        </div>
                      </Show>
                    </div>
                  </Show>
                  <Show when={!hasStoredToken()}>
                    <p class="mt-2 text-xs text-gray-500 dark:text-gray-500">
                      No saved token found on this device.
                    </p>
                  </Show>
                </button>

                <button
                  type="button"
                  onClick={openGenerateTokenFlow}
                  disabled={!canGenerateToken()}
                  class={`relative flex flex-col gap-2 p-4 text-left rounded-lg border transition shadow-sm ${
                    selectedTokenStrategy() === 'generate'
                      ? 'border-blue-500 ring-2 ring-blue-200 dark:ring-blue-400/40 bg-blue-50/40 dark:bg-blue-900/10'
                      : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 hover:border-blue-300 dark:hover:border-blue-500 hover:bg-blue-50/20 dark:hover:bg-blue-900/10'
                  } disabled:opacity-60 disabled:cursor-not-allowed`}
                >
                  <div class="flex items-start justify-between gap-3">
                    <div>
                      <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">Option 2</p>
                      <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">Generate new token</h4>
                    </div>
                    <span class="inline-flex items-center px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                      Recommended
                    </span>
                  </div>
                  <p class="text-sm text-gray-600 dark:text-gray-400">
                    Create a fresh API token for this host and insert it into the install command automatically.
                  </p>
                  <Show when={!canGenerateToken()}>
                    <p class="mt-2 text-xs text-amber-600 dark:text-amber-400">
                      Sign in with an administrator account to create tokens in the browser.
                    </p>
                  </Show>
                </button>
              </div>

              <div class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-700 dark:bg-gray-800/50">
                <div class="flex items-start gap-3">
                  <svg class="w-5 h-5 text-gray-600 dark:text-gray-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <div class="flex-1 text-sm text-gray-700 dark:text-gray-300">
                    <strong class="text-gray-900 dark:text-gray-100">Best practice:</strong> Give each Docker host its own API token. You can audit or revoke tokens anytime from{' '}
                    <a href="/settings/security" class="text-blue-600 dark:text-blue-400 underline hover:no-underline font-medium">
                      Security Settings
                    </a>
                    .
                  </div>
                </div>
              </div>

              <Show when={!commandReady()}>
                <div class="rounded-lg border border-dashed border-blue-200 bg-blue-50/60 px-4 py-3 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-200">
                  Pick an option above to unlock the install command.
                </div>
              </Show>
              <Show when={!hasStoredToken()}>
                <div class="rounded-lg border border-yellow-200 bg-yellow-50 px-4 py-3 text-xs text-yellow-800 dark:border-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-200">
                  Need a reusable token? Generate one here or visit <a href="/settings/security" class="underline hover:no-underline font-medium">Security Settings</a> to manage tokens across users.
                </div>
              </Show>
            </div>
          </Show>

          <Show when={commandReady()}>
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Step 2 · Install command</label>
                <button
                  type="button"
                  onClick={async () => {
                    const command = getInstallCommandTemplate().replace(TOKEN_PLACEHOLDER, apiToken() || TOKEN_PLACEHOLDER);
                    const success = await copyToClipboard(command);
                    if (typeof window !== 'undefined' && window.showToast) {
                      window.showToast(success ? 'success' : 'error', success ? 'Copied!' : 'Failed to copy');
                    }
                  }}
                  disabled={requiresToken() && !apiToken()}
                  class="px-3 py-1.5 text-xs font-medium rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed bg-blue-600 text-white hover:bg-blue-700"
                >
                  Copy
                </button>
              </div>
              <div class="relative rounded-lg border-2 border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-900 p-3 overflow-x-auto">
                <code class="text-sm text-gray-900 dark:text-gray-100 font-mono break-all">
                  {getInstallCommandTemplate().replace(TOKEN_PLACEHOLDER, apiToken() || TOKEN_PLACEHOLDER)}
                </code>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                The installer downloads the agent, creates a systemd service, and starts reporting automatically.
              </p>
            </div>
          </Show>

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
                    class="rounded bg-red-50 px-3 py-1 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 dark:bg-red-900/30 dark:text-red-300 dark:hover:bg-red-900/50"
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
                      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pulse-docker-agent ./cmd/pulse-docker-agent
                    </code>
                  </div>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    Building with <code class="font-mono text-[11px]">CGO_ENABLED=0</code> keeps the binary fully static so it runs on hosts with older glibc (e.g. Debian 11).
                  </p>
                  <p class="font-medium text-gray-900 dark:text-gray-100">2. Copy to host</p>
                  <div class="rounded bg-gray-900 p-3 font-mono text-xs text-gray-100 dark:bg-gray-950">
                    <code>
                      scp pulse-docker-agent user@docker-host:/usr/local/bin/
                      <br />
                      ssh user@docker-host chmod +x /usr/local/bin/pulse-docker-agent
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
                      class="absolute right-2 top-2 rounded bg-gray-700 px-3 py-1 text-xs font-medium text-gray-200 transition-colors hover:bg-gray-600"
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
                      systemctl enable --now pulse-docker-agent
                    </code>
                  </div>
                </div>
              </div>
            </div>
          </details>
        </Card>
      </Show>

      {/* Generate token modal */}
      <Show when={showGenerateTokenModal()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div class="w-full max-w-md rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800">
            <div class="space-y-2">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Generate a new Docker API token</h3>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                Pulse will create a scoped token and insert it into the install command. You can manage or revoke tokens anytime from Security Settings.
              </p>
            </div>
            <div class="mt-4 space-y-2">
              <label class="text-sm font-medium text-gray-700 dark:text-gray-300" for="docker-new-token-name">
                Token name
              </label>
              <input
                id="docker-new-token-name"
                type="text"
                value={newTokenName()}
                onInput={(event) => setNewTokenName(event.currentTarget.value)}
                class="w-full rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-900/60"
                placeholder="Docker host token"
              />
              <p class="text-xs text-gray-500 dark:text-gray-400">
                Friendly names make it easier to audit tokens later (e.g. <code class="font-mono text-xs">docker-prod-01</code>).
              </p>
            </div>
            <div class="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => {
                  setShowGenerateTokenModal(false);
                  setNewTokenName('');
                  setSelectedTokenStrategy(previousTokenStrategy);
                  previousTokenStrategy = null;
                }}
                class="rounded px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleCreateToken}
                disabled={isGeneratingToken()}
                class="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-blue-500 dark:hover:bg-blue-400"
              >
                {isGeneratingToken() ? 'Generating…' : 'Generate token'}
              </button>
            </div>
          </div>
        </div>
      </Show>

      {/* Remove Docker Host Modal */}
      <Show when={showRemoveModal()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div class="w-full max-w-2xl rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800">
            <div class="space-y-2">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                Remove Docker host "{modalDisplayName()}"
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
                      disabled={removeActionLoading() !== null || modalCommandInProgress() || modalCommandStatus() === 'completed'}
                      class={`inline-flex items-center justify-center rounded px-4 py-2 text-sm font-medium text-white transition-colors ${
                        modalCommandStatus() === 'completed'
                          ? 'bg-emerald-600 dark:bg-emerald-500'
                          : 'bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-400'
                      } disabled:cursor-not-allowed disabled:opacity-60`}
                    >
                      {(() => {
                        if (removeActionLoading() === 'queue') return 'Sending…';
                        if (modalCommandInProgress()) return 'Waiting for agent…';
                        if (modalCommandStatus() === 'completed') return 'Agent stopped';
                        if (modalCommandFailed()) return 'Retry stop command';
                        return 'Stop agent now';
                      })()}
                    </button>
                    <Show when={modalCommandInProgress()}>
                      <p class="text-xs text-blue-700 dark:text-blue-300">Hang tight—Pulse is waiting for the agent to acknowledge the stop command.</p>
                    </Show>
                    <Show when={modalCommandCompleted()}>
                      <div class="rounded border border-emerald-200 bg-white p-3 text-xs text-emerald-800 dark:border-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-100">
                        <p class="font-medium">
                          Agent confirmed the stop. Pulse has already cleaned up everything it controls:
                        </p>
                        <ul class="mt-2 space-y-1 leading-snug">
                          <li>• Terminated the running <code class="font-mono text-[11px]">pulse-docker-agent</code> process</li>
                          <li>• Disabled future auto-start (stops the systemd unit or removes the Unraid autostart script if one exists)</li>
                          <li>• Cleared the host from the dashboard so new reports won’t appear unexpectedly</li>
                        </ul>
                        <p class="mt-2">
                          The binary remains at <code class="font-mono text-[11px]">/usr/local/bin/pulse-docker-agent</code> for quick reinstalls. Use the uninstall command below if you prefer to remove it too.
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
                                    class={`h-2 w-2 rounded-full ${
                                      step.done
                                        ? 'bg-blue-500'
                                        : step.active
                                          ? 'bg-blue-400 animate-pulse'
                                          : 'bg-gray-300 dark:bg-gray-600'
                                    }`}
                                  />
                                  {step.label}
                                </li>
                              )}
                            </For>
                          </ul>
                          <p class="leading-snug">
                            Pulse responds to the agent's <code class="font-mono text-[11px]">/api/agents/docker/report</code> call with a stop command. The agent disables its service, removes
                            <code class="font-mono text-[11px]">/boot/config/go.d/pulse-docker-agent.sh</code>, and posts back to
                            <code class="font-mono text-[11px]">/api/agents/docker/commands/&lt;id&gt;/ack</code> so Pulse knows it can remove the row.
                          </p>
                        </div>
                      </details>
                    </Show>
                  </div>
                </div>
              </div>

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
                      disabled={removeActionLoading() !== null}
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
                          class="self-start rounded bg-gray-800 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-gray-700"
                        >
                          Copy command
                        </button>
                      </div>
                      <Show when={uninstallCommandCopied()}>
                        <p class="text-[11px] font-medium text-gray-600 dark:text-gray-300">Command copied to clipboard.</p>
                      </Show>
                      <p class="text-[11px] text-gray-500 dark:text-gray-400">
                        This command stops the agent, removes the systemd service (or Unraid autostart hook), deletes <code class="font-mono text-[11px]">/var/log/pulse-docker-agent.log</code>, and uninstalls the binary. Pulse will notice the host is gone after the next heartbeat (≈2 minutes) and clean up the row automatically.
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
                        disabled={removeActionLoading() !== null}
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
                        disabled={removeActionLoading() !== null || modalHostHidden()}
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
                class="rounded px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      </Show>

      {/* Active Docker Hosts */}
      <Card>
        <div class="space-y-4">
          {/* Pending hosts banner */}
          <Show when={pendingHosts().length > 0}>
            <div class="rounded-lg border border-yellow-200 bg-yellow-50 px-4 py-3 dark:border-yellow-700 dark:bg-yellow-900/30">
              <div class="flex items-start gap-3">
                <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div class="flex-1">
                  <h4 class="text-sm font-semibold text-yellow-900 dark:text-yellow-100">
                    Stopping {pendingHosts().length} host{pendingHosts().length !== 1 ? 's' : ''}
                  </h4>
                  <p class="mt-1 text-sm text-yellow-800 dark:text-yellow-200">
                    Pulse has sent the stop command. Once an agent acknowledges (or goes offline), the entry will disappear automatically.
                  </p>
                </div>
              </div>
            </div>
          </Show>

          <div class="flex items-center justify-between">
            <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
              Reporting Docker hosts ({dockerHosts().length})
            </h3>
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
                  No Docker agents are currently reporting.
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
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Containers</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Docker Version</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Agent Version</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Last Seen</th>
                    <th class="py-3 px-4" />
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={dockerHosts()}>
                    {(host) => {
                      const isOnline = host.status?.toLowerCase() === 'online';
                      const runningContainers = host.containers?.filter(c => c.state?.toLowerCase() === 'running').length || 0;
                      const displayName = getDisplayName(host);
                      const commandStatus = host.command?.status ?? null;
                      const commandInProgress = commandStatus === 'queued' || commandStatus === 'dispatched' || commandStatus === 'acknowledged';
                      const commandFailed = commandStatus === 'failed';
                      const commandCompleted = commandStatus === 'completed';
                      const offlineActionLabel = commandFailed
                        ? 'Force remove host'
                        : host.pendingUninstall
                          ? 'Clean up pending host'
                          : 'Remove offline host';

                      return (
                        <tr class={`${isOnline ? 'bg-white dark:bg-gray-900' : 'bg-gray-50 dark:bg-gray-800/50 opacity-60'}`}>
                          <td class="py-3 px-4">
                            <div class="font-medium text-gray-900 dark:text-gray-100">{host.displayName}</div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">{host.hostname}</div>
                            <Show when={host.os || host.architecture}>
                              <div class="text-xs text-gray-400 dark:text-gray-500 mt-1">
                                {host.os}
                                <Show when={host.os && host.architecture}>
                                  <span class="mx-1">•</span>
                                </Show>
                                {host.architecture}
                              </div>
                            </Show>
                          </td>
                          <td class="py-3 px-4">
                            <div class="flex items-center gap-2">
                              <span
                                class={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${
                                  isOnline
                                    ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                    : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
                                }`}
                              >
                                {host.status || 'unknown'}
                              </span>
                              <Show when={commandInProgress}>
                                <span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300">
                                  <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                                  </svg>
                                  Stopping
                                </span>
                              </Show>
                              <Show when={commandFailed}>
                                <span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300">
                                  <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 5.636l-12.728 12.728M5.636 5.636l12.728 12.728" />
                                  </svg>
                                  Failed
                                </span>
                              </Show>
                              <Show when={commandCompleted}>
                                <span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">
                                  <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                  </svg>
                                  Stopped
                                </span>
                              </Show>
                            </div>
                          </td>
                          <td class="py-3 px-4">
                            <div class="text-gray-900 dark:text-gray-100">
                              {runningContainers} / {host.containers?.length || 0}
                            </div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">running</div>
                          </td>
                          <td class="py-3 px-4">
                            <div class="text-gray-900 dark:text-gray-100">{host.dockerVersion || '—'}</div>
                          </td>
                          <td class="py-3 px-4">
                            <div class="text-gray-900 dark:text-gray-100">{host.agentVersion || '—'}</div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">
                              every {host.intervalSeconds || 0}s
                            </div>
                          </td>
                          <td class="py-3 px-4">
                            <div class="text-gray-900 dark:text-gray-100">
                              {host.lastSeen ? formatRelativeTime(host.lastSeen) : '—'}
                            </div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">
                              {host.lastSeen ? formatAbsoluteTime(host.lastSeen) : '—'}
                            </div>
                          </td>
                          <td class="py-3 px-4 text-right">
                            <Show
                              when={host.hidden}
                              fallback={
                                <>
                                  <Show when={commandInProgress || commandCompleted}>
                                    <button
                                      type="button"
                                      class="text-xs font-semibold text-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
                                      disabled
                                    >
                                      {commandCompleted ? 'Cleaning up…' : 'Stopping…'}
                                    </button>
                                  </Show>
                                  <Show when={commandFailed} fallback={
                                    <Show
                                      when={!isOnline}
                                      fallback={
                                        <button
                                          type="button"
                                          class="text-xs font-semibold text-red-600 hover:text-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                          onClick={() => openRemoveModal(host)}
                                          disabled={isRemovingHost(host.id)}
                                        >
                                          {isRemovingHost(host.id) ? 'Working…' : 'Remove'}
                                        </button>
                                      }
                                    >
                                      <button
                                        type="button"
                                        class="text-xs font-semibold text-blue-600 hover:text-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                        onClick={() => handleCleanupOfflineHost(host.id, displayName)}
                                        disabled={isRemovingHost(host.id)}
                                      >
                                        {isRemovingHost(host.id) ? 'Cleaning up…' : offlineActionLabel}
                                      </button>
                                    </Show>
                                  }>
                                    <button
                                      type="button"
                                      class="text-xs font-semibold text-red-600 hover:text-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
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
                                class="text-xs font-semibold text-blue-600 hover:text-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
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

      {/* Info Cards */}
      <div class="grid gap-4 sm:grid-cols-2">
        <Card tone="info" padding="sm">
          <div class="flex items-start gap-3">
            <div class="flex-shrink-0">
              <svg class="w-5 h-5 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Agent-based monitoring</h4>
              <p class="text-xs text-gray-600 dark:text-gray-400 mt-1">
                Docker hosts run the Pulse agent and push metrics to this server. No inbound firewall rules required.
              </p>
            </div>
          </div>
        </Card>

        <Card tone="warning" padding="sm">
          <div class="flex items-start gap-3">
            <div class="flex-shrink-0">
              <svg class="w-5 h-5 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">Agent requirements</h4>
              <p class="text-xs text-gray-600 dark:text-gray-400 mt-1">
                The agent needs access to the Docker socket (/var/run/docker.sock) and network connectivity to this Pulse instance.
              </p>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
};
