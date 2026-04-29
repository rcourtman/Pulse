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

  it('keeps the patrol configuration panel above lower page actions', () => {
    expect(patrolIntelligenceHeaderSource).toContain('fixed right-4 top-32 z-[9999] isolate');
    expect(patrolIntelligenceHeaderSource).toContain('max-h-[calc(100vh-10rem)]');
    expect(patrolIntelligenceHeaderSource).toContain('sm:max-h-[36rem]');
    expect(patrolIntelligenceHeaderSource).toContain('overflow-y-auto');
    expect(patrolIntelligenceHeaderSource).toContain('sm:top-[13rem]');
    expect(patrolIntelligenceHeaderSource).not.toContain('sm:top-[17.5rem]');
    expect(patrolIntelligenceHeaderSource).not.toContain('disabled:opacity-70');
  });
});
