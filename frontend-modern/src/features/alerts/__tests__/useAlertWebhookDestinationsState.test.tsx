import { renderHook, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI } from '@/api/notifications';
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
});
