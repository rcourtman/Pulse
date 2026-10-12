import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ResourceAPI } from '@/api/resources';
import type { Resource } from '@/types/resource';
import type { WorkloadGuest } from '@/types/workloads';
import {
  clearPendingAppShellRestoreTop,
  readPendingAppShellRestoreTop,
} from '@/utils/appShellScrollRestoration';
import {
  ROUTE_STATE_REPLACE_OPTIONS,
  createRouteStateNavigateScheduler,
} from '@/utils/routeStateNavigation';

import { resolveWorkloadResourceSelection } from '../workloadSelectionModel';
import { useWorkloadSelectionState } from '../useWorkloadSelectionState';

let locationSearch = '?resource=cluster-a:node-1:101';
let readLocationSearch = () => locationSearch;
vi.mock('@/api/resources', () => ({ ResourceAPI: { getResource: vi.fn() } }));
const navigateSpy = vi.fn();

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    pathname: '/proxmox/overview',
    get search() {
      return readLocationSearch();
    },
  }),
  useNavigate: () => navigateSpy,
}));

describe('useWorkloadSelectionState', () => {
  beforeEach(() => {
    locationSearch = '?resource=cluster-a:node-1:101';
    readLocationSearch = () => locationSearch;
    vi.mocked(ResourceAPI.getResource)
      .mockReset()
      .mockImplementation(async (id) => ({ id }) as Resource);
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

  it('owns workloads resource deep-link selection without mutating workload filters', () => {
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useWorkloadSelectionState({
        filteredGuests,
      }),
    );

    expect(result.selectedGuestId()).toBe('cluster-a:node-1:101');
    expect(resolveWorkloadResourceSelection(locationSearch)).toBe('cluster-a:node-1:101');
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
      useWorkloadSelectionState({
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
      useWorkloadSelectionState({
        filteredGuests,
      }),
    );

    expect(result.selectedGuestId()).toBe('app-container:truenas-main:nextcloud');
    expect(resolveWorkloadResourceSelection(locationSearch)).toBe(
      'app-container:truenas-main:nextcloud',
    );
  });

  it('opens workload row selection locally without route navigation', () => {
    locationSearch = '?type=app-container&platform=truenas&agent=truenas-main';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useWorkloadSelectionState({
        filteredGuests,
      }),
    );

    result.setSelectedGuestId('app-container:truenas-main:nextcloud');
    vi.runAllTimers();

    expect(result.selectedGuestId()).toBe('app-container:truenas-main:nextcloud');
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('clears the pinned workload row locally without mutating route filters', () => {
    locationSearch =
      '?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useWorkloadSelectionState({
        filteredGuests,
      }),
    );

    expect(result.selectedGuestId()).toBe('app-container:truenas-main:nextcloud');
    result.clearPinnedSummaryScope();
    vi.runAllTimers();

    expect(result.selectedGuestId()).toBeNull();
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('routes Escape through workload scope clearing and additional page reset work', () => {
    locationSearch =
      '?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);
    const clearAdditionalPageStateOnEscape = vi.fn();

    renderHook(() =>
      useWorkloadSelectionState({
        clearAdditionalPageStateOnEscape,
        filteredGuests,
      }),
    );

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    vi.runAllTimers();

    expect(clearAdditionalPageStateOnEscape).toHaveBeenCalledTimes(1);
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('preserves the nearest scrollable ancestor when row focus changes locally', () => {
    locationSearch = '?type=app-container&platform=truenas&agent=truenas-main';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useWorkloadSelectionState({
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

    expect(result.selectedGuestId()).toBe('app-container:truenas-main:nextcloud');
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('smart-reveals mounted workload detail when the row itself triggers focus near the fold', () => {
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
      useWorkloadSelectionState({
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
    result.setTableRootRef(tableWrapper as HTMLDivElement);
    result.setSelectedGuestId(workloadId);
    vi.runAllTimers();

    expect(
      scrollToSpy.mock.calls.some(
        ([firstArg]) =>
          Boolean(firstArg) &&
          typeof firstArg === 'object' &&
          'behavior' in firstArg &&
          firstArg.behavior === 'smooth',
      ),
    ).toBe(true);
  });

  it('ignores a summaryGroup param because grouped rows no longer pin a group scope', () => {
    locationSearch = '?summaryGroup=cluster-b&resource=cluster-a:node-1:101';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([
      {
        id: 'cluster-a:node-1:101',
        name: 'guest-1',
        status: 'running',
        instance: 'cluster-a',
        node: 'node-1',
        vmid: 101,
      } as unknown as WorkloadGuest,
    ]);

    const { result } = renderHook(() =>
      useWorkloadSelectionState({
        filteredGuests,
      }),
    );
    vi.runAllTimers();

    expect(result.selectedGuestId()).toBe('cluster-a:node-1:101');
    expect(result.activeSummaryWorkloadId()).toBe('cluster-a:node-1:101');
    expect(Object.keys(result).filter((key) => /group/i.test(key))).toEqual([]);
  });

  it('clears row focus locally without leaving behind an inferred agent filter', () => {
    locationSearch = '?resource=cluster-a:node-1:101';
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useWorkloadSelectionState({
        filteredGuests,
      }),
    );

    expect(result.selectedGuestId()).toBe('cluster-a:node-1:101');

    result.setSelectedGuestId(null);
    vi.runAllTimers();

    expect(result.selectedGuestId()).toBeNull();
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('stages no app-shell restore for a local row toggle that a later route change could replay', () => {
    locationSearch = '';
    clearPendingAppShellRestoreTop();
    const shell = document.createElement('div');
    shell.className = 'app-scroll-shell';
    shell.scrollTop = 3000;
    document.body.appendChild(shell);
    const [filteredGuests] = createSignal<WorkloadGuest[]>([]);

    const { result } = renderHook(() =>
      useWorkloadSelectionState({
        filteredGuests,
      }),
    );

    result.setSelectedGuestId('cluster-a:node-1:101');
    vi.runAllTimers();
    expect(result.selectedGuestId()).toBe('cluster-a:node-1:101');
    expect(readPendingAppShellRestoreTop()).toBeNull();

    result.setSelectedGuestId(null);
    vi.runAllTimers();
    expect(readPendingAppShellRestoreTop()).toBeNull();

    // Back at the top, a filter change writes the URL. The scheduler stages a
    // restore only from a scrolled shell, so App.tsx would replay anything the
    // row toggle had staged and jump the page back to 3000.
    shell.scrollTop = 0;
    const navigate = vi.fn();
    const scheduler = createRouteStateNavigateScheduler(navigate, () => '/proxmox/overview');
    scheduler.schedule('/proxmox/overview?type=vm');
    vi.runAllTimers();

    expect(navigate).toHaveBeenCalledWith('/proxmox/overview?type=vm', ROUTE_STATE_REPLACE_OPTIONS);
    expect(readPendingAppShellRestoreTop()).toBeNull();
    expect(navigateSpy).not.toHaveBeenCalled();
  });
  it('resolves an old node-scoped link through backend identity ownership', async () => {
    vi.mocked(ResourceAPI.getResource).mockResolvedValueOnce({ id: 'stable-resource' } as Resource);
    const { result } = renderHook(() => useWorkloadSelectionState({ filteredGuests: () => [] }));
    await Promise.resolve();
    await Promise.resolve();
    expect(result.selectedGuestId()).toBe('stable-resource');
    expect(ResourceAPI.getResource).toHaveBeenCalledWith('cluster-a:node-1:101', {
      signal: expect.any(AbortSignal),
    });
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('keeps exact current identity local without an alias lookup', () => {
    locationSearch = '?resource=stable-resource';
    const guest = {
      id: 'stable-resource',
      canonicalResourceId: 'stable-resource',
    } as WorkloadGuest;
    const { result } = renderHook(() =>
      useWorkloadSelectionState({ filteredGuests: () => [guest] }),
    );
    expect(result.selectedGuestId()).toBe('stable-resource');
    expect(ResourceAPI.getResource).not.toHaveBeenCalled();
  });

  it.each(['close', 'reselect', 'escape'] as const)(
    'does not replay a pending link after %s',
    async (interaction) => {
      let finish!: (resource: Resource) => void;
      vi.mocked(ResourceAPI.getResource).mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finish = resolve;
          }),
      );
      const { result } = renderHook(() => useWorkloadSelectionState({ filteredGuests: () => [] }));
      const signal = vi.mocked(ResourceAPI.getResource).mock.calls[0]![1]!.signal!;
      if (interaction === 'close') result.clearPinnedSummaryScope();
      else if (interaction === 'reselect') result.setSelectedGuestId('independent-clone');
      else document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
      expect(signal.aborted).toBe(true);
      finish({ id: 'stable-resource' } as Resource);
      await Promise.resolve();
      await Promise.resolve();
      expect(result.selectedGuestId()).toBe(
        interaction === 'reselect' ? 'independent-clone' : null,
      );
    },
  );

  it('aborts a superseded route lookup and accepts only its current reference', async () => {
    const [search, setSearch] = createSignal('?resource=old-reference');
    readLocationSearch = search;
    let finishOld!: (resource: Resource) => void;
    vi.mocked(ResourceAPI.getResource)
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finishOld = resolve;
          }),
      )
      .mockResolvedValueOnce({ id: 'new-resource' } as Resource);
    const { result } = renderHook(() => useWorkloadSelectionState({ filteredGuests: () => [] }));
    const oldSignal = vi.mocked(ResourceAPI.getResource).mock.calls[0]![1]!.signal!;
    setSearch('?resource=new-reference');
    await Promise.resolve();
    await Promise.resolve();
    finishOld({ id: 'old-resource' } as Resource);
    await Promise.resolve();
    expect(oldSignal.aborted).toBe(true);
    expect(result.selectedGuestId()).toBe('new-resource');
  });

  it('does not resolve references while route state is disabled', () => {
    const { result } = renderHook(() =>
      useWorkloadSelectionState({ filteredGuests: () => [], routeStateEnabled: () => false }),
    );
    expect(result.selectedGuestId()).toBeNull();
    expect(ResourceAPI.getResource).not.toHaveBeenCalled();
  });

  it('leaves an ambiguous or refused reference unresolved', async () => {
    vi.mocked(ResourceAPI.getResource).mockRejectedValueOnce(new Error('not found'));
    const { result } = renderHook(() => useWorkloadSelectionState({ filteredGuests: () => [] }));
    await Promise.resolve();
    await Promise.resolve();
    expect(result.selectedGuestId()).toBe('cluster-a:node-1:101');
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('keeps a pending reference lookup and local dismissal through unrelated filter changes', async () => {
    const [search, setSearch] = createSignal('?resource=old-reference');
    readLocationSearch = search;
    let finish!: (resource: Resource) => void;
    vi.mocked(ResourceAPI.getResource).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const { result } = renderHook(() => useWorkloadSelectionState({ filteredGuests: () => [] }));
    const signal = vi.mocked(ResourceAPI.getResource).mock.calls[0]![1]!.signal!;
    setSearch('?resource=old-reference&type=vm');
    expect(signal.aborted).toBe(false);
    expect(ResourceAPI.getResource).toHaveBeenCalledTimes(1);
    finish({ id: 'stable-resource' } as Resource);
    await Promise.resolve();
    await Promise.resolve();
    expect(result.selectedGuestId()).toBe('stable-resource');
    result.clearPinnedSummaryScope();
    setSearch('?resource=old-reference&type=vm&node=node-2');
    expect(result.selectedGuestId()).toBeNull();
    expect(ResourceAPI.getResource).toHaveBeenCalledTimes(1);
  });

  it('keeps the selected resource drawer state through regrouping and resets it on another selection', () => {
    locationSearch = '';
    const [guests, setGuests] = createSignal([
      { id: 'stable-resource', node: 'node-1' },
    ] as WorkloadGuest[]);
    const { result } = renderHook(() => useWorkloadSelectionState({ filteredGuests: guests }));
    result.setSelectedGuestId('stable-resource');
    result.selectedGuestDrawerState.setActiveTab('history');
    result.selectedGuestDrawerState.setHistoryRange('12h');
    setGuests([{ id: 'stable-resource', node: 'node-2' }] as WorkloadGuest[]);
    result.setSelectedGuestId('stable-resource');
    expect(result.selectedGuestDrawerState.activeTab()).toBe('history');
    expect(result.selectedGuestDrawerState.historyRange()).toBe('12h');
    result.setSelectedGuestId('independent-clone');
    expect(result.selectedGuestDrawerState.activeTab()).toBe('overview');
    expect(result.selectedGuestDrawerState.historyRange()).toBe('24h');
    result.selectedGuestDrawerState.setActiveTab('manage');
    result.clearPinnedSummaryScope();
    expect(result.selectedGuestDrawerState.activeTab()).toBe('overview');
  });
});
