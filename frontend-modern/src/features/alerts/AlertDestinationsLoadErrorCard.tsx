import { Show } from 'solid-js';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

import { Card } from '@/components/shared/Card';
import {
  getAlertDestinationsLoadErrorBanner,
  getAlertDestinationsRetryLabel,
} from '@/utils/alertDestinationsPresentation';

interface AlertDestinationsLoadErrorCardProps {
  error: string;
  isRetrying: boolean;
  onRetry: () => void;
}

export function AlertDestinationsLoadErrorCard(props: AlertDestinationsLoadErrorCardProps) {
  return (
    <Card tone="danger" padding="sm" class="border-red-200 dark:border-red-800 sm:p-4" role="alert">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-start gap-2 text-red-800 sm:items-center dark:text-red-200">
          <AlertTriangleIcon class="mt-0.5 h-4 w-4 shrink-0 sm:mt-0" aria-hidden="true" />
          <span class="min-w-0 text-sm font-medium">
            {getAlertDestinationsLoadErrorBanner(props.error)}
          </span>
        </div>
        <button
          type="button"
          class="inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-md border border-red-300 bg-transparent px-3 py-1.5 text-sm font-medium text-red-800 transition hover:bg-red-100 focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-0 dark:border-red-700 dark:text-red-200 dark:hover:bg-red-900/30"
          disabled={props.isRetrying}
          aria-busy={props.isRetrying}
          onClick={props.onRetry}
        >
          <Show when={props.isRetrying}>
            <RefreshCwIcon class="h-4 w-4 animate-spin" aria-hidden="true" />
          </Show>
          {getAlertDestinationsRetryLabel(props.isRetrying)}
        </button>
      </div>
    </Card>
  );
}
