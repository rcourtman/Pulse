import { describe, expect, it } from 'vitest';
import {
  AI_CHAT_ASSISTANT_MESSAGE_LABEL,
  AI_CHAT_CONTEXT_USED_LABEL,
  AI_CHAT_DISCOVERY_HINT_BODY,
  AI_CHAT_DISCOVERY_HINT_TITLE,
  AI_CHAT_DRAWER_SUBTITLE,
  AI_CHAT_DRAWER_TITLE,
  AI_CHAT_LAUNCHER_ARIA_LABEL,
  AI_CHAT_LAST_TURN_USAGE_LABEL,
  AI_CHAT_MODEL_SELECTOR_EMPTY_STATE,
  AI_CHAT_NEW_SESSION_MENU_LABEL,
  AI_CHAT_NEW_SESSION_SHORT_LABEL,
  AI_CHAT_PROVIDER_READINESS_RETRY_LABEL,
  AI_CHAT_PROVIDER_READINESS_SETTINGS_HREF,
  AI_CHAT_PROVIDER_READINESS_SETTINGS_LABEL,
  AI_CHAT_QUESTION_CARD_TITLE,
  AI_CHAT_SESSION_EMPTY_STATE,
  AI_CHAT_SESSION_LOADING_STATE,
  AI_CHAT_SESSION_MENU_TITLE,
  AI_CHAT_SESSION_SEARCH_EMPTY_STATE,
  AI_CHAT_SESSION_SEARCH_ERROR_STATE,
  AI_CHAT_SESSION_SEARCH_LOADING_STATE,
  AI_CHAT_SESSION_SEARCH_PLACEHOLDER,
  AI_CHAT_SESSION_SEARCH_TITLE,
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
    expect(AI_CHAT_NEW_SESSION_MENU_LABEL).toBe('New session');
    expect(AI_CHAT_LAUNCHER_ARIA_LABEL).toBe('Expand Pulse Assistant');
    expect(AI_CHAT_SESSION_MENU_TITLE).toBe('Pulse Assistant sessions');
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
  });
});
