import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ResourceOperatorState } from '@/api/resourceOperatorState';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';

const api = vi.hoisted(() => ({ apiFetchJSON: vi.fn() }));
vi.mock('@/utils/apiClient', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/utils/apiClient')>()),
  apiFetchJSON: api.apiFetchJSON,
}));

import { ResourceOperatorStateSection } from '../ResourceOperatorStateSection';

const start = Date.parse('2026-10-11T04:00:00Z');
const iso = (offset: number) => new Date(start + offset).toISOString();
const state = (overrides: Partial<ResourceOperatorState> = {}): ResourceOperatorState => ({
  canonicalId: 'clock-resource',
  monitoringMode: 'expected_offline',
  lifecycleState: 'retired',
  intentionallyOffline: true,
  neverAutoRemediate: true,
  note: 'Keep the lock',
  setAt: iso(-60_000),
  setBy: 'fixture-admin',
  ...overrides,
});
const mount = async (record: ResourceOperatorState) => {
  api.apiFetchJSON.mockResolvedValue({ configured: true, state: record });
  render(() => <ResourceOperatorStateSection resourceId={record.canonicalId} />);
  await vi.advanceTimersByTimeAsync(0);
};

describe('held operator maintenance evidence', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(start);
    api.apiFetchJSON.mockReset();
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('moves a held one-off window from scheduled to active to expired at its half-open boundaries', async () => {
    await mount(state({ maintenanceStartAt: iso(30_000), maintenanceEndAt: iso(60_000) }));
    expect(screen.getByText('Maintenance window scheduled.')).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
    expect(screen.queryByText('Maintenance window scheduled.')).not.toBeInTheDocument();
    expect(screen.getByText('Maintenance window active.')).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
    expect(screen.queryByText('Maintenance window active.')).not.toBeInTheDocument();
    expect(screen.queryByText('Maintenance window scheduled.')).not.toBeInTheDocument();
    // Clock invalidation is presentation-only: no refetch, mutation or lock release.
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(1);
    expect(screen.getByLabelText('Never auto-remediate this resource')).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByLabelText('Monitoring')).toHaveValue('expected_offline');
    expect(screen.getByLabelText('Lifecycle')).toHaveValue('retired');
  });

  it('expires an initially active one-off badge without discarding unsaved policy or schedule edits', async () => {
    await mount(state({ maintenanceStartAt: iso(-60_000), maintenanceEndAt: iso(30_000) }));
    fireEvent.input(screen.getByLabelText('Operator note'), { target: { value: 'Unsaved note' } });
    fireEvent.click(screen.getByText('Edit window'));
    const reason = screen.getByLabelText('Reason (optional)');
    fireEvent.input(reason, { target: { value: 'Unsaved reason' } });
    await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
    expect(screen.queryByText('Maintenance window active.')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Operator note')).toHaveValue('Unsaved note');
    expect(reason).toHaveValue('Unsaved reason');
    expect(screen.getByText('Save overrides')).toBeInTheDocument();
    expect(screen.getByText('Schedule maintenance window')).toBeInTheDocument();
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(1);
  });

  it.each([false, true])(
    'qualifies an expired server-active report without inferring recurrence or resumption (recurring: %s)',
    async (recurring) => {
      await mount(
        state({
          maintenanceWindowActive: true,
          maintenanceActiveEndAt: iso(30_000),
          maintenanceRecurrence: recurring
            ? {
                timezone: 'UTC',
                weekdays: ['sunday'],
                startMinute: 240,
                endMinute: 241,
              }
            : undefined,
          maintenanceScope: 'resource_and_descendants',
        }),
      );
      expect(screen.getByText('Maintenance window active.')).toBeInTheDocument();
      await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
      expect(screen.queryByText('Maintenance window active.')).not.toBeInTheDocument();
      const report = screen.getByText('Last reported maintenance window.').parentElement;
      expect(report).toHaveTextContent('Current maintenance status has not been refreshed.');
      expect(report).toHaveTextContent('Attention was reported paused');
      expect(report).toHaveTextContent('and its descendants');
      expect(screen.queryByText('Recurring maintenance configured.')).not.toBeInTheDocument();
      expect(api.apiFetchJSON).toHaveBeenCalledTimes(1);
    },
  );

  it('does not invent an active verdict for a recurring rule whose server verdict is inactive', async () => {
    await mount(
      state({
        maintenanceWindowActive: false,
        maintenanceRecurrence: {
          timezone: 'UTC',
          weekdays: ['sunday'],
          startMinute: 240,
          endMinute: 241,
        },
      }),
    );
    await vi.advanceTimersByTimeAsync(60_000);
    expect(screen.getByText('Recurring maintenance configured.')).toBeInTheDocument();
    expect(screen.queryByText('Maintenance window active.')).not.toBeInTheDocument();
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(1);
  });

  it.each([
    {},
    { maintenanceStartAt: iso(-60_000), maintenanceEndAt: iso(-30_000) },
    { maintenanceStartAt: 'invalid', maintenanceEndAt: 'invalid' },
    { maintenanceStartAt: iso(-30_000) },
  ])('keeps absent, expired, invalid or incomplete windows inactive (%j)', async (window) => {
    await mount(state(window));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(screen.queryByText('Maintenance window active.')).not.toBeInTheDocument();
    expect(screen.queryByText('Maintenance window scheduled.')).not.toBeInTheDocument();
  });

  it('does not revive a removed window on a clock tick after the drawer changes resource', async () => {
    api.apiFetchJSON
      .mockResolvedValueOnce({
        configured: true,
        state: state({ maintenanceStartAt: iso(-60_000), maintenanceEndAt: iso(120_000) }),
      })
      .mockResolvedValueOnce({ configured: false });
    const [resourceId, setResourceId] = createSignal('clock-resource-removal');
    render(() => <ResourceOperatorStateSection resourceId={resourceId()} />);
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByText('Maintenance window active.')).toBeInTheDocument();
    setResourceId('no-operator-record');
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(RELATIVE_TIME_TICK_MS);
    expect(screen.queryByText('Maintenance window active.')).not.toBeInTheDocument();
    expect(screen.queryByText('Maintenance window scheduled.')).not.toBeInTheDocument();
    expect(api.apiFetchJSON).toHaveBeenCalledTimes(2);
  });
});
