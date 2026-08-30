import { createRoot } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI } from '@/api/notifications';

import { useNotificationDeliveryLog } from '../useNotificationDeliveryLog';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: { getDeliveryLog: vi.fn() },
}));

describe('useNotificationDeliveryLog', () => {
  beforeEach(() => {
    vi.mocked(NotificationsAPI.getDeliveryLog).mockReset();
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
});
