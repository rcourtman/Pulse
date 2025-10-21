import { Show, createEffect, createMemo, createSignal } from 'solid-js';
import type { JSX } from 'solid-js';
import type { Alert } from '@/types/api';
import type { ActivationState, AlertConfig } from '@/types/alerts';
import { ActivationModal } from './ActivationModal';

interface ActivationBannerProps {
  activationState: () => ActivationState | null;
  activeAlerts: () => Alert[] | undefined;
  config: () => AlertConfig | null;
  isPastObservationWindow: () => boolean;
  isLoading: () => boolean;
  refreshActiveAlerts: () => Promise<void>;
  activate: () => Promise<boolean>;
}

export function ActivationBanner(props: ActivationBannerProps): JSX.Element {
  const [isModalOpen, setIsModalOpen] = createSignal(false);

  const shouldShow = createMemo(() => {
    const state = props.activationState();
    return state === 'pending_review' || state === 'snoozed';
  });

  createEffect(() => {
    // Close the modal automatically if activation becomes active while it is open
    if (!shouldShow() && isModalOpen()) {
      setIsModalOpen(false);
    }
  });

  const violationCount = createMemo(() => props.activeAlerts()?.length ?? 0);

  const observationSummary = createMemo(() => {
    const count = violationCount();
    if (count <= 0) {
      return 'All systems healthy — no alerts triggered.';
    }
    const label = count === 1 ? 'issue' : 'issues';
    return `${count} ${label} detected.`;
  });

  const handleReview = async () => {
    await props.refreshActiveAlerts();
    setIsModalOpen(true);
  };

  const handleActivated = async () => {
    await props.refreshActiveAlerts();
  };

  return (
    <>
      <Show when={shouldShow()}>
        <div class="bg-blue-50 dark:bg-blue-900/30 border-b border-blue-200 dark:border-blue-800 text-blue-900 dark:text-blue-100 relative animate-slideDown">
          <div class="px-4 py-2">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div class="flex items-start gap-3">
                <svg
                  class="w-5 h-5 flex-shrink-0 text-blue-600 dark:text-blue-300 mt-0.5"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    d="M12 22c1.1 0 2-.9 2-2h-4c0 1.1.9 2 2 2z"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  />
                  <path
                    d="M18 16v-5a6 6 0 1 0-12 0v5l-2 2h16l-2-2z"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  />
                </svg>
                <div class="space-y-1">
                  <p class="text-sm font-medium">
                    Monitoring is active. Review your settings to enable notifications.
                  </p>
                  <p class="text-xs text-blue-700 dark:text-blue-200">{observationSummary()}</p>
                  <Show when={props.isPastObservationWindow()}>
                    <p class="text-xs font-semibold text-blue-800 dark:text-blue-100">
                      24-hour setup period ending soon — activate to start receiving notifications.
                    </p>
                  </Show>
                </div>
              </div>

              <div class="flex items-center gap-2">
                <button
                  type="button"
                  class="inline-flex items-center justify-center px-3 py-1.5 text-sm font-medium rounded-md bg-blue-600 hover:bg-blue-700 text-white transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
                  onClick={handleReview}
                  disabled={props.isLoading()}
                >
                  {props.isLoading() ? 'Loading…' : 'Review & Activate'}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Show>

      <ActivationModal
        isOpen={isModalOpen()}
        onClose={() => setIsModalOpen(false)}
        onActivated={handleActivated}
        config={props.config}
        activeAlerts={props.activeAlerts}
        isLoading={props.isLoading}
        activate={props.activate}
        refreshActiveAlerts={props.refreshActiveAlerts}
      />
    </>
  );
}
