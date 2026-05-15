import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { KUBERNETES_TAB_SPECS, buildKubernetesPageModel } from '../kubernetesPageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'kubernetes',
  sourceType: 'agent',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('kubernetesPageModel', () => {
  it('declares the Kubernetes section set', () => {
    expect(KUBERNETES_TAB_SPECS.map((tab) => tab.id)).toEqual([
      'overview',
      'nodes',
      'pods',
      'deployments',
      'services',
    ]);
  });

  it('buckets clusters, nodes, pods, deployments, and services', () => {
    const model = buildKubernetesPageModel([
      makeResource({ id: 'cluster-1', type: 'k8s-cluster' }),
      makeResource({ id: 'node-1', type: 'k8s-node' }),
      makeResource({ id: 'pod-1', type: 'pod' }),
      makeResource({ id: 'dep-1', type: 'k8s-deployment' }),
      makeResource({ id: 'svc-1', type: 'k8s-service' }),
      makeResource({ id: 'proxmox-vm', type: 'vm', platformType: 'proxmox-pve' }),
    ]);

    expect(model.clusters.map((r) => r.id)).toEqual(['cluster-1']);
    expect(model.nodes.map((r) => r.id)).toEqual(['node-1']);
    expect(model.pods.map((r) => r.id)).toEqual(['pod-1']);
    expect(model.deployments.map((r) => r.id)).toEqual(['dep-1']);
    expect(model.services.map((r) => r.id)).toEqual(['svc-1']);
    expect(model.resources).toHaveLength(5);
  });
});
