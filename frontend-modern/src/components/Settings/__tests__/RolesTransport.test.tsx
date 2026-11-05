import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { RolesPanel } from '../RolesPanel';
import { useRolesPanelState } from '../useRolesPanelState';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import type { Role } from '@/types/rbac';

// Entitlement is the only product seam replaced here. Keep the role state,
// RBACAPI, JSON/error parsing and authenticated API client connected; their
// isolated suites cannot detect a safety option lost between these layers.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));

const originalRole = {
  id: 'custom/reader:production',
  name: 'Production reader',
  description: 'Saved constrained access',
  parentId: 'viewer',
  priority: 17,
  createdAt: '2026-10-01T09:00:00Z',
  updatedAt: '2026-11-01T09:00:00Z',
  permissions: [
    { action: 'read', resource: 'nodes' },
    {
      action: 'delete',
      resource: 'nodes',
      effect: 'deny',
      conditions: { tag: 'production', owner: '${user}' },
    },
  ],
};
const roleURL = `/api/admin/roles/${encodeURIComponent(originalRole.id)}`;
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
const deferred = () => {
  let resolve!: (value: Response) => void;
  const promise = new Promise<Response>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
};
const fetchMock = vi.fn<(url: string, init: RequestInit) => Promise<Response>>();
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => ['POST', 'PUT', 'DELETE'].includes(init.method ?? ''));
const reads = () => fetchMock.mock.calls.filter(([url]) => url === '/api/admin/roles');
const cloneRole = (): Role => structuredClone(originalRole);
let savedRoles: Role[];
let writeResult: (() => Promise<Response>) | undefined;
let csrfPreparation: ReturnType<typeof deferred> | undefined;

function mount() {
  const [user, setUser] = createSignal('reader-a');
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
    return <div />;
  });
  return { state, view, setUser, setAllowed };
}
type Operation = 'create' | 'edit' | 'delete';
const operations: Operation[] = ['create', 'edit', 'delete'];
function prepare(state: ReturnType<typeof useRolesPanelState>, operation: Operation) {
  if (operation === 'delete') state.openDeleteRole(state.roles()[0]);
  else if (operation === 'edit') {
    state.openEditRole(state.roles()[0]);
    state.setFormName('Renamed constrained reader');
  } else {
    state.openCreateRole();
    state.setFormId('  New Reader  ');
    state.setFormName('New reader');
  }
}
const submit = (state: ReturnType<typeof useRolesPanelState>, operation: Operation) =>
  operation === 'delete' ? state.handleDeleteRole() : state.handleSaveRole();
const loaded = (state: ReturnType<typeof useRolesPanelState>) =>
  waitFor(() => expect(state.canEdit()).toBe(true));

beforeEach(() => {
  clearAuth();
  window.sessionStorage.clear();
  setOrgID('estate/north');
  setApiToken('synthetic-role-reader-token');
  document.cookie = 'pulse_csrf=synthetic-role-csrf; Path=/';
  savedRoles = [cloneRole()];
  writeResult = undefined;
  csrfPreparation = undefined;
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url, init) => {
    if (url === '/api/health' && csrfPreparation) return csrfPreparation.promise;
    if (url === '/api/admin/roles' && !init.method) return json(savedRoles);
    if (url === '/api/admin/roles' && init.method === 'POST') {
      if (writeResult) return writeResult();
      const created = JSON.parse(String(init.body)) as Role;
      savedRoles.push(created);
      return json(created);
    }
    if (url === roleURL && ['PUT', 'DELETE'].includes(init.method ?? '')) {
      if (writeResult) return writeResult();
      savedRoles = init.method === 'DELETE' ? [] : [JSON.parse(String(init.body)) as Role];
      return init.method === 'DELETE' ? new Response(null, { status: 204 }) : json(savedRoles[0]);
    }
    throw new Error(`Unexpected synthetic role request: ${init.method ?? 'GET'} ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  cleanup();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  document.cookie = 'pulse_org_id=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('role state through the real RBAC and authenticated clients', () => {
  it('serializes the complete saved deny policy and identity on an encoded, org-bound PUT', async () => {
    const { state } = mount();
    await loaded(state);
    prepare(state, 'edit');
    await submit(state, 'edit');

    expect(writes()).toHaveLength(1);
    const [url, init] = writes()[0];
    expect(url).toBe(roleURL);
    expect(init).toMatchObject({
      method: 'PUT',
      credentials: 'include',
      signal: expect.any(AbortSignal),
      headers: {
        'Content-Type': 'application/json',
        'X-Pulse-Org-ID': 'estate/north',
        'X-API-Token': 'synthetic-role-reader-token',
        'X-CSRF-Token': 'synthetic-role-csrf',
      },
    });
    const expected = { ...originalRole, name: 'Renamed constrained reader' };
    expect(JSON.parse(String(init.body))).toEqual(expected);
    expect(state.roles()).toEqual([expected]);
    expect(reads().filter(([, options]) => !options.method)).toHaveLength(2);
    expect(state.showModal()).toBe(false);
  });

  it('creates only the new draft after closing a saved-role edit, without inheriting its metadata', async () => {
    const { state } = mount();
    await loaded(state);
    prepare(state, 'edit');
    state.closeModal();
    prepare(state, 'create');
    await submit(state, 'create');
    expect(writes()).toHaveLength(1);
    const [url, init] = writes()[0];
    expect(url).toBe('/api/admin/roles');
    expect(init.method).toBe('POST');
    expect(JSON.parse(String(init.body))).toEqual({
      id: 'new-reader',
      name: 'New reader',
      description: '',
      permissions: [{ action: 'read', resource: 'nodes' }],
    });
    expect(state.roles()).toHaveLength(2);
  });

  it('requires a current confirmation before one encoded DELETE, then reads the saved inventory', async () => {
    const { state } = mount();
    await loaded(state);
    await submit(state, 'delete');
    expect(writes()).toHaveLength(0);
    prepare(state, 'delete');
    state.closeDeleteRole();
    await submit(state, 'delete');
    expect(writes()).toHaveLength(0);
    prepare(state, 'delete');
    await submit(state, 'delete');
    expect(writes()).toHaveLength(1);
    expect(writes()[0]).toEqual([
      roleURL,
      expect.objectContaining({
        method: 'DELETE',
        credentials: 'include',
        signal: expect.any(AbortSignal),
        headers: expect.objectContaining({ 'X-Pulse-Org-ID': 'estate/north' }),
      }),
    ]);
    expect(writes()[0][1].body).toBeUndefined();
    expect(state.roles()).toEqual([]);
    expect(state.rolePendingDeletion()).toBeNull();
    expect(reads().filter(([, options]) => !options.method)).toHaveLength(2);
  });

  it.each(operations)(
    'does not dispatch %s when org changes during real CSRF preparation',
    async (operation) => {
      const { state } = mount();
      await loaded(state);
      prepare(state, operation);
      vi.stubEnv('MODE', 'production');
      document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
      csrfPreparation = deferred();
      const saving = submit(state, operation);
      await waitFor(() =>
        expect(fetchMock.mock.calls.some(([url]) => url === '/api/health')).toBe(true),
      );
      // Deliberately do not emit the UI event: the transport must independently
      // recheck the admitted organisation after its asynchronous preparation.
      setOrgID('estate/south');
      document.cookie = 'pulse_csrf=prepared-csrf; Path=/';
      csrfPreparation.resolve(json({}));
      await saving;
      expect(writes()).toHaveLength(0);
      expect(savedRoles).toEqual([originalRole]);
      expect(getOrgID()).toBe('estate/south');
      expect(state.successNotice()).toBeNull();
      eventBus.emit('org_switched', 'estate/south');
      await loaded(state);
      expect(fetchMock.mock.calls.at(-1)?.[1].headers).toMatchObject({
        'X-Pulse-Org-ID': 'estate/south',
      });
    },
  );

  it.each(
    operations.flatMap((operation) =>
      ['user', 'permission', 'unmount'].map((change) => ({ operation, change })),
    ),
  )(
    'cancels $operation before dispatch when $change withdraws its reader during CSRF preparation',
    async ({ operation, change }) => {
      const { state, view, setUser, setAllowed } = mount();
      await loaded(state);
      prepare(state, operation);
      vi.stubEnv('MODE', 'production');
      document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
      csrfPreparation = deferred();
      const saving = submit(state, operation);
      await waitFor(() =>
        expect(fetchMock.mock.calls.some(([url]) => url === '/api/health')).toBe(true),
      );
      if (change === 'user') setUser('reader-b');
      else if (change === 'permission') setAllowed(false);
      else view.unmount();
      document.cookie = 'pulse_csrf=prepared-csrf; Path=/';
      csrfPreparation.resolve(json({}));
      await saving;
      expect(writes()).toHaveLength(0);
      expect(savedRoles).toEqual([originalRole]);
      expect(state.successNotice()).toBeNull();
      expect(state.showModal()).toBe(false);
      expect(state.rolePendingDeletion()).toBeNull();
    },
  );

  it.each(operations.flatMap((operation) => [403, 429].map((status) => ({ operation, status }))))(
    'does not replay $operation after HTTP $status, even with refreshed CSRF and Retry-After',
    async ({ operation, status }) => {
      writeResult = async () =>
        new Response('{"error":"synthetic refusal"}', {
          status,
          headers: { 'X-CSRF-Token': 'replacement-csrf', 'Retry-After': '0.001' },
        });
      const { state } = mount();
      await loaded(state);
      prepare(state, operation);
      await submit(state, operation);
      expect(writes()).toHaveLength(1);
      expect(savedRoles).toEqual([originalRole]);
      expect(state.actionError()).toBe(
        operation === 'delete' ? 'Failed to delete role' : 'Failed to save role',
      );
      expect(state.successNotice()).toBeNull();
      expect(state.unconfirmedChange()).toBeNull();
      expect(getOrgID()).toBe('estate/north');
      if (operation !== 'delete') {
        expect(state.showModal()).toBe(true);
        expect(state.formName()).toBe(
          operation === 'edit' ? 'Renamed constrained reader' : 'New reader',
        );
      } else expect(state.rolePendingDeletion()?.id).toBe(originalRole.id);
    },
  );

  it.each(['server-error', 'lost-response', 'unreadable-success'] as const)(
    'requires an explicit saved-role read after an applied edit with %s, never a blind write retry',
    async (outcome) => {
      writeResult = async () => {
        savedRoles = [{ ...cloneRole(), name: 'Renamed constrained reader' }];
        if (outcome === 'lost-response') throw new Error('Synthetic lost response');
        if (outcome === 'unreadable-success') return new Response('{"partial":');
        return json({ error: 'Synthetic upstream failure' }, 503);
      };
      render(() => <RolesPanel currentUser="reader-a" canManage />);
      fireEvent.click((await screen.findAllByRole('button', { name: 'Edit role' }))[0]);
      const editor = within(screen.getByRole('dialog', { name: 'Edit role' }));
      fireEvent.input(editor.getByLabelText('Role Name'), {
        target: { value: 'Renamed constrained reader' },
      });
      fireEvent.click(editor.getByRole('button', { name: 'Update Role' }));
      const check = await screen.findByRole('button', { name: 'Check saved roles' });
      expect(screen.getByRole('alert')).toHaveTextContent('It may already have been applied.');
      expect(screen.queryByRole('dialog')).toBeNull();
      expect(screen.queryByText('No roles available.')).toBeNull();
      expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
      expect(writes()).toHaveLength(1);
      expect(fetchMock.mock.calls).toHaveLength(2);
      fireEvent.click(check);
      await screen.findAllByText('Renamed constrained reader');
      expect(screen.queryByRole('alert')).toBeNull();
      expect(screen.getByRole('button', { name: 'New Role' })).toBeEnabled();
      expect(writes()).toHaveLength(1);
      expect(savedRoles[0].permissions).toEqual(originalRole.permissions);
      expect(fetchMock.mock.calls).toHaveLength(3);
    },
  );

  it.each([
    { status: 400, error: 'invalid_org' },
    { status: 403, error: 'default_org_access_denied' },
  ])(
    'keeps HTTP $status role inventory unknown without silently selecting another organisation',
    async ({ status, error }) => {
      fetchMock.mockResolvedValueOnce(json({ error }, status));
      render(() => <RolesPanel currentUser="reader-a" canManage />);
      expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load roles');
      expect(screen.queryByText('No roles available.')).toBeNull();
      expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(writes()).toHaveLength(0);
      expect(getOrgID()).toBe('estate/north');
    },
  );
});
