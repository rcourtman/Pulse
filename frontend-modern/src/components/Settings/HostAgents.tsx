import { Component, For, Show, createEffect, createMemo, createSignal, onMount } from 'solid-js';
import type { JSX } from 'solid-js';
import { useWebSocket } from '@/App';
import type { Host } from '@/types/api';
import { Card } from '@/components/shared/Card';
import CopyButton from '@/components/shared/CopyButton';
import { formatBytes, formatRelativeTime, formatUptime } from '@/utils/format';
import { notificationStore } from '@/stores/notifications';
import { showTokenReveal } from '@/stores/tokenReveal';
import { HOST_AGENT_SCOPE } from '@/constants/apiScopes';
import type { SecurityStatus } from '@/types/config';
import type { APITokenRecord } from '@/api/security';
import { useScopedTokenManager } from '@/hooks/useScopedTokenManager';

type HostAgentVariant = 'all' | 'linux' | 'macos' | 'windows';

interface HostAgentsProps {
  variant?: HostAgentVariant;
}

type HostPlatform = 'linux' | 'macos' | 'windows';

const hostPlatformOptions: { id: HostPlatform; label: string; description: string }[] = [
  {
    id: 'linux',
    label: 'Linux',
    description: 'Download the static binary and enable the systemd service on Debian, Ubuntu, RHEL, Arch, and more.',
  },
  {
    id: 'macos',
    label: 'macOS',
    description: 'Use the universal binary with launchd to keep desktops and hosts reporting in the background.',
  },
  {
    id: 'windows',
    label: 'Windows',
    description: 'Native Windows service with automatic startup. PowerShell script handles binary download and service installation.',
  },
];

const TOKEN_PLACEHOLDER = '<api-token>';
const pulseUrl = () => {
  if (typeof window === 'undefined') return 'http://localhost:7655';
  const { protocol, hostname, port } = window.location;
  return `${protocol}//${hostname}${port ? `:${port}` : ''}`;
};

const commandsByVariant: Record<HostAgentVariant, { title: string; description: string; snippets: { label: string; command: string; note?: string | JSX.Element }[] }> = {
  all: {
    title: 'Installation',
    description:
      'Run the installer script to automatically download and configure the host agent on any supported platform.',
    snippets: [
      {
        label: 'Install host agent',
        command: `curl -fsSL ${pulseUrl()}/install-host-agent.sh | bash -s -- --url ${pulseUrl()} --token ${TOKEN_PLACEHOLDER} --interval 30s`,
        note: (
          <span>
            The script downloads the agent binary from Pulse and sets up systemd (Linux) or launchd (macOS) for automatic startup.
          </span>
        ),
      },
    ],
  },
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

const platformFilters: Record<HostAgentVariant, string[] | null> = {
  all: null,
  linux: ['linux'],
  macos: ['macos'],
  windows: ['windows'],
};

export const HostAgents: Component<HostAgentsProps> = (props) => {
  const variant: HostAgentVariant = props.variant ?? 'all';
  const { state } = useWebSocket();

  let hasLoggedSecurityStatusError = false;

  const [showInstructions, setShowInstructions] = createSignal(true);
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [showGenerateTokenModal, setShowGenerateTokenModal] = createSignal(false);
  const [newTokenName, setNewTokenName] = createSignal('');
  const [generateError, setGenerateError] = createSignal<string | null>(null);
  const [latestRecord, setLatestRecord] = createSignal<APITokenRecord | null>(null);
  const [stepTwoComplete, setStepTwoComplete] = createSignal(false);

  const {
    token: apiToken,
    setToken: setApiToken,
    isGeneratingToken,
    generateToken,
  } = useScopedTokenManager({
    scope: HOST_AGENT_SCOPE,
    storageKey: 'hostAgentToken',
    legacyKeys: ['apiToken'],
  });

  createEffect(() => {
    if (!apiToken()) {
      setStepTwoComplete(false);
    }
  });

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

  const [selectedPlatform, setSelectedPlatform] = createSignal<HostPlatform>('linux');

  const effectiveVariant = createMemo<HostAgentVariant>(() =>
    variant === 'all' ? selectedPlatform() : variant,
  );

  const installMeta = createMemo(() => commandsByVariant[effectiveVariant()]);
  const tokenStepLabel = () => `${variant === 'all' ? 'Step 2' : 'Step 1'} · Choose an API token`;
  const commandStepLabel = () => `${variant === 'all' ? 'Step 3' : 'Step 2'} · Installation commands`;

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

  const tokenReady = () => !requiresToken() || Boolean(apiToken());
  const commandsUnlocked = () => tokenReady() && stepTwoComplete();

  const acknowledgeTokenUse = () => {
    if (!requiresToken()) {
      setStepTwoComplete(true);
      return;
    }
    if (apiToken()) {
      setStepTwoComplete(true);
      notificationStore.success('Token ready to embed in the install commands.', 3500);
    } else {
      notificationStore.info('Generate or select a token before continuing.', 4000);
    }
  };

  const openGenerateTokenModal = () => {
    setGenerateError(null);
    const defaultName = `Host agent ${new Date().toISOString().slice(0, 10)}`;
    setNewTokenName(defaultName);
    setShowGenerateTokenModal(true);
    setStepTwoComplete(false);
  };

  const handleCreateToken = async () => {
    if (isGeneratingToken()) return;

    setGenerateError(null);
    try {
      const desiredName = newTokenName().trim() || `Host agent ${new Date().toISOString().slice(0, 10)}`;
      const { token, record } = await generateToken(desiredName);

      setShowGenerateTokenModal(false);
      setNewTokenName('');
      setLatestRecord(record);
      showTokenReveal({
        token,
        record,
        source: 'host-agent',
        note: `Copy this token into the host agent install command. Scope: ${HOST_AGENT_SCOPE}.`,
      });
      notificationStore.success('Created host agent API token with reporting scope.', 6000);
    } catch (err) {
      console.error('Failed to generate host agent token', err);
      setGenerateError('Failed to generate host agent token. Confirm you are signed in as an administrator.');
      notificationStore.error('Failed to generate API token', 6000);
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
    if (!requiresToken()) {
      return 'disabled';
    }
    return apiToken() || TOKEN_PLACEHOLDER;
  };

  return (
    <div class="space-y-6">
      <Card padding="lg" class="space-y-5">
        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">{installMeta().title}</h3>
            <p class="text-sm text-gray-600 dark:text-gray-400">{installMeta().description}</p>
          </div>
          <button
            type="button"
            onClick={() => setShowInstructions(!showInstructions())}
            class="px-4 py-2 text-sm font-medium text-blue-700 dark:text-blue-300 bg-blue-50 dark:bg-blue-900/30 rounded-lg hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors"
          >
            {showInstructions() ? 'Hide' : 'Show'} instructions
          </button>
        </div>

        <Show when={showInstructions()}>
          <div class="space-y-5">
            <Show when={variant === 'all'}>
              <div class="space-y-4">
                <div class="space-y-1">
                  <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">Step 1 · Choose the operating system</p>
                  <p class="text-sm text-gray-600 dark:text-gray-400">
                    Pick the platform you are onboarding. The install commands adapt automatically.
                  </p>
                </div>
                <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                  <For each={hostPlatformOptions}>
                    {(option) => {
                      const isActive = () => selectedPlatform() === option.id;
                      return (
                        <button
                          type="button"
                          class={`flex flex-col items-start gap-2 rounded-lg border transition-colors p-4 text-left shadow-sm focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 ${
                            isActive()
                              ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                              : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 hover:border-blue-300 dark:hover:border-blue-500'
                          }`}
                          onClick={() => {
                            setSelectedPlatform(option.id);
                            setGenerateError(null);
                            setLatestRecord(null);
                            setApiToken(null);
                            setStepTwoComplete(false);
                          }}
                        >
                          <p class="font-semibold text-gray-900 dark:text-gray-100">{option.label}</p>
                          <p class="text-xs text-gray-600 dark:text-gray-400">{option.description}</p>
                        </button>
                      );
                    }}
                  </For>
                </div>
              </div>
            </Show>

            <Show when={requiresToken()}>
              <div class="space-y-4">
                <div class="space-y-1">
                  <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">{tokenStepLabel()}</p>
                  <p class="text-sm text-gray-600 dark:text-gray-400">
                    Generate a scoped token for this host. Tokens minted here grant the <code>{HOST_AGENT_SCOPE}</code> permission only.
                  </p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    Need additional scopes? Visit <a href="/settings/security" class="text-blue-600 dark:text-blue-300 underline hover:no-underline font-medium">Security → API tokens</a> to create a bespoke credential.
                  </p>
                </div>

                <Show when={generateError()}>
                  <div class="rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-xs text-red-800 dark:border-red-800 dark:bg-red-900/30 dark:text-red-200">
                    {generateError()}
                  </div>
                </Show>

                <Show when={latestRecord()}>
                  <div class="flex items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-4 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                    <span>
                      Token <strong>{latestRecord()?.name}</strong> created ({latestRecord()?.prefix}…{latestRecord()?.suffix}). Copy the full value from the pop-up and store it securely—this is the only time it is shown.
                    </span>
                  </div>
                </Show>

                <button
                  type="button"
                  onClick={openGenerateTokenModal}
                  disabled={isGeneratingToken()}
                  class="inline-flex items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isGeneratingToken() ? 'Generating…' : 'Generate token'}
                </button>

                <Show when={apiToken()}>
                  <div class="flex flex-col sm:flex-row sm:items-center gap-2">
                    <div class={`flex-1 rounded-lg border px-4 py-2 text-xs ${
                      stepTwoComplete()
                        ? 'border-green-200 bg-green-50 text-green-800 dark:border-green-800 dark:bg-green-900/20 dark:text-green-200'
                        : 'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200'
                    }`}>
                      {stepTwoComplete()
                        ? 'Token inserted. Proceed to the install commands below.'
                        : 'Stored token detected. Press confirm to insert it into each command.'}
                    </div>
                    <button
                      type="button"
                      onClick={acknowledgeTokenUse}
                      disabled={stepTwoComplete()}
                      class={`inline-flex items-center justify-center rounded-lg px-4 py-2 text-sm font-medium transition-colors ${
                        stepTwoComplete()
                          ? 'bg-green-600 text-white cursor-default'
                          : 'bg-blue-600 text-white hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-400'
                      }`}
                    >
                      {stepTwoComplete() ? 'Token inserted' : 'Insert token into commands'}
                    </button>
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
                  onClick={acknowledgeTokenUse}
                  disabled={stepTwoComplete()}
                  class={`inline-flex items-center justify-center rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                    stepTwoComplete()
                      ? 'bg-green-600 text-white cursor-default'
                      : 'bg-gray-900 text-white hover:bg-black dark:bg-gray-100 dark:text-gray-900 dark:hover:bg-white'
                  }`}
                >
                  {stepTwoComplete() ? 'No token confirmed' : 'Confirm without token'}
                </button>
              </div>
            </Show>

            <Show when={commandsUnlocked()}>
              <div class="space-y-3">
                <div class="flex items-center justify-between">
                  <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{commandStepLabel()}</h4>
                  <button
                    type="button"
                    onClick={async () => {
                      const firstSnippet = installMeta().snippets[0];
                      if (!firstSnippet) return;
                      const command = firstSnippet.command.replace(TOKEN_PLACEHOLDER, resolvedToken());
                      const success = await copyToClipboard(command);
                      if (typeof window !== 'undefined' && window.showToast) {
                        window.showToast(success ? 'success' : 'error', success ? 'Copied!' : 'Failed to copy');
                      }
                    }}
                    class="px-3 py-1.5 text-xs font-medium rounded-lg transition-colors bg-blue-600 text-white hover:bg-blue-700"
                  >
                    Copy first command
                  </button>
                </div>
                <div class="space-y-3">
                  <For each={installMeta().snippets}>
                    {(snippet) => (
                      <div class="space-y-2">
                        <div class="flex items-center justify-between gap-3">
                          <h5 class="text-sm font-semibold text-gray-700 dark:text-gray-200">{snippet.label}</h5>
                          <CopyButton
                            text={snippet.command.replace(
                              TOKEN_PLACEHOLDER,
                              resolvedToken(),
                            )}
                          >
                            Copy command
                          </CopyButton>
                        </div>
                        <pre class="overflow-x-auto rounded-md bg-gray-900/90 p-3 text-xs text-gray-100">
                          <code>
                            {snippet.command.replace(
                              TOKEN_PLACEHOLDER,
                              resolvedToken(),
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
              </div>
            </Show>

            <Show when={requiresToken() && (!apiToken() || !stepTwoComplete())}>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                Generate a new token or confirm the stored one to unlock the install commands.
              </p>
            </Show>
            <Show when={!requiresToken() && !stepTwoComplete()}>
              <p class="text-xs text-gray-500 dark:text-gray-400">Confirm the no-token setup to continue.</p>
            </Show>
          </div>
        </Show>
      </Card>

      <Show when={showGenerateTokenModal()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div class="w-full max-w-md rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800">
            <div class="space-y-2">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Generate a new host agent token</h3>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                Pulse will create a scoped token for this host and automatically insert it into the install commands. You can manage or revoke tokens anytime from Security settings.
              </p>
            </div>
            <div class="mt-4 space-y-2">
              <label class="text-sm font-medium text-gray-700 dark:text-gray-300" for="host-agent-new-token-name">
                Token name
              </label>
              <input
                id="host-agent-new-token-name"
                type="text"
                value={newTokenName()}
                onInput={(event) => setNewTokenName(event.currentTarget.value)}
                class="w-full rounded border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-900/60"
                placeholder="Host agent token"
              />
              <p class="text-xs text-gray-500 dark:text-gray-400">
                Friendly names make it easier to audit tokens later (e.g. <code class="font-mono text-xs">host-lab-01</code>).
              </p>
            </div>
            <div class="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => {
                  setShowGenerateTokenModal(false);
                  setNewTokenName('');
                  setGenerateError(null);
                }}
                class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleCreateToken}
                disabled={isGeneratingToken()}
                class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-blue-500 dark:hover:bg-blue-400"
              >
                {isGeneratingToken() ? 'Generating…' : 'Generate token'}
              </button>
            </div>
          </div>
        </div>
      </Show>

      <Card padding="lg" class="space-y-5">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">Reporting hosts</h3>
          <span class="text-sm text-gray-500 dark:text-gray-400">{hosts().length} connected</span>
        </div>

        <Show
          when={hosts().length > 0}
          fallback={
            <p class="text-sm text-gray-600 dark:text-gray-400">
              {variant === 'windows'
                ? 'No Windows hosts have reported yet. Run the PowerShell installation script above as Administrator to deploy the agent.'
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
                  <th class="px-3 py-2 text-right font-semibold text-gray-700 dark:text-gray-300">Actions</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-800">
                <For each={hosts()}>
                  {(host) => {
                    const [isDeleting, setIsDeleting] = createSignal(false);

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
                          6000
                        );
                      } finally {
                        setIsDeleting(false);
                      }
                    };

                    return (
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
                        <td class="px-3 py-2 text-right">
                          <button
                            type="button"
                            onClick={handleDelete}
                            disabled={isDeleting()}
                            class="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                            title="Remove host from monitoring"
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
      </Card>
    </div>
  );
};

export default HostAgents;
