import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { WorkloadGuest } from '@/types/workloads';

import { resolveDashboardResourceSelection } from '../dashboardSelectionModel';
import { useDashboardSelectionState } from '../useDashboardSelectionState';

let locationSearch = '?resource=cluster-a:node-1:101';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    pathname: '/workloads',
    get search() {
      return locationSearch;
    },
  }),
}));

describe('useDashboardSelectionState', () => {
  beforeEach(() => {
    locationSearch = '?resource=cluster-a:node-1:101';
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      cb(0);
      return 1;
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('owns dashboard resource deep-link selection and node synchronization', () => {
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);
    const setSelectedNode = vi.fn();

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
        setSelectedNode,
      }),
    );

    expect(result.selectedGuestId()).toBe('cluster-a:node-1:101');
    expect(setSelectedNode).toHaveBeenCalledWith('cluster-a-node-1');
    expect(resolveDashboardResourceSelection(locationSearch)?.selectedNode).toBe('cluster-a-node-1');
  });

  it('clears stale hovered workload ids when filtered guests change', () => {
    const guest = {
      id: 'cluster-a:node-1:101',
      name: 'guest-1',
      status: 'running',
      instance: 'cluster-a',
      node: 'node-1',
      vmid: 101,
    } as unknown as WorkloadGuest;
    const [filteredGuests, setFilteredGuests] = createSignal<WorkloadGuest[]>([guest]);

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
        setSelectedNode: vi.fn(),
      }),
    );

    result.setHoveredWorkloadId('cluster-a:node-1:101');
    expect(result.hoveredWorkloadId()).toBe('cluster-a:node-1:101');

    setFilteredGuests([]);
    expect(result.hoveredWorkloadId()).toBeNull();
  });

  it('does not invent node filters for canonical app-container deep links', () => {
    locationSearch = '?type=app-container&resource=app-container:truenas-main:nextcloud';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);
    const setSelectedNode = vi.fn();

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
        setSelectedNode,
      }),
    );

    expect(result.selectedGuestId()).toBe('app-container:truenas-main:nextcloud');
    expect(setSelectedNode).not.toHaveBeenCalled();
    expect(resolveDashboardResourceSelection(locationSearch)?.selectedNode).toBeNull();
  });
});
