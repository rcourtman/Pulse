import { Show, createSignal, createEffect, onCleanup } from 'solid-js';
import type { Component } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import { Dialog } from '@/components/shared/Dialog';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import { getSourcePlatformLabel } from '@/utils/sourcePlatforms';
import { copyToClipboard } from '@/utils/clipboard';
import { showSuccess } from '@/utils/toast';
import { useTokenRevealState, dismissTokenReveal } from '@/stores/tokenReveal';

type CopyState = 'idle' | 'pending' | 'copied' | 'failed';

export const TokenRevealDialog: Component = () => {
  const state = useTokenRevealState();
  const [copyState, setCopyState] = createSignal<CopyState>('idle');
  let copyGeneration = 0;
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;

  const handleDismiss = () => {
    dismissTokenReveal();
  };

  const handleCopy = async (token: string) => {
    const current = state();
    if (!current || current.token !== token || copyState() === 'pending') return;
    const generation = ++copyGeneration;
    if (copiedTimer) clearTimeout(copiedTimer);
    copiedTimer = undefined;
    setCopyState('pending');
    const success = await copyToClipboard(token);
    // A clipboard write already requested by the operator cannot be cancelled.
    // Its completion must not confirm a different or dismissed reveal.
    if (generation !== copyGeneration || state() !== current) return;
    if (success) {
      setCopyState('copied');
      showSuccess('Token copied to clipboard');
      copiedTimer = setTimeout(() => setCopyState('idle'), 2000);
    } else {
      setCopyState('failed');
    }
  };

  onCleanup(() => {
    copyGeneration += 1;
    if (copiedTimer) clearTimeout(copiedTimer);
  });

  createEffect(() => {
    const current = state();
    copyGeneration += 1;
    if (copiedTimer) clearTimeout(copiedTimer);
    copiedTimer = undefined;
    setCopyState('idle');
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
    <Show when={state()} keyed>
      {(info) => {
        let tokenText: HTMLElement | undefined;
        const selectTokenText = () => {
          if (state() !== info || !tokenText?.isConnected) return;
          const document = tokenText.ownerDocument;
          const selection = document.getSelection();
          if (!selection) return;
          const range = document.createRange();
          range.selectNodeContents(tokenText);
          selection.removeAllRanges();
          selection.addRange(range);
        };
        const sourceBadge = getSourcePlatformBadge(info.source);
        const sourceLabel = sourceBadge?.label || getSourcePlatformLabel(info.source);
        const recordName = info.record?.name?.trim() || 'Untitled token';
        const hint =
          info.note ||
          'Copy this token now. Pulse will not display it again after you close this dialog.';
        const tokenHint =
          info.record?.prefix && info.record?.suffix
            ? `${info.record.prefix}…${info.record.suffix}`
            : info.record?.prefix
              ? `${info.record.prefix}…`
              : info.record?.suffix
                ? `…${info.record.suffix}`
                : null;

        return (
          <Dialog
            isOpen={true}
            onClose={handleDismiss}
            panelClass="max-w-xl"
            ariaLabel="API token ready"
          >
            <div class="w-full min-h-0 overflow-y-auto overscroll-contain">
              <Card padding="lg" class="shadow-xs relative">
                <div class="flex items-start gap-3">
                  <div class="hidden sm:block shrink-0 rounded-full bg-green-100 text-green-700 dark:bg-green-900/25 dark:text-green-300 p-2">
                    <svg class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                  </div>
                  <div class="min-w-0 space-y-2 flex-1">
                    <div class="flex flex-wrap items-center gap-2">
                      <h2 class="text-lg font-semibold text-base-content [overflow-wrap:anywhere]">
                        API token ready
                      </h2>
                      <Show when={sourceLabel}>
                        <span
                          class={`max-w-full [overflow-wrap:anywhere] ${
                            sourceBadge?.classes.replace(
                              'whitespace-nowrap',
                              'whitespace-normal',
                            ) ||
                            'inline-flex items-center rounded-sm px-2 py-0.5 text-[10px] font-medium bg-surface-alt text-base-content'
                          }`}
                        >
                          {sourceLabel}
                        </span>
                      </Show>
                    </div>
                  </div>
                  <button
                    type="button"
                    class="shrink-0 rounded-md px-2 py-1 text-xs font-medium hover:text-base-content transition-colors"
                    onClick={handleDismiss}
                    aria-label="Close token dialog"
                  >
                    Close
                  </button>
                </div>
                <p class="mt-3 text-sm text-base-content leading-snug [overflow-wrap:anywhere]">
                  {hint}
                </p>

                <div class="mt-5 space-y-3">
                  <div class="text-xs font-semibold uppercase tracking-wide text-muted">
                    Token value
                  </div>
                  <div class="flex flex-col sm:flex-row sm:items-center gap-2">
                    <code
                      ref={tokenText}
                      class="min-w-0 flex-1 rounded-md border border-green-300 dark:border-green-700 bg-surface px-4 py-3 font-mono text-base font-semibold text-base-content break-all"
                    >
                      {info.token}
                    </code>
                    <button
                      type="button"
                      onClick={() => handleCopy(info.token)}
                      disabled={copyState() === 'pending'}
                      class="disabled:cursor-wait disabled:opacity-60 inline-flex items-center justify-center rounded-md bg-green-600 hover:bg-green-700 text-white text-sm font-semibold px-4 py-2 transition-colors shadow-xs"
                    >
                      {copyState() === 'pending'
                        ? 'Copying…'
                        : copyState() === 'copied'
                          ? 'Copied!'
                          : 'Copy token'}
                    </button>
                  </div>
                  <Show when={copyState() === 'failed'}>
                    <div class="space-y-2">
                      <p role="alert" class="text-sm text-red-700 dark:text-red-300">
                        Could not copy the token automatically. Select the token text, then use your
                        browser’s Copy command.
                      </p>
                      <Button
                        variant="secondary"
                        size="sm"
                        class="min-h-11 sm:min-h-0"
                        onClick={selectTokenText}
                      >
                        Select token text
                      </Button>
                    </div>
                  </Show>
                  <div class="text-xs text-muted [overflow-wrap:anywhere]">
                    Label: <span class="font-semibold text-base-content">{recordName}</span>
                    <Show when={tokenHint}>
                      <span>
                        {' '}
                        · Hint:{' '}
                        <code class="rounded-sm bg-surface-alt px-1.5 py-0.5 font-mono text-[11px] text-muted break-all">
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
                    class="rounded-md border border-border px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover transition-colors"
                  >
                    Dismiss
                  </button>
                </div>
              </Card>
            </div>
          </Dialog>
        );
      }}
    </Show>
  );
};

export default TokenRevealDialog;
