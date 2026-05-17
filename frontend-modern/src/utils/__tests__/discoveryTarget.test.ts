import { describe, expect, it } from 'vitest';
import {
  canonicalDiscoveryResourceType,
  getAgentDiscoveryResourceId,
  isAgentDiscoveryResourceType,
  isAppContainerDiscoveryResourceType,
  toDiscoveryAPIResourceType,
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
    expect(canonicalDiscoveryResourceType('docker')).toBe('app-container');
    expect(canonicalDiscoveryResourceType('app-container')).toBe('app-container');
    expect(canonicalDiscoveryResourceType('k8s')).toBe('pod');
    expect(canonicalDiscoveryResourceType('pod')).toBe('pod');
  });

  it('translates frontend canonical types to discovery API vocabulary', () => {
    expect(toDiscoveryAPIResourceType('app-container')).toBe('docker');
    expect(toDiscoveryAPIResourceType('docker')).toBe('docker');
    expect(toDiscoveryAPIResourceType('pod')).toBe('k8s');
    expect(toDiscoveryAPIResourceType('k8s')).toBe('k8s');
    expect(toDiscoveryAPIResourceType('vm')).toBe('vm');
    expect(toDiscoveryAPIResourceType('system-container')).toBe('system-container');
    expect(toDiscoveryAPIResourceType('agent')).toBe('agent');
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
