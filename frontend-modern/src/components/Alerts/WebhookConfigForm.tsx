import { Index, Show, For } from 'solid-js';
import type { Accessor } from 'solid-js';

import type { WebhookTemplate } from '@/api/notifications';
import {
  ALERT_WEBHOOK_CANCEL_LABEL,
  ALERT_WEBHOOK_CUSTOM_FIELDS_HELP,
  ALERT_WEBHOOK_CUSTOM_FIELDS_PUSHHOVER_HELP,
  ALERT_WEBHOOK_CUSTOM_FIELDS_REFERENCE,
  ALERT_WEBHOOK_CUSTOM_FIELD_ADD_LABEL,
  ALERT_WEBHOOK_CUSTOM_FIELD_KEY_PLACEHOLDER,
  ALERT_WEBHOOK_CUSTOM_FIELD_REMOVE_LABEL,
  ALERT_WEBHOOK_CUSTOM_FIELD_VALUE_PLACEHOLDER,
  ALERT_WEBHOOK_ENABLE_LABEL,
  ALERT_WEBHOOK_HEADER_ADD_LABEL,
  ALERT_WEBHOOK_HEADER_HELP,
  ALERT_WEBHOOK_HEADER_KEY_PLACEHOLDER,
  ALERT_WEBHOOK_HEADER_REMOVE_LABEL,
  ALERT_WEBHOOK_HEADER_VALUE_PLACEHOLDER,
  ALERT_WEBHOOK_HEADERS_HELP_LABEL,
  ALERT_WEBHOOK_MENTION_HELP_LABEL,
  ALERT_WEBHOOK_PAYLOAD_HELP_LABEL,
  ALERT_WEBHOOK_PAYLOAD_TEMPLATE_PLACEHOLDER,
  ALERT_WEBHOOK_PAYLOAD_VARIABLES,
  ALERT_WEBHOOK_URL_HELP_PATH,
  ALERT_WEBHOOK_URL_HELP_QUERY,
  ALERT_WEBHOOK_URL_HELP_TEMPLATE_VARIABLE,
  getAlertWebhookMentionHelpFromTemplates,
  getAlertWebhookMentionPlaceholderFromTemplates,
  getAlertWebhookNamePlaceholder,
  getAlertWebhookServiceLabelFromTemplates,
  getAlertWebhookServices,
  getAlertWebhookSetupInstructionsTitle,
  getAlertWebhookSubmitLabel,
  getAlertWebhookTestLabel,
  getAlertWebhookUrlPlaceholder,
  hasAlertWebhookMentionSupportFromTemplates,
} from '@/utils/alertWebhookPresentation';
import {
  formCheckbox,
  formField,
  formHelpText,
  controlClass,
  labelClass,
} from '@/components/shared/Form';

import type {
  CustomFieldInput,
  HeaderInput,
  WebhookConfigFormData,
} from './useWebhookConfigState';

type SetterLike<T> = (value: T | ((prev: T) => T)) => T;

interface WebhookConfigFormProps {
  editingId: Accessor<string | null>;
  formData: Accessor<WebhookConfigFormData>;
  setFormData: SetterLike<WebhookConfigFormData>;
  templates: Accessor<WebhookTemplate[]>;
  currentTemplate: Accessor<WebhookTemplate | undefined>;
  showServiceDropdown: Accessor<boolean>;
  setShowServiceDropdown: SetterLike<boolean>;
  headerInputs: Accessor<HeaderInput[]>;
  customFieldInputs: Accessor<CustomFieldInput[]>;
  selectService: (service: string) => void;
  updateHeaderInput: (index: number, patch: Partial<HeaderInput>) => void;
  removeHeaderInput: (index: number) => void;
  addHeaderInput: () => void;
  updateCustomFieldInput: (index: number, patch: Partial<CustomFieldInput>) => void;
  removeCustomFieldInput: (index: number) => void;
  addCustomFieldInput: () => void;
  cancelForm: () => void;
  testWebhookForm: () => void;
  saveWebhook: () => void;
  testing?: string | null;
}

export function WebhookConfigForm(props: WebhookConfigFormProps) {
  return (
    <div class="space-y-4 text-sm">
      <div>
        <div class="flex items-center justify-between mb-4">
          <label class="text-sm font-medium text-base-content">Service Type</label>
          <button
            type="button"
            onClick={() => props.setShowServiceDropdown((open) => !open)}
            class="text-sm text-blue-600 hover:text-blue-700 dark:text-blue-400"
          >
            {getAlertWebhookServiceLabelFromTemplates(
              props.formData().service,
              props.templates(),
            )}{' '}
            →
          </button>
        </div>

        <Show when={props.showServiceDropdown()}>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-2 border border-border px-3 py-2 mb-3 text-xs">
            <For each={getAlertWebhookServices(props.templates())}>
              {(service) => (
                <button
                  type="button"
                  onClick={() => props.selectService(service.id)}
                  class={`px-2 py-1.5 text-left border transition-colors text-xs ${
                    props.formData().service === service.id
                      ? 'border-blue-500 bg-blue-50 dark:bg-blue-900'
                      : 'border-border hover:bg-surface-hover'
                  }`}
                >
                  <div class="font-medium text-xs text-base-content">{service.label}</div>
                  <div class="text-[11px] text-muted mt-1">{service.description}</div>
                </button>
              )}
            </For>
          </div>
        </Show>

        <Show when={props.currentTemplate()?.instructions}>
          <div class="mb-3 border-l-2 border-blue-300 pl-3 text-xs leading-relaxed text-blue-800 dark:border-blue-700 dark:text-blue-200">
            <h4 class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-2">
              {getAlertWebhookSetupInstructionsTitle()}
            </h4>
            {props.currentTemplate()!.instructions}
          </div>
        </Show>
      </div>

      <div class="grid w-full grid-cols-1 gap-3 md:grid-cols-2">
        <div class={formField}>
          <label class={labelClass()}>Name</label>
          <input
            type="text"
            value={props.formData().name}
            onInput={(e) =>
              props.setFormData((prev) => ({ ...prev, name: e.currentTarget.value }))
            }
            placeholder={getAlertWebhookNamePlaceholder(props.currentTemplate()?.name)}
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>HTTP method</label>
          <select
            value={props.formData().method}
            onChange={(e) =>
              props.setFormData((prev) => ({ ...prev, method: e.currentTarget.value }))
            }
            class={controlClass('px-2 py-1.5 pr-8 appearance-none')}
          >
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="PATCH">PATCH</option>
          </select>
        </div>
      </div>

      <div class={formField}>
        <label class={labelClass()}>Webhook URL</label>
        <input
          type="url"
          value={props.formData().url}
          onInput={(e) =>
            props.setFormData((prev) => ({ ...prev, url: e.currentTarget.value }))
          }
          placeholder={getAlertWebhookUrlPlaceholder(props.currentTemplate()?.urlPattern)}
          class={controlClass('px-2 py-1.5 font-mono')}
        />
        <p class={formHelpText + ' mt-1'}>
          Supports template variables like{' '}
          <code class="font-mono text-[11px] text-muted">
            {ALERT_WEBHOOK_URL_HELP_TEMPLATE_VARIABLE}
          </code>
          . Use{' '}
          <code class="font-mono text-[11px] text-muted">{ALERT_WEBHOOK_URL_HELP_PATH}</code> or{' '}
          <code class="font-mono text-[11px] text-muted">{ALERT_WEBHOOK_URL_HELP_QUERY}</code> to
          keep dynamic values URL-safe.
        </p>
      </div>

      <Show when={hasAlertWebhookMentionSupportFromTemplates(props.formData().service, props.templates())}>
        <div class={formField}>
          <label class={labelClass('flex items-center gap-2')}>
            Mention
            <span class="text-xs text-muted">{ALERT_WEBHOOK_MENTION_HELP_LABEL}</span>
          </label>
          <input
            type="text"
            value={props.formData().mention || ''}
            onInput={(e) =>
              props.setFormData((prev) => ({ ...prev, mention: e.currentTarget.value }))
            }
            placeholder={getAlertWebhookMentionPlaceholderFromTemplates(
              props.formData().service,
              props.templates(),
            )}
            class={controlClass('px-2 py-1.5')}
          />
          <Show
            when={getAlertWebhookMentionHelpFromTemplates(
              props.formData().service,
              props.templates(),
            )}
          >
            <p class={formHelpText + ' mt-1'}>
              {getAlertWebhookMentionHelpFromTemplates(
                props.formData().service,
                props.templates(),
              )}
            </p>
          </Show>
        </div>
      </Show>

      <Show when={props.formData().service === 'generic'}>
        <div class={formField}>
          <label class={labelClass('flex items-center gap-2')}>
            Custom payload template (JSON)
            <span class="text-xs text-muted">{ALERT_WEBHOOK_PAYLOAD_HELP_LABEL}</span>
          </label>
          <textarea
            value={props.formData().payloadTemplate || ''}
            onInput={(e) =>
              props.setFormData((prev) => ({
                ...prev,
                payloadTemplate: e.currentTarget.value,
              }))
            }
            placeholder={ALERT_WEBHOOK_PAYLOAD_TEMPLATE_PLACEHOLDER}
            rows={8}
            class={controlClass('px-2 py-1.5 text-xs font-mono min-h-[160px]')}
          />
          <p class={formHelpText + ' mt-1'}>
            Available variables: {ALERT_WEBHOOK_PAYLOAD_VARIABLES}
          </p>
        </div>
      </Show>

      <Show when={props.customFieldInputs().length > 0 || props.formData().service === 'pushover'}>
        <div class={formField}>
          <label class={labelClass('flex items-center gap-2')}>
            Custom fields
            <span class="text-xs text-muted">
              {ALERT_WEBHOOK_CUSTOM_FIELDS_HELP}{' '}
              <code class="font-mono text-[11px] text-muted">
                {ALERT_WEBHOOK_CUSTOM_FIELDS_REFERENCE}
              </code>{' '}
              in templates
            </span>
          </label>
          <div class="space-y-2 text-xs">
            <Index each={props.customFieldInputs()}>
              {(field, index) => (
                <div class="flex gap-2 text-xs">
                  <div class="flex flex-1 flex-col gap-1">
                    <Show when={field().label}>
                      <span class="text-[11px] text-muted">{field().label}</span>
                    </Show>
                    <input
                      type="text"
                      value={field().key}
                      disabled={field().required}
                      onInput={(e) =>
                        props.updateCustomFieldInput(index, { key: e.currentTarget.value })
                      }
                      placeholder={ALERT_WEBHOOK_CUSTOM_FIELD_KEY_PLACEHOLDER}
                      class={controlClass('flex-1 px-2 py-1.5 text-xs font-mono')}
                    />
                  </div>
                  <input
                    type="text"
                    value={field().value}
                    onInput={(e) =>
                      props.updateCustomFieldInput(index, { value: e.currentTarget.value })
                    }
                    placeholder={
                      field().placeholder || ALERT_WEBHOOK_CUSTOM_FIELD_VALUE_PLACEHOLDER
                    }
                    class={controlClass('flex-1 px-2 py-1.5 text-xs font-mono')}
                  />
                  <Show when={!field().required}>
                    <button
                      type="button"
                      onClick={() => props.removeCustomFieldInput(index)}
                      class="px-2 py-1 text-xs text-red-600 hover:underline dark:text-red-400"
                    >
                      {ALERT_WEBHOOK_CUSTOM_FIELD_REMOVE_LABEL}
                    </button>
                  </Show>
                </div>
              )}
            </Index>
            <button
              type="button"
              onClick={props.addCustomFieldInput}
              class="w-full border border-dashed border-border px-2 py-1 text-xs hover:bg-surface-hover"
            >
              {ALERT_WEBHOOK_CUSTOM_FIELD_ADD_LABEL}
            </button>
          </div>
          <p class="mt-2 text-xs text-muted">{ALERT_WEBHOOK_CUSTOM_FIELDS_PUSHHOVER_HELP}</p>
        </div>
      </Show>

      <div class={formField}>
        <label class={labelClass('flex items-center gap-2')}>
          Custom headers
          <span class="text-xs text-muted">{ALERT_WEBHOOK_HEADERS_HELP_LABEL}</span>
        </label>
        <div class="space-y-2 text-xs">
          <Index each={props.headerInputs()}>
            {(header, index) => (
              <div class="flex gap-2 text-xs">
                <input
                  type="text"
                  value={header().key}
                  onInput={(e) => props.updateHeaderInput(index, { key: e.currentTarget.value })}
                  placeholder={ALERT_WEBHOOK_HEADER_KEY_PLACEHOLDER}
                  class={controlClass('flex-1 px-2 py-1.5 text-xs font-mono')}
                />
                <input
                  type="text"
                  value={header().value}
                  onInput={(e) =>
                    props.updateHeaderInput(index, { value: e.currentTarget.value })
                  }
                  placeholder={ALERT_WEBHOOK_HEADER_VALUE_PLACEHOLDER}
                  class={controlClass('flex-1 px-2 py-1.5 text-xs font-mono')}
                />
                <button
                  type="button"
                  onClick={() => props.removeHeaderInput(index)}
                  class="px-2 py-1 text-xs text-red-600 hover:underline dark:text-red-400"
                >
                  {ALERT_WEBHOOK_HEADER_REMOVE_LABEL}
                </button>
              </div>
            )}
          </Index>
          <button
            type="button"
            onClick={props.addHeaderInput}
            class="w-full border border-dashed border-border px-2 py-1 text-xs hover:bg-surface-hover"
          >
            {ALERT_WEBHOOK_HEADER_ADD_LABEL}
          </button>
        </div>
        <p class="mt-2 text-xs text-muted">{ALERT_WEBHOOK_HEADER_HELP}</p>
      </div>

      <div>
        <label class="flex items-center gap-2 text-sm text-base-content">
          <input
            type="checkbox"
            checked={props.formData().enabled}
            onChange={(e) =>
              props.setFormData((prev) => ({ ...prev, enabled: e.currentTarget.checked }))
            }
            class={formCheckbox}
          />
          <span>{ALERT_WEBHOOK_ENABLE_LABEL}</span>
        </label>
      </div>

      <div class="flex justify-end gap-2 text-xs">
        <button
          onClick={props.cancelForm}
          class="px-3 py-1.5 border border-border rounded text-xs hover:bg-surface-hover"
        >
          {ALERT_WEBHOOK_CANCEL_LABEL}
        </button>
        <Show when={props.formData().url && props.formData().name}>
          <button
            onClick={props.testWebhookForm}
            disabled={props.testing === (props.editingId() || 'temp-new-webhook')}
            class="px-3 py-1.5 border border-border rounded text-xs hover:bg-slate-100"
          >
            {getAlertWebhookTestLabel(
              props.testing === (props.editingId() || 'temp-new-webhook'),
              true,
            )}
          </button>
        </Show>
        <button
          onClick={props.saveWebhook}
          disabled={!props.formData().name || !props.formData().url}
          class="px-3 py-1.5 bg-blue-600 text-white text-xs rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {getAlertWebhookSubmitLabel(Boolean(props.editingId()))}
        </button>
      </div>
    </div>
  );
}
