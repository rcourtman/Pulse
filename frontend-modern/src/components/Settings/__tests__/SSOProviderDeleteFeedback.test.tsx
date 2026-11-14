import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { SSOProvidersPanel } from '../SSOProvidersPanel';
import { setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';

const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
const logs = vi.hoisted(() => ({ error: vi.fn(), warn: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/utils/logger', () => ({ logger: logs }));
const fetchMock = vi.fn();
const message = 'Unable to remove the SSO provider.';
const privateDetail = 'SYNTHETIC_PRIVATE_DELETE_RESPONSE';
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const provider = (type: 'oidc' | 'saml', id = 'saved/provider') => ({
  id,
  name: id,
  type,
  enabled: true,
  priority: 0,
  oidcIssuerUrl: 'https://identity.example.test/production',
  samlIdpEntityId: 'https://identity.example.test/production/saml',
});
const writes = () => fetchMock.mock.calls.filter(([, options]) => options?.method === 'DELETE');
const reads = () => fetchMock.mock.calls.filter(([, options]) => !options?.method);
const openConfirmation = async (name = 'saved/provider') => {
  const opener = await screen.findByRole('button', { name: `Delete provider ${name}` });
  await waitFor(() => expect(opener).toBeEnabled());
  fireEvent.click(opener);
  return within(screen.getByRole('dialog', { name: 'Delete provider' }));
};
const remove = () =>
  fireEvent.click(
    within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete Provider' }),
  );

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal('fetch', fetchMock);
  document.cookie = 'pulse_csrf=synthetic-csrf';
  setOrgID('delete-feedback-a');
});
afterEach(() => {
  cleanup();
  setOrgID(null);
  vi.unstubAllGlobals();
});

describe.each(['oidc', 'saml'] as const)('%s deletion feedback', (type) => {
  // 401 sign-out remains the API client's responsibility, not modal recovery.
  it.each([400, 403, 404, 409, 422, 429])(
    'keeps a %s refusal in the active dialog without exposing the response or replaying',
    async (status) => {
      fetchMock.mockImplementation(async (_url: string, options?: RequestInit) =>
        options?.method === 'DELETE'
          ? json({ error: privateDetail, details: privateDetail }, status)
          : json({ providers: [provider(type)] }),
      );
      const changed = vi.fn();
      render(() => <SSOProvidersPanel currentUser="admin" onConfigUpdated={changed} />);
      const dialog = await openConfirmation();
      remove();
      const failure = await dialog.findByRole('alert');
      expect(failure).toHaveTextContent(message);
      expect(failure.closest('[inert]')).toBeNull();
      expect(dialog.getByText('saved/provider', { exact: true })).toBeInTheDocument();
      expect(dialog.getByRole('button', { name: 'Cancel' })).toBeEnabled();
      expect(dialog.getByRole('button', { name: 'Delete Provider' })).toBeEnabled();
      expect(dialog.queryByRole('button', { name: 'Check saved providers' })).toBeNull();
      expect(writes()).toHaveLength(1);
      expect(writes()[0][0]).toBe('/api/security/sso/providers/saved%2Fprovider');
      expect(writes()[0][1].headers['X-Pulse-Org-ID']).toBe('delete-feedback-a');
      expect(reads()).toHaveLength(1);
      expect(notices.error).toHaveBeenCalledExactlyOnceWith(message);
      expect(notices.success).not.toHaveBeenCalled();
      expect(changed).not.toHaveBeenCalled();
      expect(document.body).not.toHaveTextContent(privateDetail);
      expect(JSON.stringify(logs.error.mock.calls)).not.toContain(privateDetail);

      fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
      expect(screen.queryByRole('dialog')).toBeNull();
      const reopened = await openConfirmation();
      expect(reopened.queryByRole('alert')).toBeNull();
    },
  );

  it('clears the old refusal only for an explicit retry, then retains uncertain-result recovery', async () => {
    let release!: (response: Response) => void;
    const pending = new Promise<Response>((resolve) => {
      release = resolve;
    });
    let attempts = 0;
    fetchMock.mockImplementation(async (_url: string, options?: RequestInit) => {
      if (options?.method === 'DELETE') {
        attempts++;
        return attempts === 1 ? json({ error: privateDetail }, 409) : pending;
      }
      return json({ providers: [provider(type)] });
    });
    render(() => <SSOProvidersPanel currentUser="admin" />);
    const dialog = await openConfirmation();
    remove();
    await dialog.findByRole('alert');
    remove();
    expect(dialog.queryByRole('alert')).toBeNull();
    expect(dialog.getByRole('button', { name: 'Cancel' })).toBeDisabled();
    expect(dialog.getByRole('button', { name: 'Delete Provider' })).toBeDisabled();
    remove();
    expect(writes()).toHaveLength(2);
    release(json({ error: privateDetail }, 502));
    expect(await dialog.findByRole('alert')).toHaveTextContent('may already have been applied');
    expect(dialog.getByRole('alert')).not.toHaveTextContent(message);
    expect(dialog.getByRole('button', { name: 'Delete Provider' })).toBeDisabled();
    fireEvent.click(dialog.getByRole('button', { name: 'Check saved providers' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(reads()).toHaveLength(2);
    expect(writes()).toHaveLength(2);
    expect(notices.success).not.toHaveBeenCalled();
    expect(document.body).not.toHaveTextContent(privateDetail);
    const reopened = await openConfirmation();
    expect(reopened.queryByRole('alert')).toBeNull();
  });

  it('does not attribute a previous provider refusal to another confirmation', async () => {
    fetchMock.mockImplementation(async (_url: string, options?: RequestInit) =>
      options?.method === 'DELETE'
        ? json({}, 409)
        : json({ providers: [provider(type), provider(type, 'other/provider')] }),
    );
    render(() => <SSOProvidersPanel />);
    const dialog = await openConfirmation();
    remove();
    await dialog.findByRole('alert');
    fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
    const next = await openConfirmation('other/provider');
    expect(next.queryByRole('alert')).toBeNull();
    expect(next.getByText('other/provider', { exact: true })).toBeInTheDocument();
    expect(writes()).toHaveLength(1);
  });

  it('ignores a late refusal after an organization switch and leaves the new confirmation clean', async () => {
    let release!: (response: Response) => void;
    const pending = new Promise<Response>((resolve) => {
      release = resolve;
    });
    fetchMock.mockImplementation(async (_url: string, options?: RequestInit) =>
      options?.method === 'DELETE' ? pending : json({ providers: [provider(type)] }),
    );
    render(() => <SSOProvidersPanel currentUser="admin" />);
    await openConfirmation();
    remove();
    setOrgID('delete-feedback-b');
    eventBus.emit('org_switched', 'delete-feedback-b');
    expect(screen.queryByRole('dialog')).toBeNull();
    const next = await openConfirmation();
    release(json({ error: privateDetail }, 409));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(next.queryByRole('alert')).toBeNull();
    expect(writes()).toHaveLength(1);
    expect(writes()[0][1].signal.aborted).toBe(true);
    expect(notices.error).not.toHaveBeenCalled();
    expect(notices.success).not.toHaveBeenCalled();
    expect(document.body).not.toHaveTextContent(privateDetail);
  });
});
