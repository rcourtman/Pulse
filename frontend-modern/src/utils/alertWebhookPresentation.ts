import type { WebhookTemplate as AlertWebhookTemplate } from '@/api/notifications';

type AlertWebhookService =
  | 'generic'
  | 'discord'
  | 'slack'
  | 'mattermost'
  | 'telegram'
  | 'teams'
  | 'teams-adaptive'
  | 'pagerduty'
  | 'pushover'
  | 'gotify'
  | 'ntfy';

type AlertWebhookServicePresentation = {
  label: string;
  mentionPlaceholder?: string;
  mentionHelp?: string;
};

type AlertWebhookCustomFieldPreset = {
  key: string;
  label: string;
  placeholder?: string;
  required?: boolean;
};

const ALERT_WEBHOOK_SERVICE_PRESENTATION: Record<
  AlertWebhookService,
  AlertWebhookServicePresentation
> = {
  generic: {
    label: 'Generic',
  },
  discord: {
    label: 'Discord',
    mentionPlaceholder: '@everyone or <@USER_ID> or <@&ROLE_ID>',
    mentionHelp: 'Discord: Use @everyone, @here, <@USER_ID>, or <@&ROLE_ID>',
  },
  slack: {
    label: 'Slack',
    mentionPlaceholder: '@channel, @here, or <@USER_ID>',
    mentionHelp: 'Slack: Use @channel, @here, <@USER_ID>, or <!subteam^ID>',
  },
  mattermost: {
    label: 'Mattermost',
    mentionPlaceholder: '@channel, @all, or @username',
    mentionHelp: 'Mattermost: Use @channel, @all, or @username',
  },
  telegram: {
    label: 'Telegram',
  },
  teams: {
    label: 'Microsoft Teams',
    mentionPlaceholder: '@General or user email',
    mentionHelp: 'Teams: Use channel names like @General',
  },
  'teams-adaptive': {
    label: 'Teams (Adaptive)',
    mentionPlaceholder: '@General or user email',
    mentionHelp: 'Teams: Use channel names like @General',
  },
  pagerduty: {
    label: 'PagerDuty',
  },
  pushover: {
    label: 'Pushover',
  },
  gotify: {
    label: 'Gotify',
  },
  ntfy: {
    label: 'ntfy',
  },
};

const ALERT_WEBHOOK_CUSTOM_FIELD_PRESETS: Partial<
  Record<string, readonly AlertWebhookCustomFieldPreset[]>
> = {
  pushover: [
    {
      key: 'token',
      label: 'Application Token',
      placeholder: 'Your Pushover application token',
      required: true,
    },
    {
      key: 'user',
      label: 'User Key',
      placeholder: 'Primary user key or group key',
      required: true,
    },
  ],
};

export const ALERT_WEBHOOK_SETUP_INSTRUCTIONS_TITLE = 'Setup Instructions';
export const ALERT_WEBHOOK_NAME_PLACEHOLDER = 'My Webhook';
export const ALERT_WEBHOOK_URL_PLACEHOLDER = 'https://example.com/webhook';
export const ALERT_WEBHOOK_URL_HELP_TEMPLATE_VARIABLE = '{{.Message}}';
export const ALERT_WEBHOOK_URL_HELP_PATH = '{{urlpath ...}}';
export const ALERT_WEBHOOK_URL_HELP_QUERY = '{{urlquery ...}}';
export const ALERT_WEBHOOK_MENTION_HELP_LABEL = 'Optional — tag users or groups';
export const ALERT_WEBHOOK_MENTION_FALLBACK_PLACEHOLDER = '@everyone';
export const ALERT_WEBHOOK_PAYLOAD_HELP_LABEL = 'Optional — leave empty to use default';
export const ALERT_WEBHOOK_PAYLOAD_TEMPLATE_PLACEHOLDER = `{
  "text": "Alert: {{.Level}} - {{.Message}}",
  "resource": "{{.ResourceName}}",
  "value": {{.Value}},
  "threshold": {{.Threshold}}
}`;
export const ALERT_WEBHOOK_PAYLOAD_VARIABLES =
  '{{.ID}}, {{.Level}}, {{.Type}}, {{.ResourceName}}, {{.Node}}, {{.Message}}, {{.Value}}, {{.Threshold}}, {{.Duration}}, {{.Timestamp}}';
export const ALERT_WEBHOOK_CUSTOM_FIELDS_HELP = 'Available as';
export const ALERT_WEBHOOK_CUSTOM_FIELDS_REFERENCE = '{{.CustomFields.<name>}}';
export const ALERT_WEBHOOK_CUSTOM_FIELD_KEY_PLACEHOLDER = 'Field name';
export const ALERT_WEBHOOK_CUSTOM_FIELD_VALUE_PLACEHOLDER = 'Value';
export const ALERT_WEBHOOK_CUSTOM_FIELD_REMOVE_LABEL = 'Remove';
export const ALERT_WEBHOOK_CUSTOM_FIELD_ADD_LABEL = '+ Add custom field';
export const ALERT_WEBHOOK_CUSTOM_FIELDS_PUSHHOVER_HELP =
  'Need Pushover? Provide your Application Token and User Key here.';
export const ALERT_WEBHOOK_HEADERS_HELP_LABEL = 'Add authentication tokens or custom headers';
export const ALERT_WEBHOOK_HEADER_KEY_PLACEHOLDER = 'Header name';
export const ALERT_WEBHOOK_HEADER_VALUE_PLACEHOLDER = 'Header value';
export const ALERT_WEBHOOK_HEADER_REMOVE_LABEL = 'Remove';
export const ALERT_WEBHOOK_HEADER_ADD_LABEL = '+ Add header';
export const ALERT_WEBHOOK_HEADER_HELP =
  'Common headers: Authorization (Bearer token), X-API-Key, X-Auth-Token';
export const ALERT_WEBHOOK_ENABLE_LABEL = 'Enable this webhook';
export const ALERT_WEBHOOK_CANCEL_LABEL = 'Cancel';
export const ALERT_WEBHOOK_TEST_LABEL = 'Test';
export const ALERT_WEBHOOK_TESTING_LABEL = 'Testing…';
export const ALERT_WEBHOOK_TESTING_ASCII_LABEL = 'Testing...';
export const ALERT_WEBHOOK_TEST_SUCCESS = 'Test webhook sent successfully!';
export const ALERT_WEBHOOK_TEST_FAILURE = 'Failed to send test webhook';
export const ALERT_WEBHOOKS_SECTION_TITLE = 'Webhooks';
export const ALERT_WEBHOOKS_SECTION_DESCRIPTION = 'Push alerts to chat apps or automation systems.';
export const ALERT_WEBHOOK_ADD_SUCCESS = 'Webhook added successfully';
export const ALERT_WEBHOOK_ADD_FAILURE = 'Failed to add webhook';
export const ALERT_WEBHOOK_UPDATE_SUCCESS = 'Webhook updated successfully';
export const ALERT_WEBHOOK_UPDATE_FAILURE = 'Failed to update webhook';
export const ALERT_WEBHOOK_DELETE_SUCCESS = 'Webhook deleted successfully';
export const ALERT_WEBHOOK_DELETE_FAILURE = 'Failed to delete webhook';
export const ALERT_WEBHOOK_ADD_LABEL = 'Add Webhook';
export const ALERT_WEBHOOK_UPDATE_LABEL = 'Update Webhook';
export const ALERT_WEBHOOK_EDIT_LABEL = 'Edit';
export const ALERT_WEBHOOK_DELETE_LABEL = 'Delete';
export const ALERT_WEBHOOK_ENABLED_LABEL = 'Enabled';
export const ALERT_WEBHOOK_DISABLED_LABEL = 'Disabled';
export const ALERT_WEBHOOK_DISABLE_ALL_LABEL = 'Disable All';
export const ALERT_WEBHOOK_ENABLE_ALL_LABEL = 'Enable All';

export function getAlertWebhookServices(templates: AlertWebhookTemplate[] = []) {
  return templates.map((template) => ({
    id: template.service,
    label: template.label || getAlertWebhookServiceLabel(template.service),
    description: template.description || template.name,
  }));
}

export function getAlertWebhookServiceLabel(service: string) {
  return ALERT_WEBHOOK_SERVICE_PRESENTATION[service as AlertWebhookService]?.label || service;
}

export function getAlertWebhookServiceLabelFromTemplates(
  service: string,
  templates: AlertWebhookTemplate[] = [],
) {
  const template = templates.find((item) => item.service === service);
  if (template?.label?.trim()) {
    return template.label.trim();
  }
  return getAlertWebhookServiceLabel(service);
}

export function getAlertWebhookMentionPlaceholderFromTemplates(
  service: string,
  templates: AlertWebhookTemplate[] = [],
) {
  const template = templates.find((item) => item.service === service);
  return template?.mentionPlaceholder?.trim() || ALERT_WEBHOOK_MENTION_FALLBACK_PLACEHOLDER;
}

export function getAlertWebhookMentionHelpFromTemplates(
  service: string,
  templates: AlertWebhookTemplate[] = [],
) {
  const template = templates.find((item) => item.service === service);
  return template?.mentionHelp?.trim() || '';
}

export function hasAlertWebhookMentionSupportFromTemplates(
  service: string,
  templates: AlertWebhookTemplate[] = [],
) {
  const template = templates.find((item) => item.service === service);
  return Boolean(template?.mentionPlaceholder?.trim() || template?.mentionHelp?.trim());
}

export function getAlertWebhookCustomFieldPresets(service: string) {
  return ALERT_WEBHOOK_CUSTOM_FIELD_PRESETS[service.trim().toLowerCase()];
}

export function normalizeAlertWebhookCustomFields(
  service: string,
  fields: Record<string, string> = {},
) {
  if (service.trim().toLowerCase() !== 'pushover') {
    return { ...fields };
  }

  const normalized = { ...fields };
  if (!normalized.token?.trim() && normalized.app_token?.trim()) {
    normalized.token = normalized.app_token;
  }
  if (!normalized.user?.trim() && normalized.user_token?.trim()) {
    normalized.user = normalized.user_token;
  }
  delete normalized.app_token;
  delete normalized.user_token;
  return normalized;
}

export function getAlertWebhookSetupInstructionsTitle() {
  return ALERT_WEBHOOK_SETUP_INSTRUCTIONS_TITLE;
}

export function getAlertWebhookNamePlaceholder(templateName?: string) {
  return templateName || ALERT_WEBHOOK_NAME_PLACEHOLDER;
}

export function getAlertWebhookUrlPlaceholder(urlPattern?: string) {
  return urlPattern || ALERT_WEBHOOK_URL_PLACEHOLDER;
}

export function getAlertWebhookMentionPlaceholder(service: string) {
  return (
    ALERT_WEBHOOK_SERVICE_PRESENTATION[service as AlertWebhookService]?.mentionPlaceholder ||
    ALERT_WEBHOOK_MENTION_FALLBACK_PLACEHOLDER
  );
}

export function getAlertWebhookMentionHelp(service: string) {
  return ALERT_WEBHOOK_SERVICE_PRESENTATION[service as AlertWebhookService]?.mentionHelp || '';
}

export function getAlertWebhookSummaryLabel(enabledCount: number, totalCount: number) {
  return `${enabledCount} of ${totalCount} webhooks enabled`;
}

export function getAlertWebhookToggleAllLabel(enabled: boolean) {
  return enabled ? ALERT_WEBHOOK_ENABLE_ALL_LABEL : ALERT_WEBHOOK_DISABLE_ALL_LABEL;
}

export function getAlertWebhookToggleLabel(enabled: boolean) {
  return enabled ? ALERT_WEBHOOK_ENABLED_LABEL : ALERT_WEBHOOK_DISABLED_LABEL;
}

export function getAlertWebhookTestLabel(isTesting: boolean, ascii = false) {
  if (!isTesting) {
    return ALERT_WEBHOOK_TEST_LABEL;
  }
  return ascii ? ALERT_WEBHOOK_TESTING_ASCII_LABEL : ALERT_WEBHOOK_TESTING_LABEL;
}

export function getAlertWebhookSubmitLabel(isEditing: boolean) {
  return isEditing ? ALERT_WEBHOOK_UPDATE_LABEL : ALERT_WEBHOOK_ADD_LABEL;
}

export function getAlertWebhookTestSuccess() {
  return ALERT_WEBHOOK_TEST_SUCCESS;
}

export function getAlertWebhookTestFailure() {
  return ALERT_WEBHOOK_TEST_FAILURE;
}

export function getAlertWebhooksSectionTitle() {
  return ALERT_WEBHOOKS_SECTION_TITLE;
}

export function getAlertWebhooksSectionDescription() {
  return ALERT_WEBHOOKS_SECTION_DESCRIPTION;
}

export function getAlertWebhookMutationSuccess(action: 'add' | 'update' | 'delete') {
  switch (action) {
    case 'add':
      return ALERT_WEBHOOK_ADD_SUCCESS;
    case 'update':
      return ALERT_WEBHOOK_UPDATE_SUCCESS;
    default:
      return ALERT_WEBHOOK_DELETE_SUCCESS;
  }
}

export function getAlertWebhookMutationFailure(action: 'add' | 'update' | 'delete') {
  switch (action) {
    case 'add':
      return ALERT_WEBHOOK_ADD_FAILURE;
    case 'update':
      return ALERT_WEBHOOK_UPDATE_FAILURE;
    default:
      return ALERT_WEBHOOK_DELETE_FAILURE;
  }
}
