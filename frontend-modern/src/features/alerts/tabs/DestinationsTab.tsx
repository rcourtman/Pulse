import { createMemo, createSignal, Show } from 'solid-js';
import { hasFeature } from '@/stores/license';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { logger } from '@/utils/logger';
import type { AlertDestinationsDeliveryPausedReason } from '@/utils/alertDestinationsPresentation';
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
  const alertsActivation = useAlertsActivation();
  const [activating, setActivating] = createSignal(false);

  // Destinations configured here are inert while delivery is gated off, so the
  // pause has to be visible on this surface rather than only on the overview.
  const pausedReason = createMemo<AlertDestinationsDeliveryPausedReason | null>(() => {
    // Until the alert config resolves, activation state is null and would read
    // as paused; staying quiet avoids a false warning on every tab open.
    if (!alertsActivation.config() || alertsActivation.activationState() === null) {
      return null;
    }
    if (alertsActivation.notificationDeliveryEnabled()) {
      return null;
    }
    if (!alertsActivation.detectionEnabled()) {
      return 'detection_off';
    }
    return alertsActivation.activationState() === 'snoozed' ? 'snoozed' : 'not_activated';
  });

  const handleActivate = async () => {
    if (activating()) {
      return;
    }
    setActivating(true);
    try {
      await alertsActivation.activate();
    } catch (error) {
      logger.error('Failed to activate notification delivery from destinations tab', error);
    } finally {
      setActivating(false);
    }
  };

  return (
    <div class="flex w-full max-w-full flex-col gap-6 md:gap-8">
      <Show when={!state.isLoading()} fallback={<AlertDestinationsLoadingState />}>
        <Show when={pausedReason()}>
          {(reason) => (
            <AlertDeliveryPausedCard
              reason={reason()}
              activating={activating()}
              onActivate={() => void handleActivate()}
            />
          )}
        </Show>

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
          showUpgradePrompts={!presentationPolicyHidesUpgradePrompts()}
          upgradeDestination={getUpgradeActionDestination('relay')}
          minimumSeverity={props.pushMinimumSeverity()}
          onMinimumSeverityChange={(minimumSeverity) => {
            props.setPushMinimumSeverity(minimumSeverity);
            props.setHasUnsavedChanges(true);
          }}
        />

        <AlertDeliveryLogCard
          log={state.deliveryLog()}
          unavailable={state.deliveryLogUnavailable()}
          refreshing={state.refreshingDeliveryLog()}
          onRefresh={() => void state.loadDeliveryLog()}
          webhooks={state.webhooks()}
          heldEvents={state.heldEvents()}
        />
      </Show>
    </div>
  );
}
