import { describe, expect, it } from 'vitest';
import {
  AI_CHAT_ASSISTANT_MESSAGE_LABEL,
  AI_CHAT_CONTEXT_USED_LABEL,
  AI_CHAT_DISCOVERY_HINT_BODY,
  AI_CHAT_DISCOVERY_HINT_TITLE,
  AI_CHAT_DRAWER_SUBTITLE,
  AI_CHAT_DRAWER_TITLE,
  AI_CHAT_EMPTY_STATE_SUBTITLE,
  AI_CHAT_EMPTY_STATE_TITLE,
  AI_CHAT_LAUNCHER_ARIA_LABEL,
  AI_CHAT_MODEL_SELECTOR_EMPTY_STATE,
  AI_CHAT_NEW_SESSION_MENU_LABEL,
  AI_CHAT_NEW_SESSION_SHORT_LABEL,
  AI_CHAT_QUESTION_CARD_TITLE,
  AI_CHAT_SESSION_EMPTY_STATE,
  AI_CHAT_SESSION_MENU_TITLE,
  AI_CHAT_SUGGESTIONS_LABEL,
  getAIChatLauncherTitle,
  getAIChatEmptyStateSuggestions,
  getAIChatEmptyStatePresentation,
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
    expect(AI_CHAT_EMPTY_STATE_TITLE).toBe('Ask about your infrastructure');
    expect(AI_CHAT_EMPTY_STATE_SUBTITLE).toBe(
      'Chat with your configured model using Pulse context and governed tools.',
    );
    expect(AI_CHAT_NEW_SESSION_SHORT_LABEL).toBe('New');
    expect(AI_CHAT_NEW_SESSION_MENU_LABEL).toBe('New session');
    expect(AI_CHAT_LAUNCHER_ARIA_LABEL).toBe('Expand Pulse Assistant');
    expect(AI_CHAT_SESSION_MENU_TITLE).toBe('Pulse Assistant sessions');
    expect(AI_CHAT_SESSION_EMPTY_STATE).toBe('No previous assistant sessions');
    expect(AI_CHAT_MODEL_SELECTOR_EMPTY_STATE).toBe('No matching models.');
    expect(AI_CHAT_SUGGESTIONS_LABEL).toBe('Try asking');
    expect(AI_CHAT_QUESTION_CARD_TITLE).toBe('Pulse Assistant needs your input');
    expect(AI_CHAT_ASSISTANT_MESSAGE_LABEL).toBe('Pulse Assistant');
    expect(AI_CHAT_CONTEXT_USED_LABEL).toBe('Context used');
  });

  it('builds canonical launcher titles without implying a keyboard shortcut', () => {
    expect(getAIChatLauncherTitle()).toBe('Open Pulse Assistant');
    expect(getAIChatLauncherTitle('Core Fabric')).toBe('Open Pulse Assistant for Core Fabric');
    expect(getAIChatLauncherTitle()).not.toContain('⌘K');
  });

  it('keeps the default empty state as plain chat without prompt chips', () => {
    expect(getAIChatEmptyStateSuggestions(true)).toEqual([]);
    expect(getAIChatEmptyStateSuggestions(false)).toEqual([]);
  });

  it('uses attached briefing context for scoped assistant handoff empty states', () => {
    expect(
      getAIChatEmptyStatePresentation({
        isCluster: true,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol assessment attached',
          subject: 'Coverage incomplete',
          suggestedPrompts: [
            'Explain why coverage is incomplete',
            'Explain scoped activity and full-run gap',
          ],
        },
      }),
    ).toEqual({
      title: 'Review Pulse Patrol context',
      subtitle: 'Patrol assessment attached · Coverage incomplete',
      suggestions: [],
    });
  });
});
