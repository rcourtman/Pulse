import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  getPrimaryResourceIdentity,
  getPrimaryResourceIdentityRows,
  getResourceIdentityAliases,
} from '@/utils/resourceIdentity';

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
});
