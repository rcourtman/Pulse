import { describe, expect, it } from 'vitest';

import {
  DASHBOARD_WORKLOAD_ROUTE_RESET_STATE,
  deserializeDashboardContainerRuntime,
  resolveDashboardWorkloadNodeSelection,
} from '../dashboardWorkloadRouteStateModel';

describe('dashboardWorkloadRouteStateModel', () => {
  it('deserializes persisted container runtime values through the canonical helper', () => {
    expect(deserializeDashboardContainerRuntime('docker')).toBe('docker');
    expect(deserializeDashboardContainerRuntime('')).toBe('');
    expect(deserializeDashboardContainerRuntime(null)).toBe('');
    expect(deserializeDashboardContainerRuntime(42)).toBe('');
  });

  it('exports the canonical workload-route reset state', () => {
    expect(DASHBOARD_WORKLOAD_ROUTE_RESET_STATE).toEqual({
      selectedNode: null,
      selectedHostHint: null,
      selectedKubernetesContext: null,
      selectedKubernetesNamespace: null,
      containerRuntime: '',
      viewMode: 'all',
    });
  });

  it('applies host-node selection only for pve or neutral route contexts', () => {
    expect(
      resolveDashboardWorkloadNodeSelection({
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
      resolveDashboardWorkloadNodeSelection({
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
