import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { StoragePoolDetail } from '@/components/Storage/StoragePoolDetail';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';

const historyChartSpy = vi.fn();

vi.mock('@/components/shared/HistoryChart', () => ({
  HistoryChart: (props: {
    resourceType: string;
    resourceId: string;
    metric: string;
    range?: string;
  }) => {
    historyChartSpy(props);
    return (
      <div data-testid="history-chart">
        {props.resourceType}:{props.resourceId}:{props.metric}:{props.range}
      </div>
    );
  },
}));

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'truenas01', scope: 'host' },
  capacity: { totalBytes: 1_000, usedBytes: 400, freeBytes: 600, usagePercent: 40 },
  capabilities: ['capacity', 'health'],
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  ...overrides,
});

describe('StoragePoolDetail', () => {
  it('uses canonical metrics target for capacity history charts', () => {
    historyChartSpy.mockClear();

    render(() => (
      <table>
        <tbody>
          <StoragePoolDetail
            record={makeRecord({
              metricsTarget: { resourceType: 'storage', resourceId: 'pool:tank' },
              refs: { resourceId: 'storage-legacy-id', platformEntityId: 'truenas-1' },
            })}
            physicalDisks={[]}
            summarySeriesId="pool:tank"
          />
        </tbody>
      </table>
    ));

    expect(screen.getByTestId('history-chart')).toHaveTextContent('storage:pool:tank:usage');
    expect(historyChartSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        resourceType: 'storage',
        resourceId: 'pool:tank',
        metric: 'usage',
        range: '7d',
      }),
    );
  });

  it('keeps the Relay 14-day range available in pool detail charts', () => {
    historyChartSpy.mockClear();

    render(() => (
      <table>
        <tbody>
          <StoragePoolDetail
            record={makeRecord({
              metricsTarget: { resourceType: 'storage', resourceId: 'pool:tank' },
            })}
            physicalDisks={[]}
            summarySeriesId="pool:tank"
          />
        </tbody>
      </table>
    ));

    const rangeSelector = screen.getByRole('combobox', {
      name: 'Capacity trend range',
    }) as HTMLSelectElement;
    expect(Array.from(rangeSelector.options).map((option) => option.value)).toEqual([
      '24h',
      '7d',
      '14d',
      '30d',
      '90d',
    ]);

    fireEvent.change(rangeSelector, { target: { value: '14d' } });

    expect(historyChartSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({
        resourceType: 'storage',
        resourceId: 'pool:tank',
        metric: 'usage',
        range: '14d',
      }),
    );
  });

  it('falls back to legacy storage refs when metrics target is absent', () => {
    historyChartSpy.mockClear();

    render(() => (
      <table>
        <tbody>
          <StoragePoolDetail
            record={makeRecord({
              refs: { resourceId: 'pve1:local-zfs', platformEntityId: 'cluster-a' },
            })}
            physicalDisks={[] as Resource[]}
            summarySeriesId="pve1:local-zfs"
          />
        </tbody>
      </table>
    ));

    expect(screen.getByTestId('history-chart')).toHaveTextContent('storage:pve1:local-zfs:usage');
    expect(historyChartSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        resourceType: 'storage',
        resourceId: 'pve1:local-zfs',
        metric: 'usage',
      }),
    );
  });
});
