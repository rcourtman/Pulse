import { createRoot } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import type { NotificationDeliveryLog } from '@/api/notifications';
import { NotificationsAPI } from '@/api/notifications';

import { useNotificationDeliveryLog } from '../useNotificationDeliveryLog';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: { getDeliveryLog: vi.fn() },
}));

vi.mock('@/api/alerts', () => ({
  AlertsAPI: { getEvents: vi.fn() },
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const emptyLog: NotificationDeliveryLog = {
  entries: [],
  windowDays: 30,
  completedRetentionDays: 7,
  deadLetterRetentionDays: 30,
};

describe('useNotificationDeliveryLog', () => {
  beforeEach(() => {
    vi.mocked(NotificationsAPI.getDeliveryLog).mockReset();
    vi.mocked(AlertsAPI.getEvents).mockReset().mockResolvedValue([]);
  });

  it('exposes the loaded log and clears the unavailable flag', () =>
    createRoot(async (dispose) => {
      vi.mocked(NotificationsAPI.getDeliveryLog).mockResolvedValue({
        entries: [
          {
            notificationId: 'email-1',
            type: 'email',
            outcome: 'sent',
            alertIds: ['disk-critical-1'],
            alertCount: 1,
            attempts: 1,
            success: true,
            timestamp: '2026-08-20T12:00:00Z',
          },
        ],
        windowDays: 30,
        completedRetentionDays: 7,
        deadLetterRetentionDays: 30,
      });
      const state = useNotificationDeliveryLog();

      await state.loadDeliveryLog();
      expect(NotificationsAPI.getDeliveryLog).toHaveBeenCalledWith(200);
      expect(state.deliveryLog()?.entries).toHaveLength(1);
      expect(state.deliveryLogUnavailable()).toBe(false);
      dispose();
    }));

  it('reports an unreadable log as unavailable, never as empty', () =>
    createRoot(async (dispose) => {
      vi.mocked(NotificationsAPI.getDeliveryLog).mockRejectedValue(new Error('network down'));
      const state = useNotificationDeliveryLog();

      await state.loadDeliveryLog();
      expect(state.deliveryLogUnavailable()).toBe(true);
      expect(state.deliveryLog()).toBeNull();
      dispose();
    }));

  it('recovers the unavailable flag once a later load succeeds', () =>
    createRoot(async (dispose) => {
      vi.mocked(NotificationsAPI.getDeliveryLog).mockRejectedValueOnce(new Error('network down'));
      vi.mocked(NotificationsAPI.getDeliveryLog).mockResolvedValue({
        entries: [],
        windowDays: 30,
        completedRetentionDays: 7,
        deadLetterRetentionDays: 30,
      });
      const state = useNotificationDeliveryLog();

      await state.loadDeliveryLog();
      expect(state.deliveryLogUnavailable()).toBe(true);
      await state.loadDeliveryLog();
      expect(state.deliveryLogUnavailable()).toBe(false);
      expect(state.deliveryLog()?.entries).toHaveLength(0);
      dispose();
    }));

  it('does not wait for held events before completing the delivery read', () =>
    createRoot(async (dispose) => {
      const held = deferred<Awaited<ReturnType<typeof AlertsAPI.getEvents>>>();
      vi.mocked(AlertsAPI.getEvents).mockReturnValue(held.promise);
      vi.mocked(NotificationsAPI.getDeliveryLog).mockResolvedValue(emptyLog);
      const state = useNotificationDeliveryLog();
      try {
        await state.loadDeliveryLog();
        expect(state.deliveryLog()).toEqual(emptyLog);
        expect(state.refreshingDeliveryLog()).toBe(false);
        expect(state.heldEvents()).toEqual([]);
      } finally {
        held.resolve([]);
        dispose();
      }
    }));

  it('keeps the newest successful read when an older read fails', () =>
    createRoot(async (dispose) => {
      const older = deferred<NotificationDeliveryLog>();
      vi.mocked(NotificationsAPI.getDeliveryLog)
        .mockReturnValueOnce(older.promise)
        .mockResolvedValueOnce(emptyLog);
      const state = useNotificationDeliveryLog();
      try {
        const first = state.loadDeliveryLog();
        await state.loadDeliveryLog();
        older.reject(new Error('old read failed'));
        await first;
        expect(state.deliveryLogUnavailable()).toBe(false);
        expect(state.deliveryLog()).toEqual(emptyLog);
      } finally {
        dispose();
      }
    }));

  it('does not replace a newer unavailable result with an older success', () =>
    createRoot(async (dispose) => {
      const older = deferred<NotificationDeliveryLog>();
      vi.mocked(NotificationsAPI.getDeliveryLog)
        .mockReturnValueOnce(older.promise)
        .mockRejectedValueOnce(new Error('new read failed'));
      const state = useNotificationDeliveryLog();
      try {
        const first = state.loadDeliveryLog();
        await state.loadDeliveryLog();
        older.resolve(emptyLog);
        await first;
        expect(state.deliveryLogUnavailable()).toBe(true);
        expect(state.deliveryLog()).toBeNull();
      } finally {
        dispose();
      }
    }));

  it('keeps refreshing true while the newest read remains pending', () =>
    createRoot(async (dispose) => {
      const older = deferred<NotificationDeliveryLog>();
      const newer = deferred<NotificationDeliveryLog>();
      vi.mocked(NotificationsAPI.getDeliveryLog)
        .mockReturnValueOnce(older.promise)
        .mockReturnValueOnce(newer.promise);
      const state = useNotificationDeliveryLog();
      const first = state.loadDeliveryLog();
      const second = state.loadDeliveryLog();
      try {
        older.resolve(emptyLog);
        await first;
        expect(state.refreshingDeliveryLog()).toBe(true);
      } finally {
        newer.resolve(emptyLog);
        await second;
        dispose();
      }
    }));

  it.each(['success', 'failure'])('ignores an older held-event %s after a newer read', (outcome) =>
    createRoot(async (dispose) => {
      const older = deferred<Awaited<ReturnType<typeof AlertsAPI.getEvents>>>();
      const current = [
        {
          id: 2,
          type: 'notification_deferred',
          alertId: 'current',
          occurredAt: '2026-09-06T08:00:00Z',
        },
      ];
      vi.mocked(AlertsAPI.getEvents)
        .mockReturnValueOnce(older.promise)
        .mockResolvedValueOnce(current);
      vi.mocked(NotificationsAPI.getDeliveryLog).mockResolvedValue(emptyLog);
      const state = useNotificationDeliveryLog();
      try {
        await state.loadDeliveryLog();
        await state.loadDeliveryLog();
        if (outcome === 'success') older.resolve([]);
        else older.reject(new Error('stale held read'));
        await Promise.resolve();
        expect(state.heldEvents()).toEqual(current);
      } finally {
        dispose();
      }
    }),
  );

  it.each(['success', 'failure'])(
    'ignores pending %s after disposal and starts no new reads',
    (outcome) =>
      createRoot(async (dispose) => {
        const log = deferred<NotificationDeliveryLog>();
        const held = deferred<Awaited<ReturnType<typeof AlertsAPI.getEvents>>>();
        vi.mocked(NotificationsAPI.getDeliveryLog).mockReturnValue(log.promise);
        vi.mocked(AlertsAPI.getEvents).mockReturnValue(held.promise);
        const state = useNotificationDeliveryLog();
        const pending = state.loadDeliveryLog();
        dispose();
        if (outcome === 'success') {
          log.resolve(emptyLog);
          held.resolve([
            {
              id: 1,
              type: 'notification_suppressed',
              alertId: 'late',
              occurredAt: '2026-09-06T08:00:00Z',
            },
          ]);
        } else {
          log.reject(new Error('late log'));
          held.reject(new Error('late held'));
        }
        await pending;
        await state.loadDeliveryLog();
        expect(state.deliveryLog()).toBeNull();
        expect(state.deliveryLogUnavailable()).toBe(false);
        expect(state.heldEvents()).toEqual([]);
        // Disposal freezes state, including loading; no abandoned completion owns it.
        expect(state.refreshingDeliveryLog()).toBe(true);
        expect(NotificationsAPI.getDeliveryLog).toHaveBeenCalledTimes(1);
        expect(AlertsAPI.getEvents).toHaveBeenCalledTimes(1);
      }),
  );
});
