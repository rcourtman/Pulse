import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { SSOProvidersPanel } from '../SSOProvidersPanel';
import { eventBus } from '@/stores/events';
import { setOrgID } from '@/utils/apiClient';

const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), warn: vi.fn() } }));
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
const deferred = () => {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
const provider = (id = 'first') => ({
  id,
  name: `${id} provider`,
  type: 'oidc' as const,
  enabled: true,
  priority: 0,
  oidcCallbackUrl: `https://pulse.example.test/api/oidc/${id}/callback`,
});
const details = (id = 'first') => ({
  ...provider(id),
  oidc: {
    issuerUrl: `https://${id}.example.test`,
    clientId: 'pulse',
  },
});
const fetchMock = vi.fn();
const listCalls = () =>
  fetchMock.mock.calls.filter(
    ([url, options]) => url === '/api/security/sso/providers' && !options?.method,
  );
const writes = () =>
  fetchMock.mock.calls.filter(([, options]) => options?.method && options.method !== 'GET');
const edit = async (index = 0) => {
  fireEvent.click((await screen.findAllByRole('button', { name: /^Edit provider / }))[index]);
  return within(await screen.findByRole('dialog', { name: 'Edit OIDC provider' }));
};
const settle = async () => {
  await new Promise((resolve) => setTimeout(resolve, 0));
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal('fetch', fetchMock);
  document.cookie = 'pulse_csrf=synthetic-sso-csrf';
  setOrgID('sso-a');
  fetchMock.mockImplementation(async (url: string) => {
    if (url === '/api/security/sso/providers')
      return json({ providers: [provider(), provider('second')] });
    if (url === '/api/security/status') return json({ publicUrl: 'https://pulse.example.test' });
    if (url.endsWith('/first')) return json(details());
    if (url.endsWith('/second')) return json(details('second'));
    return json({});
  });
});
afterEach(() => {
  cleanup();
  setOrgID(null);
  vi.unstubAllGlobals();
});

describe('SSO provider read and editor ownership', () => {
  it.each([
    {},
    { providers: null },
    { providers: [provider(), provider()] },
    { providers: [{ id: 'incomplete' }] },
  ])(
    'keeps malformed inventory unknown and recovers only with an explicit read: %j',
    async (body) => {
      fetchMock.mockResolvedValueOnce(json(body));
      render(() => <SSOProvidersPanel />);
      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Unable to load SSO providers. Configured providers are unknown.',
      );
      expect(notices.error).not.toHaveBeenCalled();
      expect(screen.queryByText('No SSO providers configured')).toBeNull();
      expect(screen.getByRole('button', { name: 'Add OIDC' })).toBeDisabled();
      expect(listCalls()).toHaveLength(1);
      fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
      await screen.findAllByText('first provider');
      expect(listCalls()).toHaveLength(2);
      expect(writes()).toHaveLength(0);
      expect(screen.queryByRole('alert')).toBeNull();
      expect(notices.error).not.toHaveBeenCalled();
    },
  );

  it('reports refused inventory without private text or a false empty state, including read-only recovery', async () => {
    fetchMock.mockResolvedValueOnce(json({ error: 'PRIVATE_SSO_CANARY' }, 403));
    render(() => <SSOProvidersPanel canManage={false} />);
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Unable to load SSO providers. Configured providers are unknown.',
    );
    expect(notices.error).not.toHaveBeenCalled();
    expect(document.body).not.toHaveTextContent('PRIVATE_SSO_CANARY');
    expect(screen.queryByText('No SSO providers configured')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await screen.findByText('first provider');
    expect(screen.getAllByRole('button', { name: /^Edit provider / })[0]).toBeDisabled();
    expect(listCalls()).toHaveLength(2);
    expect(writes()).toHaveLength(0);
    expect(notices.error).not.toHaveBeenCalled();
  });

  it('accepts only the latest requested provider details, even if aborted fetch completes', async () => {
    const old = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/first') ? old.promise : base(url, options),
    );
    render(() => <SSOProvidersPanel />);
    fireEvent.click((await screen.findAllByRole('button', { name: /^Edit provider / }))[0]);
    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('Loading provider details'),
    );
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
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/first') ? old.promise : base(url, options),
    );
    render(() => <SSOProvidersPanel />);
    fireEvent.click((await screen.findAllByRole('button', { name: /^Edit provider / }))[0]);
    fireEvent.click(screen.getByRole('button', { name: 'Add SAML' }));
    old.resolve(json(details()));
    await settle();
    const dialog = screen.getByRole('dialog', { name: 'Add SAML provider' });
    expect(within(dialog).getByLabelText('Provider Name')).toHaveValue('');
    expect(screen.queryByRole('dialog', { name: 'Edit OIDC provider' })).toBeNull();
  });

  it('rejects a response for another provider rather than editing it under the selected id', async () => {
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/first') ? json(details('second')) : base(url, options),
    );
    render(() => <SSOProvidersPanel />);
    fireEvent.click((await screen.findAllByRole('button', { name: /^Edit provider / }))[0]);
    await waitFor(() =>
      expect(notices.error).toHaveBeenCalledWith('Unable to load SSO provider details.'),
    );
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it.each(['user', 'organisation', 'permission'] as const)(
    'withdraws stale detail and drafts on a %s change, without replay',
    async (change) => {
      const pending = deferred();
      const base = fetchMock.getMockImplementation()!;
      fetchMock.mockImplementation((url: string, options?: RequestInit) =>
        url.endsWith('/first') ? pending.promise : base(url, options),
      );
      const [user, setUser] = createSignal('admin-a');
      const [manage, setManage] = createSignal(true);
      render(() => <SSOProvidersPanel currentUser={user()} canManage={manage()} />);
      fireEvent.click((await screen.findAllByRole('button', { name: /^Edit provider / }))[0]);
      await waitFor(() =>
        expect(fetchMock.mock.calls.some(([url]) => url.endsWith('/first'))).toBe(true),
      );
      if (change === 'user') setUser('admin-b');
      else if (change === 'permission') setManage(false);
      else {
        setOrgID('sso-b');
        eventBus.emit('org_switched', 'sso-b');
      }
      await waitFor(() => expect(listCalls()).toHaveLength(2));
      pending.resolve(json(details()));
      await settle();
      expect(screen.queryByRole('dialog')).toBeNull();
      expect(notices.success).not.toHaveBeenCalled();
      expect(notices.error).not.toHaveBeenCalled();
      expect(writes()).toHaveLength(0);
      const create = screen.getByRole('button', { name: 'Add OIDC' });
      if (change === 'permission') {
        expect(create).toBeDisabled();
      } else {
        await waitFor(() => expect(create).toBeEnabled());
        expect(screen.queryByText(/Single sign-on is read-only/)).toBeNull();
        fireEvent.click(create);
        expect(screen.getByLabelText('Client Secret')).toHaveValue('');
        fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }));
      }
      expect(listCalls()[1][1].headers['X-Pulse-Org-ID']).toBe(
        change === 'organisation' ? 'sso-b' : 'sso-a',
      );
    },
  );

  it('does not reset the draft for an unchanged current principal and capability refresh', async () => {
    const [status, setStatus] = createSignal({ user: 'admin-a', manage: true });
    render(() => <SSOProvidersPanel currentUser={status().user} canManage={status().manage} />);
    const dialog = await edit();
    fireEvent.input(dialog.getByLabelText('Client Secret'), {
      target: { value: 'SYNTHETIC_DRAFT_ONLY' },
    });
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
    setOrgID('sso-b');
    eventBus.emit('org_switched', 'sso-b');
    await screen.findByText('first provider');
    old.resolve(json({ providers: [provider('OBSOLETE')] }));
    await settle();
    expect(screen.queryByText('OBSOLETE provider')).toBeNull();
    expect(screen.getByText('first provider')).toBeInTheDocument();
    expect(listCalls()[0][1].signal.aborted).toBe(true);
  });

  it.each(['test', 'preview'] as const)(
    'drops late %s output after dialog close and releases private draft fields',
    async (kind) => {
      const pending = deferred();
      const endpoint = kind === 'test' ? '/test' : '/metadata/preview';
      const base = fetchMock.getMockImplementation()!;
      fetchMock.mockImplementation((url: string, options?: RequestInit) =>
        url.endsWith(endpoint) ? pending.promise : base(url, options),
      );
      render(() => <SSOProvidersPanel />);
      await screen.findByText('first provider');
      fireEvent.click(
        screen.getByRole('button', { name: kind === 'test' ? 'Add OIDC' : 'Add SAML' }),
      );
      const dialog = within(screen.getByRole('dialog'));
      fireEvent.input(dialog.getByLabelText(kind === 'test' ? 'Issuer URL' : 'IdP Metadata URL'), {
        target: { value: 'https://idp.example.test/metadata' },
      });
      if (kind === 'test')
        fireEvent.input(dialog.getByLabelText('Client Secret'), {
          target: { value: 'SYNTHETIC_SECRET' },
        });
      fireEvent.click(dialog.getByRole('button', { name: kind === 'test' ? 'Test' : 'Preview' }));
      await waitFor(() => expect(writes()).toHaveLength(1));
      fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
      fireEvent.click(screen.getByRole('button', { name: 'Add OIDC' }));
      pending.resolve(
        json(
          kind === 'test'
            ? { success: true, message: 'OBSOLETE_TEST' }
            : { xml: 'OBSOLETE_XML', parsed: { entityId: 'OBSOLETE' } },
        ),
      );
      await settle();
      expect(screen.getAllByRole('dialog')).toHaveLength(1);
      expect(document.body).not.toHaveTextContent('OBSOLETE');
      expect(screen.getByLabelText('Client Secret')).toHaveValue('');
      expect(writes()[0][1].signal.aborted).toBe(true);
      expect(notices.success).not.toHaveBeenCalled();
    },
  );

  it('clears a test result and aborts a pending test when the submitted draft changes', async () => {
    const pending = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/test') ? pending.promise : base(url, options),
    );
    render(() => <SSOProvidersPanel />);
    const dialog = await edit();
    fireEvent.click(dialog.getByRole('button', { name: 'Test' }));
    await waitFor(() => expect(writes()).toHaveLength(1));
    fireEvent.input(dialog.getByLabelText('Issuer URL'), {
      target: { value: 'https://changed.example.test' },
    });
    expect(writes()[0][1].signal.aborted).toBe(true);
    pending.resolve(json({ success: true, message: 'STALE_TEST' }));
    await settle();
    expect(document.body).not.toHaveTextContent('STALE_TEST');
    expect(dialog.getByRole('button', { name: 'Test' })).toBeEnabled();
  });

  it('keeps OIDC test focus and opens the details only on request', async () => {
    const pending = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/test') ? pending.promise : base(url, options),
    );
    render(() => <SSOProvidersPanel />);
    const dialog = await edit();
    const button = dialog.getByRole('button', { name: 'Test' });
    button.focus();
    fireEvent.click(button);
    expect(dialog.getByRole('button', { name: 'Testing...' })).toBe(button);
    expect(button).toHaveFocus();
    expect(button).not.toBeDisabled();
    expect(button).toHaveAttribute('aria-disabled', 'true');
    fireEvent.click(button);
    await waitFor(() => expect(writes()).toHaveLength(1));

    const tokenEndpoint = 'https://idp.example.test/discovery/realm/protocol/openid/token';
    pending.resolve(
      json({ success: true, message: 'Discovery succeeded', details: { tokenEndpoint } }),
    );
    const result = await dialog.findByRole('status');
    expect(result).toHaveTextContent('Discovery succeeded');
    expect(result).toHaveTextContent(tokenEndpoint);
    expect(button).toHaveFocus();
    expect(button).not.toHaveAttribute('aria-disabled');
    expect(button).toHaveAccessibleDescription('Issuer matched Sign-in not tested');
    expect(dialog.getByLabelText('Client Secret').compareDocumentPosition(result)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );

    const scroller = result.closest('fieldset')!.parentElement!;
    vi.spyOn(scroller, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 100, 320, 500));
    vi.spyOn(result, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 800, 320, 100));
    const view = dialog.getByRole('button', {
      name: 'Issuer matched, Sign-in not tested — view result',
    });
    expect(view).toHaveAttribute('aria-controls', result.id);
    fireEvent.click(view);
    expect(result).toHaveFocus();
    expect(scroller.scrollTop).toBe(688);

    const dismiss = within(result).getByRole('button', { name: 'Dismiss test result' });
    dismiss.focus();
    fireEvent.click(dismiss);
    expect(result).not.toBeInTheDocument();
    expect(view).not.toBeInTheDocument();
    expect(button).toHaveFocus();
    expect(writes()).toHaveLength(1);
  });

  const issuerMismatch = {
    success: false,
    error: 'issuer_mismatch',
    message: 'UNTRUSTED_RESULT_TEXT',
    details: { entityId: 'UNTRUSTED_RESULT_DETAILS' },
  };
  const genericTestFailure = 'Unable to run the connection test.';

  it('reports the draft issuer mismatch without echoing the response or saving', async () => {
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/test') ? json(issuerMismatch, 400) : base(url, options),
    );
    render(() => <SSOProvidersPanel />);
    const dialog = await edit();
    const field = dialog.getByLabelText('Issuer URL');
    const client = dialog.getByLabelText('Client ID');
    const button = dialog.getByRole('button', { name: 'Test' });
    const issuerUrl = 'https://first.example.test/realm/';
    fireEvent.input(field, { target: { value: issuerUrl } });
    fireEvent.input(dialog.getByLabelText('Client Secret'), {
      target: { value: 'fixture-secret' },
    });
    button.focus();
    fireEvent.click(button);

    const result = await dialog.findByRole('alert');
    expect(result).toHaveTextContent('Check the exact Issuer URL in your IdP settings.');
    expect(result).toHaveTextContent('Nothing saved. Sign-in not tested.');
    expect(result).not.toHaveTextContent(/issuer_mismatch|UNTRUSTED_RESULT|fixture-secret/);
    expect(field).toHaveValue(issuerUrl);
    expect(client).toHaveValue('pulse');
    expect(button).toHaveFocus();
    expect(button).toHaveAccessibleDescription('Issuer mismatch');
    expect(dialog.getByRole('button', { name: 'Save Changes' })).toBeEnabled();
    const view = dialog.getByRole('button', { name: 'Issuer mismatch — view result' });
    expect(view).toHaveAttribute('aria-controls', result.id);
    fireEvent.click(view);
    expect(result).toHaveFocus();

    fireEvent.input(field, { target: { value: 'https://first.example.test/realm' } });
    expect(result).not.toBeInTheDocument();
    expect(view).not.toBeInTheDocument();
    expect(button).not.toHaveAttribute('aria-describedby');
    expect(writes()).toHaveLength(1);
    expect(JSON.parse(writes()[0][1].body)).toEqual({
      type: 'oidc',
      oidc: { issuerUrl, clientId: 'pulse' },
    });
  });

  it.each([
    ['unknown finding', 400, { ...issuerMismatch, error: 'other_finding' }],
    ['ordinary API error', 400, { code: 'issuer_mismatch', error: 'UNTRUSTED_RESULT_TEXT' }],
    ['contradictory success', 400, { ...issuerMismatch, success: true }],
    ['non-boolean success', 400, { ...issuerMismatch, success: 'false' }],
    ['null body', 400, null],
    ['malformed body', 400, '<p>UNTRUSTED_RESULT_TEXT</p>'],
    ['denial', 403, issuerMismatch],
    ['rate limit', 429, issuerMismatch],
    ['server error', 500, issuerMismatch],
    ['2xx failure envelope', 200, issuerMismatch],
  ])('keeps a %s generic and does not retry it', async (_label, status, body) => {
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/test')
        ? typeof body === 'string'
          ? new Response(body, { status })
          : json(body, status)
        : base(url, options),
    );
    render(() => <SSOProvidersPanel />);
    const dialog = await edit();
    fireEvent.click(dialog.getByRole('button', { name: 'Test' }));
    expect(await dialog.findByRole('alert')).toHaveTextContent(genericTestFailure);
    expect(dialog.getByRole('button', { name: 'Test failed — view result' })).toBeInTheDocument();
    expect(document.body).not.toHaveTextContent(/UNTRUSTED_RESULT|issuer_mismatch/);
    expect(writes()).toHaveLength(1);
  });

  it('never applies a mismatch decoded after the draft has changed', async () => {
    const pending = deferred();
    const response = json(issuerMismatch, 400);
    const decode = vi
      .spyOn(response, 'json')
      .mockImplementation(async () => (await pending.promise).json());
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/test') ? response : base(url, options),
    );
    render(() => <SSOProvidersPanel />);
    const dialog = await edit();
    const button = dialog.getByRole('button', { name: 'Test' });
    fireEvent.click(button);
    await waitFor(() => expect(decode).toHaveBeenCalledOnce());
    fireEvent.input(dialog.getByLabelText('Issuer URL'), {
      target: { value: 'https://changed.example.test' },
    });
    pending.resolve(json(issuerMismatch, 400));
    await settle();
    expect(writes()[0][1].signal.aborted).toBe(true);
    expect(dialog.queryByRole('alert')).toBeNull();
    expect(button).not.toHaveAttribute('aria-describedby');
    expect(button).toBeEnabled();
  });

  const currentCertificate = {
    subject: 'CN=Current signing certificate',
    notAfter: '2027-08-01T00:00:00Z',
    isExpired: false,
  };
  const expiredCertificate = {
    subject: 'CN=Retired signing certificate',
    notAfter: '2026-08-01T00:00:00Z',
    isExpired: true,
  };
  const samlFailure = (error: string) => ({
    success: false,
    error,
    message: 'UNTRUSTED_RESULT_TEXT',
    details: { entityId: 'UNTRUSTED_RESULT_DETAILS', certificates: [] },
  });

  it.each([
    {
      mode: 'manual URL only',
      saved: null,
      result: {
        success: true,
        message:
          'Only the SAML SSO URL format was checked. Certificates and sign-in have not been tested.',
      },
      summary: ['URL format OK', 'Sign-in not tested'],
      title: 'Check the IdP SSO URL format only',
    },
    {
      mode: 'manual URL rejected',
      saved: null,
      ssoUrl: 'not-a-url',
      result: samlFailure('invalid_url'),
      summary: ['Invalid SSO URL'],
      guidance: 'Enter a full HTTP or HTTPS IdP SSO URL.',
      title: 'Check the IdP SSO URL format only',
    },
    {
      mode: 'metadata URL rejected while manual and inline details are also set',
      saved: { idpMetadataUrl: 'not-a-url', idpMetadataXml: '<EntityDescriptor />' },
      result: samlFailure('invalid_url'),
      summary: ['Invalid metadata URL'],
      guidance: 'Enter a full HTTP or HTTPS IdP Metadata URL.',
      title: 'Check SAML metadata',
    },
    {
      mode: 'metadata URL could not be read',
      saved: { idpMetadataUrl: 'https://idp.example.test/metadata' },
      result: samlFailure('fetch_failed'),
      summary: ['Metadata not read'],
      guidance: 'Check that Pulse can read SAML XML from the IdP Metadata URL.',
      title: 'Check SAML metadata',
    },
    {
      mode: 'read metadata URL without a usable IdP',
      saved: { idpMetadataUrl: 'https://pulse.example.test/sp-metadata' },
      result: samlFailure('metadata_incomplete'),
      summary: ['Metadata incomplete'],
      guidance: 'Use IdP metadata with an entity ID and SSO URL, not SP metadata.',
      title: 'Check SAML metadata',
    },
    {
      mode: 'saved metadata XML cannot be parsed',
      saved: { idpMetadataUrl: '  ', idpMetadataXml: 'synthetic malformed metadata' },
      result: samlFailure('parse_failed'),
      summary: ['Invalid metadata XML'],
      guidance: 'Saved metadata XML is unreadable. Test an IdP Metadata URL instead.',
      title: 'Check SAML metadata',
    },
    {
      mode: 'saved metadata XML without a usable IdP',
      saved: { idpMetadataUrl: '  ', idpMetadataXml: '<EntityDescriptor />' },
      result: samlFailure('metadata_incomplete'),
      summary: ['Metadata incomplete'],
      guidance: 'Saved XML needs an IdP identity and SSO URL. Test an IdP Metadata URL instead.',
      title: 'Check SAML metadata',
    },
    {
      mode: 'metadata URL alongside a manual URL',
      saved: { idpMetadataUrl: 'https://idp.example.test/metadata' },
      result: {
        success: true,
        message:
          'SAML metadata includes an IdP identity and an SSO URL. Sign-in has not been tested.',
        details: { certificates: [currentCertificate] },
      },
      summary: ['Metadata read', 'Sign-in not tested'],
      title: 'Check SAML metadata',
    },
    {
      mode: 'saved metadata XML alongside a manual URL',
      saved: { idpMetadataXml: '<EntityDescriptor />', idpMetadataUrl: '  ' },
      result: { success: true, message: 'SAML metadata includes an IdP identity and an SSO URL.' },
      summary: ['Metadata read', 'Sign-in not tested'],
      title: 'Check SAML metadata',
    },
    {
      mode: 'metadata returning both an expired and a current certificate',
      saved: { idpMetadataUrl: 'https://idp.example.test/metadata' },
      result: {
        success: true,
        message:
          'SAML metadata includes an IdP identity and an SSO URL. Sign-in has not been tested.',
        details: { certificates: [expiredCertificate, currentCertificate] },
      },
      summary: ['Metadata read', 'Expired cert found', 'Sign-in not tested'],
      title: 'Check SAML metadata',
    },
  ])('summarizes the observed SAML test for $mode', async (scenario) => {
    const { saved, result, summary, title, guidance } = scenario;
    const samlProvider = {
      id: 'fixture',
      name: 'Fixture',
      type: 'saml',
      enabled: true,
      priority: 0,
    };
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) => {
      if (url.endsWith('/test')) return json(result, result.success ? 200 : 400);
      if (url.endsWith('/fixture')) return json({ ...samlProvider, saml: saved });
      if (saved && url === '/api/security/sso/providers')
        return json({ providers: [samlProvider] });
      return base(url, options);
    });
    render(() => <SSOProvidersPanel />);
    const opener = await screen.findByRole('button', {
      name: saved ? /^Edit provider / : 'Add SAML',
    });
    await waitFor(() => expect(opener).toBeEnabled());
    fireEvent.click(opener);
    const dialog = within(
      await screen.findByRole('dialog', { name: `${saved ? 'Edit' : 'Add'} SAML provider` }),
    );
    const ssoUrl = scenario.ssoUrl ?? 'https://idp.example.test/sso';
    fireEvent.input(dialog.getByLabelText('IdP SSO URL'), { target: { value: ssoUrl } });
    const testButton = dialog.getByRole('button', { name: 'Test' });
    expect(testButton).toHaveAttribute('title', title);
    fireEvent.click(testButton);

    const details = await dialog.findByRole(result.success ? 'status' : 'alert');
    expect(details).toHaveTextContent(
      result.success ? result.message : `${guidance} Nothing saved. Sign-in not tested.`,
    );
    expect(details).not.toHaveTextContent(/UNTRUSTED_RESULT/);
    expect(testButton).toHaveAccessibleDescription(summary.join(' '));
    const summaryButton = dialog.getByRole('button', {
      name: `${summary.join(', ')} — view result`,
    });
    expect(summaryButton).toHaveAttribute('aria-controls', details.id);
    expect(summaryButton).toHaveTextContent(summary.join(' '));
    if (result.details?.certificates.some((certificate) => certificate === expiredCertificate)) {
      expect(details).toHaveTextContent(currentCertificate.subject);
      expect(details).toHaveTextContent(expiredCertificate.subject);
      expect(within(details).getAllByText('(Expired!)')).toHaveLength(1);
      fireEvent.input(dialog.getByLabelText('IdP Metadata URL'), {
        target: { value: 'https://other.example.test/metadata' },
      });
      expect(summaryButton).not.toBeInTheDocument();
      expect(details).not.toBeInTheDocument();
      expect(testButton).not.toHaveAttribute('aria-describedby');
    }
    expect(dialog.getByLabelText('IdP Certificate (PEM)')).toBeEnabled();
    expect(writes()).toHaveLength(1);
    expect(JSON.parse(writes()[0][1].body)).toEqual({
      type: 'saml',
      saml: {
        idpSsoUrl: ssoUrl,
        idpMetadataUrl: saved?.idpMetadataUrl?.trim() || undefined,
        idpMetadataXml: saved && 'idpMetadataXml' in saved ? saved.idpMetadataXml : undefined,
      },
    });
  });

  it.each([
    [400, samlFailure('parse_failed')], // No inline XML in this request.
    [400, samlFailure('unrecognised')],
    [400, issuerMismatch],
    [400, { ...samlFailure('metadata_incomplete'), success: true }],
    [403, samlFailure('metadata_incomplete')],
    [429, samlFailure('metadata_incomplete')],
    [200, samlFailure('metadata_incomplete')],
  ])('keeps mismatched SAML findings generic (HTTP %s): %j', async (status, body) => {
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/test') ? json(body, status) : base(url, options),
    );
    render(() => <SSOProvidersPanel />);
    await screen.findByText('first provider');
    fireEvent.click(screen.getByRole('button', { name: 'Add SAML' }));
    const dialog = within(await screen.findByRole('dialog', { name: 'Add SAML provider' }));
    fireEvent.input(dialog.getByLabelText('IdP Metadata URL'), {
      target: { value: 'https://idp.example.test/metadata' },
    });
    fireEvent.click(dialog.getByRole('button', { name: 'Test' }));
    expect(await dialog.findByRole('alert')).toHaveTextContent(genericTestFailure);
    expect(dialog.getByRole('button', { name: 'Test failed — view result' })).toBeInTheDocument();
    expect(document.body).not.toHaveTextContent(/UNTRUSTED_RESULT/);
    expect(writes()).toHaveLength(1);
  });

  it('shows the SAML failure without reclaiming focus from draft fields', async () => {
    const pending = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/test') ? pending.promise : base(url, options),
    );
    render(() => <SSOProvidersPanel />);
    await screen.findByText('first provider');
    fireEvent.click(screen.getByRole('button', { name: 'Add SAML' }));
    const dialog = within(await screen.findByRole('dialog', { name: 'Add SAML provider' }));
    const metadata = dialog.getByLabelText('IdP Metadata URL');
    fireEvent.input(metadata, { target: { value: 'https://idp.example.test/metadata' } });
    const button = dialog.getByRole('button', { name: 'Test' });
    button.focus();
    fireEvent.click(button);
    await waitFor(() => expect(writes()).toHaveLength(1));

    metadata.focus();
    pending.resolve(json(samlFailure('metadata_incomplete'), 400));
    const result = await dialog.findByRole('alert');
    expect(result).toHaveTextContent('Use IdP metadata with an entity ID and SSO URL');
    expect(metadata).toHaveFocus();
    expect(dialog.getByLabelText('IdP SSO URL').compareDocumentPosition(result)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
    const view = dialog.getByRole('button', { name: 'Metadata incomplete — view result' });
    expect(view).toHaveAttribute('aria-controls', result.id);
    const preview = dialog.getByRole('button', { name: 'Preview' });
    expect(preview).toBeEnabled();

    fireEvent.input(metadata, { target: { value: 'https://idp.example.test/new-metadata' } });
    expect(result).not.toBeInTheDocument();
    expect(view).not.toBeInTheDocument();
    expect(writes()).toHaveLength(1);
  });

  it.each(['save', 'toggle', 'delete'] as const)(
    'serialises %s writes and ignores completion after withdrawal',
    async (kind) => {
      const pending = deferred();
      const base = fetchMock.getMockImplementation()!;
      fetchMock.mockImplementation((url: string, options?: RequestInit) =>
        options?.method && options.method !== 'GET' ? pending.promise : base(url, options),
      );
      const changed = vi.fn();
      const [manage, setManage] = createSignal(true);
      render(() => <SSOProvidersPanel canManage={manage()} onConfigUpdated={changed} />);
      if (kind === 'save') {
        const dialog = await edit();
        const form = dialog.getByRole('button', { name: 'Save Changes' }).closest('form')!;
        fireEvent.submit(form);
        fireEvent.submit(form);
      } else {
        await screen.findByText('first provider');
        if (kind === 'delete') {
          fireEvent.click(screen.getAllByRole('button', { name: /^Delete provider / })[0]);
          const confirm = within(screen.getByRole('dialog')).getByRole('button', {
            name: 'Delete Provider',
          });
          fireEvent.click(confirm);
          fireEvent.click(confirm);
        } else {
          const toggle = screen.getByRole('button', { name: 'Disable first provider' });
          fireEvent.click(toggle);
          fireEvent.click(toggle);
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
    },
  );

  it('aborts pending detail on unmount without a late notification', async () => {
    const pending = deferred();
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/first') ? pending.promise : base(url, options),
    );
    const { unmount } = render(() => <SSOProvidersPanel />);
    fireEvent.click((await screen.findAllByRole('button', { name: /^Edit provider / }))[0]);
    await waitFor(() =>
      expect(fetchMock.mock.calls.some(([url]) => url.endsWith('/first'))).toBe(true),
    );
    unmount();
    pending.resolve(json(details()));
    await settle();
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(notices.error).not.toHaveBeenCalled();
  });
});
