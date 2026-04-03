import { describe, expect, it } from 'vitest';
import {
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
  getAISettingsToggleErrorMessage,
} from '@/utils/aiSettingsPresentation';

describe('aiSettingsPresentation', () => {
  it('returns the canonical provider-backed ready presentation', () => {
    expect(
      getAISettingsReadinessPresentation({
        configured: true,
        providerCount: 2,
        modelCount: 5,
        quickstartCreditsAvailable: false,
        quickstartCreditsRemaining: 0,
        quickstartCreditsTotal: 0,
        quickstartBlockedReason: '',
      }),
    ).toEqual({
      containerClassName: 'bg-green-50 dark:bg-green-900 text-green-800 dark:text-green-200',
      dotClassName: 'bg-emerald-400',
      summary: 'Ready • 2 providers • 5 models',
    });
  });

  it('returns the canonical quickstart-ready presentation', () => {
    expect(
      getAISettingsReadinessPresentation({
        configured: false,
        providerCount: 0,
        modelCount: 0,
        quickstartCreditsAvailable: true,
        quickstartCreditsRemaining: 25,
        quickstartCreditsTotal: 25,
        quickstartBlockedReason: '',
      }),
    ).toEqual({
      containerClassName: 'bg-blue-50 dark:bg-blue-900 text-blue-800 dark:text-blue-200',
      dotClassName: 'bg-blue-400',
      summary: 'Patrol quickstart ready • 25/25 runs left • no API key needed yet',
    });
  });

  it('returns the canonical activation-required presentation', () => {
    expect(
      getAISettingsReadinessPresentation({
        configured: false,
        providerCount: 0,
        modelCount: 0,
        quickstartCreditsAvailable: false,
        quickstartCreditsRemaining: 0,
        quickstartCreditsTotal: 0,
        quickstartBlockedReason:
          'Activate this install or start a trial to use AI Patrol quickstart. Otherwise connect your API key.',
      }),
    ).toEqual({
      containerClassName: 'bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200',
      dotClassName: 'bg-amber-400',
      summary:
        'Activate this install or start a trial to use AI Patrol quickstart. Otherwise connect your API key.',
    });
  });

  it('returns the canonical not-configured presentation', () => {
    expect(
      getAISettingsReadinessPresentation({
        configured: false,
        providerCount: 0,
        modelCount: 0,
        quickstartCreditsAvailable: false,
        quickstartCreditsRemaining: 0,
        quickstartCreditsTotal: 0,
        quickstartBlockedReason: '',
      }),
    ).toEqual({
      containerClassName: 'bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200',
      dotClassName: 'bg-amber-400',
      summary: 'Configure at least one provider above to enable Pulse Assistant features',
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
      text: 'Loading Pulse Assistant settings...',
    });
    expect(getAISettingsLoadErrorMessage()).toBe(
      'Unable to load Pulse Assistant settings. Your configuration could not be retrieved.',
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
    expect(getAISettingsSaveErrorMessage()).toBe('Unable to save Pulse Assistant settings.');
    expect(getAISettingsSaveErrorMessage('bad request')).toBe('bad request');
    expect(getAICredentialsClearErrorMessage()).toBe('Unable to clear credentials.');
    expect(getAICredentialsClearErrorMessage('permission denied')).toBe('permission denied');
    expect(getAISettingsToggleErrorMessage()).toBe('Unable to update Pulse Assistant settings.');
    expect(getAISettingsToggleErrorMessage('rate limited')).toBe('rate limited');
  });
});
