import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  ChartsAPI,
  type MetricsHistoryParams,
  type SingleMetricHistoryResponse,
} from '@/api/charts';
import { DiskDetail } from '../DiskDetail';
import type { Resource } from '@/types/resource';

vi.mock('@/api/charts', () => ({ ChartsAPI: { getMetricsHistory: vi.fn() } }));
vi.mock('@/stores/license', () => ({
  isRangeLocked: () => false,
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 7,
}));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ getDiskTemperatureThresholds: () => null }),
}));
const request = vi.mocked(ChartsAPI.getMetricsHistory);
const context = {
  clearRect: vi.fn(),
  setTransform: vi.fn(),
  beginPath: vi.fn(),
  moveTo: vi.fn(),
  lineTo: vi.fn(),
  stroke: vi.fn(),
  fillText: vi.fn(),
  closePath: vi.fn(),
  fill: vi.fn(),
  arc: vi.fn(),
  save: vi.fn(),
  restore: vi.fn(),
  setLineDash: vi.fn(),
  measureText: () => ({ width: 20 }),
};
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
const response = (
  params: MetricsHistoryParams,
  value = params.metric === 'smart_temp' ? 42 : 0,
): SingleMetricHistoryResponse => ({
  resourceType: params.resourceType,
  resourceId: params.resourceId,
  metric: params.metric!,
  range: params.range!,
  start: 1000,
  end: 2000,
  source: 'store',
  points: [{ timestamp: 1000, value, min: value, max: value }],
});
const description = (canvas: HTMLElement) =>
  document.getElementById(canvas.getAttribute('aria-describedby')!)!;
const openHistory = () => fireEvent.click(screen.getByRole('tab', { name: 'History' }));

beforeEach(() => {
  request.mockReset().mockImplementation(async (params) => response(params));
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
    context as unknown as CanvasRenderingContext2D,
  );
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  );
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('Disk History independently of current collection', () => {
  it('loads stored ATA readings when current temperature and SMART are unavailable', async () => {
    render(() => <DiskDetail disk={disk(true)} nodes={[]} />);
    expect(screen.getByText(/Temperature is temporarily unavailable/)).toBeInTheDocument();
    openHistory();
    await waitFor(() =>
      expect(screen.getByRole('img', { name: 'Temperature chart' })).toBeInTheDocument(),
    );
    const thermal = screen.getByRole('img', { name: 'Temperature chart' });
    await waitFor(() => expect(description(thermal)).toHaveTextContent('42°C'));
    expect(
      description(screen.getByRole('img', { name: 'Reallocated Sectors chart' })),
    ).toHaveTextContent('0 sectors');
    expect(
      request.mock.calls.every(
        ([params]) => params.resourceType === 'disk' && params.resourceId === 'disk:nas-a:sda',
      ),
    ).toBe(true);
    expect(request.mock.calls.map(([params]) => params.metric)).not.toContain('diskread');
    expect(screen.queryByText('Live I/O (30m)')).not.toBeInTheDocument();
  });

  it('loads stored NVMe temperature and endurance without current SMART fields', async () => {
    render(() => <DiskDetail disk={disk(true, true)} nodes={[]} />);
    openHistory();
    await waitFor(() => expect(screen.getAllByRole('img')).toHaveLength(3));
    expect(screen.getByRole('img', { name: 'Life Used chart' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Available Spare chart' })).toBeInTheDocument();
    expect(
      screen.queryByRole('img', { name: 'Reallocated Sectors chart' }),
    ).not.toBeInTheDocument();
    await waitFor(() =>
      expect(description(screen.getByRole('img', { name: 'Temperature chart' }))).toHaveTextContent(
        '42°C',
      ),
    );
  });

  it('preserves mounted charts and inspection without extra reads on matching snapshots', async () => {
    const [snapshot, setSnapshot] = createSignal(disk());
    render(() => <DiskDetail disk={snapshot()} nodes={[]} />);
    openHistory();
    fireEvent.change(screen.getByRole('combobox', { name: 'Disk history range' }), {
      target: { value: '7d' },
    });
    const thermal = screen.getByRole('img', { name: 'Temperature chart' });
    await waitFor(() => expect(description(thermal)).toHaveTextContent('42°C'));
    fireEvent.focus(thermal);
    expect(document.querySelector('[data-history-chart-tooltip]')).toHaveTextContent('42°C');
    const reads = request.mock.calls.length;
    setSnapshot({ ...disk(), physicalDisk: { ...disk().physicalDisk!, temperature: 65 } });
    expect(screen.getByRole('img', { name: 'Temperature chart' })).toBe(thermal);
    expect(document.querySelector('[data-history-chart-tooltip]')).toHaveTextContent('42°C');
    expect(screen.getByRole('combobox', { name: 'Disk history range' })).toHaveValue('7d');
    expect(request).toHaveBeenCalledTimes(reads);
  });

  it('keeps stored observations mounted after current fields disappear', async () => {
    const [snapshot, setSnapshot] = createSignal(disk());
    render(() => <DiskDetail disk={snapshot()} nodes={[]} />);
    openHistory();
    const thermal = screen.getByRole('img', { name: 'Temperature chart' });
    await waitFor(() => expect(description(thermal)).toHaveTextContent('42°C'));
    const reads = request.mock.calls.length;
    setSnapshot(disk(true));
    expect(screen.getByRole('img', { name: 'Temperature chart' })).toBe(thermal);
    expect(description(thermal)).toHaveTextContent('42°C');
    expect(screen.getByRole('img', { name: 'Reallocated Sectors chart' })).toBeInTheDocument();
    expect(request).toHaveBeenCalledTimes(reads);
    fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
    expect(screen.getByText(/Temperature is temporarily unavailable/)).toBeVisible();
    expect(screen.queryByText('42°C')).not.toBeInTheDocument();
  });

  it('ignores an old target completion and uses only the explicit new disk target', async () => {
    const pending: {
      params: MetricsHistoryParams;
      resolve: (value: SingleMetricHistoryResponse) => void;
    }[] = [];
    request.mockImplementation(
      (params) => new Promise((resolve) => pending.push({ params, resolve })),
    );
    const [snapshot, setSnapshot] = createSignal(disk());
    render(() => <DiskDetail disk={snapshot()} nodes={[]} />);
    openHistory();
    await waitFor(() =>
      expect(
        pending.filter(({ params }) => params.resourceId === 'disk:nas-a:sda').length,
      ).toBeGreaterThan(0),
    );
    setSnapshot({
      ...disk(),
      metricsTarget: { resourceType: 'disk', resourceId: 'disk:nas-b:sda' },
    });
    await waitFor(() =>
      expect(
        pending.filter(({ params }) => params.resourceId === 'disk:nas-b:sda').length,
      ).toBeGreaterThan(0),
    );
    for (const item of pending.filter(({ params }) => params.resourceId === 'disk:nas-b:sda'))
      item.resolve(response(item.params, 55));
    await waitFor(() =>
      expect(description(screen.getByRole('img', { name: 'Temperature chart' }))).toHaveTextContent(
        '55°C',
      ),
    );
    for (const item of pending.filter(({ params }) => params.resourceId === 'disk:nas-a:sda')) {
      expect(item.params.signal?.aborted).toBe(true);
      item.resolve(response(item.params, 99));
    }
    await Promise.resolve();
    expect(description(screen.getByRole('img', { name: 'Temperature chart' }))).toHaveTextContent(
      '55°C',
    );
    expect(
      description(screen.getByRole('img', { name: 'Temperature chart' })),
    ).not.toHaveTextContent('99°C');
  });

  it('distinguishes empty stored series from measured zero with no live fallback', async () => {
    request.mockImplementation(async (params) => ({
      ...response(params),
      points: params.metric === 'smart_temp' ? [] : response(params, 0).points,
    }));
    render(() => <DiskDetail disk={disk(true)} nodes={[]} />);
    openHistory();
    await waitFor(() =>
      expect(screen.getByText('No history samples in this time range.')).toBeInTheDocument(),
    );
    expect(
      description(screen.getByRole('img', { name: 'Temperature chart' })),
    ).not.toHaveTextContent('0°C');
    await waitFor(() =>
      expect(
        description(screen.getByRole('img', { name: 'Reallocated Sectors chart' })),
      ).toHaveTextContent('0 sectors'),
    );
  });

  it('makes no history request without a resolvable disk identity', () => {
    render(() => (
      <DiskDetail
        disk={{
          ...disk(true),
          id: '',
          metricsTarget: undefined,
          physicalDisk: { ...disk(true).physicalDisk!, serial: '', wwn: '' },
        }}
        nodes={[]}
      />
    ));
    expect(screen.queryByRole('tab', { name: 'History' })).not.toBeInTheDocument();
    expect(request).not.toHaveBeenCalled();
  });
});
