import { describe, expect, it } from 'vitest';
import {
  AI_CHAT_ASSISTANT_MESSAGE_LABEL,
  AI_CHAT_AUTONOMOUS_WARNING_DISMISS_LABEL,
  AI_CHAT_CONTEXT_USED_LABEL,
  AI_CHAT_CLOSE_LABEL,
  AI_CHAT_CONTROL_MODE_LABEL,
  AI_CHAT_CONTROL_MODE_MENU_LABEL,
  AI_CHAT_COPY_LAST_ANSWER_ERROR_MESSAGE,
  AI_CHAT_COPY_LAST_ANSWER_LABEL,
  AI_CHAT_COPY_LAST_ANSWER_SUCCESS_MESSAGE,
  AI_CHAT_DISCOVERY_HINT_BODY,
  AI_CHAT_DISCOVERY_HINT_DISMISS_LABEL,
  AI_CHAT_DISCOVERY_HINT_TITLE,
  AI_CHAT_DRAWER_SUBTITLE,
  AI_CHAT_DRAWER_TITLE,
  AI_CHAT_LAUNCHER_ARIA_LABEL,
  AI_CHAT_LAST_TURN_USAGE_LABEL,
  AI_CHAT_MODEL_SELECTOR_EMPTY_STATE,
  AI_CHAT_NEW_SESSION_BUTTON_TITLE,
  AI_CHAT_NEW_SESSION_MENU_ARIA_LABEL,
  AI_CHAT_NEW_SESSION_MENU_LABEL,
  AI_CHAT_NEW_SESSION_SHORT_LABEL,
  AI_CHAT_PROVIDER_READINESS_RETRY_LABEL,
  AI_CHAT_PROVIDER_READINESS_SETTINGS_HREF,
  AI_CHAT_PROVIDER_READINESS_SETTINGS_LABEL,
  AI_CHAT_QUESTION_CARD_TITLE,
  AI_CHAT_RENAME_SESSION_CANCEL_LABEL,
  AI_CHAT_RENAME_SESSION_EMPTY_MESSAGE,
  AI_CHAT_RENAME_SESSION_ERROR_MESSAGE,
  AI_CHAT_RENAME_SESSION_LABEL,
  AI_CHAT_RENAME_SESSION_SAVE_LABEL,
  AI_CHAT_REDO_LAST_TURN_EMPTY_MESSAGE,
  AI_CHAT_REDO_LAST_TURN_ERROR_MESSAGE,
  AI_CHAT_REDO_LAST_TURN_LABEL,
  AI_CHAT_REDO_LAST_TURN_LOADING_MESSAGE,
  AI_CHAT_REDO_LAST_TURN_SUCCESS_MESSAGE,
  AI_CHAT_SESSION_EMPTY_STATE,
  AI_CHAT_SESSION_LOADING_STATE,
  AI_CHAT_SESSION_MENU_TITLE,
  AI_CHAT_SESSION_SEARCH_EMPTY_STATE,
  AI_CHAT_SESSION_SEARCH_ERROR_STATE,
  AI_CHAT_SESSION_SEARCH_LOADING_STATE,
  AI_CHAT_SESSION_SEARCH_PLACEHOLDER,
  AI_CHAT_SESSION_SEARCH_TITLE,
  AI_CHAT_SWITCH_TO_APPROVAL_LABEL,
  AI_CHAT_UNDO_LAST_TURN_EMPTY_MESSAGE,
  AI_CHAT_UNDO_LAST_TURN_ERROR_MESSAGE,
  AI_CHAT_UNDO_LAST_TURN_LABEL,
  AI_CHAT_UNDO_LAST_TURN_LOADING_MESSAGE,
  AI_CHAT_UNDO_LAST_TURN_SUCCESS_MESSAGE,
  getAIChatProviderReadinessPresentation,
  getAIChatLauncherTitle,
} from '@/utils/aiChatPresentation';

describe('aiChatPresentation', () => {
  it('exports canonical assistant drawer presentation copy', () => {
    expect(AI_CHAT_DRAWER_TITLE).toBe('Pulse Assistant');
    expect(AI_CHAT_DRAWER_SUBTITLE).toBe(
      'Observed context, provider-backed reasoning, and governed actions.',
    );
    expect(AI_CHAT_DISCOVERY_HINT_TITLE).toBe('Workload Discovery is off.');
    expect(AI_CHAT_DISCOVERY_HINT_BODY).toBe(
      'Enable it in Settings so Pulse Assistant can reference real services, versions, and commands instead of generic guidance.',
    );
    expect(AI_CHAT_NEW_SESSION_SHORT_LABEL).toBe('New');
    expect(AI_CHAT_NEW_SESSION_BUTTON_TITLE).toBe('Start new Assistant session');
    expect(AI_CHAT_NEW_SESSION_MENU_LABEL).toBe('New session');
    expect(AI_CHAT_NEW_SESSION_MENU_ARIA_LABEL).toBe('Start new Assistant session');
    expect(AI_CHAT_COPY_LAST_ANSWER_LABEL).toBe('Copy last Assistant answer');
    expect(AI_CHAT_COPY_LAST_ANSWER_SUCCESS_MESSAGE).toBe('Assistant answer copied');
    expect(AI_CHAT_COPY_LAST_ANSWER_ERROR_MESSAGE).toBe('Failed to copy Assistant answer');
    expect(AI_CHAT_RENAME_SESSION_LABEL).toBe('Rename Assistant session');
    expect(AI_CHAT_RENAME_SESSION_SAVE_LABEL).toBe('Save Assistant session title');
    expect(AI_CHAT_RENAME_SESSION_CANCEL_LABEL).toBe('Cancel Assistant session rename');
    expect(AI_CHAT_RENAME_SESSION_EMPTY_MESSAGE).toBe('Session title cannot be empty');
    expect(AI_CHAT_RENAME_SESSION_ERROR_MESSAGE).toBe('Failed to rename Assistant session');
    expect(AI_CHAT_UNDO_LAST_TURN_LABEL).toBe('Undo last Assistant turn');
    expect(AI_CHAT_UNDO_LAST_TURN_EMPTY_MESSAGE).toBe('No Assistant turn to undo');
    expect(AI_CHAT_UNDO_LAST_TURN_LOADING_MESSAGE).toBe('Assistant is still working');
    expect(AI_CHAT_UNDO_LAST_TURN_ERROR_MESSAGE).toBe('Failed to undo Assistant turn');
    expect(AI_CHAT_UNDO_LAST_TURN_SUCCESS_MESSAGE).toBe('Last prompt restored for editing');
    expect(AI_CHAT_REDO_LAST_TURN_LABEL).toBe('Redo last Assistant turn');
    expect(AI_CHAT_REDO_LAST_TURN_EMPTY_MESSAGE).toBe('No undone Assistant turn to redo');
    expect(AI_CHAT_REDO_LAST_TURN_LOADING_MESSAGE).toBe('Assistant is still working');
    expect(AI_CHAT_REDO_LAST_TURN_ERROR_MESSAGE).toBe('Failed to redo Assistant turn');
    expect(AI_CHAT_REDO_LAST_TURN_SUCCESS_MESSAGE).toBe('Assistant turn restored');
    expect(AI_CHAT_LAUNCHER_ARIA_LABEL).toBe('Expand Pulse Assistant');
    expect(AI_CHAT_CLOSE_LABEL).toBe('Close Pulse Assistant');
    expect(AI_CHAT_SESSION_MENU_TITLE).toBe('Pulse Assistant sessions');
    expect(AI_CHAT_AUTONOMOUS_WARNING_DISMISS_LABEL).toBe('Dismiss autonomous control warning');
    expect(AI_CHAT_DISCOVERY_HINT_DISMISS_LABEL).toBe('Dismiss discovery context warning');
    expect(AI_CHAT_CONTROL_MODE_LABEL).toBe('Assistant control mode');
    expect(AI_CHAT_CONTROL_MODE_MENU_LABEL).toBe('Assistant control mode options');
    expect(AI_CHAT_SWITCH_TO_APPROVAL_LABEL).toBe('Switch Assistant control mode to Approval');
    expect(AI_CHAT_SESSION_EMPTY_STATE).toBe('No previous assistant sessions');
    expect(AI_CHAT_SESSION_LOADING_STATE).toBe('Loading assistant sessions...');
    expect(AI_CHAT_SESSION_SEARCH_PLACEHOLDER).toBe('Search sessions...');
    expect(AI_CHAT_SESSION_SEARCH_TITLE).toBe('Search Assistant sessions');
    expect(AI_CHAT_SESSION_SEARCH_EMPTY_STATE).toBe('No sessions match your search');
    expect(AI_CHAT_SESSION_SEARCH_LOADING_STATE).toBe('Searching assistant sessions...');
    expect(AI_CHAT_SESSION_SEARCH_ERROR_STATE).toBe('Failed to search assistant sessions');
    expect(AI_CHAT_MODEL_SELECTOR_EMPTY_STATE).toBe('No matching models.');
    expect(AI_CHAT_QUESTION_CARD_TITLE).toBe('Pulse Assistant needs your input');
    expect(AI_CHAT_ASSISTANT_MESSAGE_LABEL).toBe('Pulse Assistant');
    expect(AI_CHAT_CONTEXT_USED_LABEL).toBe('Context used');
    expect(AI_CHAT_LAST_TURN_USAGE_LABEL).toBe('Last assistant turn usage');
    expect(AI_CHAT_PROVIDER_READINESS_SETTINGS_HREF).toBe('/settings/system-ai');
    expect(AI_CHAT_PROVIDER_READINESS_SETTINGS_LABEL).toBe('Open settings');
    expect(AI_CHAT_PROVIDER_READINESS_RETRY_LABEL).toBe('Retry');
  });

  it('builds canonical launcher titles without implying a keyboard shortcut', () => {
    expect(getAIChatLauncherTitle()).toBe('Open Pulse Assistant');
    expect(getAIChatLauncherTitle('Core Fabric')).toBe('Open Pulse Assistant for Core Fabric');
    expect(getAIChatLauncherTitle()).not.toContain('⌘K');
  });

  it('builds neutral provider readiness presentation copy for Assistant checks', () => {
    expect(
      getAIChatProviderReadinessPresentation({
        status: 'error',
        providerLabel: 'DeepSeek',
        summary: 'Pulse could not maintain a healthy connection to this provider.',
        recommendation: 'Check provider reachability.',
      }),
    ).toEqual({
      tone: 'error',
      title: 'DeepSeek provider issue',
      body: 'Pulse could not maintain a healthy connection to this provider.',
      recommendation: 'Check provider reachability.',
    });

    expect(
      getAIChatProviderReadinessPresentation({
        status: 'checking',
        providerLabel: 'OpenRouter',
      }).title,
    ).toBe('Verifying OpenRouter provider');

    expect(
      getAIChatProviderReadinessPresentation({
        status: 'ready',
        providerLabel: 'OpenRouter',
        message: 'Connection successful',
      }),
    ).toEqual({
      tone: 'ready',
      title: 'OpenRouter provider ready',
      body: 'Connection successful',
    });
  });
});
