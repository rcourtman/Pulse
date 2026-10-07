import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal, type Accessor } from 'solid-js';

import { ChartsAPI, type AllMetricsHistoryResponse } from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';

import { useWorkloadTableMetricHistory } from '../useWorkloadTableMetricHistory';
import {
  WORKLOAD_TABLE_HISTORY_INFRA_METRICS,
  WORKLOAD_TABLE_HISTORY_MAX_POINTS,
  WORKLOAD_TABLE_HISTORY_POLL_MS,
  type WorkloadTableMetricHistoryRange,
} from '../workloadMetricHistoryModel';

const guest = {
  id: 'cluster-a:pve1:101',
  name: 'api-101',
  type: 'qemu',
  workloadType: 'vm',
  metricsTarget: {
    resourceType: 'vm',
    resourceId: 'metrics-vm-101',
  },
} as WorkloadGuest;

const historyResponse = (
  range: WorkloadTableMetricHistoryRange,
  value: number,
): AllMetricsHistoryResponse => ({
  resourceType: 'vm',
  resourceId: 'metrics-vm-101',
  range,
  start: 1,
  end: 2,
  metrics: {
    cpu: [{ timestamp: 2, value, min: value, max: value }],
  },
  source: 'store',
});

function HistoryProbe(props: {
  activeGuest: Accessor<WorkloadGuest | null>;
  prefetchGuests?: Accessor<readonly WorkloadGuest[]>;
  range: Accessor<WorkloadTableMetricHistoryRange>;
}) {
  const reader = useWorkloadTableMetricHistory({
    activeGuest: props.activeGuest,
    enabled: () => false,
    onDemand: () => true,
    prefetchGuests: props.prefetchGuests,
    range: props.range,
    selectedNode: () => null,
    series: 'guests',
  });

  const activeHistoryValue = () => {
    const active = props.activeGuest();
    return active ? (reader.getGuestMetricSeries(active, 'cpu')[0]?.points[0]?.value ?? '') : '';
  };

  return <div data-testid="active-history-value">{activeHistoryValue()}</div>;
}

const node = { id: 'cluster-a-pve1', name: 'pve1', instance: 'cluster-a' } as Node;
const point = (value: number) => [{ timestamp: 2, value, min: value, max: value }];

function SeriesProbe(props: { series: 'guests' | 'nodes' }) {
  const reader = useWorkloadTableMetricHistory({
    enabled: () => true,
    range: () => '1h',
    series: props.series,
  });
  const latest = (series: ReturnType<typeof reader.getGuestMetricSeries>) =>
    series[0]?.points.at(-1)?.value ?? 'none';

  return (
    <div data-testid="series-values">
      {`guest=${latest(reader.getGuestMetricSeries(guest, 'cpu'))} node=${latest(
        reader.getNodeMetricSeries(node, 'cpu'),
      )}`}
    </div>
  );
}

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('useWorkloadTableMetricHistory', () => {
  it.each([
    ['guests', 'guest=11 node=none', 'workloads'],
    ['nodes', 'guest=none node=22', 'infrastructure'],
  ] as const)(
    'polls only the summary its %s rows read',
    async (series, expectedValues, polledRoute) => {
      vi.useFakeTimers();
      const workloadSpy = vi.spyOn(ChartsAPI, 'getWorkloadCharts').mockResolvedValue({
        data: { [guest.id]: { cpu: point(11) } },
        dockerData: {},
        guestTypes: {},
        timestamp: 2,
        stats: { oldestDataTimestamp: 1 },
      });
      const infrastructureSpy = vi
        .spyOn(ChartsAPI, 'getInfrastructureSummaryCharts')
        .mockResolvedValue({
          nodeData: { [node.id]: { cpu: point(22) } },
          timestamp: 2,
          stats: { oldestDataTimestamp: 1 },
        });
      const [polledSpy, idleSpy] =
        polledRoute === 'workloads'
          ? [workloadSpy, infrastructureSpy]
          : [infrastructureSpy, workloadSpy];

      render(() => <SeriesProbe series={series} />);
      await vi.advanceTimersByTimeAsync(0);

      expect(screen.getByTestId('series-values')).toHaveTextContent(expectedValues);
      expect(polledSpy).toHaveBeenCalledTimes(1);
      expect(idleSpy).not.toHaveBeenCalled();

      await vi.advanceTimersByTimeAsync(WORKLOAD_TABLE_HISTORY_POLL_MS * 2);
      expect(polledSpy).toHaveBeenCalledTimes(3);
      expect(idleSpy).not.toHaveBeenCalled();
    },
  );

  it('reads both estate summaries straight from the charts API without persisting them', async () => {
    const workloadSpy = vi.spyOn(ChartsAPI, 'getWorkloadCharts').mockResolvedValue({
      data: { [guest.id]: { cpu: point(11) } },
      dockerData: {},
      guestTypes: {},
      timestamp: 2,
      stats: { oldestDataTimestamp: 1 },
    });
    const infrastructureSpy = vi
      .spyOn(ChartsAPI, 'getInfrastructureSummaryCharts')
      .mockResolvedValue({
        nodeData: {},
        agentData: { [node.id]: { cpu: point(33) } },
        timestamp: 2,
        stats: { oldestDataTimestamp: 1 },
      });
    const setItemSpy = vi.spyOn(Storage.prototype, 'setItem');

    render(() => (
      <>
        <SeriesProbe series="guests" />
        <SeriesProbe series="nodes" />
      </>
    ));

    await waitFor(() => {
      expect(screen.getAllByTestId('series-values').map((el) => el.textContent)).toEqual([
        'guest=11 node=none',
        'guest=none node=33',
      ]);
    });
    expect(workloadSpy).toHaveBeenCalledWith('1h', expect.any(AbortSignal), {
      maxPoints: WORKLOAD_TABLE_HISTORY_MAX_POINTS,
      nodeId: undefined,
    });
    expect(infrastructureSpy).toHaveBeenCalledWith('1h', expect.any(AbortSignal), {
      metrics: WORKLOAD_TABLE_HISTORY_INFRA_METRICS,
    });
    // The query layer retains each read across remounts; nothing reads a
    // persisted copy, so the summaries must not be written to storage.
    expect(setItemSpy).not.toHaveBeenCalled();
  });

  it('bounds slow-estate warming and prioritizes a distant active guest', async () => {
    const guests = Array.from({ length: 500 }, (_, index) => ({
      ...guest,
      id: `cluster-a:pve1:${index}`,
      name: `guest-${index}`,
      metricsTarget: {
        resourceType: 'vm',
        resourceId: `metrics-vm-${index}`,
      },
    })) as WorkloadGuest[];
    const [activeGuest, setActiveGuest] = createSignal<WorkloadGuest | null>(null);
    const [range] = createSignal<WorkloadTableMetricHistoryRange>('1h');
    const calls: string[] = [];
    const resolvers = new Map<string, () => void>();
    let inFlight = 0;
    let maxInFlight = 0;

    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockImplementation((params) => {
      calls.push(params.resourceId);
      inFlight += 1;
      maxInFlight = Math.max(maxInFlight, inFlight);
      return new Promise<AllMetricsHistoryResponse>((resolve) => {
        resolvers.set(params.resourceId, () => {
          inFlight -= 1;
          resolve({
            ...historyResponse('1h', 10),
            resourceId: params.resourceId,
          });
        });
      });
    });

    render(() => (
      <HistoryProbe activeGuest={activeGuest} prefetchGuests={() => guests} range={range} />
    ));

    await waitFor(() => expect(calls).toHaveLength(4));
    expect(calls).toEqual(['metrics-vm-0', 'metrics-vm-1', 'metrics-vm-2', 'metrics-vm-3']);
    expect(maxInFlight).toBe(4);

    setActiveGuest(guests[400]);
    await Promise.resolve();
    expect(calls).toHaveLength(4);

    resolvers.get('metrics-vm-0')?.();
    await waitFor(() => expect(calls).toHaveLength(5));
    expect(calls[4]).toBe('metrics-vm-400');
    expect(maxInFlight).toBe(4);
  });

  it('warms visible guest history before the pointer reaches the row', async () => {
    const secondGuest = {
      ...guest,
      id: 'cluster-a:pve1:102',
      name: 'api-102',
      metricsTarget: {
        resourceType: 'vm',
        resourceId: 'metrics-vm-102',
      },
    } as WorkloadGuest;
    const [activeGuest, setActiveGuest] = createSignal<WorkloadGuest | null>(null);
    const [range] = createSignal<WorkloadTableMetricHistoryRange>('1h');
    const historySpy = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockImplementation((params) =>
      Promise.resolve({
        ...historyResponse('1h', params.resourceId === 'metrics-vm-102' ? 20 : 10),
        resourceId: params.resourceId,
      }),
    );

    render(() => (
      <HistoryProbe
        activeGuest={activeGuest}
        prefetchGuests={() => [guest, secondGuest]}
        range={range}
      />
    ));

    await waitFor(() => {
      expect(historySpy).toHaveBeenCalledTimes(2);
    });
    setActiveGuest(secondGuest);

    await waitFor(() => {
      expect(screen.getByTestId('active-history-value')).toHaveTextContent('20');
    });
    expect(historySpy).toHaveBeenCalledTimes(2);
  });

  it('loads only the active row and clears the old range while its replacement is pending', async () => {
    const [activeGuest, setActiveGuest] = createSignal<WorkloadGuest | null>(null);
    const [range, setRange] = createSignal<WorkloadTableMetricHistoryRange>('1h');
    let resolve12h!: (value: AllMetricsHistoryResponse) => void;
    const pending12h = new Promise<AllMetricsHistoryResponse>((resolve) => {
      resolve12h = resolve;
    });
    const historySpy = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockImplementation((params) =>
        params.range === '12h' ? pending12h : Promise.resolve(historyResponse('1h', 10)),
      );

    render(() => <HistoryProbe activeGuest={activeGuest} range={range} />);

    expect(historySpy).not.toHaveBeenCalled();
    setActiveGuest(guest);

    await waitFor(() => {
      expect(historySpy).toHaveBeenCalledWith(
        expect.objectContaining({
          resourceType: 'vm',
          resourceId: 'metrics-vm-101',
          range: '1h',
          maxPoints: 36,
          signal: expect.any(AbortSignal),
        }),
      );
      expect(screen.getByTestId('active-history-value')).toHaveTextContent('10');
    });

    setActiveGuest({ ...guest });
    await Promise.resolve();
    expect(historySpy).toHaveBeenCalledTimes(1);

    setRange('12h');
    await waitFor(() => {
      expect(historySpy).toHaveBeenCalledWith(
        expect.objectContaining({
          resourceType: 'vm',
          resourceId: 'metrics-vm-101',
          range: '12h',
          maxPoints: 36,
          signal: expect.any(AbortSignal),
        }),
      );
    });
    expect(screen.getByTestId('active-history-value').textContent).toBe('');

    resolve12h(historyResponse('12h', 20));
    await waitFor(() => {
      expect(screen.getByTestId('active-history-value')).toHaveTextContent('20');
    });
  });
});
