import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { toDiscoveryConfig } from '@/components/Infrastructure/resourceDetailMappers';
import { buildWorkloadsHref } from '@/components/Infrastructure/workloadsLink';

const baseResource = (): Resource => ({
  id: 'host-abcd',
  type: 'host',
  name: 'pve1',
  displayName: 'pve1',
  platformId: 'pve1',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.now(),
  platformData: {
    sources: ['proxmox', 'agent'],
    proxmox: { nodeName: 'pve1' },
    agent: { agentId: 'host-1', hostname: 'pve1.local' },
  },
  identity: {
    hostname: 'stale-hostname',
  },
});

describe('toDiscoveryConfig', () => {
  it('prefers backend discoveryTarget over heuristic IDs', () => {
    const resource: Resource = {
      ...baseResource(),
      discoveryTarget: {
        resourceType: 'host',
        hostId: 'host-1',
        resourceId: 'host-1',
        hostname: 'pve1.local',
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'host',
      hostId: 'host-1',
      resourceId: 'host-1',
      hostname: 'pve1.local',
      metadataKind: 'host',
      metadataId: 'host-1',
      targetLabel: 'host',
    });
  });

  it('falls back to heuristic mapping when discoveryTarget is absent', () => {
    const config = toDiscoveryConfig(baseResource());
    expect(config).toEqual({
      resourceType: 'host',
      hostId: 'host-1',
      resourceId: 'host-1',
      hostname: 'stale-hostname',
      metadataKind: 'host',
      metadataId: 'host-1',
      targetLabel: 'host',
    });
  });
});

describe('buildWorkloadsHref', () => {
  it('builds k8s workload route for cluster resources with context', () => {
    const resource: Resource = {
      ...baseResource(),
      type: 'k8s-cluster',
      name: 'cluster-a',
      displayName: 'cluster-a',
      clusterId: 'cluster-a',
      platformData: {
        sources: ['kubernetes'],
        kubernetes: {
          clusterName: 'cluster-a',
        },
      },
    };

    expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=cluster-a');
  });

  it('builds k8s workload route for node resources using cluster context', () => {
    const resource: Resource = {
      ...baseResource(),
      type: 'k8s-node',
      name: 'worker-1',
      displayName: 'worker-1',
      clusterId: 'cluster-a',
      platformData: {
        sources: ['kubernetes'],
        kubernetes: {
          clusterId: 'cluster-a',
        },
      },
    };

    expect(buildWorkloadsHref(resource)).toBe('/workloads?type=k8s&context=cluster-a');
  });

  it('returns null for non-kubernetes infrastructure resources', () => {
    const resource: Resource = {
      ...baseResource(),
      type: 'pbs',
      platformType: 'proxmox-pbs',
      sourceType: 'api',
      platformData: {
        sources: ['pbs'],
      },
    };
    expect(buildWorkloadsHref(resource)).toBeNull();
  });

  it('builds workloads route with host hint for proxmox node resources', () => {
    const resource: Resource = {
      ...baseResource(),
      type: 'node',
      platformData: {
        sources: ['proxmox'],
        proxmox: { nodeName: 'pve1' },
      },
    };
    expect(buildWorkloadsHref(resource)).toBe('/workloads?host=pve1');
  });

  it('builds workloads route with docker type and host hint for docker hosts', () => {
    const resource: Resource = {
      ...baseResource(),
      type: 'docker-host',
      platformType: 'docker',
      sourceType: 'agent',
      platformData: {
        sources: ['docker'],
        docker: { hostname: 'docker-host-1' },
      },
    };
    expect(buildWorkloadsHref(resource)).toBe('/workloads?type=docker&host=docker-host-1');
  });
});
