import { createSignal, createEffect, Show, For } from 'solid-js';
import { NotificationsAPI, Webhook } from '@/api/notifications';

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
  onTest: (id: string) => void;
  testing?: string | null;
}

export function WebhookConfig(props: WebhookConfigProps) {
  const [adding, setAdding] = createSignal(false);
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [formData, setFormData] = createSignal<Omit<Webhook, 'id'> & { service: string; payloadTemplate?: string }>({
    name: '',
    url: '',
    method: 'POST',
    service: 'generic',
    headers: { 'Content-Type': 'application/json' },
    enabled: true,
    payloadTemplate: ''
  });
  const [templates, setTemplates] = createSignal<WebhookTemplate[]>([]);
  const [showServiceDropdown, setShowServiceDropdown] = createSignal(false);
  
  // Load webhook templates
  createEffect(async () => {
    try {
      const data = await NotificationsAPI.getWebhookTemplates();
      setTemplates(data);
    } catch (err) {
      console.error('Failed to load webhook templates:', err);
    }
  });
  
  const saveWebhook = () => {
    const data = formData();
    if (!data.name || !data.url) return;
    
    if (editingId()) {
      props.onUpdate({ 
        ...data, 
        id: editingId()!, 
        service: data.service,
        template: data.payloadTemplate 
      });
      setEditingId(null);
      setAdding(false);
    } else {
      // onAdd expects a webhook without id, but with service
      const newWebhook: Omit<Webhook, 'id'> = {
        name: data.name,
        url: data.url,
        method: data.method,
        headers: data.headers,
        enabled: data.enabled,
        service: data.service,
        template: data.payloadTemplate
      };
      props.onAdd(newWebhook);
      // Reset form but keep adding state true
      setFormData({
        name: '',
        url: '',
        method: 'POST',
        service: 'generic',
        headers: { 'Content-Type': 'application/json' },
        enabled: true,
        payloadTemplate: ''
      });
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
      payloadTemplate: ''
    });
  };
  
  const editWebhook = (webhook: Webhook) => {
    setEditingId(webhook.id!);
    setFormData({
      ...webhook,
      service: webhook.service || 'generic',
      payloadTemplate: webhook.template || ''
    });
    setAdding(true);
  };
  
  const selectService = (service: string) => {
    const template = templates().find(t => t.service === service);
    if (template) {
      setFormData({
        ...formData(),
        service: template.service,
        method: template.method,
        headers: { ...template.headers },
        name: formData().name || template.name
      });
    }
    setShowServiceDropdown(false);
  };
  
  const currentTemplate = () => templates().find(t => t.service === formData().service);
  const serviceName = (service: string) => {
    const names: Record<string, string> = {
      generic: 'Generic',
      discord: 'Discord',
      slack: 'Slack',
      telegram: 'Telegram',
      teams: 'Microsoft Teams',
      'teams-adaptive': 'Teams (Adaptive)',
      pagerduty: 'PagerDuty'
    };
    return names[service] || service;
  };
  
  return (
    <div class="space-y-6">
      {/* Existing Webhooks List */}
      <Show when={props.webhooks.length > 0}>
        <div class="space-y-3">
          <For each={props.webhooks}>
            {(webhook) => (
              <div class="p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                <div class="flex items-center justify-between">
                  <div class="flex-1">
                    <div class="flex items-center gap-3 mb-1">
                      <span class="font-medium text-sm text-gray-800 dark:text-gray-200">
                        {webhook.name}
                      </span>
                      <span class="text-xs px-2 py-0.5 rounded bg-gray-200 dark:bg-gray-600 text-gray-600 dark:text-gray-300">
                        {serviceName(webhook.service || 'generic')}
                      </span>
                      <span class="text-xs px-2 py-0.5 rounded bg-gray-200 dark:bg-gray-600 text-gray-600 dark:text-gray-300">
                        {webhook.method}
                      </span>
                      {!webhook.enabled && (
                        <span class="text-xs px-2 py-0.5 rounded bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400">
                          Disabled
                        </span>
                      )}
                    </div>
                    <p class="text-xs text-gray-500 dark:text-gray-400 font-mono truncate">
                      {webhook.url}
                    </p>
                  </div>
                  <div class="flex items-center gap-2 ml-4">
                    <button 
                      onClick={() => props.onTest(webhook.id!)}
                      disabled={props.testing === webhook.id}
                      class="px-3 py-1 text-xs text-gray-600 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300 disabled:opacity-50"
                    >
                      {props.testing === webhook.id ? 'Testing...' : 'Test'}
                    </button>
                    <button 
                      onClick={() => editWebhook(webhook)}
                      class="px-3 py-1 text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400"
                    >
                      Edit
                    </button>
                    <button 
                      onClick={() => props.onDelete(webhook.id!)}
                      class="px-3 py-1 text-xs text-red-600 hover:text-red-700 dark:text-red-400"
                    >
                      Delete
                    </button>
                  </div>
                </div>
              </div>
            )}
          </For>
        </div>
      </Show>
      
      {/* Add/Edit Form */}
      <Show when={adding()}>
        <div class="space-y-4">
          {/* Service Selection */}
          <div>
            <div class="flex items-center justify-between mb-4">
              <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                Service Type
              </label>
              <button
                onClick={() => setShowServiceDropdown(!showServiceDropdown())}
                class="text-sm text-blue-600 hover:text-blue-700 dark:text-blue-400"
              >
                {serviceName(formData().service)} →
              </button>
            </div>
            
            <Show when={showServiceDropdown()}>
              <div class="grid grid-cols-1 md:grid-cols-2 gap-2 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg mb-4">
                <For each={['generic', 'discord', 'slack', 'telegram', 'teams', 'teams-adaptive', 'pagerduty']}>
                  {(service) => (
                    <button
                      onClick={() => selectService(service)}
                      class={`p-3 text-left rounded-lg border transition-all ${
                        formData().service === service
                          ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                          : 'border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700/50'
                      }`}
                    >
                      <div class="font-medium text-sm text-gray-800 dark:text-gray-200">
                        {serviceName(service)}
                      </div>
                      <div class="text-xs text-gray-600 dark:text-gray-400 mt-1">
                        {service === 'generic' ? 'Custom webhook endpoint' :
                         service === 'discord' ? 'Discord server webhook' :
                         service === 'slack' ? 'Slack incoming webhook' :
                         service === 'telegram' ? 'Telegram bot notifications' :
                         service === 'teams' ? 'Microsoft Teams webhook' :
                         service === 'teams-adaptive' ? 'Teams with Adaptive Cards' :
                         'PagerDuty Events API v2'}
                      </div>
                    </button>
                  )}
                </For>
              </div>
            </Show>
            
            <Show when={currentTemplate()?.instructions}>
              <div class="mb-4 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                <h4 class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-2">
                  Setup Instructions
                </h4>
                <pre class="text-xs text-blue-800 dark:text-blue-200 whitespace-pre-wrap">
                  {currentTemplate()!.instructions}
                </pre>
              </div>
            </Show>
          </div>
          
          {/* Basic Configuration */}
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Name
              </label>
              <input
                type="text"
                value={formData().name}
                onInput={(e) => setFormData({ ...formData(), name: e.currentTarget.value })}
                placeholder={currentTemplate()?.name || 'My Webhook'}
                class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
              />
            </div>
            
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                HTTP Method
              </label>
              <select
                value={formData().method}
                onChange={(e) => setFormData({ ...formData(), method: e.currentTarget.value })}
                class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
              >
                <option value="POST">POST</option>
                <option value="PUT">PUT</option>
                <option value="PATCH">PATCH</option>
              </select>
            </div>
          </div>
          
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Webhook URL
            </label>
            <input
              type="url"
              value={formData().url}
              onInput={(e) => setFormData({ ...formData(), url: e.currentTarget.value })}
              placeholder={currentTemplate()?.urlPattern || 'https://example.com/webhook'}
              class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600 font-mono"
            />
          </div>
          
          {/* Custom Payload Template - only show for generic service */}
          <Show when={formData().service === 'generic'}>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Custom Payload Template (JSON)
                <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">
                  Optional - Leave empty to use default
                </span>
              </label>
              <textarea
                value={formData().payloadTemplate || ''}
                onInput={(e) => setFormData({ ...formData(), payloadTemplate: e.currentTarget.value })}
                placeholder={`{
  "text": "Alert: {{.Level}} - {{.Message}}",
  "resource": "{{.ResourceName}}",
  "value": {{.Value}},
  "threshold": {{.Threshold}}
}`}
                rows={8}
                class="w-full px-3 py-2 text-xs font-mono border rounded-lg dark:bg-gray-700 dark:border-gray-600"
              />
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Available variables: {"{{.ID}}, {{.Level}}, {{.Type}}, {{.ResourceName}}, {{.Node}}, {{.Message}}, {{.Value}}, {{.Threshold}}, {{.Duration}}, {{.Timestamp}}"}
              </p>
            </div>
          </Show>
          
          <div>
            <label class="flex items-center gap-2">
              <input
                type="checkbox"
                checked={formData().enabled}
                onChange={(e) => setFormData({ ...formData(), enabled: e.currentTarget.checked })}
                class="rounded border-gray-300 dark:border-gray-600 text-blue-600"
              />
              <span class="text-sm text-gray-700 dark:text-gray-300">Enable this webhook</span>
            </label>
          </div>
          
          <div class="flex justify-end gap-2">
            <button 
              onClick={cancelForm}
              class="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
            >
              Cancel
            </button>
            <button 
              onClick={saveWebhook}
              disabled={!formData().name || !formData().url}
              class="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {editingId() ? 'Update' : 'Add'} Webhook
            </button>
          </div>
        </div>
      </Show>
      
      {/* Add Webhook Button */}
      <Show when={!adding()}>
        <button 
          onClick={() => setAdding(true)}
          class="w-full py-2 text-sm border border-dashed border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors text-gray-600 dark:text-gray-400"
        >
          + Add Webhook
        </button>
      </Show>
    </div>
  );
}