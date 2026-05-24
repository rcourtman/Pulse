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
      'nodes',
      'workloads',
      'services',
      'storage',
      'networking',
      'config',
      'policy',
      'autoscaling',
      'events',
    ]);
  });

  it('buckets clusters, nodes, workloads, services, storage, networking, config, policy, autoscaling, and events', () => {
    const model = buildKubernetesPageModel([
      makeResource({ id: 'cluster-1', type: 'k8s-cluster' }),
      makeResource({ id: 'node-1', type: 'k8s-node' }),
      makeResource({ id: 'pod-1', type: 'pod' }),
      makeResource({ id: 'dep-1', type: 'k8s-deployment' }),
      makeResource({ id: 'rs-1', type: 'k8s-replicaset' }),
      makeResource({ id: 'svc-1', type: 'k8s-service' }),
      makeResource({ id: 'sts-1', type: 'k8s-statefulset' }),
      makeResource({ id: 'ds-1', type: 'k8s-daemonset' }),
      makeResource({ id: 'job-1', type: 'k8s-job' }),
      makeResource({ id: 'cron-1', type: 'k8s-cronjob' }),
      makeResource({ id: 'ing-1', type: 'k8s-ingress' }),
      makeResource({ id: 'eps-1', type: 'k8s-endpoint-slice' }),
      makeResource({ id: 'netpol-1', type: 'k8s-network-policy' }),
      makeResource({ id: 'pv-1', type: 'k8s-persistent-volume' }),
      makeResource({ id: 'pvc-1', type: 'k8s-persistent-volume-claim' }),
      makeResource({ id: 'sc-1', type: 'k8s-storage-class' }),
      makeResource({ id: 'ns-1', type: 'k8s-namespace' }),
      makeResource({ id: 'cm-1', type: 'k8s-configmap' }),
      makeResource({ id: 'secret-1', type: 'k8s-secret' }),
      makeResource({ id: 'sa-1', type: 'k8s-serviceaccount' }),
      makeResource({ id: 'quota-1', type: 'k8s-resource-quota' }),
      makeResource({ id: 'limits-1', type: 'k8s-limit-range' }),
      makeResource({ id: 'pdb-1', type: 'k8s-pod-disruption-budget' }),
      makeResource({ id: 'hpa-1', type: 'k8s-horizontal-pod-autoscaler' }),
      makeResource({ id: 'event-1', type: 'k8s-event' }),
      makeResource({ id: 'proxmox-vm', type: 'vm', platformType: 'proxmox-pve' }),
    ]);

    expect(model.clusters.map((r) => r.id)).toEqual(['cluster-1']);
    expect(model.nodes.map((r) => r.id)).toEqual(['node-1']);
    expect(model.pods.map((r) => r.id)).toEqual(['pod-1']);
    expect(model.deployments.map((r) => r.id)).toEqual(['dep-1']);
    expect(model.replicaSets.map((r) => r.id)).toEqual(['rs-1']);
    expect(model.services.map((r) => r.id)).toEqual(['svc-1']);
    expect(model.statefulSets.map((r) => r.id)).toEqual(['sts-1']);
    expect(model.daemonSets.map((r) => r.id)).toEqual(['ds-1']);
    expect(model.jobs.map((r) => r.id)).toEqual(['job-1']);
    expect(model.cronJobs.map((r) => r.id)).toEqual(['cron-1']);
    expect(model.ingresses.map((r) => r.id)).toEqual(['ing-1']);
    expect(model.endpointSlices.map((r) => r.id)).toEqual(['eps-1']);
    expect(model.networkPolicies.map((r) => r.id)).toEqual(['netpol-1']);
    expect(model.persistentVolumes.map((r) => r.id)).toEqual(['pv-1']);
    expect(model.persistentVolumeClaims.map((r) => r.id)).toEqual(['pvc-1']);
    expect(model.storageClasses.map((r) => r.id)).toEqual(['sc-1']);
    expect(model.namespaces.map((r) => r.id)).toEqual(['ns-1']);
    expect(model.configMaps.map((r) => r.id)).toEqual(['cm-1']);
    expect(model.secrets.map((r) => r.id)).toEqual(['secret-1']);
    expect(model.serviceAccounts.map((r) => r.id)).toEqual(['sa-1']);
    expect(model.resourceQuotas.map((r) => r.id)).toEqual(['quota-1']);
    expect(model.limitRanges.map((r) => r.id)).toEqual(['limits-1']);
    expect(model.podDisruptionBudgets.map((r) => r.id)).toEqual(['pdb-1']);
    expect(model.horizontalPodAutoscalers.map((r) => r.id)).toEqual(['hpa-1']);
    expect(model.events.map((r) => r.id)).toEqual(['event-1']);
    expect(model.workloads.map((r) => r.id).sort()).toEqual(
      ['cron-1', 'dep-1', 'ds-1', 'job-1', 'pod-1', 'rs-1', 'sts-1'].sort(),
    );
    expect(model.storage.map((r) => r.id).sort()).toEqual(['pv-1', 'pvc-1', 'sc-1']);
    expect(model.networking.map((r) => r.id).sort()).toEqual(['eps-1', 'ing-1', 'svc-1'].sort());
    expect(model.config.map((r) => r.id).sort()).toEqual(
      ['cm-1', 'ns-1', 'sa-1', 'secret-1'].sort(),
    );
    expect(model.policy.map((r) => r.id).sort()).toEqual(
      ['limits-1', 'netpol-1', 'pdb-1', 'quota-1'].sort(),
    );
    expect(model.autoscaling.map((r) => r.id)).toEqual(['hpa-1']);
    expect(model.resources).toHaveLength(25);
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
