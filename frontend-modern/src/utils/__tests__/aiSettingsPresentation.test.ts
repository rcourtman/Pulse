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
  it('returns the canonical ready presentation', () => {
    expect(getAISettingsReadinessPresentation(true, 2, 5)).toEqual({
      containerClassName: 'bg-green-50 dark:bg-green-900 text-green-800 dark:text-green-200',
      dotClassName: 'bg-emerald-400',
      summary: 'Ready • 2 providers • 5 models',
    });
  });

  it('returns the canonical not-configured presentation', () => {
    expect(getAISettingsReadinessPresentation(false, 0, 0)).toEqual({
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
      'OAuth callback missing required parameters',
    );
    expect(getAIOAuthErrorMessage('invalid_state')).toBe(
      'Invalid OAuth state - please try again',
    );
    expect(getAIOAuthErrorMessage('token_exchange_failed')).toBe(
      'Failed to complete authentication with Claude',
    );
    expect(getAIOAuthErrorMessage('save_failed')).toBe('Failed to save OAuth credentials');
    expect(getAIOAuthErrorMessage('other')).toBe('OAuth error: other');
  });

  it('returns canonical ai settings loading and chat-session copy', () => {
    expect(getAISettingsLoadingState()).toEqual({
      text: 'Loading Pulse Assistant settings...',
    });
    expect(getAISettingsLoadErrorMessage()).toBe(
      'Failed to load Pulse Assistant settings. Your configuration could not be retrieved.',
    );
    expect(getAISettingsRetryLabel()).toBe('Retry');
    expect(getAIChatSessionsLoadingState()).toEqual({
      text: 'Loading chat sessions...',
    });
    expect(getAIChatSessionsEmptyState()).toEqual({
      text: 'No chat sessions yet. Start a chat to create one.',
    });
    expect(getAIModelsLoadErrorMessage()).toBe('Failed to load models');
    expect(getAIModelsLoadErrorMessage('Network request failed')).toBe('Network request failed');
    expect(getAIChatSessionsLoadErrorMessage()).toBe('Failed to load chat sessions.');
    expect(getAIChatSessionsLoadErrorMessage('Session API offline')).toBe('Session API offline');
  });

  it('returns canonical ai settings operational failure copy', () => {
    expect(getAISessionSummarizeErrorMessage()).toBe('Failed to summarize session.');
    expect(getAISessionSummarizeErrorMessage('provider offline')).toBe('provider offline');
    expect(getAISessionDiffErrorMessage()).toBe('Failed to get session diff.');
    expect(getAISessionDiffErrorMessage('git unavailable')).toBe('git unavailable');
    expect(getAISessionRevertErrorMessage()).toBe('Failed to revert session.');
    expect(getAISessionRevertErrorMessage('conflict')).toBe('conflict');
    expect(getAISettingsSaveErrorMessage()).toBe('Failed to save Pulse Assistant settings');
    expect(getAISettingsSaveErrorMessage('bad request')).toBe('bad request');
    expect(getAICredentialsClearErrorMessage()).toBe('Failed to clear credentials');
    expect(getAICredentialsClearErrorMessage('permission denied')).toBe('permission denied');
    expect(getAISettingsToggleErrorMessage()).toBe('Failed to update Pulse Assistant setting');
    expect(getAISettingsToggleErrorMessage('rate limited')).toBe('rate limited');
  });
});
