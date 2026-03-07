import { describe, expect, it } from 'vitest';

import { nodeFromResource, pbsInstanceFromResource, pmgInstanceFromResource } from '../resourceStateAdapters';
import type { Resource } from '@/types/resource';

const createNodeResource = (platformData: Record<string, unknown>): Resource =>
  ({
    id: 'node-1',
    type: 'agent',
    name: 'pve-node-1',
    displayName: 'PVE Node 1',
    platformId: 'pve-node-1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    cpu: { current: 10 },
    memory: { current: 20, total: 1024, used: 256 },
    disk: { current: 30, total: 2048, used: 512 },
    platformData,
  }) as Resource;

const createServiceResource = (
  type: 'pbs' | 'pmg',
  platformData: Record<string, unknown>,
  overrides: Partial<Resource> = {},
): Resource =>
  ({
    id: `${type}-1`,
    type,
    name: `${type}-name`,
    displayName: `${type.toUpperCase()} Display`,
    platformId: '',
    platformType: type === 'pbs' ? 'proxmox-pbs' : 'proxmox-pmg',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    cpu: { current: 10 },
    memory: { current: 20, total: 1024, used: 256 },
    disk: { current: 30, total: 2048, used: 512 },
    platformData,
    ...overrides,
  }) as Resource;

describe('resourceStateAdapters nodeFromResource', () => {
  it('maps canonical linkedAgentId', () => {
    const node = nodeFromResource(
      createNodeResource({
        linkedAgentId: 'agent-canonical',
        proxmox: { nodeName: 'pve-node-1' },
      }),
    );

    expect(node?.linkedAgentId).toBe('agent-canonical');
  });

  it('falls back to the actionable agent identity when linkedAgentId is absent', () => {
    const node = nodeFromResource(
      createNodeResource({
        proxmox: { nodeName: 'pve-node-1' },
        agent: { agentId: 'agent-from-facet' },
      }),
    );

    expect(node?.linkedAgentId).toBe('agent-from-facet');
  });

  it('uses typed canonical identity for node labels when proxmox nodeName is absent', () => {
    const node = nodeFromResource(
      ({
        ...createNodeResource({
          proxmox: {},
        } as Record<string, unknown>),
        name: '',
        displayName: '',
        platformId: '',
        canonicalIdentity: {
          displayName: 'Tower',
          hostname: 'tower.local',
          platformId: 'pve-canonical',
        },
      }) as Resource,
    );

    expect(node?.name).toBe('tower.local');
    expect(node?.displayName).toBe('Tower');
    expect(node?.host).toBe('tower.local');
    expect(node?.instance).toBe('pve-canonical');
  });

  it('maps PBS display and host identity through shared resource helpers', () => {
    const instance = pbsInstanceFromResource(
      createServiceResource('pbs', {
        pbs: { hostname: 'pbs-service.local', instanceId: 'pbs-instance-1' },
      }),
    );

    expect(instance?.name).toBe('PBS Display');
    expect(instance?.host).toBe('https://pbs-service.local:8007');
  });

  it('maps PMG identity through shared hostname fallback when displayName is absent', () => {
    const instance = pmgInstanceFromResource(
      createServiceResource(
        'pmg',
        {
          pmg: { hostname: 'pmg-service.local', instanceId: 'pmg-instance-1' },
        },
        { displayName: '' as unknown as Resource['displayName'] },
      ),
    );

    expect(instance?.name).toBe('pmg-service.local');
    expect(instance?.host).toBe('https://pmg-service.local:8006');
  });
});
