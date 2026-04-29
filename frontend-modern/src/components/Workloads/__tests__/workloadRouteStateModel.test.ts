import { describe, expect, it } from 'vitest';

import {
  WORKLOADS_WORKLOAD_ROUTE_RESET_STATE,
  deserializeWorkloadsContainerRuntime,
  resolveWorkloadsWorkloadNodeSelection,
} from '../workloadRouteStateModel';

describe('workloadRouteStateModel', () => {
  it('deserializes persisted container runtime values through the canonical helper', () => {
    expect(deserializeWorkloadsContainerRuntime('docker')).toBe('docker');
    expect(deserializeWorkloadsContainerRuntime('')).toBe('');
    expect(deserializeWorkloadsContainerRuntime(null)).toBe('');
    expect(deserializeWorkloadsContainerRuntime(42)).toBe('');
  });

  it('exports the canonical workload-route reset state', () => {
    expect(WORKLOADS_WORKLOAD_ROUTE_RESET_STATE).toEqual({
      selectedNode: null,
      selectedHostHint: null,
      selectedPlatform: null,
      selectedKubernetesContext: null,
      selectedKubernetesNamespace: null,
      containerRuntime: '',
      viewMode: 'all',
    });
  });

  it('applies host-node selection only for pve or neutral route contexts', () => {
    expect(
      resolveWorkloadsWorkloadNodeSelection({
        nodeId: 'cluster-a-node-a',
        nodeType: 'pve',
        showFilters: false,
      }),
    ).toEqual({
      selectedNode: 'cluster-a-node-a',
      selectedHostHint: null,
      shouldApply: true,
      shouldShowFilters: true,
    });

    expect(
      resolveWorkloadsWorkloadNodeSelection({
        nodeId: 'cluster-a-node-a',
        nodeType: 'pbs',
        showFilters: false,
      }),
    ).toEqual({
      selectedNode: 'cluster-a-node-a',
      selectedHostHint: null,
      shouldApply: false,
      shouldShowFilters: true,
    });
  });
});
