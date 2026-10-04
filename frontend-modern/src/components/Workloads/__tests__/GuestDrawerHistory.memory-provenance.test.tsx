import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse } from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { WorkloadGuest } from '@/types/workloads';
import { GuestDrawerHistory } from '../GuestDrawerHistory';
import {
  getGuestDrawerCurrentMetrics,
  getGuestDrawerDeferredMetrics,
  getGuestDrawerMemoryRows,
  getGuestDrawerMemoryReading,
} from '../guestDrawerModel';

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
const original = '2026-10-04T14:00:00Z';
const later = '2026-10-04T17:00:00Z';
const annotation = (
  state: string,
  observedAt: string | undefined = original,
  source = 'guest-agent-meminfo',
) => ({ state, source, ...(observedAt === undefined ? {} : { observedAt }) });
const guest = (observation?: ReturnType<typeof annotation>, usage = 25): WorkloadGuest => {
  const memory = {
    total: 100,
    used: usage,
    free: 100 - usage,
    usage,
    ...(observation ? { observation } : {}),
  };
  return {
    id: 'fixture:pve1:101',
    vmid: 101,
    name: 'backup-guest',
    node: 'pve1',
    instance: 'fixture',
    type: 'qemu',
    status: 'running',
    cpu: 0.1,
    cpus: 2,
    memory,
    disk: { total: 100, used: 50, usage: 50 },
    networkIn: 100,
    networkOut: 200,
    diskRead: 300,
    diskWrite: 400,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: later,
  };
};
const response = (
  metrics: AllMetricsHistoryResponse['metrics'] = {},
): AllMetricsHistoryResponse => ({
  resourceType: 'vm',
  resourceId: 'fixture:pve1:101',
  range: '1h',
  start: Date.parse(original),
  end: Date.parse(later),
  source: 'store',
  metrics,
});
const mount = (value: () => WorkloadGuest) =>
  render(() => (
    <GuestDrawerHistory
      target={{ resourceType: 'vm', resourceId: 'fixture:pve1:101' }}
      range="1h"
      currentMetrics={getGuestDrawerCurrentMetrics(value())}
      deferredMetrics={getGuestDrawerDeferredMetrics(value())}
    />
  ));
const utilization = () => screen.getAllByTestId('guest-history-group-chart')[0];

describe('guest memory original-observation provenance', () => {
  it.each(['qemu', 'lxc'] as const)(
    'retains %s memory as evidence without inventing current freshness',
    (type) => {
      const value = { ...guest(annotation('last-known')), type };
      expect(getGuestDrawerCurrentMetrics(value).memory).toBeUndefined();
      expect(getGuestDrawerDeferredMetrics(value).memory).toMatchObject({
        lastKnownValue: 25,
        message: expect.stringContaining('2026-10-04 14:00:00 UTC'),
      });
      expect(getGuestDrawerMemoryReading(value)?.summary).toContain('Last known');
      expect(getGuestDrawerCurrentMetrics(value)).toMatchObject({ cpu: 10, disk: 50, netin: 100 });
    },
  );

  it('never borrows disk deferral or refreshed guest LastSeen for memory provenance', () => {
    const retained = {
      ...guest(annotation('last-known')),
      diskStatusReason: undefined,
      lock: '',
      lastSeen: later,
    };
    expect(getGuestDrawerCurrentMetrics(retained).memory).toBeUndefined();
    expect(getGuestDrawerDeferredMetrics(retained).memory.message).toContain('14:00:00 UTC');
    expect(getGuestDrawerDeferredMetrics(retained).memory.message).not.toContain('17:00:00');
    const pve = {
      ...guest(annotation('current', original, 'status-mem')),
      diskStatusReason: 'prev-vm-locked',
      lock: 'backup',
    };
    expect(getGuestDrawerCurrentMetrics(pve).memory).toBe(25);
    expect(getGuestDrawerDeferredMetrics(pve).memory).toBeUndefined();
    expect(getGuestDrawerDeferredMetrics(pve).disk.lastKnownValue).toBe(50);
  });

  it('keeps unavailable numbers out of current and retained legends and usage breakdown', () => {
    const value = guest(annotation('unavailable'), 0);
    expect(getGuestDrawerCurrentMetrics(value).memory).toBeUndefined();
    expect(getGuestDrawerDeferredMetrics(value).memory.lastKnownValue).toBeUndefined();
    expect(getGuestDrawerMemoryRows(value)).toContainEqual({
      label: 'Usage',
      value: 'Unavailable',
    });
    expect(getGuestDrawerMemoryRows(value).some((row) => row.label === 'Free')).toBe(false);
    expect(getGuestDrawerMemoryRows(value)).toContainEqual({ label: 'Total', value: '100 B' });
  });

  it.each([
    undefined,
    annotation('private raw state'),
    annotation('current', 'invalid'),
    annotation('current', '0001-01-01T00:00:00Z'),
    annotation('current', '2999-01-01T00:00:00Z'),
  ])('does not promote unknown legacy or unusable memory provenance %#', (observation) => {
    const value = guest(observation);
    expect(getGuestDrawerCurrentMetrics(value).memory).toBeUndefined();
    expect(getGuestDrawerDeferredMetrics(value).memory).toMatchObject({
      lastKnownValue: 25,
      valueLabel: 'freshness unknown',
    });
    expect(getGuestDrawerDeferredMetrics(value).memory.message).not.toMatch(
      /private raw state|2999|0001/,
    );
  });

  it.each([-1, NaN, Infinity, 101])('does not expose invalid retained memory %s', (usage) => {
    expect(
      getGuestDrawerDeferredMetrics(guest(annotation('last-known'), usage)).memory.lastKnownValue,
    ).toBeUndefined();
  });

  it('retains unknown original age without exposing a raw source string', () => {
    const value = guest({ state: 'last-known', source: 'private provider detail' });
    expect(getGuestDrawerMemoryReading(value)?.summary).toBe(
      'Last known · Unknown source · time unknown',
    );
    expect(getGuestDrawerDeferredMetrics(value).memory.message).toContain(
      'Observation time unknown.',
    );
    expect(getGuestDrawerDeferredMetrics(value).memory.message).not.toContain(
      'private provider detail',
    );
  });

  it('retains measured zero and leaves unannotated non-Proxmox data unchanged', () => {
    expect(getGuestDrawerDeferredMetrics(guest(annotation('last-known'), 0)).memory).toBeDefined();
    expect(
      getGuestDrawerDeferredMetrics(guest(annotation('last-known'), 0)).memory.lastKnownValue,
    ).toBe(0);
    const vsphere = {
      ...guest(),
      vmid: 0,
      instance: '',
      node: '',
      platformScopes: ['vmware-vsphere'],
    };
    expect(getGuestDrawerCurrentMetrics(vsphere).memory).toBe(25);
    expect(getGuestDrawerDeferredMetrics(vsphere)).toEqual({});
  });

  it('updates same-guest retained, unavailable and independent live source without remount or refetch', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    const [value, setValue] = createSignal(guest(annotation('last-known')));
    mount(value);
    await waitFor(() =>
      expect(screen.getAllByText('No stored history in this range')).toHaveLength(3),
    );
    const chart = utilization();
    expect(chart.querySelector('[data-history-current="memory"]')).toBeNull();
    expect(chart.querySelector('[data-history-last-known="memory"]')).toHaveTextContent(
      'Memory25.0%last known',
    );
    expect(chart.querySelector('[data-history-deferred="memory"]')).toHaveTextContent(
      '14:00:00 UTC',
    );
    setValue({ ...value(), lastSeen: later, lock: 'backup', diskStatusReason: 'prev-vm-locked' });
    expect(chart.querySelector('[data-history-deferred="memory"]')).not.toHaveTextContent(
      '17:00:00',
    );
    setValue(guest(annotation('unavailable'), 0));
    expect(chart.querySelector('[data-history-current="memory"]')).toBeNull();
    expect(chart.querySelector('[data-history-last-known="memory"]')).toBeNull();
    expect(chart).toHaveTextContent('Memory-');
    setValue(guest(annotation('current', later, 'agent'), 35));
    expect(utilization()).toBe(chart);
    expect(chart.querySelector('[data-history-deferred="memory"]')).toBeNull();
    expect(chart.querySelector('[data-history-current="memory"]')).toHaveTextContent(
      'Memory35.0%current',
    );
    expect(chart.querySelector('path')).toBeNull();
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it('keeps stored points and dated inspection distinct from retained live memory', async () => {
    const point = (timestamp: string, value: number) => ({
      timestamp: Date.parse(timestamp),
      value,
      min: value,
      max: value,
    });
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({ memory: [point(original, 20), point(later, 30)] }),
    );
    mount(() => guest(annotation('last-known'), 25));
    await waitFor(() => expect(utilization().querySelectorAll('path')).toHaveLength(1));
    const chart = utilization();
    expect(chart).toHaveTextContent('Memory30.0%');
    expect(chart.querySelector('[data-history-current="memory"]')).toBeNull();
    expect(chart.querySelector('[data-history-last-known="memory"]')).toBeNull();
    const slider = within(chart).getByRole('slider');
    slider.focus();
    fireEvent.input(slider, { target: { value: '0' } });
    expect(chart).toHaveTextContent('Memory20.0%');
    expect(slider).toHaveAttribute('aria-valuetext', expect.stringContaining('Memory 20.0%.'));
  });

  it('withdraws retained annotations on denied History without leaking raw errors', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockRejectedValueOnce(Object.assign(new Error('private backend refusal'), { status: 403 }));
    mount(() => guest(annotation('last-known')));
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
    expect(screen.queryByText('private backend refusal')).not.toBeInTheDocument();
  });
});
