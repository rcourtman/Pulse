import { describe, expect, it } from 'vitest';
import {
  getResourceTypeLabel,
  getResourceTypePresentation,
} from '@/utils/resourceTypePresentation';

describe('resourceTypePresentation', () => {
  it('returns canonical labels for unified resource types', () => {
    expect(getResourceTypeLabel('docker-host')).toBe('Container Runtime');
    expect(getResourceTypeLabel('k8s-cluster')).toBe('K8s Cluster');
    expect(getResourceTypeLabel('network-endpoint')).toBe('Network Endpoint');
    expect(getResourceTypeLabel('network-share')).toBe('Network Share');
    expect(getResourceTypeLabel('truenas')).toBe('Agent');
    expect(getResourceTypeLabel('docker-image')).toBe('Image');
    expect(getResourceTypeLabel('docker-volume')).toBe('Volume');
    expect(getResourceTypeLabel('docker-network')).toBe('Network');
    expect(getResourceTypeLabel('docker-task')).toBe('Swarm Task');
    expect(getResourceTypeLabel('docker-swarm-node')).toBe('Swarm Node');
    expect(getResourceTypeLabel('k8s-service')).toBe('K8s Service');
    expect(getResourceTypeLabel('k8s-replicaset')).toBe('ReplicaSet');
    expect(getResourceTypeLabel('k8s-ingress')).toBe('Ingress');
    expect(getResourceTypeLabel('k8s-endpoint-slice')).toBe('EndpointSlice');
    expect(getResourceTypeLabel('k8s-network-policy')).toBe('NetworkPolicy');
    expect(getResourceTypeLabel('k8s-persistent-volume')).toBe('PV');
    expect(getResourceTypeLabel('k8s-persistent-volume-claim')).toBe('PVC');
    expect(getResourceTypeLabel('k8s-storage-class')).toBe('StorageClass');
    expect(getResourceTypeLabel('k8s-configmap')).toBe('ConfigMap');
    expect(getResourceTypeLabel('k8s-secret')).toBe('Secret');
    expect(getResourceTypeLabel('k8s-serviceaccount')).toBe('ServiceAccount');
    expect(getResourceTypeLabel('k8s-resource-quota')).toBe('ResourceQuota');
    expect(getResourceTypeLabel('k8s-limit-range')).toBe('LimitRange');
    expect(getResourceTypeLabel('k8s-pod-disruption-budget')).toBe('PodDisruptionBudget');
    expect(getResourceTypeLabel('k8s-horizontal-pod-autoscaler')).toBe('HPA');
    expect(getResourceTypeLabel('k8s-event')).toBe('Event');
  });

  it('returns shared presentations for external recovery subject aliases', () => {
    expect(getResourceTypePresentation('proxmox-vm')).toMatchObject({ label: 'VM' });
    expect(getResourceTypePresentation('proxmox-lxc')).toMatchObject({ label: 'LXC' });
    expect(getResourceTypePresentation('truenas-dataset')).toMatchObject({ label: 'Dataset' });
    expect(getResourceTypePresentation('docker-container')).toMatchObject({
      label: 'Container',
    });
  });
});
