import { Show } from 'solid-js';
import { AlertAppriseDestinationsSection } from '../AlertAppriseDestinationsSection';
import { AlertDestinationsLoadErrorCard } from '../AlertDestinationsLoadErrorCard';
import { AlertDestinationsLoadingState } from '../AlertDestinationsLoadingState';
import { AlertEmailDestinationsSection } from '../AlertEmailDestinationsSection';
import { AlertWebhookDestinationsSection } from '../AlertWebhookDestinationsSection';

import { useAlertDestinationsTabState, type AlertDestinationsTabStateProps } from '../useAlertDestinationsTabState';

export interface DestinationsTabProps extends AlertDestinationsTabStateProps {
  setHasUnsavedChanges: (value: boolean) => void;
  setEmailConfig: (config: ReturnType<AlertDestinationsTabStateProps['emailConfig']>) => void;
}

export function DestinationsTab(props: DestinationsTabProps) {
  const state = useAlertDestinationsTabState(props);

  return (
    <div class="flex w-full max-w-full flex-col gap-6 md:gap-8">
      <Show
        when={!state.isLoading()}
        fallback={<AlertDestinationsLoadingState />}
      >
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
      </Show>
    </div>
  );
}
