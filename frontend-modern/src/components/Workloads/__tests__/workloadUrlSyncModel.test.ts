import { describe, expect, it } from 'vitest';

import {
  parseWorkloadsWorkloadUrlParams,
  resolveWorkloadsManagedWorkloadsNavigateTarget,
  resolveWorkloadsWorkloadRuntimeParam,
  resolveWorkloadsWorkloadTypeParam,
} from '../workloadUrlSyncModel';

describe('workloadUrlSyncModel', () => {
  it('parses canonical workload route params and keeps resource deep links intact', () => {
    expect(
      parseWorkloadsWorkloadUrlParams(
        '?type=docker&platform=truenas&runtime=containerd&context=prod&namespace=default&agent=node-a&resource=guest-1',
      ),
    ).toEqual({
      type: 'docker',
      platform: 'truenas',
      runtime: 'containerd',
      context: 'prod',
      namespace: 'default',
      cluster: '',
      agent: 'node-a',
      resource: 'guest-1',
      summaryGroup: '',
    });
  });

  it('resolves workload type params through the canonical alias rules and k8s precedence', () => {
    expect(
      resolveWorkloadsWorkloadTypeParam({
        type: 'docker',
        platform: '',
        runtime: '',
        context: '',
        namespace: '',
        cluster: '',
        agent: '',
        resource: '',
      }),
    ).toBe('app-container');
    expect(
      resolveWorkloadsWorkloadTypeParam({
        type: 'vm',
        platform: '',
        runtime: '',
        context: 'prod',
        namespace: '',
        cluster: '',
        agent: '',
        resource: '',
      }),
    ).toBeNull();
  });

  it('only applies runtime params when the url semantics still resolve to container scope', () => {
    expect(
      resolveWorkloadsWorkloadRuntimeParam({
        type: 'docker',
        platform: '',
        runtime: 'containerd',
        context: '',
        namespace: '',
        cluster: '',
        agent: '',
        resource: '',
      }),
    ).toEqual({
      forceViewMode: 'container',
      runtime: 'containerd',
      shouldApply: true,
    });

    expect(
      resolveWorkloadsWorkloadRuntimeParam({
        type: 'vm',
        platform: '',
        runtime: 'containerd',
        context: 'prod',
        namespace: '',
        cluster: '',
        agent: '',
        resource: '',
      }),
    ).toEqual({
      forceViewMode: null,
      runtime: 'containerd',
      shouldApply: false,
    });
  });

  it('builds managed workload navigate targets without dropping unrelated resource params', () => {
    expect(
      resolveWorkloadsManagedWorkloadsNavigateTarget({
        currentPathname: '/proxmox/overview',
        currentSearch: '?resource=guest-1&status=running&type=vm&agent=node-a',
        viewMode: 'pod',
        effectiveViewMode: 'pod',
        containerRuntime: 'docker',
        selectedPlatform: 'kubernetes',
        selectedKubernetesContext: 'prod',
        selectedKubernetesNamespace: 'default',
        selectedCluster: null,
        selectedNode: 'cluster-a-node-a',
        selectedHostHint: null,
      }),
    ).toBe(
      '/proxmox/overview?resource=guest-1&status=running&type=pod&platform=kubernetes&context=prod&namespace=default',
    );

    expect(
      resolveWorkloadsManagedWorkloadsNavigateTarget({
        currentPathname: '/kubernetes/workloads',
        currentSearch: '?type=pod&context=prod&namespace=default',
        viewMode: 'pod',
        effectiveViewMode: 'pod',
        containerRuntime: '',
        selectedPlatform: null,
        selectedKubernetesContext: 'prod',
        selectedKubernetesNamespace: 'default',
        selectedCluster: null,
        selectedNode: null,
        selectedHostHint: null,
      }),
    ).toBeNull();
  });

  it('serializes the cluster param off the effective view mode (vSphere forces vm while raw stays all)', () => {
    // vSphere: raw viewMode stays 'all' (forcedViewMode drives the filter), so
    // the cluster param must key off effectiveViewMode to persist to the URL.
    expect(
      resolveWorkloadsManagedWorkloadsNavigateTarget({
        currentPathname: '/vmware',
        currentSearch: '',
        viewMode: 'all',
        effectiveViewMode: 'vm',
        containerRuntime: '',
        selectedPlatform: null,
        selectedKubernetesContext: null,
        selectedKubernetesNamespace: null,
        selectedCluster: 'Production Cluster',
        selectedNode: null,
        selectedHostHint: null,
      }),
    ).toBe('/vmware?cluster=Production+Cluster');

    // Cluster is not serialized when the effective view mode is not vm.
    const nonVm = resolveWorkloadsManagedWorkloadsNavigateTarget({
      currentPathname: '/kubernetes/workloads',
      currentSearch: '?type=pod',
      viewMode: 'pod',
      effectiveViewMode: 'pod',
      containerRuntime: '',
      selectedPlatform: null,
      selectedKubernetesContext: null,
      selectedKubernetesNamespace: null,
      selectedCluster: 'Production Cluster',
      selectedNode: null,
      selectedHostHint: null,
    });
    expect(nonVm === null || !nonVm.includes('cluster')).toBe(true);
  });
});
