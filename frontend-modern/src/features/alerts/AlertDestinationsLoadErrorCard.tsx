import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';

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
    <Card tone="danger" padding="sm" class="border-red-200 dark:border-red-800 sm:p-4">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex items-center gap-2 text-red-800 dark:text-red-200">
          <AlertTriangleIcon class="h-4 w-4 flex-shrink-0" />
          <span class="text-sm font-medium">
            {getAlertDestinationsLoadErrorBanner(props.error)}
          </span>
        </div>
        <button
          class="flex-shrink-0 rounded-md border border-red-300 bg-transparent px-3 py-1.5 text-sm font-medium text-red-800 transition hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-red-700 dark:text-red-200 dark:hover:bg-red-900/30"
          disabled={props.isRetrying}
          onClick={props.onRetry}
        >
          {getAlertDestinationsRetryLabel(props.isRetrying)}
        </button>
      </div>
    </Card>
  );
}
