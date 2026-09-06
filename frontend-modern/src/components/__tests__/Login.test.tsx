import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Login } from '@/components/Login';
import loginSource from '@/components/Login.tsx?raw';
import { SESSION_STORAGE_KEYS, STORAGE_KEYS } from '@/utils/localStorage';

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

// Mock localStorage
let mockLocalStorageData: Record<string, string>;
const mockLocalStorage = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
};
Object.defineProperty(window, 'localStorage', { value: mockLocalStorage });

// Mock history.replaceState
window.history.replaceState = vi.fn();

describe('Login', () => {
  beforeEach(() => {
    mockLocalStorageData = {};
    mockFetch.mockReset();
    mockLocalStorage.getItem.mockReset();
    mockLocalStorage.getItem.mockImplementation((key: string) => mockLocalStorageData[key] ?? null);
    mockLocalStorage.setItem.mockReset();
    mockLocalStorage.setItem.mockImplementation((key: string, value: string) => {
      mockLocalStorageData[key] = value;
    });
    mockLocalStorage.removeItem.mockReset();
    mockLocalStorage.removeItem.mockImplementation((key: string) => {
      delete mockLocalStorageData[key];
    });
    mockLocalStorage.clear.mockReset();
    mockLocalStorage.clear.mockImplementation(() => {
      mockLocalStorageData = {};
    });
    window.sessionStorage.clear();
    // Default mock for security status
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ hasAuthentication: true }),
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('shows local login form when hideLocalLogin is false', async () => {
    const mockOnLogin = vi.fn();
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: false,
      ssoEnabled: false,
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    // Username and password fields should be visible
    expect(await screen.findByPlaceholderText('Username')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Password')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in to pulse/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Can’t sign in?' })).toHaveAttribute(
      'href',
      '/docs/TROUBLESHOOTING',
    );
  });

  it('hides local login form when hideLocalLogin is true and an SSO provider is available', async () => {
    const mockOnLogin = vi.fn();
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: true,
      ssoProviders: [
        {
          id: 'legacy-oidc',
          name: 'Single Sign-On',
          type: 'oidc',
          displayName: 'Single Sign-On',
          loginUrl: '/api/oidc/legacy-oidc/login',
        },
      ],
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    // Wait for the component to render
    expect(await screen.findByText(/Welcome to Pulse/i)).toBeInTheDocument();

    // SSO button should be visible
    expect(
      screen.getByRole('button', { name: /continue with single sign-on/i }),
    ).toBeInTheDocument();

    // Username and password fields should NOT be visible
    expect(screen.queryByPlaceholderText('Username')).toBeNull();
    expect(screen.queryByPlaceholderText('Password')).toBeNull();
    expect(screen.getByRole('link', { name: 'Can’t sign in?' })).toHaveAttribute(
      'href',
      '/docs/TROUBLESHOOTING',
    );
  });

  it('shows local login form when show_local=true is in URL even if hideLocalLogin is true', async () => {
    // Set up URL with show_local=true
    const originalLocation = window.location;
    delete (window as any).location;
    window.location = {
      ...originalLocation,
      search: '?show_local=true',
      pathname: '/',
      href: 'http://localhost/?show_local=true',
    } as any;

    const mockOnLogin = vi.fn();
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: true,
      ssoProviders: [
        {
          id: 'legacy-oidc',
          name: 'Single Sign-On',
          type: 'oidc',
          displayName: 'Single Sign-On',
          loginUrl: '/api/oidc/legacy-oidc/login',
        },
      ],
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    // Local login form should still be visible due to show_local=true
    expect(await screen.findByPlaceholderText('Username')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Password')).toBeInTheDocument();
    // Restore location
    (window as any).location = originalLocation;
  });

  it('uses securityStatus prop instead of making API call when provided', async () => {
    const mockOnLogin = vi.fn();
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: false,
      ssoEnabled: false,
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    // Wait for component to render
    expect(await screen.findByPlaceholderText('Username')).toBeInTheDocument();

    // The fetch should not have been called for /api/security/status
    // because we passed securityStatus directly
    expect(mockFetch).not.toHaveBeenCalledWith('/api/security/status');
  });

  it('shows demo credentials when the visitor has signed out of the demo', async () => {
    // A sign-out marks the tab so the page does not sign the visitor straight
    // back in; the printed credentials are the way back.
    window.sessionStorage.setItem(SESSION_STORAGE_KEYS.DEMO_AUTO_LOGIN, 'suppressed');
    const mockOnLogin = vi.fn();
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: false,
      sessionCapabilities: { demoMode: true },
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    expect(await screen.findByText('Demo Mode')).toBeInTheDocument();
    expect(screen.getAllByText('demo')).toHaveLength(2);
    expect(mockFetch).not.toHaveBeenCalledWith('/api/login', expect.anything());
    expect(mockOnLogin).not.toHaveBeenCalled();
  });

  it('signs the visitor in with the demo credentials when the runtime is in demo mode', async () => {
    const mockOnLogin = vi.fn();
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ success: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: false,
      presentationPolicy: { demoMode: true },
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    await waitFor(() => expect(mockOnLogin).toHaveBeenCalledOnce());
    const loginCall = mockFetch.mock.calls.find(([url]) => url === '/api/login');
    expect(loginCall).toBeDefined();
    expect(JSON.parse((loginCall?.[1] as RequestInit).body as string)).toEqual({
      username: 'demo',
      password: 'demo',
      rememberMe: false,
    });
    expect(window.sessionStorage.getItem(SESSION_STORAGE_KEYS.DEMO_AUTO_LOGIN)).toBe('attempted');
  });

  it('falls back to the form when the demo sign-in is rejected', async () => {
    const mockOnLogin = vi.fn();
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ success: false, message: 'Invalid username or password' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: false,
      presentationPolicy: { demoMode: true },
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    expect(await screen.findByText('Invalid username or password')).toBeInTheDocument();
    expect(screen.getAllByText('demo')).toHaveLength(2);
    expect(screen.getByRole('button', { name: /sign in to pulse/i })).toBeEnabled();
    expect(mockOnLogin).not.toHaveBeenCalled();
  });

  it('does not sign in to the demo twice in one browser tab', async () => {
    window.sessionStorage.setItem(SESSION_STORAGE_KEYS.DEMO_AUTO_LOGIN, 'attempted');
    const mockOnLogin = vi.fn();
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: false,
      presentationPolicy: { demoMode: true },
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    expect(await screen.findByText('Demo Mode')).toBeInTheDocument();
    expect(mockFetch).not.toHaveBeenCalledWith('/api/login', expect.anything());
  });

  it('restores the remembered username without storing a password', async () => {
    mockLocalStorageData[STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME] = 'johannes';
    const mockOnLogin = vi.fn();
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: false,
      ssoEnabled: false,
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    const usernameInput = (await screen.findByPlaceholderText('Username')) as HTMLInputElement;
    expect(usernameInput.value).toBe('johannes');
    expect(screen.getByLabelText('Remember me')).toBeChecked();
    expect((screen.getByPlaceholderText('Password') as HTMLInputElement).value).toBe('');
  });

  it('stores the username after a successful remembered login', async () => {
    const mockOnLogin = vi.fn();
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ success: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: false,
      ssoEnabled: false,
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    fireEvent.input(await screen.findByPlaceholderText('Username'), {
      target: { value: 'johannes' },
    });
    fireEvent.input(screen.getByPlaceholderText('Password'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByLabelText('Remember me'));
    fireEvent.click(screen.getByRole('button', { name: /sign in to pulse/i }));

    await waitFor(() => expect(mockOnLogin).toHaveBeenCalledOnce());
    expect(window.sessionStorage.getItem(STORAGE_KEYS.AUTH_USER)).toBe('johannes');
    expect(mockLocalStorage.setItem).toHaveBeenCalledWith(
      STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME,
      'johannes',
    );
  });

  it('uses the submitted checkbox state when browser restore does not fire change', async () => {
    const mockOnLogin = vi.fn();
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ success: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: false,
      ssoEnabled: false,
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    fireEvent.input(await screen.findByPlaceholderText('Username'), {
      target: { value: 'johannes' },
    });
    fireEvent.input(screen.getByPlaceholderText('Password'), { target: { value: 'secret' } });
    (screen.getByLabelText('Remember me') as HTMLInputElement).checked = true;
    fireEvent.click(screen.getByRole('button', { name: /sign in to pulse/i }));

    await waitFor(() => expect(mockOnLogin).toHaveBeenCalledOnce());
    expect(mockLocalStorage.setItem).toHaveBeenCalledWith(
      STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME,
      'johannes',
    );
    expect(mockLocalStorage.removeItem).not.toHaveBeenCalledWith(
      STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME,
    );
  });

  it('clears a previously remembered username after a successful non-remembered login', async () => {
    mockLocalStorageData[STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME] = 'old-user';
    const mockOnLogin = vi.fn();
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ success: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: false,
      ssoEnabled: false,
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    const rememberCheckbox = await screen.findByLabelText('Remember me');
    fireEvent.click(rememberCheckbox);
    fireEvent.input(screen.getByPlaceholderText('Username'), {
      target: { value: 'new-user' },
    });
    fireEvent.input(screen.getByPlaceholderText('Password'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: /sign in to pulse/i }));

    await waitFor(() => expect(mockOnLogin).toHaveBeenCalledOnce());
    expect(mockLocalStorage.removeItem).toHaveBeenCalledWith(
      STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME,
    );
  });

  it('routes login loading indicators through the shared LoadingSpinner primitive', () => {
    expect(loginSource).toContain("from '@/components/shared/LoadingSpinner'");
    expect(loginSource).toContain('<LoadingSpinner size="lg" tone="info"');
    expect(loginSource).toContain('<LoadingSpinner size="button" tone="inverse"');
    expect(loginSource).not.toContain(
      'animate-spin h-12 w-12 border-4 border-blue-500 border-t-transparent rounded-full mx-auto mb-4',
    );
    expect(loginSource).not.toContain('class="animate-spin -ml-1 mr-3 h-5 w-5 text-white"');
  });
});
