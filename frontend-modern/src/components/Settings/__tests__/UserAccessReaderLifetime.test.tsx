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
let featureEnabled: () => boolean;
let featureReady: () => boolean;
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => featureReady(),
    rbacEnabled: () => featureReady() && featureEnabled(),
  }),
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
    featureEnabled = () => true;
    featureReady = () => true;
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

  function mountRefreshingReader() {
    const [status, setStatus] = createSignal({ user: 'operator-a', allowed: true });
    let state!: ReturnType<typeof useUserAssignmentsPanelState>;
    const view = render(() => {
      state = useUserAssignmentsPanelState({
        get currentUser() {
          return status().user;
        },
        get canManage() {
          return status().allowed;
        },
      });
      return <div />;
    });
    return { state, view, setStatus, refresh: () => setStatus((current) => ({ ...current })) };
  }

  it('retains a draft, search and accepted permissions when status refreshes for the same reader', async () => {
    const { state, refresh } = mountRefreshingReader();
    await loaded(state);
    await state.openManageAccess(alice);
    state.toggleRole('reader');
    state.setSearchQuery('private');
    const permissions = state.userPermissions();
    refresh();
    await pause();
    expect(state.showModal()).toBe(true);
    expect(state.editingUser()).toBe(alice);
    expect(state.formRoleIds()).toEqual([]);
    expect(state.searchQuery()).toBe('private');
    expect(state.userPermissions()).toBe(permissions);
    expect(mocks.getUsers).toHaveBeenCalledTimes(1);
    expect(mocks.getUserPermissions).toHaveBeenCalledTimes(1);
    expect(mocks.updateUserRoles).not.toHaveBeenCalled();
    expect(mocks.deleteUser).not.toHaveBeenCalled();
  });

  it('retains a pending saved-permissions read across an equal status refresh', async () => {
    const permissions = deferred<Permission[]>();
    mocks.getUserPermissions.mockReturnValueOnce(permissions.promise);
    const { state, refresh } = mountRefreshingReader();
    await loaded(state);
    const reading = state.openManageAccess(alice);
    const signal = mocks.getUserPermissions.mock.calls.at(-1)![1].signal;
    refresh();
    await pause();
    expect(signal.aborted).toBe(false);
    expect(state.loadingPermissions()).toBe(true);
    expect(state.showModal()).toBe(true);
    permissions.resolve([{ action: 'read', resource: 'north-private' }]);
    await reading;
    expect(state.userPermissions()).toEqual([{ action: 'read', resource: 'north-private' }]);
    expect(mocks.getUsers).toHaveBeenCalledTimes(1);
    expect(mocks.getUserPermissions).toHaveBeenCalledTimes(1);
  });

  it.each(['save', 'delete'] as const)(
    'retains a pending %s and its confirmed outcome across equal status refreshes',
    async (kind) => {
      const mutation = deferred<void>();
      const mutate = kind === 'save' ? mocks.updateUserRoles : mocks.deleteUser;
      mutate.mockReturnValueOnce(mutation.promise);
      const { state, refresh } = mountRefreshingReader();
      await loaded(state);
      if (kind === 'save') {
        await state.openManageAccess(alice);
        state.toggleRole('reader');
      } else state.openDeleteUser(alice);
      const writing = kind === 'save' ? state.handleSaveAssignments() : state.handleDeleteUser();
      const signal = mutate.mock.calls.at(-1)![kind === 'save' ? 2 : 1].signal;
      refresh();
      refresh();
      await pause();
      expect(signal.aborted).toBe(false);
      expect(state.canEdit()).toBe(false);
      expect(kind === 'save' ? state.saving() : state.deletingUser()).toBe(true);
      expect(kind === 'save' ? state.showModal() : !!state.userPendingDeletion()).toBe(true);
      expect(mocks.getUsers).toHaveBeenCalledTimes(1);
      await state.handleSaveAssignments();
      await state.handleDeleteUser();
      expect(mutate).toHaveBeenCalledTimes(1);
      mocks.getUsers.mockResolvedValueOnce(kind === 'save' ? [{ ...alice, roleIds: [] }] : []);
      mutation.resolve();
      await writing;
      expect(state.filteredAssignments()).toEqual(
        kind === 'save' ? [{ ...alice, roleIds: [] }] : [],
      );
      expect(state.successNotice()).toBe(
        kind === 'save'
          ? `Roles updated for ${alice.username}`
          : 'User access removed for North private user',
      );
      expect(state.showModal()).toBe(false);
      expect(state.userPendingDeletion()).toBeNull();
      expect(state.canEdit()).toBe(true);
      expect(mocks.getUsers).toHaveBeenCalledTimes(2);
      expect(mutate).toHaveBeenCalledTimes(1);
    },
  );

  it.each(['save', 'delete'] as const)(
    'retains an uncertain %s outcome and read-only recovery through equal status refreshes',
    async (kind) => {
      const mutation = deferred<void>();
      const mutate = kind === 'save' ? mocks.updateUserRoles : mocks.deleteUser;
      mutate.mockReturnValueOnce(mutation.promise);
      const { state, refresh } = mountRefreshingReader();
      await loaded(state);
      if (kind === 'save') await state.openManageAccess(alice);
      else state.openDeleteUser(alice);
      const writing = kind === 'save' ? state.handleSaveAssignments() : state.handleDeleteUser();
      refresh();
      await pause();
      mutation.reject(Object.assign(new Error('Synthetic lost reply'), { status: 503 }));
      await writing;
      const warning = state.unconfirmedChange();
      expect(warning).toContain('may already have been applied');
      refresh();
      await pause();
      expect(state.unconfirmedChange()).toBe(warning);
      expect(state.canEdit()).toBe(false);
      expect(state.filteredAssignments()).toEqual([]);
      expect(state.successNotice()).toBeNull();
      expect(mocks.getUsers).toHaveBeenCalledTimes(1);
      await state.handleSaveAssignments();
      await state.handleDeleteUser();
      expect(mutate).toHaveBeenCalledTimes(1);
      mocks.getUsers.mockResolvedValueOnce(kind === 'save' ? [{ ...alice, roleIds: [] }] : []);
      await state.loadData();
      expect(state.unconfirmedChange()).toBeNull();
      expect(state.canEdit()).toBe(true);
      expect(mocks.getUsers).toHaveBeenCalledTimes(2);
      expect(mutate).toHaveBeenCalledTimes(1);
    },
  );

  it('keeps a definitely refused save draft and error across an equal status refresh', async () => {
    mocks.updateUserRoles.mockRejectedValueOnce(
      Object.assign(new Error('Synthetic rejection'), { status: 403 }),
    );
    const { state, refresh } = mountRefreshingReader();
    await loaded(state);
    await state.openManageAccess(alice);
    state.toggleRole('reader');
    await state.handleSaveAssignments();
    const error = state.actionError();
    refresh();
    await pause();
    expect(state.actionError()).toBe(error);
    expect(error).not.toBeNull();
    expect(state.showModal()).toBe(true);
    expect(state.formRoleIds()).toEqual([]);
    expect(mocks.getUsers).toHaveBeenCalledTimes(1);
    expect(mocks.updateUserRoles).toHaveBeenCalledTimes(1);
  });

  it.each([
    'principal',
    'authority',
    'license',
    'readiness',
    'same-org-rebind',
    'org',
    'disposal',
  ] as const)(
    'still withdraws and ignores a pending mutation on %s after an equal refresh',
    async (kind) => {
      const [feature, setFeature] = createSignal(true);
      const [ready, setReady] = createSignal(true);
      featureEnabled = feature;
      featureReady = ready;
      const mutation = deferred<void>();
      mocks.updateUserRoles.mockReturnValueOnce(mutation.promise);
      const { state, setStatus, refresh, view } = mountRefreshingReader();
      await loaded(state);
      await state.openManageAccess(alice);
      const writing = state.handleSaveAssignments();
      const signal = mocks.updateUserRoles.mock.calls.at(-1)![2].signal;
      refresh();
      await pause();
      expect(signal.aborted).toBe(false);
      if (kind === 'principal') setStatus({ user: 'operator-b', allowed: true });
      else if (kind === 'authority') setStatus({ user: 'operator-a', allowed: false });
      else if (kind === 'license') setFeature(false);
      else if (kind === 'readiness') setReady(false);
      else if (kind === 'same-org-rebind') eventBus.emit('org_switched', 'north');
      else if (kind === 'org') switchOrg();
      else view.unmount();
      await pause();
      expect(signal.aborted).toBe(true);
      expect(state.showModal()).toBe(false);
      mutation.resolve();
      await writing;
      expect(state.successNotice()).toBeNull();
      expect(state.actionError()).toBeNull();
      expect(state.unconfirmedChange()).toBeNull();
      expect(mocks.updateUserRoles).toHaveBeenCalledTimes(1);
    },
  );

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
    mocks.updateUserRoles.mockRejectedValueOnce(
      Object.assign(new Error('private response canary'), { status: 403 }),
    );
    const { state } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    state.toggleRole('reader');
    await state.handleSaveAssignments();
    expect(state.showModal()).toBe(true);
    expect(state.formRoleIds()).toEqual([]);
    expect(state.actionError()).toBe('Failed to update user roles');
    expect(mocks.error).not.toHaveBeenCalled();
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

  it.each(['success', 'failure'])(
    'keeps a reopened same-row editor independent of its obsolete permissions %s',
    async (outcome) => {
      const old = deferred<Permission[]>();
      mocks.getUserPermissions.mockReturnValueOnce(old.promise);
      const { state } = mount();
      await loaded(state);
      const earlierEditor = state.openManageAccess(alice);
      const earlierSignal = mocks.getUserPermissions.mock.calls.at(-1)![1].signal;
      state.closeModal();
      expect(earlierSignal.aborted).toBe(true);
      await state.openManageAccess(alice);
      state.toggleRole('reader');
      const currentPermissions = state.userPermissions();
      if (outcome === 'success') old.resolve([{ action: 'read', resource: 'obsolete' }]);
      else old.reject(new Error('obsolete private failure'));
      await earlierEditor;
      expect(state.editingUser()).toBe(alice);
      expect(state.showModal()).toBe(true);
      expect(state.userPermissions()).toBe(currentPermissions);
      expect(state.permissionsError()).toBeNull();
      expect(state.formRoleIds()).toEqual([]);
      expect(state.loadingPermissions()).toBe(false);
    },
  );

  it('does not announce a save whose refresh is overtaken by a return to the same reader', async () => {
    const { state } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    const oldRefresh = deferred<UserRoleAssignment[]>();
    mocks.getUsers.mockReturnValueOnce(oldRefresh.promise);
    const save = state.handleSaveAssignments();
    await waitFor(() => expect(mocks.getUsers).toHaveBeenCalledTimes(2));
    // Same organisation and principal, but a new reader generation.
    eventBus.emit('org_switched', 'north');
    await loaded(state);
    const fresh = state.filteredAssignments();
    oldRefresh.resolve([{ ...alice, displayName: 'Obsolete refresh' }]);
    await save;
    expect(state.filteredAssignments()).toBe(fresh);
    expect(state.successNotice()).toBeNull();
    expect(state.loading()).toBe(false);
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
  it('owns successful identity feedback locally and withdraws it on context change', async () => {
    const { state } = mount();
    await loaded(state);
    await state.openManageAccess(alice);
    await state.handleSaveAssignments();
    expect(state.successNotice()).toBe(`Roles updated for ${alice.username}`);
    expect(mocks.success).not.toHaveBeenCalled();
    switchOrg();
    await loaded(state);
    expect(state.successNotice()).toBeNull();
    expect(state.actionError()).toBeNull();
  });

  it('withdraws failed-action feedback before another confirmation and when authority changes', async () => {
    mocks.deleteUser.mockRejectedValueOnce(
      Object.assign(new Error('private deletion details'), { status: 403 }),
    );
    const { state, setAllowed } = mount();
    await loaded(state);
    state.openDeleteUser(alice);
    await state.handleDeleteUser();
    expect(state.actionError()).not.toBeNull();
    expect(state.userPendingDeletion()).toEqual(alice);
    expect(mocks.error).not.toHaveBeenCalled();
    state.closeDeleteUser();
    await state.openManageAccess(alice);
    expect(state.actionError()).toBeNull();
    setAllowed(false);
    await pause();
    expect(state.successNotice()).toBeNull();
    expect(state.actionError()).toBeNull();
  });

  it.each([
    ['save', undefined],
    ['save', 502],
    ['save', 200],
    ['delete', undefined],
    ['delete', 502],
    ['delete', 200],
  ] as const)(
    'withdraws an unconfirmed %s (status %s) until a read-only check succeeds',
    async (kind, status) => {
      const mutate = kind === 'save' ? mocks.updateUserRoles : mocks.deleteUser;
      // The server commits the change, but its response is lost, fails, or cannot be read.
      mutate.mockImplementationOnce(async () => {
        mocks.getUsers.mockResolvedValue(kind === 'save' ? [{ ...alice, roleIds: [] }] : []);
        throw Object.assign(new Error('private unconfirmed response'), { status });
      });
      const { state } = mount();
      await loaded(state);
      const staleRow = state.filteredAssignments()[0];
      if (kind === 'save') {
        await state.openManageAccess(staleRow);
        state.toggleRole('reader');
        await state.handleSaveAssignments();
      } else {
        state.openDeleteUser(staleRow);
        await state.handleDeleteUser();
      }
      expect(state.filteredAssignments()).toEqual([]);
      expect(state.canEdit()).toBe(false);
      expect(state.unconfirmedChange()).toContain('may already have been applied');
      expect(state.roles()).toEqual([]);
      expect(state.formRoleIds()).toEqual([]);
      expect(state.userPermissions()).toBeNull();
      expect(state.editingUser()).toBeNull();
      expect(state.userPendingDeletion()).toBeNull();
      expect(state.showModal()).toBe(false);
      expect(state.actionError()).toBeNull();
      expect(state.successNotice()).toBeNull();
      expect(mocks.getUsers).toHaveBeenCalledTimes(1);
      // Neither retained callbacks nor a saved-permissions retry can replay the write.
      await state.openManageAccess(staleRow);
      state.openDeleteUser(staleRow);
      state.toggleRole('reader');
      await state.handleSaveAssignments();
      await state.handleDeleteUser();
      await state.loadUserPermissions();
      expect(mutate).toHaveBeenCalledTimes(1);
      expect(mocks.updateUserRoles.mock.calls.length + mocks.deleteUser.mock.calls.length).toBe(1);
      await state.loadData();
      expect(state.unconfirmedChange()).toBeNull();
      expect(state.canEdit()).toBe(true);
      expect(state.filteredAssignments()).toEqual(
        kind === 'save' ? [{ ...alice, roleIds: [] }] : [],
      );
      await state.openManageAccess(staleRow);
      state.openDeleteUser(staleRow);
      expect(state.showModal()).toBe(false);
      expect(state.userPendingDeletion()).toBeNull();
      expect(mocks.updateUserRoles.mock.calls.length + mocks.deleteUser.mock.calls.length).toBe(1);
      expect(mocks.error).not.toHaveBeenCalled();
      expect(mocks.success).not.toHaveBeenCalled();
      expect(mocks.log).not.toHaveBeenCalled();
    },
  );

  it.each(['failed users', 'malformed users', 'failed roles', 'malformed roles'])(
    'keeps the uncertainty after a %s recovery check and serialises checks',
    async (failure) => {
      mocks.updateUserRoles.mockRejectedValueOnce(new Error('lost reply'));
      const { state } = mount();
      await loaded(state);
      await state.openManageAccess(alice);
      await state.handleSaveAssignments();
      const warning = state.unconfirmedChange();
      const checking = deferred<UserRoleAssignment[]>();
      mocks.getUsers.mockReturnValueOnce(checking.promise);
      if (failure === 'failed roles') mocks.getRoles.mockRejectedValueOnce(new Error('lost read'));
      if (failure === 'malformed roles') mocks.getRoles.mockResolvedValueOnce({});
      const read = state.loadData();
      await state.loadData();
      expect(mocks.getUsers).toHaveBeenCalledTimes(2);
      expect(state.unconfirmedChange()).toBe(warning);
      expect(state.canEdit()).toBe(false);
      if (failure === 'failed users') checking.reject(new Error('lost read'));
      else checking.resolve(failure === 'malformed users' ? ({} as UserRoleAssignment[]) : [alice]);
      await read;
      expect(state.loadError()).not.toBeNull();
      expect(state.unconfirmedChange()).toBe(warning);
      expect(state.filteredAssignments()).toEqual([]);
      expect(state.roles()).toEqual([]);
      expect(state.canEdit()).toBe(false);
      await state.loadData();
      expect(state.unconfirmedChange()).toBeNull();
      expect(state.loadError()).toBeNull();
      expect(state.canEdit()).toBe(true);
      expect(mocks.updateUserRoles).toHaveBeenCalledTimes(1);
    },
  );

  it('aborts and ignores delayed saved permissions after an unconfirmed save', async () => {
    const permissions = deferred<Permission[]>();
    mocks.getUserPermissions.mockReturnValueOnce(permissions.promise);
    mocks.updateUserRoles.mockRejectedValueOnce(new Error('lost reply'));
    const { state } = mount();
    await loaded(state);
    const reading = state.openManageAccess(alice);
    const signal = mocks.getUserPermissions.mock.calls.at(-1)![1].signal;
    await state.handleSaveAssignments();
    expect(signal.aborted).toBe(true);
    permissions.resolve([{ action: '*', resource: 'obsolete saved permissions' }]);
    await reading;
    expect(state.unconfirmedChange()).not.toBeNull();
    expect(state.userPermissions()).toBeNull();
    expect(state.showModal()).toBe(false);
  });

  it('withdraws uncertainty and ignores its delayed recovery read on reader replacement', async () => {
    mocks.deleteUser.mockRejectedValueOnce(new Error('lost reply'));
    const { state } = mount();
    await loaded(state);
    state.openDeleteUser(alice);
    await state.handleDeleteUser();
    expect(state.unconfirmedChange()).not.toBeNull();
    const old = deferred<UserRoleAssignment[]>();
    mocks.getUsers.mockReturnValueOnce(old.promise);
    const checking = state.loadData();
    const oldSignal = mocks.getUsers.mock.calls.at(-1)![0].signal;
    mocks.getUsers.mockResolvedValueOnce([bob]);
    switchOrg();
    await loaded(state);
    expect(oldSignal.aborted).toBe(true);
    expect(state.unconfirmedChange()).toBeNull();
    expect(state.filteredAssignments()).toEqual([bob]);
    old.resolve([alice]);
    await checking;
    expect(state.filteredAssignments()).toEqual([bob]);
    expect(state.loadError()).toBeNull();
    expect(state.unconfirmedChange()).toBeNull();
  });

  it.each([400, 401, 403, 404, 409, 422, 429])(
    'retains a definitely rejected removal (%s) for correction without an uncertainty claim',
    async (status) => {
      mocks.deleteUser.mockRejectedValueOnce(
        Object.assign(new Error('private rejection'), { status, code: 'self_deprovision_denied' }),
      );
      const { state } = mount();
      await loaded(state);
      state.openDeleteUser(alice);
      await state.handleDeleteUser();
      expect(state.userPendingDeletion()).toBe(alice);
      expect(state.filteredAssignments()).toEqual([alice]);
      expect(state.canEdit()).toBe(true);
      expect(state.unconfirmedChange()).toBeNull();
      expect(state.actionError()).toBe(
        status === 403 ? 'You cannot remove your own user access.' : 'Failed to remove user access',
      );
      expect(mocks.getUsers).toHaveBeenCalledTimes(1);
      expect(mocks.deleteUser).toHaveBeenCalledTimes(1);
    },
  );
});
