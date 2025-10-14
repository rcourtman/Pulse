import { Component, createSignal, Show, For, onCleanup, onMount, createEffect } from 'solid-js';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import { MonitoringAPI } from '@/api/monitoring';
import { notificationStore } from '@/stores/notifications';
import type { SecurityStatus } from '@/types/config';
import { CommandBuilder } from './CommandBuilder';
import { SecurityAPI, type APITokenRecord } from '@/api/security';

export const DockerAgents: Component = () => {
  const { state } = useWebSocket();
  const [showInstructions, setShowInstructions] = createSignal(true);

  const dockerHosts = () => state.dockerHosts || [];

  const [removingHostId, setRemovingHostId] = createSignal<string | null>(null);
  const [apiToken, setApiToken] = createSignal<string | null>(null);
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [availableTokens, setAvailableTokens] = createSignal<APITokenRecord[]>([]);
  const [loadingTokens, setLoadingTokens] = createSignal(false);
  const [tokensLoaded, setTokensLoaded] = createSignal(false);
  const [isGeneratingToken, setIsGeneratingToken] = createSignal(false);
  const [newTokenValue, setNewTokenValue] = createSignal<string | null>(null);
  const [newTokenRecord, setNewTokenRecord] = createSignal<APITokenRecord | null>(null);
  const [copiedGeneratedToken, setCopiedGeneratedToken] = createSignal(false);

  const tokenDisplayLabel = (token: APITokenRecord) => {
    if (token.name) return token.name;
    if (token.prefix && token.suffix) return `${token.prefix}…${token.suffix}`;
    if (token.prefix) return `${token.prefix}…`;
    if (token.suffix) return `…${token.suffix}`;
    return 'Untitled token';
  };

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
        console.error('Failed to load security status', err);
      }
    };
    fetchSecurityStatus();
  });

  const loadTokens = async () => {
    if (tokensLoaded() || loadingTokens()) return;
    setLoadingTokens(true);
    try {
      const tokens = await SecurityAPI.listTokens();
      const sorted = [...tokens].sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
      setAvailableTokens(sorted);
    } catch (err) {
      console.error('Failed to load API tokens', err);
      notificationStore.error('Failed to load API tokens', 6000);
    } finally {
      setTokensLoaded(true);
      setLoadingTokens(false);
    }
  };

  createEffect(() => {
    if (showInstructions()) {
      loadTokens();
    }
  });

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

  const handleCreateToken = async () => {
    if (isGeneratingToken()) return;
    setIsGeneratingToken(true);
    try {
      const defaultName = `Docker host ${availableTokens().length + 1}`;
      const { token, record } = await SecurityAPI.createToken(defaultName);
      setAvailableTokens((prev) => [record, ...prev]);
      setNewTokenValue(token);
      setNewTokenRecord(record);
      setApiToken(token);
      setCopiedGeneratedToken(false);
      if (typeof window !== 'undefined') {
        try {
          window.localStorage.setItem('apiToken', token);
          window.dispatchEvent(new StorageEvent('storage', { key: 'apiToken', newValue: token }));
        } catch (err) {
          console.warn('Unable to persist API token in localStorage', err);
        }
      }
      notificationStore.success('New API token generated. Copy it into the install command immediately.', 6000);
    } catch (err) {
      console.error('Failed to generate API token', err);
      notificationStore.error('Failed to generate API token', 6000);
    } finally {
      setIsGeneratingToken(false);
    }
  };

  const handleCopyGeneratedToken = async () => {
    const value = newTokenValue();
    if (!value) return;

    const success = await copyToClipboard(value);
    if (success) {
      setCopiedGeneratedToken(true);
      setTimeout(() => setCopiedGeneratedToken(false), 2000);
      if (typeof window !== 'undefined' && window.showToast) {
        window.showToast('success', 'Copied to clipboard');
      }
    } else if (typeof window !== 'undefined' && window.showToast) {
      window.showToast('error', 'Failed to copy to clipboard');
    }
  };

  const requiresToken = () => {
    const status = securityStatus();
    if (status) {
      return status.requiresAuth || status.apiTokenConfigured;
    }
    return true;
  };

  // Always return command template with placeholder - CommandBuilder will do the substitution
  const getInstallCommandTemplate = () => {
    const url = pulseUrl();
    if (!requiresToken()) {
      return `curl -fsSL ${url}/install-docker-agent.sh | sudo bash -s -- --url ${url} --token disabled`;
    }
    return `curl -fsSL ${url}/install-docker-agent.sh | sudo bash -s -- --url ${url} --token ${TOKEN_PLACEHOLDER}`;
  };

  const getUninstallCommand = () => {
    const url = pulseUrl();
    return `curl -fsSL ${url}/install-docker-agent.sh | sudo bash -s -- --uninstall`;
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

  const handleRemoveHost = async (hostId: string, displayName: string) => {
    if (isRemovingHost(hostId)) return;

    const confirmed = window.confirm(
      `Remove Docker host "${displayName}"? This clears it from Pulse until the agent reports again.`,
    );
    if (!confirmed) return;

    setRemovingHostId(hostId);

    try {
      await MonitoringAPI.deleteDockerHost(hostId);
      notificationStore.success(`Removed Docker host ${displayName}`, 3500);
    } catch (error) {
      console.error('Failed to remove Docker host', error);
      const message = error instanceof Error ? error.message : 'Failed to remove Docker host';
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
        <Card class="space-y-6">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Deploy the Pulse Docker agent</h3>
            <p class="text-sm text-gray-600 dark:text-gray-400">
              Follow the steps below to create a token, build the install command, and confirm the host is reporting.
            </p>
          </div>

          <section class="space-y-3">
            <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Step 1 · Token</p>
            <div class="space-y-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-900">
              <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-gray-100">Generate or reuse a host token</p>
                  <p class="text-xs text-gray-600 dark:text-gray-400">
                    Use one API token per host. If that host is ever compromised you can revoke it without touching other machines.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={handleCreateToken}
                  disabled={isGeneratingToken()}
                  class="inline-flex items-center justify-center rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {isGeneratingToken() ? 'Generating…' : 'Generate token'}
                </button>
              </div>

              <Show when={newTokenValue() && newTokenRecord()}>
                <div class="space-y-2 rounded-lg border border-green-200 bg-green-50/80 p-4 dark:border-green-800 dark:bg-green-900/10">
                  <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <p class="text-sm font-semibold text-green-800 dark:text-green-200">Token generated</p>
                      <p class="text-xs text-green-700 dark:text-green-300">
                        “{newTokenRecord()?.name || 'Untitled token'}” will only be shown once. Copy it into the install command below.
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={handleCopyGeneratedToken}
                      class="inline-flex items-center justify-center rounded-md bg-green-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-green-700"
                    >
                      {copiedGeneratedToken() ? 'Copied!' : 'Copy token'}
                    </button>
                  </div>
                  <code class="block break-all rounded border border-green-200 bg-white px-3 py-2 font-mono text-sm dark:border-green-800 dark:bg-green-900/40">
                    {newTokenValue()}
                  </code>
                </div>
              </Show>

              <Show when={securityStatus()?.apiTokenConfigured && securityStatus()?.apiTokenHint}>
                <div class="rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-700 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
                  Existing token hint: <span class="font-mono">{securityStatus()?.apiTokenHint}</span>. Paste the full value if you want to reuse it.
                </div>
              </Show>

              <Show when={availableTokens().length > 0}>
                <details class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:border-gray-700 dark:bg-gray-800/50 dark:text-gray-300">
                  <summary class="cursor-pointer text-sm font-medium text-gray-900 dark:text-gray-100">
                    View saved tokens
                  </summary>
                  <div class="mt-3 overflow-x-auto">
                    <table class="w-full text-xs">
                      <thead>
                        <tr class="border-b border-gray-200 dark:border-gray-700">
                          <th class="py-2 px-2 text-left font-medium text-gray-600 dark:text-gray-400">Name</th>
                          <th class="py-2 px-2 text-left font-medium text-gray-600 dark:text-gray-400">Hint</th>
                          <th class="py-2 px-2 text-left font-medium text-gray-600 dark:text-gray-400">Last used</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                        <For each={availableTokens()}>
                          {(token) => (
                            <tr>
                              <td class="py-2 px-2 text-gray-900 dark:text-gray-100">{tokenDisplayLabel(token)}</td>
                              <td class="py-2 px-2 font-mono text-gray-600 dark:text-gray-400">
                                {token.prefix && token.suffix ? `${token.prefix}…${token.suffix}` : '—'}
                              </td>
                              <td class="py-2 px-2 text-gray-600 dark:text-gray-400">
                                {token.lastUsedAt ? formatRelativeTime(new Date(token.lastUsedAt).getTime()) : 'Never'}
                              </td>
                            </tr>
                          )}
                        </For>
                      </tbody>
                    </table>
                  </div>
                </details>
              </Show>
            </div>
          </section>

          <section class="space-y-3">
            <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Step 2 · Build the command</p>
            <CommandBuilder
              command={getInstallCommandTemplate()}
              placeholder={TOKEN_PLACEHOLDER}
              storedToken={apiToken()}
              currentTokenHint={securityStatus()?.apiTokenHint}
              requiresToken={requiresToken()}
              hasExistingToken={Boolean(securityStatus()?.apiTokenConfigured)}
              onTokenGenerated={(token, record) => {
                setApiToken(token);
                setAvailableTokens((prev) => {
                  const filtered = prev.filter((existing) => existing.id !== record.id);
                  return [record, ...filtered];
                });
                setSecurityStatus((prev) => {
                  if (!prev) return prev;
                  const hint =
                    record.prefix && record.suffix
                      ? `${record.prefix}…${record.suffix}`
                      : prev.apiTokenHint;
                  return {
                    ...prev,
                    apiTokenConfigured: true,
                    apiTokenHint: hint || prev.apiTokenHint,
                  };
                });
                if (typeof window !== 'undefined') {
                  try {
                    window.localStorage.setItem('apiToken', token);
                    window.dispatchEvent(new StorageEvent('storage', { key: 'apiToken', newValue: token }));
                  } catch (err) {
                    console.warn('Unable to persist API token in localStorage', err);
                  }
                }
              }}
            />
            <p class="text-xs text-gray-500 dark:text-gray-400">
              Run the command as root (prepend <code class="rounded bg-gray-100 px-1 dark:bg-gray-800">sudo</code> when needed). The installer downloads the agent, creates a systemd service, and starts reporting automatically.
            </p>
          </section>

          <section class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-xs text-blue-700 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
            Step 3 · Within one or two heartbeats (default 30 seconds) the host should appear in the table below. If not, check the agent logs with{' '}
            <code class="rounded bg-blue-100 px-1 py-0.5 font-mono dark:bg-blue-900/40">journalctl -u pulse-docker-agent -f</code>.
          </section>

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
                      GOOS=linux GOARCH=amd64 go build -o pulse-docker-agent ./cmd/pulse-docker-agent
                    </code>
                  </div>
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

      {/* Active Docker Hosts */}
      <Card>
        <div class="space-y-4">
          <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
            Reporting Docker hosts ({dockerHosts().length})
          </h3>

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
                      const displayName = host.displayName || host.hostname || host.id;

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
                            <span
                              class={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${
                                isOnline
                                  ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                  : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
                              }`}
                            >
                              {host.status || 'unknown'}
                            </span>
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
                            <button
                              type="button"
                              class="text-xs font-semibold text-red-600 hover:text-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
                              onClick={() => handleRemoveHost(host.id, displayName)}
                              disabled={isRemovingHost(host.id)}
                            >
                              {isRemovingHost(host.id) ? 'Removing…' : 'Remove'}
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
