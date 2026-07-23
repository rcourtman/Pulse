import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

import type { NotificationQueueHealth } from '@/api/notifications';
import { Card } from '@/components/shared/Card';
import {
  getAlertDestinationsDeliveryHealthDescription,
  getAlertDestinationsDeliveryHealthTitle,
  getAlertDestinationsDeliveryRefreshLabel,
} from '@/utils/alertDestinationsPresentation';

interface AlertDeliveryHealthCardProps {
  health: NotificationQueueHealth | null;
  unavailable: boolean;
  refreshing: boolean;
  onRefresh: () => void;
}

export function AlertDeliveryHealthCard(props: AlertDeliveryHealthCardProps) {
  const status = (): 'degraded' | 'unavailable' =>
    props.unavailable || props.health?.status === 'unavailable' ? 'unavailable' : 'degraded';
  const failed = () => props.health?.failed ?? 0;
  const deadLetter = () => props.health?.deadLetter ?? 0;
  const completedRetentionDays = () => props.health?.completedRetentionDays ?? 7;
  const deadLetterRetentionDays = () => props.health?.deadLetterRetentionDays ?? 30;

  return (
    <Card tone="danger" padding="sm" class="border-red-200 dark:border-red-800 sm:p-4" role="alert">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div class="flex min-w-0 items-start gap-3">
          <AlertTriangleIcon class="mt-0.5 h-4 w-4 flex-shrink-0 text-red-700 dark:text-red-300" />
          <div class="min-w-0">
            <h3 class="text-sm font-semibold text-red-900 dark:text-red-100">
              {getAlertDestinationsDeliveryHealthTitle(status())}
            </h3>
            <p class="mt-1 text-sm leading-6 text-red-800 dark:text-red-200">
              {getAlertDestinationsDeliveryHealthDescription({
                status: status(),
                failed: failed(),
                deadLetter: deadLetter(),
                completedRetentionDays: completedRetentionDays(),
                deadLetterRetentionDays: deadLetterRetentionDays(),
              })}
            </p>
          </div>
        </div>
        <button
          type="button"
          class="inline-flex flex-shrink-0 items-center justify-center gap-2 rounded-md border border-red-300 bg-transparent px-3 py-1.5 text-sm font-medium text-red-800 transition hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-red-700 dark:text-red-200 dark:hover:bg-red-900/30"
          disabled={props.refreshing}
          onClick={props.onRefresh}
        >
          <RefreshCwIcon class={`h-4 w-4 ${props.refreshing ? 'animate-spin' : ''}`} />
          {getAlertDestinationsDeliveryRefreshLabel()}
        </button>
      </div>
    </Card>
  );
}
