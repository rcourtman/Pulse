import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { SSOProvidersPanel } from '../SSOProvidersPanel';

const fetchMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const loggerErrorMock = vi.fn();
const loggerWarnMock = vi.fn();

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: (...args: unknown[]) => loggerErrorMock(...args),
    warn: (...args: unknown[]) => loggerWarnMock(...args),
  },
}));

const jsonResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const corpProvider = {
  id: 'corp-oidc',
  name: 'Corporate OIDC',
  type: 'oidc' as const,
  enabled: true,
  priority: 0,
};

const corpProviderDetails = {
  id: 'corp-oidc',
  name: 'Corporate OIDC',
  type: 'oidc' as const,
  enabled: true,
  oidc: {
    issuerUrl: 'https://idp.example.com',
    clientId: 'pulse',
    scopes: ['openid', 'profile', 'email', 'groups'],
  },
  groupsClaim: 'groups',
  allowedGroups: ['admins'],
  groupRoleMappings: { admins: 'admin' },
};

const setupFetch = (providers: unknown[], providerDetails: unknown = corpProviderDetails) => {
  fetchMock.mockImplementation((url: string, options?: RequestInit) => {
    const method = (options?.method ?? 'GET').toUpperCase();
    if (url === '/api/security/sso/providers' && method === 'GET') {
      return Promise.resolve(jsonResponse({ providers, allowMultipleProviders: false }));
    }
    if (url === '/api/security/status') {
      return Promise.resolve(jsonResponse({ publicUrl: 'https://pulse.example.com' }));
    }
    if (url === '/api/security/sso/providers/corp-oidc' && method === 'GET') {
      return Promise.resolve(jsonResponse(providerDetails));
    }
    if (method === 'POST' || method === 'PUT') {
      return Promise.resolve(jsonResponse({ id: 'corp-oidc' }));
    }
    return Promise.resolve(jsonResponse({}));
  });
};

const scopesInput = () => screen.getByPlaceholderText('openid profile email') as HTMLInputElement;

describe('SSOProvidersPanel OIDC scopes', () => {
  beforeEach(() => {
    fetchMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    loggerErrorMock.mockReset();
    loggerWarnMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
    document.cookie = 'pulse_csrf=test-csrf-token';
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('lets an admin create an OIDC provider with a custom scope set', async () => {
    setupFetch([]);
    render(() => <SSOProvidersPanel />);

    const addButton = await screen.findByRole('button', { name: /Add OIDC/i });
    fireEvent.click(addButton);

    const scopes = scopesInput();
    expect(scopes.value).toBe('openid profile email');

    fireEvent.input(screen.getByPlaceholderText('e.g., Corporate SSO'), {
      target: { value: 'Corporate OIDC' },
    });
    fireEvent.input(screen.getByPlaceholderText('https://login.example.com/realms/pulse'), {
      target: { value: 'https://idp.example.com' },
    });
    fireEvent.input(screen.getByPlaceholderText('pulse-client'), {
      target: { value: 'pulse' },
    });
    fireEvent.input(scopes, { target: { value: 'openid profile email groups' } });

    const submitButton = screen.getByRole('button', { name: 'Create Provider' });
    fireEvent.submit(submitButton.closest('form')!);

    await waitFor(() => {
      const createCall = fetchMock.mock.calls.find(
        ([url, options]) =>
          url === '/api/security/sso/providers' &&
          (options as RequestInit | undefined)?.method === 'POST',
      );
      expect(createCall).toBeTruthy();
      const body = JSON.parse((createCall![1] as RequestInit).body as string);
      expect(body.oidc.scopes).toEqual(['openid', 'profile', 'email', 'groups']);
    });
  });

  it('shows the saved scope set when reopening a provider for editing', async () => {
    setupFetch([corpProvider]);
    render(() => <SSOProvidersPanel />);

    const editButton = await screen.findByRole('button', { name: 'Edit provider' });
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(scopesInput().value).toBe('openid profile email groups');
    });
  });
});

describe('SSOProvidersPanel OIDC group role mappings', () => {
  beforeEach(() => {
    fetchMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    loggerErrorMock.mockReset();
    loggerWarnMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
    document.cookie = 'pulse_csrf=test-csrf-token';
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('preserves a multi-word group from provider details through the edit form and PUT', async () => {
    setupFetch([corpProvider], {
      ...corpProviderDetails,
      allowedGroups: ['Server Access'],
      groupRoleMappings: { 'Server Access': 'admin' },
    });
    render(() => <SSOProvidersPanel />);

    fireEvent.click(await screen.findByRole('button', { name: 'Edit provider' }));
    fireEvent.click(
      await screen.findByRole('button', { name: 'Show access restrictions & role mapping' }),
    );

    const mappings = screen.getByRole('textbox', {
      name: 'Group Role Mappings',
    }) as HTMLTextAreaElement;
    expect(mappings.value).toBe('Server Access=admin');
    fireEvent.input(mappings, { target: { value: 'Server Access=admin,\nEveryone=viewer' } });

    const saveButton = screen.getByRole('button', { name: 'Save Changes' });
    fireEvent.submit(saveButton.closest('form')!);

    await waitFor(() => {
      const saveCall = fetchMock.mock.calls.find(
        ([url, options]) =>
          url === '/api/security/sso/providers/corp-oidc' &&
          (options as RequestInit | undefined)?.method === 'PUT',
      );
      expect(saveCall).toBeTruthy();
      const body = JSON.parse((saveCall![1] as RequestInit).body as string);
      expect(body.groupRoleMappings).toEqual({ 'Server Access': 'admin', Everyone: 'viewer' });
      expect(body.allowedGroups).toEqual(['Server Access']);
      expect(body.oidc.scopes).toEqual(['openid', 'profile', 'email', 'groups']);
    });
  });
});

describe('SSOProvidersPanel IdP endpoint URLs', () => {
  beforeEach(() => {
    fetchMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    loggerErrorMock.mockReset();
    loggerWarnMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
    document.cookie = 'pulse_csrf=test-csrf-token';
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('offers the server-supplied callback URL for copying', async () => {
    setupFetch([
      { ...corpProvider, oidcCallbackUrl: 'https://pulse.example.com/api/oidc/corp-oidc/callback' },
    ]);
    render(() => <SSOProvidersPanel />);

    expect(
      await screen.findByText('https://pulse.example.com/api/oidc/corp-oidc/callback'),
    ).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Copy callback / redirect URL' })).toBeTruthy();
  });

  it.each([true, false])(
    'copies the complete server-supplied OIDC endpoint from a read-only card, enabled=%s',
    async (enabled) => {
      const callback =
        'https://monitoring.example.test/pulse/api/oidc/東京%2Foperations-with-a-long-provider-id/callback?scope=a%2Fb&state=unchanged';
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
      setupFetch([{ ...corpProvider, enabled, oidcCallbackUrl: callback }]);
      render(() => <SSOProvidersPanel canManage={false} />);

      const copy = await screen.findByRole('button', { name: 'Copy callback / redirect URL' });
      expect(copy).toBeEnabled();
      expect(copy).toHaveTextContent(callback);
      expect(screen.getByText('Callback / Redirect URL:').tagName).toBe('DT');
      fireEvent.click(copy);
      await waitFor(() => expect(writeText).toHaveBeenCalledExactlyOnceWith(callback));
      expect(notificationSuccessMock).toHaveBeenCalledWith('Callback URL has been copied');
      expect(screen.getByRole('button', { name: 'Edit provider' })).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Delete provider' })).toBeDisabled();
      expect(fetchMock.mock.calls.every(([, options]) => !options?.method)).toBe(true);
    },
  );

  it('keeps SAML metadata and ACS endpoints distinct and copyable without write access', async () => {
    const base =
      'https://monitoring.example.test/pulse/api/saml/operations-with-a-very-long-provider-id';
    const endpoints = [
      { caption: 'SP Metadata:', label: 'Copy SP metadata URL', value: `${base}/metadata` },
      { caption: 'ACS URL:', label: 'Copy ACS URL', value: `${base}/acs?target=a%2Fb` },
    ];
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    setupFetch([
      {
        id: 'corp-saml',
        name: 'Corporate SAML',
        type: 'saml',
        enabled: true,
        samlMetadataUrl: endpoints[0].value,
        samlAcsUrl: endpoints[1].value,
      },
    ]);
    render(() => <SSOProvidersPanel canManage={false} />);

    for (const endpoint of endpoints) {
      const copy = await screen.findByRole('button', { name: endpoint.label });
      expect(copy).toBeEnabled();
      expect(copy).toHaveTextContent(endpoint.value);
      expect(screen.getByText(endpoint.caption).tagName).toBe('DT');
      fireEvent.click(copy);
      await waitFor(() => expect(writeText).toHaveBeenLastCalledWith(endpoint.value));
    }
    expect(writeText).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls.every(([, options]) => !options?.method)).toBe(true);
  });

  it('reports a rejected card copy without changing its value or provider', async () => {
    const callback = 'https://monitoring.example.test/api/oidc/corp-oidc/callback';
    const writeText = vi.fn().mockRejectedValue(new Error('Clipboard unavailable'));
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    setupFetch([{ ...corpProvider, oidcCallbackUrl: callback }]);
    render(() => <SSOProvidersPanel />);

    const copy = await screen.findByRole('button', { name: 'Copy callback / redirect URL' });
    fireEvent.click(copy);
    await waitFor(() => expect(notificationErrorMock).toHaveBeenCalledWith('Unable to copy callback url'));
    expect(copy).toHaveTextContent(callback);
    expect(copy).toBeEnabled();
    expect(writeText).toHaveBeenCalledExactlyOnceWith(callback);
    expect(fetchMock.mock.calls.every(([, options]) => !options?.method)).toBe(true);
  });

  it('guides the admin to set a public URL instead of offering a copyable OIDC URL', async () => {
    setupFetch([{ ...corpProvider, oidcCallbackUrl: '' }]);
    render(() => <SSOProvidersPanel />);

    expect(
      await screen.findByText(
        'Set the public URL in Settings > System to generate the correct Callback / Redirect URL.',
      ),
    ).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Copy callback / redirect URL' })).toBeNull();
  });

  it('guides the admin to set a public URL instead of offering copyable SAML URLs', async () => {
    setupFetch([
      {
        id: 'corp-saml',
        name: 'Corporate SAML',
        type: 'saml' as const,
        enabled: true,
        priority: 0,
        samlMetadataUrl: '',
        samlAcsUrl: '',
      },
    ]);
    render(() => <SSOProvidersPanel />);

    expect(
      await screen.findByText(
        'Set the public URL in Settings > System to generate the correct SP metadata and ACS URLs.',
      ),
    ).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Copy SP metadata URL' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Copy ACS URL' })).toBeNull();
  });

  it('tells a new OIDC provider where the callback URL appears after saving', async () => {
    setupFetch([]);
    render(() => <SSOProvidersPanel />);

    const addButton = await screen.findByRole('button', { name: /Add OIDC/i });
    fireEvent.click(addButton);

    expect(
      screen.getByText(
        'The Callback / Redirect URL is generated when the provider is saved. Save this provider, then copy it from the provider card.',
      ),
    ).toBeTruthy();
  });
});

describe('SSOProvidersPanel modal feedback and recovery', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.stubGlobal('fetch', fetchMock);
    document.cookie = 'pulse_csrf=test-csrf-token';
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  const editProvider = async () => {
    render(() => <SSOProvidersPanel />);
    fireEvent.click(await screen.findByRole('button', { name: 'Edit provider' }));
    return within(await screen.findByRole('dialog', { name: 'Edit OIDC provider' }));
  };

  it('keeps a refused save and the unchanged draft inside the active dialog until an explicit correction', async () => {
    setupFetch([corpProvider]);
    const read = fetchMock.getMockImplementation()!;
    let refuse = true;
    fetchMock.mockImplementation((url: string, options?: RequestInit) => {
      if (options?.method === 'PUT') {
        return Promise.resolve(
          refuse ? new Response('Invalid role mapping', { status: 400 }) : jsonResponse({}),
        );
      }
      return read(url, options);
    });
    const dialog = await editProvider();
    const name = dialog.getByLabelText('Provider Name') as HTMLInputElement;
    const secret = dialog.getByLabelText('Client Secret') as HTMLInputElement;
    fireEvent.input(name, { target: { value: 'Edited provider' } });
    const save = dialog.getByRole('button', { name: 'Save Changes' });
    fireEvent.submit(save.closest('form')!);

    expect(await dialog.findByRole('alert')).toHaveTextContent('Invalid role mapping');
    expect(name.value).toBe('Edited provider');
    expect(secret.value).toBe('');
    expect(notificationErrorMock).not.toHaveBeenCalled();
    expect(notificationSuccessMock).not.toHaveBeenCalled();
    expect(fetchMock.mock.calls.filter(([, options]) => options?.method === 'PUT')).toHaveLength(1);

    refuse = false;
    fireEvent.input(name, { target: { value: 'Corrected provider' } });
    fireEvent.submit(save.closest('form')!);
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    const writes = fetchMock.mock.calls.filter(([, options]) => options?.method === 'PUT');
    expect(writes).toHaveLength(2);
    const payload = JSON.parse(writes[1][1].body);
    expect(payload.name).toBe('Corrected provider');
    expect(payload.oidc.clientSecret).toBeUndefined();
    expect(payload.allowedGroups).toEqual(['admins']);
    expect(payload.groupRoleMappings).toEqual({ admins: 'admin' });
    expect(notificationSuccessMock).toHaveBeenCalledOnce();
  });

  it('ignores repeated submit and keeps the modal open while a save is pending', async () => {
    setupFetch([corpProvider]);
    const read = fetchMock.getMockImplementation()!;
    let finish!: (response: Response) => void;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      options?.method === 'PUT'
        ? new Promise<Response>((resolve) => {
            finish = resolve;
          })
        : read(url, options),
    );
    const dialog = await editProvider();
    const form = dialog.getByRole('button', { name: 'Save Changes' }).closest('form')!;
    fireEvent.submit(form);
    await waitFor(() =>
      expect(fetchMock.mock.calls.filter(([, options]) => options?.method === 'PUT')).toHaveLength(
        1,
      ),
    );
    fireEvent.submit(form);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.getByRole('dialog')).toBeTruthy();
    expect(fetchMock.mock.calls.filter(([, options]) => options?.method === 'PUT')).toHaveLength(1);
    expect(dialog.getByRole('button', { name: 'Cancel' })).toBeDisabled();
    finish(jsonResponse({}));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('clears failed-save feedback when a different editor session opens', async () => {
    setupFetch([corpProvider]);
    const read = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      options?.method === 'PUT'
        ? Promise.resolve(new Response('Refused', { status: 400 }))
        : read(url, options),
    );
    const dialog = await editProvider();
    fireEvent.submit(dialog.getByRole('button', { name: 'Save Changes' }).closest('form')!);
    expect(await dialog.findByRole('alert')).toHaveTextContent('Refused');
    fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
    fireEvent.click(screen.getByRole('button', { name: 'Add OIDC' }));
    expect(within(screen.getByRole('dialog')).queryByRole('alert')).toBeNull();
  });

  it.each([false, true])(
    'reports clipboard completion inside the modal, rejected=%s',
    async (rejected) => {
      const callback = 'https://pulse.example.com/api/oidc/corp-oidc/callback';
      setupFetch([{ ...corpProvider, oidcCallbackUrl: callback }]);
      const writeText = vi
        .fn()
        .mockImplementation(() =>
          rejected ? Promise.reject(new Error('Denied')) : Promise.resolve(),
        );
      Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
      const dialog = await editProvider();
      fireEvent.click(dialog.getByRole('button', { name: 'Copy callback / redirect URL' }));
      expect(await dialog.findByRole(rejected ? 'alert' : 'status')).toHaveTextContent(
        rejected ? 'Unable to copy callback url' : 'Callback URL has been copied',
      );
      expect(writeText).toHaveBeenCalledWith(callback);
      expect(notificationErrorMock).not.toHaveBeenCalled();
      expect(notificationSuccessMock).not.toHaveBeenCalled();
    },
  );

  it('keeps SAML preview failures inside the modal without an inert toast', async () => {
    setupFetch([]);
    const read = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/metadata/preview')
        ? Promise.resolve(new Response('Metadata unavailable', { status: 502 }))
        : read(url, options),
    );
    render(() => <SSOProvidersPanel />);
    fireEvent.click(await screen.findByRole('button', { name: 'Add SAML' }));
    const dialog = within(screen.getByRole('dialog'));
    fireEvent.input(dialog.getByLabelText('IdP Metadata URL'), {
      target: { value: 'https://idp.example.com/metadata' },
    });
    fireEvent.click(dialog.getByRole('button', { name: 'Preview' }));
    expect(await dialog.findByRole('alert')).toHaveTextContent(
      'Unable to preview SAML metadata. Check the connection and metadata URL, then try again.',
    );
    expect(dialog.getByRole('alert')).not.toHaveTextContent('Metadata unavailable');
    expect(notificationErrorMock).not.toHaveBeenCalled();
  });

  it('announces connection-test failure in the existing result without a background toast', async () => {
    setupFetch([corpProvider]);
    const read = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((url: string, options?: RequestInit) =>
      url.endsWith('/test')
        ? Promise.resolve(jsonResponse({ success: false, message: 'Synthetic IdP unavailable' }))
        : read(url, options),
    );
    const dialog = await editProvider();
    fireEvent.click(dialog.getByRole('button', { name: 'Test' }));
    expect(await dialog.findByRole('alert')).toHaveTextContent('Synthetic IdP unavailable');
    expect(notificationErrorMock).not.toHaveBeenCalled();
    fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
    fireEvent.click(screen.getByRole('button', { name: 'Edit provider' }));
    expect((await screen.findByRole('dialog')).querySelector('[role="alert"]')).toBeNull();
  });
});
