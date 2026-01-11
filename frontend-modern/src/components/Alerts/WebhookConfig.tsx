import { createSignal, createEffect, Show, For, Index } from 'solid-js';
import { NotificationsAPI, Webhook } from '@/api/notifications';
import { logger } from '@/utils/logger';
import {
  formField,
  labelClass,
  controlClass,
  formHelpText,
  formCheckbox,
} from '@/components/shared/Form';

interface WebhookTemplate {
  service: string;
  name: string;
  urlPattern: string;
  method: string;
  headers: Record<string, string>;
  payloadTemplate: string;
  instructions: string;
}

interface WebhookConfigProps {
  webhooks: Webhook[];
  onAdd: (webhook: Omit<Webhook, 'id'>) => void;
  onUpdate: (webhook: Webhook) => void;
  onDelete: (id: string) => void;
  onTest: (id: string, webhookData?: Omit<Webhook, 'id'>) => void;
  testing?: string | null;
}

type HeaderInput = { id: string; key: string; value: string };

type CustomFieldPreset = {
  key: string;
  label: string;
  placeholder?: string;
  required?: boolean;
};

type CustomFieldInput = HeaderInput & {
  label?: string;
  placeholder?: string;
  required?: boolean;
};

const customFieldPresets: Record<string, CustomFieldPreset[]> = {
  pushover: [
    {
      key: 'app_token',
      label: 'Application Token',
      placeholder: 'Your Pushover application token',
      required: true,
    },
    {
      key: 'user_token',
      label: 'User Key',
      placeholder: 'Primary user key or group key',
      required: true,
    },
  ],
};

const buildMapFromInputs = (inputs: Array<{ key: string; value: string }>): Record<string, string> => {
  const map: Record<string, string> = {};
  inputs.forEach(({ key, value }) => {
    if (key) {
      map[key] = value;
    }
  });
  return map;
};

const createCustomFieldInputs = (
  service: string,
  existing: Record<string, string> = {},
): CustomFieldInput[] => {
  const presets = customFieldPresets[service];
  const timestamp = Date.now();

  if (!presets) {
    return Object.entries(existing).map(([key, value], index) => ({
      id: `custom-${key}-${timestamp}-${index}`,
      key,
      value,
    }));
  }

  const inputs: CustomFieldInput[] = presets.map((preset, index) => ({
    id: `custom-${preset.key}-${timestamp}-${index}`,
    key: preset.key,
    value: existing[preset.key] ?? '',
    label: preset.label,
    placeholder: preset.placeholder,
    required: preset.required,
  }));

  Object.entries(existing)
    .filter(([key]) => !presets.some((preset) => preset.key === key))
    .forEach(([key, value], index) => {
      inputs.push({
        id: `custom-${key}-${timestamp}-${presets.length + index}`,
        key,
        value,
      });
    });

  return inputs;
};

export function WebhookConfig(props: WebhookConfigProps) {
  const [adding, setAdding] = createSignal(false);
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [formData, setFormData] = createSignal<
    Omit<Webhook, 'id'> & { service: string; payloadTemplate?: string }
  >({
    name: '',
    url: '',
    method: 'POST',
    service: 'generic',
    headers: { 'Content-Type': 'application/json' },
    enabled: true,
    payloadTemplate: '',
    customFields: {},
  });
  const [templates, setTemplates] = createSignal<WebhookTemplate[]>([]);
  const [showServiceDropdown, setShowServiceDropdown] = createSignal(false);

  // Track header inputs separately to avoid focus loss
  const [headerInputs, setHeaderInputs] = createSignal<HeaderInput[]>([]);
  const [customFieldInputs, _setCustomFieldInputs] = createSignal<CustomFieldInput[]>([]);

  const setCustomFieldInputs = (inputs: CustomFieldInput[]) => {
    _setCustomFieldInputs(inputs);
    setFormData((prev) => ({
      ...prev,
      customFields: buildMapFromInputs(inputs),
    }));
  };

  const updateCustomFieldInputs = (
    updater: (inputs: CustomFieldInput[]) => CustomFieldInput[],
  ) => {
    _setCustomFieldInputs((prev) => {
      const next = updater(prev);
      setFormData((prevForm) => ({
        ...prevForm,
        customFields: buildMapFromInputs(next),
      }));
      return next;
    });
  };

  const ensurePresetCustomFields = (service: string) => {
    if (!customFieldPresets[service]) {
      return;
    }
    const existing = formData().customFields || {};
    const inputs = createCustomFieldInputs(service, existing);
    setCustomFieldInputs(inputs);
  };

  // Load webhook templates
  createEffect(async () => {
    try {
      const data = await NotificationsAPI.getWebhookTemplates();
      setTemplates(data);
    } catch (err) {
      logger.error('Failed to load webhook templates:', err);
    }
  });

  const saveWebhook = () => {
    const data = formData();
    if (!data.name || !data.url) return;

    // Build headers from headerInputs
    const headers: Record<string, string> = {};
    headerInputs().forEach((input) => {
      if (input.key) {
        headers[input.key] = input.value;
      }
    });
    const customFields = buildMapFromInputs(customFieldInputs());

    if (editingId()) {
      props.onUpdate({
        ...data,
        id: editingId()!,
        headers,
        service: data.service,
        template: data.payloadTemplate,
        customFields,
      });
      setEditingId(null);
      setAdding(false);
      setHeaderInputs([]);
      setCustomFieldInputs([]);
    } else {
      // onAdd expects a webhook without id, but with service
      const newWebhook: Omit<Webhook, 'id'> = {
        name: data.name,
        url: data.url,
        method: data.method,
        headers,
        enabled: data.enabled,
        service: data.service,
        template: data.payloadTemplate,
        customFields,
      };
      props.onAdd(newWebhook);
      // Reset form and close the adding panel
      setFormData({
        name: '',
        url: '',
        method: 'POST',
        service: 'generic',
        headers: { 'Content-Type': 'application/json' },
        enabled: true,
        payloadTemplate: '',
        customFields: {},
      });
      setHeaderInputs([]);
       setCustomFieldInputs([]);
      setAdding(false);
    }
  };

  const cancelForm = () => {
    setAdding(false);
    setEditingId(null);
    setFormData({
      name: '',
      url: '',
      method: 'POST',
      service: 'generic',
      headers: { 'Content-Type': 'application/json' },
      enabled: true,
      payloadTemplate: '',
      customFields: {},
    });
    setHeaderInputs([]);
    setCustomFieldInputs([]);
  };

  const editWebhook = (webhook: Webhook) => {
    if (webhook.id) {
      setEditingId(webhook.id);
    }
    setFormData({
      ...webhook,
      service: webhook.service || 'generic',
      payloadTemplate: webhook.template || '',
      customFields: webhook.customFields || {},
    });
    // Set up header inputs for editing
    const headers = webhook.headers || {};
    setHeaderInputs(
      Object.entries(headers).map(([key, value], index) => ({
        id: `header-${Date.now()}-${index}`,
        key,
        value,
      })),
    );

    const service = webhook.service || 'generic';
    const existingCustomFields = webhook.customFields || {};
    if (customFieldPresets[service] || Object.keys(existingCustomFields).length > 0) {
      setCustomFieldInputs(createCustomFieldInputs(service, existingCustomFields));
    } else {
      setCustomFieldInputs([]);
    }
    setAdding(true);
  };

  const selectService = (service: string) => {
    const template = templates().find((t) => t.service === service);
    if (template) {
      setFormData({
        ...formData(),
        service: template.service,
        method: template.method,
        headers: { ...template.headers },
        name: formData().name || template.name,
        // Clear the payload template when switching services
        // Only generic service should have custom payloads
        payloadTemplate: service === 'generic' ? formData().payloadTemplate : '',
      });
      // Update header inputs when switching services
      const headers = template.headers || {};
      setHeaderInputs(
        Object.entries(headers).map(([key, value], index) => ({
          id: `header-${Date.now()}-${index}`,
          key,
          value,
        })),
      );
    } else {
      setFormData({
        ...formData(),
        service,
      });
    }
    ensurePresetCustomFields(service);
    setShowServiceDropdown(false);
  };

  const currentTemplate = () => templates().find((t) => t.service === formData().service);
  const serviceName = (service: string) => {
    const names: Record<string, string> = {
      generic: 'Generic',
      discord: 'Discord',
      slack: 'Slack',
      mattermost: 'Mattermost',
      telegram: 'Telegram',
      teams: 'Microsoft Teams',
      'teams-adaptive': 'Teams (Adaptive)',
      pagerduty: 'PagerDuty',
      pushover: 'Pushover',
      gotify: 'Gotify',
      ntfy: 'ntfy',
    };
    return names[service] || service;
  };

  const toggleAllWebhooks = (enabled: boolean) => {
    props.webhooks.forEach((webhook) => {
      props.onUpdate({ ...webhook, enabled });
    });
  };

  const allEnabled = () => props.webhooks.every((w) => w.enabled);
  const someEnabled = () => props.webhooks.some((w) => w.enabled);

  return (
    <div class="space-y-6 min-w-0 w-full">
      {/* Existing Webhooks List */}
      <Show when={props.webhooks.length > 0}>
        <div class="space-y-3 w-full">
          {/* Quick Actions Bar */}
          <div class="flex flex-col gap-2 rounded border border-gray-200 px-3 py-3 text-xs dark:border-gray-700 sm:flex-row sm:items-center sm:justify-between">
            <div class="text-gray-600 dark:text-gray-400 sm:text-sm">
              {props.webhooks.filter((w) => w.enabled).length} of {props.webhooks.length} webhooks
              enabled
            </div>
            <div class="flex flex-wrap gap-2 sm:flex-nowrap">
              <button
                onClick={() => toggleAllWebhooks(false)}
                disabled={!someEnabled()}
                class="w-full rounded border border-gray-300 px-3 py-1 text-xs text-gray-700 transition-colors hover:bg-gray-100 dark:border-gray-600 dark:text-gray-200 dark:hover:bg-gray-700 sm:w-auto"
              >
                Disable All
              </button>
              <button
                onClick={() => toggleAllWebhooks(true)}
                disabled={allEnabled()}
                class="w-full rounded border border-green-500 px-3 py-1 text-xs text-green-700 transition-colors hover:bg-green-50 dark:border-green-600 dark:text-green-400 dark:hover:bg-green-900/20 sm:w-auto"
              >
                Enable All
              </button>
            </div>
          </div>
          <For each={props.webhooks}>
            {(webhook) => (
              <div class="w-full px-3 py-3 border border-gray-200 text-xs dark:border-gray-700 sm:text-sm">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span class="font-medium text-gray-800 dark:text-gray-200">{webhook.name}</span>
                  <button
                    onClick={() => props.onUpdate({ ...webhook, enabled: !webhook.enabled })}
                    class={`rounded border px-3 py-1 text-xs font-medium transition-colors ${
                      webhook.enabled
                        ? 'border-green-500 text-green-700 hover:bg-green-50 dark:border-green-600 dark:text-green-400 dark:hover:bg-green-900/20'
                        : 'border-gray-300 text-gray-600 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700'
                    }`}
                  >
                    {webhook.enabled ? 'Enabled' : 'Disabled'}
                  </button>
                </div>
                <div class="mt-2 flex flex-wrap gap-2 text-[11px] text-gray-600 dark:text-gray-400 sm:text-xs">
                  <span class="rounded bg-gray-200 px-2 py-0.5 text-gray-700 dark:bg-gray-600 dark:text-gray-200">
                    {serviceName(webhook.service || 'generic')}
                  </span>
                  <span class="rounded bg-gray-200 px-2 py-0.5 text-gray-700 dark:bg-gray-600 dark:text-gray-200">
                    {webhook.method}
                  </span>
                  <span class="rounded bg-gray-200 px-2 py-0.5 text-gray-700 dark:bg-gray-600 dark:text-gray-200">
                    ID: {webhook.id || '—'}
                  </span>
                </div>
                <p class="mt-2 break-all text-[11px] font-mono text-gray-500 dark:text-gray-400 sm:text-xs">
                  {webhook.url}
                </p>
                <div class="mt-3 flex flex-wrap gap-2 border-t border-gray-100 pt-2 dark:border-gray-700 sm:justify-end w-full">
                  <button
                    onClick={() => webhook.id && props.onTest(webhook.id)}
                    disabled={props.testing === webhook.id || !webhook.enabled}
                    class="rounded border border-gray-300 px-3 py-1 text-xs text-gray-700 transition-colors hover:bg-gray-100 dark:border-gray-600 dark:text-gray-200 dark:hover:bg-gray-700 disabled:opacity-50"
                  >
                    {props.testing === webhook.id ? 'Testing…' : 'Test'}
                  </button>
                  <button
                    onClick={() => editWebhook(webhook)}
                    class="rounded border border-blue-300 px-3 py-1 text-xs text-blue-600 transition-colors hover:bg-blue-50 dark:border-blue-500 dark:text-blue-300 dark:hover:bg-blue-900/20"
                  >
                    Edit
                  </button>
                  <button
                    onClick={() => webhook.id && props.onDelete(webhook.id)}
                    class="rounded border border-red-300 px-3 py-1 text-xs text-red-600 transition-colors hover:bg-red-50 dark:border-red-500 dark:text-red-300 dark:hover:bg-red-900/20"
                  >
                    Delete
                  </button>
                </div>
              </div>
            )}
          </For>
        </div>
      </Show>

      {/* Add/Edit Form */}
      <Show when={adding()}>
        <div class="space-y-4 text-sm">
          {/* Service Selection */}
          <div>
            <div class="flex items-center justify-between mb-4">
              <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                Service Type
              </label>
              <button
                type="button"
                onClick={() => setShowServiceDropdown(!showServiceDropdown())}
                class="text-sm text-blue-600 hover:text-blue-700 dark:text-blue-400"
              >
                {serviceName(formData().service)} →
              </button>
            </div>

            <Show when={showServiceDropdown()}>
              <div class="grid grid-cols-1 md:grid-cols-2 gap-2 border border-gray-200 dark:border-gray-700 px-3 py-2 mb-3 text-xs">
                <For
                  each={[
                    'generic',
                    'discord',
                    'slack',
                    'mattermost',
                    'telegram',
                    'teams',
                    'teams-adaptive',
                    'pagerduty',
                    'pushover',
                    'gotify',
                    'ntfy',
                  ]}
                >
                  {(service) => (
                    <button
                      type="button"
                      onClick={() => selectService(service)}
                      class={`px-2 py-1.5 text-left border transition-colors text-xs ${
                        formData().service === service
                          ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                          : 'border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700/30'
                      }`}
                    >
                      <div class="font-medium text-xs text-gray-800 dark:text-gray-200">
                        {serviceName(service)}
                      </div>
                      <div class="text-[11px] text-gray-600 dark:text-gray-400 mt-1">
                        {service === 'generic'
                          ? 'Custom webhook endpoint'
                          : service === 'discord'
                            ? 'Discord server webhook'
                            : service === 'slack'
                              ? 'Slack incoming webhook'
                              : service === 'mattermost'
                                ? 'Mattermost incoming webhook'
                                : service === 'telegram'
                                  ? 'Telegram bot notifications'
                                  : service === 'teams'
                                    ? 'Microsoft Teams webhook'
                                    : service === 'teams-adaptive'
                                      ? 'Teams with Adaptive Cards'
                                      : service === 'pushover'
                                        ? 'Mobile push notifications'
                                        : service === 'gotify'
                                          ? 'Self-hosted push notifications'
                                          : service === 'ntfy'
                                            ? 'Push notifications via ntfy.sh'
                                            : 'PagerDuty Events API v2'}
                      </div>
                    </button>
                  )}
                </For>
              </div>
            </Show>

            <Show when={currentTemplate()?.instructions}>
              <div class="mb-3 border-l-2 border-blue-300 pl-3 text-xs leading-relaxed text-blue-800 dark:border-blue-700 dark:text-blue-200">
                <h4 class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-2">
                  Setup Instructions
                </h4>
                {currentTemplate()!.instructions}
              </div>
            </Show>
          </div>

          {/* Basic Configuration */}
          <div class="grid w-full grid-cols-1 gap-3 md:grid-cols-2">
            <div class={formField}>
              <label class={labelClass()}>Name</label>
              <input
                type="text"
                value={formData().name}
                onInput={(e) => setFormData({ ...formData(), name: e.currentTarget.value })}
                placeholder={currentTemplate()?.name || 'My Webhook'}
                class={controlClass('px-2 py-1.5')}
              />
            </div>

            <div class={formField}>
              <label class={labelClass()}>HTTP method</label>
              <select
                value={formData().method}
                onChange={(e) => setFormData({ ...formData(), method: e.currentTarget.value })}
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
              value={formData().url}
              onInput={(e) => setFormData({ ...formData(), url: e.currentTarget.value })}
              placeholder={currentTemplate()?.urlPattern || 'https://example.com/webhook'}
              class={controlClass('px-2 py-1.5 font-mono')}
            />
            <p class={formHelpText + ' mt-1'}>
              Supports template variables like{' '}
              <code class="font-mono text-[11px] text-gray-600 dark:text-gray-300">
                {'{{.Message}}'}
              </code>
              . Use{' '}
              <code class="font-mono text-[11px] text-gray-600 dark:text-gray-300">
                {'{{urlpath ...}}'}
              </code>{' '}
              or{' '}
              <code class="font-mono text-[11px] text-gray-600 dark:text-gray-300">
                {'{{urlquery ...}}'}
              </code>{' '}
              to keep dynamic values URL-safe.
            </p>
          </div>

          {/* Custom Payload Template - only show for generic service */}
          <Show when={formData().service === 'generic'}>
            <div class={formField}>
              <label class={labelClass('flex items-center gap-2')}>
                Custom payload template (JSON)
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  Optional — leave empty to use default
                </span>
              </label>
              <textarea
                value={formData().payloadTemplate || ''}
                onInput={(e) =>
                  setFormData({ ...formData(), payloadTemplate: e.currentTarget.value })
                }
                placeholder={`{
  "text": "Alert: {{.Level}} - {{.Message}}",
  "resource": "{{.ResourceName}}",
  "value": {{.Value}},
  "threshold": {{.Threshold}}
}`}
                rows={8}
                class={controlClass('px-2 py-1.5 text-xs font-mono min-h-[160px]')}
              />
              <p class={formHelpText + ' mt-1'}>
                Available variables:{' '}
                {
                  '{{.ID}}, {{.Level}}, {{.Type}}, {{.ResourceName}}, {{.Node}}, {{.Message}}, {{.Value}}, {{.Threshold}}, {{.Duration}}, {{.Timestamp}}'
                }
              </p>
            </div>
          </Show>

          {/* Custom Fields Section */}
          <Show when={customFieldInputs().length > 0 || formData().service === 'pushover'}>
            <div class={formField}>
              <label class={labelClass('flex items-center gap-2')}>
                Custom fields
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  Available as{' '}
                  <code class="font-mono text-[11px] text-gray-600 dark:text-gray-300">
                    {'{{.CustomFields.<name>}}'}
                  </code>{' '}
                  in templates
                </span>
              </label>
              <div class="space-y-2 text-xs">
                <Index each={customFieldInputs()}>
                  {(field, index) => (
                    <div class="flex gap-2 text-xs">
                      <div class="flex flex-1 flex-col gap-1">
                        <Show when={field().label}>
                          <span class="text-[11px] text-gray-500 dark:text-gray-400">
                            {field().label}
                          </span>
                        </Show>
                        <input
                          type="text"
                          value={field().key}
                          disabled={field().required}
                          onInput={(e) => {
                            const newKey = e.currentTarget.value;
                            updateCustomFieldInputs((inputs) => {
                              const next = [...inputs];
                              next[index] = { ...next[index], key: newKey };
                              return next;
                            });
                          }}
                          placeholder="Field name"
                          class={controlClass('flex-1 px-2 py-1.5 text-xs font-mono')}
                        />
                      </div>
                      <input
                        type="text"
                        value={field().value}
                        onInput={(e) => {
                          const newValue = e.currentTarget.value;
                          updateCustomFieldInputs((inputs) => {
                            const next = [...inputs];
                            next[index] = { ...next[index], value: newValue };
                            return next;
                          });
                        }}
                        placeholder={field().placeholder || 'Value'}
                        class={controlClass('flex-1 px-2 py-1.5 text-xs font-mono')}
                      />
                      <Show when={!field().required}>
                        <button
                          type="button"
                          onClick={() => {
                            updateCustomFieldInputs((inputs) =>
                              inputs.filter((_, i) => i !== index),
                            );
                          }}
                          class="px-2 py-1 text-xs text-red-600 hover:underline dark:text-red-400"
                        >
                          Remove
                        </button>
                      </Show>
                    </div>
                  )}
                </Index>
                <button
                  type="button"
                  onClick={() => {
                    const newId = `custom-${Date.now()}-${Math.random()}`;
                    updateCustomFieldInputs((inputs) => [
                      ...inputs,
                      {
                        id: newId,
                        key: '',
                        value: '',
                      },
                    ]);
                  }}
                  class="w-full border border-dashed border-gray-300 px-2 py-1 text-xs text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800"
                >
                  + Add custom field
                </button>
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                Need Pushover? Provide your Application Token and User Key here.
              </p>
            </div>
          </Show>

          {/* Custom Headers Section */}
          <div class={formField}>
            <label class={labelClass('flex items-center gap-2')}>
              Custom headers
              <span class="text-xs text-gray-500 dark:text-gray-400">
                Add authentication tokens or custom headers
              </span>
            </label>
            <div class="space-y-2 text-xs">
              <Index each={headerInputs()}>
                {(header, index) => (
                  <div class="flex gap-2 text-xs">
                    <input
                      type="text"
                      value={header().key}
                      onInput={(e) => {
                        const newKey = e.currentTarget.value;
                        setHeaderInputs((inputs) => {
                          const newInputs = [...inputs];
                          newInputs[index] = { ...newInputs[index], key: newKey };
                          return newInputs;
                        });
                      }}
                      placeholder="Header name"
                      class={controlClass('flex-1 px-2 py-1.5 text-xs font-mono')}
                    />
                    <input
                      type="text"
                      value={header().value}
                      onInput={(e) => {
                        const newValue = e.currentTarget.value;
                        setHeaderInputs((inputs) => {
                          const newInputs = [...inputs];
                          newInputs[index] = { ...newInputs[index], value: newValue };
                          return newInputs;
                        });
                      }}
                      placeholder="Header value"
                      class={controlClass('flex-1 px-2 py-1.5 text-xs font-mono')}
                    />
                    <button
                      type="button"
                      onClick={() => {
                        setHeaderInputs((inputs) => inputs.filter((_, i) => i !== index));
                      }}
                      class="px-2 py-1 text-xs text-red-600 hover:underline dark:text-red-400"
                    >
                      Remove
                    </button>
                  </div>
                )}
              </Index>
              <button
                type="button"
                onClick={() => {
                  const newId = `header-${Date.now()}-${Math.random()}`;
                  setHeaderInputs([
                    ...headerInputs(),
                    {
                      id: newId,
                      key: '',
                      value: '',
                    },
                  ]);
                }}
                class="w-full border border-dashed border-gray-300 px-2 py-1 text-xs text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800"
              >
                + Add header
              </button>
            </div>
            <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
              Common headers: Authorization (Bearer token), X-API-Key, X-Auth-Token
            </p>
          </div>

          <div>
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input
                type="checkbox"
                checked={formData().enabled}
                onChange={(e) => setFormData({ ...formData(), enabled: e.currentTarget.checked })}
                class={formCheckbox}
              />
              <span>Enable this webhook</span>
            </label>
          </div>

          <div class="flex justify-end gap-2 text-xs">
            <button
              onClick={cancelForm}
              class="px-3 py-1.5 border border-gray-300 rounded text-xs hover:bg-gray-100 dark:border-gray-600 dark:text-gray-200"
            >
              Cancel
            </button>
            <Show when={formData().url && formData().name}>
              <button
                onClick={() => {
                  // Test the webhook with current form data
                  // Build headers from headerInputs
                  const headers: Record<string, string> = {};
                  headerInputs().forEach((input) => {
                    if (input.key) {
                      headers[input.key] = input.value;
                    }
                  });
                  const customFields = buildMapFromInputs(customFieldInputs());
                  const { payloadTemplate, ...restFormData } = formData();
                  const testPayload = {
                    ...restFormData,
                    headers,
                    customFields,
                    template: payloadTemplate ?? restFormData.template ?? '',
                  };
                  // Use a consistent temporary ID for this form session
                  const tempId = editingId() || 'temp-new-webhook';
                  props.onTest(tempId, testPayload);
                }}
                disabled={props.testing === (editingId() || 'temp-new-webhook')}
                class="px-3 py-1.5 border border-gray-300 rounded text-xs hover:bg-gray-100 dark:border-gray-600 dark:text-gray-200"
              >
                {props.testing === (editingId() || 'temp-new-webhook') ? 'Testing...' : 'Test'}
              </button>
            </Show>
            <button
              onClick={saveWebhook}
              disabled={!formData().name || !formData().url}
              class="px-3 py-1.5 bg-blue-600 text-white text-xs rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {editingId() ? 'Update' : 'Add'} Webhook
            </button>
          </div>
        </div>
      </Show>

      {/* Add Webhook Button */}
      <Show when={!adding()}>
        <button
          onClick={() => {
            setAdding(true);
            // Initialize with default Content-Type header
            setHeaderInputs([
              {
                id: `header-${Date.now()}-0`,
                key: 'Content-Type',
                value: 'application/json',
              },
            ]);
            setCustomFieldInputs([]);
          }}
          class="w-full border border-dashed border-gray-300 px-2 py-1 text-xs text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800"
        >
          + Add Webhook
        </button>
      </Show>
    </div>
  );
}
