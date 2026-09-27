import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
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
