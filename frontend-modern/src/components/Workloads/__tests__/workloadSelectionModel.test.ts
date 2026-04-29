import { describe, expect, it } from 'vitest';

import type { WorkloadGuest } from '@/types/workloads';

import {
  workloadsHasHoveredWorkload,
  resolveWorkloadResourceSelection,
  resolveWorkloadsSelectionNavigateTarget,
} from '../workloadSelectionModel';

describe('workloadSelectionModel', () => {
  it('resolves workloads resource deep links into focused guest ids without inventing filters', () => {
    expect(resolveWorkloadResourceSelection('?resource=cluster-a:node-1:101')).toEqual({
      resourceId: 'cluster-a:node-1:101',
      summaryGroupId: null,
    });
    expect(
      resolveWorkloadResourceSelection(
        '?type=app-container&resource=app-container:truenas-main:nextcloud',
      ),
    ).toEqual({
      resourceId: 'app-container:truenas-main:nextcloud',
      summaryGroupId: null,
    });
    expect(
      resolveWorkloadResourceSelection('?resource=app-container:docker-main:container-123'),
    ).toEqual({
      resourceId: 'app-container:docker-main:container-123',
      summaryGroupId: null,
    });
    expect(resolveWorkloadResourceSelection('?resource=guest-1')).toEqual({
      resourceId: 'guest-1',
      summaryGroupId: null,
    });
    expect(resolveWorkloadResourceSelection('')).toBeNull();
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

    expect(workloadsHasHoveredWorkload(guests, 'cluster-a:node-1:101')).toBe(true);
    expect(workloadsHasHoveredWorkload(guests, 'cluster-a:node-1:102')).toBe(false);
  });

  it('builds route-backed workload selection targets without dropping other filters', () => {
    expect(
      resolveWorkloadsSelectionNavigateTarget({
        pathname: '/workloads',
        search: '?type=app-container&platform=truenas&agent=truenas-main',
        resourceId: 'app-container:truenas-main:nextcloud',
        summaryGroupId: null,
      }),
    ).toBe(
      '/workloads?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
    );

    expect(
      resolveWorkloadsSelectionNavigateTarget({
        pathname: '/workloads',
        search:
          '?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
        resourceId: null,
        summaryGroupId: null,
      }),
    ).toBe('/workloads?type=app-container&platform=truenas&agent=truenas-main');

    expect(
      resolveWorkloadsSelectionNavigateTarget({
        pathname: '/workloads',
        search:
          '?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
        resourceId: 'app-container:truenas-main:nextcloud',
        summaryGroupId: null,
      }),
    ).toBeNull();

    expect(
      resolveWorkloadsSelectionNavigateTarget({
        pathname: '/workloads',
        search: '?type=app-container&platform=truenas&agent=truenas-main',
        resourceId: null,
        summaryGroupId: 'docker-host:truenas-main',
      }),
    ).toBe(
      '/workloads?type=app-container&platform=truenas&agent=truenas-main&summaryGroup=docker-host%3Atruenas-main',
    );

    expect(
      resolveWorkloadsSelectionNavigateTarget({
        pathname: '/workloads',
        search:
          '?type=app-container&platform=truenas&agent=truenas-main&summaryGroup=docker-host%3Atruenas-main',
        resourceId: null,
        summaryGroupId: 'docker-host:truenas-main',
      }),
    ).toBeNull();
  });
});
