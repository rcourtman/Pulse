import { A } from '@solidjs/router';
import BellIcon from 'lucide-solid/icons/bell';

import { Card } from '@/components/shared/Card';

interface AlertNoDestinationsCardProps {
  setupHref: string;
  onDismiss: () => void;
}

// Shown on the Alerts Overview when alert delivery is active but zero
// notification channels are enabled, so operators know nobody is being
// notified outside the dashboard yet and have a direct setup link.
export function AlertNoDestinationsCard(props: AlertNoDestinationsCardProps) {
  return (
    <Card
      tone="info"
      padding="sm"
      class="border-blue-200 dark:border-blue-800 sm:p-4"
      role="status"
      data-testid="alerts-no-destinations-card"
    >
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div class="flex min-w-0 items-start gap-3">
          <BellIcon class="mt-0.5 h-4 w-4 shrink-0 text-blue-700 dark:text-blue-300" />
          <div class="min-w-0">
            <h3 class="text-sm font-semibold text-blue-900 dark:text-blue-100">
              No notification destinations configured
            </h3>
            <p class="mt-1 text-sm leading-6 text-blue-800 dark:text-blue-200">
              Pulse is monitoring alerts here, but nobody is notified outside the dashboard yet. Add
              Email, Discord, Telegram, Slack, ntfy, or Apprise to receive alerts.
            </p>
          </div>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-3">
          <A
            href={props.setupHref}
            class="inline-flex shrink-0 items-center justify-center rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white transition hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:bg-blue-500 dark:hover:bg-blue-600 dark:focus-visible:ring-offset-surface"
          >
            Set up notifications
          </A>
          <button
            type="button"
            class="inline-flex shrink-0 items-center justify-center rounded-md border border-blue-300 bg-transparent px-3 py-1.5 text-sm font-medium text-blue-800 transition hover:bg-blue-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:border-blue-700 dark:text-blue-200 dark:hover:bg-blue-900/30 dark:focus-visible:ring-offset-surface"
            onClick={props.onDismiss}
          >
            Dismiss
          </button>
        </div>
      </div>
    </Card>
  );
}
