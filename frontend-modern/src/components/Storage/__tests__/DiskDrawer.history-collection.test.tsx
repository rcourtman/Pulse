import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  ChartsAPI,
  type AggregatedMetricPoint,
  type AllMetricsHistoryResponse,
  type HistoryTimeRange,
  type MetricsHistoryParams,
} from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { Resource } from '@/types/resource';
import { extractPhysicalDiskPresentationData } from '@/features/storageBackups/diskPresentation';
import { DiskDrawer } from '../DiskDrawer';
import { DiskDrawerOverview } from '../DiskDrawerOverview';

// The disk drawer charts through the real drawer history owner; only the
// history client, the plan limits and the alert thresholds are stubbed.
vi.mock('@/api/charts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/charts')>()),
  ChartsAPI: { getMetricsHistory: vi.fn() },
}));
vi.mock('@/stores/license', () => ({
  isRangeLocked: (range: HistoryTimeRange) => ['14d', '30d', '90d'].includes(range),
  loadRuntimeCapabilities: vi.fn(async () => undefined),
  maxHistoryDays: () => 7,
}));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ getDiskTemperatureThresholds: () => null }),
}));
vi.mock('@/components/Infrastructure/ResourceOperatorStateSection', () => ({
  ResourceOperatorStateSection: () => <div data-testid="operator-state" />,
}));

const request = vi.mocked(ChartsAPI.getMetricsHistory);

const OBSERVED_AT = 1_700_000_000_000;

const disk = (missing = false, nvme = false): Resource => ({
  id: 'disk-a',
  type: 'physical_disk',
  name: 'Archive disk',
  displayName: 'Archive disk',
  platformId: 'nas-a',
  platformType: 'truenas',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1000,
  identity: { hostname: 'nas-a' },
  metricsTarget: { resourceType: 'disk', resourceId: 'disk:nas-a:sda' },
  physicalDisk: {
    devPath: nvme ? '/dev/nvme0n1' : '/dev/sda',
    model: nvme ? 'NVMe' : 'Archive HDD',
    serial: 'SERIAL-A',
    diskType: nvme ? 'nvme' : 'hdd',
    temperature: missing ? undefined : 42,
    smart: missing
      ? undefined
      : nvme
        ? { percentageUsed: 12, availableSpare: 97 }
        : { reallocatedSectors: 0 },
    collection: {
      temperature: {
        state: missing ? 'unavailable' : 'available',
        reason: missing ? 'collection deadline exceeded' : undefined,
      },
      io: { state: 'unsupported', reason: 'per-member counters unavailable' },
    },
  },
});

const points = (value: number): AggregatedMetricPoint[] => [
  { timestamp: OBSERVED_AT, value, min: value, max: value },
  { timestamp: OBSERVED_AT + 60_000, value, min: value, max: value },
];

// The drawer reads every series of its one disk target in a single request.
const response = (
  params: MetricsHistoryParams,
  temperature = 42,
  metrics?: Record<string, AggregatedMetricPoint[]>,
): AllMetricsHistoryResponse => ({
  resourceType: params.resourceType,
  resourceId: params.resourceId,
  range: params.range!,
  start: OBSERVED_AT - 60_000,
  end: OBSERVED_AT + 120_000,
  source: 'store',
  metrics: metrics ?? {
    smart_temp: points(temperature),
    smart_reallocated_sectors: points(0),
    smart_percentage_used: points(12),
    smart_available_spare: points(97),
  },
});

const chart = (label: string) => screen.getByRole('img', { name: `${label} history` });
const chartLabels = () =>
  screen.queryAllByTestId('guest-history-plot').map((plot) => plot.getAttribute('aria-label'));
// The legend reading beside a series label, e.g. "Disk 42°C".
const legendValue = (label: string, series: string) =>
  within(panel(label)).getByText(series).nextElementSibling?.textContent;
const description = (plot: Element) =>
  plot
    .getAttribute('aria-describedby')!
    .split(' ')
    .map((id) => document.getElementById(id)?.textContent ?? '')
    .join(' ');
const panel = (label: string) =>
  chart(label).closest<HTMLElement>('[data-testid="guest-history-group-chart"]')!;
const renderDrawer = (snapshot: () => Resource) =>
  render(() => <DiskDrawer disk={snapshot()} onClose={() => {}} />);

beforeEach(() => {
  request.mockReset().mockImplementation(async (params) => response(params));
});

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
});

describe('Disk drawer History independently of current collection', () => {
  it('loads stored ATA readings when current temperature and SMART are unavailable', async () => {
    renderDrawer(() => disk(true));
    expect(screen.getByTestId('disk-drawer-notices')).toHaveTextContent(
      'Temperature is temporarily unavailable: collection deadline exceeded',
    );
    await waitFor(() => expect(description(chart('Temperature'))).toContain('Disk 42°C'));
    expect(description(chart('Reallocated sectors'))).toContain('Reallocated 0 sectors');
    // The disk family picks the panels; unsupported I/O leaves its panels out.
    expect(chartLabels()).toEqual(['Temperature history', 'Reallocated sectors history']);
    expect(screen.queryByRole('img', { name: 'Disk I/O history' })).toBeNull();
    expect(screen.queryByRole('img', { name: 'Busy history' })).toBeNull();
    // Only the resolved canonical disk target is read.
    expect(request.mock.calls.length).toBeGreaterThan(0);
    for (const [params] of request.mock.calls) {
      expect(params).toMatchObject({ resourceType: 'disk', resourceId: 'disk:nas-a:sda' });
    }
  });

  it('loads stored NVMe temperature and endurance without current SMART fields', async () => {
    renderDrawer(() => disk(true, true));
    await waitFor(() => expect(description(chart('Temperature'))).toContain('Disk 42°C'));
    expect(chartLabels()).toEqual(['Temperature history', 'Wear history']);
    expect(description(chart('Wear'))).toContain('Life used 12.0%');
    expect(description(chart('Wear'))).toContain('Spare 97.0%');
    expect(screen.queryByRole('img', { name: 'Reallocated sectors history' })).toBeNull();
  });

  it('preserves mounted charts, range and inspection without extra reads on matching snapshots', async () => {
    const [snapshot, setSnapshot] = createSignal(disk());
    renderDrawer(snapshot);
    const range = await screen.findByRole('combobox', { name: 'History range' });
    fireEvent.change(range, { target: { value: '7d' } });
    await waitFor(() =>
      expect(request).toHaveBeenLastCalledWith(expect.objectContaining({ range: '7d' })),
    );
    const thermal = chart('Temperature');
    await waitFor(() => expect(description(thermal)).toContain('Disk 42°C'));
    // Inspect the older of the two stored observations.
    const inspect = within(panel('Temperature')).getByRole('slider', {
      name: 'Inspect Temperature history',
    });
    inspect.focus();
    fireEvent.input(inspect, { target: { value: '0' } });
    const inspected = description(thermal);
    expect(inspected).toContain(new Date(OBSERVED_AT).toLocaleString());
    const reads = request.mock.calls.length;

    setSnapshot({ ...disk(), physicalDisk: { ...disk().physicalDisk!, temperature: 65 } });

    expect(chart('Temperature')).toBe(thermal);
    expect(document.activeElement).toBe(inspect);
    expect(description(thermal)).toBe(inspected);
    expect(screen.getByRole('combobox', { name: 'History range' })).toHaveValue('7d');
    expect(screen.getByTestId('disk-drawer-health-card')).toHaveTextContent('65°C');
    expect(request).toHaveBeenCalledTimes(reads);
  });

  it('drops inspection and reads again when the range or disk target changes', async () => {
    const [snapshot, setSnapshot] = createSignal(disk());
    renderDrawer(snapshot);
    const thermal = await screen.findByRole('img', { name: 'Temperature history' });
    await waitFor(() => expect(description(thermal)).toContain('2 stored observation times'));
    const inspect = () =>
      within(panel('Temperature')).getByRole('slider', { name: 'Inspect Temperature history' });
    const inspectOldest = () => {
      inspect().focus();
      fireEvent.input(inspect(), { target: { value: '0' } });
      expect(description(chart('Temperature'))).not.toContain('stored observation times');
    };

    inspectOldest();
    let reads = request.mock.calls.length;
    fireEvent.change(screen.getByRole('combobox', { name: 'History range' }), {
      target: { value: '6h' },
    });
    await waitFor(() => expect(request).toHaveBeenCalledTimes(reads + 1));
    expect(request).toHaveBeenLastCalledWith(
      expect.objectContaining({ resourceId: 'disk:nas-a:sda', range: '6h' }),
    );
    await waitFor(() =>
      expect(description(chart('Temperature'))).toContain('2 stored observation times'),
    );

    inspectOldest();
    reads = request.mock.calls.length;
    setSnapshot({
      ...disk(),
      metricsTarget: { resourceType: 'disk', resourceId: 'disk:nas-b:sda' },
    });
    await waitFor(() => expect(request).toHaveBeenCalledTimes(reads + 1));
    expect(request).toHaveBeenLastCalledWith(
      expect.objectContaining({ resourceId: 'disk:nas-b:sda', range: '6h' }),
    );
    await waitFor(() =>
      expect(description(chart('Temperature'))).toContain('2 stored observation times'),
    );
  });

  it('keeps stored observations mounted after current fields disappear', async () => {
    const [snapshot, setSnapshot] = createSignal(disk());
    renderDrawer(snapshot);
    const thermal = await screen.findByRole('img', { name: 'Temperature history' });
    await waitFor(() => expect(description(thermal)).toContain('Disk 42°C'));
    const reads = request.mock.calls.length;

    setSnapshot(disk(true));

    expect(chart('Temperature')).toBe(thermal);
    expect(description(thermal)).toContain('Disk 42°C');
    expect(chart('Reallocated sectors')).toBeInTheDocument();
    expect(request).toHaveBeenCalledTimes(reads);
    // The current reading is withdrawn beside the charts, which keep the
    // stored observation: 42°C survives only inside the history region.
    expect(screen.getByTestId('disk-drawer-notices')).toHaveTextContent(
      'Temperature is temporarily unavailable',
    );
    expect(within(screen.getByTestId('disk-drawer-health-card')).queryByText(/°C/)).toBeNull();
    const readings = screen.getAllByText('42°C');
    expect(readings.length).toBeGreaterThan(0);
    for (const reading of readings) {
      expect(reading.closest('[data-testid="guest-history-region"]')).not.toBeNull();
    }
  });

  it('ignores a former target completion and reads only the new disk target', async () => {
    const pending: {
      params: MetricsHistoryParams;
      resolve: (value: AllMetricsHistoryResponse) => void;
    }[] = [];
    request.mockImplementation(
      (params) => new Promise((resolve) => pending.push({ params, resolve })),
    );
    const [snapshot, setSnapshot] = createSignal(disk());
    renderDrawer(snapshot);
    const forTarget = (resourceId: string) =>
      pending.filter(({ params }) => params.resourceId === resourceId);
    await waitFor(() => expect(forTarget('disk:nas-a:sda').length).toBeGreaterThan(0));

    setSnapshot({
      ...disk(),
      metricsTarget: { resourceType: 'disk', resourceId: 'disk:nas-b:sda' },
    });
    await waitFor(() => expect(forTarget('disk:nas-b:sda').length).toBeGreaterThan(0));
    for (const item of forTarget('disk:nas-b:sda')) item.resolve(response(item.params, 55));
    await waitFor(() => expect(description(chart('Temperature'))).toContain('Disk 55°C'));

    for (const item of forTarget('disk:nas-a:sda')) {
      expect(item.params.signal?.aborted).toBe(true);
      item.resolve(response(item.params, 99));
    }
    await Promise.resolve();
    await Promise.resolve();
    expect(description(chart('Temperature'))).toContain('Disk 55°C');
    expect(description(chart('Temperature'))).not.toContain('99°C');
  });

  it('keeps an empty stored series apart from a measured zero', async () => {
    request.mockImplementation(async (params) =>
      response(params, 42, { smart_temp: [], smart_reallocated_sectors: points(0) }),
    );
    renderDrawer(() => disk(true));
    await waitFor(() =>
      expect(description(chart('Reallocated sectors'))).toContain('Reallocated 0 sectors'),
    );
    const thermal = panel('Temperature');
    expect(within(thermal).getByText('No stored history in this range')).toBeInTheDocument();
    expect(description(chart('Temperature'))).toContain('No stored history observations.');
    expect(description(chart('Temperature'))).not.toContain('0°C');
    // No reading is drawn or quoted for the empty series.
    expect(legendValue('Temperature', 'Disk')).toBe('-');
    expect(thermal.querySelector('[data-history-series]')).toBeNull();
    // The measured zero is drawn as stored history.
    expect(panel('Reallocated sectors').querySelector('[data-history-series]')).not.toBeNull();
  });

  it('labels a snapshot reading as current while no history is stored, never as history', async () => {
    request.mockImplementation(async (params) => response(params, 42, {}));
    renderDrawer(() => disk());
    const thermal = await screen.findByRole('img', { name: 'Temperature history' });
    await waitFor(() =>
      expect(
        within(panel('Temperature')).getByText('No stored history in this range'),
      ).toBeInTheDocument(),
    );
    expect(description(thermal)).toContain('No stored history observations.');
    const current = panel('Temperature').querySelector('[data-history-current="smart_temp"]');
    expect(current).toHaveTextContent('42°C');
    expect(current).toHaveTextContent('current');
  });

  it('makes no history request without a resolvable disk identity', () => {
    // The guest drawer mounts the Overview for disks it lists; a record with
    // no target, serial, WWN or ID has nothing stable to read history for.
    const anonymous: Resource = {
      ...disk(true),
      id: '',
      metricsTarget: undefined,
      physicalDisk: { ...disk(true).physicalDisk!, serial: '', wwn: '' },
    };
    render(() => (
      <DiskDrawerOverview
        disk={anonymous}
        diskData={extractPhysicalDiskPresentationData(anonymous, () => null)}
      />
    ));
    expect(screen.getByTestId('disk-drawer-history-unavailable')).toHaveTextContent(
      'History is unavailable until Pulse can resolve a stable identity for this disk.',
    );
    expect(chartLabels()).toEqual([]);
    expect(request).not.toHaveBeenCalled();
  });
});
