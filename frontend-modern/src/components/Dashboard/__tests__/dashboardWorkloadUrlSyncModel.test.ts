import { describe, expect, it } from 'vitest';

import {
  parseDashboardWorkloadUrlParams,
  resolveDashboardManagedWorkloadsNavigateTarget,
  resolveDashboardWorkloadRuntimeParam,
  resolveDashboardWorkloadTypeParam,
} from '../dashboardWorkloadUrlSyncModel';

describe('dashboardWorkloadUrlSyncModel', () => {
  it('parses canonical workload route params and keeps resource deep links intact', () => {
    expect(
      parseDashboardWorkloadUrlParams(
        '?type=docker&runtime=containerd&context=prod&namespace=default&agent=node-a&resource=guest-1',
      ),
    ).toEqual({
      type: 'docker',
      runtime: 'containerd',
      context: 'prod',
      namespace: 'default',
      agent: 'node-a',
      resource: 'guest-1',
    });
  });

  it('resolves workload type params through the canonical alias rules and k8s precedence', () => {
    expect(
      resolveDashboardWorkloadTypeParam({
        type: 'docker',
        runtime: '',
        context: '',
        namespace: '',
        agent: '',
        resource: '',
      }),
    ).toBe('app-container');
    expect(
      resolveDashboardWorkloadTypeParam({
        type: 'vm',
        runtime: '',
        context: 'prod',
        namespace: '',
        agent: '',
        resource: '',
      }),
    ).toBeNull();
  });

  it('only applies runtime params when the url semantics still resolve to app-container scope', () => {
    expect(
      resolveDashboardWorkloadRuntimeParam({
        type: 'docker',
        runtime: 'containerd',
        context: '',
        namespace: '',
        agent: '',
        resource: '',
      }),
    ).toEqual({
      forceViewMode: 'app-container',
      runtime: 'containerd',
      shouldApply: true,
    });

    expect(
      resolveDashboardWorkloadRuntimeParam({
        type: 'vm',
        runtime: 'containerd',
        context: 'prod',
        namespace: '',
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
      resolveDashboardManagedWorkloadsNavigateTarget({
        currentSearch: '?resource=guest-1&type=vm&agent=node-a',
        viewMode: 'pod',
        containerRuntime: 'docker',
        selectedKubernetesContext: 'prod',
        selectedKubernetesNamespace: 'default',
        selectedNode: 'cluster-a-node-a',
        selectedHostHint: null,
      }),
    ).toBe('/workloads?resource=guest-1&type=pod&context=prod&namespace=default');

    expect(
      resolveDashboardManagedWorkloadsNavigateTarget({
        currentSearch: '?type=pod&context=prod&namespace=default',
        viewMode: 'pod',
        containerRuntime: '',
        selectedKubernetesContext: 'prod',
        selectedKubernetesNamespace: 'default',
        selectedNode: null,
        selectedHostHint: null,
      }),
    ).toBeNull();
  });
});
