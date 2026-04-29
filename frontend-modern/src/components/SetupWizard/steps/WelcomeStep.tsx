import { Component, createMemo, createSignal, onMount, Show } from 'solid-js';
import { showError, showSuccess } from '@/utils/toast';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { copyToClipboard } from '@/utils/clipboard';
import { Copy, Check, Terminal } from 'lucide-solid';

interface WelcomeStepProps {
  onNext: () => void;
  bootstrapToken: string;
  setBootstrapToken: (token: string) => void;
  isUnlocked: boolean;
  setIsUnlocked: (unlocked: boolean) => void;
}

export const WelcomeStep: Component<WelcomeStepProps> = (props) => {
  const [isValidating, setIsValidating] = createSignal(false);
  const [isDocker, setIsDocker] = createSignal(false);
  const [inContainer, setInContainer] = createSignal(false);
  const [lxcCtid, setLxcCtid] = createSignal('');
  const [dockerContainerName, setDockerContainerName] = createSignal('');
  const [copied, setCopied] = createSignal(false);

  const copyCommand = async () => {
    const copied = await copyToClipboard(getTokenCommand());
    if (copied) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
      showSuccess('Command copied to clipboard');
    } else {
      showError('Failed to copy command');
    }
  };

  // Fetch bootstrap info on mount
  const fetchBootstrapInfo = async () => {
    try {
      const data = await apiFetchJSON<{
        bootstrapTokenPath?: string;
        isDocker?: boolean;
        inContainer?: boolean;
        lxcCtid?: string;
        dockerContainerName?: string;
      }>('/api/security/status');
      setIsDocker(data?.isDocker || false);
      setInContainer(data?.inContainer || false);
      setLxcCtid(data?.lxcCtid || '');
      setDockerContainerName(data?.dockerContainerName || '');
    } catch (error) {
      logger.error('Failed to fetch bootstrap info:', error);
    }
  };

  onMount(() => {
    void fetchBootstrapInfo();
  });

  const looksLikeBootstrapTokenSnapshot = (value: string) => {
    const trimmed = value.trim();
    if (!trimmed.startsWith('{')) {
      return false;
    }
    try {
      const parsed = JSON.parse(trimmed) as {
        token_ciphertext?: unknown;
        token_hash?: unknown;
        version?: unknown;
      };
      return (
        typeof parsed.version === 'number' &&
        typeof parsed.token_ciphertext === 'string' &&
        typeof parsed.token_hash === 'string'
      );
    } catch {
      return false;
    }
  };

  const snapshotPasteHelp =
    'That looks like the encrypted .bootstrap_token file contents, not the raw setup token. Run the command above and paste the token string it prints.';

  const handleUnlock = async () => {
    const trimmedToken = props.bootstrapToken.trim();
    if (!trimmedToken) {
      showError('Please enter the bootstrap token');
      return;
    }
    if (looksLikeBootstrapTokenSnapshot(trimmedToken)) {
      showError(snapshotPasteHelp);
      return;
    }

    setIsValidating(true);
    try {
      const response = await apiFetch('/api/security/validate-bootstrap-token', {
        method: 'POST',
        body: JSON.stringify({ token: trimmedToken }),
      });

      if (!response.ok) {
        throw new Error('Invalid bootstrap token');
      }

      props.setIsUnlocked(true);
      props.onNext();
    } catch (_error) {
      showError('Invalid bootstrap token. Please check and try again.');
    } finally {
      setIsValidating(false);
    }
  };

  const getTokenCommand = () => {
    if (isDocker()) {
      return `docker exec ${dockerContainerName() || '<pulse-container>'} /app/pulse bootstrap-token`;
    }
    if (inContainer() && lxcCtid()) {
      return `pct exec ${lxcCtid()} -- pulse bootstrap-token`;
    }
    if (inContainer()) {
      return `pct exec <ctid> -- pulse bootstrap-token`;
    }
    return `sudo pulse bootstrap-token`;
  };

  const deploymentLabel = createMemo(() => {
    if (isDocker()) {
      return 'Docker deployment';
    }
    if (inContainer() && lxcCtid()) {
      return 'LXC container';
    }
    if (inContainer()) {
      return 'Containerized deployment';
    }
    return 'Direct host install';
  });

  const deploymentHint = createMemo(() => {
    if (isDocker()) {
      return dockerContainerName()
        ? `Pulse appears to be running in Docker as container "${dockerContainerName()}". Run the command on the Docker host to print the one-time setup token from that container.`
        : 'Pulse appears to be running in Docker. Run the command on the Docker host and replace <pulse-container> with the running Pulse container name.';
    }
    if (inContainer() && lxcCtid()) {
      return `Pulse appears to be running in LXC container ${lxcCtid()}. Run the command on the Proxmox host to execute into that container and print the one-time setup token.`;
    }
    if (inContainer()) {
      return 'Pulse appears to be running in a containerized environment. Run the command from the host that manages the container so you can print the one-time setup token.';
    }
    return 'Run the command directly in a shell on the Pulse server to print the one-time setup token.';
  });

  const unlockHelp = createMemo(() => {
    if (isDocker()) {
      return 'This one-time bootstrap token only unlocks first-run setup. Run the command above and paste the token string it prints. After verification, you will create the admin account and Pulse will generate the long-lived API token separately.';
    }
    return 'This one-time bootstrap token only unlocks first-run setup on this Pulse server. Run the command above and paste the token string it prints. It is not your admin password and it is not the API token you will use after setup.';
  });

  return (
    <div class="text-center relative">
      <div class="mb-8 relative z-10">
        <img
          src="/logo.svg"
          alt="Pulse Logo"
          class="w-20 h-20 rounded-md mb-6 mx-auto dark:shadow-none"
        />
        <h1 class="text-3xl sm:text-4xl font-bold tracking-tight text-base-content mb-3">
          Welcome to Pulse
        </h1>
        <p class="text-base text-muted max-w-xl mx-auto">
          Three steps:{' '}
          <span class="text-base-content font-medium">Unlock this Pulse server</span>,{' '}
          <span class="text-base-content font-medium">Create the admin account</span>, then{' '}
          <span class="text-base-content font-medium">Choose the first source</span>.
        </p>
        <p class="text-sm text-muted max-w-xl mx-auto mt-2">
          <span>Connect a platform API, install Pulse Agent, or use both for full coverage.</span>
        </p>
      </div>

      <Show when={!props.isUnlocked}>
        <div class="p-6 sm:p-8 max-w-2xl mx-auto bg-surface border border-border rounded-md text-left">
          <h3 class="text-lg font-semibold text-base-content mb-2 tracking-tight">Unlock setup</h3>
          <p class="text-sm text-muted mb-5">{deploymentHint()}</p>

          <div class="mb-5">
            <div class="bg-base rounded-md p-4 font-mono text-sm text-emerald-400 border border-border-subtle flex items-center justify-between">
              <div class="flex items-center space-x-3 overflow-x-auto scrollbar-hide">
                <Terminal class="w-4 h-4 flex-shrink-0" />
                <code class="whitespace-nowrap select-all">{getTokenCommand()}</code>
              </div>
              <button
                onClick={copyCommand}
                class="ml-4 flex-shrink-0 p-2 rounded-md bg-surface hover:bg-slate-700 text-slate-300 hover:text-white transition-colors focus:outline-none focus:ring-0"
                title="Copy command"
              >
                <Show when={copied()} fallback={<Copy class="w-4 h-4" />}>
                  <Check class="w-4 h-4 text-emerald-400" />
                </Show>
              </button>
            </div>
            <div class="mt-2 flex items-center gap-2 text-[11px] text-muted">
              <span class="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 font-semibold text-blue-700 dark:bg-blue-950/40 dark:text-blue-300">
                {deploymentLabel()}
              </span>
            </div>
          </div>

          <div class="mb-5 text-left">
            <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
              What this token does
            </div>
            <p class="mt-1 text-sm text-muted">{unlockHelp()}</p>
          </div>

          <div class="space-y-3">
              <input
                type="text"
                value={props.bootstrapToken}
                onInput={(e) => {
                  const val = e.currentTarget.value;
                  props.setBootstrapToken(val);
                  // Premium UX: Auto-submit if we detect a pasted token (length heuristic)
                  if (val.length > 20) {
                    setTimeout(() => {
                      if (props.bootstrapToken === val && !isValidating()) {
                        handleUnlock();
                      }
                    }, 400);
                  }
                }}
                onKeyDown={(e) => e.key === 'Enter' && void handleUnlock()}
                class="w-full px-5 py-3.5 bg-surface border border-border rounded-md text-base-content placeholder-slate-400 focus:outline-none focus:ring-0 focus:border-blue-500 transition-colors font-mono"
                placeholder="Paste your bootstrap token"
                autofocus
              />

              <p class="text-xs text-muted">
                After Pulse verifies this token, the next step is creating the admin account for
                this server.
              </p>
              <Show when={looksLikeBootstrapTokenSnapshot(props.bootstrapToken)}>
                <p class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-100">
                  {snapshotPasteHelp}
                </p>
              </Show>

              <button
                onClick={handleUnlock}
                disabled={isValidating() || !props.bootstrapToken.trim()}
                class="w-full py-3.5 px-6 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:bg-surface-alt disabled:text-muted disabled:cursor-not-allowed text-white font-medium rounded-md transition-colors flex justify-center items-center gap-2 duration-200"
              >
                {isValidating() ? (
                  <>
                    <svg
                      class="animate-spin -ml-1 mr-2 h-5 w-5 text-white"
                      fill="none"
                      viewBox="0 0 24 24"
                    >
                      <circle
                        class="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        stroke-width="4"
                      ></circle>
                      <path
                        class="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      ></path>
                    </svg>
                    Verifying bootstrap token...
                  </>
                ) : (
                  'Verify bootstrap token →'
                )}
              </button>
            </div>
        </div>
      </Show>

      <Show when={props.isUnlocked}>
        <div>
          <button
            onClick={props.onNext}
            class="py-4 px-10 bg-blue-600 hover:bg-blue-700 text-white text-lg font-medium rounded-md transition-colors duration-200"
          >
            Continue to Security →
          </button>
        </div>
      </Show>
    </div>
  );
};
