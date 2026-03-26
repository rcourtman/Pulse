import { For } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import ServerIcon from 'lucide-solid/icons/server';
import BoxesIcon from 'lucide-solid/icons/boxes';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import ChartBarIcon from 'lucide-solid/icons/chart-bar';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import XIcon from 'lucide-solid/icons/x';
import { buildRecoveryPath } from '@/routing/resourceLinks';
import {
  WHATS_NEW_CLOSE_LABEL,
  WHATS_NEW_DOCS_LABEL,
  WHATS_NEW_DOCS_URL,
  WHATS_NEW_DO_NOT_SHOW_LABEL,
  WHATS_NEW_FEATURE_CARDS,
  WHATS_NEW_PRIMARY_ACTION_LABEL,
  WHATS_NEW_PRIVACY_URL,
  WHATS_NEW_RECOVERY_LINK_LABEL,
  WHATS_NEW_SUBTITLE,
  WHATS_NEW_TELEMETRY_COPY,
  WHATS_NEW_TELEMETRY_ENV_VAR,
  WHATS_NEW_TELEMETRY_PRIVACY_LABEL,
  WHATS_NEW_TELEMETRY_SETTINGS_PATH,
  WHATS_NEW_TELEMETRY_TITLE,
  WHATS_NEW_TITLE,
  type WhatsNewFeatureCard,
} from './whatsNewModalModel';
import { useWhatsNewModalState } from './useWhatsNewModalState';

function WhatsNewFeatureIcon(props: { card: WhatsNewFeatureCard }) {
  switch (props.card.icon) {
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
  const recoveryEventsHref = buildRecoveryPath({ view: 'events', mode: 'remote' });

  return (
    <Dialog
      isOpen={state.isOpen()}
      onClose={state.handleClose}
      panelClass="max-w-2xl"
      ariaLabelledBy="whats-new-title"
    >
      <div class="flex max-h-[90vh] flex-col overflow-hidden">
        <div class="flex-shrink-0 flex items-start justify-between border-b border-border px-6 py-4">
          <div>
            <h2 id="whats-new-title" class="text-xl sm:text-2xl font-semibold text-base-content">
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

        <div class="flex-1 overflow-y-auto space-y-4 sm:space-y-6 px-4 sm:px-6 py-4 sm:py-5">
          <div class="grid gap-3 sm:gap-4 sm:grid-cols-2">
            <For each={WHATS_NEW_FEATURE_CARDS}>
              {(card) => (
                <div class={`rounded-md border p-3 sm:p-4 ${card.accent}`}>
                  <div class="flex items-center gap-2 text-sm font-semibold text-inherit">
                    <WhatsNewFeatureIcon card={card} />
                    {card.title}
                  </div>
                  <p class="mt-1.5 sm:mt-2 text-xs text-inherit">{card.description}</p>
                </div>
              )}
            </For>
          </div>

          <div class="rounded-md border border-sky-200 bg-sky-50 p-3 sm:p-4 dark:border-sky-800 dark:bg-sky-900/40">
            <div class="flex items-center gap-2 text-sm font-medium text-sky-900 dark:text-sky-100">
              <ChartBarIcon class="h-4 w-4 flex-shrink-0" />
              {WHATS_NEW_TELEMETRY_TITLE}
            </div>
            <p class="mt-1.5 text-xs text-sky-900 dark:text-sky-200">{WHATS_NEW_TELEMETRY_COPY[0]}</p>
            <p class="mt-1.5 text-xs text-sky-900 dark:text-sky-200">
              {WHATS_NEW_TELEMETRY_COPY[1]} You can disable it any time in{' '}
              <span class="font-medium">{WHATS_NEW_TELEMETRY_SETTINGS_PATH}</span> or by setting{' '}
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

          <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 sm:gap-4">
            <label class="flex items-center gap-2 text-sm text-muted">
              <input
                type="checkbox"
                checked={state.dontShowAgain()}
                onChange={(event) => state.setDontShowAgain(event.currentTarget.checked)}
                class="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500 focus:ring-2"
              />
              {WHATS_NEW_DO_NOT_SHOW_LABEL}
            </label>

            <a
              href={WHATS_NEW_DOCS_URL}
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1 text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
            >
              {WHATS_NEW_DOCS_LABEL}
              <ExternalLinkIcon class="h-4 w-4" />
            </a>
            <a
              href={recoveryEventsHref}
              class="inline-flex items-center gap-1 text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
            >
              {WHATS_NEW_RECOVERY_LINK_LABEL}
            </a>
          </div>
        </div>

        <div class="flex-shrink-0 flex items-center justify-end border-t border-border bg-surface-hover px-4 sm:px-6 py-3 sm:py-4">
          <button
            onClick={state.handleClose}
            class="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
            type="button"
          >
            {WHATS_NEW_PRIMARY_ACTION_LABEL}
          </button>
        </div>
      </div>
    </Dialog>
  );
}

export default WhatsNewModal;
