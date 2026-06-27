import { Component, createMemo, createSignal, onMount, onCleanup, Show } from 'solid-js';
import { t } from '@/i18n';
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
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;

  onCleanup(() => {
    if (copiedTimer) clearTimeout(copiedTimer);
  });

  const copyCommand = async () => {
    const copied = await copyToClipboard(getTokenCommand());
    if (copied) {
      setCopied(true);
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => setCopied(false), 2000);
      showSuccess(t('setup.welcome.success.commandCopied'));
    } else {
      showError(t('setup.welcome.error.copyCommandFailed'));
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

  const snapshotPasteHelp = () => t('setup.welcome.error.snapshotPaste');

  const handleUnlock = async () => {
    const trimmedToken = props.bootstrapToken.trim();
    if (!trimmedToken) {
      showError(t('setup.welcome.error.missingBootstrapToken'));
      return;
    }
    if (looksLikeBootstrapTokenSnapshot(trimmedToken)) {
      showError(snapshotPasteHelp());
      return;
    }

    setIsValidating(true);
    try {
      const response = await apiFetch('/api/security/validate-bootstrap-token', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        skipAuth: true,
        skipOrgContext: true,
        body: JSON.stringify({ token: trimmedToken }),
      });

      if (!response.ok) {
        throw new Error(t('setup.welcome.error.invalidBootstrapTokenResponse'));
      }

      props.setIsUnlocked(true);
      props.onNext();
    } catch (_error) {
      showError(t('setup.welcome.error.invalidBootstrapToken'));
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
      return t('setup.welcome.deploymentLabel.docker');
    }
    if (inContainer() && lxcCtid()) {
      return t('setup.welcome.deploymentLabel.lxc');
    }
    if (inContainer()) {
      return t('setup.welcome.deploymentLabel.containerized');
    }
    return t('setup.welcome.deploymentLabel.direct');
  });

  const deploymentHint = createMemo(() => {
    if (isDocker()) {
      return dockerContainerName()
        ? t('setup.welcome.deploymentHint.dockerNamed', {
            containerName: dockerContainerName(),
          })
        : t('setup.welcome.deploymentHint.dockerUnnamed');
    }
    if (inContainer() && lxcCtid()) {
      return t('setup.welcome.deploymentHint.lxc', { ctid: lxcCtid() });
    }
    if (inContainer()) {
      return t('setup.welcome.deploymentHint.containerized');
    }
    return t('setup.welcome.deploymentHint.direct');
  });

  const unlockHelp = createMemo(() => {
    if (isDocker()) {
      return t('setup.welcome.tokenHelp.docker');
    }
    return t('setup.welcome.tokenHelp.host');
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
          {t('setup.welcome.hero.title')}
        </h1>
        <p class="text-base text-muted max-w-xl mx-auto">
          {t('setup.welcome.hero.stepsIntro')}{' '}
          <span class="text-base-content font-medium">{t('setup.welcome.hero.step.unlock')}</span>,{' '}
          <span class="text-base-content font-medium">{t('setup.welcome.hero.step.admin')}</span>,{' '}
          {t('setup.welcome.hero.stepsThen')}{' '}
          <span class="text-base-content font-medium">{t('setup.welcome.hero.step.source')}</span>.
        </p>
        <p class="text-sm text-muted max-w-xl mx-auto mt-2">
          <span>{t('setup.welcome.hero.coverage')}</span>
        </p>
      </div>

      <Show when={!props.isUnlocked}>
        <div class="p-6 sm:p-8 max-w-2xl mx-auto bg-surface border border-border rounded-md text-left">
          <h3 class="text-lg font-semibold text-base-content mb-2 tracking-tight">
            {t('setup.welcome.unlockTitle')}
          </h3>
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
                title={t('setup.welcome.copyCommandTitle')}
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
              {t('setup.welcome.tokenHelp.title')}
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
              placeholder={t('setup.welcome.placeholder.bootstrapToken')}
              autofocus
            />

            <p class="text-xs text-muted">{t('setup.welcome.tokenHelp.afterVerify')}</p>
            <Show when={looksLikeBootstrapTokenSnapshot(props.bootstrapToken)}>
              <p class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-100">
                {snapshotPasteHelp()}
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
                  {t('setup.welcome.action.verifyingToken')}
                </>
              ) : (
                <>
                  {t('setup.welcome.action.verifyToken')} <span>→</span>
                </>
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
            {t('setup.welcome.action.continueSecurity')} <span>→</span>
          </button>
        </div>
      </Show>
    </div>
  );
};
