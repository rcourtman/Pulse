export interface AISettingsReadinessPresentation {
  containerClassName: string;
  dotClassName: string;
  summary: string;
}

const AI_OAUTH_ERROR_MESSAGES: Record<string, string> = {
  missing_params: 'OAuth callback missing required parameters',
  invalid_state: 'Invalid OAuth state - please try again',
  token_exchange_failed: 'Failed to complete authentication with Claude',
  save_failed: 'Failed to save OAuth credentials',
};

export const AI_SETTINGS_LOAD_MODELS_ERROR = 'Failed to load models';
export const AI_SETTINGS_LOAD_CHAT_SESSIONS_ERROR = 'Failed to load chat sessions.';
export const AI_SETTINGS_LOAD_FAILURE_MESSAGE =
  'Failed to load Pulse Assistant settings. Your configuration could not be retrieved.';
export const AI_SETTINGS_LOAD_RETRY_LABEL = 'Retry';

export function getAIProviderTestResultTextClass(success: boolean): string {
  return success ? 'text-green-600' : 'text-red-600';
}

export function getAISettingsReadinessPresentation(
  configured: boolean,
  providerCount: number,
  modelCount: number,
): AISettingsReadinessPresentation {
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
    summary: 'Configure at least one provider above to enable Pulse Assistant features',
  };
}

export function getAIOAuthErrorMessage(errorCode: string): string {
  return AI_OAUTH_ERROR_MESSAGES[errorCode] || `OAuth error: ${errorCode}`;
}

export function getAISettingsLoadingState() {
  return {
    text: 'Loading Pulse Assistant settings...',
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
  return detail || 'Failed to summarize session.';
}

export function getAISessionDiffErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || 'Failed to get session diff.';
}

export function getAISessionRevertErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || 'Failed to revert session.';
}

export function getAISettingsSaveErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || 'Failed to save Pulse Assistant settings';
}

export function getAICredentialsClearErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || 'Failed to clear credentials';
}

export function getAISettingsToggleErrorMessage(message?: string | null) {
  const detail = (message || '').trim();
  return detail || 'Failed to update Pulse Assistant setting';
}
