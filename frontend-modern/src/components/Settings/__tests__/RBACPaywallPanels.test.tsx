import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

import { RolesPanel } from '../RolesPanel';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';

const hasFeatureMock = vi.fn();
const loadLicenseStatusMock = vi.fn();
const startProTrialMock = vi.fn();
const entitlementsMock = vi.fn();
const trackPaywallViewedMock = vi.fn();
const trackUpgradeClickedMock = vi.fn();
const getRolesMock = vi.fn();
const getUsersMock = vi.fn();
const getUserPermissionsMock = vi.fn();
const updateUserRolesMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const loggerErrorMock = vi.fn();

vi.mock('@/stores/license', () => ({
  getUpgradeActionUrlOrFallback: (feature: string) => `/upgrade?feature=${feature}`,
  hasFeature: (...args: unknown[]) => hasFeatureMock(...args),
  loadLicenseStatus: (...args: unknown[]) => loadLicenseStatusMock(...args),
  licenseLoaded: () => true,
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
  entitlements: (...args: unknown[]) => entitlementsMock(...args),
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: (...args: unknown[]) => trackPaywallViewedMock(...args),
  trackUpgradeClicked: (...args: unknown[]) => trackUpgradeClickedMock(...args),
}));

vi.mock('@/api/rbac', () => ({
  RBACAPI: {
    getRoles: (...args: unknown[]) => getRolesMock(...args),
    getUsers: (...args: unknown[]) => getUsersMock(...args),
    getUserPermissions: (...args: unknown[]) => getUserPermissionsMock(...args),
    updateUserRoles: (...args: unknown[]) => updateUserRolesMock(...args),
    saveRole: vi.fn(),
    deleteRole: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: (...args: unknown[]) => loggerErrorMock(...args),
    debug: vi.fn(),
    warn: vi.fn(),
  },
}));

describe('RBAC paywall settings panels', () => {
  beforeEach(() => {
    hasFeatureMock.mockReset();
    loadLicenseStatusMock.mockReset();
    startProTrialMock.mockReset();
    entitlementsMock.mockReset();
    trackPaywallViewedMock.mockReset();
    trackUpgradeClickedMock.mockReset();
    getRolesMock.mockReset();
    getUsersMock.mockReset();
    getUserPermissionsMock.mockReset();
    updateUserRolesMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    loggerErrorMock.mockReset();

    hasFeatureMock.mockReturnValue(true);
    loadLicenseStatusMock.mockResolvedValue(undefined);
    startProTrialMock.mockResolvedValue({ outcome: 'started' });
    entitlementsMock.mockReturnValue({ trial_eligible: false });
    getRolesMock.mockResolvedValue([
      {
        id: 'admin',
        name: 'Admin',
        description: 'Full access',
        permissions: [{ action: 'read', resource: 'alerts' }],
        isBuiltIn: true,
      },
    ]);
    getUsersMock.mockResolvedValue([
      {
        username: 'alice',
        roleIds: ['admin'],
      },
    ]);
    getUserPermissionsMock.mockResolvedValue([]);
    updateUserRolesMock.mockResolvedValue(undefined);
  });

  afterEach(() => {
    cleanup();
  });

  it('shows the roles paywall for free entitlements and does not load RBAC roles', async () => {
    hasFeatureMock.mockImplementation((feature: string) => feature !== 'rbac');

    render(() => <RolesPanel />);

    await waitFor(() => {
      expect(screen.getByText('Custom Roles (Pro)')).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: 'Upgrade to Pro' })).toHaveAttribute(
      'href',
      '/upgrade?feature=rbac',
    );
    expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
    expect(getRolesMock).not.toHaveBeenCalled();
    expect(trackPaywallViewedMock).toHaveBeenCalledWith('rbac', 'settings_roles_panel');
  });

  it('loads roles when the RBAC entitlement is granted', async () => {
    render(() => <RolesPanel />);

    await waitFor(() => {
      expect(screen.getByText('Admin')).toBeInTheDocument();
    });

    expect(screen.queryByText('Custom Roles (Pro)')).not.toBeInTheDocument();
    expect(getRolesMock).toHaveBeenCalled();
    expect(trackPaywallViewedMock).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'New Role' })).not.toBeDisabled();
  });

  it('shows the user assignments paywall for free entitlements and does not load users', async () => {
    hasFeatureMock.mockImplementation((feature: string) => feature !== 'rbac');

    render(() => <UserAssignmentsPanel />);

    await waitFor(() => {
      expect(screen.getByText('Centralized Access Control (Pro)')).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: 'Upgrade to Pro' })).toHaveAttribute(
      'href',
      '/upgrade?feature=rbac',
    );
    expect(screen.getByPlaceholderText('Search users...')).toBeDisabled();
    expect(getUsersMock).not.toHaveBeenCalled();
    expect(getRolesMock).not.toHaveBeenCalled();
    expect(trackPaywallViewedMock).toHaveBeenCalledWith('rbac', 'settings_user_assignments_panel');
  });

  it('loads user assignments when the RBAC entitlement is granted', async () => {
    render(() => <UserAssignmentsPanel />);

    await waitFor(() => {
      expect(screen.getByText('alice')).toBeInTheDocument();
    });

    expect(screen.queryByText('Centralized Access Control (Pro)')).not.toBeInTheDocument();
    expect(getUsersMock).toHaveBeenCalled();
    expect(getRolesMock).toHaveBeenCalled();
    expect(trackPaywallViewedMock).not.toHaveBeenCalled();
    expect(screen.getByPlaceholderText('Search users...')).not.toBeDisabled();
  });

  it('shows the canonical update error when self role modification is denied', async () => {
    updateUserRolesMock.mockRejectedValueOnce(
      Object.assign(new Error('Cannot modify your own role assignments'), { status: 403, code: 'self_modification_denied' }),
    );

    render(() => <UserAssignmentsPanel />);

    await waitFor(() => {
      expect(screen.getByText('alice')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Manage Access' }));

    await waitFor(() => {
      expect(screen.getByRole('dialog', { name: 'Manage access: alice' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    await waitFor(() => {
      expect(updateUserRolesMock).toHaveBeenCalledWith('alice', ['admin']);
    });
    expect(notificationErrorMock).toHaveBeenCalledWith('Failed to update user roles');
    expect(notificationSuccessMock).not.toHaveBeenCalled();
    expect(screen.getByRole('dialog', { name: 'Manage access: alice' })).toBeInTheDocument();
  });
});
