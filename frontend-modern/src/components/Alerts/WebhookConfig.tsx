import { Show } from 'solid-js';

import type { Webhook } from '@/api/notifications';
import { ALERT_WEBHOOK_ADD_LABEL } from '@/utils/alertWebhookPresentation';

import { WebhookConfigForm } from './WebhookConfigForm';
import { WebhookConfigList } from './WebhookConfigList';
import { useWebhookConfigState } from './useWebhookConfigState';

export interface WebhookConfigProps {
  webhooks: Webhook[];
  onAdd: (webhook: Omit<Webhook, 'id'>) => void;
  onUpdate: (webhook: Webhook) => void;
  onDelete: (id: string) => void;
  onTest: (id: string, webhookData?: Omit<Webhook, 'id'>) => void;
  testing?: string | null;
}

export function WebhookConfig(props: WebhookConfigProps) {
  const state = useWebhookConfigState(props);

  return (
    <div class="space-y-6 min-w-0 w-full">
      <Show when={props.webhooks.length > 0}>
        <WebhookConfigList
          webhooks={props.webhooks}
          templates={state.templates}
          testing={props.testing}
          allEnabled={state.allEnabled}
          someEnabled={state.someEnabled}
          toggleAllWebhooks={state.toggleAllWebhooks}
          onToggleWebhook={(webhook) => props.onUpdate({ ...webhook, enabled: !webhook.enabled })}
          onTestWebhook={(webhook) => webhook.id && props.onTest(webhook.id)}
          onEditWebhook={state.editWebhook}
          onDeleteWebhook={(webhook) => webhook.id && props.onDelete(webhook.id)}
        />
      </Show>

      <Show when={state.adding()}>
        <WebhookConfigForm
          editingId={state.editingId}
          formData={state.formData}
          setFormData={state.setFormData}
          templates={state.templates}
          currentTemplate={state.currentTemplate}
          showServiceDropdown={state.showServiceDropdown}
          setShowServiceDropdown={state.setShowServiceDropdown}
          headerInputs={state.headerInputs}
          customFieldInputs={state.customFieldInputs}
          selectService={state.selectService}
          updateHeaderInput={state.updateHeaderInput}
          removeHeaderInput={state.removeHeaderInput}
          addHeaderInput={state.addHeaderInput}
          updateCustomFieldInput={state.updateCustomFieldInput}
          removeCustomFieldInput={state.removeCustomFieldInput}
          addCustomFieldInput={state.addCustomFieldInput}
          cancelForm={state.cancelForm}
          testWebhookForm={state.testWebhookForm}
          saveWebhook={state.saveWebhook}
          testing={props.testing}
        />
      </Show>

      <Show when={!state.adding()}>
        <button
          onClick={state.openAddForm}
          class="w-full border border-dashed border-border px-2 py-1 text-xs text-muted hover:bg-surface-hover"
        >
          + {ALERT_WEBHOOK_ADD_LABEL}
        </button>
      </Show>
    </div>
  );
}
