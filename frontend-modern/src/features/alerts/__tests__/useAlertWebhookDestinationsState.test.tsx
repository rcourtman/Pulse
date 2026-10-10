import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI, type Webhook } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { showErrorWithDetail } from '@/utils/toast';

import { useAlertWebhookDestinationsState } from '../useAlertWebhookDestinationsState';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    createWebhook: vi.fn(),
    deleteWebhook: vi.fn(),
    getWebhooks: vi.fn(),
    testNotification: vi.fn(),
    testWebhook: vi.fn(),
    updateWebhook: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

vi.mock('@/utils/toast', () => ({
  showErrorWithDetail: vi.fn(),
}));

describe('useAlertWebhookDestinationsState', () => {
  beforeEach(() => {
    vi.mocked(NotificationsAPI.createWebhook).mockReset();
    vi.mocked(NotificationsAPI.deleteWebhook).mockReset();
    vi.mocked(NotificationsAPI.getWebhooks).mockReset();
    vi.mocked(NotificationsAPI.testNotification).mockReset();
    vi.mocked(NotificationsAPI.testWebhook).mockReset();
    vi.mocked(NotificationsAPI.updateWebhook).mockReset();
    vi.mocked(notificationStore.error).mockReset();
    vi.mocked(notificationStore.success).mockReset();
    vi.mocked(showErrorWithDetail).mockReset();
  });

  it('owns webhook load, mutation, and test runtime for alert destinations', async () => {
    vi.mocked(NotificationsAPI.getWebhooks).mockResolvedValue([
      {
        enabled: true,
        headers: {},
        id: 'hook-1',
        method: 'POST',
        name: 'Ops',
        url: 'https://hooks.example.test/ops',
      },
    ] as never);
    vi.mocked(NotificationsAPI.testNotification).mockResolvedValue({ success: true } as never);
    vi.mocked(NotificationsAPI.createWebhook).mockResolvedValue({
      enabled: true,
      headers: {},
      id: 'hook-2',
      method: 'POST',
      name: 'Pager',
      service: 'slack',
      url: 'https://hooks.example.test/pager',
    } as never);
    vi.mocked(NotificationsAPI.updateWebhook).mockResolvedValue({
      enabled: false,
      headers: {},
      id: 'hook-2',
      method: 'POST',
      name: 'Pager Updated',
      service: 'slack',
      url: 'https://hooks.example.test/pager',
    } as never);
    vi.mocked(NotificationsAPI.deleteWebhook).mockResolvedValue({ success: true } as never);

    const { result } = renderHook(() => useAlertWebhookDestinationsState());

    await waitFor(() => expect(NotificationsAPI.getWebhooks).toHaveBeenCalledTimes(1));
    expect(result.webhooks()).toEqual([
      expect.objectContaining({ id: 'hook-1', service: 'generic' }),
    ]);

    await result.addWebhook({
      enabled: true,
      headers: {},
      method: 'POST',
      name: 'Pager',
      service: 'slack',
      url: 'https://hooks.example.test/pager',
    });
    expect(result.webhooks().map((hook) => hook.id)).toEqual(['hook-1', 'hook-2']);

    await result.updateWebhook({
      enabled: true,
      headers: {},
      id: 'hook-2',
      method: 'POST',
      name: 'Pager',
      service: 'slack',
      url: 'https://hooks.example.test/pager',
    });
    expect(result.webhooks().find((hook) => hook.id === 'hook-2')).toEqual(
      expect.objectContaining({ enabled: false, name: 'Pager Updated' }),
    );

    await result.testWebhook('hook-2');
    expect(NotificationsAPI.testNotification).toHaveBeenCalledWith({
      type: 'webhook',
      webhookId: 'hook-2',
    });

    await result.deleteWebhook('hook-1');
    expect(result.webhooks().map((hook) => hook.id)).toEqual(['hook-2']);

    await result.loadWebhooks();
    expect(NotificationsAPI.getWebhooks).toHaveBeenCalledTimes(2);
    expect(notificationStore.success).toHaveBeenCalled();
    expect(showErrorWithDetail).not.toHaveBeenCalled();
  });

  it.each(['create', 'update', 'delete'] as const)(
    'withdraws the retired %s owner before a late reply can change replacement inventory',
    async (operation) => {
      const saved: Webhook = {
        id: 'old-hook',
        name: 'Previous context',
        url: 'https://example.test/old',
        method: 'POST',
        headers: {},
        enabled: true,
      };
      const replacement = { ...saved, name: 'Replacement context' };
      let resolve!: (value: Webhook | { success: boolean }) => void;
      const pending = new Promise<Webhook | { success: boolean }>((yes) => {
        resolve = yes;
      });
      vi.mocked(NotificationsAPI.createWebhook).mockReturnValue(pending as Promise<Webhook>);
      vi.mocked(NotificationsAPI.updateWebhook).mockReturnValue(pending as Promise<Webhook>);
      vi.mocked(NotificationsAPI.deleteWebhook).mockReturnValue(
        pending as Promise<{ success: boolean }>,
      );
      // The shared inventory outlives its child destination-tab owner, as it
      // does when saved-policy loading replaces that child on an org change.
      const [webhooks, setWebhooks] = createSignal([saved]);
      const { result, cleanup: dispose } = renderHook(() =>
        useAlertWebhookDestinationsState({ webhooks, setWebhooks, autoLoad: false }),
      );
      const invoke = () =>
        operation === 'create'
          ? result.addWebhook(saved)
          : operation === 'update'
            ? result.updateWebhook(saved)
            : result.deleteWebhook(saved.id);
      const write = invoke();
      dispose();
      setWebhooks([replacement]);
      resolve(operation === 'delete' ? { success: true } : saved);
      expect(await write).toBe(false);
      expect(webhooks()).toEqual([replacement]);
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(await invoke()).toBe(false);
      expect(
        vi.mocked(NotificationsAPI.createWebhook).mock.calls.length +
          vi.mocked(NotificationsAPI.updateWebhook).mock.calls.length +
          vi.mocked(NotificationsAPI.deleteWebhook).mock.calls.length,
      ).toBe(1);
    },
  );
});
