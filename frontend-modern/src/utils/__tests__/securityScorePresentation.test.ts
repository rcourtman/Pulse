import { describe, expect, it } from 'vitest';
import {
  getSecurityScorePresentation,
  getSecurityScoreSymbol,
  getSecurityScoreTextClass,
} from '@/utils/securityScorePresentation';

describe('securityScorePresentation', () => {
  it('returns strong posture presentation for high scores', () => {
    expect(getSecurityScorePresentation(92)).toMatchObject({
      label: 'Strong',
      icon: 'shield-check',
    });
    expect(getSecurityScorePresentation(92).tone.badge).toContain('emerald');
  });

  it('returns moderate posture presentation for medium scores', () => {
    expect(getSecurityScorePresentation(65)).toMatchObject({
      label: 'Moderate',
      icon: 'shield',
    });
    expect(getSecurityScorePresentation(65).tone.badge).toContain('amber');
  });

  it('returns weak posture presentation for low scores', () => {
    expect(getSecurityScorePresentation(25)).toMatchObject({
      label: 'Weak',
      icon: 'shield-alert',
    });
    expect(getSecurityScorePresentation(25).tone.badge).toContain('rose');
  });

  it('returns matching text classes and symbols for score tiers', () => {
    expect(getSecurityScoreTextClass(90)).toContain('emerald');
    expect(getSecurityScoreSymbol(90)).toBe('✓');
    expect(getSecurityScoreTextClass(60)).toContain('amber');
    expect(getSecurityScoreSymbol(60)).toBe('!');
    expect(getSecurityScoreTextClass(20)).toContain('rose');
    expect(getSecurityScoreSymbol(20)).toBe('!!');
  });
});
