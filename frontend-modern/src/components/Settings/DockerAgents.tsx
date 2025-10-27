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

export const DockerAgents: Component = () => {
  const { state } = useWebSocket();

  let hasLoggedSecurityStatusError = false;

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
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [isGeneratingToken, setIsGeneratingToken] = createSignal(false);
  const [currentToken, setCurrentToken] = createSignal<string | null>(null);
  const [latestRecord, setLatestRecord] = createSignal<APITokenRecord | null>(null);
  const [tokenName, setTokenName] = createSignal('');

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

  const showInstallCommand = () => !requiresToken() || Boolean(currentToken());

  // Find the token record that matches the currently stored token
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

  const handleGenerateToken = async () => {
    if (isGeneratingToken()) return;
    setIsGeneratingToken(true);
    try {
      const name = tokenName().trim() || `Docker host ${new Date().toISOString().slice(0, 10)}`;
      const { token, record } = await SecurityAPI.createToken(name, [DOCKER_REPORT_SCOPE]);

      setCurrentToken(token);
      setLatestRecord(record);
      setTokenName('');
      notificationStore.success('Token generated and inserted into the command below.', 4000);
    } catch (err) {
      console.error('Failed to generate API token', err);
      notificationStore.error('Failed to generate API token. Confirm you are signed in as an administrator.', 6000);
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
        <Card padding="lg" class="space-y-5">
          <div class="space-y-1">
            <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Add a Docker host</h3>
            <p class="text-sm text-gray-600 dark:text-gray-400">
              Run this command as root on your Docker host to start monitoring.
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
                  Run as root on your Docker host. The installer downloads the agent, creates a systemd service, and starts reporting automatically.
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
                      systemctl enable --now pulse-docker-agent
                    </code>
                  </div>
                </div>
              </div>
            </div>
          </details>
        </Card>

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
                          class="self-start rounded-lg bg-gray-800 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-gray-700"
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
                class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
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

          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                Reporting Docker hosts ({dockerHosts().length})
              </h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Use this list to enroll or retire agents. For live health and troubleshooting, open the Docker monitoring view.
              </p>
            </div>
            <div class="flex flex-wrap items-center justify-end gap-2">
              <a
                href="/docker"
                class="inline-flex items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-1.5 text-xs font-semibold text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-600/60 dark:bg-blue-900/30 dark:text-blue-200 dark:hover:bg-blue-900/50"
              >
                <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
                </svg>
                Open Docker monitoring
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
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Agent &amp; Docker</th>
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
                      const commandInProgress =
                        commandStatus === 'queued' ||
                        commandStatus === 'dispatched' ||
                        commandStatus === 'acknowledged';
                      const commandFailed = commandStatus === 'failed';
                      const commandCompleted = commandStatus === 'completed';
                      const offlineActionLabel = commandFailed
                        ? 'Force remove host'
                        : host.pendingUninstall
                          ? 'Clean up pending host'
                          : 'Remove offline host';
                      const tokenRevoked = typeof host.tokenRevokedAt === 'number';
                      const tokenRevokedRelative = tokenRevoked ? formatRelativeTime(host.tokenRevokedAt!) : '';

                      return (
                        <tr class={`${isOnline ? 'bg-white dark:bg-gray-900' : 'bg-gray-50 dark:bg-gray-800/50 opacity-60'}`}>
                          <td class="py-3 px-4 align-top">
                            <div class="font-medium text-gray-900 dark:text-gray-100">{displayName}</div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">{host.hostname}</div>
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
                            <Show when={host.pendingUninstall}>
                              <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                                Stop command queued; waiting for acknowledgement.
                              </div>
                            </Show>
                          </td>
                          <td class="py-3 px-4 align-top">
                            <div class="text-sm font-medium text-gray-900 dark:text-gray-100">
                              {host.dockerVersion ? `Docker ${host.dockerVersion}` : 'Docker version unavailable'}
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
