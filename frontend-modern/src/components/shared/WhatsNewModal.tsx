import { For, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import LayoutDashboardIcon from 'lucide-solid/icons/layout-dashboard';
import ServerIcon from 'lucide-solid/icons/server';
import BoxesIcon from 'lucide-solid/icons/boxes';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import ChartBarIcon from 'lucide-solid/icons/chart-bar';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import XIcon from 'lucide-solid/icons/x';
import {
  WHATS_NEW_BACK_LABEL,
  WHATS_NEW_CLOSE_LABEL,
  WHATS_NEW_DOCS_LABEL,
  WHATS_NEW_DOCS_URL,
  WHATS_NEW_DO_NOT_SHOW_LABEL,
  WHATS_NEW_FEATURE_CARDS,
  WHATS_NEW_NEXT_LABEL,
  WHATS_NEW_PRIMARY_ACTION_LABEL,
  WHATS_NEW_PRIVACY_URL,
  WHATS_NEW_SKIP_LABEL,
  WHATS_NEW_SUBTITLE,
  WHATS_NEW_TELEMETRY_COPY,
  WHATS_NEW_TELEMETRY_ENV_VAR,
  WHATS_NEW_TELEMETRY_PRIVACY_LABEL,
  WHATS_NEW_TELEMETRY_SETTINGS_PATH,
  WHATS_NEW_TELEMETRY_TITLE,
  WHATS_NEW_TITLE,
  type WhatsNewFeatureCard,
} from './whatsNewModalModel';
import { useDialogState } from './useDialogState';
import { useWhatsNewModalState } from './useWhatsNewModalState';

function WhatsNewFeatureIcon(props: { card: WhatsNewFeatureCard }) {
  switch (props.card.icon) {
    case 'dashboard':
      return <LayoutDashboardIcon class="h-4 w-4" />;
    case 'infrastructure':
      return <ServerIcon class="h-4 w-4" />;
    case 'workloads':
      return <BoxesIcon class="h-4 w-4" />;
    case 'storage':
      return <HardDriveIcon class="h-4 w-4" />;
    case 'recovery':
      return <ShieldCheckIcon class="h-4 w-4" />;
  }
}

export function WhatsNewModal() {
  const state = useWhatsNewModalState();
  const dialogState = useDialogState({
    get isOpen() {
      return state.isOpen();
    },
    onClose: state.handleClose,
  });
  const step = () => state.currentStep();
  const setPanelRef = (element: HTMLDivElement) => {
    state.setPanelRef(element);
    dialogState.setPanelRef(element);
  };

  return (
    <Show when={state.isOpen()}>
      <Portal mount={document.body}>
        <div class="fixed inset-0 z-[1000]">
          <div class="absolute inset-0" data-dialog-backdrop onClick={dialogState.handleBackdropClick} />
          <Show when={state.spotlightStyle()}>
            {(style) => (
              <div
                data-tour-spotlight=""
                data-tour-step={step().target}
                class="pointer-events-none absolute rounded-[1.25rem] border border-blue-300/80 bg-white/5 transition-all duration-200 dark:border-blue-300/60"
                style={style()}
              />
            )}
          </Show>
          <div
            ref={setPanelRef}
            data-tour-panel=""
            data-tour-step={step().target}
            role="dialog"
            aria-modal="true"
            aria-labelledby="whats-new-title"
            tabindex="-1"
            class="fixed z-[1001] max-h-[min(90vh,44rem)] overflow-y-auto rounded-[1.75rem] border border-border bg-surface shadow-2xl backdrop-blur-sm focus:outline-none"
            style={state.panelStyle()}
            onClick={(event) => event.stopPropagation()}
          >
            <div class="flex items-start justify-between border-b border-border bg-gradient-to-r from-blue-50/90 via-surface to-slate-50/80 px-6 py-5 dark:from-blue-950/30 dark:via-surface dark:to-slate-950/40">
              <div>
                <div class="inline-flex items-center rounded-full border border-blue-200 bg-white/80 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-blue-700 shadow-sm dark:border-blue-800 dark:bg-slate-950/60 dark:text-blue-200">
                  Guided Welcome Tour
                </div>
                <div class="mt-3 text-xs font-semibold uppercase tracking-[0.18em] text-blue-600 dark:text-blue-300">
                  Step {state.stepIndex() + 1} of {state.stepCount()}
                </div>
                <h2 id="whats-new-title" class="mt-1 text-xl font-semibold text-base-content">
                  {WHATS_NEW_TITLE}
                </h2>
                <p class="mt-1 text-sm text-muted">{WHATS_NEW_SUBTITLE}</p>
              </div>
              <button
                onClick={state.handleClose}
                class="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-surface-hover hover:text-muted"
                aria-label={WHATS_NEW_CLOSE_LABEL}
                type="button"
              >
                <XIcon class="h-5 w-5" />
              </button>
            </div>

            <div class="space-y-5 px-6 py-5">
              <div class={`rounded-[1.5rem] border p-5 shadow-sm ${step().accent}`}>
                <div class="flex items-start gap-4">
                  <div class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-2xl bg-white/80 text-inherit shadow-sm dark:bg-slate-950/40">
                    <WhatsNewFeatureIcon card={step()} />
                  </div>
                  <div class="min-w-0">
                    <div class="text-[11px] font-semibold uppercase tracking-[0.16em] opacity-70">
                      Now showing
                    </div>
                    <div class="mt-1 text-base font-semibold text-inherit">{step().title}</div>
                    <p class="mt-2 text-sm leading-6 text-inherit">{step().description}</p>
                  </div>
                </div>
              </div>

              <div class="space-y-2.5">
                <div class="flex items-center gap-3">
                  <div class="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted">
                    In This Tour
                  </div>
                  <div class="h-px flex-1 bg-border"></div>
                </div>
                <p class="text-xs text-muted">Jump ahead or follow the highlighted path.</p>
                <div class="grid grid-cols-2 gap-2.5">
                  <For each={WHATS_NEW_FEATURE_CARDS}>
                    {(card, index) => (
                      <button
                        type="button"
                        onClick={() => state.handleSelectStep(index())}
                        aria-current={index() === state.stepIndex() ? 'step' : undefined}
                        class={`group min-h-[3.35rem] rounded-2xl border px-3.5 py-3 text-left transition-colors ${
                          index() === state.stepIndex()
                            ? 'border-blue-300 bg-gradient-to-br from-blue-50 to-white text-blue-900 shadow-sm dark:border-blue-700 dark:from-blue-950 dark:to-slate-950 dark:text-blue-100'
                            : 'border-border bg-surface-hover text-base-content hover:border-slate-300 hover:bg-surface dark:hover:border-slate-700'
                        }`}
                      >
                        <div class="flex items-center gap-3">
                          <div
                            class={`flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-[11px] font-semibold font-mono ${
                              index() === state.stepIndex()
                                ? 'bg-white/85 text-blue-700 shadow-sm dark:bg-slate-950/50 dark:text-blue-200'
                                : 'bg-surface text-muted'
                            }`}
                          >
                            {String(index() + 1).padStart(2, '0')}
                          </div>
                          <div class="min-w-0 flex-1 truncate text-sm font-semibold">{card.title}</div>
                        </div>
                      </button>
                    )}
                  </For>
                </div>
              </div>

              <div class="rounded-[1.35rem] border border-sky-200 bg-gradient-to-br from-sky-50/95 to-white p-4 dark:border-sky-800 dark:from-sky-900/45 dark:to-slate-950">
                <div class="flex items-start gap-2.5">
                  <div class="mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-xl bg-white/70 text-sky-800 shadow-sm dark:bg-slate-950/40 dark:text-sky-100">
                    <ChartBarIcon class="h-4 w-4" />
                  </div>
                  <div>
                    <div class="text-[11px] font-semibold uppercase tracking-[0.16em] text-sky-700 dark:text-sky-300">
                      Privacy note
                    </div>
                    <div class="mt-1 text-sm font-medium text-sky-900 dark:text-sky-100">
                      {WHATS_NEW_TELEMETRY_TITLE}
                    </div>
                    <p class="mt-1.5 text-xs leading-5 text-sky-900 dark:text-sky-200">
                      {WHATS_NEW_TELEMETRY_COPY[0]}
                    </p>
                  </div>
                </div>
                <p class="mt-1.5 text-xs leading-5 text-sky-900 dark:text-sky-200">
                  {WHATS_NEW_TELEMETRY_COPY[1]} You can disable it any time in{' '}
                  <span class="font-medium">{WHATS_NEW_TELEMETRY_SETTINGS_PATH}</span> or by
                  setting{' '}
                  <code class="rounded bg-sky-100 px-1 py-0.5 text-[10px] font-mono dark:bg-sky-800">
                    {WHATS_NEW_TELEMETRY_ENV_VAR}
                  </code>
                  .{' '}
                  <a
                    href={WHATS_NEW_PRIVACY_URL}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="underline hover:text-sky-700 dark:hover:text-sky-100"
                  >
                    {WHATS_NEW_TELEMETRY_PRIVACY_LABEL}
                  </a>
                </p>
              </div>

              <div class="flex flex-col gap-3 rounded-2xl border border-border bg-surface-hover/70 p-3.5 sm:flex-row sm:items-center sm:justify-between">
                <label class="flex items-center gap-2 text-sm text-muted">
                  <input
                    type="checkbox"
                    checked={state.dontShowAgain()}
                    onChange={(event) => state.setDontShowAgain(event.currentTarget.checked)}
                    class="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-2 focus:ring-blue-500"
                  />
                  {WHATS_NEW_DO_NOT_SHOW_LABEL}
                </label>

                <div class="flex items-center gap-4">
                  <a
                    href={WHATS_NEW_DOCS_URL}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex items-center gap-1 rounded-full border border-blue-200 bg-white px-3 py-1.5 text-sm font-medium text-blue-700 shadow-sm transition-colors hover:border-blue-300 hover:text-blue-800 dark:border-blue-900 dark:bg-slate-950 dark:text-blue-300 dark:hover:border-blue-700 dark:hover:text-blue-200"
                  >
                    {WHATS_NEW_DOCS_LABEL}
                    <ExternalLinkIcon class="h-4 w-4" />
                  </a>
                </div>
              </div>
            </div>

            <div class="flex items-center justify-between border-t border-border bg-surface-hover px-6 py-4">
              <button
                type="button"
                onClick={state.handleClose}
                class="text-sm font-medium text-muted transition-colors hover:text-base-content"
              >
                {WHATS_NEW_SKIP_LABEL}
              </button>
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  onClick={state.handlePrevious}
                  disabled={state.isFirstStep()}
                  class="rounded-xl border border-border px-3.5 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {WHATS_NEW_BACK_LABEL}
                </button>
                <button
                  onClick={state.handleNext}
                  class="rounded-xl bg-blue-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-blue-700"
                  type="button"
                >
                  {state.isLastStep() ? WHATS_NEW_PRIMARY_ACTION_LABEL : WHATS_NEW_NEXT_LABEL}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Portal>
    </Show>
  );
}

export default WhatsNewModal;
