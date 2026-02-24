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

  it('prefers docker hostSourceId for docker-host fallback mapping', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:docker:abc123',
      type: 'docker-host',
      platformType: 'docker',
      platformData: {
        sources: ['docker'],
        docker: {
          hostSourceId: 'docker-host-1',
          hostname: 'edge-docker',
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'host',
      hostId: 'docker-host-1',
      resourceId: 'docker-host-1',
      hostname: 'stale-hostname',
      metadataKind: 'host',
      metadataId: 'docker-host-1',
      targetLabel: 'host',
    });
  });

  it('prefers proxmox vmid for vm fallback resourceId', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:vm:hash-101',
      type: 'vm',
      platformData: {
        sources: ['proxmox'],
        proxmox: {
          nodeName: 'pve1',
          vmid: 101,
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'vm',
      hostId: 'pve1',
      resourceId: '101',
      hostname: 'stale-hostname',
      metadataKind: 'guest',
      metadataId: 'resource:vm:hash-101',
      targetLabel: 'guest',
    });
  });

  it('prefers docker hostSourceId for docker-container fallback hostId', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:docker-container:hash-1',
      type: 'docker-container',
      platformType: 'docker',
      platformData: {
        sources: ['docker'],
        docker: {
          hostSourceId: 'docker-host-1',
          containerId: 'container-abc123',
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'docker',
      hostId: 'docker-host-1',
      resourceId: 'container-abc123',
      hostname: 'stale-hostname',
      metadataKind: 'guest',
      metadataId: 'resource:docker-container:hash-1',
      targetLabel: 'container',
    });
  });

  it('prefers kubernetes cluster/pod IDs for pod fallback mapping', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:pod:hash-1',
      type: 'pod',
      platformType: 'kubernetes',
      clusterId: 'cluster-a',
      kubernetes: {
        clusterId: 'cluster-a',
        podUid: 'pod-uid-1',
        namespace: 'default',
      },
      platformData: {
        sources: ['kubernetes'],
        kubernetes: {
          clusterId: 'cluster-a',
          namespace: 'default',
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'k8s',
      hostId: 'cluster-a',
      resourceId: 'pod-uid-1',
      hostname: 'stale-hostname',
      metadataKind: 'guest',
      metadataId: 'resource:pod:hash-1',
      targetLabel: 'workload',
    });
  });

  it('prefers kubernetes agentId over clusterId for pod fallback hostId', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:pod:hash-2',
      type: 'pod',
      platformType: 'kubernetes',
      kubernetes: {
        agentId: 'k8s-agent-1',
        clusterId: 'cluster-a',
        podUid: 'pod-uid-2',
        namespace: 'default',
      },
      platformData: {
        sources: ['kubernetes'],
        kubernetes: {
          agentId: 'k8s-agent-1',
          clusterId: 'cluster-a',
          namespace: 'default',
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'k8s',
      hostId: 'k8s-agent-1',
      resourceId: 'pod-uid-2',
      hostname: 'stale-hostname',
      metadataKind: 'guest',
      metadataId: 'resource:pod:hash-2',
      targetLabel: 'workload',
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
