import { AlertQueueActionFeedback } from '../AlertQueueActionFeedback';
import { Show } from 'solid-js';
import { hasFeature } from '@/stores/license';
import { AlertAppriseDestinationsSection } from '../AlertAppriseDestinationsSection';
import { AlertDeliveryHealthCard } from '../AlertDeliveryHealthCard';
import { AlertDeliveryLogCard } from '../AlertDeliveryLogCard';
import { AlertDeliveryPausedCard } from '../AlertDeliveryPausedCard';
import { AlertDeadManDestinationSection } from '../AlertDeadManDestinationSection';
import { AlertDestinationsLoadErrorCard } from '../AlertDestinationsLoadErrorCard';
import { AlertDestinationsLoadingState } from '../AlertDestinationsLoadingState';
import { AlertEmailDestinationsSection } from '../AlertEmailDestinationsSection';
import { AlertPushDestinationsSection } from '../AlertPushDestinationsSection';
import { AlertWebhookDestinationsSection } from '../AlertWebhookDestinationsSection';
import { useAlertDeliveryPausedReason } from '../useAlertDeliveryPausedReason';

import {
  useAlertDestinationsTabState,
  type AlertDestinationsTabStateProps,
} from '../useAlertDestinationsTabState';

export interface DestinationsTabProps extends AlertDestinationsTabStateProps {
  setHasUnsavedChanges: (value: boolean) => void;
  setEmailConfig: (config: ReturnType<AlertDestinationsTabStateProps['emailConfig']>) => void;
  deadManPingUrl: () => string;
  setDeadManPingUrl: (value: string) => void;
  pushMinimumSeverity: () => 'all' | 'critical';
  setPushMinimumSeverity: (value: 'all' | 'critical') => void;
}

export function DestinationsTab(props: DestinationsTabProps) {
  const state = useAlertDestinationsTabState(props);
  // Destinations configured here are inert while delivery is gated off, so the
  // pause has to be visible on this surface rather than only on the overview.
  const deliveryPaused = useAlertDeliveryPausedReason();

  return (
    <div class="flex w-full max-w-full flex-col gap-6 md:gap-8">
      <Show when={!state.isLoading()} fallback={<AlertDestinationsLoadingState />}>
        <Show when={deliveryPaused.pausedReason()}>
          {(reason) => (
            <AlertDeliveryPausedCard
              reason={reason()}
              activating={deliveryPaused.activating()}
              onActivate={() => void deliveryPaused.activate()}
            />
          )}
        </Show>

        <AlertQueueActionFeedback
          message={state.queueActionFeedback()}
          onClear={state.clearQueueActionFeedback}
        />
        <Show when={state.deliveryNeedsAttention()}>
          <AlertDeliveryHealthCard
            health={state.deliveryHealth()?.queue ?? null}
            unavailable={state.deliveryHealthUnavailable()}
            refreshing={state.refreshingDeliveryHealth()}
            onRefresh={() => void state.loadDeliveryHealth()}
            retryingFailures={state.retryingTerminalFailures()}
            dismissingFailures={state.dismissingTerminalFailures()}
            onRetryFailures={() => void state.retryTerminalFailures()}
            onDismissFailures={() => void state.dismissTerminalFailures()}
          />
        </Show>

        <AlertDeliveryLogCard
          log={state.deliveryLog()}
          unavailable={state.deliveryLogUnavailable()}
          refreshing={state.refreshingDeliveryLog()}
          onRefresh={() => void state.loadDeliveryLog()}
          webhooks={state.webhooks()}
          heldEvents={state.heldEvents()}
          heldEventsUnavailable={state.heldEventsUnavailable()}
          refreshingHeldEvents={state.refreshingHeldEvents()}
        />

        <Show when={state.hasLoadError()}>
          <AlertDestinationsLoadErrorCard
            error={props.configLoadError() || state.webhookLoadError() || ''}
            isRetrying={props.isRetrying()}
            onRetry={state.handleRetry}
          />
        </Show>

        <AlertEmailDestinationsSection
          config={props.emailConfig()}
          setConfig={props.setEmailConfig}
          setHasUnsavedChanges={props.setHasUnsavedChanges}
          onTest={state.testEmailConfig}
          testing={state.testingEmail()}
        />

        <AlertAppriseDestinationsSection
          config={state.appriseState()}
          updateApprise={state.updateApprise}
          setHasUnsavedChanges={props.setHasUnsavedChanges}
          onTest={state.testApprise}
          testing={state.testingApprise()}
        />

        <AlertWebhookDestinationsSection
          webhooks={state.webhooks()}
          addWebhook={state.addWebhook}
          updateWebhook={state.updateWebhook}
          deleteWebhook={state.deleteWebhook}
          testWebhook={state.testWebhook}
          testingWebhook={state.testingWebhook()}
        />

        <AlertDeadManDestinationSection
          pingUrl={props.deadManPingUrl}
          setPingUrl={props.setDeadManPingUrl}
          setHasUnsavedChanges={props.setHasUnsavedChanges}
        />

        <AlertPushDestinationsSection
          relayLicensed={hasFeature('relay')}
          minimumSeverity={props.pushMinimumSeverity()}
          onMinimumSeverityChange={(minimumSeverity) => {
            props.setPushMinimumSeverity(minimumSeverity);
            props.setHasUnsavedChanges(true);
          }}
        />
      </Show>
    </div>
  );
}
