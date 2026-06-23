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
    expect(normalizeAIControlLevel('autonomous')).toBe('autonomous');
    expect(normalizeAIControlLevel('suggest')).toBe('controlled');
    expect(normalizeAIControlLevel('unexpected')).toBe('read_only');
    expect(normalizeAIControlLevel(undefined)).toBe('read_only');
  });

  it('returns canonical panel, badge, and description presentation', () => {
    expect(getAIControlLevelPanelClass('read_only')).toContain('border-blue-200');
    expect(getAIControlLevelPanelClass('autonomous')).toContain('border-amber-200');
    expect(getAIControlLevelBadgeClass('controlled')).toContain('bg-amber-100');
    expect(getAIControlLevelBadgeClass('autonomous')).toContain('bg-red-100');
    expect(getAIControlLevelDescription('read_only')).toContain('Assistant can query and explain');
    expect(getAIControlLevelDescription('controlled')).toContain(
      'asks before chat-only actions',
    );
    expect(getAIControlLevelDescription('autonomous')).toContain('eligible chat-only actions');
    expect(getAIControlLevelDescription('autonomous')).toContain(
      'Infrastructure work stays with Patrol mode',
    );
  });

  it('returns canonical chat control-level presentation', () => {
    expect(getAIChatControlLevelPresentation('read_only')).toMatchObject({
      label: 'Read-only',
      description: 'Observes only',
      dotClassName: 'bg-slate-400',
    });
    expect(getAIChatControlLevelPresentation('controlled')).toMatchObject({
      label: 'Ask first',
      description: 'Asks before chat-only actions',
      dotClassName: 'bg-amber-500',
    });
    expect(getAIChatControlLevelPresentation('autonomous')).toMatchObject({
      label: 'Chat actions',
      description: 'Eligible chat-only actions',
      dotClassName: 'bg-red-500',
    });
  });
});
