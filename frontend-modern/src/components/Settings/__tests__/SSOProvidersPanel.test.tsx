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

const setupFetch = (providers: unknown[]) => {
  fetchMock.mockImplementation((url: string, options?: RequestInit) => {
    const method = (options?.method ?? 'GET').toUpperCase();
    if (url === '/api/security/sso/providers' && method === 'GET') {
      return Promise.resolve(jsonResponse({ providers, allowMultipleProviders: false }));
    }
    if (url === '/api/security/status') {
      return Promise.resolve(jsonResponse({ publicUrl: 'https://pulse.example.com' }));
    }
    if (url === '/api/security/sso/providers/corp-oidc' && method === 'GET') {
      return Promise.resolve(jsonResponse(corpProviderDetails));
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
