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
  const [tokenPath, setTokenPath] = createSignal('');
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
      if (data?.bootstrapTokenPath) {
        setTokenPath(data.bootstrapTokenPath);
        setIsDocker(data.isDocker || false);
        setInContainer(data.inContainer || false);
        setLxcCtid(data.lxcCtid || '');
        setDockerContainerName(data.dockerContainerName || '');
      }
    } catch (error) {
      logger.error('Failed to fetch bootstrap info:', error);
    }
  };

  onMount(() => {
    void fetchBootstrapInfo();
  });

  const handleUnlock = async () => {
    if (!props.bootstrapToken.trim()) {
      showError('Please enter the bootstrap token');
      return;
    }

    setIsValidating(true);
    try {
      const response = await apiFetch('/api/security/validate-bootstrap-token', {
        method: 'POST',
        body: JSON.stringify({ token: props.bootstrapToken.trim() }),
      });

      if (!response.ok) {
        throw new Error('Invalid bootstrap token');
      }

      props.setIsUnlocked(true);
      showSuccess('Token verified!');
      props.onNext();
    } catch (_error) {
      showError('Invalid bootstrap token. Please check and try again.');
    } finally {
      setIsValidating(false);
    }
  };

  const getTokenCommand = () => {
    const path = tokenPath() || '/etc/pulse/.bootstrap_token';
    if (isDocker()) {
      return `docker exec ${dockerContainerName() || '<pulse-container>'} cat ${path}`;
    }
    if (inContainer() && lxcCtid()) {
      return `pct exec ${lxcCtid()} -- cat ${path}`;
    }
    if (inContainer()) {
      return `pct exec <ctid> -- cat ${path}`;
    }
    return `cat ${path}`;
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
    const path = tokenPath() || '/etc/pulse/.bootstrap_token';
    if (isDocker()) {
      return dockerContainerName()
        ? `Pulse appears to be running in Docker as container "${dockerContainerName()}". Run the command on the Docker host so you can read ${path} from that container.`
        : `Pulse appears to be running in Docker. Run the command on the Docker host and replace <pulse-container> with the running Pulse container name.`;
    }
    if (inContainer() && lxcCtid()) {
      return `Pulse appears to be running in LXC container ${lxcCtid()}. Run the command on the Proxmox host so you can execute into that container and read ${path}.`;
    }
    if (inContainer()) {
      return `Pulse appears to be running in a containerized environment. Run the command from the host that manages the container so you can read ${path}.`;
    }
    return `Run the command directly in a shell on the Pulse server to read ${path}.`;
  });

  const unlockHelp = createMemo(() => {
    if (isDocker()) {
      return 'This one-time bootstrap token only unlocks first-run setup. After verification, you will create the admin account and Pulse will generate the long-lived API token separately.';
    }
    return 'This one-time bootstrap token only unlocks first-run setup on this Pulse server. It is not your admin password and it is not the API token you will use after setup.';
  });

  return (
    <div class="text-center relative">
      {/* Logo */}
      <div class="mb-10 relative z-10">
        <img
          src="/logo.svg"
          alt="Pulse Logo"
          class="w-24 h-24 rounded-md mb-8 mx-auto dark:shadow-none"
        />
        <h1 class="text-4xl sm:text-5xl font-bold tracking-tight text-base-content mb-4 animate-fade-in delay-100">
          Welcome to Pulse
        </h1>
        <p class="text-xl dark:text-blue-200 font-light animate-fade-in delay-200 max-w-md mx-auto">
          Unified infrastructure intelligence
        </p>
        <p class="mt-4 text-sm text-muted max-w-xl mx-auto animate-fade-in delay-300">
          You are about to do three things: unlock setup on this Pulse server, create your admin
          account, and install the first system you want Pulse to monitor.
        </p>
      </div>

      <div class="mb-8 grid gap-3 text-left sm:grid-cols-3">
        <div class="rounded-md border border-border bg-surface px-4 py-3">
          <div class="text-[11px] font-semibold uppercase tracking-wide text-blue-700 dark:text-blue-300">
            Step 1
          </div>
          <div class="mt-1 text-sm font-semibold text-base-content">Unlock this Pulse server</div>
          <p class="mt-1 text-xs text-muted">
            Read the one-time bootstrap token from the system where Pulse is installed.
          </p>
        </div>
        <div class="rounded-md border border-border bg-surface px-4 py-3">
          <div class="text-[11px] font-semibold uppercase tracking-wide text-blue-700 dark:text-blue-300">
            Step 2
          </div>
          <div class="mt-1 text-sm font-semibold text-base-content">Create the admin account</div>
          <p class="mt-1 text-xs text-muted">
            Set the first login and let Pulse generate the credentials you need to save.
          </p>
        </div>
        <div class="rounded-md border border-border bg-surface px-4 py-3">
          <div class="text-[11px] font-semibold uppercase tracking-wide text-blue-700 dark:text-blue-300">
            Step 3
          </div>
          <div class="mt-1 text-sm font-semibold text-base-content">Install the first host</div>
          <p class="mt-1 text-xs text-muted">
            Open Infrastructure Install and connect the first system you want Pulse to monitor.
          </p>
        </div>
      </div>

      {/* Bootstrap token unlock */}
      <Show when={!props.isUnlocked}>
        <div class="p-8 max-w-lg mx-auto bg-surface border border-border rounded-md text-left animate-slide-up delay-300 relative group">
          <div class="relative z-10">
            <h3 class="text-xl font-semibold text-base-content mb-2 tracking-tight">
              Unlock Setup
            </h3>
            <p class="text-sm text-muted mb-6">
              Run the following command on the Pulse server to retrieve the one-time bootstrap
              token that unlocks this wizard:
            </p>

            <div class="mb-4 rounded-md border border-blue-200 bg-blue-50 px-4 py-3 dark:border-blue-800 dark:bg-blue-950/40">
              <div class="text-[11px] font-semibold uppercase tracking-wide text-blue-700 dark:text-blue-300">
                Where to run it
              </div>
              <div class="mt-1 flex items-center gap-2">
                <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-semibold text-blue-700 dark:bg-blue-900 dark:text-blue-200">
                  {deploymentLabel()}
                </span>
              </div>
              <p class="mt-2 text-sm text-blue-900 dark:text-blue-100">{deploymentHint()}</p>
            </div>

            <div class="mb-8">
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
            </div>

            <div class="mb-6 rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 dark:border-emerald-800 dark:bg-emerald-950/40">
              <div class="text-[11px] font-semibold uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                What this token does
              </div>
              <p class="mt-2 text-sm text-emerald-900 dark:text-emerald-100">{unlockHelp()}</p>
            </div>

            <div class="space-y-4">
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
        </div>
      </Show>

      <Show when={props.isUnlocked}>
        <div class="animate-enter delay-200">
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
