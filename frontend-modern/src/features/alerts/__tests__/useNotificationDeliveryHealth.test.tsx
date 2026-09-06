import { createRoot } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI } from '@/api/notifications';

import { useNotificationDeliveryHealth } from '../useNotificationDeliveryHealth';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getHealth: vi.fn(),
    dismissTerminalFailures: vi.fn(),
    retryTerminalFailures: vi.fn(),
  },
}));

const healthWith = (status: string) => ({ queue: { status, failed: 3, deadLetter: 1 } }) as never;

describe('useNotificationDeliveryHealth', () => {
  beforeEach(() => {
    vi.mocked(NotificationsAPI.getHealth).mockReset();
    vi.mocked(NotificationsAPI.dismissTerminalFailures).mockReset();
    vi.mocked(NotificationsAPI.retryTerminalFailures).mockReset();
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
  it.each([
    ['healthy', 'degraded'],
    ['degraded', 'healthy'],
    ['error', 'healthy'],
    ['healthy', 'error'],
  ])('ignores older %s completion after newer %s result', (older, newer) =>
    createRoot(async (dispose) => {
      let resolve!: (value: Awaited<ReturnType<typeof NotificationsAPI.getHealth>>) => void;
      let reject!: (error: Error) => void;
      vi.mocked(NotificationsAPI.getHealth).mockReturnValueOnce(
        new Promise((yes, no) => {
          resolve = yes;
          reject = no;
        }),
      );
      const state = useNotificationDeliveryHealth();
      const pending = state.loadDeliveryHealth();
      if (newer === 'error') {
        vi.mocked(NotificationsAPI.getHealth).mockRejectedValueOnce(new Error('new failure'));
      } else {
        vi.mocked(NotificationsAPI.getHealth).mockResolvedValueOnce(healthWith(newer));
      }
      await state.loadDeliveryHealth();
      if (older === 'error') reject(new Error('old failure'));
      else resolve(healthWith(older));
      await pending;
      expect(state.deliveryHealth()?.queue.status).toBe(newer === 'error' ? undefined : newer);
      expect(state.deliveryHealthUnavailable()).toBe(newer === 'error');
      expect(state.deliveryNeedsAttention()).toBe(newer !== 'healthy');
      expect(state.refreshingDeliveryHealth()).toBe(false);
      dispose();
    }),
  );

  it.each(['healthy', 'error'])(
    'keeps loading and first-load silence when older %s finishes first',
    (older) =>
      createRoot(async (dispose) => {
        let finishOld!: () => void;
        let finishNew!: () => void;
        vi.mocked(NotificationsAPI.getHealth)
          .mockReturnValueOnce(
            new Promise((resolve, reject) => {
              finishOld = () =>
                older === 'error'
                  ? reject(new Error('old failure'))
                  : resolve(healthWith('healthy'));
            }),
          )
          .mockReturnValueOnce(
            new Promise((resolve) => {
              finishNew = () => resolve(healthWith('degraded'));
            }),
          );
        const state = useNotificationDeliveryHealth();
        const oldRequest = state.loadDeliveryHealth();
        const newRequest = state.loadDeliveryHealth();
        finishOld();
        await oldRequest;
        expect(state.refreshingDeliveryHealth()).toBe(true);
        expect(state.deliveryHealth()).toBeNull();
        expect(state.deliveryNeedsAttention()).toBe(false);
        finishNew();
        await newRequest;
        expect(state.refreshingDeliveryHealth()).toBe(false);
        expect(state.deliveryNeedsAttention()).toBe(true);
        dispose();
      }),
  );

  it.each(['dismissTerminalFailures', 'retryTerminalFailures'] as const)(
    'keeps post-%s health when a pre-action request finishes late',
    (action) =>
      createRoot(async (dispose) => {
        const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
        try {
          const state = useNotificationDeliveryHealth();
          vi.mocked(NotificationsAPI.getHealth).mockResolvedValueOnce({
            queue: { status: 'degraded', attentionRequired: 2 },
          } as never);
          await state.loadDeliveryHealth();
          let finishOld!: (health: Awaited<ReturnType<typeof NotificationsAPI.getHealth>>) => void;
          vi.mocked(NotificationsAPI.getHealth)
            .mockReturnValueOnce(
              new Promise((resolve) => {
                finishOld = resolve;
              }),
            )
            .mockResolvedValueOnce(healthWith('healthy'));
          const pending = state.loadDeliveryHealth();
          vi.mocked(NotificationsAPI[action]).mockResolvedValueOnce({ affected: 2 } as never);
          await state[action]();
          expect(state.deliveryNeedsAttention()).toBe(false);
          finishOld(healthWith('degraded'));
          await pending;
          expect(state.deliveryHealth()?.queue.status).toBe('healthy');
          expect(state.deliveryNeedsAttention()).toBe(false);
        } finally {
          confirmSpy.mockRestore();
          dispose();
        }
      }),
  );

  describe.each(['dismissTerminalFailures', 'retryTerminalFailures'] as const)('%s', (action) => {
    it.each(['cancelled', 'rejected'] as const)(
      'preserves retained failure evidence when the action is %s',
      (outcome) => createRoot(async (dispose) => {
        const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(outcome !== 'cancelled');
        const onAfterQueueAction = vi.fn();
        try {
          const health = { queue: { status: 'degraded', attentionRequired: 2 } } as never;
          vi.mocked(NotificationsAPI.getHealth).mockResolvedValueOnce(health);
          const state = useNotificationDeliveryHealth({ onAfterQueueAction });
          await state.loadDeliveryHealth();
          let rejectAction!: (reason: Error) => void;
          vi.mocked(NotificationsAPI[action]).mockReturnValueOnce(new Promise((_, reject) => {
            rejectAction = reject;
          }));
          const pending = state[action]();
          const busy = action === 'dismissTerminalFailures'
            ? state.dismissingTerminalFailures : state.retryingTerminalFailures;
          expect(busy()).toBe(outcome === 'rejected');
          if (outcome === 'rejected') rejectAction(new Error('request rejected'));
          await pending;

          expect(confirmSpy).toHaveBeenCalledOnce();
          expect(NotificationsAPI[action]).toHaveBeenCalledTimes(outcome === 'rejected' ? 1 : 0);
          expect(NotificationsAPI.getHealth).toHaveBeenCalledTimes(1);
          expect(onAfterQueueAction).not.toHaveBeenCalled();
          expect(state.deliveryHealth()).toBe(health);
          expect(state.deliveryNeedsAttention()).toBe(true);
          expect(state.deliveryHealthUnavailable()).toBe(false);
          expect(busy()).toBe(false);
        } finally {
          confirmSpy.mockRestore();
          dispose();
        }
      }),
    );
  });

});
