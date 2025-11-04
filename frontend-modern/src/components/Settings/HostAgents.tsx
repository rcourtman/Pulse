import { type Component, For, Show, createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import type { JSX } from 'solid-js';
import { useWebSocket } from '@/App';
import type { Host, HostLookupResponse } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { formatBytes, formatRelativeTime, formatUptime, formatAbsoluteTime } from '@/utils/format';
import { copyToClipboard } from '@/utils/clipboard';
import { getPulseBaseUrl } from '@/utils/url';
import { notificationStore } from '@/stores/notifications';
import { HOST_AGENT_SCOPE } from '@/constants/apiScopes';
import type { SecurityStatus } from '@/types/config';
import type { APITokenRecord } from '@/api/security';
import { SecurityAPI } from '@/api/security';
import { MonitoringAPI } from '@/api/monitoring';
import { logger } from '@/utils/logger';

const TOKEN_PLACEHOLDER = '<api-token>';
const pulseUrl = () => getPulseBaseUrl();

const buildDefaultTokenName = () => {
  const now = new Date();
  const iso = now.toISOString().slice(0, 16); // YYYY-MM-DDTHH:MM
  const stamp = iso.replace('T', ' ').replace(/:/g, '-');
  return `Host agent ${stamp}`;
};

type HostAgentPlatform = 'linux' | 'macos' | 'windows';

const commandsByPlatform: Record<
  HostAgentPlatform,
  {
    title: string;
    description: string;
    snippets: { label: string; command: string; note?: string | JSX.Element }[];
  }
> = {
  linux: {
    title: 'Install on Linux',
    description:
      'The installer downloads the agent binary and configures it as a systemd service.',
    snippets: [
      {
        label: 'Install with systemd',
        command: `curl -fsSL ${pulseUrl()}/install-host-agent.sh | bash -s -- --url ${pulseUrl()} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
        note: (
          <span>
            Automatically installs to <code>/usr/local/bin/pulse-host-agent</code> and creates <code>/etc/systemd/system/pulse-host-agent.service</code>.
          </span>
        ),
      },
    ],
  },
  macos: {
    title: 'Install on macOS',
    description:
      'The installer downloads the universal binary and sets up a launchd service for background monitoring.',
    snippets: [
      {
        label: 'Install with launchd',
        command: `curl -fsSL ${pulseUrl()}/install-host-agent.sh | bash -s -- --url ${pulseUrl()} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
        note: (
          <span>
            Creates <code>~/Library/LaunchAgents/com.pulse.host-agent.plist</code> and starts the agent automatically.
          </span>
        ),
      },
    ],
  },
  windows: {
    title: 'Install on Windows',
    description:
      'Run the PowerShell script to install and configure the host agent as a Windows service with automatic startup.',
    snippets: [
      {
        label: 'Install as Windows Service (PowerShell)',
        command: `irm ${pulseUrl()}/install-host-agent.ps1 | iex`,
        note: (
          <span>
            Run in PowerShell as Administrator. The script will prompt for the Pulse URL and API token, download the agent binary, and install it as a Windows service with automatic startup. The agent runs natively and can access all Windows performance counters.
          </span>
        ),
      },
      {
        label: 'Install with parameters (PowerShell)',
        command: `$env:PULSE_URL="${pulseUrl()}"; $env:PULSE_TOKEN="${TOKEN_PLACEHOLDER}"; irm ${pulseUrl()}/install-host-agent.ps1 | iex`,
        note: (
          <span>
            Non-interactive installation. Set environment variables before running to skip prompts.
          </span>
        ),
      },
    ],
  },
};

const computeHostStaleness = (host: Host) => {
  const intervalSeconds = host.intervalSeconds && host.intervalSeconds > 0 ? host.intervalSeconds : 30;
  const staleThresholdMs = Math.max(intervalSeconds * 1000 * 3, 60_000);
  const lastSeenValue =
    typeof host.lastSeen === 'number'
      ? host.lastSeen
      : Number.isFinite(Number(host.lastSeen))
        ? Number(host.lastSeen)
        : NaN;
  const lastSeenMs = Number.isFinite(lastSeenValue) ? lastSeenValue : null;
  const isStale = lastSeenMs === null ? true : Date.now() - lastSeenMs >= staleThresholdMs;

  return {
    isStale,
    lastSeenMs,
    staleThresholdMs,
  };
};

export const HostAgents: Component = () => {
  const { state } = useWebSocket();

  let hasLoggedSecurityStatusError = false;

  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [latestRecord, setLatestRecord] = createSignal<APITokenRecord | null>(null);
  const [tokenName, setTokenName] = createSignal('');
  const [confirmedNoToken, setConfirmedNoToken] = createSignal(false);
  const [currentToken, setCurrentToken] = createSignal<string | null>(null);
  const [isGeneratingToken, setIsGeneratingToken] = createSignal(false);
  const [lookupValue, setLookupValue] = createSignal('');
  const [lookupResult, setLookupResult] = createSignal<HostLookupResponse | null>(null);
  const [lookupError, setLookupError] = createSignal<string | null>(null);
  const [lookupLoading, setLookupLoading] = createSignal(false);
  const [highlightedHostId, setHighlightedHostId] = createSignal<string | null>(null);
  let highlightTimer: ReturnType<typeof setTimeout> | null = null;
  const [showRemoveModal, setShowRemoveModal] = createSignal(false);
  const [hostToRemoveId, setHostToRemoveId] = createSignal<string | null>(null);
  const [removeActionLoading, setRemoveActionLoading] = createSignal<'remove' | null>(null);
  const [uninstallCommandCopied, setUninstallCommandCopied] = createSignal(false);
  const [uninstallCommandCopiedAt, setUninstallCommandCopiedAt] = createSignal<number | null>(null);
  const [hostRemovalCountdownSeconds, setHostRemovalCountdownSeconds] = createSignal<number | null>(null);
  const [uninstallConfirmed, setUninstallConfirmed] = createSignal(false);

  createEffect(() => {
    if (requiresToken()) {
      setConfirmedNoToken(false);
    } else {
      setCurrentToken(null);
      setLatestRecord(null);
    }
  });


  const allHosts = createMemo(() => {
    const list = state.hosts ?? [];
    return [...list].sort((a, b) => (a.hostname || '').localeCompare(b.hostname || ''));
  });

  const renderTags = (host: Host) => {
    const tags = host.tags ?? [];
    if (!tags.length) return '—';
    return tags.join(', ');
  };

  const commandSections = createMemo(() =>
    Object.entries(commandsByPlatform).map(([platform, meta]) => ({
      platform: platform as HostAgentPlatform,
      ...meta,
    })),
  );

  const connectedFromStatus = (status: string | undefined | null) => {
    if (!status) return false;
    const value = status.toLowerCase();
    return value === 'online' || value === 'running' || value === 'healthy';
  };

  const hostToRemove = createMemo(() => {
    const id = hostToRemoveId();
    if (!id) return null;
    return allHosts().find((host) => host.id === id) ?? null;
  });

  const hostRemovalDisplayName = () => {
    const host = hostToRemove();
    return host ? host.displayName || host.hostname || host.id : '';
  };

  const hostRemovalPlatform = createMemo(() => hostToRemove()?.platform?.toLowerCase() || '');
  const hostRemovalStatus = createMemo(() => {
    const host = hostToRemove();
    return (host?.status || 'unknown').toLowerCase();
  });

  const hostRemovalStatusLabel = () => hostToRemove()?.status || 'unknown';

  const hostRemovalIsOnline = createMemo(() => {
    const status = hostRemovalStatus();
    return status === 'online' || status === 'running' || status === 'healthy';
  });

  const hostRemovalStaleness = createMemo(() => {
    const host = hostToRemove();
    if (!host) {
      return { isStale: false, lastSeenMs: null as number | null, staleThresholdMs: 90_000 };
    }
    return computeHostStaleness(host);
  });

  const hostRemovalIsStale = createMemo(() => hostRemovalStaleness().isStale);

  const hostRemovalLastSeen = createMemo(() => {
    const { lastSeenMs } = hostRemovalStaleness();
    if (!lastSeenMs) return null;
    return {
      relative: formatRelativeTime(lastSeenMs),
      absolute: formatAbsoluteTime(lastSeenMs),
    };
  });

  const hostRemovalStaleThresholdSeconds = createMemo(() => {
    const threshold = hostRemovalStaleness().staleThresholdMs;
    return Math.max(Math.round(threshold / 1000), 0);
  });

  const hostRemovalUninstallCommand = createMemo(() => getHostUninstallCommand(hostToRemove()));

  const hostRemovalUninstallNote = () => {
    const platform = hostRemovalPlatform();
    if (platform === 'macos' || platform === 'darwin' || platform === 'mac') {
      return 'Unloads the launch agent, removes the plist, deletes the binary, and clears the local log.';
    }
    if (platform === 'windows' || platform === 'win32' || platform === 'windows_nt') {
      return 'Stops the Windows service, removes it, and deletes the installed binary and log. Run from an elevated PowerShell window.';
    }
    return 'Stops the agent, removes the systemd unit, deletes the binary, and reloads systemd.';
  };

  const formatCountdown = (seconds: number | null) => {
    if (seconds === null || seconds < 0) return null;
    if (seconds === 0) return 'any moment now';
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    if (mins <= 0) {
      return `${secs}s`;
    }
    if (secs === 0) {
      return `${mins}m`;
    }
    return `${mins}m ${secs}s`;
  };

  const countdownLabel = createMemo(() => formatCountdown(hostRemovalCountdownSeconds()));

  const canRemoveHost = createMemo(() => uninstallCommandCopied() && uninstallConfirmed());

  const openRemoveModal = (host: Host) => {
    setHostToRemoveId(host.id);
    setShowRemoveModal(true);
    setRemoveActionLoading(null);
    setUninstallCommandCopied(false);
    setUninstallCommandCopiedAt(null);
    setHostRemovalCountdownSeconds(null);
    setUninstallConfirmed(false);
  };

  const closeRemoveModal = () => {
    setShowRemoveModal(false);
    setHostToRemoveId(null);
    setRemoveActionLoading(null);
    setUninstallCommandCopied(false);
    setUninstallCommandCopiedAt(null);
    setHostRemovalCountdownSeconds(null);
    setUninstallConfirmed(false);
  };

  const performHostRemoval = async () => {
    const host = hostToRemove();
    if (!host || removeActionLoading()) return;

    setRemoveActionLoading('remove');
    const displayName = host.displayName || host.hostname || host.id;

    try {
      await MonitoringAPI.deleteHostAgent(host.id);
      notificationStore.success(`Host "${displayName}" removed`, 4000);
      closeRemoveModal();
    } catch (error) {
      logger.error('Failed to remove host agent', error);
      const message = error instanceof Error ? error.message : 'Failed to remove host. Please try again.';
      notificationStore.error(message, 6000);
    } finally {
      setRemoveActionLoading(null);
    }
  };

  const handleRemoveHost = () => {
    void performHostRemoval();
  };

  createEffect(() => {
    const current = lookupResult();
    if (!current) return;

    const targetId = current.host.id;
    const targetHostname = current.host.hostname;
    const hosts = allHosts();
    const match = hosts.find((host) => host.id === targetId || host.hostname === targetHostname);
    if (!match) return;

    if (highlightTimer) {
      clearTimeout(highlightTimer);
      highlightTimer = null;
    }

    setHighlightedHostId(match.id);
    highlightTimer = setTimeout(() => {
      setHighlightedHostId(null);
      highlightTimer = null;
    }, 10_000);

    const updated = {
      success: true,
      host: {
        id: match.id,
        hostname: match.hostname,
        displayName: match.displayName,
        status: match.status,
        connected: connectedFromStatus(match.status),
        lastSeen: match.lastSeen ?? Date.now(),
        agentVersion: match.agentVersion ?? current.host.agentVersion,
      },
    } satisfies HostLookupResponse;

    const currentHost = current.host;
    if (
      currentHost.status === updated.host.status &&
      currentHost.connected === updated.host.connected &&
      currentHost.lastSeen === updated.host.lastSeen &&
      currentHost.agentVersion === updated.host.agentVersion &&
      (currentHost.displayName || '') === (updated.host.displayName || '') &&
      currentHost.hostname === updated.host.hostname
    ) {
      return;
    }

    setLookupResult(updated);
    setLookupError(null);
  });

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
      setHostRemovalCountdownSeconds(null);
      return;
    }

    const updateCountdown = () => {
      const host = hostToRemove();
      if (!host) {
        setHostRemovalCountdownSeconds(null);
        return;
      }

      const { lastSeenMs, staleThresholdMs } = computeHostStaleness(host);
      if (!lastSeenMs) {
        setHostRemovalCountdownSeconds(null);
        return;
      }

      const elapsed = Date.now() - lastSeenMs;
      const remaining = staleThresholdMs - elapsed;
      setHostRemovalCountdownSeconds(remaining > 0 ? Math.ceil(remaining / 1000) : 0);
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, 1000);

    return () => {
      clearInterval(interval);
      setHostRemovalCountdownSeconds(null);
    };
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


    const requiresToken = () => {
      const status = securityStatus();
      if (status) {
        return status.requiresAuth || status.apiTokenConfigured;
      }
      return true;
    };

    onCleanup(() => {
      if (highlightTimer) {
        clearTimeout(highlightTimer);
        highlightTimer = null;
      }
    });

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
      const { token, record } = await SecurityAPI.createToken(desiredName, [HOST_AGENT_SCOPE]);

      setCurrentToken(token);
      setLatestRecord(record);
      setTokenName('');
      setConfirmedNoToken(false);
      notificationStore.success('Token generated and inserted into the command below.', 4000);
    } catch (err) {
      logger.error('Failed to generate host agent token', err);
      notificationStore.error('Failed to generate host agent token. Confirm you are signed in as an administrator.', 6000);
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
      setLookupError('Enter a hostname or host ID to check.');
      return;
    }

    setLookupLoading(true);
    try {
      const result = await MonitoringAPI.lookupHost({ id: query, hostname: query });
      if (!result) {
        setLookupResult(null);
        setLookupError(`No host has reported with "${query}" yet. Try again in a few seconds.`);
      } else {
        setLookupResult(result);
        setLookupError(null);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Host lookup failed.';
      setLookupResult(null);
      setLookupError(message);
    } finally {
      setLookupLoading(false);
    }
  };

  const getSystemdServiceUnit = () => `[Unit]
Description=Pulse Host Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pulse-host-agent --url ${pulseUrl()} --token ${resolvedToken()} --interval 30s
Restart=always
RestartSec=5s
User=root

[Install]
WantedBy=multi-user.target`;

  function getManualUninstallCommand(): string {
    return `sudo systemctl stop pulse-host-agent && \\
sudo systemctl disable pulse-host-agent && \\
sudo rm -f /etc/systemd/system/pulse-host-agent.service && \\
sudo rm -f /usr/local/bin/pulse-host-agent && \\
sudo systemctl daemon-reload`;
  }

  function getHostUninstallCommand(host: Host | null): string {
    const platform = host?.platform?.toLowerCase();
    if (platform === 'macos' || platform === 'darwin' || platform === 'mac') {
      return `launchctl unload ~/Library/LaunchAgents/com.pulse.host-agent.plist >/dev/null 2>&1 || true && \\
rm -f ~/Library/LaunchAgents/com.pulse.host-agent.plist && \\
sudo rm -f /usr/local/bin/pulse-host-agent && \\
rm -f ~/Library/Logs/pulse-host-agent.log`;
    }
    if (platform === 'windows' || platform === 'win32' || platform === 'windows_nt') {
      return `Stop-Service -Name PulseHostAgent -ErrorAction SilentlyContinue; \\
sc.exe delete PulseHostAgent; \\
Remove-Item 'C:\\\\Program Files\\\\Pulse\\\\pulse-host-agent.exe' -Force -ErrorAction SilentlyContinue; \\
Remove-Item '$env:ProgramData\\\\Pulse\\\\pulse-host-agent.log' -Force -ErrorAction SilentlyContinue`;
    }
    return getManualUninstallCommand();
  }

  return (
    <div class="space-y-6">
      <Card padding="lg" class="space-y-5">
        <div class="space-y-1">
          <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Add a host agent</h3>
          <p class="text-sm text-gray-600 dark:text-gray-400">
            Generate a token once, then run the matching command on Linux, macOS, or Windows to register new hosts.
          </p>
        </div>

        <div class="space-y-5">
          <Show when={requiresToken()}>
            <div class="space-y-3">
              <div class="space-y-1">
                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">Generate API token</p>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  Create a fresh token scoped to <code>{HOST_AGENT_SCOPE}</code>.
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
                  class="flex-1 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-900/60"
                />
                <button
                  type="button"
                  onClick={handleGenerateToken}
                  disabled={isGeneratingToken()}
                  class="inline-flex items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isGeneratingToken() ? 'Generating…' : hasToken() ? 'Generate another' : 'Generate token'}
                </button>
              </div>

              <Show when={latestRecord()}>
                <div class="flex items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-4 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
                  <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                  </svg>
                  <span>
                    Token <strong>{latestRecord()?.name}</strong> created ({latestRecord()?.prefix}…{latestRecord()?.suffix}). Commands below now include this credential.
                  </span>
                </div>
              </Show>

            </div>
          </Show>

            <Show when={!requiresToken()}>
              <div class="space-y-3">
                <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-900/30 dark:text-amber-200">
                  Tokens are optional on this Pulse instance. Confirm to generate commands without embedding a token.
                </div>
                <button
                  type="button"
                  onClick={acknowledgeNoToken}
                  disabled={confirmedNoToken()}
                  class={`inline-flex items-center justify-center rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                    confirmedNoToken()
                      ? 'bg-green-600 text-white cursor-default'
                      : 'bg-gray-900 text-white hover:bg-black dark:bg-gray-100 dark:text-gray-900 dark:hover:bg-white'
                  }`}
                >
                  {confirmedNoToken() ? 'No token confirmed' : 'Confirm without token'}
                </button>
              </div>
            </Show>

            <Show when={commandsUnlocked()}>
              <div class="space-y-3">
                <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Installation commands</h4>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  Copy the command for the platform you are deploying.
                </p>
                <div class="space-y-4">
                  <For each={commandSections()}>
                    {(section) => (
                      <div class="space-y-3 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
                        <div class="space-y-1">
                          <h5 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{section.title}</h5>
                          <p class="text-xs text-gray-500 dark:text-gray-400">{section.description}</p>
                        </div>
                        <div class="space-y-3">
                          <For each={section.snippets}>
                            {(snippet) => {
                              const copyCommand = () =>
                                snippet.command.replace(TOKEN_PLACEHOLDER, resolvedToken());

                              return (
                                <div class="space-y-2">
                                  <div class="flex items-center justify-between gap-3">
                                    <h6 class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                                      {snippet.label}
                                    </h6>
                                    <button
                                      type="button"
                                      onClick={async () => {
                                        const success = await copyToClipboard(copyCommand());
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
                                    <code>{copyCommand()}</code>
                                  </pre>
                                  <Show when={snippet.note}>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">{snippet.note}</p>
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
                <div class="space-y-3 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-100">
                  <div class="flex items-center justify-between gap-3">
                    <h5 class="text-sm font-semibold">Check installation status</h5>
                    <button
                      type="button"
                      onClick={handleLookup}
                      disabled={lookupLoading()}
                      class="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      {lookupLoading() ? 'Checking…' : 'Check status'}
                    </button>
                  </div>
                  <p class="text-xs text-blue-800 dark:text-blue-200">
                    Enter the hostname (or host ID) from the machine you just installed. Pulse returns the latest status instantly.
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
                      placeholder="Hostname or host ID"
                      class="flex-1 rounded-lg border border-blue-200 bg-white px-3 py-2 text-sm text-blue-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100 dark:focus:border-blue-300 dark:focus:ring-blue-800/60"
                    />
                  </div>
                  <Show when={lookupError()}>
                    <p class="text-xs font-medium text-red-600 dark:text-red-300">{lookupError()}</p>
                  </Show>
                  <Show when={lookupResult()}>
                    {(result) => {
                      const host = () => result().host;
                      const statusBadgeClasses = () =>
                        host().connected
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                          : 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200';
                      return (
                        <div class="space-y-1 rounded-lg border border-blue-200 bg-white px-3 py-2 text-xs text-blue-900 dark:border-blue-700 dark:bg-blue-900/40 dark:text-blue-100">
                          <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                            <div class="text-sm font-semibold">
                              {host().displayName || host().hostname}
                            </div>
                            <div class="flex items-center gap-2">
                              <span class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold ${statusBadgeClasses()}`}>
                                {host().connected ? 'Connected' : 'Not reporting yet'}
                              </span>
                              <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium text-blue-700 dark:bg-blue-900/60 dark:text-blue-200">
                                {host().status || 'unknown'}
                              </span>
                            </div>
                          </div>
                          <div>
                            Last seen {formatRelativeTime(host().lastSeen)} ({formatAbsoluteTime(host().lastSeen)})
                          </div>
                          <Show when={host().agentVersion}>
                            <div class="text-xs text-blue-700 dark:text-blue-200">
                              Agent version {host().agentVersion}
                            </div>
                          </Show>
                        </div>
                      );
                    }}
                  </Show>
                </div>
                <details class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-700 dark:border-gray-700 dark:bg-gray-800/50 dark:text-gray-300">
                  <summary class="cursor-pointer text-sm font-medium text-gray-900 dark:text-gray-100">
                    Advanced options (manual install & uninstall)
                  </summary>
                  <div class="mt-3 space-y-4">
                    <div class="space-y-3 rounded-lg border border-gray-200 bg-white p-3 text-xs dark:border-gray-700 dark:bg-gray-900">
                      <p class="font-medium text-gray-900 dark:text-gray-100">Manual Linux install</p>
                      <p class="text-xs text-gray-500 dark:text-gray-400">
                        Build the agent from source and manage the service yourself instead of using the helper script.
                      </p>
                      <p class="font-medium text-gray-900 dark:text-gray-100">1. Build the binary</p>
                      <div class="rounded bg-gray-900 p-3 font-mono text-xs text-gray-100 dark:bg-gray-950">
                        <code>
                          cd /opt/pulse
                          <br />
                          CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pulse-host-agent ./cmd/pulse-host-agent
                        </code>
                      </div>
                      <p class="font-medium text-gray-900 dark:text-gray-100">2. Copy to host</p>
                      <div class="rounded bg-gray-900 p-3 font-mono text-xs text-gray-100 dark:bg-gray-950">
                        <code>
                          scp pulse-host-agent user@host:/usr/local/bin/
                          <br />
                          ssh user@host sudo chmod +x /usr/local/bin/pulse-host-agent
                        </code>
                      </div>
                      <p class="font-medium text-gray-900 dark:text-gray-100">3. Systemd service template</p>
                      <div class="relative">
                        <button
                          type="button"
                          onClick={async () => {
                            const success = await copyToClipboard(getSystemdServiceUnit());
                            if (typeof window !== 'undefined' && window.showToast) {
                              window.showToast(success ? 'success' : 'error', success ? 'Copied to clipboard' : 'Failed to copy to clipboard');
                            }
                          }}
                          class="absolute right-2 top-2 rounded-lg bg-gray-700 px-3 py-1.5 text-xs font-medium text-gray-200 transition-colors hover:bg-gray-600"
                        >
                          Copy
                        </button>
                        <div class="rounded bg-gray-900 p-3 font-mono text-xs text-gray-100 dark:bg-gray-950 overflow-x-auto">
                          <pre>{getSystemdServiceUnit()}</pre>
                        </div>
                      </div>
                      <p class="font-medium text-gray-900 dark:text-gray-100">4. Enable & start</p>
                      <div class="rounded bg-gray-900 p-3 font-mono text-xs text-gray-100 dark:bg-gray-950">
                        <code>
                          sudo systemctl daemon-reload
                          <br />
                          sudo systemctl enable --now pulse-host-agent
                        </code>
                      </div>
                    </div>
                    <div>
                      <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Manual uninstall</p>
                      <div class="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                        <code class="flex-1 break-all rounded bg-gray-900 px-3 py-2 font-mono text-xs text-gray-100 dark:bg-gray-950">
                          {getManualUninstallCommand()}
                        </code>
                        <button
                          type="button"
                          onClick={async () => {
                            const success = await copyToClipboard(getManualUninstallCommand());
                            if (typeof window !== 'undefined' && window.showToast) {
                              window.showToast(success ? 'success' : 'error', success ? 'Copied to clipboard' : 'Failed to copy to clipboard');
                            }
                          }}
                          class="self-start rounded-lg bg-red-50 px-3 py-1.5 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 dark:bg-red-900/30 dark:text-red-300 dark:hover:bg-red-900/50"
                        >
                          Copy
                        </button>
                      </div>
                      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                        Stops the agent, removes the systemd unit, and deletes the binary.
                      </p>
                    </div>
                  </div>
                </details>
              </div>
            </Show>

            <Show when={requiresToken() && !hasToken()}>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                Generate a new token to unlock the install commands.
              </p>
            </Show>
            <Show when={!requiresToken() && !confirmedNoToken() && !hasToken()}>
              <p class="text-xs text-gray-500 dark:text-gray-400">Confirm the no-token setup to continue.</p>
            </Show>
        </div>
      </Card>

      <Card>
        <div class="space-y-4">
          <div class="flex items-center justify-between">
            <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Reporting hosts</h3>
            <span class="text-sm text-gray-500 dark:text-gray-400">{allHosts().length} connected</span>
          </div>

          <Show
            when={allHosts().length > 0}
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
                  No host agents are reporting yet.
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-500 mt-1">
                  Deploy the agent using the commands above to see hosts listed here.
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
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Platform</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Uptime</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Memory</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Last Seen</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-600 dark:text-gray-400">Tags</th>
                    <th class="py-3 px-4" />
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={allHosts()}>
                    {(host) => {
                      const staleness = computeHostStaleness(host);
                      const isStale = staleness.isStale;
                      const tokenRevokedAt = host.tokenRevokedAt;
                      const tokenRevoked = typeof tokenRevokedAt === 'number';
                      const status = (host.status || 'unknown').toLowerCase();
                      const isOnline =
                        status === 'online' || status === 'running' || status === 'healthy';
                      const isHighlighted = highlightedHostId() === host.id;
                      const isRemovingThisHost = hostToRemoveId() === host.id && removeActionLoading() !== null;

                      const baseRowClass = isStale
                        ? 'bg-gray-50 dark:bg-gray-800/50 opacity-60'
                        : 'bg-white dark:bg-gray-900';

                      const rowClass =
                        tokenRevoked && !isStale ? `${baseRowClass} opacity-60` : baseRowClass;
                      const highlightClass = isHighlighted
                        ? 'ring-2 ring-blue-500/70 dark:ring-blue-400/70 shadow-md'
                        : '';

                      const handleRemoveClick = () => {
                        openRemoveModal(host);
                      };

                      return (
                        <tr data-host-id={host.id} class={`${rowClass} ${highlightClass}`}>
                          <td class="py-3 px-4">
                            <div class="font-medium text-gray-900 dark:text-gray-100">
                              {host.displayName || host.hostname || host.id}
                            </div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">
                              {host.hostname}
                            </div>
                            <Show when={host.agentVersion}>
                              <div class="text-xs text-gray-400 dark:text-gray-500 mt-1">
                                Agent {host.agentVersion}
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
                              <Show when={isStale}>
                                <span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300">
                                  <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                                  </svg>
                                  No recent data
                                </span>
                              </Show>
                              <Show when={tokenRevoked}>
                                <span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-semibold bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                                  <svg class="h-3 w-3" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                    <path
                                      fill-rule="evenodd"
                                      d="M8.257 3.099c.764-1.36 2.722-1.36 3.486 0l6.518 11.62c.75 1.338-.213 3.005-1.743 3.005H3.482c-1.53 0-2.493-1.667-1.743-3.005l6.518-11.62ZM11 5a1 1 0 1 0-2 0v4.5a1 1 0 1 0 2 0V5Zm0 8a1 1 0 1 0-2 0 1 1 0 0 0 2 0Z"
                                      clip-rule="evenodd"
                                    />
                                  </svg>
                                  Token revoked
                                </span>
                              </Show>
                            </div>
                          </td>
                          <td class="py-3 px-4 text-gray-700 dark:text-gray-300 capitalize">
                            {host.platform || '—'}
                          </td>
                          <td class="py-3 px-4 text-gray-700 dark:text-gray-300">
                            {host.uptimeSeconds ? formatUptime(host.uptimeSeconds) : '—'}
                          </td>
                          <td class="py-3 px-4 text-gray-700 dark:text-gray-300">
                            {host.memory?.total
                              ? `${formatBytes(host.memory.used ?? 0)} / ${formatBytes(host.memory.total)}`
                              : '—'}
                          </td>
                          <td class="py-3 px-4">
                            <div class="text-gray-900 dark:text-gray-100">
                              {host.lastSeen ? formatRelativeTime(host.lastSeen) : '—'}
                            </div>
                            <Show when={host.lastSeen}>
                              <div class="text-xs text-gray-500 dark:text-gray-400">
                                {formatAbsoluteTime(host.lastSeen!)}
                              </div>
                            </Show>
                          </td>
                          <td class="py-3 px-4 text-gray-700 dark:text-gray-300">
                            {renderTags(host)}
                          </td>
                          <td class="py-3 px-4 text-right">
                            <button
                              type="button"
                              onClick={handleRemoveClick}
                              disabled={isRemovingThisHost}
                              class="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                              title={
                                isStale
                                  ? 'Remove this stale host entry from the inventory'
                                  : 'Host is still reporting — review the removal steps first'
                              }
                            >
                              {isRemovingThisHost ? (
                                <>
                                  <svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
                                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                                  </svg>
                                  <span>Removing...</span>
                                </>
                              ) : (
                                <>
                                  <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1 1v3M4 7h16" />
                                  </svg>
                                  <span>Remove</span>
                                </>
                              )}
                            </button>
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

      <Show when={showRemoveModal()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div class="w-full max-w-2xl rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800">
            <div class="space-y-2">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                Remove host "{hostRemovalDisplayName()}"
              </h3>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                Walk through uninstalling the agent and cleaning up the entry in Pulse.
              </p>
            </div>

            <div class="mt-4 space-y-4">
              <div class="space-y-3 rounded-lg border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-900/20">
                <div class="flex items-start gap-3">
                  <svg class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <div class="flex-1 space-y-2">
                    <h4 class="text-sm font-semibold text-blue-900 dark:text-blue-100">Step 1 · Stop the agent on {hostRemovalDisplayName()}</h4>
                    <p class="text-sm text-blue-800 dark:text-blue-200">
                      Copy the tailored uninstall script below, run it on the host, then confirm once the command finishes. It runs silently, so no terminal output is expected.
                    </p>
                  </div>
                </div>

                <div class="space-y-2 rounded border border-blue-200 bg-white p-3 text-xs text-blue-800 dark:border-blue-700 dark:bg-blue-800/20 dark:text-blue-200">
                  <div class="flex items-center justify-between gap-2">
                    <span class="font-semibold uppercase tracking-wide text-[11px] text-blue-600 dark:text-blue-300">Manual uninstall</span>
                    <button
                      type="button"
                      onClick={async () => {
                        const command = hostRemovalUninstallCommand();
                        if (!command) return;
                        const success = await copyToClipboard(command);
                        if (success) {
                          setUninstallCommandCopied(true);
                          setUninstallCommandCopiedAt(Date.now());
                        }
                        if (typeof window !== 'undefined' && window.showToast) {
                          window.showToast(success ? 'success' : 'error', success ? 'Copied!' : 'Failed to copy');
                        }
                      }}
                      class="inline-flex items-center gap-2 rounded bg-blue-600 px-3 py-1.5 text-[11px] font-semibold text-white transition-colors hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-400"
                    >
                      <svg class="h-3 w-3" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                        <path d="M4 4a2 2 0 012-2h5a2 2 0 012 2v2h-2V4H6v10h5v-2h2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" />
                        <path d="M14.293 7.293a1 1 0 011.414 0L18 9.586l-2.293 2.293a1 1 0 01-1.414-1.415L14.586 10H9a1 1 0 110-2h5.586l-.293-.293a1 1 0 010-1.414z" />
                      </svg>
                      {uninstallCommandCopied() ? 'Copied' : 'Copy command'}
                    </button>
                  </div>
                  <code class="block overflow-x-auto rounded bg-gray-900 px-3 py-2 font-mono text-xs text-gray-100 dark:bg-gray-950 whitespace-pre-wrap">
                    {hostRemovalUninstallCommand()}
                  </code>
                  <p class="text-[11px] leading-snug">{hostRemovalUninstallNote()}</p>
                  <Show when={uninstallCommandCopied()}>
                    <div class="space-y-2 rounded border border-blue-200 bg-white p-3 text-[11px] text-blue-800 dark:border-blue-700 dark:bg-blue-800/20 dark:text-blue-200">
                      <p class="font-medium">Command copied.</p>
                      <p>This script runs silently—no CLI output is expected. Once it completes, mark it finished below.</p>
                      <Show when={uninstallCommandCopiedAt()}>
                        <p class="text-blue-700/80 dark:text-blue-200/80">Copied {formatRelativeTime(uninstallCommandCopiedAt()!)}.</p>
                      </Show>
                      <button
                        type="button"
                        onClick={() => setUninstallConfirmed(true)}
                        disabled={uninstallConfirmed()}
                        class={`inline-flex items-center gap-2 rounded ${uninstallConfirmed() ? 'bg-blue-100 text-blue-500 cursor-default' : 'bg-blue-600 text-white hover:bg-blue-700'} px-3 py-1.5 text-[11px] font-semibold transition-colors dark:${uninstallConfirmed() ? 'bg-blue-900/30 text-blue-200' : 'bg-blue-500 hover:bg-blue-400'}`}
                      >
                        <svg class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.707a1 1 0 00-1.414-1.414L9 10.172 7.707 8.879A1 1 0 006.293 10.293l2 2a1 1 0 001.414 0l3-3z" clip-rule="evenodd" />
                        </svg>
                        {uninstallConfirmed() ? 'Marked complete' : 'I ran this command'}
                      </button>
                      <Show when={!uninstallConfirmed()}>
                        <p>Click once you have run the script on {hostRemovalDisplayName()}.</p>
                      </Show>
                    </div>
                  </Show>
                </div>
              </div>

              <div class="space-y-3 rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-900">
                <div class="flex items-center justify-between text-xs text-gray-600 dark:text-gray-300">
                  <span class="font-semibold uppercase tracking-wide text-[11px] text-gray-500 dark:text-gray-400">Host status</span>
                  <span
                    class={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase ${
                      hostRemovalIsOnline()
                        ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
                        : 'bg-gray-200 text-gray-700 dark:bg-gray-700 dark:text-gray-200'
                    }`}
                  >
                    {hostRemovalStatusLabel()}
                  </span>
                </div>
                <div class="text-xs text-gray-600 dark:text-gray-300">
                  <div class="flex items-center justify-between">
                    <span class="font-medium">Last heartbeat</span>
                    <span class="text-gray-700 dark:text-gray-200">
                      {hostRemovalLastSeen()?.relative ?? 'No reports yet'}
                    </span>
                  </div>
                  <Show when={hostRemovalLastSeen()}>
                    <div class="text-[11px] text-gray-500 dark:text-gray-400 text-right">
                      {hostRemovalLastSeen()?.absolute}
                    </div>
                  </Show>
                </div>
                <Show when={!hostRemovalIsStale()}>
                  <div class="rounded border border-yellow-200 bg-yellow-50 p-3 text-xs text-yellow-800 dark:border-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-200">
                    <p class="font-semibold">Host still reporting</p>
                    <p class="mt-1 leading-snug">
                      Pulse revokes the host's API token as soon as you remove it. After the uninstall script stops the service, the next heartbeat will fail and the agent will disappear within about {hostRemovalStaleThresholdSeconds()} seconds.
                    </p>
                    <Show when={countdownLabel()}>
                      <p class="mt-2 rounded bg-yellow-100/60 px-2 py-1 text-[11px] font-medium text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-100">
                        Waiting for the next missed heartbeat ({countdownLabel()}).
                      </p>
                    </Show>
                  </div>
                </Show>
                <Show when={hostRemovalIsStale()}>
                  <div class="rounded border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-800 dark:border-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-200">
                    <p class="flex items-center gap-1 font-semibold">
                      <svg class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.707a1 1 0 00-1.414-1.414L9 10.172 7.707 8.879A1 1 0 006.293 10.293l2 2a1 1 0 001.414 0l3-3z" clip-rule="evenodd" />
                      </svg>
                      Host offline
                    </p>
                    <p class="mt-1 leading-snug">
                      Pulse no longer receives heartbeats from {hostRemovalDisplayName()}. It’s safe to remove the entry now.
                    </p>
                  </div>
                </Show>
              </div>
            </div>

            <div class="mt-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <button
                type="button"
                onClick={closeRemoveModal}
                class="self-start rounded-lg px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
              >
                Close
              </button>
              <Show
                when={canRemoveHost()}
                fallback={
                  <div class="max-w-sm text-xs text-gray-500 dark:text-gray-400">
                    <p class="font-semibold text-gray-600 dark:text-gray-200">Run the uninstall script and mark it complete above.</p>
                    <p class="mt-1">Once confirmed, Pulse will enable the final removal step.</p>
                  </div>
                }
              >
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                  <button
                    type="button"
                    onClick={handleRemoveHost}
                    disabled={removeActionLoading() !== null}
                    class="rounded bg-red-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-red-500 dark:hover:bg-red-400"
                  >
                    {removeActionLoading() === 'remove' ? 'Removing…' : 'Remove host'}
                  </button>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    <p>
                      Removing a host revokes its API token so it cannot register again. With the service already uninstalled, Pulse just needs one more missed heartbeat to clear the row automatically.
                    </p>
                  </div>
                </div>
              </Show>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};

export default HostAgents;
