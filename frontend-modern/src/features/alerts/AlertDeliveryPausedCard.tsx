import BellOffIcon from 'lucide-solid/icons/bell-off';

import { Card } from '@/components/shared/Card';
import {
  type AlertDestinationsDeliveryPausedReason,
  getAlertDestinationsDeliveryPausedActionLabel,
  getAlertDestinationsDeliveryPausedDescription,
  getAlertDestinationsDeliveryPausedTitle,
} from '@/utils/alertDestinationsPresentation';

interface AlertDeliveryPausedCardProps {
  reason: AlertDestinationsDeliveryPausedReason;
  activating: boolean;
  onActivate: () => void;
}

// Shown on the destinations surface whenever notification delivery is gated
// off. Without it a user configures a destination, sends a passing test, and
// never learns that live alerts are being dropped before they reach the queue.
export function AlertDeliveryPausedCard(props: AlertDeliveryPausedCardProps) {
  return (
    <Card
      tone="warning"
      padding="sm"
      class="border-amber-200 dark:border-amber-800 sm:p-4"
      role="alert"
    >
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div class="flex min-w-0 items-start gap-3">
          <BellOffIcon class="mt-0.5 h-4 w-4 flex-shrink-0 text-amber-700 dark:text-amber-300" />
          <div class="min-w-0">
            <h3 class="text-sm font-semibold text-amber-900 dark:text-amber-100">
              {getAlertDestinationsDeliveryPausedTitle()}
            </h3>
            <p class="mt-1 text-sm leading-6 text-amber-800 dark:text-amber-200">
              {getAlertDestinationsDeliveryPausedDescription(props.reason)}
            </p>
          </div>
        </div>
        <button
          type="button"
          class="inline-flex flex-shrink-0 items-center justify-center gap-2 rounded-md border border-amber-300 bg-transparent px-3 py-1.5 text-sm font-medium text-amber-800 transition hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-amber-700 dark:text-amber-200 dark:hover:bg-amber-900/30"
          disabled={props.activating}
          onClick={props.onActivate}
        >
          {getAlertDestinationsDeliveryPausedActionLabel()}
        </button>
      </div>
    </Card>
  );
}
