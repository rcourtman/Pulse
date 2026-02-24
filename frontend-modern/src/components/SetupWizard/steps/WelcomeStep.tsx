import { Component, createSignal, Show } from 'solid-js';
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
      }>('/api/security/status');
      if (data?.bootstrapTokenPath) {
        setTokenPath(data.bootstrapTokenPath);
        setIsDocker(data.isDocker || false);
        setInContainer(data.inContainer || false);
        setLxcCtid(data.lxcCtid || '');
      }
    } catch (error) {
      logger.error('Failed to fetch bootstrap info:', error);
    }
  };

  // Call on component load
  fetchBootstrapInfo();

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
      return `docker exec <container> cat ${path}`;
    }
    if (inContainer() && lxcCtid()) {
      return `pct exec ${lxcCtid()} -- cat ${path}`;
    }
    if (inContainer()) {
      return `pct exec <ctid> -- cat ${path}`;
    }
    return `cat ${path}`;
  };

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
      </div>

      {/* Bootstrap token unlock */}
      <Show when={!props.isUnlocked}>
        <div class="p-8 max-w-lg mx-auto bg-surface border border-border rounded-md text-left animate-slide-up delay-300 relative group">
          <div class="relative z-10">
            <h3 class="text-xl font-semibold text-base-content mb-2 tracking-tight">
              Unlock Setup
            </h3>
            <p class="text-sm text-muted mb-6">
              Run the following command on your host to retrieve the secure bootstrap token:
            </p>

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
                onKeyPress={(e) => e.key === 'Enter' && handleUnlock()}
                class="w-full px-5 py-3.5 bg-surface border border-border rounded-md text-base-content placeholder-slate-400 focus:outline-none focus:ring-0 focus:border-blue-500 transition-colors font-mono"
                placeholder="Paste your bootstrap token"
                autofocus
              />

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
                    Validating...
                  </>
                ) : (
                  'Continue to Setup →'
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
            Get Started →
          </button>
        </div>
      </Show>
    </div>
  );
};
