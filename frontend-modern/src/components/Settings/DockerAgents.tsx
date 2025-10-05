import { Component, createEffect, createSignal, Show, For } from 'solid-js';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { formatRelativeTime, formatAbsoluteTime } from '@/utils/format';
import { Toggle } from '@/components/shared/Toggle';

export const DockerAgents: Component = () => {
  const { state } = useWebSocket();
  const [showInstructions, setShowInstructions] = createSignal(false);

  const dockerHosts = () => state.dockerHosts || [];

  const STORAGE_KEY = 'pulse-show-docker-tab';
  const readPreference = () => {
    if (typeof window === 'undefined') return true;
    const stored = window.localStorage.getItem(STORAGE_KEY);
    return stored !== 'false';
  };

  const [showDockerTab, setShowDockerTab] = createSignal(readPreference());

  const persistPreference = (value: boolean) => {
    setShowDockerTab(value);
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(STORAGE_KEY, value ? 'true' : 'false');
      window.dispatchEvent(
        new CustomEvent('pulse:docker-tab-visibility', {
          detail: { value },
        }),
      );
    }
  };

  createEffect(() => {
    if (dockerHosts().length > 0 && !showDockerTab()) {
      persistPreference(true);
    }
  });

  const pulseUrl = () => {
    if (typeof window !== 'undefined') {
      const protocol = window.location.protocol;
      const hostname = window.location.hostname;
      const port = window.location.port;
      return `${protocol}//${hostname}${port ? `:${port}` : ''}`;
    }
    return 'http://localhost:7655';
  };

  const getInstallCommand = () => {
    const url = pulseUrl();
    return `curl -fsSL ${url}/install-docker-agent.sh | bash -s -- --url ${url}`;
  };

  const getUninstallCommand = () => {
    const url = pulseUrl();
    return `curl -fsSL ${url}/install-docker-agent.sh | bash -s -- --uninstall`;
  };

  const getSystemdService = () => {
    return `[Unit]
Description=Pulse Docker Agent
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pulse-docker-agent --url ${pulseUrl()} --token disabled --interval 30s
Restart=always
RestartSec=5s
User=root

[Install]
WantedBy=multi-user.target`;
  };

  const copyToClipboard = async (text: string) => {
    try {
      // Try modern clipboard API first
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(text);
        window.showToast('success', 'Copied to clipboard');
        return;
      }

      // Fallback for non-secure contexts (http://)
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.style.position = 'fixed';
      textarea.style.left = '-999999px';
      textarea.style.top = '-999999px';
      document.body.appendChild(textarea);
      textarea.focus();
      textarea.select();

      try {
        const successful = document.execCommand('copy');
        if (successful) {
          window.showToast('success', 'Copied to clipboard');
        } else {
          window.showToast('error', 'Failed to copy to clipboard');
        }
      } finally {
        document.body.removeChild(textarea);
      }
    } catch (err) {
      console.error('Failed to copy:', err);
      window.showToast('error', 'Failed to copy to clipboard');
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

      <Card class="space-y-3" padding="lg">
        <SectionHeader
          title="Docker tab visibility"
          description="Hide the Docker tab if you don’t plan to monitor container hosts. We’ll show it automatically once an agent reports in."
          size="sm"
          align="left"
        />
        <div class="flex items-center justify-between gap-3">
          <div class="flex-1 text-sm text-gray-600 dark:text-gray-400">
            Show Docker tab in navigation
          </div>
          <Toggle
            checked={showDockerTab()}
            onChange={(event) => persistPreference((event.currentTarget as HTMLInputElement).checked)}
          />
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          Preference is saved per browser. Hiding the tab won’t stop existing Docker hosts from reporting metrics.
        </p>
      </Card>

      {/* Deployment Instructions */}
      <Show when={showInstructions()}>
        <Card class="space-y-4">
          <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
            Deploy the Pulse Docker agent
          </h3>
          <p class="text-sm text-gray-600 dark:text-gray-400">
            Run this command on your Docker host. If you're not root (most cases), add <code class="text-xs bg-gray-100 dark:bg-gray-800 px-1 rounded">sudo</code> before <code class="text-xs bg-gray-100 dark:bg-gray-800 px-1 rounded">bash</code>. If you're already root (e.g., in a container), the command works as-is.
          </p>

          {/* Quick Install - One-liner */}
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                Quick install (one command)
              </h4>
              <button
                type="button"
                onClick={() => copyToClipboard(getInstallCommand())}
                class="px-3 py-1 text-xs font-medium text-blue-700 dark:text-blue-300 bg-blue-50 dark:bg-blue-900/30 rounded hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors"
                title="Copy to clipboard"
              >
                Copy command
              </button>
            </div>
            <div class="bg-gray-900 dark:bg-gray-950 rounded-lg p-4 overflow-x-auto">
              <code class="text-sm text-green-400 font-mono">{getInstallCommand()}</code>
            </div>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              This will download the agent binary, create a systemd service, and start monitoring automatically.
            </p>
          </div>

          {/* Uninstall */}
          <div class="space-y-2 border-t border-gray-200 dark:border-gray-700 pt-4">
            <div class="flex items-center justify-between">
              <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                Uninstall the agent
              </h4>
              <button
                type="button"
                onClick={() => copyToClipboard(getUninstallCommand())}
                class="px-3 py-1 text-xs font-medium text-red-700 dark:text-red-300 bg-red-50 dark:bg-red-900/30 rounded hover:bg-red-100 dark:hover:bg-red-900/50 transition-colors"
                title="Copy to clipboard"
              >
                Copy command
              </button>
            </div>
            <div class="bg-gray-900 dark:bg-gray-950 rounded-lg p-4 overflow-x-auto">
              <code class="text-sm text-red-400 font-mono">{getUninstallCommand()}</code>
            </div>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              This will stop the agent, remove the binary, service file, and all configuration.
            </p>
          </div>

          {/* Manual Installation */}
          <details class="border-t border-gray-200 dark:border-gray-700 pt-4">
            <summary class="text-sm font-semibold text-gray-900 dark:text-gray-100 cursor-pointer hover:text-blue-600 dark:hover:text-blue-400">
              Manual installation (advanced)
            </summary>
            <div class="mt-4 space-y-4">
              {/* Step 1: Build or download */}
              <div>
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">
                  1. Build the agent binary
                </h4>
                <div class="bg-gray-900 dark:bg-gray-950 rounded-lg p-4 overflow-x-auto">
                  <code class="text-sm text-gray-100 font-mono">
                    cd /opt/pulse
                    <br />
                    GOOS=linux GOARCH=amd64 go build -o pulse-docker-agent ./cmd/pulse-docker-agent
                  </code>
                </div>
              </div>

              {/* Step 2: Copy to host */}
              <div>
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">
                  2. Copy to Docker host
                </h4>
                <div class="bg-gray-900 dark:bg-gray-950 rounded-lg p-4 overflow-x-auto">
                  <code class="text-sm text-gray-100 font-mono">
                    scp pulse-docker-agent user@docker-host:/usr/local/bin/
                    <br />
                    ssh user@docker-host chmod +x /usr/local/bin/pulse-docker-agent
                  </code>
                </div>
              </div>

              {/* Step 3: Create systemd service */}
              <div>
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">
                  3. Create systemd service file
                </h4>
                <div class="relative">
                  <button
                    type="button"
                    onClick={() => copyToClipboard(getSystemdService())}
                    class="absolute top-2 right-2 px-3 py-1 text-xs font-medium text-gray-300 hover:text-white bg-gray-700 hover:bg-gray-600 rounded transition-colors z-10"
                    title="Copy to clipboard"
                  >
                    Copy
                  </button>
                  <div class="bg-gray-900 dark:bg-gray-950 rounded-lg p-4 overflow-x-auto">
                    <pre class="text-sm text-gray-100 font-mono">{getSystemdService()}</pre>
                  </div>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">
                  Save to <code class="px-1 py-0.5 bg-gray-100 dark:bg-gray-800 rounded">/etc/systemd/system/pulse-docker-agent.service</code>
                </p>
              </div>

              {/* Step 4: Enable and start */}
              <div>
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">
                  4. Enable and start
                </h4>
                <div class="bg-gray-900 dark:bg-gray-950 rounded-lg p-4 overflow-x-auto">
                  <code class="text-sm text-gray-100 font-mono">
                    systemctl daemon-reload
                    <br />
                    systemctl enable --now pulse-docker-agent
                  </code>
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
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={dockerHosts()}>
                    {(host) => {
                      const isOnline = host.status?.toLowerCase() === 'online';
                      const runningContainers = host.containers?.filter(c => c.state?.toLowerCase() === 'running').length || 0;

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
