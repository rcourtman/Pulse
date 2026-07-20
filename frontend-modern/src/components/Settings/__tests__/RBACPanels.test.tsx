import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { RolesPanel } from '../RolesPanel';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';

const mocks = vi.hoisted(() => ({
  hasRBAC: false,
  getRoles: vi.fn(),
  getUsers: vi.fn(),
  getSystemSettings: vi.fn(),
  notificationError: vi.fn(),
}));

vi.mock('@/stores/license', () => ({
  hasFeature: (feature: string) => feature === 'rbac' && mocks.hasRBAC,
  loadLicenseStatus: () => Promise.resolve(),
  licenseLoaded: () => true,
}));

vi.mock('@/api/rbac', () => ({
  RBACAPI: {
    getRoles: (...args: unknown[]) => mocks.getRoles(...args),
    getUsers: (...args: unknown[]) => mocks.getUsers(...args),
    getUserPermissions: vi.fn(),
    updateUserRoles: vi.fn(),
    saveRole: vi.fn(),
    deleteRole: vi.fn(),
  },
}));

vi.mock('@/api/settings', () => ({
  SettingsAPI: {
    getSystemSettings: (...args: unknown[]) => mocks.getSystemSettings(...args),
    updateSystemSettings: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: (...args: unknown[]) => mocks.notificationError(...args),
    success: vi.fn(),
  },
}));

describe('RBAC settings panels', () => {
  afterEach(() => {
    cleanup();
    mocks.hasRBAC = false;
    mocks.getRoles.mockReset();
    mocks.getUsers.mockReset();
    mocks.getSystemSettings.mockReset();
    mocks.notificationError.mockReset();
  });

  it('does not call role endpoints before the RBAC feature is licensed', async () => {
    render(() => <RolesPanel />);

    await waitFor(() => expect(screen.getByText('Custom Roles (Pro)')).toBeInTheDocument());

    expect(mocks.getRoles).not.toHaveBeenCalled();
    expect(mocks.getSystemSettings).not.toHaveBeenCalled();
    expect(mocks.notificationError).not.toHaveBeenCalledWith('Failed to load roles');
    expect(screen.queryByRole('button', { name: /new role/i })).not.toBeInTheDocument();
  });

  it('does not call user assignment endpoints before the RBAC feature is licensed', async () => {
    render(() => <UserAssignmentsPanel />);

    await waitFor(() => expect(screen.getByText('Centralized Access Control (Pro)')).toBeInTheDocument());

    expect(mocks.getUsers).not.toHaveBeenCalled();
    expect(mocks.getRoles).not.toHaveBeenCalled();
    expect(mocks.notificationError).not.toHaveBeenCalledWith('Failed to load user assignments');
    expect(screen.queryByPlaceholderText('Search users...')).not.toBeInTheDocument();
  });
});
