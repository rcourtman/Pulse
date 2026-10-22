import {
  cleanup,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
  within,
} from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI, type Webhook } from '@/api/notifications';
import { WebhookConfig } from '@/components/Alerts/WebhookConfig';
import { notificationStore } from '@/stores/notifications';
import { showErrorWithDetail } from '@/utils/toast';
import { useAlertWebhookDestinationsState } from '../useAlertWebhookDestinationsState';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getWebhookTemplates: vi.fn(),
    testNotification: vi.fn(),
    testWebhook: vi.fn(),
  },
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), warning: vi.fn() },
}));
vi.mock('@/utils/toast', () => ({ showErrorWithDetail: vi.fn() }));

const first: Webhook = {
  id: 'first',
  name: 'First destination',
  url: 'https://example.test/first',
  method: 'POST',
  headers: {},
  enabled: true,
};
const second = { ...first, id: 'second', name: 'Second destination' };
const draft = { ...first, name: 'Unsaved draft' };

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

function mount() {
  const { result: owner, cleanup: dispose } = renderHook(() =>
    useAlertWebhookDestinationsState({ autoLoad: false }),
  );
  return { owner, dispose };
}

describe('webhook test ownership', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(NotificationsAPI.getWebhookTemplates).mockResolvedValue([]);
  });
  afterEach(cleanup);

  it.each(['saved', 'draft'] as const)(
    'serialises %s tests with all saved and draft callers',
    async (kind) => {
      const pending = deferred<never>();
      vi.mocked(NotificationsAPI.testNotification).mockReturnValueOnce(pending.promise);
      vi.mocked(NotificationsAPI.testWebhook).mockReturnValueOnce(pending.promise);
      const { owner } = mount();
      const sending = owner.testWebhook(first.id, kind === 'draft' ? draft : undefined);
      await owner.testWebhook(second.id);
      await owner.testWebhook(first.id);
      await owner.testWebhook('temp-new-webhook', draft);
      expect(owner.testingWebhook()).toBe(first.id);
      expect(NotificationsAPI.testNotification).toHaveBeenCalledTimes(kind === 'saved' ? 1 : 0);
      expect(NotificationsAPI.testWebhook).toHaveBeenCalledTimes(kind === 'draft' ? 1 : 0);
      if (kind === 'saved')
        expect(NotificationsAPI.testNotification).toHaveBeenCalledWith({
          type: 'webhook',
          webhookId: first.id,
        });
      else expect(NotificationsAPI.testWebhook).toHaveBeenCalledWith(draft);
      expect(notificationStore.success).not.toHaveBeenCalled();
      pending.resolve({ status: 'success', success: true } as never);
      await sending;
      expect(owner.testingWebhook()).toBeNull();
      expect(notificationStore.success).toHaveBeenCalledTimes(1);
      // Suppressed attempts are discarded, not queued for a later send.
      expect(
        vi.mocked(NotificationsAPI.testNotification).mock.calls.length +
          vi.mocked(NotificationsAPI.testWebhook).mock.calls.length,
      ).toBe(1);
      vi.mocked(NotificationsAPI.testNotification).mockResolvedValue({ status: 'success' });
      await owner.testWebhook(second.id);
      expect(NotificationsAPI.testNotification).toHaveBeenLastCalledWith({
        type: 'webhook',
        webhookId: second.id,
      });
      expect(notificationStore.success).toHaveBeenCalledTimes(2);
    },
  );

  it('unlocks on rejection without retrying and preserves the original diagnostic', async () => {
    const pending = deferred<never>();
    vi.mocked(NotificationsAPI.testNotification).mockReturnValue(pending.promise);
    const { owner } = mount();
    const sending = owner.testWebhook(first.id);
    const error = Object.assign(new Error('Synthetic rejection'), { detail: 'Synthetic detail' });
    pending.reject(error);
    await sending;
    expect(owner.testingWebhook()).toBeNull();
    expect(showErrorWithDetail).toHaveBeenCalledExactlyOnceWith(error.message, error.detail);
    expect(NotificationsAPI.testNotification).toHaveBeenCalledTimes(1);
    expect(notificationStore.success).not.toHaveBeenCalled();
  });

  it.each(['saved', 'draft'] as const)(
    'preserves the paused warning for an acknowledged %s test',
    async (kind) => {
      vi.mocked(NotificationsAPI.testNotification).mockResolvedValue({
        status: 'success',
        deliveryPaused: true,
      });
      vi.mocked(NotificationsAPI.testWebhook).mockResolvedValue({
        success: true,
        deliveryPaused: true,
      });
      const { owner } = mount();
      await owner.testWebhook(first.id, kind === 'draft' ? draft : undefined);
      expect(notificationStore.warning).toHaveBeenCalledOnce();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(owner.testingWebhook()).toBeNull();
    },
  );

  it.each(['saved', 'draft'] as const)(
    'ignores late success, pause and failure from a retired %s owner',
    async (kind) => {
      for (const outcome of ['success', 'paused', 'failure'] as const) {
        const pending = deferred<never>();
        vi.mocked(NotificationsAPI.testNotification).mockReturnValue(pending.promise);
        vi.mocked(NotificationsAPI.testWebhook).mockReturnValue(pending.promise);
        const { owner, dispose } = mount();
        const sending = owner.testWebhook(first.id, kind === 'draft' ? draft : undefined);
        dispose();
        if (outcome === 'failure') pending.reject(new Error('Retired request'));
        else
          pending.resolve({
            status: 'success',
            success: true,
            deliveryPaused: outcome === 'paused',
          } as never);
        await sending;
        await owner.testWebhook(second.id);
        await owner.testWebhook('temp-new-webhook', draft);
        expect(notificationStore.success).not.toHaveBeenCalled();
        expect(notificationStore.warning).not.toHaveBeenCalled();
        expect(showErrorWithDetail).not.toHaveBeenCalled();
      }
      expect(NotificationsAPI.testNotification).toHaveBeenCalledTimes(kind === 'saved' ? 3 : 0);
      expect(NotificationsAPI.testWebhook).toHaveBeenCalledTimes(kind === 'draft' ? 3 : 0);
    },
  );

  it('disables every saved Test and a different open draft while the real owner is pending', async () => {
    const pending = deferred<never>();
    vi.mocked(NotificationsAPI.testNotification).mockReturnValue(pending.promise);
    const { owner } = mount();
    render(() => (
      <WebhookConfig
        webhooks={[first, second]}
        onAdd={() => true}
        onUpdate={() => true}
        onDelete={() => true}
        onTest={owner.testWebhook}
        testing={owner.testingWebhook()}
      />
    ));
    fireEvent.click(screen.getAllByRole('button', { name: 'Test' })[0]);
    expect(screen.getByRole('button', { name: 'Testing…' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Test' })).toBeDisabled();
    fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[1]);
    fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Keep this draft' } });
    const form = screen.getByLabelText('Name').closest('fieldset')!;
    expect(within(form).getByRole('button', { name: 'Test' })).toBeDisabled();
    fireEvent.click(within(form).getByRole('button', { name: 'Test' }));
    expect(NotificationsAPI.testWebhook).not.toHaveBeenCalled();
    pending.resolve({ status: 'success' } as never);
    await waitFor(() => expect(within(form).getByRole('button', { name: 'Test' })).toBeEnabled());
    expect(screen.getByLabelText('Name')).toHaveValue('Keep this draft');
    expect(NotificationsAPI.testNotification).toHaveBeenCalledTimes(1);
  });
});
