import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createResource, createSignal, Show } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ResourceAPI } from '@/api/resources';
import type { HostCustomSensorMetric } from '@/types/api';
import type { Resource } from '@/types/resource';
import { clearAuth, setOrgID } from '@/utils/apiClient';
import { nodeFromResource } from '@/utils/resourceStateAdapters';

const chartsApiMocks = vi.hoisted(() => ({ getMetricsHistory: vi.fn() }));
vi.mock('@/api/charts', async () => ({
  ...(await vi.importActual<typeof import('@/api/charts')>('@/api/charts')),
  ChartsAPI: { getMetricsHistory: chartsApiMocks.getMetricsHistory },
}));
vi.mock('@/stores/license', () => ({
  isRangeLocked: () => false,
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 7,
}));

import { NodeDrawer } from './NodeDrawer';
import './GuestDrawerHistory';

const metric = (overrides: Partial<HostCustomSensorMetric> = {}): HostCustomSensorMetric => ({
  id: 'vram-101',
  name: 'VRAM',
  group: 'GPU',
  subgroup: 'LXC 101',
  kind: 'number',
  unit: 'bytes',
  value: 1024 ** 3,
  status: 'ok',
  observedAt: '2026-11-17T17:00:00Z',
  ...overrides,
});

const resource = (custom?: HostCustomSensorMetric[], stale = false): Resource => ({
  id: 'node-fixture',
  type: 'agent',
  name: 'pve-fixture',
  displayName: 'PVE lab node',
  platformId: 'lab',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  sources: ['proxmox-pve', 'agent'],
  status: 'online',
  lastSeen: 1794934800000,
  cpu: { current: 14 },
  memory: { current: 25, total: 16 * 1024 ** 3, used: 4 * 1024 ** 3 },
  disk: { current: 35, total: 100 * 1024 ** 3, used: 35 * 1024 ** 3 },
  platformData: {
    linkedAgentId: 'agent:linked-fixture',
    proxmox: { nodeName: 'pve-fixture', pveVersion: 'pve-manager/9.1.0' },
  },
  agent: {
    agentId: 'linked-fixture',
    stale,
    sensors: {
      custom,
      powerWatts: { cpu_package: 12.9 },
      gpu: [{ id: '0', name: 'NVIDIA fixture GPU', utilizationPercent: 15 }],
    },
  },
});

const customRows = () => within(screen.getByTestId('node-drawer-custom-metrics'));

beforeEach(() => {
  clearAuth();
  setOrgID('default');
  chartsApiMocks.getMetricsHistory.mockResolvedValue({
    resourceType: 'agent',
    resourceId: 'linked-fixture',
    range: '24h',
    start: 1794848400000,
    end: 1794934800000,
    source: 'store',
    metrics: {},
  });
});

afterEach(() => {
  cleanup();
  clearAuth();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe('linked-node custom readings', () => {
  it('renders the real resource read through the node adapter without another lookup or write', async () => {
    const readings = [
      metric({ id: 'vram-202', subgroup: 'LXC 202', value: 2 * 1024 ** 3, status: 'warning' }),
      metric(),
      metric({ id: 'fan', name: 'Fan', subgroup: undefined, unit: 'rpm', value: 0 }),
      metric({
        id: 'core',
        name: 'Core temperature',
        unit: 'celsius',
        value: 91,
        status: 'critical',
      }),
      metric({ id: 'load', name: 'Utilization', unit: 'percent', value: 12.5 }),
    ];
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify(resource(readings)), {
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);
    render(() => {
      const [snapshot] = createResource(() => ResourceAPI.getResource('node-fixture'));
      return (
        <Show when={snapshot()}>
          {(current) => <NodeDrawer node={nodeFromResource(current())!} />}
        </Show>
      );
    });

    await screen.findByTestId('node-drawer-custom-metrics');
    const rows = customRows();
    for (const label of ['GPU / LXC 101 / VRAM', 'GPU / LXC 202 / VRAM']) {
      expect(rows.getByText(label)).toBeVisible();
    }
    for (const value of [
      '1,073,741,824 bytes · OK',
      '2,147,483,648 bytes · Warning',
      '0 rpm · OK',
      '91 celsius · Critical',
      '12.5 percent · OK',
    ]) {
      expect(rows.getByText(value)).toBeVisible();
    }
    expect(rows.getAllByRole('row')).toHaveLength(6);
    expect(screen.getByTestId('node-drawer-custom-metrics').closest('details')).toBeNull();
    expect(screen.getByTestId('node-technical-details')).not.toHaveAttribute('open');
    expect(screen.getByText('NVIDIA fixture GPU · 15%')).toBeVisible();
    expect(screen.getByRole('tab', { name: 'Manage' })).toBeVisible();
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/resources/node-fixture',
      expect.objectContaining({
        cache: 'no-store',
        credentials: 'include',
        headers: expect.objectContaining({ 'X-Pulse-Org-ID': 'default' }),
      }),
    );
  });

  it('shows unavailable, stale, error, unknown and typed values inline without parsing labels as markup', () => {
    const label = '<img src=x onerror=alert(1)> مختبر';
    render(() => (
      <NodeDrawer
        node={nodeFromResource(
          resource([
            metric({ value: undefined, status: 'error', error: 'Collector timed out' }),
            metric({
              id: 'zero',
              name: label,
              value: 0,
              unit: 'percent',
              stale: true,
              status: 'unknown',
            }),
            metric({
              id: 'bool',
              name: 'Reachable',
              kind: 'boolean',
              value: 0,
              unit: undefined,
              status: 'critical',
            }),
            metric({
              id: 'age',
              name: 'Last check',
              kind: 'timestamp',
              value: 3600,
              unit: undefined,
              status: 'warning',
            }),
            metric({ id: 'invalid', name: 'Invalid reading', value: NaN }),
            metric({ id: '', name: 'Missing identity' }),
            metric({ id: 'missing-name', name: '' }),
          ]),
        )!}
      />
    ));
    const rows = customRows();
    expect(rows.getByText('Unavailable · Error · Collector timed out')).toBeVisible();
    expect(rows.getByText('0 percent (stale) · Unknown')).toBeVisible();
    expect(rows.getByText('Offline · Critical')).toBeVisible();
    expect(rows.getByText('1h ago · Warning')).toBeVisible();
    expect(rows.getByText('Unavailable · OK')).toBeVisible();
    expect(rows.getByText(`GPU / LXC 101 / ${label}`)).toBeVisible();
    expect(screen.getByTestId('node-drawer-custom-metrics').querySelector('img')).toBeNull();
    expect(rows.queryByText('Missing identity')).toBeNull();
    expect(rows.getAllByRole('row')).toHaveLength(6);
  });

  it('updates the open drawer, qualifies a silent linked agent and removes withdrawn readings', () => {
    const [snapshot, setSnapshot] = createSignal(resource([metric()]));
    render(() => <NodeDrawer node={nodeFromResource(snapshot())!} />);
    const details = screen.getByTestId('node-technical-details') as HTMLDetailsElement;
    expect(customRows().getByText('1,073,741,824 bytes · OK')).toBeVisible();

    setSnapshot(resource([metric({ value: 0, status: 'warning' })]));
    expect(customRows().queryByText('1,073,741,824 bytes · OK')).toBeNull();
    expect(customRows().getByText('0 bytes · Warning')).toBeVisible();

    setSnapshot(resource([metric({ value: 0, status: 'warning' })], true));
    expect(
      customRows().getByText('0 bytes · Warning (last known: host agent stopped reporting)'),
    ).toBeVisible();
    setSnapshot(resource([metric({ value: 0, status: 'warning', stale: true })], true));
    expect(
      customRows().getByText(
        '0 bytes (stale) · Warning (last known: host agent stopped reporting)',
      ),
    ).toBeVisible();

    setSnapshot(resource([]));
    expect(screen.queryByTestId('node-drawer-custom-metrics')).toBeNull();
    expect(screen.getByText('NVIDIA fixture GPU · 15%')).toBeVisible();
    expect(screen.getByTestId('node-technical-details')).toBe(details);
    setSnapshot(resource([metric({ name: 'Recovered' })]));
    expect(customRows().getByText('GPU / LXC 101 / Recovered')).toBeVisible();
    expect(customRows().queryByText(/last known/)).toBeNull();

    setSnapshot({ ...resource(), agent: undefined });
    expect(screen.queryByTestId('node-drawer-custom-metrics')).toBeNull();
    expect(screen.queryByText('GPU / LXC 101 / Recovered')).toBeNull();
  });

  it.each([undefined, [], [metric({ id: '' }), metric({ name: ' ' })]])(
    'omits an empty custom section without inventing sensor readings (%j)',
    (readings) => {
      render(() => <NodeDrawer node={nodeFromResource(resource(readings))!} />);
      expect(screen.queryByTestId('node-drawer-custom-metrics')).toBeNull();
      expect(screen.getByText('NVIDIA fixture GPU · 15%')).toBeVisible();
    },
  );
});
