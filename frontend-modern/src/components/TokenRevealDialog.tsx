import { Show, createSignal, createEffect, onCleanup } from 'solid-js';
import type { Component } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { copyToClipboard } from '@/utils/clipboard';
import { showError, showSuccess } from '@/utils/toast';
import { useTokenRevealState, dismissTokenReveal } from '@/stores/tokenReveal';

const formatSourceLabel = (value?: string) => {
  if (!value) return '';
  return value
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(' ');
};

export const TokenRevealDialog: Component = () => {
  const state = useTokenRevealState();
  const [copied, setCopied] = createSignal(false);

  const handleDismiss = () => {
    dismissTokenReveal();
    setCopied(false);
  };

  const handleCopy = async (token: string) => {
    const success = await copyToClipboard(token);
    if (success) {
      setCopied(true);
      showSuccess('Token copied to clipboard');
      setTimeout(() => setCopied(false), 2000);
    } else {
      showError('Failed to copy token');
    }
  };

  createEffect(() => {
    const current = state();
    setCopied(false);
    if (!current) {
      return;
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        handleDismiss();
      }
    };

    window.addEventListener('keydown', onKeyDown);
    onCleanup(() => window.removeEventListener('keydown', onKeyDown));
  });

  return (
    <Show when={state()}>
      {(tokenInfo) => {
        const info = tokenInfo();
        const sourceLabel = formatSourceLabel(info.source);
        const recordName = info.record?.name?.trim() || 'Untitled token';
        const hint =
          info.note ||
          'Copy this token now; Pulse will not display it again after you close this dialog.';
        const tokenHint =
          info.record?.prefix && info.record?.suffix
            ? `${info.record.prefix}…${info.record.suffix}`
            : info.record?.prefix
            ? `${info.record.prefix}…`
            : info.record?.suffix
            ? `…${info.record.suffix}`
            : null;

        return (
          <div class="fixed inset-0 z-[60] flex items-center justify-center px-4 py-6">
            <div
              class="absolute inset-0 bg-slate-900/70"
              role="presentation"
              onClick={handleDismiss}
            />
            <div class="relative z-[61] w-full max-w-xl">
              <Card padding="lg" class="shadow-sm relative">
                <button
                  type="button"
                  class="absolute top-3 right-3 rounded-md px-2 py-1 text-xs font-medium text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200 transition-colors"
                  onClick={handleDismiss}
                  aria-label="Close token dialog"
                >
                  Close
                </button>

                <div class="flex items-start gap-3">
                  <div class="flex-shrink-0 rounded-full bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300 p-2">
                    <svg class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                  </div>
                  <div class="space-y-2 flex-1">
                    <div class="flex flex-wrap items-center gap-2">
                      <h2 class="text-lg font-semibold text-slate-900 dark:text-slate-100">
                        API token ready
                      </h2>
                      <Show when={sourceLabel}>
                        <span class="inline-flex items-center rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide">
                          {sourceLabel}
                        </span>
                      </Show>
                    </div>
                    <p class="text-sm text-slate-700 dark:text-slate-300 leading-snug">{hint}</p>
                  </div>
                </div>

                <div class="mt-5 space-y-3">
                  <div class="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
                    Token value
                  </div>
                  <div class="flex flex-col sm:flex-row sm:items-center gap-2">
                    <code class="flex-1 rounded-md border border-green-300 dark:border-green-700 bg-white dark:bg-slate-900 px-4 py-3 font-mono text-base font-semibold text-slate-900 dark:text-slate-100 break-all">
                      {info.token}
                    </code>
                    <button
                      type="button"
                      onClick={() => handleCopy(info.token)}
                      class="inline-flex items-center justify-center rounded-md bg-green-600 hover:bg-green-700 text-white text-sm font-semibold px-4 py-2 transition-colors shadow-sm"
                    >
                      {copied() ? 'Copied!' : 'Copy token'}
                    </button>
                  </div>
                  <div class="text-xs text-slate-600 dark:text-slate-400">
                    Label: <span class="font-semibold text-slate-800 dark:text-slate-200">{recordName}</span>
                    <Show when={tokenHint}>
                      <span>
                        {' '}
                        · Hint:{' '}
                        <code class="rounded bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 font-mono text-[11px] text-slate-600 dark:text-slate-400">
                          {tokenHint as string}
                        </code>
                      </span>
                    </Show>
                  </div>
                </div>

                <div class="mt-6 flex justify-end">
                  <button
                    type="button"
                    onClick={handleDismiss}
                    class="rounded-md border border-slate-300 dark:border-slate-600 px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors"
                  >
                    Dismiss
                  </button>
                </div>
              </Card>
            </div>
          </div>
        );
      }}
    </Show>
  );
};

export default TokenRevealDialog;
