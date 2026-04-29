import { describe, expect, it } from 'vitest';
import patrolIntelligenceBannersSource from '../PatrolIntelligenceBanners.tsx?raw';
import patrolIntelligenceHeaderSource from '../PatrolIntelligenceHeader.tsx?raw';

describe('patrol commercial boundary', () => {
  it('suppresses patrol upgrade surfaces when upgrade prompts are hidden', () => {
    expect(patrolIntelligenceBannersSource).toContain('presentationPolicyHidesUpgradePrompts');
    expect(patrolIntelligenceBannersSource).toContain('!presentationPolicyHidesUpgradePrompts()');
    expect(patrolIntelligenceBannersSource).toContain('state.licenseRequired()');
    expect(patrolIntelligenceBannersSource).toContain('!state.showBlockedBanner()');
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

  it('keeps patrol configuration selects on the shared labelled primitive', () => {
    expect(patrolIntelligenceHeaderSource).toContain(
      "import { FormSelect } from '@/components/shared/FormSelect';",
    );
    expect(patrolIntelligenceHeaderSource).toContain('label="Provider model"');
    expect(patrolIntelligenceHeaderSource).toContain('label="Run Every"');
    expect(patrolIntelligenceHeaderSource).not.toContain('<select');
  });
});
