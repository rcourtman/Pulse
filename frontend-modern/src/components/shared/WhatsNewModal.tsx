import { For, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import LayoutDashboardIcon from 'lucide-solid/icons/layout-dashboard';
import ServerIcon from 'lucide-solid/icons/server';
import BoxesIcon from 'lucide-solid/icons/boxes';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import XIcon from 'lucide-solid/icons/x';
import {
  WHATS_NEW_BACK_LABEL,
  WHATS_NEW_CLOSE_LABEL,
  WHATS_NEW_DOCS_LABEL,
  WHATS_NEW_DOCS_URL,
  WHATS_NEW_DO_NOT_SHOW_LABEL,
  WHATS_NEW_FEATURE_CARDS,
  WHATS_NEW_KICKER_LABEL,
  WHATS_NEW_NEXT_LABEL,
  WHATS_NEW_PRIMARY_ACTION_LABEL,
  WHATS_NEW_PROGRESS_PREFIX,
  WHATS_NEW_PRIVACY_URL,
  WHATS_NEW_TELEMETRY_LINK_LABEL,
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
                class="pointer-events-none absolute rounded-[1.25rem] border border-blue-300/80 bg-white/10 transition-all duration-200 dark:border-blue-300/60"
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
            aria-label={WHATS_NEW_TITLE}
            tabindex="-1"
            class="fixed z-[1001] max-h-[min(90vh,32rem)] overflow-y-auto rounded-md border border-border bg-surface shadow-xl focus:outline-none"
            style={state.panelStyle()}
            onClick={(event) => event.stopPropagation()}
          >
            <div class="flex items-start justify-between border-b border-border bg-surface px-5 py-4">
              <div class="min-w-0">
                <div class="flex items-center gap-2 text-[10px] font-semibold uppercase tracking-[0.18em] text-muted">
                  <span class="inline-flex items-center rounded-full border border-border bg-surface-hover px-2 py-1">
                    {WHATS_NEW_KICKER_LABEL}
                  </span>
                  <span>
                    {WHATS_NEW_PROGRESS_PREFIX} {state.stepIndex() + 1} of {state.stepCount()}
                  </span>
                </div>
                <div class="mt-3 flex items-start gap-3">
                  <div class={`flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md border bg-surface ${step().accent}`}>
                    <WhatsNewFeatureIcon card={step()} />
                  </div>
                  <div class="min-w-0">
                    <div class="text-lg font-semibold text-base-content">{step().title}</div>
                    <p class="mt-1 text-sm leading-6 text-muted">{step().description}</p>
                  </div>
                </div>
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

            <div class="space-y-4 px-5 py-4">
              <div class="flex flex-wrap gap-2">
                <For each={WHATS_NEW_FEATURE_CARDS}>
                  {(card, index) => (
                    <button
                      type="button"
                      onClick={() => state.handleSelectStep(index())}
                      aria-current={index() === state.stepIndex() ? 'step' : undefined}
                      class={`inline-flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs font-medium transition-colors ${
                        index() === state.stepIndex()
                          ? 'border-blue-300 bg-blue-50 text-blue-900 dark:border-blue-700 dark:bg-blue-950 dark:text-blue-100'
                          : 'border-border bg-surface-hover text-base-content hover:border-slate-300 hover:bg-surface dark:hover:border-slate-700'
                      }`}
                    >
                      <span
                        class={`inline-flex h-5 w-5 items-center justify-center rounded-sm border text-[10px] font-semibold font-mono ${
                          index() === state.stepIndex()
                            ? 'border-blue-200 bg-surface text-blue-700 dark:border-blue-800 dark:bg-slate-900 dark:text-blue-200'
                            : 'border-border bg-surface text-muted'
                        }`}
                      >
                        {String(index() + 1).padStart(2, '0')}
                      </span>
                      <span>{card.title}</span>
                    </button>
                  )}
                </For>
              </div>

              <div class="flex flex-col gap-3 border-t border-border pt-3 sm:flex-row sm:items-center sm:justify-between">
                <div class="flex flex-wrap items-center gap-3 text-xs text-muted">
                  <label class="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={state.dontShowAgain()}
                      onChange={(event) => state.setDontShowAgain(event.currentTarget.checked)}
                      class="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-2 focus:ring-blue-500"
                    />
                    {WHATS_NEW_DO_NOT_SHOW_LABEL}
                  </label>
                  <a
                    href={WHATS_NEW_DOCS_URL}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="underline hover:text-base-content"
                  >
                    {WHATS_NEW_DOCS_LABEL}
                  </a>
                  <a
                    href={WHATS_NEW_PRIVACY_URL}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="underline hover:text-base-content"
                  >
                    {WHATS_NEW_TELEMETRY_LINK_LABEL}
                  </a>
                </div>

                <div class="flex items-center gap-2 sm:justify-end">
                  <button
                    type="button"
                    onClick={state.handlePrevious}
                    disabled={state.isFirstStep()}
                    class="rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {WHATS_NEW_BACK_LABEL}
                  </button>
                  <button
                    onClick={state.handleNext}
                    class="rounded-md bg-blue-600 px-3.5 py-1.5 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
                    type="button"
                  >
                    {state.isLastStep() ? WHATS_NEW_PRIMARY_ACTION_LABEL : WHATS_NEW_NEXT_LABEL}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Portal>
    </Show>
  );
}

export default WhatsNewModal;
