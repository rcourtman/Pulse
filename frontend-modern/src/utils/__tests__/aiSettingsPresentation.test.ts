import { describe, expect, it } from 'vitest';
import {
  AI_SETTINGS_ASSISTANT_PERMISSIONS_TITLE,
  AI_SETTINGS_ASSISTANT_SESSIONS_TITLE,
  AI_SETTINGS_MODEL_OVERRIDES_TITLE,
  AI_SETTINGS_PANEL_DESCRIPTION,
  AI_SETTINGS_PANEL_TITLE,
  getAICredentialsClearErrorMessage,
  getAIOAuthErrorMessage,
  getAIChatSessionsEmptyState,
  getAIChatSessionsLoadErrorMessage,
  getAIChatSessionsLoadingState,
  getAIModelsLoadErrorMessage,
  getAIProviderTestResultTextClass,
  getAISessionDiffErrorMessage,
  getAISessionRevertErrorMessage,
  getAISessionSummarizeErrorMessage,
  getAISettingsLoadErrorMessage,
  getAISettingsLoadingState,
  getAISettingsReadinessPresentation,
  getAISettingsRetryLabel,
  getAISettingsSaveErrorMessage,
  getAISettingsSetupDialogPresentation,
  getAISettingsToggleErrorMessage,
  getAISettingsWorkloadDiscoveryHelpContent,
  getAISettingsWorkloadDiscoverySummary,
} from '@/utils/aiSettingsPresentation';

describe('aiSettingsPresentation', () => {
  it('returns the canonical assistant and patrol shell framing copy', () => {
    expect(AI_SETTINGS_PANEL_TITLE).toBe('Assistant & Patrol');
    expect(AI_SETTINGS_PANEL_DESCRIPTION).toBe(
      'Configure providers and models for Pulse Assistant and Patrol.',
    );
    expect(AI_SETTINGS_MODEL_OVERRIDES_TITLE).toBe('Assistant & Patrol Model Overrides');
    expect(AI_SETTINGS_ASSISTANT_SESSIONS_TITLE).toBe('Pulse Assistant Sessions');
    expect(AI_SETTINGS_ASSISTANT_PERMISSIONS_TITLE).toBe('Pulse Assistant Permissions');
    expect(getAISettingsWorkloadDiscoveryHelpContent()).toEqual({
      title: 'What is workload discovery?',
      description:
        'Workload discovery scans your VMs, containers, and container runtimes to identify running services, versions, and access details. Pulse stores that context so Assistant can use it in chat and Patrol can use it during verification.',
    });
    expect(getAISettingsWorkloadDiscoverySummary()).toEqual({
      text: 'Workload discovery stores concrete service context for Assistant chat and Patrol verification, so responses and findings can reference real services and commands instead of generic advice.',
    });
    expect(getAISettingsSetupDialogPresentation()).toEqual({
      ariaLabel: 'Set up Assistant and Patrol',
      title: 'Set Up Assistant & Patrol',
      description: 'Connect a provider to power Pulse Assistant and Patrol.',
      submitLabel: 'Enable Assistant & Patrol',
    });
  });

  it('returns the canonical provider-backed ready presentation', () => {
    expect(
      getAISettingsReadinessPresentation({
        configured: true,
        providerCount: 2,
        modelCount: 5,
      }),
    ).toEqual({
      containerClassName: 'bg-green-50 dark:bg-green-900 text-green-800 dark:text-green-200',
      dotClassName: 'bg-emerald-400',
      summary: 'Ready • 2 providers • 5 models',
    });
  });

  it('returns the canonical not-configured presentation', () => {
    expect(
      getAISettingsReadinessPresentation({
        configured: false,
        providerCount: 0,
        modelCount: 0,
      }),
    ).toEqual({
      containerClassName: 'bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200',
      dotClassName: 'bg-amber-400',
      summary: 'Configure at least one provider above to enable Pulse Assistant and Patrol.',
    });
  });

  it('returns canonical provider test result text classes', () => {
    expect(getAIProviderTestResultTextClass(true)).toBe('text-green-600');
    expect(getAIProviderTestResultTextClass(false)).toBe('text-red-600');
  });

  it('returns canonical OAuth callback error messages', () => {
    expect(getAIOAuthErrorMessage('missing_params')).toBe(
      'The authentication callback is missing required parameters.',
    );
    expect(getAIOAuthErrorMessage('invalid_state')).toBe(
      'The authentication session is no longer valid. Try again.',
    );
    expect(getAIOAuthErrorMessage('token_exchange_failed')).toBe(
      'Unable to complete authentication with Claude.',
    );
    expect(getAIOAuthErrorMessage('save_failed')).toBe('Unable to save OAuth credentials.');
    expect(getAIOAuthErrorMessage('other')).toBe('Authentication error: other');
  });

  it('returns canonical ai settings loading and chat-session copy', () => {
    expect(getAISettingsLoadingState()).toEqual({
      text: 'Loading Assistant & Patrol settings...',
    });
    expect(getAISettingsLoadErrorMessage()).toBe(
      'Unable to load Assistant & Patrol settings. Your configuration could not be retrieved.',
    );
    expect(getAISettingsRetryLabel()).toBe('Retry');
    expect(getAIChatSessionsLoadingState()).toEqual({
      text: 'Loading chat sessions...',
    });
    expect(getAIChatSessionsEmptyState()).toEqual({
      text: 'No chat sessions yet. Start a chat to create one.',
    });
    expect(getAIModelsLoadErrorMessage()).toBe('Unable to load models.');
    expect(getAIModelsLoadErrorMessage('Network request failed')).toBe('Network request failed');
    expect(getAIChatSessionsLoadErrorMessage()).toBe('Unable to load chat sessions.');
    expect(getAIChatSessionsLoadErrorMessage('Session API offline')).toBe('Session API offline');
  });

  it('returns canonical ai settings operational failure copy', () => {
    expect(getAISessionSummarizeErrorMessage()).toBe('Unable to summarize the session.');
    expect(getAISessionSummarizeErrorMessage('provider offline')).toBe('provider offline');
    expect(getAISessionDiffErrorMessage()).toBe('Unable to load the session diff.');
    expect(getAISessionDiffErrorMessage('git unavailable')).toBe('git unavailable');
    expect(getAISessionRevertErrorMessage()).toBe('Unable to revert the session.');
    expect(getAISessionRevertErrorMessage('conflict')).toBe('conflict');
    expect(getAISettingsSaveErrorMessage()).toBe('Unable to save Assistant & Patrol settings.');
    expect(getAISettingsSaveErrorMessage('bad request')).toBe('bad request');
    expect(getAICredentialsClearErrorMessage()).toBe('Unable to clear credentials.');
    expect(getAICredentialsClearErrorMessage('permission denied')).toBe('permission denied');
    expect(getAISettingsToggleErrorMessage()).toBe('Unable to update Assistant & Patrol settings.');
    expect(getAISettingsToggleErrorMessage('rate limited')).toBe('rate limited');
  });
});
