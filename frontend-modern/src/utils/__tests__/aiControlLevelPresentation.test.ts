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
    expect(getAIControlLevelDescription('read_only')).toContain('query and observe only');
    expect(getAIControlLevelDescription('controlled')).toContain('with approval');
    expect(getAIControlLevelDescription('autonomous')).toContain('without confirmation');
  });

  it('returns canonical chat control-level presentation', () => {
    expect(getAIChatControlLevelPresentation('read_only')).toMatchObject({
      label: 'Read-only',
      description: 'No commands or control actions',
      dotClassName: 'bg-slate-400',
    });
    expect(getAIChatControlLevelPresentation('controlled')).toMatchObject({
      label: 'Approval',
      description: 'Ask before running commands',
      dotClassName: 'bg-amber-500',
    });
    expect(getAIChatControlLevelPresentation('autonomous')).toMatchObject({
      label: 'Autonomous',
      description: 'Executes without approval (Pro)',
      dotClassName: 'bg-red-500',
    });
  });
});
