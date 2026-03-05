import { describe, expect, it } from 'vitest';
import {
  canonicalDiscoveryResourceType,
  getAgentDiscoveryResourceId,
  isAgentDiscoveryResourceType,
  isAppContainerDiscoveryResourceType,
} from '@/utils/discoveryTarget';

describe('discoveryTarget utils', () => {
  it('treats only agent as an agent discovery type', () => {
    expect(isAgentDiscoveryResourceType('agent')).toBe(true);
    expect(isAgentDiscoveryResourceType('host')).toBe(false);
    expect(isAgentDiscoveryResourceType('docker')).toBe(false);
  });

  it('uses canonical app-container discovery type only', () => {
    expect(isAppContainerDiscoveryResourceType('app-container')).toBe(true);
    expect(isAppContainerDiscoveryResourceType('docker')).toBe(false);
    expect(canonicalDiscoveryResourceType('docker')).toBe('docker');
    expect(canonicalDiscoveryResourceType('app-container')).toBe('app-container');
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
        resourceType: 'app-container',
        resourceId: 'docker-host-1',
        agentId: 'docker-host-1',
      }),
    ).toBeUndefined();
  });
});
