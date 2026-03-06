import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getActionableAgentIdFromResource,
  getExplicitAgentIdFromResource,
  hasAgentFacet,
  isAgentFacetInfrastructureResource,
  isAgentProfileAssignableResource,
} from '@/utils/agentResources';

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

describe('agentResources', () => {
  it('resolves explicit agent ids from canonical and platform facets', () => {
    expect(
      getExplicitAgentIdFromResource(
        makeResource({
          agent: { agentId: 'agent-direct' },
        }),
      ),
    ).toBe('agent-direct');

    expect(
      getExplicitAgentIdFromResource(
        makeResource({
          platformData: {
            agent: { agentId: 'agent-platform' },
          },
        }),
      ),
    ).toBe('agent-platform');

    expect(
      getExplicitAgentIdFromResource(
        makeResource({
          type: 'k8s-cluster',
          kubernetes: { agentId: 'agent-k8s' },
        }),
      ),
    ).toBe('agent-k8s');
  });

  it('falls back to discovery coordinates for actionable agent ids', () => {
    expect(
      getActionableAgentIdFromResource(
        makeResource({
          discoveryTarget: {
            resourceType: 'agent',
            resourceId: 'agent-discovery',
            agentId: 'agent-discovery-fallback',
          },
        }),
      ),
    ).toBe('agent-discovery');

    expect(
      getActionableAgentIdFromResource(
        makeResource({
          discoveryTarget: {
            resourceType: 'vm',
            resourceId: 'vm-100',
            agentId: 'agent-from-target',
          },
        }),
      ),
    ).toBe('agent-from-target');
  });

  it('detects agent facets without relying on node-only typing', () => {
    expect(
      hasAgentFacet(
        makeResource({
          type: 'pbs',
          platformData: {
            agentId: 'pbs-agent',
          },
        }),
      ),
    ).toBe(true);

    expect(
      hasAgentFacet(
        makeResource({
          type: 'docker-host',
          platformData: {
            docker: { hostSourceId: 'docker-1' },
          },
        }),
      ),
    ).toBe(false);
  });

  it('distinguishes infrastructure agent-facet resources from profile-assignable resources', () => {
    expect(
      isAgentFacetInfrastructureResource(
        makeResource({
          type: 'pmg',
          platformData: {
            agent: { agentId: 'pmg-agent' },
          },
        }),
      ),
    ).toBe(true);

    expect(
      isAgentFacetInfrastructureResource(
        makeResource({
          type: 'docker-host',
          agent: { agentId: 'docker-agent' },
        }),
      ),
    ).toBe(false);

    expect(
      isAgentProfileAssignableResource(
        makeResource({
          type: 'docker-host',
        }),
      ),
    ).toBe(true);

    expect(
      isAgentProfileAssignableResource(
        makeResource({
          type: 'vm',
        }),
      ),
    ).toBe(false);
  });
});
