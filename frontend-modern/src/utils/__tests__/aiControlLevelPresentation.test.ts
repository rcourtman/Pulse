import { describe, expect, it } from 'vitest';
import {
  getAIControlLevelBadgeClass,
  getAIChatControlLevelPresentation,
  getAIControlLevelDescription,
  getAIControlLevelPanelClass,
  normalizeAIControlLevel,
} from '@/utils/aiControlLevelPresentation';

describe('aiControlLevelPresentation', () => {
  it('normalizes legacy and unknown control levels', () => {
    expect(normalizeAIControlLevel('read_only')).toBe('read_only');
    expect(normalizeAIControlLevel('controlled')).toBe('controlled');
    expect(normalizeAIControlLevel('autonomous')).toBe('controlled');
    expect(normalizeAIControlLevel('suggest')).toBe('controlled');
    expect(normalizeAIControlLevel('unexpected')).toBe('read_only');
    expect(normalizeAIControlLevel(undefined)).toBe('read_only');
  });

  it('returns canonical panel, badge, and description presentation', () => {
    expect(getAIControlLevelPanelClass('read_only')).toContain('border-blue-200');
    expect(getAIControlLevelPanelClass('controlled')).toContain('border-blue-200');
    expect(getAIControlLevelBadgeClass('controlled')).toContain('bg-amber-100');
    expect(getAIControlLevelDescription('read_only')).toContain('Assistant can query and explain');
    expect(getAIControlLevelDescription('controlled')).toContain('saves each plan to Actions');
    expect(getAIControlLevelDescription('controlled')).toContain('Chat does not execute the plan');
  });

  it('returns canonical chat control-level presentation', () => {
    expect(getAIChatControlLevelPresentation('read_only')).toMatchObject({
      label: 'Read-only',
      description: 'Observes only',
      dotClassName: 'bg-slate-400',
    });
    expect(getAIChatControlLevelPresentation('controlled')).toMatchObject({
      label: 'Ask first',
      description: 'Plans actions for your review',
      dotClassName: 'bg-amber-500',
    });
  });
});
