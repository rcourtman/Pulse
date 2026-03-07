import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  getAgentLikeIdentityAliases,
  getAgentLikeMetadataIds,
  getPreferredConfiguredNodeLabel,
  getPreferredNormalizedPlatformId,
  getInfrastructureDiscoveryHostname,
  getInfrastructureMetadataId,
  getPreferredResourceDisplayName,
  getPreferredResourceHostname,
  getPreferredWorkloadsAgentHint,
  getPrimaryResourceIdentity,
  getPrimaryResourceIdentityRows,
  getResourceIdentityAliases,
} from '@/utils/resourceIdentity';
import type { Agent, Node } from '@/types/api';

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'resource-1',
    type: 'agent',
    name: 'resource-1',
    displayName: 'resource-1',
    platformId: 'resource-1',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    ...overrides,
  }) as Resource;

describe('resourceIdentity', () => {
  it('prefers metrics and discovery targets for primary identity', () => {
    expect(
      getPrimaryResourceIdentity(
        makeResource({
          metricsTarget: { resourceType: 'docker-host', resourceId: 'docker-host-1' },
        }),
      ),
    ).toBe('docker-host:docker-host-1');

    expect(
      getPrimaryResourceIdentity(
        makeResource({
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'agent-1',
            resourceId: 'agent-1',
          },
        }),
      ),
    ).toBe('agent:agent-1');
  });

  it('uses actionable linked identities before falling back to unified ids', () => {
    expect(
      getPrimaryResourceIdentity(
        makeResource({
          platformData: {
            linkedAgentId: 'agent-linked',
          },
        }),
      ),
    ).toBe('agent:agent-linked');

    expect(
      getPrimaryResourceIdentity(
        makeResource({
          type: 'docker-host',
          platformData: {
            docker: { hostSourceId: 'docker-host-2' },
          },
        }),
      ),
    ).toBe('docker-host:docker-host-2');
  });

  it('deduplicates aliases and includes linked agent identities', () => {
    expect(
      getResourceIdentityAliases(
        makeResource({
          metricsTarget: { resourceType: 'docker-host', resourceId: 'docker-host-1' },
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'agent-linked',
            resourceId: 'agent-linked',
          },
          platformData: {
            linkedAgentId: 'agent-linked',
            agent: {
              agentId: 'agent-linked',
              hostname: 'tower.local',
            },
            docker: {
              hostSourceId: 'docker-host-1',
            },
          },
          identity: {
            hostname: 'tower.local',
            machineId: 'machine-1',
          },
        }),
      ),
    ).toEqual(['docker-host-1', 'agent-linked', 'tower.local', 'machine-1']);
  });

  it('builds primary identity rows from enriched targets', () => {
    expect(
      getPrimaryResourceIdentityRows(
        makeResource({
          identity: {
            hostname: 'tower.local',
            machineId: 'machine-1',
          },
          clusterId: 'cluster-a',
          parentId: 'parent-1',
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'agent-1',
            resourceId: 'agent-1',
          },
          metricsTarget: {
            resourceType: 'docker-host',
            resourceId: 'docker-host-1',
          },
        }),
      ),
    ).toEqual([
      { label: 'Hostname', value: 'tower.local' },
      { label: 'Machine ID', value: 'machine-1' },
      { label: 'Cluster', value: 'cluster-a' },
      { label: 'Parent', value: 'parent-1' },
      { label: 'Discovery', value: 'agent:agent-1' },
      { label: 'Metrics Target', value: 'docker-host:docker-host-1' },
    ]);
  });

  it('builds agent-like aliases for legacy summary/detail surfaces', () => {
    const agent = {
      id: 'agent-explicit',
      hostname: 'tower.local',
      displayName: 'Tower',
      status: 'online',
      lastSeen: Date.now(),
      platformData: {
        linkedAgentId: 'agent-linked',
        agent: {
          agentId: 'agent-platform',
          hostname: 'tower.internal',
        },
      },
      discoveryTarget: {
        resourceType: 'agent',
        resourceId: 'agent-discovery',
        agentId: 'agent-discovery',
      },
    } as unknown as Agent;

    expect(getAgentLikeIdentityAliases(agent)).toEqual([
      'agent-explicit',
      'agent-discovery',
      'agent-linked',
      'agent-platform',
      'tower.local',
      'tower.internal',
    ]);
  });

  it('builds agent-like metadata ids without hostnames', () => {
    const agent = {
      id: 'agent-explicit',
      hostname: 'tower.local',
      status: 'online',
      lastSeen: Date.now(),
      platformData: {
        linkedAgentId: 'agent-linked',
        agent: {
          agentId: 'agent-platform',
          hostname: 'tower.internal',
        },
      },
      discoveryTarget: {
        resourceType: 'agent',
        resourceId: 'agent-discovery',
        agentId: 'agent-discovery',
      },
    } as unknown as Agent;

    expect(getAgentLikeMetadataIds(agent)).toEqual([
      'agent-explicit',
      'agent-discovery',
      'agent-linked',
      'agent-platform',
    ]);
  });

  it('resolves infrastructure metadata ids and discovery hostnames', () => {
    const node = {
      id: 'node-1',
      name: 'pve1',
      linkedAgentId: 'agent-linked',
    } as Pick<Node, 'id' | 'name' | 'linkedAgentId'>;

    const agent = {
      id: 'agent-explicit',
      hostname: 'pve1.local',
      status: 'online',
      lastSeen: Date.now(),
    } as Agent;

    expect(getInfrastructureMetadataId(node, agent)).toBe('agent-explicit');
    expect(getInfrastructureMetadataId(node)).toBe('agent-linked');
    expect(getInfrastructureDiscoveryHostname({ name: 'pve1' }, agent)).toBe('pve1.local');
    expect(getInfrastructureDiscoveryHostname({ name: 'pve1' })).toBe('pve1');
  });

  it('resolves configured node labels with display-first precedence', () => {
    expect(
      getPreferredConfiguredNodeLabel({
        id: 'node-1',
        displayName: 'Cluster Node',
        name: 'pve1',
        host: 'pve1.local',
      } as never),
    ).toBe('Cluster Node');

    expect(
      getPreferredConfiguredNodeLabel({
        id: 'node-2',
        displayName: '',
        name: '',
        host: 'pbs.local',
      } as never),
    ).toBe('pbs.local');
  });

  it('resolves normalized platform ids with source-aware precedence', () => {
    expect(
      getPreferredNormalizedPlatformId({
        id: 'resource-1',
        name: 'fallback-name',
        proxmox: { nodeName: 'pve-node-1' },
        agent: { hostname: 'agent-host.local' },
        docker: { hostname: 'docker-host.local' },
      }),
    ).toBe('pve-node-1');

    expect(
      getPreferredNormalizedPlatformId({
        id: 'resource-2',
        name: 'fallback-name',
        agent: { hostname: 'agent-host.local' },
        docker: { hostname: 'docker-host.local' },
      }),
    ).toBe('agent-host.local');
  });

  it('resolves shared resource hostnames and display names', () => {
    const resource = makeResource({
      name: 'fallback-name',
      displayName: '',
      platformId: 'platform-name',
      platformData: {
        agent: {
          hostname: 'platform-host.local',
        },
      },
    });

    expect(getPreferredResourceHostname(resource)).toBe('platform-host.local');
    expect(getPreferredResourceDisplayName(resource)).toBe('platform-host.local');

    expect(
      getPreferredResourceDisplayName(
        makeResource({
          displayName: 'Display Label',
          platformData: {
            agent: {
              hostname: 'platform-host.local',
            },
          },
        }),
      ),
    ).toBe('Display Label');
  });

  it('resolves workloads agent hints with source-specific precedence', () => {
    expect(
      getPreferredWorkloadsAgentHint(
        makeResource({
          type: 'docker-host',
          identity: { hostname: 'identity-host' },
          platformData: {
            docker: { hostname: 'docker-host-1' },
          },
        }),
      ),
    ).toBe('docker-host-1');

    expect(
      getPreferredWorkloadsAgentHint(
        makeResource({
          type: 'agent',
          identity: { hostname: 'identity-host' },
          platformData: {
            proxmox: { nodeName: 'pve1' },
          },
        }),
      ),
    ).toBe('pve1');
  });
});
