import { Show } from 'solid-js';
import { A } from '@solidjs/router';
import BellOffIcon from 'lucide-solid/icons/bell-off';

import { Card } from '@/components/shared/Card';
import {
  type AlertDestinationsDeliveryPausedReason,
  getAlertDestinationsDeliveryPausedActionLabel,
  getAlertDestinationsDeliveryPausedDescription,
  getAlertDestinationsDeliveryPausedTitle,
  getAlertDestinationsDeliverySetupLinkLabel,
} from '@/utils/alertDestinationsPresentation';

interface AlertDeliveryPausedCardProps {
  reason: AlertDestinationsDeliveryPausedReason;
  activating: boolean;
  onActivate: () => void;
  // The overview states the gate once for every listed alert and links to
  // where destinations are set up; the destinations tab is that place.
  surface?: 'destinations' | 'overview';
  setupHref?: string;
}

// Shown wherever notification delivery being gated off changes what the user
// should believe. Without it a user configures a destination, sends a passing
// test, and never learns that live alerts are being dropped before they reach
// the queue; on the overview it replaces a warning repeated on every alert.
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
          <BellOffIcon class="mt-0.5 h-4 w-4 shrink-0 text-amber-700 dark:text-amber-300" />
          <div class="min-w-0">
            <h3 class="text-sm font-semibold text-amber-900 dark:text-amber-100">
              {getAlertDestinationsDeliveryPausedTitle()}
            </h3>
            <p class="mt-1 text-sm leading-6 text-amber-800 dark:text-amber-200">
              {getAlertDestinationsDeliveryPausedDescription(props.reason, props.surface)}
            </p>
          </div>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-3">
          <Show when={props.setupHref}>
            {(href) => (
              <A
                href={href()}
                class="text-sm font-medium text-amber-800 underline underline-offset-2 hover:text-amber-900 dark:text-amber-200 dark:hover:text-amber-100"
              >
                {getAlertDestinationsDeliverySetupLinkLabel()}
              </A>
            )}
          </Show>
          <button
            type="button"
            class="inline-flex shrink-0 items-center justify-center gap-2 rounded-md border border-amber-300 bg-transparent px-3 py-1.5 text-sm font-medium text-amber-800 transition hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-amber-700 dark:text-amber-200 dark:hover:bg-amber-900/30"
            disabled={props.activating}
            onClick={props.onActivate}
          >
            {getAlertDestinationsDeliveryPausedActionLabel()}
          </button>
        </div>
      </div>
    </Card>
  );
}
