import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { Role, UserRoleAssignment } from '@/types/rbac';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';

// Keep the actual dialog, state, RBAC API and HTTP client connected. Only
// entitlement availability and the final network boundary are synthetic.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));
const roles: Role[] = [
  { id: 'read/nodes', name: 'Reader', description: 'Read monitoring', permissions: [] },
  {
    id: 'operate/selected',
    name: 'Operator',
    description: 'Operate selected nodes',
    permissions: [],
  },
];
const user: UserRoleAssignment = {
  username: 'sso:oidc:lab:subject/second?realm=north#saved',
  displayName: 'Alex Example',
  providerType: 'oidc',
  providerId: 'lab',
  roleIds: [roles[0].id, 'saved/unlisted:constraint'],
};
const principalPath = `/api/admin/users/${encodeURIComponent(user.username)}`;
const permissionsPath = `${principalPath}/permissions`;
const fetchMock = vi.fn<typeof fetch>();
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
const deferredResponse = () => {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
let permissionsReply: (attempt: number) => Response | Promise<Response>;
let mutationReply: Promise<Response> | undefined;
let release: (response: Response) => void;
const permissionReads = () => fetchMock.mock.calls.filter(([url]) => url === permissionsPath);
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => !['GET', 'HEAD'].includes(init?.method ?? 'GET'));

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID('estate/north');
  setApiToken('synthetic-only-permission-recovery-auth');
  vi.stubEnv('MODE', 'production');
  document.cookie = 'pulse_csrf=synthetic-only-permission-recovery-csrf; Path=/';
  const pending = deferredResponse();
  release = pending.resolve;
  permissionsReply = (attempt) =>
    attempt === 1 ? json({ error: 'synthetic read failure' }, 503) : pending.promise;
  mutationReply = undefined;
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    const path = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET' && path === '/api/admin/users') return json([user]);
    if (method === 'GET' && path === '/api/admin/roles') return json(roles);
    if (method === 'GET' && path === permissionsPath)
      return permissionsReply(permissionReads().length);
    if (method === 'PUT' && path === `${principalPath}/roles` && mutationReply)
      return mutationReply;
    throw new Error(`Unexpected synthetic permission-recovery request: ${method} ${path}`);
  });
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  cleanup();
  clearAuth();
  setOrgID(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

async function openEditor() {
  const [reader, setReader] = createSignal({ user: 'access-maintainer', allowed: true });
  const view = render(() => (
    <>
      <button type="button">Outside task</button>
      <UserAssignmentsPanel currentUser={reader().user} canManage={reader().allowed} />
    </>
  ));
  const trigger = await screen.findByRole('button', {
    name: `Manage access for Alex Example (${user.username})`,
  });
  trigger.focus();
  fireEvent.click(trigger);
  const dialog = within(await screen.findByRole('dialog'));
  return { dialog, trigger, setReader, view };
}

async function openFailedRead() {
  const view = await openEditor();
  const { dialog } = view;
  const retry = await dialog.findByRole('button', { name: 'Retry saved permissions' });
  return { ...view, retry };
}

async function startRetry() {
  const view = await openFailedRead();
  view.retry.focus();
  fireEvent.click(view.retry);
  await waitFor(() => expect(permissionReads()).toHaveLength(2));
  return view;
}

function expectReadOnlyBoundRequests() {
  expect(writes()).toHaveLength(0);
  for (const [, init] of fetchMock.mock.calls) {
    expect(init).toMatchObject({
      credentials: 'include',
      signal: expect.any(AbortSignal),
      headers: {
        'X-Pulse-Org-ID': 'estate/north',
        'X-API-Token': 'synthetic-only-permission-recovery-auth',
      },
    });
  }
}

describe('Saved-permissions recovery through real transport', () => {
  it('replaces the stalled GET for the same saved principal without replacing the draft', async () => {
    const original = deferredResponse();
    const replacement = deferredResponse();
    permissionsReply = (attempt) => (attempt === 1 ? original.promise : replacement.promise);
    const { dialog } = await openEditor();
    const region = dialog.getByRole('region', { name: 'Saved permissions' });
    const operator = dialog.getByRole('checkbox', { name: /Operator \(Role ID:/ });
    fireEvent.click(operator);
    const restart = dialog.getByRole('button', { name: 'Restart saved permissions' });
    expect(region).not.toContainElement(restart);
    expect(permissionReads()).toHaveLength(1);
    restart.focus();
    fireEvent.click(restart);
    await waitFor(() => expect(permissionReads()).toHaveLength(2));
    expect(permissionReads()[0][1]?.signal?.aborted).toBe(true);
    expect(permissionReads()[1][1]?.signal?.aborted).toBe(false);
    expect(region).toHaveFocus();

    original.resolve(json([{ action: 'read', resource: 'superseded/reading' }]));
    await new Promise((done) => setTimeout(done, 0));
    expect(region).toHaveAttribute('aria-busy', 'true');
    expect(dialog.queryByText('superseded/reading')).not.toBeInTheDocument();
    expect(dialog.queryByText(/No permissions were returned/)).not.toBeInTheDocument();
    replacement.resolve(json([{ action: 'read', resource: 'current/reading' }]));
    await within(region).findByText('current/reading');
    expect(region).toHaveAttribute('aria-busy', 'false');
    expect(region).toHaveFocus();
    expect(restart).not.toBeInTheDocument();
    expect(operator).toBeChecked();
    expect(dialog.getByText('saved/unlisted:constraint')).toBeVisible();
    expect(dialog.getByRole('button', { name: 'Save Changes' })).toBeEnabled();
    expect(fetchMock).toHaveBeenCalledTimes(4); // two inventory GETs, two permissions GETs
    expectReadOnlyBoundRequests();
  });

  it('keeps a failed replacement unknown and ignores the late result of the superseded retry', async () => {
    const replacement = deferredResponse();
    const retryReply = deferredResponse();
    permissionsReply = (attempt) => {
      if (attempt === 1) return json({}, 503);
      return attempt === 2 ? retryReply.promise : replacement.promise;
    };
    const { dialog } = await startRetry();
    const region = dialog.getByRole('region', { name: 'Saved permissions' });
    fireEvent.click(dialog.getByRole('button', { name: 'Restart saved permissions' }));
    expect(permissionReads()).toHaveLength(3);
    expect(permissionReads()[1][1]?.signal?.aborted).toBe(true);
    replacement.resolve(json([{ action: 'read' }]));
    const retry = await dialog.findByRole('button', { name: 'Retry saved permissions' });
    expect(retry).toBeEnabled();
    expect(region).toHaveFocus();
    retryReply.resolve(json([]));
    await new Promise((done) => setTimeout(done, 0));
    expect(within(region).getByRole('alert')).toHaveTextContent('Access is unknown.');
    expect(dialog.queryByText(/No permissions were returned/)).not.toBeInTheDocument();
    expect(dialog.queryByRole('button', { name: 'Restart saved permissions' })).toBeNull();
    expect(permissionReads()).toHaveLength(3);
    expectReadOnlyBoundRequests();
  });

  it('retires both reads when the existing editor is withdrawn on an organization change', async () => {
    const original = deferredResponse();
    const replacement = deferredResponse();
    permissionsReply = (attempt) => (attempt === 1 ? original.promise : replacement.promise);
    const { dialog } = await openEditor();
    fireEvent.click(dialog.getByRole('button', { name: 'Restart saved permissions' }));
    expect(permissionReads()).toHaveLength(2);
    setOrgID('estate/south');
    eventBus.emit('org_switched', 'estate/south');
    expect(permissionReads().every(([, init]) => init?.signal?.aborted)).toBe(true);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    original.resolve(json({ error: 'superseded read' }, 503));
    replacement.resolve(json([{ action: 'read', resource: 'withdrawn/reading' }]));
    await new Promise((done) => setTimeout(done, 0));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.queryByText('withdrawn/reading')).not.toBeInTheDocument();
    expect(permissionReads()).toHaveLength(2);
    expect(writes()).toHaveLength(0);
  });

  it('does not restart a read behind a submitted role change', async () => {
    const pendingRead = deferredResponse();
    const pendingWrite = deferredResponse();
    permissionsReply = () => pendingRead.promise;
    mutationReply = pendingWrite.promise;
    const { dialog } = await openEditor();
    fireEvent.click(dialog.getByRole('checkbox', { name: /Operator \(Role ID:/ }));
    fireEvent.click(dialog.getByRole('button', { name: 'Save Changes' }));
    const restart = dialog.getByRole('button', { name: 'Restart saved permissions' });
    expect(restart).toBeDisabled();
    restart.click();
    expect(permissionReads()).toHaveLength(1);
    expect(writes()).toHaveLength(1);
    expect(JSON.parse(String(writes()[0][1]?.body))).toEqual({
      roleIds: [...user.roleIds, roles[1].id],
    });
    pendingWrite.resolve(json({}, 409));
    await dialog.findByText('Failed to update user roles');
    expect(restart).toBeEnabled();
    pendingRead.resolve(json([]));
    expect(await dialog.findByText(/No permissions were returned/)).toBeVisible();
    expect(writes()).toHaveLength(1);
  });

  it.each([
    { outcome: 'saved grants', reply: [{ action: 'read', resource: 'saved/selected:nodes' }] },
    { outcome: 'confirmed empty', reply: [] },
    { outcome: 'malformed response', reply: [{ action: 'read' }] },
  ])(
    'keeps keyboard review at the result during and after $outcome',
    async ({ outcome, reply }) => {
      const { dialog, retry } = await startRetry();
      const region = dialog.getByRole('region', { name: 'Saved permissions' });
      expect(retry).not.toBeInTheDocument();
      expect(region).toHaveAttribute('tabindex', '-1');
      expect(region).toHaveAttribute('aria-busy', 'true');
      expect(region).toHaveFocus();
      release(json(reply));
      await waitFor(() => expect(region).toHaveAttribute('aria-busy', 'false'));
      expect(region).toHaveFocus();
      if (outcome === 'saved grants')
        expect(within(region).getByText('saved/selected:nodes')).toBeVisible();
      else if (outcome === 'confirmed empty')
        expect(
          within(region).getByText("No permissions were returned for this user's saved roles."),
        ).toBeVisible();
      else {
        expect(within(region).getByRole('alert')).toHaveTextContent('Access is unknown.');
        expect(within(region).queryByText(/No permissions were returned/)).not.toBeInTheDocument();
        expect(
          within(region).getByRole('button', { name: 'Retry saved permissions' }),
        ).toBeVisible();
      }
      expect(dialog.getByText(user.username)).toBeVisible();
      expect(dialog.getByRole('checkbox', { name: /Reader \(Role ID:/ })).toBeChecked();
      expect(dialog.getByRole('checkbox', { name: /Operator \(Role ID:/ })).not.toBeChecked();
      expect(dialog.getByText('saved/unlisted:constraint')).toBeVisible();
      expect(permissionReads()).toHaveLength(2);
      expectReadOnlyBoundRequests();
    },
  );

  it('does not steal a deliberate focus move or replace an unsaved role choice', async () => {
    const { dialog } = await startRetry();
    const operator = dialog.getByRole('checkbox', { name: /Operator \(Role ID:/ });
    operator.focus();
    fireEvent.click(operator);
    release(json([{ action: 'read', resource: 'saved/selected:nodes' }]));
    await dialog.findByText('saved/selected:nodes');
    expect(operator).toHaveFocus();
    expect(operator).toBeChecked();
    expect(dialog.getByText(/Unsaved role selections are not included/)).toBeVisible();
    expectReadOnlyBoundRequests();
  });

  it('does not move initial editor focus when a read fails without an explicit retry', async () => {
    const { dialog } = await openFailedRead();
    expect(dialog.getByRole('button', { name: 'Close user assignments' })).toHaveFocus();
    expectReadOnlyBoundRequests();
  });

  it('cancels an in-flight read on Escape without reopening or refocusing the old result', async () => {
    const { trigger } = await startRetry();
    const signal = permissionReads()[1][1]?.signal;
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(signal?.aborted).toBe(true);
    expect(trigger).toHaveFocus();
    release(json([{ action: 'read', resource: 'late/old-reader' }]));
    await Promise.resolve();
    expect(screen.queryByText('late/old-reader')).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
    expectReadOnlyBoundRequests();
  });

  it.each(['permission', 'principal', 'organization', 'unmount'] as const)(
    'never refocuses a withdrawn %s reader after a late completion',
    async (change) => {
      const { setReader, view } = await startRetry();
      const signal = permissionReads()[1][1]?.signal;
      if (change === 'permission') setReader({ user: 'access-maintainer', allowed: false });
      if (change === 'principal') setReader({ user: 'different-maintainer', allowed: true });
      if (change === 'organization') {
        setOrgID('estate/south');
        eventBus.emit('org_switched', 'estate/south');
      }
      if (change === 'unmount') view.unmount();
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(signal?.aborted).toBe(true);
      const deliberateTarget = document.createElement('button');
      deliberateTarget.textContent = 'New task';
      document.body.append(deliberateTarget);
      deliberateTarget.focus();
      release(json([{ action: 'read', resource: 'late/old-reader' }]));
      await Promise.resolve();
      expect(deliberateTarget).toHaveFocus();
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(screen.queryByText('late/old-reader')).not.toBeInTheDocument();
      expect(writes()).toHaveLength(0);
      deliberateTarget.remove();
    },
  );
});
