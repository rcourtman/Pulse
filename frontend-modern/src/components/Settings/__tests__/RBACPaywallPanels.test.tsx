import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { Dynamic } from 'solid-js/web';

import { RolesPanel } from '../RolesPanel';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';

const hasFeatureMock = vi.fn();
const loadLicenseStatusMock = vi.fn();
const loadCommercialPostureMock = vi.fn();
const startProTrialMock = vi.fn();
const commercialPostureMock = vi.fn();
const entitlementsMock = vi.fn();
const presentationPolicyHidesUpgradePromptsMock = vi.fn();
const getRolesMock = vi.fn();
const getUsersMock = vi.fn();
const getUserPermissionsMock = vi.fn();
const updateUserRolesMock = vi.fn();
const deleteUserMock = vi.fn();
const deleteRoleMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const loggerErrorMock = vi.fn();

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

vi.mock('@/stores/license', () => ({
  hasFeature: (...args: unknown[]) => hasFeatureMock(...args),
  loadRuntimeCapabilities: (...args: unknown[]) => loadLicenseStatusMock(...args),
  runtimeCapabilitiesLoaded: () => true,
}));

vi.mock('@/stores/licenseCommercial', () => ({
  canOfferCommercialTrial: () => commercialPostureMock()?.trial_eligible !== false,
  commercialPosture: (...args: unknown[]) => commercialPostureMock(...args),
  getUpgradeActionDestination: (feature: string) => ({
    href: `https://example.com/upgrade?feature=${feature}`,
    external: true,
  }),
  getUpgradeActionUrlOrFallback: (feature: string) => `/upgrade?feature=${feature}`,
  loadCommercialPosture: (...args: unknown[]) => loadCommercialPostureMock(...args),
  loadRuntimeCapabilities: (...args: unknown[]) => loadCommercialPostureMock(...args),
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
  entitlements: (...args: unknown[]) => entitlementsMock(...args),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => presentationPolicyHidesUpgradePromptsMock(),
}));

vi.mock('@/api/rbac', () => ({
  RBACAPI: {
    getRoles: (...args: unknown[]) => getRolesMock(...args),
    getUsers: (...args: unknown[]) => getUsersMock(...args),
    getUserPermissions: (...args: unknown[]) => getUserPermissionsMock(...args),
    updateUserRoles: (...args: unknown[]) => updateUserRolesMock(...args),
    deleteUser: (...args: unknown[]) => deleteUserMock(...args),
    saveRole: vi.fn(),
    deleteRole: (...args: unknown[]) => deleteRoleMock(...args),
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
    loadCommercialPostureMock.mockReset();
    startProTrialMock.mockReset();
    commercialPostureMock.mockReset();
    entitlementsMock.mockReset();
    presentationPolicyHidesUpgradePromptsMock.mockReset();
    getRolesMock.mockReset();
    getUsersMock.mockReset();
    getUserPermissionsMock.mockReset();
    updateUserRolesMock.mockReset();
    deleteUserMock.mockReset();
    deleteRoleMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    loggerErrorMock.mockReset();

    hasFeatureMock.mockReturnValue(true);
    loadLicenseStatusMock.mockResolvedValue(undefined);
    loadCommercialPostureMock.mockResolvedValue(undefined);
    startProTrialMock.mockResolvedValue({ outcome: 'started' });
    commercialPostureMock.mockReturnValue({ trial_eligible: false });
    entitlementsMock.mockReturnValue({ trial_eligible: false });
    presentationPolicyHidesUpgradePromptsMock.mockReturnValue(false);
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
    deleteUserMock.mockResolvedValue(undefined);
    deleteRoleMock.mockResolvedValue(undefined);
  });

  afterEach(() => {
    cleanup();
  });

  it('shows the roles paywall for free entitlements and does not load RBAC roles', async () => {
    hasFeatureMock.mockImplementation((feature: string) => feature !== 'rbac');

    render(() => <RolesPanel />);

    await waitFor(() => {
      expect(screen.getByText('Custom Roles')).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: 'View plans' })).toHaveAttribute(
      'href',
      'https://example.com/upgrade?feature=rbac',
    );
    expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
    expect(getRolesMock).not.toHaveBeenCalled();
  });

  it('keeps RBAC upgrade actions quiet when self-hosted upgrade prompts are hidden', async () => {
    hasFeatureMock.mockImplementation((feature: string) => feature !== 'rbac');
    presentationPolicyHidesUpgradePromptsMock.mockReturnValue(true);

    render(() => <RolesPanel />);

    await waitFor(() => {
      expect(screen.getByText('Custom Roles')).toBeInTheDocument();
    });

    expect(screen.queryByText('Custom Roles (Pro)')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Start trial' })).not.toBeInTheDocument();
    expect(getRolesMock).not.toHaveBeenCalled();
  });

  it('loads roles when the RBAC entitlement is granted', async () => {
    render(() => <RolesPanel />);

    await waitFor(() => {
      expect(screen.getByText('Admin')).toBeInTheDocument();
    });

    expect(screen.queryByText('Custom Roles (Pro)')).not.toBeInTheDocument();
    expect(getRolesMock).toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'New Role' })).not.toBeDisabled();
  });

  it('keeps complete saved grants in each role-list projection, including read-only roles', async () => {
    const permissions = [
      { action: '*', resource: '*' },
      { action: 'read', resource: 'nodes' },
      { action: 'read', resource: 'resource_beyond_the_editor_shortcuts' },
    ];
    getRolesMock.mockResolvedValueOnce([
      {
        id: 'auditor',
        name: 'Saved auditor',
        description: 'Review only',
        permissions,
        isBuiltIn: true,
      },
      {
        id: 'draft',
        name: 'Draft role',
        description: 'Not configured',
        permissions: [],
        isBuiltIn: false,
      },
    ]);

    render(() => <RolesPanel />);

    const auditorName = await screen.findByText('Saved auditor');
    const auditor = within(auditorName.closest('tr')!);
    // CSS chooses the phone identity cell or desktop column; every projection
    // must retain every grant rather than a count or an editable-only summary.
    const lists = auditor.getAllByRole('list', { name: 'Permissions for Saved auditor' });
    for (const list of lists) {
      expect(within(list).getAllByRole('listitem')).toHaveLength(permissions.length);
      for (const permission of permissions) {
        expect(
          within(list).getByText(`${permission.action}:${permission.resource}`),
        ).toBeInTheDocument();
      }
    }
    expect(auditor.getByText('Read-only')).toBeInTheDocument();
    expect(auditor.queryByRole('button')).not.toBeInTheDocument();
    const draft = within(screen.getByText('Draft role').closest('tr')!);
    expect(draft.getAllByText('No permissions').length).toBeGreaterThan(0);
    expect(draft.queryByRole('list')).not.toBeInTheDocument();
    expect(draft.getByRole('button', { name: 'Edit role' })).toBeEnabled();
    expect(draft.getByRole('button', { name: 'Delete role' })).toBeEnabled();
    expect(getRolesMock).toHaveBeenCalledTimes(1);
  });

  it('keeps editable role actions touch-sized on mobile', async () => {
    getRolesMock.mockResolvedValueOnce([
      {
        id: 'operator',
        name: 'Operator',
        description: 'Operational access',
        permissions: [{ action: 'read', resource: 'alerts' }],
        isBuiltIn: false,
      },
    ]);

    render(() => <RolesPanel />);

    const edit = await screen.findByRole('button', { name: 'Edit role' });
    // The negative margin keeps the 44px phone target inside the row's
    // existing height, so custom roles line up with built-in ones.
    expect(edit).toHaveClass(
      '-my-2',
      'min-h-11',
      'min-w-11',
      'sm:my-0',
      'sm:min-h-0',
      'sm:min-w-0',
    );
    expect(screen.getByRole('button', { name: 'Delete role' })).toHaveClass(
      '-my-2',
      'sm:my-0',
      'min-h-11',
      'min-w-11',
      'sm:min-h-0',
      'sm:min-w-0',
    );
  });

  it('fails closed with a persistent retry notice when roles cannot be read', async () => {
    getRolesMock.mockRejectedValueOnce(
      Object.assign(new Error('RBAC data could not be loaded'), {
        status: 503,
        code: 'rbac_store_unavailable',
      }),
    );

    render(() => <RolesPanel />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Failed to load roles');
    });
    expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
    expect(notificationErrorMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await waitFor(() => {
      expect(screen.getByText('Admin')).toBeInTheDocument();
    });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('replaces uncertain confirmations with focused, read-only recovery in the consuming panel', async () => {
    const north = {
      id: 'fleet-auditor',
      name: 'Fleet auditor',
      description: 'Synthetic role',
      permissions: [{ action: 'read', resource: 'nodes' }],
      createdAt: '2026-11-01',
    };
    getRolesMock.mockResolvedValue([north]);
    deleteRoleMock.mockRejectedValueOnce(new Error('PRIVATE DETAIL'));
    render(() => <RolesPanel currentUser="reader-a" canManage={true} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Delete role' }));
    const dialog = await screen.findByRole('dialog', { name: `Delete role: ${north.name}` });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete Role' }));
    const notice = await screen.findByRole('alert');
    expect(notice).toHaveTextContent('could not be confirmed');
    expect(notice).not.toHaveTextContent('PRIVATE');
    await waitFor(() => expect(notice).toHaveFocus());
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.queryByText('No roles available.')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
    expect(getRolesMock).toHaveBeenCalledTimes(1);
    getRolesMock.mockRejectedValueOnce(new Error('PRIVATE READ DETAIL'));
    fireEvent.click(screen.getByRole('button', { name: 'Check saved roles' }));
    expect(await screen.findByText('Failed to load roles')).toBeInTheDocument();
    expect(notice).toHaveTextContent('could not be confirmed');
    expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument();
    getRolesMock.mockResolvedValueOnce([]);
    fireEvent.click(screen.getByRole('button', { name: 'Check saved roles' }));
    expect(await screen.findByText('No roles available.')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New Role' })).not.toBeDisabled();
    expect(deleteRoleMock).toHaveBeenCalledTimes(1);
  });

  it('keeps the rendered pending User Access dialog across unchanged status and shows its uncertain outcome', async () => {
    const [status, setStatus] = createSignal({ user: 'operator-a', allowed: true });
    const pending = deferred<void>();
    updateUserRolesMock.mockReturnValueOnce(pending.promise);
    const panelProps = () => ({ currentUser: status().user, canManage: status().allowed });
    render(() => <Dynamic component={UserAssignmentsPanel} {...panelProps()} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Manage access for alice' }));
    const dialog = await screen.findByRole('dialog', { name: 'Manage access: alice' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save Changes' }));
    expect(within(dialog).getByRole('button', { name: 'Applying...' })).toBeDisabled();
    const signal = updateUserRolesMock.mock.calls[0][2].signal;
    setStatus({ user: 'operator-a', allowed: true });
    await waitFor(() =>
      expect(within(dialog).getByRole('button', { name: 'Applying...' })).toBeDisabled(),
    );
    expect(dialog).toBeInTheDocument();
    expect(signal.aborted).toBe(false);
    expect(getUsersMock).toHaveBeenCalledTimes(1);
    pending.reject(Object.assign(new Error('Synthetic lost reply'), { status: 503 }));
    const warning = await screen.findByRole('alert');
    expect(warning).toHaveTextContent('could not be confirmed');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    setStatus({ user: 'operator-a', allowed: true });
    await waitFor(() => expect(warning).toHaveFocus());
    expect(screen.getByRole('button', { name: 'Check saved access' })).toBeInTheDocument();
    expect(getUsersMock).toHaveBeenCalledTimes(1);
    getUsersMock.mockResolvedValueOnce([]);
    fireEvent.click(screen.getByRole('button', { name: 'Check saved access' }));
    expect(await screen.findByText('No users yet')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(updateUserRolesMock).toHaveBeenCalledTimes(1);
  });

  it('shows the user assignments paywall for free entitlements and does not load users', async () => {
    hasFeatureMock.mockImplementation((feature: string) => feature !== 'rbac');

    render(() => <UserAssignmentsPanel />);

    await waitFor(() => {
      expect(screen.getByText('Centralized Access Control')).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: 'View plans' })).toHaveAttribute(
      'href',
      'https://example.com/upgrade?feature=rbac',
    );
    expect(screen.getByPlaceholderText('Search users...')).toBeDisabled();
    expect(getUsersMock).not.toHaveBeenCalled();
    expect(getRolesMock).not.toHaveBeenCalled();
  });

  it('loads user assignments when the RBAC entitlement is granted', async () => {
    render(() => <UserAssignmentsPanel />);

    await waitFor(() => {
      expect(screen.getByText('alice')).toBeInTheDocument();
    });

    expect(screen.queryByText('Centralized Access Control (Pro)')).not.toBeInTheDocument();
    expect(getUsersMock).toHaveBeenCalled();
    expect(getRolesMock).toHaveBeenCalled();
    expect(screen.getByPlaceholderText('Search users...')).not.toBeDisabled();
    expect(screen.getByRole('button', { name: 'Manage access for alice' })).toHaveClass(
      'min-h-11',
      'sm:min-h-9',
    );
  });

  const searchUsers = [
    {
      username: 'sso:oidc:fixture:alice-001',
      displayName: 'Alice Example',
      email: 'alice@example.invalid',
      providerType: 'oidc',
      providerId: 'Engineering',
      roleIds: ['admin'],
    },
    { username: 'bob', displayName: 'Bob Operator', roleIds: [] },
  ];

  it('distinguishes a filtered directory from no users and clears the filter without a request', async () => {
    getUsersMock.mockResolvedValue(searchUsers);
    render(() => <UserAssignmentsPanel />);
    await screen.findByRole('button', { name: 'Manage access for Alice Example' });
    const search = screen.getByRole('textbox', { name: 'Search users...' });
    fireEvent.input(search, { target: { value: 'absent-user' } });

    expect(screen.getByRole('heading', { name: 'No matching users' })).toBeInTheDocument();
    expect(screen.getByRole('status', { name: 'User search results' })).toHaveTextContent(
      'Showing 0 of 2 users',
    );
    expect(screen.queryByText('No users yet')).not.toBeInTheDocument();
    expect(screen.queryByText('Configure SSO in Security settings')).not.toBeInTheDocument();
    expect(screen.queryByText('Users sync on first login')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Manage access for/ })).not.toBeInTheDocument();
    const showAll = screen.getByRole('button', { name: 'Show all users' });
    showAll.focus();
    fireEvent.click(showAll);

    expect(search).toHaveValue('');
    expect(search).toHaveFocus();
    expect(screen.getAllByRole('button', { name: /Manage access for/ })).toHaveLength(2);
    expect(screen.getByRole('status', { name: 'User search results' })).toHaveTextContent(
      /^2 users$/,
    );
    expect(getUsersMock).toHaveBeenCalledTimes(1);
    expect(getRolesMock).toHaveBeenCalledTimes(1);
    expect(getUserPermissionsMock).not.toHaveBeenCalled();
    expect(updateUserRolesMock).not.toHaveBeenCalled();
    expect(deleteUserMock).not.toHaveBeenCalled();
  });

  it.each([
    '  ALICE EXAMPLE  ',
    ' ALICE@EXAMPLE.INVALID ',
    '  alice-001 ',
    ' OIDC ',
    ' Engineering ',
  ])('searches identity metadata with a trimmed, case-insensitive query: %s', async (query) => {
    getUsersMock.mockResolvedValue(searchUsers);
    render(() => <UserAssignmentsPanel />);
    await screen.findByRole('button', { name: 'Manage access for Alice Example' });
    fireEvent.input(screen.getByRole('textbox', { name: 'Search users...' }), {
      target: { value: query },
    });
    expect(screen.getAllByRole('button', { name: /Manage access for/ })).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'Manage access for Alice Example' })).toBeEnabled();
    expect(screen.getByRole('status', { name: 'User search results' })).toHaveTextContent(
      'Showing 1 of 2 users',
    );
    expect(getUsersMock).toHaveBeenCalledTimes(1);
    expect(getRolesMock).toHaveBeenCalledTimes(1);
  });

  it('keeps a whitespace-only search unfiltered and reports a singular user correctly', async () => {
    render(() => <UserAssignmentsPanel />);
    await screen.findByRole('button', { name: 'Manage access for alice' });
    fireEvent.input(screen.getByRole('textbox', { name: 'Search users...' }), {
      target: { value: '   ' },
    });
    expect(screen.getByRole('button', { name: 'Manage access for alice' })).toBeInTheDocument();
    expect(screen.getByRole('status', { name: 'User search results' })).toHaveTextContent(
      /^1 user$/,
    );
  });

  it('keeps first-login guidance for an accepted empty directory, even with a query', async () => {
    getUsersMock.mockResolvedValue([]);
    render(() => <UserAssignmentsPanel />);
    await screen.findByRole('heading', { name: 'No users yet' });
    fireEvent.input(screen.getByRole('textbox', { name: 'Search users...' }), {
      target: { value: 'alice' },
    });
    expect(screen.getByText('Configure SSO in Security settings')).toBeInTheDocument();
    expect(screen.getByText('Users sync on first login')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'No matching users' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Show all users' })).not.toBeInTheDocument();
    expect(screen.queryByRole('status', { name: 'User search results' })).not.toBeInTheDocument();
  });

  it('never presents a pending or failed directory read as a search result or empty account list', async () => {
    const pending = deferred<typeof searchUsers>();
    getUsersMock.mockReturnValueOnce(pending.promise);
    render(() => <UserAssignmentsPanel />);
    fireEvent.input(screen.getByRole('textbox', { name: 'Search users...' }), {
      target: { value: 'alice' },
    });
    expect(screen.queryByRole('status', { name: 'User search results' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /No (matching )?users/ })).not.toBeInTheDocument();
    pending.reject(new Error('synthetic unavailable'));
    await screen.findByText('Failed to load user assignments');
    expect(screen.queryByRole('status', { name: 'User search results' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /No (matching )?users/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Show all users' })).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Search users...' })).toBeDisabled();
  });

  it('withdraws search counts and recovery actions with the current reader', async () => {
    getUsersMock.mockResolvedValue(searchUsers);
    const [canManage, setCanManage] = createSignal(true);
    render(() => <UserAssignmentsPanel canManage={canManage()} />);
    await screen.findByRole('button', { name: 'Manage access for Alice Example' });
    fireEvent.input(screen.getByRole('textbox', { name: 'Search users...' }), {
      target: { value: 'absent-user' },
    });
    expect(screen.getByRole('status', { name: 'User search results' })).toHaveTextContent(
      'Showing 0 of 2 users',
    );
    setCanManage(false);
    await screen.findByText('User access is unavailable for this session.');
    expect(screen.queryByRole('status', { name: 'User search results' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Show all users' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /No (matching )?users/ })).not.toBeInTheDocument();
  });

  it('clears the previous user permissions during a new read and reports failure as unknown', async () => {
    const pending = deferred<{ action: string; resource: string }[]>();
    getUsersMock.mockResolvedValue([
      { username: 'alice', roleIds: ['admin'] },
      { username: 'bob', roleIds: [] },
    ]);
    getUserPermissionsMock
      .mockResolvedValueOnce([{ action: 'write', resource: 'private-alice-resource' }])
      .mockReturnValueOnce(pending.promise);
    render(() => <UserAssignmentsPanel />);

    fireEvent.click(await screen.findByRole('button', { name: 'Manage access for alice' }));
    const aliceDialog = await screen.findByRole('dialog', { name: 'Manage access: alice' });
    expect(await within(aliceDialog).findByText('private-alice-resource')).toBeInTheDocument();
    fireEvent.click(within(aliceDialog).getByRole('button', { name: 'Cancel' }));
    fireEvent.click(screen.getByRole('button', { name: 'Manage access for bob' }));
    const bobDialog = await screen.findByRole('dialog', { name: 'Manage access: bob' });

    expect(
      within(bobDialog).getByRole('status', { name: 'Loading effective permissions' }),
    ).toBeInTheDocument();
    expect(within(bobDialog).queryByText('private-alice-resource')).not.toBeInTheDocument();
    pending.reject(new Error('private-provider-detail'));
    expect(await within(bobDialog).findByRole('alert')).toHaveTextContent('Access is unknown');
    expect(bobDialog).not.toHaveTextContent('private-provider-detail');
    expect(bobDialog).not.toHaveTextContent('No effective permissions');
    expect(bobDialog).not.toHaveTextContent('private-alice-resource');
    expect(updateUserRolesMock).not.toHaveBeenCalled();
    expect(deleteUserMock).not.toHaveBeenCalled();
  });

  it('retries only the saved-permissions read and preserves unsaved role selections', async () => {
    const pending = deferred<{ action: string; resource: string }[]>();
    getRolesMock.mockResolvedValue([
      { id: 'admin', name: 'Admin', description: 'Full access', permissions: [] },
      { id: 'viewer', name: 'Viewer', description: 'Read only', permissions: [] },
    ]);
    getUserPermissionsMock
      .mockRejectedValueOnce(new Error('feature not included in license: private detail'))
      .mockReturnValueOnce(pending.promise);
    render(() => <UserAssignmentsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: 'Manage access for alice' }));
    const dialog = await screen.findByRole('dialog', { name: 'Manage access: alice' });
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Access is unknown');
    expect(dialog).not.toHaveTextContent('private detail');
    fireEvent.click(within(dialog).getByRole('checkbox', { name: /Admin/ }));
    fireEvent.click(within(dialog).getByRole('checkbox', { name: /Viewer/ }));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Retry saved permissions' }));
    expect(
      within(dialog).getByRole('status', { name: 'Loading effective permissions' }),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByRole('button', { name: 'Retry saved permissions' }),
    ).not.toBeInTheDocument();
    expect(getUserPermissionsMock).toHaveBeenCalledTimes(2);
    expect(getUserPermissionsMock).toHaveBeenLastCalledWith(
      'alice',
      expect.objectContaining({
        expectedOrgID: 'default',
        retry: false,
        signal: expect.any(AbortSignal),
      }),
    );
    pending.resolve([{ action: 'read', resource: 'alerts' }]);
    expect(await within(dialog).findByText('alerts')).toBeInTheDocument();
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
    expect(within(dialog).getByRole('checkbox', { name: /Admin/ })).not.toBeChecked();
    expect(within(dialog).getByRole('checkbox', { name: /Viewer/ })).toBeChecked();
    expect(updateUserRolesMock).not.toHaveBeenCalled();
    expect(deleteUserMock).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole('button', { name: 'Save Changes' }));
    await waitFor(() =>
      expect(updateUserRolesMock).toHaveBeenCalledWith(
        'alice',
        ['viewer'],
        expect.objectContaining({
          expectedOrgID: 'default',
          retry: false,
          signal: expect.any(AbortSignal),
        }),
      ),
    );
    expect(updateUserRolesMock).toHaveBeenCalledTimes(1);
  });

  it('describes accepted empty permissions as saved state, not the result of unsaved changes', async () => {
    render(() => <UserAssignmentsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: 'Manage access for alice' }));
    const dialog = await screen.findByRole('dialog', { name: 'Manage access: alice' });
    expect(
      await within(dialog).findByText("No permissions were returned for this user's saved roles."),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('checkbox', { name: /Admin/ }));
    expect(dialog).toHaveTextContent('Unsaved role selections are not included.');
    expect(dialog).not.toHaveTextContent('This user will have no access.');
    expect(dialog).not.toHaveTextContent('Effective Permissions Preview');
  });

  it.each(['success', 'failure'])(
    'ignores an obsolete %s while another user permissions read is pending',
    async (outcome) => {
      const oldRead = deferred<{ action: string; resource: string }[]>();
      const currentRead = deferred<{ action: string; resource: string }[]>();
      getUsersMock.mockResolvedValue([
        { username: 'alice', roleIds: ['admin'] },
        { username: 'bob', roleIds: [] },
      ]);
      getUserPermissionsMock
        .mockReturnValueOnce(oldRead.promise)
        .mockReturnValueOnce(currentRead.promise);
      render(() => <UserAssignmentsPanel />);
      fireEvent.click(await screen.findByRole('button', { name: 'Manage access for alice' }));
      fireEvent.click(
        within(await screen.findByRole('dialog')).getByRole('button', { name: 'Cancel' }),
      );
      fireEvent.click(screen.getByRole('button', { name: 'Manage access for bob' }));
      const dialog = await screen.findByRole('dialog', { name: 'Manage access: bob' });
      if (outcome === 'success')
        oldRead.resolve([{ action: 'write', resource: 'private-alice-resource' }]);
      else oldRead.reject(new Error('private old read failure'));
      await oldRead.promise.catch(() => undefined);

      expect(
        within(dialog).getByRole('status', { name: 'Loading effective permissions' }),
      ).toBeInTheDocument();
      expect(dialog).not.toHaveTextContent('private-alice-resource');
      expect(dialog).not.toHaveTextContent('No effective permissions');
      expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
      currentRead.resolve([{ action: 'read', resource: 'bob-resource' }]);
      expect(await within(dialog).findByText('bob-resource')).toBeInTheDocument();
      expect(dialog).not.toHaveTextContent('private-alice-resource');
      expect(loggerErrorMock).not.toHaveBeenCalled();
    },
  );

  it('does not let an earlier dialog read overwrite a reopened dialog for the same principal', async () => {
    const oldRead = deferred<{ action: string; resource: string }[]>();
    getUserPermissionsMock
      .mockReturnValueOnce(oldRead.promise)
      .mockRejectedValueOnce(new Error('current read failed'));
    render(() => <UserAssignmentsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: 'Manage access for alice' }));
    fireEvent.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: 'Cancel' }),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Manage access for alice' }));
    const dialog = await screen.findByRole('dialog', { name: 'Manage access: alice' });
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Access is unknown');
    oldRead.resolve([{ action: 'write', resource: 'obsolete-resource' }]);
    await oldRead.promise;
    expect(within(dialog).getByRole('alert')).toHaveTextContent('Access is unknown');
    expect(dialog).not.toHaveTextContent('obsolete-resource');
    expect(getUserPermissionsMock).toHaveBeenCalledTimes(2);
  });

  it('discards pending permission reads when the RBAC capability is lost', async () => {
    const [enabled, setEnabled] = createSignal(true);
    const pending = deferred<{ action: string; resource: string }[]>();
    hasFeatureMock.mockImplementation(() => enabled());
    getUserPermissionsMock.mockReturnValueOnce(pending.promise);
    render(() => <UserAssignmentsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: 'Manage access for alice' }));
    expect(await screen.findByRole('dialog', { name: 'Manage access: alice' })).toBeInTheDocument();

    setEnabled(false);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByText('Centralized Access Control')).toBeInTheDocument();
    pending.resolve([{ action: 'write', resource: 'obsolete-resource' }]);
    await pending.promise;
    expect(screen.queryByText('obsolete-resource')).not.toBeInTheDocument();
    expect(updateUserRolesMock).not.toHaveBeenCalled();
  });

  it('ignores a permission read failure after the panel is disposed', async () => {
    const pending = deferred<{ action: string; resource: string }[]>();
    getUserPermissionsMock.mockReturnValueOnce(pending.promise);
    const view = render(() => <UserAssignmentsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: 'Manage access for alice' }));
    view.unmount();
    pending.reject(new Error('obsolete private failure'));
    await pending.promise.catch(() => undefined);
    expect(loggerErrorMock).not.toHaveBeenCalled();
    expect(notificationErrorMock).not.toHaveBeenCalled();
  });

  it('shows readable SSO identity details and confirms deprovisioning', async () => {
    const user = {
      username: 'sso:oidc:okta:opaque_subject',
      displayName: 'Alice Example',
      email: 'alice@example.com',
      providerType: 'oidc',
      providerId: 'okta',
      roleIds: ['admin'],
    };
    getUsersMock.mockResolvedValueOnce([user]).mockResolvedValueOnce([]);

    render(() => <UserAssignmentsPanel />);

    expect(await screen.findByText('Alice Example')).toBeInTheDocument();
    expect(screen.getByText('alice@example.com')).toBeInTheDocument();
    expect(screen.getByText('OIDC · okta')).toBeInTheDocument();
    expect(screen.getByTitle('sso:oidc:okta:opaque_subject')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Remove Alice Example' }));
    const dialog = await screen.findByRole('dialog', { name: 'Remove user access: Alice Example' });
    expect(dialog).toHaveTextContent('does not disable the account at the identity provider');

    fireEvent.click(screen.getByRole('button', { name: 'Remove User Access' }));
    await waitFor(() => {
      expect(deleteUserMock).toHaveBeenCalledWith(
        'sso:oidc:okta:opaque_subject',
        expect.objectContaining({
          expectedOrgID: 'default',
          retry: false,
          signal: expect.any(AbortSignal),
        }),
      );
    });
    expect(await screen.findByText('User access removed for Alice Example')).toBeInTheDocument();
    expect(notificationSuccessMock).not.toHaveBeenCalled();
    await waitFor(() => {
      expect(screen.queryByText('Alice Example')).not.toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText('Search users...')).toHaveFocus();
  });

  it('does not render stale user data when assignment storage is unavailable', async () => {
    getUsersMock.mockRejectedValueOnce(
      Object.assign(new Error('RBAC data could not be loaded'), {
        status: 503,
        code: 'rbac_store_unavailable',
      }),
    );

    render(() => <UserAssignmentsPanel />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Failed to load user assignments');
    });
    expect(screen.queryByText('alice')).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText('Search users...')).toBeDisabled();
    expect(notificationErrorMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await waitFor(() => {
      expect(screen.getByText('alice')).toBeInTheDocument();
    });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('keeps user assignment upgrade actions quiet when self-hosted upgrade prompts are hidden', async () => {
    hasFeatureMock.mockImplementation((feature: string) => feature !== 'rbac');
    presentationPolicyHidesUpgradePromptsMock.mockReturnValue(true);

    render(() => <UserAssignmentsPanel />);

    await waitFor(() => {
      expect(screen.getByText('Centralized Access Control')).toBeInTheDocument();
    });

    expect(screen.queryByText('Centralized Access Control (Pro)')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Start trial' })).not.toBeInTheDocument();
    expect(getUsersMock).not.toHaveBeenCalled();
    expect(getRolesMock).not.toHaveBeenCalled();
  });

  it('shows the canonical update error when self role modification is denied', async () => {
    updateUserRolesMock.mockRejectedValueOnce(
      Object.assign(new Error('Cannot modify your own role assignments'), {
        status: 403,
        code: 'self_modification_denied',
      }),
    );

    render(() => <UserAssignmentsPanel />);

    await waitFor(() => {
      expect(screen.getByText('alice')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Manage access for alice' }));

    await waitFor(() => {
      expect(screen.getByRole('dialog', { name: 'Manage access: alice' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    await waitFor(() => {
      expect(updateUserRolesMock).toHaveBeenCalledWith(
        'alice',
        ['admin'],
        expect.objectContaining({
          expectedOrgID: 'default',
          retry: false,
          signal: expect.any(AbortSignal),
        }),
      );
    });
    expect(
      await within(screen.getByRole('dialog', { name: 'Manage access: alice' })).findByRole(
        'alert',
      ),
    ).toHaveTextContent('Failed to update user roles');
    expect(notificationErrorMock).not.toHaveBeenCalled();
    expect(notificationSuccessMock).not.toHaveBeenCalled();
    expect(screen.getByRole('dialog', { name: 'Manage access: alice' })).toBeInTheDocument();
  });

  it.each(['save', 'delete'])(
    'reconciles an unconfirmed user %s with reads, never a repeated mutation',
    async (kind) => {
      const mutation = kind === 'save' ? updateUserRolesMock : deleteUserMock;
      mutation.mockRejectedValueOnce(new Error('private response must not be rendered'));
      render(() => <UserAssignmentsPanel />);
      await screen.findByRole('button', { name: 'Manage access for alice' });
      if (kind === 'save') {
        fireEvent.click(screen.getByRole('button', { name: 'Manage access for alice' }));
        fireEvent.click(await screen.findByRole('button', { name: 'Save Changes' }));
      } else {
        fireEvent.click(screen.getByRole('button', { name: 'Remove alice' }));
        fireEvent.click(
          within(screen.getByRole('dialog')).getByRole('button', { name: 'Remove User Access' }),
        );
      }
      const check = await screen.findByRole('button', { name: 'Check saved access' });
      const warning = screen.getByRole('alert');
      expect(warning).toHaveTextContent('may already have been applied');
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: 'Manage access for alice' }),
      ).not.toBeInTheDocument();
      expect(screen.queryByText('No users yet')).not.toBeInTheDocument();
      expect(screen.queryByText('private response must not be rendered')).not.toBeInTheDocument();
      await waitFor(() => expect(warning).toHaveFocus());
      expect(getUsersMock).toHaveBeenCalledTimes(1);
      getUsersMock.mockRejectedValueOnce(new Error('private read failure'));
      fireEvent.click(check);
      await screen.findByText('Failed to load user assignments');
      expect(screen.getByRole('button', { name: 'Check saved access' })).toBeInTheDocument();
      expect(screen.queryByText('No users yet')).not.toBeInTheDocument();
      getUsersMock.mockResolvedValueOnce([]);
      fireEvent.click(screen.getByRole('button', { name: 'Check saved access' }));
      await screen.findByText('No users yet');
      expect(screen.queryByText(/may already have been applied/)).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Check saved access' })).not.toBeInTheDocument();
      expect(getUsersMock).toHaveBeenCalledTimes(3);
      expect(mutation).toHaveBeenCalledTimes(1);
      expect(updateUserRolesMock.mock.calls.length + deleteUserMock.mock.calls.length).toBe(1);
      expect(notificationErrorMock).not.toHaveBeenCalled();
      expect(notificationSuccessMock).not.toHaveBeenCalled();
      expect(loggerErrorMock).not.toHaveBeenCalled();
    },
  );

  it('keeps trial start out of RBAC feature gates', async () => {
    hasFeatureMock.mockImplementation((feature: string) => feature !== 'rbac');
    commercialPostureMock.mockReturnValue({ trial_eligible: true });
    entitlementsMock.mockReturnValue({ trial_eligible: true });

    render(() => <RolesPanel />);

    await waitFor(() => {
      expect(screen.getByText('Custom Roles')).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: 'View plans' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Start trial' })).not.toBeInTheDocument();
    expect(startProTrialMock).not.toHaveBeenCalled();
  });
});
