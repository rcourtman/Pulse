import { describe, expect, it } from 'vitest';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

describe('resourceTypeCompat', () => {
  it('canonicalizes the small frontend compatibility alias set', () => {
    expect(canonicalizeFrontendResourceType('host')).toBe('agent');
    expect(canonicalizeFrontendResourceType('docker')).toBe('app-container');
    expect(canonicalizeFrontendResourceType('docker_host')).toBe('docker-host');
    expect(canonicalizeFrontendResourceType('k8s')).toBe('pod');
    expect(canonicalizeFrontendResourceType('kubernetes')).toBe('pod');
    expect(canonicalizeFrontendResourceType('kubernetes_cluster')).toBe('k8s-cluster');
    expect(canonicalizeFrontendResourceType('kubernetes-node')).toBe('k8s-node');
    expect(canonicalizeFrontendResourceType('ceph')).toBe('ceph');
    expect(canonicalizeFrontendResourceType('truenas')).toBe('agent');
    expect(canonicalizeFrontendResourceType('availability')).toBe('network-endpoint');
    expect(canonicalizeFrontendResourceType('network_endpoint')).toBe('network-endpoint');
    expect(canonicalizeFrontendResourceType('network-share')).toBe('network-share');
    expect(canonicalizeFrontendResourceType('network')).toBe('network');
    expect(canonicalizeFrontendResourceType('docker-image')).toBe('docker-image');
    expect(canonicalizeFrontendResourceType('docker-volume')).toBe('docker-volume');
    expect(canonicalizeFrontendResourceType('docker-network')).toBe('docker-network');
    expect(canonicalizeFrontendResourceType('docker-task')).toBe('docker-task');
    expect(canonicalizeFrontendResourceType('docker-swarm-node')).toBe('docker-swarm-node');
    expect(canonicalizeFrontendResourceType('docker-node')).toBe('docker-swarm-node');
    expect(canonicalizeFrontendResourceType('swarm_node')).toBe('docker-swarm-node');
    expect(canonicalizeFrontendResourceType('k8s-namespace')).toBe('k8s-namespace');
    expect(canonicalizeFrontendResourceType('k8s-service')).toBe('k8s-service');
    expect(canonicalizeFrontendResourceType('k8s-replicaset')).toBe('k8s-replicaset');
    expect(canonicalizeFrontendResourceType('k8s-statefulset')).toBe('k8s-statefulset');
    expect(canonicalizeFrontendResourceType('k8s-daemonset')).toBe('k8s-daemonset');
    expect(canonicalizeFrontendResourceType('k8s-job')).toBe('k8s-job');
    expect(canonicalizeFrontendResourceType('k8s-cronjob')).toBe('k8s-cronjob');
    expect(canonicalizeFrontendResourceType('k8s-ingress')).toBe('k8s-ingress');
    expect(canonicalizeFrontendResourceType('k8s-endpoint-slice')).toBe('k8s-endpoint-slice');
    expect(canonicalizeFrontendResourceType('k8s-network-policy')).toBe('k8s-network-policy');
    expect(canonicalizeFrontendResourceType('k8s-persistent-volume')).toBe('k8s-persistent-volume');
    expect(canonicalizeFrontendResourceType('k8s-persistent-volume-claim')).toBe(
      'k8s-persistent-volume-claim',
    );
    expect(canonicalizeFrontendResourceType('k8s-storage-class')).toBe('k8s-storage-class');
    expect(canonicalizeFrontendResourceType('k8s-configmap')).toBe('k8s-configmap');
    expect(canonicalizeFrontendResourceType('k8s-secret')).toBe('k8s-secret');
    expect(canonicalizeFrontendResourceType('k8s-serviceaccount')).toBe('k8s-serviceaccount');
    expect(canonicalizeFrontendResourceType('k8s-resource-quota')).toBe('k8s-resource-quota');
    expect(canonicalizeFrontendResourceType('k8s-limit-range')).toBe('k8s-limit-range');
    expect(canonicalizeFrontendResourceType('k8s-pod-disruption-budget')).toBe(
      'k8s-pod-disruption-budget',
    );
    expect(canonicalizeFrontendResourceType('k8s-horizontal-pod-autoscaler')).toBe(
      'k8s-horizontal-pod-autoscaler',
    );
    expect(canonicalizeFrontendResourceType('k8s-event')).toBe('k8s-event');
  });

  it('does not silently canonicalize removed non-canonical workload aliases', () => {
    expect(canonicalizeFrontendResourceType('docker-container')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('docker_service')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('container')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('qemu')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('lxc')).toBeUndefined();
  });
});
