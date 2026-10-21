import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AlertsAPI } from '@/api/alerts';
import { apiFetchJSON } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import { useAlertAcknowledgementState } from '../useAlertAcknowledgementState';
import { isAlertSnoozed, useAlertSnoozeState } from '../useAlertSnoozeState';

// Only transport is mocked: exercise the real response admission and state owners.
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));
const until = '2026-10-21T14:00:00.000Z';
const initial: Alert = {
  id: 'cpu:vm/100',
  type: 'cpu',
  level: 'critical',
  resourceId: 'vm/100',
  resourceName: 'Database',
  node: 'pve-a',
  instance: 'synthetic',
  message: 'Synthetic CPU alert',
  value: 95,
  threshold: 80,
  startTime: '2026-10-21T11:00:00Z',
  acknowledged: false,
};
const actions = ['acknowledge', 'unacknowledge', 'snooze', 'unsnooze'] as const;
const unconfirmed: unknown[] = [
  undefined,
  null,
  {},
  [],
  'success',
  { success: false },
  { success: 'true' },
  { success: 1 },
  { success: true, error: 'Synthetic diagnostic must not be echoed' },
];
const call = (action: (typeof actions)[number]) =>
  action === 'snooze' ? AlertsAPI.snooze(initial.id, until) : AlertsAPI[action](initial.id);

describe('alert action confirmation through real adapters and state', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-10-21T12:00:00Z'));
    vi.resetAllMocks();
  });
  afterEach(() => vi.useRealTimers());

  describe.each(actions)('%s', (action) => {
    it.each(unconfirmed)('does not acknowledge a malformed/rejected 2xx body: %j', async (body) => {
      vi.mocked(apiFetchJSON).mockResolvedValue(body);
      await expect(call(action)).rejects.toThrow(/could not be confirmed/);
      expect(apiFetchJSON).toHaveBeenCalledTimes(1);
      expect(apiFetchJSON).toHaveBeenCalledWith(`/api/alerts/${action}`, {
        method: 'POST',
        body: JSON.stringify({
          alertIdentifier: initial.id,
          ...(action === 'snooze' ? { until } : {}),
        }),
      });
    });
    it('preserves the affirmative producer contract and sends once', async () => {
      const response =
        action === 'snooze'
          ? { success: true, snoozedUntil: '2026-10-21T14:00:00Z' }
          : { success: true };
      vi.mocked(apiFetchJSON).mockResolvedValue(response);
      expect(await call(action)).toEqual(response);
      expect(apiFetchJSON).toHaveBeenCalledTimes(1);
    });
    it('preserves a transport failure without an automatic second write', async () => {
      const failure = new Error('Synthetic transport failure');
      vi.mocked(apiFetchJSON).mockRejectedValue(failure);
      await expect(call(action)).rejects.toBe(failure);
      expect(apiFetchJSON).toHaveBeenCalledTimes(1);
    });
  });
  it.each([undefined, null, 0, '', 'not-a-date', '2026-10-21T15:00:00Z'])(
    'does not confirm another or missing snooze expiry: %j',
    async (snoozedUntil) => {
      vi.mocked(apiFetchJSON).mockResolvedValue({ success: true, snoozedUntil });
      await expect(AlertsAPI.snooze(initial.id, until)).rejects.toThrow(/could not be confirmed/);
      expect(apiFetchJSON).toHaveBeenCalledTimes(1);
    },
  );
  it('accepts an equivalent UTC instant and keeps an explicit acknowledge actor in the body', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      success: true,
      snoozedUntil: '2026-10-21T15:00:00+01:00',
    });
    await expect(AlertsAPI.snooze(initial.id, until)).resolves.toHaveProperty('success', true);
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true });
    await AlertsAPI.acknowledge(initial.id, 'synthetic-operator');
    expect(apiFetchJSON).toHaveBeenLastCalledWith('/api/alerts/acknowledge', {
      method: 'POST',
      body: JSON.stringify({ alertIdentifier: initial.id, user: 'synthetic-operator' }),
    });
  });
  it.each([false, true])(
    'does not change acknowledgement after an unconfirmed action (previous=%s)',
    async (acknowledged) => {
      const updateAlert = vi.fn();
      const { result, cleanup } = renderHook(() =>
        useAlertAcknowledgementState({
          alerts: () => [{ ...initial, acknowledged }],
          updateAlert,
          allowRestore: true,
        }),
      );
      try {
        vi.mocked(apiFetchJSON).mockResolvedValue({ success: false });
        await result.handleAlertAcknowledgement({ ...initial, acknowledged });
        expect(updateAlert).not.toHaveBeenCalled();
        expect(notificationStore.success).not.toHaveBeenCalled();
        expect(notificationStore.error).toHaveBeenCalledWith(
          expect.stringMatching(/could not be confirmed.*Refresh alerts/),
        );
        await vi.advanceTimersByTimeAsync(1500);
        expect(result.processingAlerts().size).toBe(0);
      } finally {
        cleanup();
      }
    },
  );
  it.each(['snooze', 'unsnooze'] as const)(
    'rolls back an unconfirmed %s without changing a same-name neighbour',
    async (action) => {
      const [alerts, setAlerts] = createSignal([
        initial,
        { ...initial, id: 'cpu:vm/200', resourceId: 'vm/200' },
      ]);
      const updateAlert = vi.fn((id: string, changes: Partial<Alert>) =>
        setAlerts((rows) => rows.map((row) => (row.id === id ? { ...row, ...changes } : row))),
      );
      const { result, cleanup } = renderHook(() => useAlertSnoozeState({ alerts, updateAlert }));
      try {
        if (action === 'unsnooze') {
          vi.mocked(apiFetchJSON).mockResolvedValue({ success: true, snoozedUntil: until });
          await result.handleSnooze(alerts()[0], new Date(until));
        }
        const before = structuredClone(alerts());
        vi.mocked(notificationStore.success).mockClear();
        vi.mocked(apiFetchJSON).mockClear().mockResolvedValue({ success: false });
        const pending =
          action === 'snooze'
            ? result.handleSnooze(alerts()[0], new Date(until))
            : result.handleUnsnooze(alerts()[0]);
        await expect(pending).rejects.toThrow(/could not be confirmed/);
        expect(alerts()).toEqual(before);
        expect(isAlertSnoozed(result.effectiveAlerts()[0])).toBe(action === 'unsnooze');
        expect(result.processing().size).toBe(0);
        expect(notificationStore.success).not.toHaveBeenCalled();
        expect(notificationStore.error).toHaveBeenCalledWith(
          expect.stringMatching(/could not be confirmed.*Refresh alerts/),
        );
        expect(apiFetchJSON).toHaveBeenCalledTimes(1);
      } finally {
        cleanup();
      }
    },
  );
});
