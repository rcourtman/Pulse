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
  it('declares API-native Kubernetes sections', () => {
    expect(KUBERNETES_TAB_SPECS.map((tab) => tab.id)).toEqual([
      'overview',
      'workloads',
      'services',
      'storage',
      'networking',
      'config',
      'events',
    ]);
  });

  it('buckets clusters, nodes, workloads, services, storage, networking, config, and events', () => {
    const model = buildKubernetesPageModel([
      makeResource({ id: 'cluster-1', type: 'k8s-cluster' }),
      makeResource({ id: 'node-1', type: 'k8s-node' }),
      makeResource({ id: 'pod-1', type: 'pod' }),
      makeResource({ id: 'dep-1', type: 'k8s-deployment' }),
      makeResource({ id: 'svc-1', type: 'k8s-service' }),
      makeResource({ id: 'sts-1', type: 'k8s-statefulset' }),
      makeResource({ id: 'ds-1', type: 'k8s-daemonset' }),
      makeResource({ id: 'job-1', type: 'k8s-job' }),
      makeResource({ id: 'cron-1', type: 'k8s-cronjob' }),
      makeResource({ id: 'ing-1', type: 'k8s-ingress' }),
      makeResource({ id: 'pv-1', type: 'k8s-persistent-volume' }),
      makeResource({ id: 'pvc-1', type: 'k8s-persistent-volume-claim' }),
      makeResource({ id: 'ns-1', type: 'k8s-namespace' }),
      makeResource({ id: 'event-1', type: 'k8s-event' }),
      makeResource({ id: 'proxmox-vm', type: 'vm', platformType: 'proxmox-pve' }),
    ]);

    expect(model.clusters.map((r) => r.id)).toEqual(['cluster-1']);
    expect(model.nodes.map((r) => r.id)).toEqual(['node-1']);
    expect(model.pods.map((r) => r.id)).toEqual(['pod-1']);
    expect(model.deployments.map((r) => r.id)).toEqual(['dep-1']);
    expect(model.services.map((r) => r.id)).toEqual(['svc-1']);
    expect(model.statefulSets.map((r) => r.id)).toEqual(['sts-1']);
    expect(model.daemonSets.map((r) => r.id)).toEqual(['ds-1']);
    expect(model.jobs.map((r) => r.id)).toEqual(['job-1']);
    expect(model.cronJobs.map((r) => r.id)).toEqual(['cron-1']);
    expect(model.ingresses.map((r) => r.id)).toEqual(['ing-1']);
    expect(model.persistentVolumes.map((r) => r.id)).toEqual(['pv-1']);
    expect(model.persistentVolumeClaims.map((r) => r.id)).toEqual(['pvc-1']);
    expect(model.namespaces.map((r) => r.id)).toEqual(['ns-1']);
    expect(model.events.map((r) => r.id)).toEqual(['event-1']);
    expect(model.workloads.map((r) => r.id).sort()).toEqual(
      ['cron-1', 'dep-1', 'ds-1', 'job-1', 'pod-1', 'sts-1'].sort(),
    );
    expect(model.storage.map((r) => r.id).sort()).toEqual(['pv-1', 'pvc-1']);
    expect(model.networking.map((r) => r.id).sort()).toEqual(['ing-1', 'svc-1']);
    expect(model.config.map((r) => r.id)).toEqual(['ns-1']);
    expect(model.resources).toHaveLength(14);
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
