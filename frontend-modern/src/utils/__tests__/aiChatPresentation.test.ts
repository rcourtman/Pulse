import { describe, expect, it } from 'vitest';
import {
  AI_CHAT_MODEL_SELECTOR_EMPTY_STATE,
  AI_CHAT_SESSION_EMPTY_STATE,
} from '@/utils/aiChatPresentation';

describe('aiChatPresentation', () => {
  it('exports canonical AI chat empty-state copy', () => {
    expect(AI_CHAT_SESSION_EMPTY_STATE).toBe('No previous conversations');
    expect(AI_CHAT_MODEL_SELECTOR_EMPTY_STATE).toBe('No matching models.');
  });
});
