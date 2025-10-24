import { type Component, For, Show, createEffect, createMemo, createSignal, onMount } from 'solid-js';
import type { JSX } from 'solid-js';
import { useWebSocket } from '@/App';
import type { Host } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { formatBytes, formatRelativeTime, formatUptime, formatAbsoluteTime } from '@/utils/format';
import { notificationStore } from '@/stores/notifications';
import { HOST_AGENT_SCOPE } from '@/constants/apiScopes';
import type { SecurityStatus } from '@/types/config';
import type { APITokenRecord } from '@/api/security';
import { SecurityAPI } from '@/api/security';

const TOKEN_PLACEHOLDER = '<api-token>';
const pulseUrl = () => {
  if (typeof window === 'undefined') return 'http://localhost:7655';
  const { protocol, hostname, port } = window.location;
  return `${protocol}//${hostname}${port ? `:${port}` : ''}`;
};

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

export const HostAgents: Component = () => {
  const { state } = useWebSocket();

  let hasLoggedSecurityStatusError = false;

  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [latestRecord, setLatestRecord] = createSignal<APITokenRecord | null>(null);
  const [tokenName, setTokenName] = createSignal('');
  const [confirmedNoToken, setConfirmedNoToken] = createSignal(false);
  const [currentToken, setCurrentToken] = createSignal<string | null>(null);
  const [isGeneratingToken, setIsGeneratingToken] = createSignal(false);

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
      const { token, record } = await SecurityAPI.createToken(desiredName, [HOST_AGENT_SCOPE]);

      setCurrentToken(token);
      setLatestRecord(record);
      setTokenName('');
      setConfirmedNoToken(false);
      notificationStore.success('Token generated and inserted into the command below.', 4000);
    } catch (err) {
      console.error('Failed to generate host agent token', err);
      notificationStore.error('Failed to generate host agent token. Confirm you are signed in as an administrator.', 6000);
    } finally {
      setIsGeneratingToken(false);
    }
  };

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

  const resolvedToken = () => {
    if (requiresToken()) {
      return currentToken() || TOKEN_PLACEHOLDER;
    }
    return currentToken() || 'disabled';
  };

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
                      const [isDeleting, setIsDeleting] = createSignal(false);
                      const tokenRevokedAt = host.tokenRevokedAt;
                      const tokenRevoked = typeof tokenRevokedAt === 'number';
                      const tokenRevokedRelative = tokenRevokedAt ? formatRelativeTime(tokenRevokedAt) : '';
                      const lastSeenMs = host.lastSeen ? new Date(host.lastSeen).getTime() : null;
                      const expectedIntervalMs =
                        (host.intervalSeconds && host.intervalSeconds > 0 ? host.intervalSeconds : 30) * 1000;
                      const staleThresholdMs = Math.max(expectedIntervalMs * 3, 60_000);
                      const isStale =
                        lastSeenMs === null || Date.now() - lastSeenMs >= staleThresholdMs;

                      const status = (host.status || 'unknown').toLowerCase();
                      const isOnline =
                        status === 'online' ||
                        status === 'running' ||
                        status === 'healthy';

                      const baseRowClass = isStale
                        ? 'bg-gray-50 dark:bg-gray-800/50 opacity-60'
                        : 'bg-white dark:bg-gray-900';

                      const rowClass =
                        tokenRevoked && !isStale ? `${baseRowClass} opacity-60` : baseRowClass;

                      const handleDelete = async () => {
                        if (!confirm(`Remove host "${host.displayName || host.hostname || host.id}"?\n\nThis will remove the host from Pulse monitoring. The host agent will re-register if it continues to report.`)) {
                          return;
                        }

                        setIsDeleting(true);
                        try {
                          const response = await fetch(`/api/agents/host/${host.id}`, {
                            method: 'DELETE',
                            credentials: 'include',
                          });

                          if (!response.ok) {
                            const errorData = await response.json();
                            throw new Error(errorData.message || 'Failed to delete host');
                          }

                          notificationStore.success(`Host "${host.displayName || host.hostname}" removed`, 4000);
                        } catch (err) {
                          console.error('Failed to delete host:', err);
                          notificationStore.error(
                            err instanceof Error ? err.message : 'Failed to delete host. Please try again.',
                            6000,
                          );
                        } finally {
                          setIsDeleting(false);
                        }
                      };

                      return (
                        <tr class={rowClass}>
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
                              onClick={handleDelete}
                              disabled={isDeleting() || !isStale}
                              class="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                              title={
                                isStale
                                  ? 'Remove this stale host entry from the inventory'
                                  : 'Host is still reporting — stop the agent before removing'
                              }
                            >
                              {isDeleting() ? (
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
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
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
    </div>
  );
};

export default HostAgents;
