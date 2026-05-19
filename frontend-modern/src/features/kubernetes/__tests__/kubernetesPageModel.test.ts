import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { KUBERNETES_TAB_SPECS, buildKubernetesPageModel } from '../kubernetesPageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('kubernetesPageModel', () => {
  it('declares the Kubernetes section set as a single Overview tab', () => {
    expect(KUBERNETES_TAB_SPECS.map((tab) => tab.id)).toEqual(['overview']);
  });

  it('buckets clusters, nodes, pods, and deployments', () => {
    const model = buildKubernetesPageModel([
      makeResource({ id: 'cluster-1', type: 'k8s-cluster' }),
      makeResource({ id: 'node-1', type: 'k8s-node' }),
      makeResource({ id: 'pod-1', type: 'pod' }),
      makeResource({ id: 'dep-1', type: 'k8s-deployment' }),
      makeResource({ id: 'proxmox-vm', type: 'vm', platformType: 'proxmox-pve' }),
    ]);

    expect(model.clusters.map((r) => r.id)).toEqual(['cluster-1']);
    expect(model.nodes.map((r) => r.id)).toEqual(['node-1']);
    expect(model.pods.map((r) => r.id)).toEqual(['pod-1']);
    expect(model.deployments.map((r) => r.id)).toEqual(['dep-1']);
    expect(model.resources).toHaveLength(4);
  });

  it('treats agent rows that report a kubernetes source as Kubernetes nodes', () => {
    const model = buildKubernetesPageModel([
      makeResource({
        id: 'merged-node-1',
        type: 'agent',
        platformType: 'kubernetes',
        sources: ['agent', 'kubernetes'],
      }),
      makeResource({
        id: 'plain-host',
        type: 'agent',
        platformType: 'agent',
        sources: ['agent'],
      }),
    ]);

    expect(model.nodes.map((r) => r.id)).toEqual(['merged-node-1']);
  });
});
