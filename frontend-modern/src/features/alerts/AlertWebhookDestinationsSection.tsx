import { WebhookConfig, type WebhookConfigProps } from '@/components/Alerts/WebhookConfig';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import type { Webhook } from '@/api/notifications';
import {
  getAlertWebhooksSectionDescription,
  getAlertWebhooksSectionTitle,
} from '@/utils/alertWebhookPresentation';

interface AlertWebhookDestinationsSectionProps {
  webhooks: Webhook[];
  addWebhook: WebhookConfigProps['onAdd'];
  updateWebhook: WebhookConfigProps['onUpdate'];
  deleteWebhook: (id: string) => void;
  testWebhook: (webhookId: string, webhookData?: Omit<Webhook, 'id'>) => void;
  testingWebhook: string | null;
}

export function AlertWebhookDestinationsSection(props: AlertWebhookDestinationsSectionProps) {
  return (
    <SettingsPanel
      title={getAlertWebhooksSectionTitle()}
      description={getAlertWebhooksSectionDescription()}
      action={
        <span class="whitespace-nowrap text-xs text-muted">{props.webhooks.length} configured</span>
      }
      class="min-w-0"
      bodyClass="space-y-4"
    >
      <WebhookConfig
        webhooks={props.webhooks}
        onAdd={props.addWebhook}
        onUpdate={props.updateWebhook}
        onDelete={props.deleteWebhook}
        onTest={props.testWebhook}
        testing={props.testingWebhook}
      />
    </SettingsPanel>
  );
}
