import { describe, expect, it } from 'vitest';

import type { WorkloadGuest } from '@/types/workloads';

import {
  dashboardHasHoveredWorkload,
  resolveDashboardResourceSelection,
  resolveDashboardSelectionNavigateTarget,
} from '../dashboardSelectionModel';

describe('dashboardSelectionModel', () => {
  it('resolves dashboard resource deep links into focused guest ids without inventing filters', () => {
    expect(resolveDashboardResourceSelection('?resource=cluster-a:node-1:101')).toEqual({
      resourceId: 'cluster-a:node-1:101',
    });
    expect(
      resolveDashboardResourceSelection(
        '?type=app-container&resource=app-container:truenas-main:nextcloud',
      ),
    ).toEqual({
      resourceId: 'app-container:truenas-main:nextcloud',
    });
    expect(
      resolveDashboardResourceSelection('?resource=app-container:docker-main:container-123'),
    ).toEqual({
      resourceId: 'app-container:docker-main:container-123',
    });
    expect(resolveDashboardResourceSelection('?resource=guest-1')).toEqual({
      resourceId: 'guest-1',
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

  it('builds route-backed workload selection targets without dropping other filters', () => {
    expect(
      resolveDashboardSelectionNavigateTarget({
        pathname: '/workloads',
        search: '?type=app-container&platform=truenas&agent=truenas-main',
        resourceId: 'app-container:truenas-main:nextcloud',
      }),
    ).toBe(
      '/workloads?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
    );

    expect(
      resolveDashboardSelectionNavigateTarget({
        pathname: '/workloads',
        search:
          '?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
        resourceId: null,
      }),
    ).toBe('/workloads?type=app-container&platform=truenas&agent=truenas-main');

    expect(
      resolveDashboardSelectionNavigateTarget({
        pathname: '/workloads',
        search:
          '?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
        resourceId: 'app-container:truenas-main:nextcloud',
      }),
    ).toBeNull();
  });
});
