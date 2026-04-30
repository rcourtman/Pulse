import { describe, expect, it } from 'vitest';
import {
  getAgentProfileAssignmentsEmptyState,
  getAgentProfilesFeatureGateCopy,
  getAgentProfilesEmptyState,
} from '@/utils/agentProfilesPresentation';

describe('agentProfilesPresentation', () => {
  it('returns capability-focused feature gate copy', () => {
    expect(getAgentProfilesFeatureGateCopy()).toMatchObject({
      title: 'Agent Profiles',
      subtitle: 'Centralized agent configuration',
      body: expect.not.stringContaining('Pro'),
    });
  });

  it('returns canonical agent profile empty-state copy', () => {
    expect(getAgentProfilesEmptyState()).toBe('No profiles yet. Create one to get started.');
    expect(getAgentProfileAssignmentsEmptyState()).toBe(
      'No agents connected. Install an agent to assign profiles.',
    );
  });
});
