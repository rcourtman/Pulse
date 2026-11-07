import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { OrganizationRole } from '@/api/orgs';
import type { SecurityStatusPresentationPolicy } from '@/types/config';
import { eventBus } from '@/stores/events';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { useOrganizationAccessPanelState } from '../useOrganizationAccessPanelState';

// Exercise the real owner, API methods, production CSRF preparation, event bus
// and capability/policy stores together. Warm-cookie and mocked-client tests
// cannot prove that an intent is still admitted when preparation finishes.
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

type State = ReturnType<typeof useOrganizationAccessPanelState>;
const operations = ['invite', 'role', 'transfer', 'remove', 'revoke', 'accept', 'decline'] as const;
type Operation = (typeof operations)[number];
const north = 'north/operations';
const south = 'south/support';
const invitingOrg = 'inviting/west';
const owner = 'owner@example.test';
const member = 'reader/team@example.test';
const invitedUser = 'invited/operator@example.test';
const pendingUser = 'pending/team@example.test';
const storedToken = 'synthetic-unrelated-api-token';
const capabilityURL = '/api/license/runtime-capabilities';
const orgPath = (id = north) => `/api/orgs/${encodeURIComponent(id)}`;
const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}
const settle = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
const fetchMock = vi.fn<typeof fetch>();
const writes = () =>
  fetchMock.mock.calls.filter(([, options]) =>
    ['POST', 'DELETE'].includes(options?.method ?? 'GET'),
  );
const healthReads = () => fetchMock.mock.calls.filter(([url]) => url === '/api/health');
const orgReads = (id = north) =>
  fetchMock.mock.calls.filter(
    ([url, options]) => url === orgPath(id) && (options?.method ?? 'GET') === 'GET',
  );
let operation: Operation;
let selectedOrg: string;
let applied: boolean;
let capabilityFailed: boolean;
let preparation: ReturnType<typeof deferred<Response>>;
let reply: ReturnType<typeof deferred<Response>>;

function snapshot(id = selectedOrg) {
  const changed = applied && id === selectedOrg;
  const transferred = changed && operation === 'transfer';
  return {
    organization: {
      id,
      displayName: id === south ? 'South support' : 'North operations',
      ownerUserId: transferred ? member : owner,
      canManageMembers: id === 'default',
    },
    members: [
      { userId: owner, role: transferred ? 'admin' : 'owner', addedAt: '2026-11-01T12:00:00Z' },
      ...(changed && operation === 'remove'
        ? []
        : [
            {
              userId: member,
              role: transferred ? 'owner' : changed && operation === 'role' ? 'editor' : 'viewer',
              addedAt: '2026-11-02T12:00:00Z',
            },
          ]),
    ],
    pending: [
      ...(changed && operation === 'revoke'
        ? []
        : [{ userId: pendingUser, role: 'viewer', invitedBy: owner, invitedAt: '' }]),
      ...(changed && operation === 'invite'
        ? [{ userId: invitedUser, role: 'editor', invitedBy: owner, invitedAt: '' }]
        : []),
    ],
    personal:
      changed && ['accept', 'decline'].includes(operation)
        ? []
        : [
            {
              orgId: invitingOrg,
              orgDisplayName: 'West invitation',
              userId: owner,
              role: 'viewer',
              invitedBy: 'west-owner@example.test',
              invitedAt: '',
            },
          ],
  };
}
function mutationReply() {
  if (operation === 'invite')
    return json({ kind: 'invitation', invitation: snapshot().pending[1] });
  if (operation === 'role' || operation === 'transfer')
    return json({ kind: 'member', member: snapshot().members[1] });
  if (operation === 'accept')
    return json({ kind: 'member', member: { userId: owner, role: 'viewer', addedAt: '' } });
  return new Response(null, { status: 204 });
}
function expectedWrite() {
  if (operation === 'invite' || operation === 'role' || operation === 'transfer')
    return {
      url: `${orgPath(selectedOrg)}/members`,
      method: 'POST',
      body: {
        userId: operation === 'invite' ? invitedUser : member,
        role: operation === 'transfer' ? 'owner' : 'editor',
      },
    };
  if (operation === 'remove' || operation === 'revoke')
    return {
      url: `${orgPath(selectedOrg)}/${operation === 'remove' ? 'members' : 'invitations'}/${encodeURIComponent(operation === 'remove' ? member : pendingUser)}`,
      method: 'DELETE',
      body: undefined,
    };
  return {
    url: `/api/org-invitations/${encodeURIComponent(invitingOrg)}${operation === 'accept' ? '/accept' : ''}`,
    method: operation === 'accept' ? 'POST' : 'DELETE',
    body: undefined,
  };
}
const setPolicy = (policy: Partial<SecurityStatusPresentationPolicy> = {}) =>
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode: false,
      readOnly: false,
      hideCommercial: false,
      hideUpgrade: true,
      ...policy,
    },
  });
function mount() {
  const [user, setUser] = createSignal(owner);
  let state!: State;
  const view = render(() => {
    state = useOrganizationAccessPanelState({
      get currentUser() {
        return user();
      },
    });
    return <div />;
  });
  return { state, view, setUser };
}
async function loaded(state: State, id = selectedOrg) {
  await waitFor(() => {
    expect(state.loading()).toBe(false);
    expect(state.loadError()).toBeNull();
    expect(state.org()?.id).toBe(id);
  });
}
async function intent(state: State) {
  await loaded(state);
  expect(state.canManageCurrentOrg()).toBe(true);
  const row = state.members().find((candidate) => candidate.userId === member)!;
  if (operation === 'invite') {
    state.setInviteUserID(`  ${invitedUser}  `);
    state.setInviteRole('editor');
    return () => state.inviteMember();
  }
  if (operation === 'role') return () => state.updateRole(row, 'editor' as OrganizationRole);
  if (operation === 'transfer') {
    await state.updateRole(row, 'owner');
    const transfer = state.ownershipTransfer()!;
    expect(transfer).not.toBeNull();
    return () => state.confirmOwnershipTransfer(transfer);
  }
  if (operation === 'remove') return () => state.removeMember(row);
  if (operation === 'revoke') return () => state.revokeInvitation(pendingUser);
  if (operation === 'accept') return () => state.acceptInvitation(invitingOrg);
  return () => state.declineInvitation(invitingOrg);
}
function finishPreparation() {
  document.cookie = 'pulse_csrf=synthetic-prepared-csrf; Path=/';
  preparation.resolve(json({}));
}
function expectBoundWrite() {
  expect(writes()).toHaveLength(1);
  const [url, options] = writes()[0];
  const expected = expectedWrite();
  expect(url).toBe(expected.url);
  expect(options).toMatchObject({
    method: expected.method,
    credentials: 'include',
    signal: expect.any(AbortSignal),
    headers: {
      'X-Pulse-Org-ID': selectedOrg,
      'X-CSRF-Token': 'synthetic-prepared-csrf',
    },
  });
  expect(options?.body ? JSON.parse(String(options.body)) : undefined).toEqual(expected.body);
  expect(options?.headers).not.toHaveProperty('X-API-Token');
  for (const internal of ['expectedOrgID', 'retry', 'preferSessionAuth', 'skipOrgContext'])
    expect(options).not.toHaveProperty(internal);
  expect(getOrgID()).toBe(selectedOrg);
  expect(getApiToken()).toBe(storedToken);
}
function expectAcceptedSnapshot(state: State) {
  const saved = snapshot();
  expect(state.org()).toEqual(saved.organization);
  expect(state.members()).toEqual(saved.members);
  expect(state.pendingInvitations()).toEqual(saved.pending);
  expect(state.myInvitations()).toEqual(saved.personal);
}

beforeEach(async () => {
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal(
    'confirm',
    vi.fn(() => true),
  );
  vi.stubEnv('MODE', 'production');
  clearAuth();
  sessionStorage.clear();
  localStorage.clear();
  selectedOrg = north;
  setOrgID(selectedOrg);
  setApiToken(storedToken);
  setPolicy();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  operation = 'invite';
  applied = false;
  capabilityFailed = false;
  preparation = deferred<Response>();
  reply = deferred<Response>();
  fetchMock.mockImplementation(async (input, options = {}) => {
    const url = String(input);
    const method = options.method ?? 'GET';
    if (url === capabilityURL) {
      if (capabilityFailed) throw new TypeError('Synthetic unavailable capability read');
      return json({ capabilities: ['multi_tenant'], limits: [] });
    }
    if (url === '/api/health') return preparation.promise;
    if (method !== 'GET') {
      // This is fixture state, not original-client or server persistence proof.
      // Ignore signal deliberately: cancellation must prevent dispatch itself,
      // not merely ask native fetch to hide an obsolete confirmation.
      applied = true;
      return reply.promise;
    }
    const data = snapshot(new Headers(options.headers).get('X-Pulse-Org-ID') ?? 'default');
    if (url === '/api/org-invitations') return json(data.personal);
    if (url.endsWith('/members')) return json(data.members);
    if (url.endsWith('/invitations')) return json(data.pending);
    if (url.startsWith('/api/orgs/')) return json(data.organization);
    throw new Error(`Unexpected synthetic access request: ${method} ${url}`);
  });
  await loadRuntimeCapabilities(true);
});
afterEach(() => {
  cleanup();
  syncSessionPresentationPolicy(null);
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('organization access through production cold-CSRF preparation', () => {
  it.each(
    operations.flatMap((action) =>
      (['confirmed', 'unconfirmed'] as const).map((outcome) => ({ action, outcome })),
    ),
  )(
    'retains one $action and its $outcome outcome across equal-reader refreshes',
    async ({ action, outcome }) => {
      operation = action;
      const { state } = mount();
      const submit = await intent(state);
      const saving = submit();
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      expect(healthReads()[0][1]).toEqual({ method: 'GET', credentials: 'include' });
      expect(writes()).toHaveLength(0);
      expect(state.saving()).toBe(true);
      eventBus.emit('organizations_changed');
      setPolicy();
      await loadRuntimeCapabilities(true);
      await state.loadOrganizationAccess();
      await submit();
      expect(orgReads()).toHaveLength(1);
      expect(writes()).toHaveLength(0);
      expect(state.saving()).toBe(true);

      finishPreparation();
      await waitFor(() => expect(writes()).toHaveLength(1));
      expectBoundWrite();
      expect(writes()[0][1]?.signal?.aborted).toBe(false);
      expect(notices.success).not.toHaveBeenCalled();
      eventBus.emit('organizations_changed');
      await submit();
      expect(orgReads()).toHaveLength(1);
      expect(writes()).toHaveLength(1);
      reply.resolve(
        outcome === 'confirmed'
          ? mutationReply()
          : json({ error: 'Synthetic lost acknowledgement' }, 502),
      );
      await saving;
      if (outcome === 'unconfirmed') {
        expect(state.loadError()).toContain('could not be confirmed');
        expect(state.org()).toBeNull();
        expect(state.members()).toEqual([]);
        expect(state.pendingInvitations()).toEqual([]);
        expect(state.myInvitations()).toEqual([]);
        expect(state.canChangeAccess()).toBe(false);
        expect(notices.success).not.toHaveBeenCalled();
        await submit();
        expect(orgReads()).toHaveLength(1);
        expect(writes()).toHaveLength(1);
        await state.loadOrganizationAccess();
      } else expect(notices.success).toHaveBeenCalledTimes(1);
      await loaded(state);
      expectAcceptedSnapshot(state);
      expect(state.saving()).toBe(false);
      expect(orgReads()).toHaveLength(2);
      expect(healthReads()).toHaveLength(1);
      expectBoundWrite();
      expect(notices.error).toHaveBeenCalledTimes(outcome === 'confirmed' ? 0 : 1);
    },
  );

  it.each(operations.filter((action) => action !== 'transfer'))(
    'binds a default-organization %s to the default session, not the invitation target or stored token',
    async (action) => {
      operation = action;
      selectedOrg = 'default';
      setOrgID(null);
      const { state } = mount();
      const submit = await intent(state);
      const saving = submit();
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      finishPreparation();
      await waitFor(() => expect(writes()).toHaveLength(1));
      // getOrgID retains null for the default; its admitted request header is explicit.
      expect(writes()[0][1]?.headers).toMatchObject({ 'X-Pulse-Org-ID': 'default' });
      expect(writes()[0][1]?.headers).not.toHaveProperty('X-API-Token');
      expect(String(writes()[0][0])).toBe(expectedWrite().url);
      reply.resolve(mutationReply());
      await saving;
      await loaded(state);
      expectAcceptedSnapshot(state);
      expect(getOrgID()).toBeNull();
      expect(writes()).toHaveLength(1);
    },
  );

  it.each(operations)(
    'does not replay %s after a CSRF refusal, even when the response supplies a new token',
    async (action) => {
      operation = action;
      const { state } = mount();
      const submit = await intent(state);
      const saving = submit();
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      finishPreparation();
      await waitFor(() => expect(writes()).toHaveLength(1));
      reply.resolve(
        json({ error: 'Synthetic CSRF refusal' }, 403, { 'X-CSRF-Token': 'synthetic-new-csrf' }),
      );
      await saving;
      expect(state.loadError()).toContain('could not be confirmed');
      expect(state.canChangeAccess()).toBe(false);
      expect(notices.success).not.toHaveBeenCalled();
      expectBoundWrite();
      await state.loadOrganizationAccess();
      await loaded(state);
      expectAcceptedSnapshot(state);
      expectBoundWrite();
    },
  );
});

describe('organization access intent withdrawal before HTTP dispatch', () => {
  it.each(
    operations.flatMap((action) =>
      (['organization', 'user', 'read-only', 'demo', 'capability-failure', 'unmount'] as const).map(
        (change) => ({ action, change }),
      ),
    ),
  )(
    'never dispatches $action after $change changes during preparation',
    async ({ action, change }) => {
      operation = action;
      const { state, view, setUser } = mount();
      const submit = await intent(state);
      const saving = submit();
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      if (change === 'organization') {
        setOrgID(south);
        eventBus.emit('org_switched', south);
      }
      if (change === 'user') setUser('other-reader@example.test');
      if (change === 'read-only') setPolicy({ readOnly: true });
      if (change === 'demo') setPolicy({ demoMode: true });
      if (change === 'capability-failure') {
        capabilityFailed = true;
        await loadRuntimeCapabilities(true);
      }
      if (change === 'unmount') view.unmount();
      await settle();
      finishPreparation();
      // A broken guard must settle too, so a false dispatch fails an assertion,
      // not a timeout waiting for this deliberately held synthetic response.
      reply.resolve(json({}));
      await saving;
      expect(writes()).toHaveLength(0);
      expect(applied).toBe(false);
      expect(notices.success).not.toHaveBeenCalled();
      expect(notices.error).not.toHaveBeenCalled();
      expect(healthReads()).toHaveLength(1);
      expect(getApiToken()).toBe(storedToken);
      if (change === 'organization') {
        await loaded(state, south);
        expect(state.org()?.displayName).toBe('South support');
        expect(getOrgID()).toBe(south);
      }
      if (change === 'read-only' || change === 'demo' || change === 'capability-failure')
        expect(state.canChangeAccess()).toBe(false);
      if (change === 'unmount') {
        const readsBefore = fetchMock.mock.calls.length;
        eventBus.emit('organizations_changed');
        await state.loadOrganizationAccess();
        await settle();
        expect(fetchMock.mock.calls).toHaveLength(readsBefore);
        expect(state.canChangeAccess()).toBe(false);
      }
    },
  );

  it.each(operations)(
    'rechecks the admitted org for %s before a switch event arrives',
    async (action) => {
      operation = action;
      const { state } = mount();
      const submit = await intent(state);
      const saving = submit();
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      setOrgID(south);
      finishPreparation();
      reply.resolve(json({}));
      await saving;
      expect(writes()).toHaveLength(0);
      expect(applied).toBe(false);
      expect(notices.success).not.toHaveBeenCalled();
      expect(notices.error).not.toHaveBeenCalled();
      expect(getOrgID()).toBe(south);
      await loaded(state, south);
      expect(state.org()?.displayName).toBe('South support');
    },
  );
});
