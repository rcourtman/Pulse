import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal, type Accessor } from 'solid-js';

import { ChartsAPI, type AllMetricsHistoryResponse } from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { WorkloadGuest } from '@/types/workloads';

import { useWorkloadTableMetricHistory } from '../useWorkloadTableMetricHistory';
import type { WorkloadTableMetricHistoryRange } from '../workloadMetricHistoryModel';

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
  });

  const activeHistoryValue = () => {
    const active = props.activeGuest();
    return active ? (reader.getGuestMetricSeries(active, 'cpu')[0]?.points[0]?.value ?? '') : '';
  };

  return <div data-testid="active-history-value">{activeHistoryValue()}</div>;
}

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
});

describe('useWorkloadTableMetricHistory', () => {
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
