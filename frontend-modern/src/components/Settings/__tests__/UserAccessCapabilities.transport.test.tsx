import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { Role, UserRoleAssignment } from '@/types/rbac';
import { loadRuntimeCapabilities, runtimeCapabilitiesLoadError } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';

// Do not replace the capability store or gate: the older transport/control
// suites fix it to enabled. Exercise its actual HTTP producer, page, dialogs,
// state owner, RBACAPI and production CSRF preparation together instead.
// Capability loads are deliberately sequential and same-org. This suite does
// not establish ordering of overlapping capability responses or shell routing.
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const orgID = 'estate/north?realm=lab#saved';
const token = 'synthetic-user-access-capability-token';
const csrf = 'synthetic-user-access-capability-csrf';
const capabilityPath = '/api/license/runtime-capabilities';
const directoryPath = '/api/admin/users';
const rolesPath = '/api/admin/roles';
const unlistedRole = 'saved/unlisted:constraint';
const roles: Role[] = [
  {
    id: 'read/nodes',
    name: 'Fleet role',
    description: 'Read nodes',
    permissions: [],
  },
  {
    id: 'write/nodes',
    name: 'Fleet role',
    description: 'Operate nodes',
    permissions: [],
  },
];
const originals: UserRoleAssignment[] = ['first', 'second', 'third'].map((part) => ({
  username: `sso:oidc:lab:subject/${part}?realm=north#saved`,
  displayName: 'Alex Example',
  email: `${part}@example.invalid`,
  roleIds: [roles[0].id, unlistedRole],
}));
const selected = originals[1];
const userPath = (user = selected) => `/api/admin/users/${encodeURIComponent(user.username)}`;
const fetchMock = vi.fn<typeof fetch>();
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
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
function heldBody() {
  const body = deferred<string>();
  const response = json(null);
  const read = vi.spyOn(response, 'text').mockImplementation(() => body.promise);
  return { ...body, response, read };
}
const turn = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
const reads = (path: string) =>
  fetchMock.mock.calls.filter(([url, init]) => url === path && (init?.method ?? 'GET') === 'GET');
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => ['PUT', 'DELETE'].includes(init?.method ?? 'GET'));
type Operation = 'save' | 'remove';
const operations: Operation[] = ['save', 'remove'];
let savedUsers: UserRoleAssignment[];
let capabilityReply: () => Promise<Response>;
let healthReply: () => Promise<Response>;
let permissionReply: () => Response;
let mutationReply: (init: RequestInit) => Response | Promise<Response>;
let unexpected: string[];

function rowFor(user: UserRoleAssignment) {
  const row = screen.getByTitle(user.username).closest('tr');
  expect(row).not.toBeNull();
  return within(row!);
}
const editor = () =>
  within(
    screen.getByRole('dialog', { name: `Manage access: Alex Example (${selected.username})` }),
  );
const roleChoice = (description: string) =>
  editor().getByRole('checkbox', { name: `Fleet role ${description}` });
const search = () => screen.getByPlaceholderText('Search users...');

async function openPanel() {
  render(() => <UserAssignmentsPanel currentUser="synthetic-maintainer" canManage />);
  await screen.findByTitle(selected.username);
  expect(reads(directoryPath)).toHaveLength(1);
  expect(reads(rolesPath)).toHaveLength(1);
  expect(writes()).toHaveLength(0);
}
async function openEditor(waitForPermissions = true) {
  fireEvent.click(
    rowFor(selected).getByRole('button', {
      name: `Manage access for Alex Example (${selected.username})`,
    }),
  );
  expect(editor().getByText(selected.email!)).toBeInTheDocument();
  expect(editor().getByText(selected.username)).toBeInTheDocument();
  if (waitForPermissions) await editor().findByText(`saved:${selected.username}`);
}
async function prepare(operation: Operation) {
  if (operation === 'save') {
    await openEditor();
    fireEvent.click(roleChoice('Read nodes'));
    fireEvent.click(roleChoice('Operate nodes'));
    return editor().getByRole('button', { name: 'Save Changes' });
  }
  fireEvent.click(
    rowFor(selected).getByRole('button', { name: `Remove Alex Example (${selected.username})` }),
  );
  return within(
    screen.getByRole('dialog', { name: `Remove user access: Alex Example (${selected.username})` }),
  ).getByRole('button', { name: 'Remove User Access' });
}
async function capabilities(kind: 'enabled' | 'withdrawn' | 'failed' | 'malformed') {
  capabilityReply = async () => {
    if (kind === 'failed') throw new TypeError('synthetic-private-capability-unavailable');
    return json({
      capabilities: kind === 'malformed' ? 'rbac' : kind === 'enabled' ? ['rbac'] : [],
      limits: [],
    });
  };
  await loadRuntimeCapabilities(true);
}
function expectedSavedUsers(operation: Operation) {
  return operation === 'save'
    ? originals.map((user) =>
        user === selected ? { ...user, roleIds: [unlistedRole, roles[1].id] } : user,
      )
    : originals.filter((user) => user !== selected);
}
function expectWithdrawn() {
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(search()).toBeDisabled();
  expect(screen.queryByTitle(selected.username)).toBeNull();
  expect(screen.queryByText('No users yet')).toBeNull();
  expect(screen.queryByRole('button', { name: 'Check saved access' })).toBeNull();
  expect(screen.queryByRole('link', { name: 'View plans' })).toBeNull();
}
function expectNoObsoleteFeedback() {
  expect(screen.queryByText(/Roles updated for|User access removed for/)).toBeNull();
  expect(screen.queryByText(/could not be confirmed|Failed to update user roles/)).toBeNull();
  expect(screen.queryByText(/Failed to remove user access|synthetic-private/)).toBeNull();
}
function expectBoundRequests() {
  expect(unexpected).toEqual([]);
  for (const [url, init] of fetchMock.mock.calls) {
    expect(init?.credentials).toBe('include');
    if (url === '/api/health') continue;
    const headers = new Headers(init?.headers);
    expect(headers.get('X-Pulse-Org-ID')).toBe(orgID);
    expect(headers.get('X-API-Token')).toBe(token);
    if (url !== capabilityPath) expect(init?.signal).toBeInstanceOf(AbortSignal);
    if (['PUT', 'DELETE'].includes(init?.method ?? 'GET'))
      expect(headers.get('X-CSRF-Token')).toBe(csrf);
    for (const internal of ['expectedOrgID', 'retry', 'preferSessionAuth', 'skipOrgContext'])
      expect(init).not.toHaveProperty(internal);
  }
  expect(getOrgID()).toBe(orgID);
  expect(getApiToken()).toBe(token);
}
async function expectFreshInventory(expectedReads = 2) {
  await waitFor(() => {
    expect(reads(directoryPath)).toHaveLength(expectedReads);
    for (const user of savedUsers) expect(rowFor(user).getByText(unlistedRole)).toBeInTheDocument();
  });
  expect(reads(rolesPath)).toHaveLength(expectedReads);
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(search()).toHaveValue('');
  expectNoObsoleteFeedback();
}

beforeEach(async () => {
  clearAuth();
  sessionStorage.clear();
  localStorage.clear();
  setOrgID(orgID);
  setApiToken(token);
  vi.stubEnv('MODE', 'production');
  document.cookie = `pulse_csrf=${csrf}; Path=/`;
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode: false,
      readOnly: false,
      hideCommercial: false,
      hideUpgrade: true,
    },
  });
  savedUsers = structuredClone(originals);
  unexpected = [];
  capabilityReply = async () => json({ capabilities: ['rbac'], limits: [] });
  healthReply = async () => json({});
  permissionReply = () => json([{ action: 'read', resource: `saved:${selected.username}` }]);
  mutationReply = () => new Response(null, { status: 204 });
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    const path = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET') {
      if (path === capabilityPath) return capabilityReply();
      if (path === '/api/health') return healthReply();
      if (path === directoryPath) return json(savedUsers);
      if (path === rolesPath) return json(roles);
      if (path === `${userPath()}/permissions`) return permissionReply();
    }
    // Apply only the identity and roles actually dispatched. A later local
    // abort cannot roll back a write the synthetic server has already applied.
    const user = savedUsers.find((candidate) =>
      [userPath(candidate), `${userPath(candidate)}/roles`].includes(path),
    );
    if (user && method === 'PUT' && path.endsWith('/roles')) {
      const { roleIds } = JSON.parse(String(init.body));
      savedUsers = savedUsers.map((candidate) =>
        candidate.username === user.username ? { ...candidate, roleIds } : candidate,
      );
      return mutationReply(init);
    }
    if (user && method === 'DELETE' && path === userPath(user)) {
      savedUsers = savedUsers.filter((candidate) => candidate.username !== user.username);
      return mutationReply(init);
    }
    unexpected.push(`${method} ${path}`);
    throw new Error('Unexpected synthetic User Access request');
  });
  vi.stubGlobal('fetch', fetchMock);
  // Each mount starts with an actual accepted HTTP capability read, not a
  // mocked gate or a dependency on whichever test last populated the store.
  await loadRuntimeCapabilities(true);
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  syncSessionPresentationPolicy(null);
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('User Access through real runtime capability transitions', () => {
  it('keeps an admitted draft and pending saved-permissions body across an equal capability refresh', async () => {
    const oldPermissions = heldBody();
    permissionReply = () => oldPermissions.response;
    await openPanel();
    fireEvent.input(search(), { target: { value: selected.email } });
    await openEditor(false);
    await waitFor(() => expect(oldPermissions.read).toHaveBeenCalledOnce());
    const permissionSignal = reads(`${userPath()}/permissions`)[0][1]?.signal;
    fireEvent.click(roleChoice('Operate nodes'));
    await capabilities('enabled');
    expect(search()).toHaveValue(selected.email);
    expect(roleChoice('Read nodes')).toBeChecked();
    expect(roleChoice('Operate nodes')).toBeChecked();
    expect(permissionSignal?.aborted).toBe(false);
    oldPermissions.resolve(JSON.stringify([{ action: 'read', resource: 'accepted-saved-nodes' }]));
    await editor().findByText('accepted-saved-nodes');
    expect(reads(directoryPath)).toHaveLength(1);
    expect(reads(rolesPath)).toHaveLength(1);
    expect(reads(`${userPath()}/permissions`)).toHaveLength(1);
    expect(writes()).toHaveLength(0);
    expect(savedUsers).toEqual(originals);
    expectBoundRequests();
  });

  it.each(
    operations.flatMap((operation) =>
      (['withdrawn', 'failed', 'malformed'] as const).map((kind) => ({
        operation,
        kind,
      })),
    ),
  )(
    'retires a cold $operation confirmation on $kind capability before dispatch',
    async ({ operation, kind }) => {
      const health = deferred<Response>();
      healthReply = () => health.promise;
      await openPanel();
      const confirmation = await prepare(operation);
      document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
      fireEvent.click(confirmation);
      await waitFor(() => expect(reads('/api/health')).toHaveLength(1));
      expect(writes()).toHaveLength(0);
      // Only advance the existing ten-millisecond CSRF-cookie delay. All page,
      // capability, HTTP parsing and request-retirement behavior stays real.
      vi.useFakeTimers();
      await capabilities(kind);
      expectWithdrawn();
      expect(Boolean(runtimeCapabilitiesLoadError())).toBe(kind === 'failed');
      expect(reads(capabilityPath)).toHaveLength(2);
      document.cookie = `pulse_csrf=${csrf}; Path=/`;
      health.resolve(json({}));
      await vi.advanceTimersByTimeAsync(25);
      vi.useRealTimers();
      expect(writes()).toHaveLength(0);
      expect(reads('/api/health')).toHaveLength(1);
      expect(reads(directoryPath)).toHaveLength(1);
      expect(savedUsers).toEqual(originals);
      expectNoObsoleteFeedback();
      await capabilities('enabled');
      await expectFreshInventory();
      await openEditor();
      expect(roleChoice('Read nodes')).toBeChecked();
      expect(roleChoice('Operate nodes')).not.toBeChecked();
      expect(writes()).toHaveLength(0);
      expectBoundRequests();
    },
  );

  it.each(
    operations.flatMap((operation) =>
      (['success', 'unreadable'] as const).map((outcome) => ({
        operation,
        outcome,
      })),
    ),
  )(
    'rereads applied $operation state without reviving its late $outcome body after capability restoration',
    async ({ operation, outcome }) => {
      const reply = heldBody();
      mutationReply = () => reply.response;
      await openPanel();
      fireEvent.click(await prepare(operation));
      await waitFor(() => expect(reply.read).toHaveBeenCalledOnce());
      expect(writes()).toHaveLength(1);
      const [path, init] = writes()[0];
      expect(path).toBe(operation === 'save' ? `${userPath()}/roles` : userPath());
      expect(init?.method).toBe(operation === 'save' ? 'PUT' : 'DELETE');
      const expectedUsers = expectedSavedUsers(operation);
      expect(savedUsers).toEqual(expectedUsers);
      await capabilities('withdrawn');
      expectWithdrawn();
      expect(init?.signal?.aborted).toBe(true);
      expect(reads(directoryPath)).toHaveLength(1);
      await capabilities('enabled');
      await expectFreshInventory();
      if (operation === 'save') {
        await openEditor();
        expect(roleChoice('Read nodes')).not.toBeChecked();
        expect(roleChoice('Operate nodes')).toBeChecked();
      } else {
        expect(screen.queryByTitle(selected.username)).toBeNull();
      }
      if (outcome === 'success') reply.resolve('');
      else reply.reject(new Error('synthetic-private-unreadable-write-body'));
      await turn();
      expectNoObsoleteFeedback();
      expect(writes()).toHaveLength(1);
      expect(reads('/api/health')).toHaveLength(0);
      expect(reads(directoryPath)).toHaveLength(2);
      expect(savedUsers).toEqual(expectedUsers);
      expectBoundRequests();
    },
  );

  it.each(operations)(
    'recovers applied %s state when the dispatched transport rejects on capability withdrawal',
    async (operation) => {
      const response = deferred<Response>();
      const observedAbort = vi.fn();
      mutationReply = (init) => {
        init.signal!.addEventListener(
          'abort',
          () => {
            observedAbort();
            response.reject(new DOMException('Synthetic request aborted', 'AbortError'));
          },
          { once: true },
        );
        return response.promise;
      };
      await openPanel();
      fireEvent.click(await prepare(operation));
      await waitFor(() => expect(writes()).toHaveLength(1));
      const [path, init] = writes()[0];
      expect(path).toBe(operation === 'save' ? `${userPath()}/roles` : userPath());
      expect(init?.method).toBe(operation === 'save' ? 'PUT' : 'DELETE');
      expect(init?.signal?.aborted).toBe(false);
      expect(savedUsers).toEqual(expectedSavedUsers(operation));
      await capabilities('withdrawn');
      expectWithdrawn();
      await waitFor(() => expect(observedAbort).toHaveBeenCalledOnce());
      expect(init?.signal?.aborted).toBe(true);
      expectNoObsoleteFeedback();
      expect(reads(directoryPath)).toHaveLength(1);
      await capabilities('enabled');
      await expectFreshInventory();
      if (operation === 'save') {
        await openEditor();
        expect(roleChoice('Read nodes')).not.toBeChecked();
        expect(roleChoice('Operate nodes')).toBeChecked();
      } else {
        expect(screen.queryByTitle(selected.username)).toBeNull();
      }
      expect(writes()).toHaveLength(1);
      expect(savedUsers).toEqual(expectedSavedUsers(operation));
      expectBoundRequests();
    },
  );

  it('never imports a retired permissions body into the restored same-principal editor', async () => {
    const oldPermissions = heldBody();
    permissionReply = () => oldPermissions.response;
    await openPanel();
    await openEditor(false);
    await waitFor(() => expect(oldPermissions.read).toHaveBeenCalledOnce());
    const oldSignal = reads(`${userPath()}/permissions`)[0][1]?.signal;
    fireEvent.click(roleChoice('Operate nodes'));
    await capabilities('withdrawn');
    expectWithdrawn();
    expect(oldSignal?.aborted).toBe(true);
    permissionReply = () => json([{ action: 'read', resource: 'fresh-saved-nodes' }]);
    await capabilities('enabled');
    await expectFreshInventory();
    await openEditor(false);
    await editor().findByText('fresh-saved-nodes');
    expect(roleChoice('Operate nodes')).not.toBeChecked();
    oldPermissions.resolve(
      JSON.stringify([{ action: 'write', resource: 'synthetic-private-retired-nodes' }]),
    );
    await turn();
    expect(editor().getByText('fresh-saved-nodes')).toBeInTheDocument();
    expect(screen.queryByText('synthetic-private-retired-nodes')).toBeNull();
    expect(roleChoice('Operate nodes')).not.toBeChecked();
    expect(reads(`${userPath()}/permissions`)).toHaveLength(2);
    expect(writes()).toHaveLength(0);
    expect(savedUsers).toEqual(originals);
    expectBoundRequests();
  });
});
