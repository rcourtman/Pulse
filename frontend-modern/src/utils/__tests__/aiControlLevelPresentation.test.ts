import { describe, expect, it } from 'vitest';
import {
  AI_CONTROL_LEVEL_ASK_FIRST_BADGE_CLASS,
  AI_CONTROL_LEVEL_PANEL_CLASS,
  getAIChatControlLevelPresentation,
  getAIControlLevelDescription,
  normalizeAIControlLevel,
} from '@/utils/aiControlLevelPresentation';

describe('aiControlLevelPresentation', () => {
  it('normalizes retired and unknown control levels', () => {
    expect(normalizeAIControlLevel('read_only')).toBe('read_only');
    expect(normalizeAIControlLevel('controlled')).toBe('controlled');
    expect(normalizeAIControlLevel('autonomous')).toBe('controlled');
    expect(normalizeAIControlLevel('suggest')).toBe('controlled');
    expect(normalizeAIControlLevel('unexpected')).toBe('read_only');
    expect(normalizeAIControlLevel(undefined)).toBe('read_only');
  });

  it('returns canonical panel, badge, and description presentation', () => {
    expect(AI_CONTROL_LEVEL_PANEL_CLASS).toContain('border-blue-200');
    expect(AI_CONTROL_LEVEL_ASK_FIRST_BADGE_CLASS).toContain('bg-amber-100');
    expect(getAIControlLevelDescription('read_only')).toBe(
      'Assistant answers questions and cannot plan actions.',
    );
    expect(getAIControlLevelDescription('controlled')).toContain('saves each plan to Actions');
    expect(getAIControlLevelDescription('controlled')).not.toContain('chat-only');
  });

  it('returns canonical chat control-level presentation', () => {
    expect(getAIChatControlLevelPresentation('read_only')).toMatchObject({
      label: 'Read-only',
      description: 'Answers questions only',
      dotClassName: 'bg-slate-400',
    });
    expect(getAIChatControlLevelPresentation('controlled')).toMatchObject({
      label: 'Ask first',
      description: 'Plans actions for your review',
      dotClassName: 'bg-amber-500',
    });
  });
});
