import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { IncomingOrganizationShare, Organization, OrganizationShare } from '@/api/orgs';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { useOrganizationSharingPanelState } from '../useOrganizationSharingPanelState';

// Keep the sharing state, OrgsAPI, apiClient and event bus connected. Unit tests
// that replace both API and context cannot detect a lost option between layers.
vi.mock('@/stores/license', () => ({ isMultiTenantEnabled: () => true }));
vi.mock('@/hooks/useResources', () => ({ useResources: () => ({ resources: () => [] }) }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const owner = 'synthetic-owner';
const storedToken = 'synthetic-unrelated-api-token';
const csrfToken = 'synthetic-csrf-after-health';
const north: Organization = { id: 'north/one', displayName: 'North', ownerUserId: owner };
const south: Organization = { id: 'south/two', displayName: 'South', ownerUserId: owner };
const orgPath = (id: string) => `/api/orgs/${encodeURIComponent(id)}`;
const outgoing: OrganizationShare = {
  id: 'outgoing/one',
  targetOrgId: south.id,
  resourceType: 'vm',
  resourceId: 'opaque::vm/101',
  resourceName: 'Saved VM',
  accessRole: 'viewer',
  status: 'pending',
  createdAt: '2026-11-01T00:00:00Z',
  createdBy: owner,
};
const incoming: IncomingOrganizationShare = {
  ...outgoing,
  id: 'incoming/one',
  targetOrgId: north.id,
  sourceOrgId: south.id,
  sourceOrgName: south.displayName,
};
const draft = {
  targetOrgId: south.id,
  resourceType: 'vm',
  resourceId: 'opaque::draft/202',
  resourceName: 'Unsent draft',
  accessRole: 'viewer',
} as const;
type State = ReturnType<typeof useOrganizationSharingPanelState>;
type Operation = 'create' | 'remove' | 'accept' | 'decline';
const operations: Operation[] = ['create', 'remove', 'accept', 'decline'];
const methodFor = (operation: Operation) =>
  operation === 'create' || operation === 'accept' ? 'POST' : 'DELETE';
const pathFor = (operation: Operation) => {
  const base = `${orgPath(north.id)}/shares`;
  if (operation === 'create') return base;
  if (operation === 'remove') return `${base}/${encodeURIComponent(outgoing.id)}`;
  return `${base}/incoming/${encodeURIComponent(incoming.id)}${operation === 'accept' ? '/accept' : ''}`;
};
const readPaths = (id: string) => [
  orgPath(id),
  `${orgPath(id)}/members`,
  '/api/orgs',
  `${orgPath(id)}/shares`,
  `${orgPath(id)}/shares/incoming`,
];
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}

const fetchMock = vi.fn<typeof fetch>();
let outgoingInventory: OrganizationShare[];
let incomingInventory: IncomingOrganizationShare[];
let healthReply: () => Promise<Response>;
let writeReply: (url: string, options: RequestInit) => Promise<Response>;
let failedRead: string | undefined;

function apply(operation: Operation) {
  if (operation === 'create')
    outgoingInventory = [...outgoingInventory, { ...outgoing, ...draft, id: 'new/share' }];
  if (operation === 'remove') outgoingInventory = [];
  if (operation === 'accept') incomingInventory = [{ ...incoming, status: 'accepted' }];
  if (operation === 'decline') incomingInventory = [];
}

function mount() {
  const [user, setUser] = createSignal<string | undefined>(owner);
  let state!: State;
  const view = render(() => {
    state = useOrganizationSharingPanelState({
      get currentUser() {
        return user();
      },
    });
    return <div />;
  });
  return { state, setUser, view };
}

async function accepted(state: State, id = north.id) {
  await waitFor(() => {
    expect(state.loading()).toBe(false);
    expect(state.loadError()).toBeNull();
    expect(state.org()?.id).toBe(id);
  });
}

async function prepared() {
  const mounted = mount();
  await accepted(mounted.state);
  expect(mounted.state.canManageCurrentOrg()).toBe(true);
  mounted.state.updateTargetOrg(draft.targetOrgId);
  mounted.state.updateResourceType(draft.resourceType);
  mounted.state.updateResourceId(draft.resourceId);
  mounted.state.updateResourceName(draft.resourceName);
  expect(mounted.state.canCreateShare()).toBe(true);
  return mounted;
}

function confirm(state: State, operation: Operation) {
  if (operation === 'create') return state.createShare();
  if (operation === 'remove') return state.deleteShare(state.outgoingShares()[0] ?? outgoing);
  if (operation === 'accept')
    return state.acceptIncomingShare(state.incomingShares()[0] ?? incoming);
  return state.declineIncomingShare(state.incomingShares()[0] ?? incoming);
}

const writes = () =>
  fetchMock.mock.calls.filter(([, options]) => (options?.method ?? 'GET') !== 'GET');
const reads = () =>
  fetchMock.mock.calls.filter(
    ([url, options]) => (options?.method ?? 'GET') === 'GET' && url !== '/api/health',
  );
const switchOrg = () => {
  setOrgID(south.id);
  eventBus.emit('org_switched', south.id);
};
function expectSessionRequest(options: RequestInit | undefined, id = north.id) {
  expect(options).toMatchObject({
    credentials: 'include',
    signal: expect.any(AbortSignal),
    headers: { 'X-Pulse-Org-ID': id },
  });
  expect(options?.headers).not.toHaveProperty('X-API-Token');
  expect(options).not.toHaveProperty('retry');
  expect(options).not.toHaveProperty('expectedOrgID');
}

beforeEach(() => {
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal(
    'confirm',
    vi.fn(() => true),
  );
  vi.stubEnv('MODE', 'production');
  sessionStorage.clear();
  clearAuth();
  setOrgID(north.id);
  setApiToken(storedToken);
  syncSessionPresentationPolicy(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  outgoingInventory = [outgoing];
  incomingInventory = [incoming];
  failedRead = undefined;
  healthReply = async () => {
    document.cookie = `pulse_csrf=${csrfToken}; Path=/`;
    return json({});
  };
  writeReply = async () => new Response(null, { status: 204 });
  fetchMock.mockImplementation(async (input, options = {}) => {
    const url = String(input);
    const method = options.method ?? 'GET';
    if (url === '/api/health' && method === 'GET') return healthReply();
    if (method !== 'GET') return writeReply(url, options);
    if (url === failedRead) return json({ error: 'synthetic-unavailable' }, 503);
    if (url === '/api/orgs') return json([north, south]);
    for (const organization of [north, south]) {
      const base = orgPath(organization.id);
      if (url === base) return json(organization);
      if (url === `${base}/members`) return json([]);
      if (url === `${base}/shares`)
        return json(organization.id === north.id ? outgoingInventory : []);
      if (url === `${base}/shares/incoming`)
        return json(organization.id === north.id ? incomingInventory : []);
    }
    throw new Error(`Unexpected synthetic read: ${method} ${url}`);
  });
});

afterEach(() => {
  cleanup();
  clearAuth();
  syncSessionPresentationPolicy(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('organization sharing through the real transport', () => {
  it.each(operations)(
    'confirms one encoded %s with session auth and rereads its complete snapshot',
    async (operation) => {
      writeReply = async () => {
        apply(operation);
        return new Response(null, { status: 204 });
      };
      const { state } = await prepared();
      expect(reads().map(([url]) => url)).toEqual(readPaths(north.id));
      await confirm(state, operation);
      await accepted(state);
      expect(writes()).toHaveLength(1);
      const [url, options] = writes()[0];
      expect(url).toBe(pathFor(operation));
      expect(options?.method).toBe(methodFor(operation));
      expect(options?.body ? JSON.parse(String(options.body)) : undefined).toEqual(
        operation === 'create' ? draft : undefined,
      );
      expectSessionRequest(options);
      expect(options?.headers).toMatchObject({ 'X-CSRF-Token': csrfToken });
      expect(fetchMock.mock.calls.filter(([url]) => url === '/api/health')).toHaveLength(1);
      expect(reads().map(([url]) => url)).toEqual([...readPaths(north.id), ...readPaths(north.id)]);
      for (const [, options] of reads()) expectSessionRequest(options);
      expect(state.outgoingShares()).toEqual(outgoingInventory);
      expect(state.incomingShares()).toEqual(incomingInventory);
      expect(notificationStore.success).toHaveBeenCalledTimes(1);
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(getOrgID()).toBe(north.id);
      expect(getApiToken()).toBe(storedToken);
    },
  );

  it.each(
    operations.flatMap((operation) =>
      ['organization', 'user', 'read-only', 'disposal'].map((withdrawal) => ({
        operation,
        withdrawal,
      })),
    ),
  )(
    'cancels $operation before dispatch on $withdrawal withdrawal during cold CSRF',
    async ({ operation, withdrawal }) => {
      const health = deferred<Response>();
      healthReply = () => health.promise;
      const { state, setUser, view } = await prepared();
      const pending = confirm(state, operation);
      expect(state.saving()).toBe(true);
      expect(fetchMock.mock.calls.filter(([url]) => url === '/api/health')).toHaveLength(1);
      if (withdrawal === 'organization') switchOrg();
      if (withdrawal === 'user') setUser('another-synthetic-user');
      if (withdrawal === 'read-only')
        syncSessionPresentationPolicy({
          presentationPolicy: {
            demoMode: false,
            readOnly: true,
            hideCommercial: false,
            hideUpgrade: true,
          },
        });
      if (withdrawal === 'disposal') view.unmount();
      // Let the hook invalidate its reader before the cookie and response arrive.
      await Promise.resolve();
      document.cookie = `pulse_csrf=${csrfToken}; Path=/`;
      health.resolve(json({}));
      await pending;
      expect(writes()).toHaveLength(0);
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).not.toHaveBeenCalled();
      if (withdrawal === 'organization') {
        await accepted(state, south.id);
        expect(state.targetOrgId()).toBe('');
        expect(state.outgoingShares()).toEqual([]);
        expect(state.incomingShares()).toEqual([]);
      }
      expect(getApiToken()).toBe(storedToken);
    },
  );

  it.each(
    operations.flatMap((operation) => [false, true].map((applied) => ({ operation, applied }))),
  )(
    'reconciles uncertain $operation (applied: $applied) only through an explicit read, never replay',
    async ({ operation, applied }) => {
      const reply = deferred<Response>();
      writeReply = async () => {
        if (applied) apply(operation);
        return reply.promise;
      };
      const { state } = await prepared();
      const pending = confirm(state, operation);
      await waitFor(() => expect(writes()).toHaveLength(1));
      const signal = writes()[0][1]?.signal;
      eventBus.emit('organizations_changed');
      await state.loadSharingData();
      expect(reads()).toHaveLength(5);
      expect(state.saving()).toBe(true);
      expect(signal?.aborted).toBe(false);
      await confirm(state, operation);
      expect(writes()).toHaveLength(1);
      // A response that invites CSRF or idempotent DELETE retry is still not a
      // second confirmation. Neither HTTP failure establishes server rollback.
      reply.resolve(
        new Response('{"error":"synthetic-unconfirmed"}', {
          status: operation === 'create' || operation === 'accept' ? 403 : 429,
          headers: { 'X-CSRF-Token': 'synthetic-refreshed', 'Retry-After': '0.001' },
        }),
      );
      await pending;
      expect(writes()).toHaveLength(1);
      expect(reads()).toHaveLength(5);
      expect(state.loadError()).toContain('could not be confirmed');
      expect(state.canManageCurrentOrg()).toBe(false);
      expect(state.outgoingShares()).toEqual([]);
      expect(state.incomingShares()).toEqual([]);
      expect(state.targetOrgId()).toBe(draft.targetOrgId);
      expect(state.resourceId()).toBe(draft.resourceId);
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).toHaveBeenCalledTimes(1);
      await confirm(state, operation);
      expect(writes()).toHaveLength(1);
      await state.loadSharingData();
      await accepted(state);
      expect(reads()).toHaveLength(10);
      expect(state.outgoingShares()).toEqual(outgoingInventory);
      expect(state.incomingShares()).toEqual(incomingInventory);
      expect(writes()).toHaveLength(1);
      expect(state.canCreateShare()).toBe(true);
      expect(getOrgID()).toBe(north.id);
      expect(getApiToken()).toBe(storedToken);
    },
  );

  it.each(readPaths(north.id))('rejects the whole sharing snapshot when %s fails', async (path) => {
    failedRead = path;
    const { state } = mount();
    await waitFor(() => expect(state.loading()).toBe(false));
    expect(state.loadError()).toBeTruthy();
    expect(state.org()).toBeNull();
    expect(state.outgoingShares()).toEqual([]);
    expect(state.incomingShares()).toEqual([]);
    expect(state.canManageCurrentOrg()).toBe(false);
    expect(state.canCreateShare()).toBe(false);
    expect(reads()).toHaveLength(5);
    for (const [, options] of reads()) {
      expectSessionRequest(options);
      expect(options?.signal?.aborted).toBe(true);
    }
    for (const operation of operations) await confirm(state, operation);
    expect(writes()).toHaveLength(0);
    expect(getOrgID()).toBe(north.id);
  });

  it.each(operations)(
    'ignores a late %s confirmation after selecting a different organization',
    async (operation) => {
      const reply = deferred<Response>();
      writeReply = () => reply.promise;
      const { state } = await prepared();
      const pending = confirm(state, operation);
      await waitFor(() => expect(writes()).toHaveLength(1));
      const signal = writes()[0][1]?.signal;
      switchOrg();
      await accepted(state, south.id);
      state.updateResourceId('new-organization-draft');
      reply.resolve(new Response(null, { status: 204 }));
      await pending;
      expect(signal?.aborted).toBe(true);
      expect(writes()).toHaveLength(1);
      expect(state.org()?.id).toBe(south.id);
      expect(state.resourceId()).toBe('new-organization-draft');
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(reads()).toHaveLength(10);
      for (const [, options] of reads().slice(5)) expectSessionRequest(options, south.id);
    },
  );
});
