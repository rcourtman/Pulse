import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
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

  it('owns dashboard resource deep-link selection without mutating workload filters', () => {
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
      }),
    );

    expect(result.selectedGuestId()).toBe('cluster-a:node-1:101');
    expect(resolveDashboardResourceSelection(locationSearch)).toEqual({
      resourceId: 'cluster-a:node-1:101',
    });
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

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
      }),
    );

    expect(result.selectedGuestId()).toBe('app-container:truenas-main:nextcloud');
    expect(resolveDashboardResourceSelection(locationSearch)).toEqual({
      resourceId: 'app-container:truenas-main:nextcloud',
    });
  });

  it('writes workload row selection back into the route state without dropping filters', () => {
    locationSearch = '?type=app-container&platform=truenas&agent=truenas-main';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
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

  it('shows a deliberate jump affordance when a hovered workload row is off-screen', () => {
    locationSearch = '?type=app-container&platform=truenas&agent=truenas-main';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([
      {
        id: 'app-container:truenas-main:nextcloud',
        name: 'nextcloud',
        status: 'running',
        instance: 'truenas-main',
        node: 'truenas-main',
      } as unknown as WorkloadGuest,
    ]);

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
      }),
    );

    const tableWrapper = document.createElement('div');
    const row = document.createElement('div');
    row.dataset.summarySeriesId = 'app-container:truenas-main:nextcloud';
    row.scrollIntoView = vi.fn();
    row.getBoundingClientRect = vi.fn(() => ({
      top: window.innerHeight + 120,
      bottom: window.innerHeight + 160,
      left: 0,
      right: 240,
      width: 240,
      height: 40,
      x: 0,
      y: window.innerHeight + 120,
      toJSON: () => ({}),
    })) as unknown as typeof row.getBoundingClientRect;
    tableWrapper.appendChild(row);
    document.body.appendChild(tableWrapper);

    result.setTableWrapperRef(tableWrapper as HTMLDivElement);
    result.setHoveredWorkloadId('app-container:truenas-main:nextcloud');

    expect(result.activeSummaryWorkloadId()).toBe('app-container:truenas-main:nextcloud');
    expect(result.shouldShowJumpToActiveWorkloadRow()).toBe(true);

    result.jumpToActiveWorkloadRow();

    expect(result.revealedGuestId()).toBe('app-container:truenas-main:nextcloud');
    expect(row.scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' });
  });

  it('reveals mounted workload detail after selecting a row', () => {
    locationSearch = '?type=app-container&platform=truenas&agent=truenas-main';
    const workloadId = 'app-container:truenas-main:nextcloud';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([
      {
        id: workloadId,
        name: 'nextcloud',
        status: 'running',
        instance: 'truenas-main',
        node: 'truenas-main',
      } as unknown as WorkloadGuest,
    ]);

    const scrollToSpy = vi.fn();
    Object.defineProperty(window, 'scrollY', {
      configurable: true,
      value: 180,
    });
    window.scrollTo = scrollToSpy as typeof window.scrollTo;

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
      }),
    );

    const tableWrapper = document.createElement('div');
    const row = document.createElement('div');
    row.dataset.summarySeriesId = workloadId;
    row.getBoundingClientRect = vi.fn(() => ({
      top: 624,
      bottom: 664,
      left: 0,
      right: 320,
      width: 320,
      height: 40,
      x: 0,
      y: 624,
      toJSON: () => ({}),
    })) as unknown as typeof row.getBoundingClientRect;

    const detail = document.createElement('div');
    detail.dataset.inlineDetailFor = workloadId;
    detail.getBoundingClientRect = vi.fn(() => ({
      top: 668,
      bottom: 964,
      left: 0,
      right: 320,
      width: 320,
      height: 296,
      x: 0,
      y: 668,
      toJSON: () => ({}),
    })) as unknown as typeof detail.getBoundingClientRect;

    tableWrapper.appendChild(row);
    tableWrapper.appendChild(detail);
    document.body.appendChild(tableWrapper);

    result.setTableWrapperRef(tableWrapper as HTMLDivElement);
    result.setSelectedGuestId(workloadId);
    vi.runAllTimers();

    expect(scrollToSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        behavior: 'smooth',
      }),
    );
  });

  it('tracks hovered workload groups without letting them override entity selection outside scope', () => {
    const [filteredGuests] = createSignal<WorkloadGuest[]>([
      {
        id: 'cluster-a:node-1:101',
        name: 'guest-1',
        status: 'running',
        instance: 'cluster-a',
        node: 'node-1',
        vmid: 101,
      } as unknown as WorkloadGuest,
      {
        id: 'cluster-b:node-2:202',
        name: 'guest-2',
        status: 'running',
        instance: 'cluster-b',
        node: 'node-2',
        vmid: 202,
      } as unknown as WorkloadGuest,
    ]);
    const groupScope: SummarySeriesGroupScope = {
      id: 'cluster-b',
      label: 'Cluster B (1 workload)',
      seriesIds: ['cluster-b:node-2:202'],
    };

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
      }),
    );

    expect(result.selectedGuestId()).toBe('cluster-a:node-1:101');
    result.setHoveredWorkloadGroupScope(groupScope);

    expect(result.activeSummaryWorkloadGroupScope()?.id).toBe('cluster-b');
    expect(result.activeSummaryWorkloadId()).toBeNull();
  });

  it('clears row focus without leaving behind an inferred agent filter', () => {
    locationSearch = '?resource=cluster-a:node-1:101';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useDashboardSelectionState({
        filteredGuests,
      }),
    );

    expect(result.selectedGuestId()).toBe('cluster-a:node-1:101');

    result.setSelectedGuestId(null);
    vi.runAllTimers();

    expect(navigateSpy).toHaveBeenCalledWith('/workloads', ROUTE_STATE_REPLACE_OPTIONS);
  });
});
