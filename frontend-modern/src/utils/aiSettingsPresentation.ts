export interface AISettingsReadinessPresentation {
  containerClassName: string;
  dotClassName: string;
  summary: string;
}

export interface AISettingsReadinessInput {
  configured: boolean;
  providerCount: number;
  modelCount: number;
}

const AI_OAUTH_ERROR_MESSAGES: Record<string, string> = {
  missing_params: 'The authentication callback is missing required parameters.',
  invalid_state: 'The authentication session is no longer valid. Try again.',
  token_exchange_failed: 'Unable to complete authentication with Claude.',
  save_failed: 'Unable to save OAuth credentials.',
};

export const AI_SETTINGS_PANEL_TITLE = 'Assistant & Patrol';
export const AI_SETTINGS_PANEL_DESCRIPTION =
  'Configure providers and models for Pulse Assistant and Patrol.';
export const AI_SETTINGS_MODEL_OVERRIDES_TITLE = 'Model Overrides';
export const AI_SETTINGS_ASSISTANT_SESSIONS_TITLE = 'Pulse Assistant Sessions';
export const AI_SETTINGS_ASSISTANT_PERMISSIONS_TITLE = 'Pulse Assistant Permissions';
export const AI_SETTINGS_LOAD_MODELS_ERROR = 'Unable to load models.';
export const AI_SETTINGS_LOAD_CHAT_SESSIONS_ERROR = 'Unable to load chat sessions.';
export const AI_SETTINGS_LOAD_FAILURE_MESSAGE =
  'Unable to load Assistant & Patrol settings. Your configuration could not be retrieved.';
export const AI_SETTINGS_LOAD_RETRY_LABEL = 'Retry';

export interface AISettingsSetupDialogPresentation {
  ariaLabel: string;
  description: string;
  submitLabel: string;
  title: string;
}

export function getAIProviderTestResultTextClass(success: boolean): string {
  return success ? 'text-green-600' : 'text-red-600';
}

export function getAISettingsWorkloadDiscoveryHelpContent() {
  return {
    title: 'What is workload discovery?',
    description:
      'Workload discovery scans your VMs, containers, and container runtimes to identify running services, versions, and access details. Pulse stores that context so Assistant can use it in chat and Patrol can use it during verification.',
  } as const;
}

export function getAISettingsWorkloadDiscoverySummary() {
  return {
    text: 'Workload discovery stores concrete service context for Assistant chat and Patrol verification, so responses and findings can reference real services and commands instead of generic advice.',
  } as const;
}

export function getAISettingsSetupDialogPresentation(): AISettingsSetupDialogPresentation {
  return {
    ariaLabel: 'Set up Assistant and Patrol',
    title: 'Set Up Assistant & Patrol',
    description: 'Connect a provider to power Pulse Assistant and Patrol.',
    submitLabel: 'Enable Assistant & Patrol',
  };
}

export function getAISettingsReadinessPresentation(
  input: AISettingsReadinessInput,
): AISettingsReadinessPresentation {
  const { configured, providerCount, modelCount } = input;

  if (configured) {
    return {
      containerClassName: 'bg-green-50 dark:bg-green-900 text-green-800 dark:text-green-200',
      dotClassName: 'bg-emerald-400',
      summary: `Ready • ${providerCount} provider${providerCount !== 1 ? 's' : ''} • ${modelCount} models`,
    };
  }

  return {
    containerClassName: 'bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200',
    dotClassName: 'bg-amber-400',
    summary: 'Configure at least one provider above to enable Pulse Assistant and Patrol.',
  };
}

export function getAIOAuthErrorMessage(errorCode: string): string {
  return AI_OAUTH_ERROR_MESSAGES[errorCode] || `Authentication error: ${errorCode}`;
}

export function getAISettingsLoadingState() {
  return {
    text: 'Loading Assistant & Patrol settings...',
  } as const;
}

export function getAISettingsLoadErrorMessage() {
  return AI_SETTINGS_LOAD_FAILURE_MESSAGE;
}

export function getAISettingsRetryLabel() {
  return AI_SETTINGS_LOAD_RETRY_LABEL;
}

export function getAIModelsLoadErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || AI_SETTINGS_LOAD_MODELS_ERROR;
}

export function getAIChatSessionsLoadErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || AI_SETTINGS_LOAD_CHAT_SESSIONS_ERROR;
}

export function getAIChatSessionsLoadingState() {
  return {
    text: 'Loading chat sessions...',
  } as const;
}

export function getAIChatSessionsEmptyState() {
  return {
    text: 'No chat sessions yet. Start a chat to create one.',
  } as const;
}

export function getAISessionSummarizeErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || 'Unable to summarize the session.';
}

export function getAISettingsSaveErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || 'Unable to save Assistant & Patrol settings.';
}

export function getAICredentialsClearErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || 'Unable to clear credentials.';
}

export function getAISettingsToggleErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || 'Unable to update Assistant & Patrol settings.';
}
