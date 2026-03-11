import { describe, expect, it } from 'vitest';
import {
  getAgentProfileAssignmentsEmptyState,
  getAgentProfilesEmptyState,
} from '@/utils/agentProfilesPresentation';

describe('agentProfilesPresentation', () => {
  it('returns canonical agent profile empty-state copy', () => {
    expect(getAgentProfilesEmptyState()).toBe('No profiles yet. Create one to get started.');
    expect(getAgentProfileAssignmentsEmptyState()).toBe(
      'No agents connected. Install an agent to assign profiles.',
    );
  });
});
