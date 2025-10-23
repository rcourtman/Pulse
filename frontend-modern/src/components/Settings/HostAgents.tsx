import { Component, For, Show, createEffect, createMemo, createSignal, onMount } from 'solid-js';
import type { JSX } from 'solid-js';
import { useWebSocket } from '@/App';
import type { Host } from '@/types/api';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Card } from '@/components/shared/Card';
import CopyButton from '@/components/shared/CopyButton';
import { formatBytes, formatRelativeTime, formatUptime } from '@/utils/format';
import { SecurityAPI } from '@/api/security';
import { notificationStore } from '@/stores/notifications';
import { showTokenReveal } from '@/stores/tokenReveal';
import { HOST_AGENT_SCOPE } from '@/constants/apiScopes';

type HostAgentVariant = 'all' | 'linux' | 'macos' | 'windows';

interface HostAgentsProps {
  variant?: HostAgentVariant;
}

const RELEASE_BASE = 'https://github.com/rcourtman/Pulse/releases/latest/download';

const TOKEN_PLACEHOLDER = '<api-token>';

const pulseUrl = () => {
  if (typeof window === 'undefined') return 'http://localhost:7655';
  const { protocol, hostname, port } = window.location;
  return `${protocol}//${hostname}${port ? `:${port}` : ''}`;
};

const commandsByVariant: Record<HostAgentVariant, { title: string; description: string; snippets: { label: string; command: string; note?: string | JSX.Element }[] }> = {
  all: {
    title: 'Installation quick start',
    description:
      'Generate an API token from Settings → Security with the host agent reporting scope, then replace the highlighted token placeholder. Agents only require outbound HTTP(S) access to Pulse.',
    snippets: [
      {
        label: 'Linux (systemd)',
        command: [
          `curl -fsSL ${RELEASE_BASE}/pulse-host-agent-linux-amd64 -o /usr/local/bin/pulse-host-agent`,
          'sudo chmod +x /usr/local/bin/pulse-host-agent',
          `sudo /usr/local/bin/pulse-host-agent --url ${pulseUrl()} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
        ].join(' && '),
      },
      {
        label: 'macOS (launchd)',
        command: [
          `curl -fsSL ${RELEASE_BASE}/pulse-host-agent-darwin-arm64 -o /usr/local/bin/pulse-host-agent`,
          'sudo chmod +x /usr/local/bin/pulse-host-agent',
          `sudo /usr/local/bin/pulse-host-agent --url ${pulseUrl()} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
        ].join(' && '),
        note: (
          <span>
            Create <code>~/Library/LaunchAgents/com.pulse.host-agent.plist</code> to keep the agent running between logins.
          </span>
        ),
      },
      {
        label: 'Ad-hoc execution',
        command: `/usr/local/bin/pulse-host-agent --url ${pulseUrl()} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
      },
    ],
  },
  linux: {
    title: 'Install on Linux',
    description:
      'Download the static binary, make it executable, and (optionally) register it as a systemd service. Replace the token placeholder with an API token scoped for host agent reporting.',
    snippets: [
      {
        label: 'Install + enable (systemd)',
        command: [
          `curl -fsSL ${RELEASE_BASE}/pulse-host-agent-linux-amd64 -o /usr/local/bin/pulse-host-agent`,
          'sudo chmod +x /usr/local/bin/pulse-host-agent',
          `sudo /usr/local/bin/pulse-host-agent --url ${pulseUrl()} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
        ].join(' && '),
        note: (
          <span>
            For persistence, create <code>/etc/systemd/system/pulse-host-agent.service</code> and enable it with{' '}
            <code>systemctl enable --now pulse-host-agent</code>.
          </span>
        ),
      },
    ],
  },
  macos: {
    title: 'Install on macOS',
    description:
      'Use the universal macOS build (arm64) with an API token that grants the host agent reporting scope, then register it via launchd for continuous reporting.',
    snippets: [
      {
        label: 'Install binary',
        command: [
          `curl -fsSL ${RELEASE_BASE}/pulse-host-agent-darwin-arm64 -o /usr/local/bin/pulse-host-agent`,
          'sudo chmod +x /usr/local/bin/pulse-host-agent',
        ].join(' && '),
      },
      {
        label: 'Launchd service',
        command: `launchctl load ~/Library/LaunchAgents/com.pulse.host-agent.plist`,
        note: (
          <span>
            Create a plist pointing to{' '}
            <code>/usr/local/bin/pulse-host-agent --url {pulseUrl()} --token {TOKEN_PLACEHOLDER} --interval 30s</code> to run at login.
          </span>
        ),
      },
    ],
  },
  windows: {
    title: 'Install on Windows',
    description:
      'Native Windows builds are coming soon. In the interim you can run the Linux binary under WSL or compile from source using an API token scoped for host agent reporting.',
    snippets: [
      {
        label: 'Compile from source (PowerShell)',
        command: [
          'git clone https://github.com/rcourtman/Pulse.git',
          'cd Pulse',
          'go build -o pulse-host-agent.exe ./cmd/pulse-host-agent',
          `./pulse-host-agent.exe --url ${pulseUrl()} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
        ].join(' && '),
        note: (
          <span>
            Consider registering the executable as a Windows Service via <code>sc.exe</code> or NSSM once native artefacts ship.
          </span>
        ),
      },
    ],
  },
};

const platformFilters: Record<HostAgentVariant, string[] | null> = {
  all: null,
  linux: ['linux'],
  macos: ['macos'],
  windows: ['windows'],
};

export const HostAgents: Component<HostAgentsProps> = (props) => {
  const variant: HostAgentVariant = props.variant ?? 'all';
  const { state } = useWebSocket();
  const [apiToken, setApiToken] = createSignal('');
  const [isGeneratingToken, setIsGeneratingToken] = createSignal(false);
  const [tokenAccessDenied, setTokenAccessDenied] = createSignal(false);

  const hosts = createMemo(() => {
    const list = state.hosts ?? [];
    const filters = platformFilters[variant];
    const filtered = filters ? list.filter((host) => filters.includes((host.platform ?? '').toLowerCase())) : list;
    return [...filtered].sort((a, b) => (a.hostname || '').localeCompare(b.hostname || ''));
  });

  const renderTags = (host: Host) => {
    const tags = host.tags ?? [];
    if (!tags.length) return '—';
    return tags.join(', ');
  };

  const installMeta = commandsByVariant[variant];

  onMount(() => {
    if (typeof window === 'undefined') return;
    try {
      const stored = window.localStorage.getItem('apiToken');
      if (stored) {
        setApiToken(stored);
      }
    } catch (err) {
      console.warn('Unable to read API token from localStorage', err);
    }
  });

  createEffect(() => {
    if (typeof window === 'undefined') return;
    const token = apiToken();
    try {
      if (token) {
        window.localStorage.setItem('apiToken', token);
      } else {
        window.localStorage.removeItem('apiToken');
      }
    } catch (err) {
      console.warn('Unable to persist API token in localStorage', err);
    }
  });

  const generateToken = async () => {
    if (isGeneratingToken()) return;
    if (tokenAccessDenied()) {
      notificationStore.error('Administrator access required to generate host agent tokens.', 6000);
      return;
    }

    setIsGeneratingToken(true);
    try {
      const defaultName = `Host agent ${new Date().toISOString().slice(0, 10)}`;
      const { token, record } = await SecurityAPI.createToken(defaultName, [HOST_AGENT_SCOPE]);
      setApiToken(token);
      showTokenReveal({
        token,
        record,
        source: 'host-agent',
        note: 'Copy this token into the host agent install command or store it securely for automation.',
      });
      notificationStore.success('Created host agent API token with reporting scope.', 6000);
    } catch (err) {
      console.error('Failed to create host agent token', err);
      if (err instanceof Error && /authentication required|forbidden/i.test(err.message)) {
        setTokenAccessDenied(true);
        notificationStore.error('Sign in with an administrator account to generate tokens here.', 6000);
      } else {
        notificationStore.error('Failed to generate API token', 6000);
      }
    } finally {
      setIsGeneratingToken(false);
    }
  };

  const cardTitle = () => {
    switch (variant) {
      case 'linux':
        return 'Linux servers';
      case 'macos':
        return 'macOS devices';
      case 'windows':
        return 'Windows servers';
      default:
        return 'Host agents';
    }
  };

  const cardDescription = () => {
    switch (variant) {
      case 'linux':
        return 'Install the Pulse host agent on Debian, Ubuntu, RHEL, Arch, or other Linux hosts to surface uptime and capacity metrics.';
      case 'macos':
        return 'Deploy the lightweight host agent via launchd to keep macOS hardware in view alongside your Proxmox estate.';
      case 'windows':
        return 'Track Windows Server hosts through a native Pulse agent. A first-party build is on the roadmap—compile from source today or watch this space.';
      default:
        return 'Install the Pulse host agent on Linux, macOS, or Windows servers to surface uptime, OS metadata, and capacity metrics.';
    }
  };

  return (
    <div class="space-y-6">
      <SectionHeader title={cardTitle()} description={cardDescription()} />

      <Card padding="lg" class="space-y-4">
        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">API token</h3>
          <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
            Manage tokens via <strong>Settings → Security</strong>
          </div>
        </div>
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
          <input
            type="text"
            value={apiToken()}
            onInput={(e) => setApiToken(e.currentTarget.value.trim())}
            placeholder="Paste API token (leave blank to keep <api-token> placeholder)"
            class="flex-1 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            type="button"
            onClick={generateToken}
            disabled={isGeneratingToken()}
            class="inline-flex items-center justify-center rounded-md border border-transparent bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition-colors hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isGeneratingToken() ? 'Generating…' : 'Generate token'}
          </button>
          <Show when={apiToken()}>
            <span class="text-xs text-gray-500 dark:text-gray-400">
              Token will be embedded in the commands below.
            </span>
          </Show>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          Tokens generated here automatically include the host agent reporting scope (<code>host-agent:report</code>).
        </p>

        <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">{installMeta.title}</h3>
        <p class="text-sm text-gray-600 dark:text-gray-400">{installMeta.description}</p>

        <div class="space-y-3">
          <For each={installMeta.snippets}>
            {(snippet) => (
              <div class="space-y-2">
                <div class="flex items-center justify-between gap-3">
                  <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-200">{snippet.label}</h4>
                  <CopyButton
                    text={snippet.command.replace(
                      TOKEN_PLACEHOLDER,
                      apiToken() || TOKEN_PLACEHOLDER,
                    )}
                  >
                    Copy command
                  </CopyButton>
                </div>
                <pre class="overflow-x-auto rounded-md bg-gray-900/90 p-3 text-xs text-gray-100">
                  <code>
                    {snippet.command.replace(
                      TOKEN_PLACEHOLDER,
                      apiToken() || TOKEN_PLACEHOLDER,
                    )}
                  </code>
                </pre>
                <Show when={snippet.note}>
                  <p class="text-xs text-gray-500 dark:text-gray-400">{snippet.note}</p>
                </Show>
              </div>
            )}
          </For>
        </div>
      </Card>

      <Card padding="lg" class="space-y-4">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Reporting hosts</h3>
          <span class="text-sm text-gray-500 dark:text-gray-400">{hosts().length} connected</span>
        </div>

        <Show
          when={hosts().length > 0}
          fallback={
            <p class="text-sm text-gray-600 dark:text-gray-400">
              {variant === 'windows'
                ? 'No Windows hosts have reported yet. Compile the agent from source or check back when native artefacts are published.'
                : 'No host agents are reporting yet. Deploy the binary using the commands above to see hosts listed here.'}
            </p>
          }
        >
          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-sm">
              <thead class="bg-gray-50 dark:bg-gray-900/40">
                <tr>
                  <th class="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Hostname</th>
                  <th class="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Platform</th>
                  <th class="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Uptime</th>
                  <th class="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Memory</th>
                  <th class="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Last seen</th>
                  <th class="px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-300">Tags</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-800">
                <For each={hosts()}>
                  {(host) => (
                    <tr class="hover:bg-gray-50 dark:hover:bg-gray-800/60">
                      <td class="px-3 py-2 font-medium text-gray-900 dark:text-gray-100">
                        {host.displayName || host.hostname || host.id}
                      </td>
                      <td class="px-3 py-2 text-gray-600 dark:text-gray-300 capitalize">
                        {host.platform || '—'}
                      </td>
                      <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                        {host.uptimeSeconds ? formatUptime(host.uptimeSeconds) : '—'}
                      </td>
                      <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                        {host.memory?.total
                          ? `${formatBytes(host.memory.used ?? 0)} / ${formatBytes(host.memory.total)}`
                          : '—'}
                      </td>
                      <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                        {host.lastSeen ? formatRelativeTime(host.lastSeen) : '—'}
                      </td>
                      <td class="px-3 py-2 text-gray-600 dark:text-gray-300">{renderTags(host)}</td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div>
        </Show>
      </Card>
    </div>
  );
};

export default HostAgents;
