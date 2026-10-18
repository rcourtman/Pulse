import { createRoot } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI, type NotificationHealth } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';

import { useNotificationDeliveryHealth } from '../useNotificationDeliveryHealth';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getHealth: vi.fn(),
    dismissTerminalFailures: vi.fn(),
    retryTerminalFailures: vi.fn(),
  },
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

const health = { queue: { status: 'degraded', attentionRequired: 2 } } as NotificationHealth;
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const mount = (onAfterQueueAction = vi.fn()) => {
  let dispose!: () => void;
  const state = createRoot((stop) => {
    dispose = stop;
    return useNotificationDeliveryHealth({ onAfterQueueAction });
  });
  return { state, dispose, onAfterQueueAction };
};

describe('notification recovery view lifetime', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(NotificationsAPI.getHealth).mockResolvedValue(health);
  });

  it.each(['accepted', 'rejected'] as const)(
    'withdraws a retired health read even when it is %s',
    async (outcome) => {
      const read = deferred<NotificationHealth>();
      vi.mocked(NotificationsAPI.getHealth).mockReturnValueOnce(read.promise);
      const { state, dispose } = mount();
      const pending = state.loadDeliveryHealth();
      dispose();
      if (outcome === 'accepted') read.resolve(health);
      else read.reject(new Error('retired read failed'));
      await pending;
      expect(state.deliveryHealth()).toBeNull();
      expect(state.deliveryHealthUnavailable()).toBe(false);
      expect(state.deliveryNeedsAttention()).toBe(false);
      expect(state.refreshingDeliveryHealth()).toBe(true);
      expect(logger.error).not.toHaveBeenCalled();
      await state.loadDeliveryHealth();
      expect(NotificationsAPI.getHealth).toHaveBeenCalledOnce();
    },
  );

  describe.each(['retryTerminalFailures', 'dismissTerminalFailures'] as const)('%s', (action) => {
    it.each(['accepted', 'rejected'] as const)(
      'does not publish a late %s outcome or start follow-up reads after unmount',
      async (outcome) => {
        const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true);
        const write = deferred<Awaited<ReturnType<(typeof NotificationsAPI)[typeof action]>>>();
        vi.mocked(NotificationsAPI[action]).mockReturnValueOnce(write.promise);
        const { state, dispose, onAfterQueueAction } = mount();
        try {
          await state.loadDeliveryHealth();
          const pending = state[action]();
          dispose();
          if (outcome === 'accepted') write.resolve({ success: true, affected: 2 });
          else write.reject(new Error('retired action failed'));
          await pending;
          expect(NotificationsAPI[action]).toHaveBeenCalledOnce();
          expect(confirm).toHaveBeenCalledOnce();
          expect(NotificationsAPI.getHealth).toHaveBeenCalledOnce();
          expect(onAfterQueueAction).not.toHaveBeenCalled();
          expect(notificationStore.success).not.toHaveBeenCalled();
          expect(notificationStore.error).not.toHaveBeenCalled();
          expect(logger.error).not.toHaveBeenCalled();
          expect(state.queueActionFeedback()).toBeNull();
          expect(state.deliveryHealth()).toBe(health);
          await state[action]();
          expect(NotificationsAPI[action]).toHaveBeenCalledOnce();
          expect(confirm).toHaveBeenCalledOnce();
        } finally {
          confirm.mockRestore();
          dispose();
        }
      },
    );

    it('withdraws optional activity work if the view retires before its scheduled callback', async () => {
      const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true);
      const { state, dispose, onAfterQueueAction } = mount();
      try {
        await state.loadDeliveryHealth();
        vi.mocked(NotificationsAPI[action]).mockResolvedValueOnce({ success: true, affected: 2 });
        // Simulate navigation after the accepted mutation, while starting its
        // health refresh but before the optional activity microtask executes.
        vi.mocked(NotificationsAPI.getHealth).mockImplementationOnce(async () => {
          dispose();
          return { ...health, queue: { ...health.queue, status: 'healthy' } };
        });
        await state[action]();
        expect(notificationStore.success).toHaveBeenCalledOnce();
        expect(onAfterQueueAction).not.toHaveBeenCalled();
        expect(state.deliveryHealth()).toBe(health);
        expect(notificationStore.error).not.toHaveBeenCalled();
      } finally {
        confirm.mockRestore();
        dispose();
      }
    });

    it.each(['accepted', 'rejected'] as const)(
      'keeps a retired post-action refresh inert when its already-started work is %s',
      async (outcome) => {
        const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true);
        const read = deferred<NotificationHealth>();
        const activity = deferred<void>();
        const onAfterQueueAction = vi.fn().mockReturnValue(activity.promise);
        const { state, dispose } = mount(onAfterQueueAction);
        try {
          await state.loadDeliveryHealth();
          vi.mocked(NotificationsAPI.getHealth).mockReturnValueOnce(read.promise);
          vi.mocked(NotificationsAPI[action]).mockResolvedValueOnce({ success: true, affected: 2 });
          const pending = state[action]();
          await Promise.resolve();
          await Promise.resolve();
          expect(onAfterQueueAction).toHaveBeenCalledOnce();
          expect(notificationStore.success).toHaveBeenCalledOnce();
          dispose();
          if (outcome === 'accepted') {
            activity.resolve();
            read.resolve({ ...health, queue: { ...health.queue, status: 'healthy' } });
          } else {
            activity.reject(new Error('retired activity failed'));
            read.reject(new Error('retired health failed'));
          }
          await pending;
          expect(state.queueActionFeedback()).toBeNull();
          expect(state.deliveryHealth()).toBe(health);
          expect(state.deliveryNeedsAttention()).toBe(true);
          expect(state.refreshingDeliveryHealth()).toBe(true);
          expect(notificationStore.error).not.toHaveBeenCalled();
          expect(logger.error).not.toHaveBeenCalled();
        } finally {
          confirm.mockRestore();
          dispose();
        }
      },
    );
  });

  it('does not change retained feedback through a retired clear callback', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const { state, dispose } = mount();
    try {
      await state.loadDeliveryHealth();
      vi.mocked(NotificationsAPI.retryTerminalFailures).mockRejectedValueOnce(new Error('failed'));
      await state.retryTerminalFailures();
      const feedback = state.queueActionFeedback();
      expect(feedback).toMatch(/^Unable to/);
      dispose();
      state.clearQueueActionFeedback();
      expect(state.queueActionFeedback()).toBe(feedback);
    } finally {
      confirm.mockRestore();
      dispose();
    }
  });
});
