import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { JSX } from 'solid-js';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';
import { getTeamSignInMethods, getTeamSignInUrl } from '@/utils/rbacPresentation';

const mocks = vi.hoisted(() => ({
  status: vi.fn(),
  users: vi.fn(),
  roles: vi.fn(),
  permissions: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
  copy: vi.fn(),
  enabled: true,
}));

vi.mock('@/api/security', () => ({ SecurityAPI: { getStatus: mocks.status } }));
vi.mock('@/api/rbac', () => ({
  RBACAPI: {
    getUsers: mocks.users,
    getRoles: mocks.roles,
    getUserPermissions: mocks.permissions,
    updateUserRoles: mocks.update,
    deleteUser: mocks.remove,
  },
}));
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    rbacEnabled: () => mocks.enabled,
    licenseReady: () => true,
    paywallVisible: () => false,
  }),
}));
vi.mock('@solidjs/router', () => ({
  A: (props: JSX.AnchorHTMLAttributes<HTMLAnchorElement>) => <a {...props} />,
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { error: vi.fn(), success: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

const configured = {
  hasAuthentication: true,
  requiresAuth: true,
  ssoEnabled: true,
  ssoProviders: [
    {
      id: 'company',
      name: 'company',
      displayName: 'Company sign-in',
      type: 'oidc',
      loginUrl: '/api/oidc/login',
    },
  ],
};

beforeEach(() => {
  vi.resetAllMocks();
  mocks.enabled = true;
  mocks.status.mockResolvedValue(configured);
  mocks.users.mockResolvedValue([]);
  mocks.roles.mockResolvedValue([
    { id: 'viewer', name: 'Viewer', permissions: [], isBuiltIn: true },
  ]);
  mocks.permissions.mockResolvedValue([]);
  mocks.copy.mockResolvedValue(undefined);
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: mocks.copy },
  });
});
afterEach(cleanup);

async function openGuide() {
  render(() => <UserAssignmentsPanel />);
  fireEvent.click(await screen.findByRole('button', { name: 'Add teammate' }));
  return screen.findByRole('dialog', { name: 'Add a teammate' });
}

describe('team sign-in journey', () => {
  it('shares the normal login entry and refreshes the unfiltered list before managing a stable identity', async () => {
    await openGuide();
    expect(await screen.findByText('Company sign-in (OIDC)')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Copy link' }));
    await waitFor(() => expect(mocks.copy).toHaveBeenCalledWith(`${window.location.origin}/login`));
    expect(await screen.findByRole('status')).toHaveTextContent('Link copied');
    fireEvent.input(screen.getByRole('textbox', { name: 'Search users...' }), {
      target: { value: 'old search' },
    });
    mocks.users.mockResolvedValue([
      { username: 'sso:oidc:company:stable-subject', displayName: 'New colleague', roleIds: [] },
    ]);
    fireEvent.click(screen.getByRole('button', { name: 'Refresh user list' }));
    expect(await screen.findByText('New colleague')).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Search users...' })).toHaveValue('');
    fireEvent.click(screen.getByRole('button', { name: 'Manage access for New colleague' }));
    await waitFor(() =>
      expect(mocks.permissions).toHaveBeenCalledWith('sso:oidc:company:stable-subject'),
    );
    expect(mocks.update).not.toHaveBeenCalled();
  });

  it('offers setup instead of a link when no provider is enabled', async () => {
    mocks.status.mockResolvedValue({ ...configured, ssoEnabled: false });
    await openGuide();
    expect(await screen.findByText('Set up team sign-in first')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Configure Single Sign-On' })).toHaveAttribute(
      'href',
      '/settings/security-sso',
    );
    expect(screen.queryByRole('button', { name: 'Copy link' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Refresh user list' })).not.toBeInTheDocument();
  });

  it('supports trusted proxy sign-in without SSO providers', async () => {
    mocks.status.mockResolvedValue({ ...configured, ssoEnabled: false, hasProxyAuth: true });
    await openGuide();
    expect(await screen.findByText('Trusted proxy authentication')).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Pulse sign-in link' })).toBeInTheDocument();
  });

  it('keeps sharing unavailable while loading and retries a failed discovery', async () => {
    let rejectStatus!: (error: Error) => void;
    mocks.status.mockReturnValueOnce(
      new Promise((_, reject) => {
        rejectStatus = reject;
      }),
    );
    await openGuide();
    expect(screen.queryByRole('button', { name: 'Copy link' })).not.toBeInTheDocument();
    rejectStatus(new Error('Forbidden'));
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not check team sign-in');
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByText('Company sign-in (OIDC)')).toBeInTheDocument();
    expect(mocks.status).toHaveBeenCalledTimes(2);
  });

  it('keeps the link selectable if clipboard access fails', async () => {
    mocks.copy.mockRejectedValue(new Error('Clipboard denied'));
    await openGuide();
    fireEvent.click(await screen.findByRole('button', { name: 'Copy link' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('copy it manually');
    expect(screen.getByRole('textbox', { name: 'Pulse sign-in link' })).toHaveAttribute('readonly');
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('performs no new reads and exposes no onboarding when RBAC is unavailable', async () => {
    mocks.enabled = false;
    render(() => <UserAssignmentsPanel />);
    expect(screen.queryByRole('button', { name: 'Add teammate' })).not.toBeInTheDocument();
    expect(mocks.status).not.toHaveBeenCalled();
    expect(mocks.users).not.toHaveBeenCalled();
  });

  it('shows a refresh failure instead of treating it as an empty team', async () => {
    await openGuide();
    await screen.findByRole('button', { name: 'Copy link' });
    mocks.users.mockRejectedValueOnce(new Error('Storage unavailable'));
    fireEvent.click(screen.getByRole('button', { name: 'Refresh user list' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load user assignments');
    expect(screen.queryByText('No users yet')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByText('No users yet')).toBeInTheDocument();
  });
});

describe('team sign-in presentation', () => {
  it.each([
    'javascript:alert(1)',
    'file:///private',
    'https://user:password@pulse.example',
    'invalid',
  ])('rejects unsafe source %s', (origin) => {
    expect(getTeamSignInUrl(origin)).toBeNull();
  });
  it('does not share an administrator route or query', () => {
    expect(
      getTeamSignInUrl('https://pulse.example:8443/settings/security-users?token=private#section'),
    ).toBe('https://pulse.example:8443/login');
  });
  it('does not count disabled SSO as configured', () => {
    expect(
      getTeamSignInMethods({ ...configured, ssoEnabled: false } as Parameters<
        typeof getTeamSignInMethods
      >[0]),
    ).toEqual([]);
  });
});
