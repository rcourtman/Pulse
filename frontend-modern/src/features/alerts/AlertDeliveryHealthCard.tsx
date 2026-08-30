import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

import type { NotificationQueueHealth } from '@/api/notifications';
import { Button, ButtonLink } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import {
  getAlertDestinationsDeliveryDismissLabel,
  getAlertDestinationsDeliveryHealthDescription,
  getAlertDestinationsDeliveryHealthTitle,
  getAlertDestinationsDeliveryRefreshLabel,
  getAlertDestinationsDeliveryReviewLabel,
  getAlertDestinationsDeliveryRetryLabel,
} from '@/utils/alertDestinationsPresentation';

interface AlertDeliveryHealthCardProps {
  health: NotificationQueueHealth | null;
  unavailable: boolean;
  refreshing: boolean;
  onRefresh: () => void;
  retryingFailures?: boolean;
  dismissingFailures?: boolean;
  onRetryFailures?: () => void;
  onDismissFailures?: () => void;
  detailsHref?: string;
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
                failureClasses7d: props.health?.failureClasses7d,
                failureClassesAvailable: props.health?.failureClassesAvailable ?? false,
              })}
            </p>
          </div>
        </div>
        <div class="flex flex-shrink-0 flex-wrap items-center gap-2">
          {props.detailsHref ? (
            <ButtonLink variant="secondary" size="sm" href={props.detailsHref}>
              {getAlertDestinationsDeliveryReviewLabel()}
            </ButtonLink>
          ) : null}
          {props.onRetryFailures ? (
            <Button
              variant="secondary"
              size="sm"
              isLoading={props.retryingFailures}
              disabled={props.dismissingFailures}
              onClick={props.onRetryFailures}
            >
              {getAlertDestinationsDeliveryRetryLabel()}
            </Button>
          ) : null}
          {props.onDismissFailures ? (
            <Button
              variant="danger"
              size="sm"
              isLoading={props.dismissingFailures}
              disabled={props.retryingFailures}
              onClick={props.onDismissFailures}
            >
              {getAlertDestinationsDeliveryDismissLabel()}
            </Button>
          ) : null}
          <Button
            variant="ghost"
            size="sm"
            disabled={props.refreshing || props.retryingFailures || props.dismissingFailures}
            onClick={props.onRefresh}
          >
            <RefreshCwIcon class={`mr-2 h-4 w-4 ${props.refreshing ? 'animate-spin' : ''}`} />
            {getAlertDestinationsDeliveryRefreshLabel()}
          </Button>
        </div>
      </div>
    </Card>
  );
}
