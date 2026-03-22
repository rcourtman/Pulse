import { describe, expect, it } from 'vitest';

import type { WorkloadGuest } from '@/types/workloads';

import {
  dashboardHasHoveredWorkload,
  resolveDashboardResourceSelection,
} from '../dashboardSelectionModel';

describe('dashboardSelectionModel', () => {
  it('resolves dashboard resource deep links into selected guest and node scope', () => {
    expect(resolveDashboardResourceSelection('?resource=cluster-a:node-1:101')).toEqual({
      resourceId: 'cluster-a:node-1:101',
      selectedNode: 'cluster-a-node-1',
    });
    expect(resolveDashboardResourceSelection('?resource=guest-1')).toEqual({
      resourceId: 'guest-1',
      selectedNode: null,
    });
    expect(resolveDashboardResourceSelection('')).toBeNull();
  });

  it('checks hovered workload continuity against canonical workload ids', () => {
    const guests = [
      {
        id: 'cluster-a:node-1:101',
        name: 'guest-1',
        status: 'running',
        instance: 'cluster-a',
        node: 'node-1',
        vmid: 101,
      } as unknown as WorkloadGuest,
    ];

    expect(dashboardHasHoveredWorkload(guests, 'cluster-a:node-1:101')).toBe(true);
    expect(dashboardHasHoveredWorkload(guests, 'cluster-a:node-1:102')).toBe(false);
  });
});
