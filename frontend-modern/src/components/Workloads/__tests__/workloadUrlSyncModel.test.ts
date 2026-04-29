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
        agent: '',
        resource: '',
      }),
    ).toBeNull();
  });

  it('only applies runtime params when the url semantics still resolve to app-container scope', () => {
    expect(
      resolveWorkloadsWorkloadRuntimeParam({
        type: 'docker',
        platform: '',
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
      resolveWorkloadsWorkloadRuntimeParam({
        type: 'vm',
        platform: '',
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
      resolveWorkloadsManagedWorkloadsNavigateTarget({
        currentSearch: '?resource=guest-1&type=vm&agent=node-a',
        viewMode: 'pod',
        containerRuntime: 'docker',
        selectedPlatform: 'kubernetes',
        selectedKubernetesContext: 'prod',
        selectedKubernetesNamespace: 'default',
        selectedNode: 'cluster-a-node-a',
        selectedHostHint: null,
      }),
    ).toBe('/workloads?resource=guest-1&type=pod&platform=kubernetes&context=prod&namespace=default');

    expect(
      resolveWorkloadsManagedWorkloadsNavigateTarget({
        currentSearch: '?type=pod&context=prod&namespace=default',
        viewMode: 'pod',
        containerRuntime: '',
        selectedPlatform: null,
        selectedKubernetesContext: 'prod',
        selectedKubernetesNamespace: 'default',
        selectedNode: null,
        selectedHostHint: null,
      }),
    ).toBeNull();
  });
});
