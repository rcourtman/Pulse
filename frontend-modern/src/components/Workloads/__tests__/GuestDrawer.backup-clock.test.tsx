import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { GuestDrawer } from '../GuestDrawer';
import { useGuestDrawerState } from '../useGuestDrawerState';

const policy = vi.hoisted(() => ({ freshHours: 24, staleHours: 72 }));
const calls = vi.hoisted(() => ({ history: vi.fn() }));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    getBackupThresholds: () => policy,
    getMetricThresholds: () => null,
  }),
}));
vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return { ...actual, ChartsAPI: { ...actual.ChartsAPI, getMetricsHistory: calls.history } };
});
vi.mock('@/api/resourceOperatorState', async () => {
  const actual = await vi.importActual<typeof import('@/api/resourceOperatorState')>(
    '@/api/resourceOperatorState',
  );
  return { ...actual, getResourceOperatorState: vi.fn().mockResolvedValue(null) };
});
vi.mock('@/stores/license', () => ({
  isRangeLocked: () => false,
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 90,
}));

const start = Date.parse('2026-10-12T12:00:00Z');
const hour = 3600_000;
const guest = (type: 'qemu' | 'lxc', lastBackup: number | string): WorkloadGuest => ({
  id: 'lab:pve-a:101',
  name: 'backup-clock-fixture',
  vmid: 101,
  instance: 'lab',
  node: 'pve-a',
  type,
  status: 'running',
  cpu: 0.1,
  cpus: 1,
  memory: { total: 1024, used: 256, free: 768, usage: 25 },
  disk: { total: 1024, used: 256, free: 768, usage: 25 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 3600,
  template: false,
  tags: [],
  lastBackup,
});
const details = () => within(screen.getByTestId('guest-technical-details'));
const backupRow = () => details().getByText('Last completed backup').closest('tr')!;
const completion = () => backupRow().querySelector('time');

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(start);
  policy.freshHours = 24;
  policy.staleHours = 72;
  calls.history.mockClear();
});
afterEach(() => {
  cleanup();
  expect(vi.getTimerCount()).toBe(0);
  vi.useRealTimers();
});

describe('mounted completed-backup evidence', () => {
  it('invalidates the saved age policy on time alone, independently of new copy or markup', () => {
    function StateProbe() {
      const state = useGuestDrawerState({
        guest: guest('qemu', start - 24 * hour + 5_000),
        onClose: () => {},
      });
      return <span data-testid="age-policy">{state.backupPresentation().ageClass}</span>;
    }
    render(() => <StateProbe />);
    expect(screen.getByTestId('age-policy')).toHaveTextContent('text-green-600');
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(screen.getByTestId('age-policy')).toHaveTextContent('text-yellow-600');
  });
  it.each(['qemu', 'lxc'] as const)(
    'ages an unchanged %s backup across the default policy boundary',
    (type) => {
      const timestamp = start - 24 * hour + 5_000;
      render(() => <GuestDrawer guest={guest(type, timestamp)} onClose={() => {}} />);
      const drawer = screen.getByTestId('guest-technical-details');
      const row = backupRow();
      expect(within(row).getByText('23 hours ago').closest('td')).toHaveClass('text-emerald-700');
      const originalTime = completion();
      expect(originalTime).toHaveAttribute('datetime', new Date(timestamp).toISOString());
      expect(originalTime).toHaveTextContent('2026-10-11 12:00:05 UTC');
      vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
      expect(within(backupRow()).getByText('1d ago').closest('td')).toHaveClass('text-amber-700');
      expect(completion()).toHaveAttribute('datetime', new Date(timestamp).toISOString());
      expect(screen.getByTestId('guest-technical-details')).toBe(drawer);
      expect(calls.history).not.toHaveBeenCalled();
    },
  );

  it.each(['qemu', 'lxc'] as const)(
    'ages an unchanged %s backup under a one-hour policy',
    (type) => {
      policy.freshHours = 1;
      policy.staleHours = 3;
      render(() => <GuestDrawer guest={guest(type, start - hour + 5_000)} onClose={() => {}} />);
      expect(details().getByText('59 mins ago').closest('td')).toHaveClass('text-emerald-700');
      vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
      expect(details().getByText('1 hour ago').closest('td')).toHaveClass('text-amber-700');
      expect(calls.history).not.toHaveBeenCalled();
    },
  );

  it('does not label a previous-calendar-day completion Today', () => {
    vi.setSystemTime(Date.parse('2026-10-12T00:30:00Z'));
    render(() => <GuestDrawer guest={guest('qemu', '2026-10-11T23:30:00Z')} onClose={() => {}} />);
    expect(details().getByText('1 hour ago')).toBeVisible();
    expect(completion()).toHaveTextContent('2026-10-11 23:30:00 UTC');
    expect(details().queryByText('Today')).not.toBeInTheDocument();
  });

  it('keeps activity separate and ages while another drawer tab is open', () => {
    const timestamp = start - 24 * hour + 5_000;
    const [current, setCurrent] = createSignal({
      ...guest('qemu', timestamp),
      backupInProgress: true,
    });
    render(() => <GuestDrawer guest={current()} onClose={() => {}} />);
    expect(details().getByText('Running · not completed yet')).toBeVisible();
    fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    setCurrent({ ...current(), backupInProgress: false });
    fireEvent.click(screen.getByRole('tab', { name: 'Overview' }));
    expect(details().getByText('1d ago').closest('td')).toHaveClass('text-amber-700');
    expect(completion()).toHaveAttribute('datetime', new Date(timestamp).toISOString());
    expect(details().queryByText('Running · not completed yet')).not.toBeInTheDocument();
    setCurrent({ ...current(), lastBackup: start + RELATIVE_TIME_TICK_MS });
    expect(details().getByText('0s ago').closest('td')).toHaveClass('text-emerald-700');
    expect(completion()).toHaveAttribute('datetime', '2026-10-12T12:00:30.000Z');
    vi.advanceTimersByTime(60_000);
    expect(details().getByText('1 min ago')).toBeVisible();
    expect(calls.history).not.toHaveBeenCalled();
  });

  it('rejudges future and backwards-clock times without manufacturing completion', () => {
    const timestamp = start + 45_000;
    render(() => <GuestDrawer guest={guest('qemu', timestamp)} onClose={() => {}} />);
    expect(
      details()
        .getByText(/timestamp is in the future/)
        .closest('td'),
    ).toHaveClass('text-amber-700');
    expect(completion()).toBeNull();
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(completion()).toBeNull();
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(details().getByText('15s ago').closest('td')).toHaveClass('text-emerald-700');
    expect(completion()).toHaveAttribute('datetime', new Date(timestamp).toISOString());
    vi.setSystemTime(start - hour);
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(
      details()
        .getByText(/timestamp is in the future/)
        .closest('td'),
    ).toHaveClass('text-amber-700');
    expect(completion()).toBeNull();
    expect(details().queryByText('No completed backup found')).not.toBeInTheDocument();
    expect(calls.history).not.toHaveBeenCalled();
  });

  it.each([0, NaN, 'invalid-time'])(
    'does not turn absent or invalid evidence %s into a dated completion',
    (timestamp) => {
      render(() => <GuestDrawer guest={guest('lxc', timestamp)} onClose={() => {}} />);
      const initial = backupRow().textContent;
      vi.advanceTimersByTime(60_000);
      expect(backupRow()).toHaveTextContent(initial!);
      expect(completion()).toBeNull();
      expect(calls.history).not.toHaveBeenCalled();
    },
  );
});
