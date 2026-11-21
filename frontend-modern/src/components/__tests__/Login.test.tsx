import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Login, SIGN_IN_CHECK_SLOW_DELAY_MS } from '@/components/Login';
// Compile the real lazy destination before timed assertions; do not mock setup
// or make module transformation part of the read-recovery deadline.
import '@/components/SetupWizard';
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
    expect(await screen.findByLabelText('Username')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
    // Older browser clients still find these inputs by placeholder.
    expect(screen.getByPlaceholderText('Username')).toBe(screen.getByLabelText('Username'));
    expect(screen.getByPlaceholderText('Password')).toBe(screen.getByLabelText('Password'));
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
    expect(screen.queryByLabelText('Username')).toBeNull();
    expect(screen.queryByLabelText('Password')).toBeNull();
    expect(screen.getByRole('link', { name: 'Can’t sign in?' })).toHaveAttribute(
      'href',
      '/docs/TROUBLESHOOTING',
    );
    expect(screen.queryByRole('heading', { name: 'No sign-in options on this page' })).toBeNull();
  });

  it('explains a discovered hidden form without guessing a provider or offering setup', async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ hasAuthentication: true, hideLocalLogin: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    const onLogin = vi.fn();
    render(() => <Login onLogin={onLogin} />);

    expect(
      await screen.findByRole('heading', { name: 'No sign-in options on this page' }),
    ).toBeVisible();
    expect(screen.getByText(/Ask your Pulse administrator how to sign in/)).toBeVisible();
    expect(screen.queryByLabelText('Username')).toBeNull();
    expect(screen.queryByRole('button')).toBeNull();
    expect(screen.getByRole('link', { name: 'Can’t sign in?' })).toHaveAttribute(
      'href',
      '/docs/TROUBLESHOOTING',
    );
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch.mock.calls[0][0]).toBe('/api/security/status');
    expect(onLogin).not.toHaveBeenCalled();
  });

  describe('single sign-on return with listed providers', () => {
    afterEach(() => window.history.pushState(null, '', '/'));

    it.each([
      ['oidc', 'audience_mismatch', 'Check the client ID against your provider application.', true],
      ['saml', 'invalid_state', 'The sign-in attempt expired. Please try again.', false],
    ])(
      'shows the %s return reason before any retry choice (%s)',
      async (protocol, code, reason, hideLocalLogin) => {
        const params = new URLSearchParams({ [protocol]: 'error', [`${protocol}_error`]: code });
        window.history.pushState(null, '', `/login?${params}`);
        const onLogin = vi.fn();
        render(() => (
          <Login
            onLogin={onLogin}
            securityStatus={{
              hasAuthentication: true,
              requiresAuth: true,
              hideLocalLogin,
              ssoProviders: [
                {
                  id: 'workforce',
                  name: 'Workforce',
                  displayName: 'Workforce',
                  type: 'oidc',
                  loginUrl: '/api/oidc/workforce/login',
                },
                {
                  id: 'operations',
                  name: 'Operations',
                  displayName: 'Operations',
                  type: 'saml',
                  loginUrl: '/api/saml/operations/login',
                },
              ],
            }}
          />
        ));

        const alert = await screen.findByRole('alert');
        expect(alert).toHaveTextContent(reason);
        const providers = screen.getAllByRole('button', { name: /^Continue with / });
        expect(providers.map((button) => button.textContent?.trim())).toEqual([
          'Continue with Workforce',
          'Continue with Operations',
        ]);
        for (const provider of providers) {
          expect(
            alert.compareDocumentPosition(provider) & Node.DOCUMENT_POSITION_FOLLOWING,
          ).toBeTruthy();
          expect(provider).toBeEnabled();
        }
        const localSubmit = screen.queryByRole('button', { name: 'Sign in to Pulse' });
        if (hideLocalLogin) {
          expect(localSubmit).not.toBeInTheDocument();
        } else {
          expect(localSubmit).toBeVisible();
        }
        expect(screen.getByRole('link', { name: 'Can’t sign in?' })).toBeVisible();
        expect(mockFetch).not.toHaveBeenCalled();
        expect(onLogin).not.toHaveBeenCalled();
      },
    );
  });

  describe('single sign-on return without listed providers', () => {
    afterEach(() => window.history.pushState(null, '', '/'));

    it.each([
      ['oidc', 'provider_not_found', 'SSO provider not found or not configured.', true],
      ['saml', 'invalid_state', 'The sign-in attempt expired. Please try again.', false],
      [
        'oidc',
        'unrecognized callback detail',
        'Single sign-on failed. Please try again or contact an administrator.',
        true,
      ],
    ])(
      'shows the %s error (%s) and preserves the available options',
      async (protocol, code, message, hideLocalLogin) => {
        const params = new URLSearchParams({ [protocol]: 'error', [`${protocol}_error`]: code });
        window.history.pushState(null, '', `/?${params}`);
        const onLogin = vi.fn();
        render(() => (
          <Login
            onLogin={onLogin}
            securityStatus={{
              hasAuthentication: true,
              requiresAuth: true,
              hideLocalLogin,
              ssoProviders: [],
            }}
          />
        ));

        expect(await screen.findByRole('alert')).toHaveTextContent(message);
        expect(screen.queryByText(code, { exact: false })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /continue with/i })).not.toBeInTheDocument();
        const localSubmit = screen.queryByRole('button', { name: 'Sign in to Pulse' });
        const noOptions = screen.queryByRole('heading', {
          name: 'No sign-in options on this page',
        });
        if (hideLocalLogin) {
          expect(localSubmit).not.toBeInTheDocument();
          expect(noOptions).toBeVisible();
        } else {
          expect(localSubmit).toBeVisible();
          expect(noOptions).not.toBeInTheDocument();
        }
        expect(screen.getByRole('link', { name: 'Can’t sign in?' })).toBeVisible();
        expect(mockFetch).not.toHaveBeenCalled();
        expect(onLogin).not.toHaveBeenCalled();
      },
    );

    it('does not report a successful sign-in on an unauthenticated page with no providers', () => {
      window.history.pushState(null, '', '/?oidc=success');
      const onLogin = vi.fn();
      render(() => (
        <Login
          onLogin={onLogin}
          securityStatus={{ hasAuthentication: true, requiresAuth: true, hideLocalLogin: true }}
        />
      ));

      expect(screen.queryByText(/signed in successfully/i)).not.toBeInTheDocument();
      expect(
        screen.getByRole('heading', { name: 'No sign-in options on this page' }),
      ).toBeVisible();
      expect(onLogin).not.toHaveBeenCalled();
    });

    it.each([
      ['oidc', 'group_restricted', 'Your account is not part of an authorized group to use Pulse.'],
      [
        'saml',
        'fixture-private-detail',
        'Single sign-on failed. Please try again or contact an administrator.',
      ],
    ])(
      'keeps a returned %s error visible while options are unknown and after retry',
      async (protocol, code, message) => {
        const url = new URLSearchParams({
          [protocol]: 'error',
          [`${protocol}_error`]: code,
          view: 'fixture',
        });
        window.history.pushState(null, '', `/login?${url}`);
        let finish!: (response: Response) => void;
        let recover!: (response: Response) => void;
        mockFetch
          .mockReturnValueOnce(
            new Promise<Response>((resolve) => {
              finish = resolve;
            }),
          )
          .mockReturnValueOnce(
            new Promise<Response>((resolve) => {
              recover = resolve;
            }),
          );
        const onLogin = vi.fn();
        render(() => <Login onLogin={onLogin} />);

        const alert = screen.getByRole('alert');
        const expectUnknownOptions = () => {
          expect(screen.getByRole('alert')).toBe(alert);
          expect(alert).toHaveTextContent('Last single sign-on attempt');
          expect(alert).toHaveTextContent(message);
          expect(alert).not.toHaveTextContent(code);
          expect(
            screen.queryByRole('heading', { name: 'No sign-in options on this page' }),
          ).toBeNull();
          expect(screen.queryByRole('button', { name: 'Sign in to Pulse' })).toBeNull();
          expect(screen.queryByRole('main', { name: 'Pulse Setup Wizard' })).toBeNull();
        };
        expectUnknownOptions();
        expect(screen.getByRole('status')).toHaveTextContent('Checking which sign-in options');
        finish(new Response('fixture-private-detail', { status: 503 }));

        const retry = await screen.findByRole('button', { name: 'Retry sign-in check' });
        expectUnknownOptions();
        expect(screen.getByRole('status')).toHaveTextContent(
          'does not mean authentication is unconfigured',
        );
        retry.focus();
        fireEvent.click(retry);
        expect(retry).toHaveFocus();
        expect(retry).toHaveAttribute('aria-disabled', 'true');
        expectUnknownOptions();

        recover(
          new Response(JSON.stringify({ hasAuthentication: true, hideLocalLogin: true }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          }),
        );
        expect(
          await screen.findByRole('heading', { name: 'No sign-in options on this page' }),
        ).toBeVisible();
        expect(screen.getByRole('alert')).toHaveTextContent(message);
        expect(screen.getByRole('alert')).not.toHaveTextContent('Last single sign-on attempt');
        expect(window.history.replaceState).toHaveBeenLastCalledWith(
          {},
          expect.any(String),
          '/login?view=fixture',
        );
        expect(mockFetch.mock.calls.map(([request]) => request)).toEqual([
          '/api/security/status',
          '/api/security/status',
        ]);
        expect(onLogin).not.toHaveBeenCalled();
      },
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
    expect(await screen.findByLabelText('Username')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
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
    expect(await screen.findByLabelText('Username')).toBeInTheDocument();

    // The fetch should not have been called for /api/security/status
    // because we passed securityStatus directly
    expect(mockFetch).not.toHaveBeenCalledWith('/api/security/status');
  });

  it.each([false, true])('shows demo credentials after sign-out (hidden=%s)', async (hidden) => {
    // A sign-out marks the tab so the page does not sign the visitor straight
    // back in; the printed credentials are the way back.
    window.sessionStorage.setItem(SESSION_STORAGE_KEYS.DEMO_AUTO_LOGIN, 'suppressed');
    const mockOnLogin = vi.fn();
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: hidden,
      sessionCapabilities: { demoMode: true },
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    expect(await screen.findByText('Demo Mode')).toBeInTheDocument();
    expect(screen.getAllByText('demo')).toHaveLength(2);
    expect(screen.queryByText('Signing you in to the demo…')).toBeNull();
    if (hidden) {
      expect(screen.getByRole('status')).toBeEmptyDOMElement();
      expect(screen.getByText(/The local sign-in form is hidden/)).toBeVisible();
      expect(screen.queryByLabelText('Username')).toBeNull();
    }
    expect(mockFetch).not.toHaveBeenCalledWith('/api/login', expect.anything());
    expect(mockOnLogin).not.toHaveBeenCalled();
  });

  it.each([false, true])('signs in to the demo (hidden=%s)', async (hidden) => {
    const mockOnLogin = vi.fn();
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ success: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: hidden,
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

  it('keeps hidden-form demo progress and refusal in one live region without exposing the reply', async () => {
    let finish!: (response: Response) => void;
    mockFetch.mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        finish = resolve;
      }),
    );
    const onLogin = vi.fn();
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: true,
      presentationPolicy: { demoMode: true },
    };
    render(() => <Login onLogin={onLogin} securityStatus={securityStatus as any} />);
    const status = screen.getByRole('status');
    const help = screen.getByRole('link', { name: 'Can’t sign in?' });
    help.focus();

    expect(screen.getByText('Demo Mode')).toBeVisible();
    expect(status).toHaveTextContent('Signing you in to the demo…');
    expect(screen.queryByRole('heading', { name: 'No sign-in options on this page' })).toBeNull();
    expect(screen.queryByText(/^demo$/)).toBeNull();

    finish(
      new Response(JSON.stringify({ success: false, message: 'Synthetic provider detail' }), {
        status: 401,
      }),
    );

    await waitFor(() =>
      expect(status).toHaveTextContent('Automatic demo sign-in didn’t complete.'),
    );
    expect(screen.getByRole('status')).toBe(status);
    expect(screen.getByText(/The local sign-in form is hidden/)).toBeVisible();
    expect(screen.getAllByText('demo')).toHaveLength(2);
    expect(screen.queryByText('Synthetic provider detail')).toBeNull();
    expect(screen.queryByLabelText('Username')).toBeNull();
    expect(help).toHaveFocus();
    expect(mockFetch.mock.calls.filter(([url]) => url === '/api/login')).toHaveLength(1);
    expect(onLogin).not.toHaveBeenCalled();
  });

  it('does not restart demo sign-in when pending settles and the tab marker cannot be saved', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementationOnce(() => {
      throw new Error('storage unavailable');
    });
    const securityStatus = {
      hasAuthentication: true,
      presentationPolicy: { demoMode: true },
    };
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ success: false, message: 'Invalid username or password' }), {
        status: 401,
      }),
    );

    try {
      render(() => <Login onLogin={vi.fn()} securityStatus={securityStatus as any} />);

      expect(await screen.findByRole('alert')).toHaveTextContent('Invalid username or password');
      expect(screen.getByRole('button', { name: 'Sign in to Pulse' })).toHaveAttribute(
        'aria-disabled',
        'false',
      );
      expect(mockFetch.mock.calls.filter(([url]) => url === '/api/login')).toHaveLength(1);
    } finally {
      setItem.mockRestore();
    }
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

    const usernameInput = (await screen.findByLabelText('Username')) as HTMLInputElement;
    const passwordInput = screen.getByLabelText('Password') as HTMLInputElement;
    expect(usernameInput.value).toBe('johannes');
    expect(screen.getByLabelText('Remember me')).toBeChecked();
    expect(passwordInput.value).toBe('');
  });

  it('does not hide the field labels when the browser restores filled inputs', async () => {
    render(() => (
      <Login onLogin={vi.fn()} securityStatus={{ hasAuthentication: true, requiresAuth: true }} />
    ));
    const username = await screen.findByLabelText<HTMLInputElement>('Username');
    const password = screen.getByLabelText<HTMLInputElement>('Password');
    // Restore without input events, like a password manager.
    username.value = 'fixture-operator';
    password.value = 'synthetic-input';

    // JSDOM does not load Tailwind; the browser comparison checks actual geometry.
    for (const input of [username, password]) {
      expect(input.labels).toHaveLength(1);
      expect(input.labels![0]).not.toHaveClass('sr-only');
    }
    expect(password).toHaveAttribute('type', 'password');
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

    fireEvent.input(await screen.findByLabelText('Username'), {
      target: { value: 'johannes' },
    });
    fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'secret' } });
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

    fireEvent.input(await screen.findByLabelText('Username'), {
      target: { value: 'johannes' },
    });
    fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'secret' } });
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
    fireEvent.input(screen.getByLabelText('Username'), {
      target: { value: 'new-user' },
    });
    fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: /sign in to pulse/i }));

    await waitFor(() => expect(mockOnLogin).toHaveBeenCalledOnce());
    expect(mockLocalStorage.removeItem).toHaveBeenCalledWith(
      STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME,
    );
  });

  it('keeps the focused submit in place and ignores extra activations until a refusal settles', async () => {
    const onLogin = vi.fn();
    const securityStatus = { hasAuthentication: true, hideLocalLogin: false };
    let finish!: (response: Response) => void;
    mockFetch.mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        finish = resolve;
      }),
    );
    render(() => <Login onLogin={onLogin} securityStatus={securityStatus as any} />);
    const submit = screen.getByRole<HTMLButtonElement>('button', { name: 'Sign in to Pulse' });
    const form = submit.form!;
    const username = screen.getByLabelText('Username');
    const password = screen.getByLabelText('Password');
    fireEvent.input(username, { target: { value: 'fixture-operator' } });
    fireEvent.input(password, { target: { value: 'synthetic-input' } });
    fireEvent.click(screen.getByLabelText('Remember me'));
    submit.focus();
    fireEvent.click(submit);

    expect(submit).toHaveFocus();
    expect(submit).not.toBeDisabled(); // Native disabled would drop browser focus.
    expect(submit).toHaveAttribute('aria-disabled', 'true');
    const status = screen.getByRole('status');
    expect(status).toHaveTextContent('Sign-in in progress.');
    fireEvent.click(submit);
    fireEvent.submit(form);
    await waitFor(() =>
      expect(mockFetch.mock.calls.filter(([url]) => url === '/api/login')).toHaveLength(1),
    );

    finish(
      new Response(
        JSON.stringify({ success: false, message: 'Invalid username or password.', remaining: 2 }),
        { status: 401 },
      ),
    );
    expect(await screen.findByRole('alert')).toHaveTextContent('2 attempts remaining');
    expect(submit).toHaveFocus();
    expect(submit).toHaveAttribute('aria-disabled', 'false');
    expect(screen.getByRole('status')).toBe(status);
    expect(status.textContent).toBe('');
    expect(username).toHaveValue('fixture-operator');
    expect(password).toHaveValue('');
    expect(onLogin).not.toHaveBeenCalled();

    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ success: true }), { status: 200 }),
    );
    fireEvent.input(password, { target: { value: 'corrected-fixture-input' } });
    fireEvent.submit(form);
    await waitFor(() => expect(onLogin).toHaveBeenCalledOnce());
    expect(mockFetch.mock.calls.filter(([url]) => url === '/api/login')).toHaveLength(2);
  });

  describe('authentication discovery recovery', () => {
    const discoveryResponse = (data: unknown) =>
      new Response(JSON.stringify(data), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    const expectNoGuessedAccess = () => {
      expect(screen.queryByRole('main', { name: 'Pulse Setup Wizard' })).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
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
      expect(await screen.findByLabelText('Username')).toBeVisible();
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });

    it.each(['late setup response', 'late failure'] as const)(
      'lets the user replace a slow first check and ignores its %s',
      async (lateResult) => {
        let resolveOld!: (response: Response) => void;
        let rejectOld!: (reason: Error) => void;
        let finish!: (response: Response) => void;
        mockFetch
          .mockReturnValueOnce(
            new Promise<Response>((resolve, reject) => {
              resolveOld = resolve;
              rejectOld = reject;
            }),
          )
          .mockReturnValueOnce(
            new Promise<Response>((resolve) => {
              finish = resolve;
            }),
          );
        const onLogin = vi.fn();
        vi.useFakeTimers();
        try {
          render(() => <Login onLogin={onLogin} />);
          await vi.advanceTimersByTimeAsync(SIGN_IN_CHECK_SLOW_DELAY_MS - 1);
          expect(screen.queryByRole('button', { name: 'Restart sign-in check' })).toBeNull();
          expectNoGuessedAccess();
          await vi.advanceTimersByTimeAsync(1);
          const restart = screen.getByRole('button', { name: 'Restart sign-in check' });
          const firstSignal = mockFetch.mock.calls[0][1].signal as AbortSignal;
          expect(mockFetch).toHaveBeenCalledTimes(1);
          expect(firstSignal.aborted).toBe(false);

          restart.focus();
          fireEvent.click(restart);
          fireEvent.click(restart);
          expect(mockFetch).toHaveBeenCalledTimes(2);
          expect(firstSignal.aborted).toBe(true);
          expect(mockFetch.mock.calls[1][1].signal.aborted).toBe(false);
          expect(restart).toHaveFocus();
          expect(restart).toHaveAttribute('aria-disabled', 'true');

          if (lateResult === 'late setup response') {
            resolveOld(discoveryResponse({ hasAuthentication: false }));
          } else {
            rejectOld(new Error('Discarded fixture read'));
          }
          await vi.advanceTimersByTimeAsync(0);
          expect(screen.getByRole('status')).toHaveTextContent('Checking authentication again');
          expect(screen.queryByText('Could not check sign-in options')).toBeNull();
          expectNoGuessedAccess();

          // The second read remains owned and can itself surface recovery.
          await vi.advanceTimersByTimeAsync(SIGN_IN_CHECK_SLOW_DELAY_MS);
          expect(restart).toHaveAttribute('aria-disabled', 'false');
          expect(screen.getByRole('button', { name: 'Restart sign-in check' })).toBe(restart);
          finish(discoveryResponse({ hasAuthentication: true }));
          await vi.advanceTimersByTimeAsync(0);
          expect(screen.getByLabelText('Username')).toBeVisible();
          expect(screen.getByRole('region', { name: 'Pulse access' })).toHaveFocus();
          expect(mockFetch).toHaveBeenCalledTimes(2);
          expect(onLogin).not.toHaveBeenCalled();
        } finally {
          vi.useRealTimers();
        }
      },
    );

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
      expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
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
      expect(await screen.findByLabelText('Username')).toBeVisible();
      expect(independent).toHaveFocus();
    });

    it('keeps the troubleshooting link focused if recovery completes while the user is on Help', async () => {
      mockFetch.mockResolvedValueOnce(new Response('{}', { status: 503 }));
      let finish!: (response: Response) => void;
      mockFetch.mockReturnValueOnce(
        new Promise<Response>((resolve) => {
          finish = resolve;
        }),
      );
      render(() => <Login onLogin={vi.fn()} />);
      const retry = await screen.findByRole('button', { name: 'Retry sign-in check' });
      retry.focus();
      fireEvent.click(retry);
      const help = screen.getByRole('link', { name: 'Can’t sign in?' });
      help.focus();

      finish(discoveryResponse({ hasAuthentication: true }));
      await waitFor(() => {
        const currentHelp = screen.getByRole('link', { name: 'Can’t sign in?' });
        expect(currentHelp).not.toBe(help);
        expect(currentHelp).toHaveFocus();
        expect(currentHelp).toHaveAttribute('href', '/docs/TROUBLESHOOTING');
      });
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
      expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });
  });
});
