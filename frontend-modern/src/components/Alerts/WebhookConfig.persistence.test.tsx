import { createSignal } from 'solid-js';
import { cleanup, fireEvent, render, renderHook, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI, type Webhook } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { useAlertWebhookDestinationsState } from '@/features/alerts/useAlertWebhookDestinationsState';
import { WebhookConfig } from './WebhookConfig';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getWebhookTemplates: vi.fn(),
    createWebhook: vi.fn(),
    updateWebhook: vi.fn(),
  },
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

const savedWebhook: Webhook = {
  id: 'saved-hook',
  name: 'Existing destination',
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

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

function mount(initial: Webhook[] = []) {
  const { result: owner } = renderHook(() => {
    const [webhooks, setWebhooks] = createSignal(initial);
    return useAlertWebhookDestinationsState({ webhooks, setWebhooks, autoLoad: false });
  });
  render(() => (
    <WebhookConfig
      webhooks={owner.webhooks()}
      onAdd={owner.addWebhook}
      onUpdate={owner.updateWebhook}
      onDelete={owner.deleteWebhook}
      onTest={owner.testWebhook}
    />
  ));
  return owner;
}

function openNewDraft() {
  fireEvent.click(screen.getByRole('button', { name: '+ Add Webhook' }));
  fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'New destination' } });
  fireEvent.input(screen.getByLabelText('Webhook URL'), {
    target: { value: 'https://example.test/new' },
  });
}

describe('webhook persistence acknowledgement', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(NotificationsAPI.getWebhookTemplates).mockResolvedValue([]);
  });
  afterEach(cleanup);

  it('keeps the draft and locks conflicting controls until a single create is acknowledged', async () => {
    const pending = deferred<Webhook>();
    vi.mocked(NotificationsAPI.createWebhook).mockReturnValue(pending.promise);
    const owner = mount([savedWebhook]);
    openNewDraft();
    const save = screen.getByRole('button', { name: 'Add Webhook', exact: true });
    fireEvent.click(save);
    fireEvent.click(save);

    expect(NotificationsAPI.createWebhook).toHaveBeenCalledTimes(1);
    expect(screen.getByLabelText('Name')).toHaveValue('New destination');
    expect(screen.getByLabelText('Name')).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Saving…' })).toBeDisabled();
    for (const name of ['Cancel', 'Edit', 'Delete', 'Disable All', 'Enabled']) {
      expect(screen.getByRole('button', { name, exact: true })).toBeDisabled();
    }
    expect(owner.webhooks()).toEqual([savedWebhook]);

    pending.resolve({ ...savedWebhook, id: 'new-hook', name: 'New destination' });
    await waitFor(() => expect(screen.queryByLabelText('Name')).not.toBeInTheDocument());
    expect(owner.webhooks().map((hook) => hook.id)).toEqual(['saved-hook', 'new-hook']);
    expect(notificationStore.success).toHaveBeenCalledTimes(1);
  });

  it('retains a failed create draft and requires an explicit retry before clearing it', async () => {
    vi.mocked(NotificationsAPI.createWebhook).mockRejectedValueOnce(new Error('Save unavailable'));
    const owner = mount();
    openNewDraft();
    fireEvent.input(screen.getByLabelText('Custom header 1 value'), {
      target: { value: 'application/custom+json' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add Webhook', exact: true }));

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Could not confirm'));
    expect(screen.getByLabelText('Name')).toHaveValue('New destination');
    expect(screen.getByLabelText('Webhook URL')).toHaveValue('https://example.test/new');
    expect(screen.getByLabelText('Custom header 1 value')).toHaveValue('application/custom+json');
    expect(screen.getByLabelText('Name')).toBeEnabled();
    expect(owner.webhooks()).toEqual([]);
    expect(NotificationsAPI.createWebhook).toHaveBeenCalledTimes(1);
    expect(notificationStore.success).not.toHaveBeenCalled();

    vi.mocked(NotificationsAPI.createWebhook).mockResolvedValueOnce({
      ...savedWebhook,
      id: 'new-hook',
      name: 'Accepted destination',
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add Webhook', exact: true }));
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument());
    expect(screen.queryByLabelText('Name')).not.toBeInTheDocument();
    expect(NotificationsAPI.createWebhook).toHaveBeenCalledTimes(2);
    expect(vi.mocked(NotificationsAPI.createWebhook).mock.calls[1][0]).toEqual(
      vi.mocked(NotificationsAPI.createWebhook).mock.calls[0][0],
    );
    expect(owner.webhooks()[0].name).toBe('Accepted destination');
  });

  it('retains masked credentials, routing and payload edits on update failure, then accepts the response', async () => {
    const pending = deferred<Webhook>();
    vi.mocked(NotificationsAPI.updateWebhook).mockReturnValueOnce(pending.promise);
    const owner = mount([savedWebhook]);
    fireEvent.click(screen.getByRole('button', { name: 'Edit', exact: true }));
    fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Edited destination' } });
    const draft = screen.getByLabelText('Name');
    fireEvent.click(screen.getByRole('button', { name: 'Update Webhook' }));
    pending.reject(new Error('Save unavailable'));

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(screen.getByLabelText('Name')).toBe(draft);
    expect(draft).toHaveValue('Edited destination');
    expect(screen.getByLabelText('Custom header 1 value')).toHaveValue('********');
    expect(owner.webhooks()).toEqual([savedWebhook]);
    const [id, payload] = vi.mocked(NotificationsAPI.updateWebhook).mock.calls[0];
    expect(id).toBe(savedWebhook.id);
    expect(payload).toMatchObject({
      ...savedWebhook,
      name: 'Edited destination',
    });

    vi.mocked(NotificationsAPI.updateWebhook).mockResolvedValueOnce({
      ...savedWebhook,
      name: 'Accepted edit',
      enabled: false,
    });
    fireEvent.click(screen.getByRole('button', { name: 'Update Webhook' }));
    await waitFor(() => expect(screen.queryByLabelText('Name')).not.toBeInTheDocument());
    expect(owner.webhooks()[0]).toEqual({ ...savedWebhook, name: 'Accepted edit', enabled: false });
    expect(vi.mocked(NotificationsAPI.updateWebhook).mock.calls[1]).toEqual([id, payload]);
  });

  it('allows cancellation after a failure without an automatic save or stale error on reopening', async () => {
    vi.mocked(NotificationsAPI.createWebhook).mockRejectedValue(new Error('Save unavailable'));
    mount();
    openNewDraft();
    fireEvent.click(screen.getByRole('button', { name: 'Add Webhook', exact: true }));
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    fireEvent.click(screen.getByRole('button', { name: '+ Add Webhook' }));
    expect(screen.getByLabelText('Name')).toHaveValue('');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(NotificationsAPI.createWebhook).toHaveBeenCalledTimes(1);
  });

  it('handles a rejected callback with fixed guidance, without putting its text in the editor', async () => {
    const privateDetail = 'synthetic receiver-controlled detail';
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={async () => { throw new Error(privateDetail); }}
        onUpdate={async () => false}
        onDelete={() => {}}
        onTest={() => {}}
      />
    ));
    openNewDraft();
    fireEvent.click(screen.getByRole('button', { name: 'Add Webhook', exact: true }));
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Check the configured destinations'));
    expect(screen.getByRole('alert')).not.toHaveTextContent(privateDetail);
    expect(screen.getByLabelText('Name')).toHaveValue('New destination');
  });

  it('returns true only after an accepted create or update and false on failure', async () => {
    const { result: owner } = renderHook(() => useAlertWebhookDestinationsState({ autoLoad: false }));
    vi.mocked(NotificationsAPI.createWebhook).mockResolvedValue(savedWebhook);
    expect(await owner.addWebhook(savedWebhook)).toBe(true);
    vi.mocked(NotificationsAPI.updateWebhook).mockRejectedValue(new Error('Save unavailable'));
    expect(await owner.updateWebhook(savedWebhook)).toBe(false);
    expect(owner.webhooks()).toEqual([savedWebhook]);
    vi.mocked(NotificationsAPI.updateWebhook).mockResolvedValue({ ...savedWebhook, enabled: false });
    expect(await owner.updateWebhook(savedWebhook)).toBe(true);
  });
});
