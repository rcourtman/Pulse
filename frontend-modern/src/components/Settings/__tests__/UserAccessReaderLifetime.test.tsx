import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { eventBus } from '@/stores/events';
import { useUserAssignmentsPanelState } from '../useUserAssignmentsPanelState';
import type { Permission, Role, UserRoleAssignment } from '@/types/rbac';

const mocks = vi.hoisted(() => ({
  org: 'north',
  getUsers: vi.fn(),
  getRoles: vi.fn(),
  getUserPermissions: vi.fn(),
  updateUserRoles: vi.fn(),
  deleteUser: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  log: vi.fn(),
}));
vi.mock('@/utils/apiClient', () => ({ getOrgID: () => mocks.org }));
vi.mock('@/api/rbac', () => ({ RBACAPI: mocks }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: mocks.success, error: mocks.error },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: mocks.log } }));
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({ licenseReady: () => true, rbacEnabled: () => true }),
}));
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}
const alice: UserRoleAssignment = {
  username: 'old-principal',
  displayName: 'North private user',
  email: 'north@example.invalid',
  roleIds: ['reader'],
};
const bob: UserRoleAssignment = {
  username: 'new-principal',
  displayName: 'South user',
  roleIds: ['reader'],
};
const roles: Role[] = [
  { id: 'reader', name: 'Reader', description: 'Read monitoring', permissions: [] },
];
const pause = () => new Promise((resolve) => setTimeout(resolve, 0));
function mount() {
  const [user, setUser] = createSignal<string | undefined>('operator-a');
  const [allowed, setAllowed] = createSignal(true);
  let state!: ReturnType<typeof useUserAssignmentsPanelState>;
  const view = render(() => {
    state = useUserAssignmentsPanelState({
      get currentUser() {
        return user();
      },
      get canManage() {
        return allowed();
      },
    });
    return <div />;
  });
  return { state, setUser, setAllowed, view };
}
async function loaded(state: ReturnType<typeof useUserAssignmentsPanelState>) {
  await waitFor(() => expect(state.loading()).toBe(false));
}
function switchOrg() {
  mocks.org = 'south';
  eventBus.emit('org_switched', 'south');
}

describe('User Access reader lifetime', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.org = 'north';
    mocks.getUsers.mockReset().mockResolvedValue([alice]);
    mocks.getRoles.mockReset().mockResolvedValue(roles);
    mocks.getUserPermissions
      .mockReset()
      .mockResolvedValue([{ action: 'read', resource: 'north-private' }]);
    mocks.updateUserRoles.mockReset().mockResolvedValue(undefined);
    mocks.deleteUser.mockReset().mockResolvedValue(undefined);
  });
  afterEach(cleanup);

  it('withdraws the account directory, draft, permissions and removal confirmation on org switch', async () => {
    const { state } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    state.setSearchQuery('private');
    state.openDeleteUser(alice);
    const next = deferred<UserRoleAssignment[]>();
    mocks.getUsers.mockReturnValueOnce(next.promise);
    switchOrg();
    await pause();
    expect(state.filteredAssignments()).toEqual([]);
    expect(state.editingUser()).toBeNull();
    expect(state.formRoleIds()).toEqual([]);
    expect(state.userPermissions()).toBeNull();
    expect(state.userPendingDeletion()).toBeNull();
    expect(state.searchQuery()).toBe('');
    next.resolve([bob]);
    await loaded(state);
    expect(state.filteredAssignments()).toEqual([bob]);
  });

  it('ignores an old directory response after a new organisation read has completed', async () => {
    const old = deferred<UserRoleAssignment[]>();
    mocks.getUsers.mockReturnValueOnce(old.promise);
    const { state } = mount();
    await pause();
    mocks.getUsers.mockResolvedValueOnce([bob]);
    switchOrg();
    await pause();
    old.resolve([alice]);
    await pause();
    expect(state.filteredAssignments()).toEqual([bob]);
  });

  it('does not import an old permissions response into a same-principal new reader', async () => {
    const old = deferred<Permission[]>();
    mocks.getUserPermissions.mockReturnValueOnce(old.promise);
    const { state, setUser } = mount();
    await loaded(state);
    const pending = state.openManageAccess(alice);
    await pause();
    setUser('operator-b');
    await pause();
    expect(state.showModal()).toBe(false);
    old.resolve([{ action: 'read', resource: 'private-from-old-reader' }]);
    await pending;
    expect(state.userPermissions()).toBeNull();
    expect(state.editingUser()).toBeNull();
  });

  it('withdraws all account state and refuses actions when administration is revoked', async () => {
    const { state, setAllowed } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    setAllowed(false);
    await pause();
    expect(state.filteredAssignments()).toEqual([]);
    expect(state.showModal()).toBe(false);
    await state.openManageAccess(alice);
    state.openDeleteUser(alice);
    await state.handleSaveAssignments();
    await state.handleDeleteUser();
    expect(mocks.updateUserRoles).not.toHaveBeenCalled();
    expect(mocks.deleteUser).not.toHaveBeenCalled();
  });

  it('does not announce an obsolete save or reload its account into the new organisation', async () => {
    const old = deferred<void>();
    mocks.updateUserRoles.mockReturnValueOnce(old.promise);
    const { state } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    const saving = state.handleSaveAssignments();
    await pause();
    mocks.getUsers.mockResolvedValueOnce([bob]);
    switchOrg();
    await pause();
    old.resolve();
    await saving;
    expect(mocks.success).not.toHaveBeenCalled();
    expect(state.filteredAssignments()).toEqual([bob]);
  });

  it('releases a pending directory read on disposal without notification or retained accounts', async () => {
    const old = deferred<UserRoleAssignment[]>();
    mocks.getUsers.mockReturnValueOnce(old.promise);
    const { state, view } = mount();
    await pause();
    view.unmount();
    old.resolve([alice]);
    await pause();
    expect(state.filteredAssignments()).toEqual([]);
    expect(mocks.error).not.toHaveBeenCalled();
    expect(mocks.success).not.toHaveBeenCalled();
  });
  it('recovers a fresh inventory after administration is restored, without restoring the old draft', async () => {
    const { state, setAllowed } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    setAllowed(false);
    await pause();
    mocks.getUsers.mockResolvedValueOnce([bob]);
    setAllowed(true);
    await loaded(state);
    expect(state.filteredAssignments()).toEqual([bob]);
    expect(state.editingUser()).toBeNull();
    expect(state.canEdit()).toBe(true);
  });

  it('refuses a stale row even when its principal also exists in the refreshed inventory', async () => {
    const { state } = mount();
    await loaded(state);
    const fresh = { ...alice, displayName: 'Fresh account' };
    mocks.getUsers.mockResolvedValueOnce([fresh]);
    await state.loadData();
    await state.openManageAccess(alice);
    state.openDeleteUser(alice);
    expect(state.showModal()).toBe(false);
    expect(state.userPendingDeletion()).toBeNull();
    await state.openManageAccess(fresh);
    expect(state.showModal()).toBe(true);
  });

  it('refuses a retained action before the org-switch event is delivered', async () => {
    const { state } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    mocks.org = 'south';
    await state.handleSaveAssignments();
    state.openDeleteUser(alice);
    await state.handleDeleteUser();
    expect(mocks.updateUserRoles).not.toHaveBeenCalled();
    expect(mocks.deleteUser).not.toHaveBeenCalled();
  });

  it('binds every read and write to an abortable, no-retry organisation request', async () => {
    const { state } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    const options = expect.objectContaining({
      expectedOrgID: 'north',
      retry: false,
      signal: expect.any(AbortSignal),
    });
    expect(mocks.getUsers).toHaveBeenCalledWith(options);
    expect(mocks.getRoles).toHaveBeenCalledWith(options);
    expect(mocks.getUserPermissions).toHaveBeenCalledWith(alice.username, options);
    await state.handleSaveAssignments();
    expect(mocks.updateUserRoles).toHaveBeenCalledWith(alice.username, ['reader'], options);
    state.openDeleteUser(alice);
    await state.handleDeleteUser();
    expect(mocks.deleteUser).toHaveBeenCalledWith(alice.username, options);
  });

  it('allows only one mutation and holds its confirmed role selection while it is pending', async () => {
    const pending = deferred<void>();
    mocks.updateUserRoles.mockReturnValueOnce(pending.promise);
    const { state } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    const save = state.handleSaveAssignments();
    state.toggleRole('reader');
    state.closeModal();
    await state.handleSaveAssignments();
    state.openDeleteUser(alice);
    await state.handleDeleteUser();
    expect(mocks.updateUserRoles).toHaveBeenCalledTimes(1);
    expect(mocks.deleteUser).not.toHaveBeenCalled();
    expect(state.formRoleIds()).toEqual(['reader']);
    expect(state.showModal()).toBe(true);
    pending.resolve();
    await save;
    expect(state.saving()).toBe(false);
    expect(state.showModal()).toBe(false);
  });

  it('preserves the current draft on a refused save without logging the private response', async () => {
    mocks.updateUserRoles.mockRejectedValueOnce(new Error('private response canary'));
    const { state } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    state.toggleRole('reader');
    await state.handleSaveAssignments();
    expect(state.showModal()).toBe(true);
    expect(state.formRoleIds()).toEqual([]);
    expect(mocks.error).toHaveBeenCalledWith('Failed to update user roles');
    expect(JSON.stringify(mocks.log.mock.calls)).not.toContain('private response canary');
    expect(mocks.success).not.toHaveBeenCalled();
  });

  it.each(['success', 'failure'])(
    'ignores obsolete deletion %s without closing the new reader editor',
    async (outcome) => {
      const old = deferred<void>();
      mocks.deleteUser.mockReturnValueOnce(old.promise);
      const { state } = mount();
      await loaded(state);
      state.openDeleteUser(alice);
      const deletion = state.handleDeleteUser();
      await pause();
      mocks.getUsers.mockResolvedValueOnce([bob]);
      switchOrg();
      await loaded(state);
      await state.openManageAccess(bob);
      if (outcome === 'success') old.resolve();
      else old.reject(new Error('private old failure'));
      await deletion;
      expect(state.showModal()).toBe(true);
      expect(state.editingUser()).toEqual(bob);
      expect(mocks.success).not.toHaveBeenCalled();
      expect(mocks.error).not.toHaveBeenCalled();
    },
  );

  it('aborts the previous directory and permission reads on withdrawal', async () => {
    const { state, setAllowed } = mount();
    await loaded(state);
    const permission = deferred<Permission[]>();
    mocks.getUserPermissions.mockReturnValueOnce(permission.promise);
    const reading = state.openManageAccess(alice);
    await pause();
    const permissionOptions = mocks.getUserPermissions.mock.calls.at(-1)![1];
    setAllowed(false);
    await pause();
    expect(permissionOptions.signal.aborted).toBe(true);
    permission.reject(new Error('old private failure'));
    await reading;
    expect(mocks.log).not.toHaveBeenCalled();
    expect(mocks.error).not.toHaveBeenCalled();
  });

  it('does not reuse a response after leaving and returning to the same organisation', async () => {
    const old = deferred<UserRoleAssignment[]>();
    mocks.getUsers.mockReturnValueOnce(old.promise);
    const { state } = mount();
    await pause();
    switchOrg();
    await loaded(state);
    mocks.getUsers.mockResolvedValueOnce([bob]);
    mocks.org = 'north';
    eventBus.emit('org_switched', 'north');
    await loaded(state);
    old.resolve([alice]);
    await pause();
    expect(state.filteredAssignments()).toEqual([bob]);
  });

  it.each([null, {}, [null], [{ ...alice, roleIds: null }], [alice, alice]])(
    'reports a malformed user inventory as unknown: %j',
    async (value) => {
      mocks.getUsers.mockResolvedValueOnce(value);
      const { state } = mount();
      await loaded(state);
      expect(state.loadError()).toBe('Failed to load user assignments');
      expect(state.canEdit()).toBe(false);
      expect(state.filteredAssignments()).toEqual([]);
      expect(state.roles()).toEqual([]);
    },
  );

  it.each([null, {}, [null], [roles[0], roles[0]]])(
    'reports a malformed role inventory as unknown: %j',
    async (value) => {
      mocks.getRoles.mockResolvedValueOnce(value);
      const { state } = mount();
      await loaded(state);
      expect(state.loadError()).toBe('Failed to load user assignments');
      expect(state.canEdit()).toBe(false);
      expect(state.filteredAssignments()).toEqual([]);
    },
  );

  it('distinguishes a healthy empty directory from a failed one and serialises Retry', async () => {
    mocks.getUsers.mockRejectedValueOnce(new Error('private directory failure'));
    const { state } = mount();
    await loaded(state);
    expect(state.loadError()).not.toBeNull();
    expect(JSON.stringify(mocks.log.mock.calls)).not.toContain('private directory failure');
    const retry = deferred<UserRoleAssignment[]>();
    mocks.getUsers.mockReturnValueOnce(retry.promise);
    const read = state.loadData();
    await state.loadData();
    expect(mocks.getUsers).toHaveBeenCalledTimes(2);
    retry.resolve([]);
    await read;
    expect(state.loadError()).toBeNull();
    expect(state.canEdit()).toBe(true);
    expect(state.filteredAssignments()).toEqual([]);
  });
  it('accepts the current datagrid store proxy without admitting an earlier-read proxy by principal alone', async () => {
    const { state } = mount();
    await loaded(state);
    const [row] = createStore(alice);
    await state.openManageAccess(row);
    expect(state.showModal()).toBe(true);
    state.closeModal();
    mocks.getUsers.mockResolvedValueOnce([{ ...alice }]);
    await state.loadData();
    await state.openManageAccess(row);
    state.openDeleteUser(row);
    expect(state.showModal()).toBe(false);
    expect(state.userPendingDeletion()).toBeNull();
  });
});
