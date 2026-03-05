import { describe, expect, it } from 'vitest';
import { getAgentDiscoveryResourceId, isAgentDiscoveryResourceType } from '@/utils/discoveryTarget';

describe('discoveryTarget utils', () => {
  it('treats only agent as an agent discovery type', () => {
    expect(isAgentDiscoveryResourceType('agent')).toBe(true);
    expect(isAgentDiscoveryResourceType('host')).toBe(false);
    expect(isAgentDiscoveryResourceType('docker')).toBe(false);
  });

  it('prefers resourceId for agent-like discovery targets', () => {
    expect(
      getAgentDiscoveryResourceId({
        resourceType: 'agent',
        resourceId: 'agent-1',
        agentId: 'fallback-agent-id',
      }),
    ).toBe('agent-1');

    expect(
      getAgentDiscoveryResourceId({
        resourceType: 'agent',
        resourceId: '',
        agentId: 'agent-1',
      }),
    ).toBe('agent-1');
  });

  it('returns undefined for non-agent discovery types', () => {
    expect(
      getAgentDiscoveryResourceId({
        resourceType: 'docker',
        resourceId: 'docker-host-1',
        agentId: 'docker-host-1',
      }),
    ).toBeUndefined();
  });
});
