import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse } from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { WorkloadGuest } from '@/types/workloads';
import { GuestDrawerHistory } from '../GuestDrawerHistory';
import { getGuestDrawerCurrentMetrics, getGuestDrawerDeferredMetrics } from '../guestDrawerModel';
import { guestDiskDeferrals } from '../__fixtures__/guestDiskDeferrals';

vi.mock('@/stores/license', () => ({
  loadRuntimeCapabilities: vi.fn(async () => undefined),
  maxHistoryDays: () => 90,
  isRangeLocked: () => false,
}));

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
});

const guest = (reason: string, usage = 50): WorkloadGuest => ({
  id: 'fixture:pve-a:101',
  vmid: 101,
  name: 'backup-guest',
  node: 'pve-a',
  instance: 'fixture',
  type: 'qemu',
  status: 'running',
  cpu: 0.1,
  cpus: 2,
  memory: { total: 100, used: 25, free: 75, usage: 25 },
  disk: { total: 100, used: usage, usage },
  networkIn: 100,
  networkOut: 200,
  diskRead: 300,
  diskWrite: 400,
  uptime: 3600,
  template: false,
  lastBackup: 0,
  tags: [],
  lock: '',
  diskStatusReason: reason,
  lastSeen: '2026-10-04T03:00:00Z',
});
const start = Date.UTC(2026, 9, 4, 2);
const end = start + 60 * 60_000;
const response = (
  metrics: AllMetricsHistoryResponse['metrics'] = {},
): AllMetricsHistoryResponse => ({
  resourceType: 'vm',
  resourceId: 'fixture:pve-a:101',
  range: '1h',
  start,
  end,
  metrics,
  source: 'store',
});
const point = (timestamp: number, value: number) => ({ timestamp, value, min: value, max: value });
const target = { resourceType: 'vm' as const, resourceId: 'fixture:pve-a:101' };
const mount = (value: () => WorkloadGuest) =>
  render(() => (
    <GuestDrawerHistory
      target={target}
      range="1h"
      currentMetrics={getGuestDrawerCurrentMetrics(value())}
      deferredMetrics={getGuestDrawerDeferredMetrics(value())}
    />
  ));
const utilization = () => screen.getAllByTestId('guest-history-group-chart')[0];

// Actual backend provenance remains authoritative. Removing a VM lock alone
// cannot turn retained readings into current data or prove native thaw.
describe('GuestDrawerHistory filesystem provenance', () => {
  it.each(guestDiskDeferrals)(
    'labels retained %s as last known without creating a stored observation',
    async (reason, message) => {
      const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
      mount(() => guest(`prev-${reason}`));
      await waitFor(() =>
        expect(screen.getAllByText('No stored history in this range')).toHaveLength(3),
      );
      const chart = utilization();
      expect(chart.querySelector('[data-history-current="disk"]')).toBeNull();
      const retained = chart.querySelector('[data-history-last-known="disk"]')!;
      expect(retained).toHaveTextContent('Disk50.0%last known');
      expect(retained).toHaveAccessibleDescription(
        `Disk live reading: Using last known disk stats. ${message}`,
      );
      expect(chart.querySelector('[data-history-deferred="disk"]')).toBeVisible();
      expect(chart.querySelector('[data-history-current="cpu"]')).toHaveTextContent(
        'CPU10.0%current',
      );
      expect(chart.querySelector('[data-history-current="memory"]')).toHaveTextContent(
        'Memory25.0%current',
      );
      expect(chart.querySelector('path')).toBeNull();
      expect(chart.querySelector('[data-history-observation]')).toBeNull();
      expect(fetch).toHaveBeenCalledTimes(1);
    },
  );

  it.each(guestDiskDeferrals)(
    'shows unavailable %s without promoting an unretained numeric summary',
    async (reason, message) => {
      vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
      mount(() => guest(reason));
      await waitFor(() =>
        expect(screen.getAllByText('No stored history in this range')).toHaveLength(3),
      );
      const chart = utilization();
      expect(chart.querySelector('[data-history-current="disk"]')).toBeNull();
      expect(chart.querySelector('[data-history-last-known]')).toBeNull();
      expect(chart).toHaveTextContent('Disk-');
      expect(chart.querySelector('[data-history-deferred="disk"]')).toHaveTextContent(message);
    },
  );

  it.each([-1, NaN, Infinity, undefined])(
    'does not display unusable retained disk usage %s',
    (usage) => {
      const value = guest('prev-vm-locked');
      value.disk.usage = usage as number;
      expect(getGuestDrawerCurrentMetrics(value).disk).toBeUndefined();
      expect(getGuestDrawerDeferredMetrics(value).disk.lastKnownValue).toBeUndefined();
      expect(getGuestDrawerDeferredMetrics(value).disk.message).toContain('Guest reads paused');
    },
  );

  it('keeps retained zero valid and honours explicit disk telemetry unavailability', () => {
    const value = guest('prev-agent-busy', 0);
    expect(getGuestDrawerDeferredMetrics(value).disk.lastKnownValue).toBe(0);
    value.telemetryAvailability = {
      cpu: true,
      memory: true,
      disk: false,
      networkIO: true,
      diskIO: true,
      uptime: true,
    };
    expect(getGuestDrawerDeferredMetrics(value).disk.lastKnownValue).toBeUndefined();
    expect(getGuestDrawerCurrentMetrics(value).disk).toBeUndefined();
    expect(getGuestDrawerCurrentMetrics(value)).toMatchObject({
      cpu: 10,
      memory: 25,
      diskread: 300,
      diskwrite: 400,
    });
  });

  it('keeps non-VM data independent and maps unknown reason text without exposing it', () => {
    for (const type of ['lxc', 'app-container'] as const) {
      const value = { ...guest('prev-vm-locked'), type };
      expect(getGuestDrawerDeferredMetrics(value)).toEqual({});
      expect(getGuestDrawerCurrentMetrics(value).disk).toBe(50);
    }
    const value = guest('private transport error');
    expect(getGuestDrawerDeferredMetrics(value).disk.message).toBe(
      'Disk stats unavailable. Guest agent may not be installed.',
    );
    expect(getGuestDrawerCurrentMetrics(value).disk).toBeUndefined();
  });

  it('keeps real stored points and dated keyboard inspection separate from live deferral', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        disk: [point(start, 20), point(end, 30)],
        cpu: [point(start, 10), point(end, 15)],
      }),
    );
    mount(() => guest('prev-agent-timeout'));
    await waitFor(() => expect(utilization().querySelectorAll('path')).toHaveLength(2));
    const chart = utilization();
    expect(chart).toHaveTextContent('Disk30.0%');
    expect(chart.querySelector('[data-history-last-known]')).toBeNull();
    expect(chart.querySelector('[data-history-deferred="disk"]')).toHaveTextContent(
      'Completion is uncertain. Do not restart the guest agent during a backup.',
    );
    const slider = within(chart).getByRole('slider');
    slider.focus();
    fireEvent.input(slider, { target: { value: '0' } });
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      `${new Date(start).toLocaleString()}. CPU 10.0%. Memory no observation. Disk 20.0%.`,
    );
    expect(chart).toHaveTextContent('Disk20.0%');
    expect(chart.querySelector('[data-history-current]')).toBeNull();
    expect(chart.querySelector('[data-history-deferred="disk"]')).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("does not borrow retained live disk evidence during another metric's dated inspection", async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({ cpu: [point(start, 10), point(end, 15)] }),
    );
    mount(() => guest('prev-vm-locked'));
    await waitFor(() => expect(screen.getByRole('slider')).toBeInTheDocument());
    const slider = screen.getByRole('slider');
    slider.focus();
    fireEvent.input(slider, { target: { value: '0' } });
    expect(utilization()).toHaveTextContent('Disk-');
    expect(utilization().querySelector('[data-history-last-known]')).toBeNull();
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      expect.stringContaining('Disk no observation.'),
    );
    fireEvent.blur(slider);
    expect(utilization().querySelector('[data-history-last-known="disk"]')).toHaveTextContent(
      '50.0%last known',
    );
  });

  it('withdraws retained context only with fresh same-source data, without refetch or remount', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    const [value, setValue] = createSignal(guest('prev-vm-locked'));
    mount(value);
    await waitFor(() =>
      expect(screen.getAllByText('No stored history in this range')).toHaveLength(3),
    );
    const chart = utilization();
    setValue({ ...value(), lock: '', diskStatusReason: 'prev-agent-busy' });
    expect(chart.querySelector('[data-history-deferred="disk"]')).toHaveTextContent(
      'still in progress',
    );
    expect(chart.querySelector('[data-history-current="disk"]')).toBeNull();
    setValue(guest('', 75));
    expect(utilization()).toBe(chart);
    expect(chart.querySelector('[data-history-deferred]')).toBeNull();
    expect(chart.querySelector('[data-history-last-known]')).toBeNull();
    expect(chart.querySelector('[data-history-current="disk"]')).toHaveTextContent('75.0%current');
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it('does not reveal retained evidence through denied History', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockRejectedValueOnce(Object.assign(new Error('private refusal'), { status: 403 }))
      .mockResolvedValueOnce(response());
    mount(() => guest('prev-vm-locked'));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Refresh history' })).toHaveAttribute(
        'aria-busy',
        'false',
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Retry history' })).toBeInTheDocument(),
    );
    expect(screen.queryByTestId('guest-history-group-chart')).not.toBeInTheDocument();
    expect(document.querySelector('[data-history-deferred]')).toBeNull();
    expect(screen.queryByText('private refusal')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry history' }));
    await waitFor(() =>
      expect(utilization().querySelector('[data-history-last-known]')).not.toBeNull(),
    );
    expect(fetch).toHaveBeenCalledTimes(3);
  });
});
