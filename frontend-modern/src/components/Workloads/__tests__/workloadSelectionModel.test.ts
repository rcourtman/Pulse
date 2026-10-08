import { describe, expect, it } from 'vitest';

import type { WorkloadGuest } from '@/types/workloads';

import {
  workloadsHasHoveredWorkload,
  resolveWorkloadResourceSelection,
} from '../workloadSelectionModel';

describe('workloadSelectionModel', () => {
  it('resolves workloads resource deep links into focused guest ids without inventing filters', () => {
    expect(resolveWorkloadResourceSelection('?resource=cluster-a:node-1:101')).toBe(
      'cluster-a:node-1:101',
    );
    expect(
      resolveWorkloadResourceSelection(
        '?type=app-container&resource=app-container:truenas-main:nextcloud',
      ),
    ).toBe('app-container:truenas-main:nextcloud');
    expect(
      resolveWorkloadResourceSelection('?resource=app-container:docker-main:container-123'),
    ).toBe('app-container:docker-main:container-123');
    expect(resolveWorkloadResourceSelection('?resource=guest-1')).toBe('guest-1');
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

  it('ignores summary group params, which no longer pin a workload group', () => {
    expect(resolveWorkloadResourceSelection('?summaryGroup=docker-host%3Atruenas-main')).toBeNull();
  });
});
