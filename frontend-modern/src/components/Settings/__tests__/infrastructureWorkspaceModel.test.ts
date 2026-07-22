import { describe, expect, it } from 'vitest';
import {
  buildInfrastructureAgentDoctorPath,
  buildInfrastructureAgentUpdatesPath,
  buildInfrastructureOnboardingPath,
  buildInfrastructureWorkspacePath,
  deriveAgentUpdateScopeFromLocation,
  deriveAgentUpdatesFromLocation,
  deriveAddStepFromLocation,
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
    expect(buildInfrastructureOnboardingPath('unraid')).toBe('/settings/infrastructure?add=unraid');
    expect(buildInfrastructureOnboardingPath('docker')).toBe('/settings/infrastructure?add=docker');
    expect(buildInfrastructureOnboardingPath('vmware')).toBe('/settings/infrastructure?add=vmware');
  });

  it('builds the canonical Agent Doctor route and accepts legacy update deep links', () => {
    expect(buildInfrastructureAgentDoctorPath()).toBe('/settings/infrastructure/agent-doctor');
    expect(buildInfrastructureAgentUpdatesPath()).toBe('/settings/infrastructure/agent-doctor');
    expect(buildInfrastructureAgentUpdatesPath(['agent:agent-delly', 'agent-pi'])).toBe(
      '/settings/infrastructure/agent-doctor?agents=agent%3Aagent-delly&agents=agent%3Aagent-pi',
    );
    expect(deriveAgentUpdatesFromLocation('/settings/infrastructure/agent-doctor', '')).toBe(true);
    expect(
      deriveAgentUpdateScopeFromLocation(
        '/settings/infrastructure/agent-doctor',
        '?agents=agent%3Aagent-pi&agents=agent-delly',
      ),
    ).toEqual(['agent:agent-delly', 'agent:agent-pi']);
    expect(deriveAgentUpdatesFromLocation('/settings/infrastructure', '?agentDoctor=1')).toBe(true);
    expect(deriveAgentUpdatesFromLocation('/settings/infrastructure', '?agentUpdates=1')).toBe(
      true,
    );
    expect(
      deriveAgentUpdateScopeFromLocation(
        '/settings/infrastructure',
        '?agentUpdates=1&agents=agent%3Aagent-pi&agents=agent-delly',
      ),
    ).toEqual(['agent:agent-delly', 'agent:agent-pi']);
    expect(deriveAgentUpdatesFromLocation('/settings/infrastructure', '?agentUpdates=0')).toBe(
      false,
    );
    expect(
      deriveAgentUpdateScopeFromLocation('/settings/infrastructure', '?agentUpdates=0&agents=a'),
    ).toEqual([]);
    expect(
      deriveAgentUpdatesFromLocation('/settings/infrastructure/install', '?agentUpdates=1'),
    ).toBe(false);
  });

  it('derives add steps only from the canonical infrastructure workspace query', () => {
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=agent')).toBe('agent');
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=pick')).toBe('pick');
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=truenas')).toBe('truenas');
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=unraid')).toBe('unraid');
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=kubernetes')).toBe(
      'kubernetes',
    );
    expect(deriveAddStepFromLocation('/settings/infrastructure', '?add=availability')).toBeNull();
    expect(deriveAddStepFromLocation('/settings/infrastructure/install', '')).toBeNull();
    expect(deriveAddStepFromLocation('/settings/infrastructure/platforms', '?add=pick')).toBeNull();
  });
});
