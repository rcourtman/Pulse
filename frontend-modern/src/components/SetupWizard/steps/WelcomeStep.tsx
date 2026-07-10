import { Component, createSignal, onCleanup, For, Show } from 'solid-js';
import { t } from '@/i18n';
import { showError, showSuccess } from '@/utils/toast';
import { apiFetch } from '@/utils/apiClient';
import { copyToClipboard } from '@/utils/clipboard';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { PRIVACY_DOC_URL } from '@/utils/docsLinks';
import { Copy, Check, Terminal, ShieldCheck } from 'lucide-solid';

interface WelcomeStepProps {
  onNext: () => void;
  bootstrapToken: string;
  setBootstrapToken: (token: string) => void;
  isUnlocked: boolean;
  setIsUnlocked: (unlocked: boolean) => void;
}

export const WelcomeStep: Component<WelcomeStepProps> = (props) => {
  const [isValidating, setIsValidating] = createSignal(false);
  const [copiedCommand, setCopiedCommand] = createSignal<string | null>(null);
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;
  let unlockInFlight = false;

  onCleanup(() => {
    if (copiedTimer) clearTimeout(copiedTimer);
  });

  const bootstrapCommands = () => [
    {
      id: 'host',
      label: t('setup.welcome.deploymentLabel.direct'),
      command: 'sudo pulse bootstrap-token',
    },
    {
      id: 'docker',
      label: t('setup.welcome.deploymentLabel.docker'),
      command: 'docker exec <pulse-container> /app/pulse bootstrap-token',
    },
    {
      id: 'lxc',
      label: t('setup.welcome.deploymentLabel.lxc'),
      command: 'pct exec <ctid> -- pulse bootstrap-token',
    },
  ];

  const copyCommand = async (id: string, command: string) => {
    const copied = await copyToClipboard(command);
    if (copied) {
      setCopiedCommand(id);
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => setCopiedCommand(null), 2000);
      showSuccess(t('setup.welcome.success.commandCopied'));
    } else {
      showError(t('setup.welcome.error.copyCommandFailed'));
    }
  };

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
    if (unlockInFlight) {
      return;
    }

    const trimmedToken = props.bootstrapToken.trim();
    if (!trimmedToken) {
      showError(t('setup.welcome.error.missingBootstrapToken'));
      return;
    }
    if (looksLikeBootstrapTokenSnapshot(trimmedToken)) {
      showError(snapshotPasteHelp());
      return;
    }

    unlockInFlight = true;
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
      unlockInFlight = false;
      setIsValidating(false);
    }
  };

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

      <div class="mb-6 max-w-2xl mx-auto rounded-md border border-border bg-surface px-4 py-3 text-left">
        <div class="flex items-start gap-3">
          <ShieldCheck class="mt-0.5 h-4 w-4 shrink-0 text-blue-500" strokeWidth={2} />
          <div class="min-w-0">
            <p class="text-sm font-medium text-base-content">
              {t('setup.welcome.telemetryNotice.title')}
            </p>
            <p class="mt-1 text-xs leading-5 text-muted">
              {t('setup.welcome.telemetryNotice.description')}{' '}
              <ExternalTextLink href={PRIVACY_DOC_URL} variant="muted">
                {t('setup.welcome.telemetryNotice.detailsLink')}
              </ExternalTextLink>
            </p>
          </div>
        </div>
      </div>

      <Show when={!props.isUnlocked}>
        <div class="p-6 sm:p-8 max-w-2xl mx-auto bg-surface border border-border rounded-md text-left">
          <h3 class="text-lg font-semibold text-base-content mb-2 tracking-tight">
            {t('setup.welcome.unlockTitle')}
          </h3>
          <p class="text-sm text-muted mb-5">{t('setup.welcome.deploymentHint.choose')}</p>

          <div class="mb-5 space-y-2">
            <For each={bootstrapCommands()}>
              {(item) => (
                <div class="bg-base rounded-md border border-border-subtle px-3 py-2.5">
                  <div class="mb-1.5 text-[11px] font-semibold text-muted">{item.label}</div>
                  <div class="flex items-center justify-between gap-3 font-mono text-sm text-emerald-400">
                    <div class="flex min-w-0 items-center space-x-3 overflow-x-auto scrollbar-hide">
                      <Terminal class="h-4 w-4 flex-shrink-0" />
                      <code class="whitespace-nowrap select-all">{item.command}</code>
                    </div>
                    <button
                      onClick={() => void copyCommand(item.id, item.command)}
                      class="flex-shrink-0 rounded-md bg-surface p-2 text-slate-300 transition-colors hover:bg-slate-700 hover:text-white focus:outline-none focus:ring-0"
                      title={`${t('setup.welcome.copyCommandTitle')}: ${item.label}`}
                    >
                      <Show when={copiedCommand() === item.id} fallback={<Copy class="h-4 w-4" />}>
                        <Check class="h-4 w-4 text-emerald-400" />
                      </Show>
                    </button>
                  </div>
                </div>
              )}
            </For>
          </div>

          <div class="mb-5 text-left">
            <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
              {t('setup.welcome.tokenHelp.title')}
            </div>
            <p class="mt-1 text-sm text-muted">{t('setup.welcome.tokenHelp.generic')}</p>
          </div>

          <div class="space-y-3">
            <input
              type="text"
              value={props.bootstrapToken}
              onInput={(e) => {
                props.setBootstrapToken(e.currentTarget.value);
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
