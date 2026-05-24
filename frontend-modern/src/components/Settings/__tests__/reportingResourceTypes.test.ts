import { describe, expect, it } from 'vitest';
import { toReportingResourceType } from '@/utils/reportingResourceTypes';

describe('toReportingResourceType', () => {
  it('keeps canonical v6 workload and infrastructure types unchanged where supported', () => {
    expect(toReportingResourceType('agent')).toBe('agent');
    expect(toReportingResourceType('vm')).toBe('vm');
    expect(toReportingResourceType('system-container')).toBe('system-container');
    expect(toReportingResourceType('app-container')).toBe('app-container');
    expect(toReportingResourceType('docker-host')).toBe('docker-host');
    expect(toReportingResourceType('network-endpoint')).toBe('network-endpoint');
    expect(toReportingResourceType('storage')).toBe('storage');
    expect(toReportingResourceType('network')).toBe('network');
  });

  it('adapts kubernetes resource kinds to the current reporting API token at the edge', () => {
    expect(toReportingResourceType('k8s-cluster')).toBe('k8s');
    expect(toReportingResourceType('k8s-node')).toBe('k8s');
    expect(toReportingResourceType('pod')).toBe('k8s');
    expect(toReportingResourceType('k8s-replicaset')).toBe('k8s');
    expect(toReportingResourceType('k8s-endpoint-slice')).toBe('k8s');
    expect(toReportingResourceType('k8s-network-policy')).toBe('k8s');
    expect(toReportingResourceType('k8s-storage-class')).toBe('k8s');
    expect(toReportingResourceType('k8s-configmap')).toBe('k8s');
    expect(toReportingResourceType('k8s-serviceaccount')).toBe('k8s');
  });
});
