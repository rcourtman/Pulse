import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { useUserAssignmentsPanelState } from '../useUserAssignmentsPanelState';
import type { Role, UserRoleAssignment } from '@/types/rbac';

// Keep the state hook, RBACAPI and apiClient real. Only feature availability
// and the final network boundary are synthetic; no grant or live account is used.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({ licenseReady: () => true, rbacEnabled: () => true }),
}));
const log = vi.hoisted(() => ({ error: vi.fn(), warn: vi.fn(), debug: vi.fn() }));
vi.mock('@/utils/logger', () => ({ logger: log }));

const fetchMock = vi.fn<typeof fetch>();
const principal = 'sso:oidc:example:subject/one';
const userPath = `/api/admin/users/${encodeURIComponent(principal)}`;
const roles: Role[] = [
  { id: 'reader', name: 'Reader', description: 'Read monitoring', permissions: [] },
  { id: 'operator', name: 'Operator', description: 'Operate monitoring', permissions: [] },
];
const original: UserRoleAssignment = {
  username: principal,
  displayName: 'Synthetic saved account',
  // A role missing from this catalogue must not silently disappear on save.
  roleIds: ['reader', 'saved-unlisted-role'],
};
const nextOrgUser: UserRoleAssignment = {
  username: principal,
  displayName: 'Other organisation account',
  roleIds: ['operator'],
};
type Operation = 'save' | 'delete';
const operations: Operation[] = ['save', 'delete'];
let savedUsers: UserRoleAssignment[];
let mutationReply: (options: RequestInit) => Promise<Response>;
let healthReply: () => Promise<Response>;
let usersReply: () => Response;

function json(value: unknown, status = 200, headers?: HeadersInit) {
  return new Response(JSON.stringify(value), { status, headers });
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

function mount() {
  const [status, setStatus] = createSignal<{ user: string | undefined; allowed: boolean }>({
    user: 'operator-a',
    allowed: true,
  });
  const setUser = (user: string | undefined) => setStatus((current) => ({ ...current, user }));
  const setAllowed = (allowed: boolean) => setStatus((current) => ({ ...current, allowed }));
  const refresh = () => setStatus((current) => ({ ...current }));
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
  return { state, setUser, setAllowed, refresh, view };
}
type State = ReturnType<typeof useUserAssignmentsPanelState>;

async function loaded(state: State) {
  await waitFor(() => expect(state.loading()).toBe(false));
  expect(state.loadError()).toBeNull();
  expect(state.canEdit()).toBe(true);
}

async function prepare(state: State, operation: Operation) {
  await loaded(state);
  const row = state.filteredAssignments()[0];
  if (operation === 'save') {
    await state.openManageAccess(row);
    state.toggleRole('operator');
  } else {
    state.openDeleteUser(row);
  }
  return row;
}

function confirm(state: State, operation: Operation) {
  return operation === 'save' ? state.handleSaveAssignments() : state.handleDeleteUser();
}

function mutations() {
  return fetchMock.mock.calls.filter(([, options]) =>
    ['PUT', 'DELETE'].includes(options?.method ?? 'GET'),
  );
}

function applyMutation(options: RequestInit) {
  savedUsers =
    options.method === 'DELETE'
      ? []
      : [{ ...original, roleIds: JSON.parse(String(options.body)).roleIds }];
}

function switchOrg() {
  setOrgID('south');
  eventBus.emit('org_switched', 'south');
}

beforeEach(() => {
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  // Exercise production's real CSRF preparation, not apiClient's test-mode shortcut.
  vi.stubEnv('MODE', 'production');
  sessionStorage.clear();
  clearAuth();
  setOrgID('north');
  setApiToken('synthetic-only-auth');
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  savedUsers = [{ ...original, roleIds: [...original.roleIds] }];
  usersReply = () => json(getOrgID() === 'south' ? [nextOrgUser] : savedUsers);
  healthReply = async () => {
    document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
    return json({});
  };
  mutationReply = async (options) => {
    applyMutation(options);
    return new Response(null, { status: 204 });
  };
  fetchMock.mockImplementation(async (input, options = {}) => {
    const url = String(input);
    const method = options.method ?? 'GET';
    if (url === '/api/health' && method === 'GET') return healthReply();
    if (method === 'GET') {
      if (url === '/api/admin/users') return usersReply();
      if (url === '/api/admin/roles') return json(roles);
      if (url === `${userPath}/permissions`) return json([{ action: 'read', resource: 'nodes' }]);
    }
    if (
      (url === `${userPath}/roles` && method === 'PUT') ||
      (url === userPath && method === 'DELETE')
    ) {
      return mutationReply(options);
    }
    throw new Error(`Unexpected synthetic request: ${method} ${url}`);
  });
});

afterEach(() => {
  cleanup();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('User Access through the real RBAC transport', () => {
  it('suppresses untouched and reordered-equivalent saves before CSRF or mutation work', async () => {
    const { state } = mount();
    await loaded(state);
    await state.openManageAccess(state.filteredAssignments()[0]);
    document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
    await state.handleSaveAssignments();
    expect(mutations()).toHaveLength(0);
    expect(fetchMock.mock.calls.filter(([url]) => url === '/api/health')).toHaveLength(0);
    state.toggleRole('reader');
    state.toggleRole('reader');
    expect(state.formRoleIds()).toEqual(['saved-unlisted-role', 'reader']);
    await state.handleSaveAssignments();
    expect(mutations()).toHaveLength(0);
    expect(state.showModal()).toBe(true);
    expect(state.saving()).toBe(false);
    expect(state.successNotice()).toBeNull();
    expect(fetchMock.mock.calls.filter(([url]) => url === '/api/health')).toHaveLength(0);
    expect(fetchMock.mock.calls.filter(([url]) => url === '/api/admin/users')).toHaveLength(1);
    // Deliberate changes still use the normal cold-CSRF and bound write path.
    state.toggleRole('operator');
    await state.handleSaveAssignments();
    expect(mutations()).toHaveLength(1);
    expect(fetchMock.mock.calls.filter(([url]) => url === '/api/health')).toHaveLength(1);
    expect(JSON.parse(String(mutations()[0][1]?.body))).toEqual({
      roleIds: ['saved-unlisted-role', 'reader', 'operator'],
    });
  });

  it.each(
    operations.flatMap((operation) =>
      ['confirmed', 'uncertain'].map((outcome) => ({ operation, outcome })),
    ),
  )(
    'keeps one $operation and its $outcome outcome through equal refreshes before and after dispatch',
    async ({ operation, outcome }) => {
      const csrf = deferred<Response>();
      const response = deferred<Response>();
      healthReply = () => csrf.promise;
      mutationReply = (options) => {
        applyMutation(options);
        return response.promise;
      };
      const { state, refresh } = mount();
      await prepare(state, operation);
      const pending = confirm(state, operation);
      await waitFor(() =>
        expect(fetchMock.mock.calls.some(([url]) => url === '/api/health')).toBe(true),
      );
      refresh();
      await Promise.resolve();
      expect(operation === 'save' ? state.saving() : state.deletingUser()).toBe(true);
      expect(state.canEdit()).toBe(false);
      document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
      csrf.resolve(json({}));
      await waitFor(() => expect(mutations()).toHaveLength(1));
      const request = mutations()[0][1];
      expect(request?.headers).toMatchObject({
        'X-Pulse-Org-ID': 'north',
        'X-CSRF-Token': 'synthetic-csrf',
      });
      refresh();
      await Promise.resolve();
      expect(request?.signal?.aborted).toBe(false);
      expect(operation === 'save' ? state.showModal() : !!state.userPendingDeletion()).toBe(true);
      expect(fetchMock.mock.calls.filter(([url]) => url === '/api/admin/users')).toHaveLength(1);
      await state.handleSaveAssignments();
      await state.handleDeleteUser();
      expect(mutations()).toHaveLength(1);
      response.resolve(
        outcome === 'confirmed'
          ? new Response(null, { status: 204 })
          : json({ error: 'synthetic-lost-confirmation' }, 503),
      );
      await pending;
      if (outcome === 'confirmed') {
        expect(state.successNotice()).toBeTruthy();
        expect(state.unconfirmedChange()).toBeNull();
      } else {
        expect(state.unconfirmedChange()).toContain('may already have been applied');
        expect(state.successNotice()).toBeNull();
        refresh();
        await Promise.resolve();
        expect(state.unconfirmedChange()).toContain('may already have been applied');
        expect(state.canEdit()).toBe(false);
        expect(fetchMock.mock.calls.filter(([url]) => url === '/api/admin/users')).toHaveLength(1);
        await state.loadData();
      }
      await loaded(state);
      expect(state.filteredAssignments()).toEqual(savedUsers);
      expect(fetchMock.mock.calls.filter(([url]) => url === '/api/admin/users')).toHaveLength(2);
      expect(mutations()).toHaveLength(1);
    },
  );

  it.each(operations)(
    'dispatches one correctly bound %s after cold CSRF preparation and rereads saved access',
    async (operation) => {
      const { state } = mount();
      await prepare(state, operation);
      await confirm(state, operation);

      expect(fetchMock.mock.calls.filter(([url]) => url === '/api/health')).toHaveLength(1);
      expect(mutations()).toHaveLength(1);
      const [url, options] = mutations()[0];
      expect(url).toBe(operation === 'save' ? `${userPath}/roles` : userPath);
      expect(options).toMatchObject({
        method: operation === 'save' ? 'PUT' : 'DELETE',
        credentials: 'include',
        signal: expect.any(AbortSignal),
        headers: {
          'X-Pulse-Org-ID': 'north',
          'X-API-Token': 'synthetic-only-auth',
          'X-CSRF-Token': 'synthetic-csrf',
        },
      });
      if (operation === 'save') {
        expect(JSON.parse(String(options?.body))).toEqual({
          roleIds: ['reader', 'saved-unlisted-role', 'operator'],
        });
        expect(state.filteredAssignments()[0].roleIds).toEqual([
          'reader',
          'saved-unlisted-role',
          'operator',
        ]);
      } else {
        expect(options).not.toHaveProperty('body');
        expect(state.filteredAssignments()).toEqual([]);
      }
      expect(state.unconfirmedChange()).toBeNull();
      expect(state.successNotice()).toBeTruthy();
      expect(fetchMock.mock.calls.filter(([path]) => path === '/api/admin/users')).toHaveLength(2);
      for (const [, request] of fetchMock.mock.calls) {
        expect(request).toMatchObject({ credentials: 'include' });
        expect(request).not.toHaveProperty('expectedOrgID');
        expect(request).not.toHaveProperty('retry');
      }
      // Include inventory and saved permissions, not only the mutation request.
      for (const [path, request] of fetchMock.mock.calls.filter(
        ([path]) => path !== '/api/health',
      )) {
        expect(request?.headers, String(path)).toMatchObject({ 'X-Pulse-Org-ID': 'north' });
      }
    },
  );

  it.each(operations)(
    'does not dispatch a stale %s when org changes during CSRF preparation, even before the org event',
    async (operation) => {
      const csrf = deferred<Response>();
      healthReply = () => csrf.promise;
      const { state } = mount();
      await prepare(state, operation);
      const pending = confirm(state, operation);
      await waitFor(() =>
        expect(fetchMock.mock.calls.some(([url]) => url === '/api/health')).toBe(true),
      );
      setOrgID('south');
      csrf.resolve(json({}));
      await pending;
      expect(mutations()).toHaveLength(0);
      expect(getOrgID()).toBe('south');
      expect(state.successNotice()).toBeNull();
    },
  );

  it.each(
    operations.flatMap((operation) =>
      ['user', 'permission', 'dispose'].map((withdrawal) => ({ operation, withdrawal })),
    ),
  )(
    'cancels $operation before dispatch when $withdrawal withdraws it during cold CSRF preparation',
    async ({ operation, withdrawal }) => {
      const csrf = deferred<Response>();
      healthReply = () => csrf.promise;
      const { state, setUser, setAllowed, view } = mount();
      await prepare(state, operation);
      const pending = confirm(state, operation);
      await waitFor(() =>
        expect(fetchMock.mock.calls.some(([url]) => url === '/api/health')).toBe(true),
      );
      if (withdrawal === 'user') setUser('operator-b');
      else if (withdrawal === 'permission') setAllowed(false);
      else view.unmount();
      csrf.resolve(json({}));
      await pending;
      expect(mutations()).toHaveLength(0);
      expect(state.successNotice()).toBeNull();
      expect(state.showModal()).toBe(false);
      expect(state.userPendingDeletion()).toBeNull();
    },
  );

  it.each(operations.flatMap((operation) => [403, 429].map((status) => ({ operation, status }))))(
    'retains a rejected $operation confirmation without replay after HTTP $status',
    async ({ operation, status }) => {
      mutationReply = async () =>
        json({ message: 'synthetic-private-response' }, status, {
          'X-CSRF-Token': 'synthetic-refreshed-csrf',
          'Retry-After': '1',
        });
      const { state } = mount();
      const row = await prepare(state, operation);
      await confirm(state, operation);
      expect(mutations()).toHaveLength(1);
      expect(state.actionError()).toBeTruthy();
      expect(state.unconfirmedChange()).toBeNull();
      expect(state.successNotice()).toBeNull();
      expect(state.filteredAssignments()).toEqual([original]);
      expect(fetchMock.mock.calls.filter(([url]) => url === '/api/admin/users')).toHaveLength(1);
      if (operation === 'save') {
        expect(state.showModal()).toBe(true);
        expect(state.formRoleIds()).toEqual(['reader', 'saved-unlisted-role', 'operator']);
      } else expect(state.userPendingDeletion()).toBe(row);
      expect(
        JSON.stringify([
          state.actionError(),
          log.error.mock.calls,
          log.warn.mock.calls,
          log.debug.mock.calls,
        ]),
      ).not.toContain('synthetic-private-response');
      expect(getOrgID()).toBe('north');
    },
  );

  it.each(
    operations.flatMap((operation) =>
      ['503', 'lost-response', 'unreadable-success'].map((failure) => ({ operation, failure })),
    ),
  )(
    'keeps an uncertain $operation blocked after $failure until an explicit saved-access read',
    async ({ operation, failure }) => {
      mutationReply = async (options) => {
        // Model an applied change whose response does not establish success.
        applyMutation(options);
        if (failure === 'lost-response') throw new TypeError('synthetic-private-response');
        if (failure === 'unreadable-success')
          return new Response('synthetic-private-response', { status: 200 });
        return json({ message: 'synthetic-private-response' }, 503);
      };
      const { state } = mount();
      const row = await prepare(state, operation);
      await confirm(state, operation);
      expect(mutations()).toHaveLength(1);
      expect(state.unconfirmedChange()).toContain('may already have been applied');
      expect(state.filteredAssignments()).toEqual([]);
      expect(state.canEdit()).toBe(false);
      expect(state.showModal()).toBe(false);
      expect(state.userPendingDeletion()).toBeNull();
      expect(state.successNotice()).toBeNull();
      await state.openManageAccess(row);
      state.openDeleteUser(row);
      await state.handleSaveAssignments();
      await state.handleDeleteUser();
      expect(mutations()).toHaveLength(1);
      expect(fetchMock.mock.calls.filter(([url]) => url === '/api/admin/users')).toHaveLength(1);
      expect(
        JSON.stringify([
          state.unconfirmedChange(),
          state.actionError(),
          log.error.mock.calls,
          log.warn.mock.calls,
          log.debug.mock.calls,
        ]),
      ).not.toContain('synthetic-private-response');

      await state.loadData();
      await loaded(state);
      expect(state.unconfirmedChange()).toBeNull();
      expect(state.filteredAssignments()).toEqual(savedUsers);
      expect(state.showModal()).toBe(false);
      expect(state.successNotice()).toBeNull();
      expect(mutations()).toHaveLength(1);
    },
  );

  it.each(operations)(
    'does not repeat or relabel a pending %s when its old response arrives in another org',
    async (operation) => {
      const response = deferred<Response>();
      mutationReply = () => response.promise;
      const { state } = mount();
      const row = await prepare(state, operation);
      const pending = confirm(state, operation);
      await waitFor(() => expect(mutations()).toHaveLength(1));
      const signal = mutations()[0][1]?.signal;
      state.closeModal();
      state.closeDeleteUser();
      await state.openManageAccess(row);
      state.openDeleteUser(row);
      await state.handleSaveAssignments();
      await state.handleDeleteUser();
      expect(mutations()).toHaveLength(1);
      switchOrg();
      await loaded(state);
      expect(signal?.aborted).toBe(true);
      // The synthetic transport deliberately ignores cancellation to exercise the late-result guard.
      response.resolve(new Response(null, { status: 204 }));
      await pending;
      expect(mutations()).toHaveLength(1);
      expect(state.filteredAssignments()).toEqual([nextOrgUser]);
      expect(state.successNotice()).toBeNull();
      expect(state.unconfirmedChange()).toBeNull();
      expect(state.showModal()).toBe(false);
      expect(state.userPendingDeletion()).toBeNull();
      expect(getOrgID()).toBe('south');
    },
  );

  it('keeps a refused org-bound directory unavailable without silently selecting default or enabling writes', async () => {
    usersReply = () => json({ error: 'invalid_org' }, 400);
    const { state } = mount();
    await waitFor(() => expect(state.loading()).toBe(false));
    expect(state.loadError()).toBeTruthy();
    expect(state.canEdit()).toBe(false);
    expect(state.filteredAssignments()).toEqual([]);
    expect(getOrgID()).toBe('north');
    expect(fetchMock.mock.calls.filter(([url]) => url === '/api/admin/users')).toHaveLength(1);
    await state.openManageAccess(original);
    state.openDeleteUser(original);
    await state.handleSaveAssignments();
    await state.handleDeleteUser();
    expect(mutations()).toHaveLength(0);
  });
});
