import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { WorkloadGuest } from '@/types/workloads';
import { ROUTE_STATE_REPLACE_OPTIONS } from '@/utils/routeStateNavigation';

import { resolveDashboardResourceSelection } from '../dashboardSelectionModel';
import { useDashboardSelectionState } from '../useDashboardSelectionState';

let locationSearch = '?resource=cluster-a:node-1:101';
const navigateSpy = vi.fn();

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    pathname: '/workloads',
    get search() {
      return locationSearch;
    },
  }),
  useNavigate: () => navigateSpy,
}));

describe('useDashboardSelectionState', () => {
  beforeEach(() => {
    locationSearch = '?resource=cluster-a:node-1:101';
    navigateSpy.mockReset();
    vi.useFakeTimers();
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      cb(0);
      return 1;
    });
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
    document.body.innerHTML = '';
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
    expect(resolveDashboardResourceSelection(locationSearch)?.selectedNode).toBe(
      'cluster-a-node-1',
    );
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

  it('writes workload row selection back into the route state without dropping filters', () => {
    locationSearch = '?type=app-container&platform=truenas&agent=truenas-main';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
        setSelectedNode: vi.fn(),
      }),
    );

    result.setSelectedGuestId('app-container:truenas-main:nextcloud');
    vi.runAllTimers();

    expect(navigateSpy).toHaveBeenCalledWith(
      '/workloads?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
      ROUTE_STATE_REPLACE_OPTIONS,
    );
  });

  it('preserves the nearest scrollable ancestor when row focus changes locally', () => {
    locationSearch = '?type=app-container&platform=truenas&agent=truenas-main';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
        setSelectedNode: vi.fn(),
      }),
    );

    const scroller = document.createElement('div');
    scroller.style.overflowY = 'auto';
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      value: 400,
    });
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      value: 200,
    });
    scroller.scrollTop = 140;

    const tableWrapper = document.createElement('div');
    scroller.appendChild(tableWrapper);
    document.body.appendChild(scroller);

    result.setTableWrapperRef(tableWrapper as HTMLDivElement);
    result.setSelectedGuestId('app-container:truenas-main:nextcloud');

    expect(scroller.scrollTop).toBe(140);

    vi.runAllTimers();

    expect(navigateSpy).toHaveBeenCalledWith(
      '/workloads?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
      ROUTE_STATE_REPLACE_OPTIONS,
    );
  });
});
