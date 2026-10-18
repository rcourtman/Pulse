import { Show } from 'solid-js';

import type { Webhook } from '@/api/notifications';
import { ALERT_WEBHOOK_ADD_LABEL } from '@/utils/alertWebhookPresentation';

import { WebhookConfigForm } from './WebhookConfigForm';
import { WebhookConfigList } from './WebhookConfigList';
import { useWebhookConfigState } from './useWebhookConfigState';

export interface WebhookConfigProps {
  webhooks: Webhook[];
  onAdd: (webhook: Omit<Webhook, 'id'>) => boolean | Promise<boolean>;
  onUpdate: (webhook: Webhook) => boolean | Promise<boolean>;
  onDelete: (id: string) => boolean | Promise<boolean>;
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
          saving={state.saving() || state.adding()}
          allEnabled={state.allEnabled}
          someEnabled={state.someEnabled}
          toggleAllWebhooks={state.toggleAllWebhooks}
          onToggleWebhook={state.toggleWebhook}
          onTestWebhook={(webhook) => webhook.id && props.onTest(webhook.id)}
          onEditWebhook={state.editWebhook}
          onDeleteWebhook={state.deleteWebhook}
        />
      </Show>

      <Show when={state.saving() && !state.adding()}>
        <p role="status" class="text-xs text-muted">
          Saving webhook changes…
        </p>
      </Show>
      <Show when={state.mutationError()}>
        <p role="alert" class="text-xs text-red-600 dark:text-red-400">
          {state.mutationError()}
        </p>
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
          saving={state.saving}
          saveError={state.saveError}
        />
      </Show>

      <Show when={!state.adding()}>
        <button
          onClick={state.openAddForm}
          disabled={state.saving()}
          class="min-h-11 w-full border border-dashed border-border px-2 py-1 text-xs text-muted hover:bg-surface-hover sm:min-h-0"
        >
          + {ALERT_WEBHOOK_ADD_LABEL}
        </button>
      </Show>
    </div>
  );
}
