import { describe, expect, it } from 'vitest';
import { toReportingResourceType } from '@/utils/reportingResourceTypes';

describe('toReportingResourceType', () => {
  it('keeps canonical v6 workload and infrastructure types unchanged where supported', () => {
    expect(toReportingResourceType('agent')).toBe('agent');
    expect(toReportingResourceType('vm')).toBe('vm');
    expect(toReportingResourceType('system-container')).toBe('system-container');
    expect(toReportingResourceType('app-container')).toBe('app-container');
    expect(toReportingResourceType('docker-host')).toBe('docker-host');
    expect(toReportingResourceType('docker-service')).toBe('app-container');
    expect(toReportingResourceType('docker-image')).toBe('app-container');
    expect(toReportingResourceType('docker-volume')).toBe('storage');
    expect(toReportingResourceType('docker-network')).toBe('network');
    expect(toReportingResourceType('docker-task')).toBe('app-container');
    expect(toReportingResourceType('docker-swarm-node')).toBe('app-container');
    expect(toReportingResourceType('network-endpoint')).toBe('network-endpoint');
    expect(toReportingResourceType('network-share')).toBe('network-share');
    expect(toReportingResourceType('storage')).toBe('storage');
  });

  it('adapts kubernetes resource kinds to the current reporting API token at the edge', () => {
    expect(toReportingResourceType('k8s-cluster')).toBe('k8s');
    expect(toReportingResourceType('k8s-node')).toBe('k8s');
    expect(toReportingResourceType('pod')).toBe('k8s');
    expect(toReportingResourceType('k8s-namespace')).toBe('k8s');
    expect(toReportingResourceType('k8s-service')).toBe('k8s');
    expect(toReportingResourceType('k8s-replicaset')).toBe('k8s');
    expect(toReportingResourceType('k8s-statefulset')).toBe('k8s');
    expect(toReportingResourceType('k8s-daemonset')).toBe('k8s');
    expect(toReportingResourceType('k8s-job')).toBe('k8s');
    expect(toReportingResourceType('k8s-cronjob')).toBe('k8s');
    expect(toReportingResourceType('k8s-ingress')).toBe('k8s');
    expect(toReportingResourceType('k8s-endpoint-slice')).toBe('k8s');
    expect(toReportingResourceType('k8s-network-policy')).toBe('k8s');
    expect(toReportingResourceType('k8s-persistent-volume')).toBe('k8s');
    expect(toReportingResourceType('k8s-persistent-volume-claim')).toBe('k8s');
    expect(toReportingResourceType('k8s-storage-class')).toBe('k8s');
    expect(toReportingResourceType('k8s-configmap')).toBe('k8s');
    expect(toReportingResourceType('k8s-secret')).toBe('k8s');
    expect(toReportingResourceType('k8s-serviceaccount')).toBe('k8s');
    expect(toReportingResourceType('k8s-resource-quota')).toBe('k8s');
    expect(toReportingResourceType('k8s-limit-range')).toBe('k8s');
    expect(toReportingResourceType('k8s-pod-disruption-budget')).toBe('k8s');
    expect(toReportingResourceType('k8s-horizontal-pod-autoscaler')).toBe('k8s');
    expect(toReportingResourceType('k8s-event')).toBe('k8s');
  });
});
