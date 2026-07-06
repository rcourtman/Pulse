import { describe, expect, it } from 'vitest';
import patrolAutonomyAvailabilitySource from '../patrolAutonomyAvailability.ts?raw';
import patrolIntelligenceBannersSource from '../PatrolIntelligenceBanners.tsx?raw';
import patrolIntelligenceHeaderSource from '../PatrolIntelligenceHeader.tsx?raw';

describe('patrol commercial boundary', () => {
  it('suppresses patrol upgrade surfaces when upgrade prompts are hidden', () => {
    expect(patrolIntelligenceBannersSource).toContain('presentationPolicyHidesUpgradePrompts');
    expect(patrolIntelligenceBannersSource).toContain('!presentationPolicyHidesUpgradePrompts()');
    expect(patrolIntelligenceBannersSource).toContain('state.licenseRequired()');
    expect(patrolIntelligenceBannersSource).toContain('!state.showBlockedBanner()');
    expect(patrolIntelligenceBannersSource).toContain('!state.shouldShowPatrolSetupOnly()');
    expect(patrolIntelligenceHeaderSource).toContain('presentationPolicyHidesUpgradePrompts');
    expect(patrolIntelligenceHeaderSource).toContain('presentationPolicyHidesCommercialSurfaces');
    expect(patrolIntelligenceHeaderSource).toContain('!presentationPolicyHidesUpgradePrompts()');
    expect(patrolIntelligenceHeaderSource).toContain('!commercialSurfacesHidden()');
    expect(patrolIntelligenceHeaderSource).toContain('state.autoFixLocked()');
    expect(patrolIntelligenceHeaderSource).toContain(
      "autonomyAvailability().kind === 'runtime_locked'",
    );
    expect(patrolIntelligenceHeaderSource).toContain('showAutonomyPlanBillingAction');
    expect(patrolIntelligenceHeaderSource).toContain(
      "autonomyAvailability().kind === 'plan_locked'",
    );
    expect(patrolAutonomyAvailabilitySource).toContain('Plans & Billing');
    expect(patrolAutonomyAvailabilitySource).toContain('input.upgradePromptsHidden');
    expect(patrolIntelligenceHeaderSource).toContain('getPatrolAutonomyAvailabilityPresentation');
    expect(patrolIntelligenceHeaderSource).toContain('!presentationPolicyHidesUpgradePrompts()');
    expect(patrolIntelligenceHeaderSource).toContain('canChooseAutonomyLevel');
    expect(patrolIntelligenceHeaderSource).toContain('shouldShowAutonomyOptions');
    expect(patrolIntelligenceHeaderSource).toContain('shouldShowAutonomyActionColumn');
    expect(patrolIntelligenceHeaderSource).toContain(
      "autonomyAvailability().kind === 'runtime_locked'",
    );
    expect(patrolIntelligenceHeaderSource).toContain('<Show when={shouldShowAutonomyOptions()}>');
    expect(patrolIntelligenceHeaderSource).toContain(
      '<Show when={shouldShowAutonomyActionColumn()}>',
    );
    expect(patrolIntelligenceHeaderSource).toContain('showProBadge');
    expect(patrolIntelligenceHeaderSource).not.toContain('const isProLocked = () =>');
    expect(patrolIntelligenceHeaderSource).not.toContain('requires Pulse Pro');
  });

  it('keeps patrol configuration out of the operator header', () => {
    expect(patrolIntelligenceHeaderSource).toContain('Open Patrol settings');
    expect(patrolIntelligenceHeaderSource).toContain("settingsTabPath('system-ai-patrol')");
    expect(patrolIntelligenceHeaderSource).not.toContain(
      "import { FormSelect } from '@/components/shared/FormSelect';",
    );
    expect(patrolIntelligenceHeaderSource).not.toContain('label="Provider model"');
    expect(patrolIntelligenceHeaderSource).not.toContain('label="Run Every"');
    expect(patrolIntelligenceHeaderSource).not.toContain('<select');
  });

  it('keeps blocked provider readiness to one primary action', () => {
    expect(patrolIntelligenceBannersSource).toContain('shouldShowReadinessAction');
    expect(patrolIntelligenceBannersSource).toContain(
      "state.patrolReadiness()?.status !== 'not_ready'",
    );
    expect(patrolIntelligenceBannersSource).toContain('<Show when={shouldShowReadinessAction()}>');
    expect(patrolIntelligenceHeaderSource).toContain('Check model');
  });

  it('does not carry the removed patrol configuration panel chrome', () => {
    expect(patrolIntelligenceHeaderSource).not.toContain('fixed right-4 top-32 z-[9999] isolate');
    expect(patrolIntelligenceHeaderSource).not.toContain('max-h-[calc(100vh-10rem)]');
    expect(patrolIntelligenceHeaderSource).not.toContain('sm:max-h-[calc(100vh-14rem)]');
    expect(patrolIntelligenceHeaderSource).not.toContain('sm:top-[13rem]');
    expect(patrolIntelligenceHeaderSource).not.toContain('bg-white p-5 shadow-xl');
    expect(patrolIntelligenceHeaderSource).not.toContain('invisible pointer-events-none');
  });
});
