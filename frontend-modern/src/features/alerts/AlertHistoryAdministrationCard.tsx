import { Show } from 'solid-js';

import {
  getAlertAdministrationClearHistoryLabel,
  getAlertAdministrationSectionDescription,
  getAlertAdministrationSectionTitle,
} from '@/utils/alertAdministrationPresentation';

import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertHistoryAdministrationCardProps {
  state: AlertHistoryState;
}

export function AlertHistoryAdministrationCard(props: AlertHistoryAdministrationCardProps) {
  return (
    <Show when={props.state.alertHistory().length > 0}>
      <div class="mt-8 border-t border-border pt-6">
        <div class="rounded-md bg-surface-alt p-4">
          <div class="flex items-start justify-between">
            <div>
              <h4 class="mb-1 text-sm font-medium text-base-content">
                {getAlertAdministrationSectionTitle()}
              </h4>
              <p class="text-xs text-muted">{getAlertAdministrationSectionDescription()}</p>
            </div>
            <button
              type="button"
              onClick={() => {
                void props.state.clearAlertHistory();
              }}
              class="flex-shrink-0 rounded-md border border-red-300 px-3 py-2 text-xs text-red-600 transition-colors hover:bg-red-50 dark:border-red-600 dark:text-red-400 dark:hover:bg-red-900"
            >
              {getAlertAdministrationClearHistoryLabel()}
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
}
