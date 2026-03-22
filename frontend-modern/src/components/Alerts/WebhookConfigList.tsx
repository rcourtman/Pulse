import { For } from 'solid-js';
import type { Accessor } from 'solid-js';

import type { Webhook, WebhookTemplate } from '@/api/notifications';
import {
  ALERT_WEBHOOK_DELETE_LABEL,
  ALERT_WEBHOOK_EDIT_LABEL,
  getAlertWebhookServiceLabelFromTemplates,
  getAlertWebhookSummaryLabel,
  getAlertWebhookTestLabel,
  getAlertWebhookToggleAllLabel,
  getAlertWebhookToggleLabel,
} from '@/utils/alertWebhookPresentation';

interface WebhookConfigListProps {
  webhooks: Webhook[];
  templates: Accessor<WebhookTemplate[]>;
  testing?: string | null;
  allEnabled: Accessor<boolean>;
  someEnabled: Accessor<boolean>;
  toggleAllWebhooks: (enabled: boolean) => void;
  onToggleWebhook: (webhook: Webhook) => void;
  onTestWebhook: (webhook: Webhook) => void;
  onEditWebhook: (webhook: Webhook) => void;
  onDeleteWebhook: (webhook: Webhook) => void;
}

export function WebhookConfigList(props: WebhookConfigListProps) {
  return (
    <div class="space-y-3 w-full">
      <div class="flex flex-col gap-2 rounded border border-border px-3 py-3 text-xs sm:flex-row sm:items-center sm:justify-between">
        <div class="text-muted sm:text-sm">
          {getAlertWebhookSummaryLabel(
            props.webhooks.filter((webhook) => webhook.enabled).length,
            props.webhooks.length,
          )}
        </div>
        <div class="flex flex-wrap gap-2 sm:flex-nowrap">
          <button
            onClick={() => props.toggleAllWebhooks(false)}
            disabled={!props.someEnabled()}
            class="w-full rounded border px-3 py-1 text-xs transition-colors hover:bg-surface-hover sm:w-auto"
          >
            {getAlertWebhookToggleAllLabel(false)}
          </button>
          <button
            onClick={() => props.toggleAllWebhooks(true)}
            disabled={props.allEnabled()}
            class="w-full rounded border border-green-500 px-3 py-1 text-xs text-green-700 transition-colors hover:bg-green-50 dark:border-green-600 dark:text-green-400 dark:hover:bg-green-900 sm:w-auto"
          >
            {getAlertWebhookToggleAllLabel(true)}
          </button>
        </div>
      </div>

      <For each={props.webhooks}>
        {(webhook) => (
          <div class="w-full px-3 py-3 border border-border text-xs sm:text-sm">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span class="font-medium text-base-content">{webhook.name}</span>
              <button
                onClick={() => props.onToggleWebhook(webhook)}
                class={`rounded border px-3 py-1 text-xs font-medium transition-colors ${webhook.enabled ? 'border-green-500 text-green-700 hover:bg-green-50 dark:border-green-600 dark:text-green-400 dark:hover:bg-green-900' : 'border-border text-muted hover:bg-surface-hover'}`}
              >
                {getAlertWebhookToggleLabel(webhook.enabled)}
              </button>
            </div>
            <div class="mt-2 flex flex-wrap gap-2 text-[11px] text-muted sm:text-xs">
              <span class="rounded bg-surface-alt px-2 py-0.5 text-base-content">
                {getAlertWebhookServiceLabelFromTemplates(
                  webhook.service || 'generic',
                  props.templates(),
                )}
              </span>
              <span class="rounded bg-surface-alt px-2 py-0.5 text-base-content">
                {webhook.method}
              </span>
            </div>
            <p class="mt-2 break-all text-[11px] font-mono text-muted sm:text-xs">{webhook.url}</p>
            <div class="mt-3 flex flex-wrap gap-2 border-t border-border-subtle pt-2 sm:justify-end w-full">
              <button
                onClick={() => props.onTestWebhook(webhook)}
                disabled={props.testing === webhook.id || !webhook.enabled}
                class="rounded border px-3 py-1 text-xs text-base-content transition-colors hover:bg-surface-hover disabled:opacity-50"
              >
                {getAlertWebhookTestLabel(props.testing === webhook.id)}
              </button>
              <button
                onClick={() => props.onEditWebhook(webhook)}
                class="rounded border border-blue-300 px-3 py-1 text-xs text-blue-600 transition-colors hover:bg-blue-50 dark:border-blue-500 dark:text-blue-300 dark:hover:bg-blue-900"
              >
                {ALERT_WEBHOOK_EDIT_LABEL}
              </button>
              <button
                onClick={() => props.onDeleteWebhook(webhook)}
                class="rounded border border-red-300 px-3 py-1 text-xs text-red-600 transition-colors hover:bg-red-50 dark:border-red-500 dark:text-red-300 dark:hover:bg-red-900"
              >
                {ALERT_WEBHOOK_DELETE_LABEL}
              </button>
            </div>
          </div>
        )}
      </For>
    </div>
  );
}
