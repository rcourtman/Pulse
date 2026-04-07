import { describe, expect, it } from 'vitest';
import patrolIntelligenceBannersSource from '../PatrolIntelligenceBanners.tsx?raw';
import patrolIntelligenceHeaderSource from '../PatrolIntelligenceHeader.tsx?raw';

describe('patrol commercial boundary', () => {
  it('suppresses patrol upgrade surfaces in demo mode', () => {
    expect(patrolIntelligenceBannersSource).toContain('presentationPolicyHidesUpgradePrompts');
    expect(patrolIntelligenceBannersSource).toContain(
      '!presentationPolicyHidesUpgradePrompts() && state.licenseRequired()',
    );
    expect(patrolIntelligenceHeaderSource).toContain('presentationPolicyHidesUpgradePrompts');
    expect(patrolIntelligenceHeaderSource).toContain(
      '!presentationPolicyHidesUpgradePrompts() && state.autoFixLocked()',
    );
    expect(patrolIntelligenceHeaderSource).toContain(
      '!presentationPolicyHidesUpgradePrompts() && state.alertAnalysisLocked()',
    );
    expect(patrolIntelligenceHeaderSource).toContain(
      '!presentationPolicyHidesUpgradePrompts() && isProLocked()',
    );
  });
});
