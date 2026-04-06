import { describe, expect, it } from 'vitest';
import patrolIntelligenceBannersSource from '../PatrolIntelligenceBanners.tsx?raw';
import patrolIntelligenceHeaderSource from '../PatrolIntelligenceHeader.tsx?raw';

describe('patrol commercial boundary', () => {
  it('suppresses patrol upgrade surfaces in demo mode', () => {
    expect(patrolIntelligenceBannersSource).toContain('demoModeEnabled');
    expect(patrolIntelligenceBannersSource).toContain(
      '!demoModeEnabled() && state.licenseRequired()',
    );
    expect(patrolIntelligenceHeaderSource).toContain('demoModeEnabled');
    expect(patrolIntelligenceHeaderSource).toContain(
      '!demoModeEnabled() && state.autoFixLocked()',
    );
    expect(patrolIntelligenceHeaderSource).toContain(
      '!demoModeEnabled() && state.alertAnalysisLocked()',
    );
    expect(patrolIntelligenceHeaderSource).toContain('!demoModeEnabled() && isProLocked()');
  });
});
