import { createRoot } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI } from '@/api/notifications';

import { useNotificationDeliveryHealth } from '../useNotificationDeliveryHealth';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: { getHealth: vi.fn() },
}));

const healthWith = (status: string) => ({ queue: { status, failed: 3, deadLetter: 1 } }) as never;

describe('useNotificationDeliveryHealth', () => {
  beforeEach(() => {
    vi.mocked(NotificationsAPI.getHealth).mockReset();
  });

  it('stays silent before the first load resolves so it cannot flash a warning', () =>
    createRoot(async (dispose) => {
      vi.mocked(NotificationsAPI.getHealth).mockResolvedValue(healthWith('degraded'));
      const state = useNotificationDeliveryHealth();

      expect(state.deliveryNeedsAttention()).toBe(false);

      await state.loadDeliveryHealth();
      expect(state.deliveryNeedsAttention()).toBe(true);
      dispose();
    }));

  it('does not raise attention for a healthy queue', () =>
    createRoot(async (dispose) => {
      vi.mocked(NotificationsAPI.getHealth).mockResolvedValue(healthWith('healthy'));
      const state = useNotificationDeliveryHealth();

      await state.loadDeliveryHealth();
      expect(state.deliveryNeedsAttention()).toBe(false);
      expect(state.deliveryHealthUnavailable()).toBe(false);
      dispose();
    }));

  it('treats an unreachable health endpoint as needing attention', () =>
    createRoot(async (dispose) => {
      vi.mocked(NotificationsAPI.getHealth).mockRejectedValue(new Error('network down'));
      const state = useNotificationDeliveryHealth();

      await state.loadDeliveryHealth();
      expect(state.deliveryHealthUnavailable()).toBe(true);
      expect(state.deliveryNeedsAttention()).toBe(true);
      expect(state.deliveryHealth()).toBeNull();
      dispose();
    }));

  it('treats a queue the server reports as unavailable as needing attention', () =>
    createRoot(async (dispose) => {
      vi.mocked(NotificationsAPI.getHealth).mockResolvedValue(healthWith('unavailable'));
      const state = useNotificationDeliveryHealth();

      await state.loadDeliveryHealth();
      expect(state.deliveryNeedsAttention()).toBe(true);
      dispose();
    }));
});
