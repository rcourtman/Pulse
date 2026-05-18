import { describe, expect, it } from 'vitest';
import {
  buildInfrastructureOnboardingPath,
  buildInfrastructureWorkspacePath,
  deriveAddStepFromLocation,
  deriveAddStepFromLegacyPath,
} from '../infrastructureWorkspaceModel';

describe('infrastructureWorkspaceModel', () => {
  it('keeps the canonical infrastructure workspace path at the single workspace shell', () => {
    expect(buildInfrastructureWorkspacePath()).toBe('/settings/infrastructure');
  });

  it('builds explicit onboarding paths for first-task handoffs', () => {
    expect(buildInfrastructureOnboardingPath('agent')).toBe('/settings/infrastructure?add=agent');
    expect(buildInfrastructureOnboardingPath('linux-host')).toBe(
      '/settings/infrastructure?add=linux-host',
    );
    expect(buildInfrastructureOnboardingPath('pick')).toBe('/settings/infrastructure?add=pick');
    expect(buildInfrastructureOnboardingPath('truenas')).toBe(
      '/settings/infrastructure?add=truenas',
    );
    expect(buildInfrastructureOnboardingPath('unraid')).toBe(
      '/settings/infrastructure?add=unraid',
    );
    expect(buildInfrastructureOnboardingPath('docker')).toBe(
      '/settings/infrastructure?add=docker',
    );
    expect(buildInfrastructureOnboardingPath('vmware')).toBe(
      '/settings/infrastructure?add=vmware',
    );
  });

  it('derives add steps from onboarding deep links', () => {
    expect(deriveAddStepFromLegacyPath('/settings/infrastructure/install')).toBe('linux-host');
    expect(deriveAddStepFromLegacyPath('/settings/infrastructure/platforms')).toBe('pick');
    expect(deriveAddStepFromLegacyPath('/settings/infrastructure/platforms/truenas')).toBeNull();
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=agent')).toBe('agent');
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=pick')).toBe('pick');
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=truenas')).toBe(
      'truenas',
    );
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=unraid')).toBe('unraid');
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=kubernetes')).toBe(
      'kubernetes',
    );
  });
});
