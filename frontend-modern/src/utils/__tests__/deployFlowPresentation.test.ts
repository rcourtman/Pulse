import { describe, expect, it } from 'vitest';
import {
  getDeployCandidatesLoadingState,
  getDeployInstallCommandLoadingState,
  getDeployNoCandidatesState,
  getDeployNoSourceAgentsState,
} from '@/utils/deployFlowPresentation';

describe('deployFlowPresentation', () => {
  it('returns canonical deploy flow loading and empty-state copy', () => {
    expect(getDeployCandidatesLoadingState()).toBe('Loading cluster nodes...');
    expect(getDeployNoSourceAgentsState()).toBe(
      'No online source agents found. At least one node in this cluster must have a connected Pulse agent to deploy to other nodes.',
    );
    expect(getDeployNoCandidatesState()).toBe('No nodes found in this cluster.');
    expect(getDeployInstallCommandLoadingState()).toBe('Loading install command...');
  });
});
