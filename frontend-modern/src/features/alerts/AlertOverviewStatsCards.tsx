import { Card } from '@/components/shared/Card';

import type { AlertOverviewState } from './useAlertOverviewState';

interface AlertOverviewStatsCardsProps {
  state: AlertOverviewState;
}

export function AlertOverviewStatsCards(props: AlertOverviewStatsCardsProps) {
  return (
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2 sm:gap-4">
      <Card padding="sm" class="sm:p-4">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-[10px] sm:text-sm text-muted uppercase tracking-wider sm:normal-case">
              Acknowledged
            </p>
            <p class="text-lg sm:text-2xl font-semibold text-yellow-600 dark:text-yellow-400">
              {props.state.alertStats().acknowledged}
            </p>
          </div>
          <div class="w-8 h-8 sm:w-10 sm:h-10 bg-yellow-100 dark:bg-yellow-900 rounded-full flex items-center justify-center">
            <svg
              width="16"
              height="16"
              class="sm:w-5 sm:h-5 text-yellow-600 dark:text-yellow-400"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M9 11L12 14L22 4"></path>
              <path d="M21 12V19C21 20.1046 20.1046 21 19 21H5C3.89543 21 3 20.1046 3 19V5C3 3.89543 3 3 5 3H16"></path>
            </svg>
          </div>
        </div>
      </Card>

      <Card padding="sm" class="sm:p-4">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-[10px] sm:text-sm text-muted uppercase tracking-wider sm:normal-case">
              Last 24 Hours
            </p>
            <p class="text-lg sm:text-2xl font-semibold text-base-content">
              {props.state.alertStats().total24h}
            </p>
          </div>
          <div class="w-8 h-8 sm:w-10 sm:h-10 bg-surface-hover rounded-full flex items-center justify-center">
            <svg
              width="16"
              height="16"
              class="sm:w-5 sm:h-5 text-muted"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <circle cx="12" cy="12" r="10"></circle>
              <polyline points="12 6 12 12 16 14"></polyline>
            </svg>
          </div>
        </div>
      </Card>

      <Card padding="sm" class="sm:p-4">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-[10px] sm:text-sm text-muted uppercase tracking-wider sm:normal-case">
              Guest Overrides
            </p>
            <p class="text-lg sm:text-2xl font-semibold text-blue-600 dark:text-blue-400">
              {props.state.alertStats().overrides}
            </p>
          </div>
          <div class="w-8 h-8 sm:w-10 sm:h-10 bg-blue-100 dark:bg-blue-900 rounded-full flex items-center justify-center">
            <svg
              width="16"
              height="16"
              class="sm:w-5 sm:h-5 text-blue-600 dark:text-blue-400"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
              <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
            </svg>
          </div>
        </div>
      </Card>
    </div>
  );
}
