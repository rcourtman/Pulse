import { createSignal } from 'solid-js';
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
import { useAlertWebhookDestinationsState } from '@/features/alerts/useAlertWebhookDestinationsState';
import { WebhookConfig } from './WebhookConfig';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getWebhookTemplates: vi.fn(),
    createWebhook: vi.fn(),
    updateWebhook: vi.fn(),
    deleteWebhook: vi.fn(),
    testNotification: vi.fn(),
  },
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

const webhook: Webhook = {
  id: 'hook-1',
  name: 'First destination',
  url: 'https://example.test/receiver',
  method: 'POST',
  enabled: true,
  service: 'generic',
  headers: { Authorization: '********' },
  customFields: { team: 'ops' },
  template: '{"message":"{{message}}"}',
  mention: 'operators',
  tagFilter: ['department-a'],
  tagFilterMode: 'any',
  minimumSeverity: 'warning',
};
const secondWebhook = { ...webhook, id: 'hook-2', name: 'Second destination' };
const thirdWebhook = { ...webhook, id: 'hook-3', name: 'Third destination' };

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

function mount(initial: Webhook[] = [webhook]) {
  const { result: owner } = renderHook(() => {
    const [webhooks, setWebhooks] = createSignal(initial);
    return useAlertWebhookDestinationsState({ webhooks, setWebhooks, autoLoad: false });
  });
  const rendered = render(() => (
    <WebhookConfig
      webhooks={owner.webhooks()}
      onAdd={owner.addWebhook}
      onUpdate={owner.updateWebhook}
      onDelete={owner.deleteWebhook}
      onTest={owner.testWebhook}
    />
  ));
  return { owner, unmount: rendered.unmount };
}

function expectListLocked() {
  const list = screen.getByRole('button', { name: 'Disable All' }).closest('.space-y-3')!;
  for (const name of [
    'Disable All',
    'Enable All',
    'Edit',
    'Delete',
    'Enabled',
    'Disabled',
    'Test',
  ]) {
    for (const button of within(list as HTMLElement).queryAllByRole('button', {
      name,
    })) {
      expect(button).toBeDisabled();
    }
  }
}

describe('acknowledged webhook list mutations', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(NotificationsAPI.getWebhookTemplates).mockResolvedValue([]);
  });
  afterEach(cleanup);

  it('serialises a bulk change, skips unchanged rows and locks all conflicting controls', async () => {
    const first = deferred<Webhook>();
    const second = deferred<Webhook>();
    vi.mocked(NotificationsAPI.updateWebhook)
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);
    const unchanged = { ...thirdWebhook, enabled: false };
    const { owner } = mount([webhook, secondWebhook, unchanged]);
    const disableAll = screen.getByRole('button', { name: 'Disable All' });
    fireEvent.click(disableAll);
    fireEvent.click(disableAll);

    expect(NotificationsAPI.updateWebhook).toHaveBeenCalledTimes(1);
    expect(NotificationsAPI.updateWebhook).toHaveBeenNthCalledWith(1, 'hook-1', {
      ...webhook,
      enabled: false,
    });
    expectListLocked();
    expect(screen.getByRole('button', { name: '+ Add Webhook' })).toBeDisabled();
    expect(screen.getByRole('status')).toHaveTextContent('Saving webhook changes');
    expect(owner.webhooks()).toEqual([webhook, secondWebhook, unchanged]);

    first.resolve({ ...webhook, enabled: false });
    await waitFor(() => expect(NotificationsAPI.updateWebhook).toHaveBeenCalledTimes(2));
    expect(NotificationsAPI.updateWebhook).toHaveBeenNthCalledWith(2, 'hook-2', {
      ...secondWebhook,
      enabled: false,
    });
    expect(owner.webhooks()).toEqual([{ ...webhook, enabled: false }, secondWebhook, unchanged]);
    expectListLocked();

    second.resolve({ ...secondWebhook, enabled: false });
    await waitFor(() => expect(screen.queryByRole('status')).not.toBeInTheDocument());
    expect(owner.webhooks().every((entry) => !entry.enabled)).toBe(true);
    expect(NotificationsAPI.updateWebhook).toHaveBeenCalledTimes(2);
    expect(screen.getByRole('button', { name: 'Enable All' })).toBeEnabled();
  });

  it('keeps the saved row unchanged and admits no competing action during a pending toggle', async () => {
    const pending = deferred<Webhook>();
    vi.mocked(NotificationsAPI.updateWebhook).mockReturnValueOnce(pending.promise);
    const { owner } = mount();
    const toggle = screen.getByRole('button', { name: 'Enabled' });
    fireEvent.click(toggle);
    fireEvent.click(toggle);
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
    fireEvent.click(screen.getByRole('button', { name: '+ Add Webhook' }));
    expect(NotificationsAPI.updateWebhook).toHaveBeenCalledTimes(1);
    expect(NotificationsAPI.deleteWebhook).not.toHaveBeenCalled();
    expect(screen.queryByLabelText('Name')).not.toBeInTheDocument();
    expect(owner.webhooks()).toEqual([webhook]);
    expectListLocked();

    pending.resolve({ ...webhook, enabled: false });
    await waitFor(() => expect(screen.queryByRole('status')).not.toBeInTheDocument());
    expect(owner.webhooks()).toEqual([{ ...webhook, enabled: false }]);
    expect(screen.getByRole('button', { name: 'Disabled' })).toBeEnabled();
  });

  it('waits for deletion acknowledgement without removing the row or allowing another write', async () => {
    const pending = deferred<{ success: boolean }>();
    vi.mocked(NotificationsAPI.deleteWebhook).mockReturnValueOnce(pending.promise);
    const { owner } = mount();
    const remove = screen.getByRole('button', { name: 'Delete' });
    fireEvent.click(remove);
    fireEvent.click(remove);
    expect(NotificationsAPI.deleteWebhook).toHaveBeenCalledTimes(1);
    expect(owner.webhooks()).toEqual([webhook]);
    expectListLocked();
    expect(screen.getByRole('button', { name: '+ Add Webhook' })).toBeDisabled();

    pending.resolve({ success: true });
    await waitFor(() => expect(screen.queryByText(webhook.name)).not.toBeInTheDocument());
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '+ Add Webhook' })).toBeEnabled();
  });

  it('stops a partially acknowledged bulk change at the first failed write without rollback or retry', async () => {
    vi.mocked(NotificationsAPI.updateWebhook)
      .mockResolvedValueOnce({ ...webhook, enabled: false })
      .mockRejectedValueOnce(new Error('Synthetic receiver detail'));
    const { owner } = mount([webhook, secondWebhook, thirdWebhook]);
    fireEvent.click(screen.getByRole('button', { name: 'Disable All' }));
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Some destinations may already have changed',
    );
    expect(screen.getByRole('alert')).not.toHaveTextContent('Synthetic receiver detail');
    expect(owner.webhooks()).toEqual([{ ...webhook, enabled: false }, secondWebhook, thirdWebhook]);
    expect(NotificationsAPI.updateWebhook).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disable All' })).toBeEnabled();
  });

  it('preserves an open edit draft by refusing list changes until it is saved or cancelled', async () => {
    const { owner } = mount();
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
    fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Unsaved destination' } });
    expectListLocked();
    fireEvent.click(screen.getByRole('button', { name: 'Enabled' }));
    fireEvent.click(screen.getByRole('button', { name: 'Disable All' }));
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(NotificationsAPI.updateWebhook).not.toHaveBeenCalled();
    expect(NotificationsAPI.deleteWebhook).not.toHaveBeenCalled();
    expect(owner.webhooks()).toEqual([webhook]);
    expect(screen.getByLabelText('Name')).toHaveValue('Unsaved destination');
    expect(screen.getByLabelText('Custom header 1 value')).toHaveValue('********');
    expect(screen.getByRole('button', { name: 'Update Webhook' })).toBeEnabled();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.getByRole('button', { name: 'Enabled' })).toBeEnabled();
  });

  it('withdraws unsent bulk writes when the editor is unmounted', async () => {
    const pending = deferred<Webhook>();
    vi.mocked(NotificationsAPI.updateWebhook).mockReturnValueOnce(pending.promise);
    const { unmount } = mount([webhook, secondWebhook]);
    fireEvent.click(screen.getByRole('button', { name: 'Disable All' }));
    expect(NotificationsAPI.updateWebhook).toHaveBeenCalledTimes(1);
    unmount();
    pending.resolve({ ...webhook, enabled: false });
    // Drain the promise chain, including the batch continuation, without a timer retry.
    for (let index = 0; index < 10; index++) await Promise.resolve();
    expect(NotificationsAPI.updateWebhook).toHaveBeenCalledTimes(1);
  });

  it('retains a failed deletion and requires a deliberate retry before changing inventory', async () => {
    vi.mocked(NotificationsAPI.deleteWebhook).mockRejectedValueOnce(
      new Error('Delete unavailable'),
    );
    const { owner } = mount();
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(owner.webhooks()).toEqual([webhook]);
    expect(NotificationsAPI.deleteWebhook).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('alert')).toHaveTextContent('Check the configured destinations');

    const retry = deferred<{ success: boolean }>();
    vi.mocked(NotificationsAPI.deleteWebhook).mockReturnValueOnce(retry.promise);
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expectListLocked();
    retry.resolve({ success: true });
    await waitFor(() => expect(owner.webhooks()).toEqual([]));
    expect(NotificationsAPI.deleteWebhook).toHaveBeenCalledTimes(2);
  });

  it('does not treat an unaccepted deletion response as an acknowledged removal', async () => {
    vi.mocked(NotificationsAPI.deleteWebhook).mockResolvedValueOnce({ success: false });
    const { owner } = mount();
    expect(await owner.deleteWebhook(webhook.id)).toBe(false);
    expect(owner.webhooks()).toEqual([webhook]);
    expect(NotificationsAPI.deleteWebhook).toHaveBeenCalledTimes(1);
  });
});
