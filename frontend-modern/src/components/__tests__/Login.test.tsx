import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { Login } from '@/components/Login';

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
    mockFetch.mockReset();
    mockLocalStorage.getItem.mockReset();
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
      oidcEnabled: false,
    };

    render(() => (
      <Login onLogin={mockOnLogin} hasAuth={true} securityStatus={securityStatus as any} />
    ));

    // Username and password fields should be visible
    expect(await screen.findByPlaceholderText('Username')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Password')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in to pulse/i })).toBeInTheDocument();
  });

  it('hides local login form when hideLocalLogin is true and OIDC is enabled', async () => {
    const mockOnLogin = vi.fn();
    const securityStatus = {
      hasAuthentication: true,
      hideLocalLogin: true,
      oidcEnabled: true,
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
      oidcEnabled: true,
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
      oidcEnabled: false,
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
});
