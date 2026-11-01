import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { SSOProvidersPanel } from '../SSOProvidersPanel';
import { eventBus } from '@/stores/events';
import { setOrgID } from '@/utils/apiClient';

const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), warn: vi.fn() } }));
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
  status, headers: { 'Content-Type': 'application/json' },
});
const deferred = () => {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((done) => { resolve = done; });
  return { promise, resolve };
};
const provider = (id = 'first') => ({
  id, name: `${id} provider`, type: 'oidc' as const, enabled: true, priority: 0,
  oidcCallbackUrl: `https://pulse.example.test/api/oidc/${id}/callback`,
});
const details = (id = 'first') => ({ ...provider(id), oidc: {
  issuerUrl: `https://${id}.example.test`, clientId: 'pulse',
} });
const fetchMock = vi.fn();
const listCalls = () => fetchMock.mock.calls.filter(([url, options]) => url === '/api/security/sso/providers' && !options?.method);
const writes = () => fetchMock.mock.calls.filter(([, options]) => options?.method && options.method !== 'GET');
const edit = async (index = 0) => {
  fireEvent.click((await screen.findAllByRole('button', { name: 'Edit provider' }))[index]);
  return within(await screen.findByRole('dialog', { name: 'Edit OIDC provider' }));
};
const settle = async () => { await new Promise((resolve) => setTimeout(resolve, 0)); };

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal('fetch', fetchMock);
  document.cookie = 'pulse_csrf=synthetic-sso-csrf';
  setOrgID('sso-a');
  fetchMock.mockImplementation(async (url: string) => {
    if (url === '/api/security/sso/providers') return json({ providers: [provider(), provider('second')] });
    if (url === '/api/security/status') return json({ publicUrl: 'https://pulse.example.test' });
    if (url.endsWith('/first')) return json(details());
    if (url.endsWith('/second')) return json(details('second'));
    return json({});
  });
});
afterEach(() => { cleanup(); setOrgID(null); vi.unstubAllGlobals(); });

describe('SSO provider read and editor ownership', () => {
  it.each([{}, { providers: null }, { providers: [provider(), provider()] }, { providers: [{ id: 'incomplete' }] }])(
    'keeps malformed inventory unknown and recovers only with an explicit read: %j', async (body) => {
      fetchMock.mockResolvedValueOnce(json(body));
      render(() => <SSOProvidersPanel />);
      expect(await screen.findByRole('alert')).toHaveTextContent('Configured providers are unknown');
      expect(screen.queryByText('No SSO providers configured')).toBeNull();
      expect(screen.getByRole('button', { name: 'Add OIDC' })).toBeDisabled();
      expect(listCalls()).toHaveLength(1);
      fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
      await screen.findAllByText('first provider');
      expect(listCalls()).toHaveLength(2);
      expect(writes()).toHaveLength(0);
      expect(screen.queryByRole('alert')).toBeNull();
    },
  );

  it('reports refused inventory without private text or a false empty state, including read-only recovery', async () => {
    fetchMock.mockResolvedValueOnce(json({ error: 'PRIVATE_SSO_CANARY' }, 403));
    render(() => <SSOProvidersPanel canManage={false} />);
    expect(await screen.findByRole('alert')).toHaveTextContent('Configured providers are unknown');
    expect(document.body).not.toHaveTextContent('PRIVATE_SSO_CANARY');
    expect(screen.queryByText('No SSO providers configured')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await screen.findByText('first provider');
    expect(screen.getAllByRole('button', { name: 'Edit provider' })[0]).toBeDisabled();
    expect(listCalls()).toHaveLength(2);
    expect(writes()).toHaveLength(0);
  });

  it('accepts only the latest requested provider details, even if aborted fetch completes', async () => {
    const old = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) => url.endsWith('/first') ? old.promise : base(url, options));
    render(() => <SSOProvidersPanel />);
    fireEvent.click((await screen.findAllByRole('button', { name: 'Edit provider' }))[0]);
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Loading provider details'));
    const dialog = await edit(1);
    expect(dialog.getByLabelText('Provider Name')).toHaveValue('second provider');
    old.resolve(json(details()));
    await settle();
    expect(dialog.getByLabelText('Provider Name')).toHaveValue('second provider');
    const request = fetchMock.mock.calls.find(([url]) => url.endsWith('/first'))!;
    expect(request[1].signal.aborted).toBe(true);
    expect(writes()).toHaveLength(0);
  });

  it('does not reopen an old detail read over a newly chosen create dialog', async () => {
    const old = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) => url.endsWith('/first') ? old.promise : base(url, options));
    render(() => <SSOProvidersPanel />);
    fireEvent.click((await screen.findAllByRole('button', { name: 'Edit provider' }))[0]);
    fireEvent.click(screen.getByRole('button', { name: 'Add SAML' }));
    old.resolve(json(details()));
    await settle();
    const dialog = screen.getByRole('dialog', { name: 'Add SAML provider' });
    expect(within(dialog).getByLabelText('Provider Name')).toHaveValue('');
    expect(screen.queryByRole('dialog', { name: 'Edit OIDC provider' })).toBeNull();
  });

  it('rejects a response for another provider rather than editing it under the selected id', async () => {
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) => url.endsWith('/first') ? json(details('second')) : base(url, options));
    render(() => <SSOProvidersPanel />);
    fireEvent.click((await screen.findAllByRole('button', { name: 'Edit provider' }))[0]);
    await waitFor(() => expect(notices.error).toHaveBeenCalledWith('Unable to load SSO provider details.'));
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it.each(['user', 'organisation', 'permission'] as const)('withdraws stale detail and drafts on a %s change, without replay', async (change) => {
    const pending = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) => url.endsWith('/first') ? pending.promise : base(url, options));
    const [user, setUser] = createSignal('admin-a');
    const [manage, setManage] = createSignal(true);
    render(() => <SSOProvidersPanel currentUser={user()} canManage={manage()} />);
    fireEvent.click((await screen.findAllByRole('button', { name: 'Edit provider' }))[0]);
    await waitFor(() => expect(fetchMock.mock.calls.some(([url]) => url.endsWith('/first'))).toBe(true));
    if (change === 'user') setUser('admin-b');
    else if (change === 'permission') setManage(false);
    else { setOrgID('sso-b'); eventBus.emit('org_switched', 'sso-b'); }
    await waitFor(() => expect(listCalls()).toHaveLength(2));
    pending.resolve(json(details()));
    await settle();
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(notices.success).not.toHaveBeenCalled();
    expect(notices.error).not.toHaveBeenCalled();
    expect(writes()).toHaveLength(0);
    expect(listCalls()[1][1].headers['X-Pulse-Org-ID']).toBe(change === 'organisation' ? 'sso-b' : 'sso-a');
  });

  it('does not reset the draft for an unchanged current principal and capability refresh', async () => {
    const [status, setStatus] = createSignal({ user: 'admin-a', manage: true });
    render(() => <SSOProvidersPanel currentUser={status().user} canManage={status().manage} />);
    const dialog = await edit();
    fireEvent.input(dialog.getByLabelText('Client Secret'), { target: { value: 'SYNTHETIC_DRAFT_ONLY' } });
    setStatus({ user: 'admin-a', manage: true });
    await settle();
    expect(dialog.getByLabelText('Client Secret')).toHaveValue('SYNTHETIC_DRAFT_ONLY');
    expect(listCalls()).toHaveLength(1);
  });

  it('discards an older inventory after organisation switch, even if its fetch ignores abort', async () => {
    const old = deferred();
    fetchMock.mockReturnValueOnce(old.promise);
    render(() => <SSOProvidersPanel />);
    await waitFor(() => expect(listCalls()).toHaveLength(1));
    setOrgID('sso-b'); eventBus.emit('org_switched', 'sso-b');
    await screen.findByText('first provider');
    old.resolve(json({ providers: [provider('OBSOLETE')] }));
    await settle();
    expect(screen.queryByText('OBSOLETE provider')).toBeNull();
    expect(screen.getByText('first provider')).toBeInTheDocument();
    expect(listCalls()[0][1].signal.aborted).toBe(true);
  });

  it.each(['test', 'preview'] as const)('drops late %s output after dialog close and releases private draft fields', async (kind) => {
    const pending = deferred();
    const endpoint = kind === 'test' ? '/test' : '/metadata/preview';
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) => url.endsWith(endpoint) ? pending.promise : base(url, options));
    render(() => <SSOProvidersPanel />);
    await screen.findByText('first provider');
    fireEvent.click(screen.getByRole('button', { name: kind === 'test' ? 'Add OIDC' : 'Add SAML' }));
    const dialog = within(screen.getByRole('dialog'));
    fireEvent.input(dialog.getByLabelText(kind === 'test' ? 'Issuer URL' : 'IdP Metadata URL'), { target: { value: 'https://idp.example.test/metadata' } });
    if (kind === 'test') fireEvent.input(dialog.getByLabelText('Client Secret'), { target: { value: 'SYNTHETIC_SECRET' } });
    fireEvent.click(dialog.getByRole('button', { name: kind === 'test' ? 'Test' : 'Preview' }));
    await waitFor(() => expect(writes()).toHaveLength(1));
    fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
    fireEvent.click(screen.getByRole('button', { name: 'Add OIDC' }));
    pending.resolve(json(kind === 'test' ? { success: true, message: 'OBSOLETE_TEST' } : { xml: 'OBSOLETE_XML', parsed: { entityId: 'OBSOLETE' } }));
    await settle();
    expect(screen.getAllByRole('dialog')).toHaveLength(1);
    expect(document.body).not.toHaveTextContent('OBSOLETE');
    expect(screen.getByLabelText('Client Secret')).toHaveValue('');
    expect(writes()[0][1].signal.aborted).toBe(true);
    expect(notices.success).not.toHaveBeenCalled();
  });

  it('clears a test result and aborts a pending test when the submitted draft changes', async () => {
    const pending = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) => url.endsWith('/test') ? pending.promise : base(url, options));
    render(() => <SSOProvidersPanel />);
    const dialog = await edit();
    fireEvent.click(dialog.getByRole('button', { name: 'Test' }));
    await waitFor(() => expect(writes()).toHaveLength(1));
    fireEvent.input(dialog.getByLabelText('Issuer URL'), { target: { value: 'https://changed.example.test' } });
    expect(writes()[0][1].signal.aborted).toBe(true);
    pending.resolve(json({ success: true, message: 'STALE_TEST' }));
    await settle();
    expect(document.body).not.toHaveTextContent('STALE_TEST');
    expect(dialog.getByRole('button', { name: 'Test' })).toBeEnabled();
  });

  it.each(['save', 'toggle', 'delete'] as const)('serialises %s writes and ignores completion after withdrawal', async (kind) => {
    const pending = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) => options?.method && options.method !== 'GET' ? pending.promise : base(url, options));
    const changed = vi.fn();
    const [manage, setManage] = createSignal(true);
    render(() => <SSOProvidersPanel canManage={manage()} onConfigUpdated={changed} />);
    if (kind === 'save') {
      const dialog = await edit();
      const form = dialog.getByRole('button', { name: 'Save Changes' }).closest('form')!;
      fireEvent.submit(form); fireEvent.submit(form);
    } else {
      await screen.findByText('first provider');
      if (kind === 'delete') {
        fireEvent.click(screen.getAllByRole('button', { name: 'Delete provider' })[0]);
        const confirm = within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete Provider' });
        fireEvent.click(confirm); fireEvent.click(confirm);
      } else {
        const toggle = screen.getAllByRole('switch')[0];
        fireEvent.click(toggle); fireEvent.click(toggle);
      }
    }
    await waitFor(() => expect(writes()).toHaveLength(1));
    setManage(false);
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    pending.resolve(json({}));
    await settle();
    expect(notices.success).not.toHaveBeenCalled();
    expect(notices.error).not.toHaveBeenCalled();
    expect(changed).not.toHaveBeenCalled();
    expect(writes()).toHaveLength(1);
    expect(writes()[0][1].signal.aborted).toBe(true);
    expect(writes()[0][1].headers['X-Pulse-Org-ID']).toBe('sso-a');
  });

  it('aborts pending detail on unmount without a late notification', async () => {
    const pending = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) => url.endsWith('/first') ? pending.promise : base(url, options));
    const { unmount } = render(() => <SSOProvidersPanel />);
    fireEvent.click((await screen.findAllByRole('button', { name: 'Edit provider' }))[0]);
    await waitFor(() => expect(fetchMock.mock.calls.some(([url]) => url.endsWith('/first'))).toBe(true));
    unmount(); pending.resolve(json(details())); await settle();
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(notices.error).not.toHaveBeenCalled();
  });
});
