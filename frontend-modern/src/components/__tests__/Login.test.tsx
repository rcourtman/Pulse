import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Login } from '@/components/Login';
// Compile the real lazy destination before timed assertions; do not mock setup
// or make module transformation part of the read-recovery deadline.
import '@/components/SetupWizard';
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

  describe('authentication discovery recovery', () => {
    const discoveryResponse = (data: unknown) =>
      new Response(JSON.stringify(data), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    const expectNoGuessedAccess = () => {
      expect(screen.queryByRole('main', { name: 'Pulse Setup Wizard' })).not.toBeInTheDocument();
      expect(screen.queryByPlaceholderText('Username')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /sign in to pulse/i })).not.toBeInTheDocument();
    };

    it.each([401, 403, 429, 503])(
      'keeps a failed HTTP %s read unknown and never starts setup or retries automatically',
      async (status) => {
        const onLogin = vi.fn();
        mockFetch.mockResolvedValueOnce(
          new Response(JSON.stringify({ error: 'SYNTHETIC_PRIVATE_RESPONSE' }), { status }),
        );
        render(() => <Login onLogin={onLogin} hasAuth={false} />);

        expect(
          await screen.findByRole('heading', { name: 'Could not check sign-in options' }),
        ).toBeVisible();
        expect(screen.getByRole('status')).toHaveTextContent(
          'does not mean authentication is unconfigured',
        );
        expectNoGuessedAccess();
        expect(screen.queryByText(/SYNTHETIC_PRIVATE_RESPONSE/)).not.toBeInTheDocument();
        expect(mockFetch).toHaveBeenCalledTimes(1);
        expect(mockFetch.mock.calls[0][0]).toBe('/api/security/status');
        expect(onLogin).not.toHaveBeenCalled();
        expect(screen.getByRole('link', { name: 'Can’t sign in?' })).toHaveAttribute(
          'href',
          '/docs/TROUBLESHOOTING',
        );
      },
    );

    it('keeps a network failure unknown without exposing its raw text', async () => {
      mockFetch.mockRejectedValueOnce(new Error('SYNTHETIC_PRIVATE_TRANSPORT'));
      render(() => <Login onLogin={vi.fn()} />);
      expect(await screen.findByRole('button', { name: 'Retry sign-in check' })).toBeVisible();
      expectNoGuessedAccess();
      expect(document.body).not.toHaveTextContent('SYNTHETIC_PRIVATE_TRANSPORT');
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });

    it.each([
      null,
      [],
      {},
      { hasAuthentication: null },
      { hasAuthentication: 0 },
      { hasAuthentication: 'false' },
    ])('keeps incomplete or malformed discovery %j unknown', async (data) => {
      mockFetch.mockResolvedValueOnce(discoveryResponse(data));
      render(() => <Login onLogin={vi.fn()} />);
      expect(await screen.findByRole('button', { name: 'Retry sign-in check' })).toBeVisible();
      expectNoGuessedAccess();
    });

    it('keeps invalid JSON unknown instead of entering setup', async () => {
      mockFetch.mockResolvedValueOnce(new Response('not-json', { status: 200 }));
      render(() => <Login onLogin={vi.fn()} />);
      expect(await screen.findByRole('button', { name: 'Retry sign-in check' })).toBeVisible();
      expectNoGuessedAccess();
    });

    it.each([true, false])(
      'waits for full discovery despite a hasAuth=%s bootstrap hint',
      async (hasAuth) => {
        mockFetch.mockReturnValueOnce(new Promise(() => {}));
        const view = render(() => <Login onLogin={vi.fn()} hasAuth={hasAuth} />);
        expect(screen.getByText('Checking authentication...')).toBeVisible();
        expectNoGuessedAccess();
        const signal = mockFetch.mock.calls[0][1].signal as AbortSignal;
        expect(signal.aborted).toBe(false);
        view.unmount();
        expect(signal.aborted).toBe(true);
      },
    );

    it('reads again when bootstrap supplied no explicit authentication observation', async () => {
      mockFetch.mockResolvedValueOnce(discoveryResponse({ hasAuthentication: true }));
      render(() => <Login onLogin={vi.fn()} securityStatus={{} as any} />);
      expect(await screen.findByPlaceholderText('Username')).toBeVisible();
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });

    it('retries one read, retains pending focus and restores SSO policy without local login', async () => {
      mockFetch.mockResolvedValueOnce(new Response('{}', { status: 503 }));
      let finish!: (response: Response) => void;
      mockFetch.mockReturnValueOnce(
        new Promise<Response>((resolve) => {
          finish = resolve;
        }),
      );
      const onLogin = vi.fn();
      render(() => <Login onLogin={onLogin} hasAuth={true} />);
      const retry = await screen.findByRole('button', { name: 'Retry sign-in check' });
      retry.focus();
      fireEvent.click(retry);
      fireEvent.click(retry);
      expect(screen.getByRole('button', { name: 'Retry sign-in check' })).toBe(retry);
      expect(retry).toHaveAttribute('aria-disabled', 'true');
      expect(retry).toHaveFocus();
      expect(screen.getByRole('status')).toHaveTextContent('Checking authentication again');
      expect(mockFetch).toHaveBeenCalledTimes(2);
      expectNoGuessedAccess();
      finish(
        discoveryResponse({
          hasAuthentication: true,
          hideLocalLogin: true,
          ssoProviders: [
            {
              id: 'lab',
              name: 'Lab SSO',
              displayName: 'Lab SSO',
              type: 'oidc',
              loginUrl: '/api/oidc/lab/login',
            },
          ],
        }),
      );
      expect(await screen.findByRole('button', { name: 'Continue with Lab SSO' })).toBeVisible();
      expect(screen.queryByPlaceholderText('Username')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Retry sign-in check' })).not.toBeInTheDocument();
      await waitFor(() =>
        expect(screen.getByRole('region', { name: 'Pulse access' })).toHaveFocus(),
      );
      expect(
        mockFetch.mock.calls.every(
          ([url, options]) =>
            url === '/api/security/status' && (!options.method || options.method === 'GET'),
        ),
      ).toBe(true);
      expect(onLogin).not.toHaveBeenCalled();
    });

    it('retains the same retry control and focus after another failed read', async () => {
      mockFetch.mockResolvedValueOnce(new Response('{}', { status: 503 }));
      mockFetch.mockResolvedValueOnce(new Response('{}', { status: 503 }));
      render(() => <Login onLogin={vi.fn()} />);
      const retry = await screen.findByRole('button', { name: 'Retry sign-in check' });
      retry.focus();
      fireEvent.click(retry);
      await waitFor(() => expect(retry).toHaveAttribute('aria-disabled', 'false'));
      expect(screen.getByRole('button', { name: 'Retry sign-in check' })).toBe(retry);
      expect(retry).toHaveFocus();
      expectNoGuessedAccess();
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });

    it('does not steal focus moved outside the failed-read notice while a retry is pending', async () => {
      mockFetch.mockResolvedValueOnce(new Response('{}', { status: 503 }));
      let finish!: (response: Response) => void;
      mockFetch.mockReturnValueOnce(
        new Promise<Response>((resolve) => {
          finish = resolve;
        }),
      );
      render(() => (
        <>
          <button type="button">Independent control</button>
          <Login onLogin={vi.fn()} />
        </>
      ));
      const retry = await screen.findByRole('button', { name: 'Retry sign-in check' });
      retry.focus();
      fireEvent.click(retry);
      const independent = screen.getByRole('button', { name: 'Independent control' });
      independent.focus();
      finish(discoveryResponse({ hasAuthentication: true }));
      expect(await screen.findByPlaceholderText('Username')).toBeVisible();
      expect(independent).toHaveFocus();
    });

    it('ignores late discovery after disposal and aborts its transport', async () => {
      let finish!: (response: Response) => void;
      mockFetch.mockReturnValueOnce(
        new Promise<Response>((resolve) => {
          finish = resolve;
        }),
      );
      const onLogin = vi.fn();
      const view = render(() => <Login onLogin={onLogin} />);
      const signal = mockFetch.mock.calls[0][1].signal as AbortSignal;
      view.unmount();
      expect(signal.aborted).toBe(true);
      finish(discoveryResponse({ hasAuthentication: false }));
      await Promise.resolve();
      await Promise.resolve();
      expect(screen.queryByRole('region', { name: 'Pulse access' })).not.toBeInTheDocument();
      expect(screen.queryByRole('main', { name: 'Pulse Setup Wizard' })).not.toBeInTheDocument();
      expect(onLogin).not.toHaveBeenCalled();
    });

    it('enters the existing protected setup only after retry explicitly observes unconfigured authentication', async () => {
      mockFetch.mockResolvedValueOnce(new Response('{}', { status: 503 }));
      mockFetch.mockResolvedValueOnce(discoveryResponse({ hasAuthentication: false }));
      render(() => <Login onLogin={vi.fn()} hasAuth={false} />);
      fireEvent.click(await screen.findByRole('button', { name: 'Retry sign-in check' }));
      expect(await screen.findByRole('main', { name: 'Pulse Setup Wizard' })).toBeVisible();
      expect(screen.queryByPlaceholderText('Username')).not.toBeInTheDocument();
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });
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
