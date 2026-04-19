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
            class="fixed z-[1001] max-h-[min(90vh,42rem)] overflow-y-auto rounded-2xl border border-border bg-surface shadow-2xl focus:outline-none"
            style={state.panelStyle()}
            onClick={(event) => event.stopPropagation()}
          >
            <div class="flex items-start justify-between border-b border-border px-5 py-4">
              <div>
                <div class="text-xs font-semibold uppercase tracking-[0.18em] text-blue-600 dark:text-blue-300">
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

            <div class="space-y-5 px-5 py-5">
              <div class={`rounded-2xl border p-4 ${step().accent}`}>
                <div class="flex items-center gap-2 text-sm font-semibold text-inherit">
                  <WhatsNewFeatureIcon card={step()} />
                  {step().title}
                </div>
                <p class="mt-2 text-sm leading-6 text-inherit">{step().description}</p>
              </div>

              <div class="grid gap-2 sm:grid-cols-5">
                <For each={WHATS_NEW_FEATURE_CARDS}>
                  {(card, index) => (
                    <div
                      class={`rounded-xl border px-2.5 py-2 text-left text-xs ${
                        index() === state.stepIndex()
                          ? 'border-blue-300 bg-blue-50 text-blue-900 dark:border-blue-700 dark:bg-blue-950 dark:text-blue-100'
                          : 'border-border bg-surface-hover text-muted'
                      }`}
                    >
                      {card.title}
                    </div>
                  )}
                </For>
              </div>

              <div class="rounded-xl border border-sky-200 bg-sky-50 p-3 dark:border-sky-800 dark:bg-sky-900/40">
                <div class="flex items-center gap-2 text-sm font-medium text-sky-900 dark:text-sky-100">
                  <ChartBarIcon class="h-4 w-4 flex-shrink-0" />
                  {WHATS_NEW_TELEMETRY_TITLE}
                </div>
                <p class="mt-1.5 text-xs leading-5 text-sky-900 dark:text-sky-200">
                  {WHATS_NEW_TELEMETRY_COPY[0]}
                </p>
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

              <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
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
                    class="inline-flex items-center gap-1 text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    {WHATS_NEW_DOCS_LABEL}
                    <ExternalLinkIcon class="h-4 w-4" />
                  </a>
                </div>
              </div>
            </div>

            <div class="flex items-center justify-between border-t border-border bg-surface-hover px-5 py-4">
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
                  class="rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {WHATS_NEW_BACK_LABEL}
                </button>
                <button
                  onClick={state.handleNext}
                  class="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
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
