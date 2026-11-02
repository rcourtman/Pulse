import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal, ErrorBoundary, Show } from 'solid-js';
import { createStore } from 'solid-js/store';
import { eventBus } from '@/stores/events';
import { useRolesPanelState } from '../useRolesPanelState';
import { RolesPanel } from '../RolesPanel';
import type { Role } from '@/types/rbac';

const mocks = vi.hoisted(() => ({
  org: 'north',
  getRoles: vi.fn(),
  saveRole: vi.fn(),
  deleteRole: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  log: vi.fn(),
}));
vi.mock('@/utils/apiClient', () => ({ getOrgID: () => mocks.org }));
vi.mock('@/api/rbac', () => ({ RBACAPI: mocks }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: mocks.success, error: mocks.error },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: mocks.log, debug: vi.fn(), warn: vi.fn() } }));
let enabled: () => boolean;
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => enabled(),
    paywallVisible: () => false,
  }),
}));
const north: Role = {
  id: 'custom/reader',
  name: 'North private role',
  description: 'North private description',
  permissions: [{ action: 'read', resource: 'north-private' }],
  createdAt: '2026-01-01',
};
const south: Role = {
  ...north,
  name: 'South role',
  description: 'South description',
  permissions: [],
};
const pause = () => new Promise((resolve) => setTimeout(resolve, 0));
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { resolve, reject, promise };
}
function mount() {
  const [user, setUser] = createSignal<string | undefined>('reader-a');
  const [allowed, setAllowed] = createSignal(true);
  let state!: ReturnType<typeof useRolesPanelState>;
  const view = render(() => {
    state = useRolesPanelState({
      get currentUser() {
        return user();
      },
      get canManage() {
        return allowed();
      },
    });
    return <button disabled={!state.canEdit()}>Current reader action</button>;
  });
  return { state, setUser, setAllowed, view };
}
const loaded = (state: ReturnType<typeof useRolesPanelState>) =>
  waitFor(() => expect(state.loading()).toBe(false));
function switchOrg(org = 'south') {
  mocks.org = org;
  eventBus.emit('org_switched', org);
}

describe('Roles reader lifetime', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    enabled = () => true;
    mocks.org = 'north';
    mocks.getRoles.mockReset().mockResolvedValue([north]);
    mocks.saveRole.mockReset().mockResolvedValue(undefined);
    mocks.deleteRole.mockReset().mockResolvedValue(undefined);
  });
  afterEach(cleanup);

  it('withdraws roles, editor and draft on an organisation switch, even for the same role ID', async () => {
    const { state } = mount();
    await loaded(state);
    state.openEditRole(north);
    state.setFormName('Unsaved private draft');
    const next = deferred<Role[]>();
    mocks.getRoles.mockReturnValueOnce(next.promise);
    switchOrg();
    expect(state.roles()).toEqual([]);
    expect(state.showModal()).toBe(false);
    expect(state.editingRole()).toBeNull();
    expect(state.formName()).toBe('');
    expect(state.formPermissions()).toEqual([]);
    expect(state.loading()).toBe(true);
    expect(state.canEdit()).toBe(false);
    next.resolve([south]);
    await loaded(state);
    state.openEditRole(north);
    expect(state.showModal()).toBe(false);
    state.openEditRole(south);
    expect(state.formName()).toBe('South role');
    expect(screen.getByRole('button', { name: 'Current reader action' })).not.toBeDisabled();
  });

  it.each(['success', 'failure'] as const)(
    'ignores an obsolete %s after switching away and back',
    async (outcome) => {
      const old = deferred<Role[]>();
      mocks.getRoles.mockReturnValueOnce(old.promise);
      const { state } = mount();
      await pause();
      const options = mocks.getRoles.mock.calls[0][0];
      switchOrg();
      await loaded(state);
      switchOrg('north');
      await loaded(state);
      if (outcome === 'success') old.resolve([south]);
      else old.reject(new Error('PRIVATE OBSOLETE RESPONSE'));
      await pause();
      expect(options.signal.aborted).toBe(true);
      expect(state.roles()).toEqual([north]);
      expect(state.loadError()).toBeNull();
      expect(mocks.log).not.toHaveBeenCalled();
      expect(mocks.error).not.toHaveBeenCalled();
    },
  );

  it('reactivates rendered controls after capability withdrawal and restoration', async () => {
    const [feature, setFeature] = createSignal(true);
    enabled = feature;
    const { state } = mount();
    await loaded(state);
    expect(screen.getByRole('button', { name: 'Current reader action' })).not.toBeDisabled();
    setFeature(false);
    expect(screen.getByRole('button', { name: 'Current reader action' })).toBeDisabled();
    setFeature(true);
    await loaded(state);
    expect(screen.getByRole('button', { name: 'Current reader action' })).not.toBeDisabled();
  });

  it('does not let an old completion finish a newer loading state', async () => {
    const old = deferred<Role[]>(),
      current = deferred<Role[]>();
    mocks.getRoles.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise);
    const { state } = mount();
    await pause();
    switchOrg();
    old.resolve([north]);
    await pause();
    expect(state.loading()).toBe(true);
    expect(state.roles()).toEqual([]);
    current.resolve([south]);
    await loaded(state);
    expect(state.roles()).toEqual([south]);
  });

  it.each(['authority', 'license', 'principal'] as const)(
    'withdraws confirmation and refuses stale actions on %s change',
    async (kind) => {
      const [feature, setFeature] = createSignal(true);
      enabled = feature;
      const { state, setUser, setAllowed } = mount();
      await loaded(state);
      state.openDeleteRole(north);
      expect(state.rolePendingDeletion()).toBe(north);
      if (kind === 'authority') setAllowed(false);
      if (kind === 'license') setFeature(false);
      if (kind === 'principal') setUser('reader-b');
      await state.handleDeleteRole();
      await state.handleSaveRole();
      expect(state.rolePendingDeletion()).toBeNull();
      expect(state.showModal()).toBe(false);
      expect(mocks.deleteRole).not.toHaveBeenCalled();
      expect(mocks.saveRole).not.toHaveBeenCalled();
      if (kind !== 'principal') {
        expect(state.roles()).toEqual([]);
        expect(state.loading()).toBe(false);
      }
    },
  );

  it.each(['save', 'delete'] as const)(
    'ignores old %s feedback and refresh after a principal changes',
    async (kind) => {
      const old = deferred<void>();
      const write = kind === 'save' ? mocks.saveRole : mocks.deleteRole;
      write.mockReturnValueOnce(old.promise);
      const { state, setUser } = mount();
      await loaded(state);
      state.openEditRole(north);
      if (kind === 'delete') state.openDeleteRole(north);
      const pending = kind === 'save' ? state.handleSaveRole() : state.handleDeleteRole();
      expect(write).toHaveBeenCalledTimes(1);
      const options = write.mock.calls[0][1];
      setUser('reader-b');
      await loaded(state);
      const reads = mocks.getRoles.mock.calls.length;
      old.resolve();
      await pending;
      expect(options.signal.aborted).toBe(true);
      expect(options.expectedOrgID).toBe('north');
      expect(options.retry).toBe(false);
      expect(state.successNotice()).toBeNull();
      expect(state.actionError()).toBeNull();
      expect(mocks.getRoles).toHaveBeenCalledTimes(reads);
      expect(mocks.success).not.toHaveBeenCalled();
    },
  );

  it('keeps a denied save and draft local, serialises writes and permits only explicit retry', async () => {
    const old = deferred<void>();
    mocks.saveRole.mockReturnValueOnce(old.promise);
    const { state } = mount();
    await loaded(state);
    state.openEditRole(north);
    state.setFormName('Changed role');
    const pending = state.handleSaveRole();
    await state.handleSaveRole();
    state.closeModal();
    state.openDeleteRole(north);
    expect(state.showModal()).toBe(true);
    expect(state.rolePendingDeletion()).toBeNull();
    expect(mocks.saveRole).toHaveBeenCalledTimes(1);
    old.reject(new Error('PRIVATE WRITE DETAIL'));
    await pending;
    expect(state.formName()).toBe('Changed role');
    expect(state.actionError()).toBe('Failed to save role');
    expect(mocks.error).not.toHaveBeenCalled();
    await state.handleSaveRole();
    expect(mocks.saveRole).toHaveBeenCalledTimes(2);
    expect(mocks.saveRole.mock.calls[1][0]).toEqual(mocks.saveRole.mock.calls[0][0]);
    expect(state.showModal()).toBe(false);
    expect(state.formName()).toBe('');
    expect(state.successNotice()).toBe('Role "Changed role" saved');
    switchOrg();
    expect(state.successNotice()).toBeNull();
  });

  it('keeps delete failure in the live confirmation and never retries it on a read', async () => {
    mocks.deleteRole.mockRejectedValueOnce(new Error('PRIVATE DELETE DETAIL'));
    const { state } = mount();
    await loaded(state);
    state.openDeleteRole(north);
    await state.handleDeleteRole();
    expect(state.actionError()).toBe('Failed to delete role');
    expect(state.rolePendingDeletion()).toBe(north);
    state.closeDeleteRole();
    await state.loadRoles();
    expect(mocks.deleteRole).toHaveBeenCalledTimes(1);
    expect(state.actionError()).toBeNull();
    expect(mocks.error).not.toHaveBeenCalled();
  });

  it('requires accepted inventory and protects built-ins and stale row identities', async () => {
    const pending = deferred<Role[]>();
    mocks.getRoles.mockReturnValueOnce(pending.promise);
    const { state } = mount();
    state.openCreateRole();
    state.openEditRole(north);
    expect(state.showModal()).toBe(false);
    const builtIn = { ...north, isBuiltIn: true };
    pending.resolve([builtIn]);
    await loaded(state);
    state.openEditRole(builtIn);
    state.openDeleteRole(builtIn);
    state.openEditRole({ ...builtIn, isBuiltIn: false });
    expect(state.showModal()).toBe(false);
    expect(state.rolePendingDeletion()).toBeNull();
    await state.handleSaveRole();
    await state.handleDeleteRole();
    expect(mocks.saveRole).not.toHaveBeenCalled();
    expect(mocks.deleteRole).not.toHaveBeenCalled();
  });

  it.each([
    null,
    {},
    [null],
    [{ ...north, permissions: null }],
    [{ ...north, permissions: [{}] }],
    [north, north],
  ])('treats malformed inventory as unknown, not an empty directory: %j', async (value) => {
    mocks.getRoles.mockResolvedValueOnce(value);
    const { state } = mount();
    await loaded(state);
    expect(state.roles()).toEqual([]);
    expect(state.loadError()).toBe('Failed to load roles');
    expect(state.canEdit()).toBe(false);
    await state.loadRoles();
    expect(state.roles()).toEqual([north]);
    expect(state.canEdit()).toBe(true);
    expect(mocks.saveRole).not.toHaveBeenCalled();
  });

  it('accepts empty permission lists and future permission strings without changing them', async () => {
    const role = { ...north, permissions: [{ action: 'future:read', resource: 'future/*' }] };
    mocks.getRoles.mockResolvedValueOnce([role]);
    const { state } = mount();
    await loaded(state);
    state.openEditRole(role);
    await state.handleSaveRole();
    expect(mocks.saveRole.mock.calls[0][0].permissions).toEqual(role.permissions);
    mocks.getRoles.mockResolvedValueOnce([]);
    await state.loadRoles();
    expect(state.loadError()).toBeNull();
    expect(state.canEdit()).toBe(true);
  });

  it('accepts reactive current rows without allowing draft changes to mutate their permissions', async () => {
    const [role] = createStore(north);
    mocks.getRoles.mockResolvedValueOnce([role]);
    const { state } = mount();
    await loaded(state);
    state.openEditRole(role);
    state.updatePermission(0, 'action', 'write');
    expect(north.permissions[0].action).toBe('read');
    expect(state.formPermissions()[0].action).toBe('write');
    state.closeModal();
    expect(state.formPermissions()).toEqual([]);
  });

  it('aborts pending reads and writes and unsubscribes on disposal without feedback', async () => {
    const pending = deferred<void>();
    mocks.saveRole.mockReturnValueOnce(pending.promise);
    const { state, view } = mount();
    await loaded(state);
    state.openEditRole(north);
    const writing = state.handleSaveRole();
    const options = mocks.saveRole.mock.calls[0][1];
    const reads = mocks.getRoles.mock.calls.length;
    view.unmount();
    switchOrg();
    pending.reject(new Error('obsolete write'));
    await writing;
    expect(options.signal.aborted).toBe(true);
    expect(state.formName()).toBe('');
    expect(mocks.getRoles).toHaveBeenCalledTimes(reads);
    expect(mocks.error).not.toHaveBeenCalled();
  });

  it('disposes an open confirmation without reading a stale Show accessor', async () => {
    const [visible, setVisible] = createSignal(true);
    const errors = vi.fn();
    render(() => (
      <ErrorBoundary
        fallback={(error) => {
          errors(error);
          return <p>Boundary failure</p>;
        }}
      >
        <Show when={visible()}>
          <RolesPanel currentUser="reader-a" canManage={visible()} />
        </Show>
      </ErrorBoundary>
    ));
    fireEvent.click(await screen.findByRole('button', { name: 'Delete role' }));
    expect(
      await screen.findByRole('dialog', { name: 'Delete role: North private role' }),
    ).toBeInTheDocument();
    setVisible(false);
    await pause();
    expect(errors).not.toHaveBeenCalled();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(mocks.deleteRole).not.toHaveBeenCalled();
  });

  it('keeps unlisted saved permission strings visible and unchanged in the real editor', async () => {
    mocks.getRoles.mockResolvedValueOnce([
      { ...north, permissions: [{ action: 'future:read', resource: 'future/*' }] },
    ]);
    render(() => <RolesPanel currentUser="reader-a" canManage={true} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Edit role' }));
    const dialog = await screen.findByRole('dialog', { name: 'Edit role' });
    expect(within(dialog).getByLabelText('Permission 1 action')).toHaveValue('future:read');
    expect(within(dialog).getByLabelText('Permission 1 resource')).toHaveValue('future/*');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Update Role' }));
    await waitFor(() => expect(mocks.saveRole).toHaveBeenCalled());
    expect(mocks.saveRole.mock.calls[0][0].permissions).toEqual([
      { action: 'future:read', resource: 'future/*' },
    ]);
  });

  it('shows inline draft errors and a cancellable, withdrawable confirmation in the consuming panel', async () => {
    const [allowed, setAllowed] = createSignal(true);
    mocks.saveRole.mockRejectedValueOnce(new Error('PRIVATE DETAIL'));
    render(() => <RolesPanel currentUser="reader-a" canManage={allowed()} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Edit role' }));
    let dialog = await screen.findByRole('dialog', { name: 'Edit role' });
    fireEvent.input(within(dialog).getByLabelText('Role Name'), {
      target: { value: 'Private draft' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Update Role' }));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Failed to save role');
    expect(within(dialog).getByLabelText('Role Name')).toHaveValue('Private draft');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    fireEvent.click(screen.getByRole('button', { name: 'Delete role' }));
    dialog = await screen.findByRole('dialog', { name: 'Delete role: North private role' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(mocks.deleteRole).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Delete role' }));
    setAllowed(false);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.queryByText('North private role')).not.toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('Roles are unavailable for this session.');
    expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
  });
});
