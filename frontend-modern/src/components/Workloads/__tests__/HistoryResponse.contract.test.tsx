import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { useHistoryChartState } from '@/components/shared/useHistoryChartState';
import type { WorkloadGuest } from '@/types/workloads';
import { GuestDrawerHistory } from '../GuestDrawerHistory';
import { useWorkloadTableMetricHistory } from '../useWorkloadTableMetricHistory';

// Mock the transport, not ChartsAPI: every production consumer uses the real
// response-admission boundary and its existing retention/error policy.
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));
vi.mock('@/stores/license', () => ({
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 7,
  isRangeLocked: () => false,
}));

const target = { resourceType: 'agent' as const, resourceId: 'pbs-current' };
const body = (value = 42) => ({
  ...target,
  range: '24h',
  start: 1_700_000_000_000,
  end: 1_700_000_060_000,
  source: 'store',
  metrics: {
    cpu: [0, 1].map((i) => ({
      timestamp: 1_700_000_000_000 + i * 60_000,
      value,
      min: value,
      max: value,
    })),
  },
});
const transport = vi.mocked(apiFetchJSON);
const settle = () => vi.advanceTimersByTimeAsync(0);
const plots = (container: HTMLElement) =>
  container.querySelectorAll('[data-testid="guest-history-plot"] path');

beforeEach(() => {
  vi.useFakeTimers();
  transport.mockReset();
  vi.spyOn(console, 'error').mockImplementation(() => {});
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('production History consumers reject unbound response evidence', () => {
  it.each([
    ['wrong resource', { resourceId: 'private-predecessor' }],
    ['wrong range', { range: '1h' }],
    ['wrong type', { resourceType: 'node' }],
    ['malformed metrics', { metrics: [] }],
  ])(
    'keeps initial %s unavailable, offers retry and admits real zero/empty success',
    async (_label, change) => {
      transport.mockResolvedValueOnce({ ...body(97), ...change, error: 'private diagnostics' });
      const view = render(() => (
        <GuestDrawerHistory target={target} range="24h" currentMetrics={{ cpu: 99 }} />
      ));
      await settle();
      expect(screen.getByRole('status', { name: 'History refresh status' })).toHaveTextContent(
        'Failed to load history data',
      );
      expect(plots(view.container)).toHaveLength(0);
      expect(view.container).not.toHaveTextContent('97.0%');
      expect(view.container).not.toHaveTextContent('99.0%');
      expect(view.container).not.toHaveTextContent('No stored history');
      expect(view.container).not.toHaveTextContent('private');
      transport.mockResolvedValueOnce(body(0));
      fireEvent.click(screen.getByRole('button', { name: 'Retry history' }));
      await settle();
      expect(plots(view.container)).toHaveLength(1);
      expect(view.container).toHaveTextContent('0.0%');
      transport.mockResolvedValueOnce({ ...body(), metrics: {} });
      fireEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
      await settle();
      expect(plots(view.container)).toHaveLength(0);
      expect(view.container).toHaveTextContent('No stored history in this range');
      expect(screen.getByRole('status', { name: 'History refresh status' })).toBeEmptyDOMElement();
    },
  );

  it('retains only its validated same-target result on an invalid refresh, including remount, then recovers', async () => {
    transport.mockResolvedValueOnce(body());
    const view = render(() => <GuestDrawerHistory target={target} range="24h" />);
    await settle();
    transport.mockResolvedValueOnce({ ...body(97), resourceId: 'wrong-pbs' });
    fireEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
    await settle();
    expect(plots(view.container)).toHaveLength(1);
    expect(view.container).toHaveTextContent('42.0%');
    expect(view.container).not.toHaveTextContent('97.0%');
    expect(view.container).toHaveTextContent(
      'History refresh failed. Showing previously loaded history.',
    );
    view.unmount();
    transport.mockImplementationOnce(() => new Promise(() => {}));
    const remount = render(() => <GuestDrawerHistory target={target} range="24h" />);
    expect(remount.container).toHaveTextContent('42.0%');
    expect(remount.container).not.toHaveTextContent('97.0%');
    await settle();
    transport.mockResolvedValueOnce(body(12));
    await vi.advanceTimersByTimeAsync(30_000);
    expect(remount.container).toHaveTextContent('12.0%');
    expect(remount.container).not.toHaveTextContent('42.0%');
    expect(screen.getByRole('status', { name: 'History refresh status' })).toBeEmptyDOMElement();
  });

  it('does not cache an invalid first response as empty history on remount', async () => {
    transport.mockResolvedValueOnce({ ...body(97), metrics: null });
    const first = render(() => <GuestDrawerHistory target={target} range="24h" />);
    await settle();
    first.unmount();
    transport.mockImplementationOnce(() => new Promise(() => {}));
    const second = render(() => <GuestDrawerHistory target={target} range="24h" />);
    expect(second.container).not.toHaveTextContent('No stored history');
    expect(second.container).toHaveTextContent('Failed to load history data');
    expect(plots(second.container)).toHaveLength(0);
  });

  it('keeps true denial separate from invalid HTTP200 and withdraws previously validated evidence', async () => {
    transport.mockResolvedValueOnce(body());
    const view = render(() => <GuestDrawerHistory target={target} range="24h" />);
    await settle();
    transport.mockResolvedValueOnce({ ...body(97), resourceId: 'wrong', status: 403 });
    fireEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
    await settle();
    expect(view.container).toHaveTextContent('previously loaded history');
    transport.mockRejectedValueOnce(Object.assign(new Error('private denial'), { status: 403 }));
    fireEvent.click(screen.getByRole('button', { name: 'Retry history' }));
    await settle();
    expect(plots(view.container)).toHaveLength(0);
    expect(view.container).not.toHaveTextContent('42.0%');
    expect(view.container).toHaveTextContent(
      'Access denied. Check your permissions and license plan.',
    );
  });

  it('protects the shared single-metric chart across initial, refresh and selection failures', async () => {
    const valid = (value = 42) => {
      const { metrics, ...response } = body(value);
      return { ...response, metric: 'cpu', points: metrics.cpu };
    };
    const [resourceId, setResourceId] = createSignal(target.resourceId);
    function Probe() {
      const chart = useHistoryChartState(
        {
          get resourceId() {
            return resourceId();
          },
          resourceType: 'agent',
          metric: 'cpu',
          range: '24h',
        },
        { getCanvas: () => undefined, getContainer: () => undefined },
      );
      return (
        <output>
          {JSON.stringify({
            values: chart.data().map((p) => p.value),
            error: chart.error(),
            refresh: chart.refreshFailed(),
          })}
        </output>
      );
    }
    transport.mockResolvedValueOnce({ ...valid(97), metric: 'memory' });
    const view = render(() => <Probe />);
    await settle();
    expect(view.container).toHaveTextContent('Failed to load history data');
    expect(view.container).not.toHaveTextContent('97');
    transport.mockResolvedValueOnce(valid());
    await vi.advanceTimersByTimeAsync(30_000);
    expect(view.container).toHaveTextContent('"values":[42,42]');
    transport.mockResolvedValueOnce({ ...valid(97), points: null });
    await vi.advanceTimersByTimeAsync(30_000);
    expect(view.container).toHaveTextContent('"values":[42,42]');
    expect(view.container).toHaveTextContent('"refresh":true');
    transport.mockResolvedValueOnce(valid(97));
    setResourceId('pbs-replacement');
    await settle();
    expect(view.container).toHaveTextContent('"values":[]');
    expect(view.container).toHaveTextContent('Failed to load history data');
  });

  it('keeps wrong-target or malformed hover warming out of the row cache and permits a later valid request', async () => {
    const guest = {
      id: 'vm-current',
      name: 'vm-current',
      type: 'qemu',
      workloadType: 'vm',
      metricsTarget: { resourceType: 'vm', resourceId: 'metrics-current' },
    } as WorkloadGuest;
    const [active, setActive] = createSignal<WorkloadGuest | null>(guest);
    function Probe() {
      const reader = useWorkloadTableMetricHistory({
        activeGuest: active,
        enabled: () => false,
        onDemand: () => true,
        range: () => '24h',
        selectedNode: () => null,
      });
      return (
        <output>
          {JSON.stringify(active() ? reader.getGuestMetricSeries(active()!, 'cpu') : [])}
        </output>
      );
    }
    transport.mockResolvedValueOnce({ ...body(97), resourceType: 'vm', resourceId: 'wrong-vm' });
    const view = render(() => <Probe />);
    await settle();
    expect(view.container).not.toHaveTextContent('97');
    transport.mockResolvedValueOnce({
      ...body(97),
      resourceType: 'vm',
      resourceId: 'metrics-current',
      metrics: [],
    });
    setActive(null);
    setActive(guest);
    await settle();
    expect(view.container).not.toHaveTextContent('97');
    transport.mockResolvedValueOnce({
      ...body(0),
      resourceType: 'vm',
      resourceId: 'metrics-current',
    });
    setActive(null);
    setActive(guest);
    await settle();
    expect(view.container).toHaveTextContent('"value":0');
    expect(transport).toHaveBeenCalledTimes(3);
  });
});
