import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import type { Role, UserRoleAssignment } from '@/types/rbac';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';

// Exercise the actual panel, dialog, state, RBACAPI and HTTP client. Only
// entitlement availability and the final HTTP boundary are synthetic.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));

const roles: Role[] = [
  { id: 'read/nodes', name: 'Fleet role', description: 'Read monitoring', permissions: [] },
  {
    id: 'write/selected',
    name: 'Fleet role',
    description: 'Operate selected nodes',
    permissions: [],
  },
];
const originalUsers: UserRoleAssignment[] = ['first', 'second', 'third'].map((part) => ({
  username: `sso:oidc:lab:subject/${part}?realm=north#saved["literal"]`,
  displayName: 'Alex Example',
  roleIds: [roles[0].id, 'saved/unlisted:constraint'],
}));
const selected = originalUsers[1];
const manageName = (user = selected) => `Manage access for Alex Example (${user.username})`;
const userPath = `/api/admin/users/${encodeURIComponent(selected.username)}`;
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
const fetchMock = vi.fn<typeof fetch>();
let users: UserRoleAssignment[];
let releaseRead: (response: Response) => void;
let userReads: number;
let writeStatus: number;
let writeResponse: Promise<Response> | undefined;
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => !['GET', 'HEAD'].includes(init?.method ?? 'GET'));

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID('estate/north');
  setApiToken('synthetic-saved-focus-token');
  vi.stubEnv('MODE', 'production');
  document.cookie = 'pulse_csrf=synthetic-saved-focus-csrf; Path=/';
  users = structuredClone(originalUsers);
  userReads = 0;
  writeStatus = 204;
  writeResponse = undefined;
  const savedRead = new Promise<Response>((resolve) => {
    releaseRead = resolve;
  });
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    const path = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET' && path === '/api/admin/users') {
      userReads++;
      return userReads === 2 ? savedRead : json(users);
    }
    if (method === 'GET' && path === '/api/admin/roles') return json(roles);
    if (method === 'GET' && path.endsWith('/permissions')) return json([]);
    if (method === 'PUT' && path === `${userPath}/roles`) {
      if (writeResponse) return writeResponse;
      if (writeStatus !== 204) return json({ error: 'Synthetic write outcome' }, writeStatus);
      const { roleIds } = JSON.parse(String(init.body));
      users = users.map((user) =>
        user.username === selected.username ? { ...user, roleIds } : user,
      );
      return new Response(null, { status: 204 });
    }
    throw new Error(`Unexpected saved-focus fixture request: ${method} ${path}`);
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
  const trigger = await screen.findByRole('button', { name: manageName() });
  trigger.focus();
  fireEvent.click(trigger);
  const dialog = within(await screen.findByRole('dialog'));
  await waitFor(() => expect(dialog.getByRole('region')).toHaveAttribute('aria-busy', 'false'));
  const operator = dialog.getByRole('checkbox', { name: /Operate selected nodes/ });
  fireEvent.click(operator);
  const save = dialog.getByRole('button', { name: 'Save Changes' });
  save.focus();
  fireEvent.click(save);
  return { trigger, dialog, operator, setReader, view };
}

async function startSavedRead() {
  const view = await openEditor();
  await waitFor(() => expect(userReads).toBe(2));
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(view.trigger).not.toBeInTheDocument();
  return view;
}

function expectSingleBoundWrite() {
  expect(writes()).toHaveLength(1);
  const [path, init] = writes()[0];
  expect(path).toBe(`${userPath}/roles`);
  expect(JSON.parse(String(init?.body))).toEqual({
    roleIds: [roles[0].id, 'saved/unlisted:constraint', roles[1].id],
  });
  expect(init).toMatchObject({
    method: 'PUT',
    credentials: 'include',
    signal: expect.any(AbortSignal),
    headers: {
      'X-Pulse-Org-ID': 'estate/north',
      'X-API-Token': 'synthetic-saved-focus-token',
      'X-CSRF-Token': 'synthetic-saved-focus-csrf',
    },
  });
}

describe('User Access focus after a confirmed write and saved reread', () => {
  it('returns to the exact replaced principal, not a same-name user or row position', async () => {
    const { trigger } = await startSavedRead();
    users.reverse();
    releaseRead(json(users));
    const replacement = await screen.findByRole('button', { name: manageName() });
    await waitFor(() => expect(replacement).toHaveFocus());
    expect(replacement).not.toBe(trigger);
    for (const sibling of [originalUsers[0], originalUsers[2]])
      expect(screen.getByRole('button', { name: manageName(sibling) })).not.toHaveFocus();
    fireEvent.click(replacement);
    const dialog = within(await screen.findByRole('dialog'));
    expect(dialog.getByRole('checkbox', { name: /Operate selected nodes/ })).toBeChecked();
    expect(dialog.getByText('saved/unlisted:constraint')).toBeVisible();
    expectSingleBoundWrite();
  });

  it('focuses a failed reread, then returns from Retry to the same saved user without another write', async () => {
    await startSavedRead();
    releaseRead(json({ error: 'Synthetic read failure' }, 503));
    const notice = await screen.findByRole('alert');
    await waitFor(() => expect(notice).toHaveFocus());
    expect(screen.queryByRole('button', { name: manageName() })).not.toBeInTheDocument();
    const retry = within(notice).getByRole('button', { name: 'Retry' });
    retry.focus();
    fireEvent.click(retry);
    const replacement = await screen.findByRole('button', { name: manageName() });
    await waitFor(() => expect(replacement).toHaveFocus());
    expect(userReads).toBe(3);
    expectSingleBoundWrite();
  });

  it('uses search rather than a namesake when the saved principal is no longer listed', async () => {
    await startSavedRead();
    users = users.filter((user) => user.username !== selected.username);
    releaseRead(json(users));
    await waitFor(() =>
      expect(screen.getByRole('textbox', { name: 'Search users...' })).toHaveFocus(),
    );
    expect(screen.queryByRole('button', { name: manageName() })).not.toBeInTheDocument();
    expectSingleBoundWrite();
  });

  it.each([200, 503])(
    'does not steal a deliberate focus move while the saved read returns %s',
    async (status) => {
      await startSavedRead();
      const outside = screen.getByRole('button', { name: 'Outside task' });
      outside.focus();
      releaseRead(json(status === 200 ? users : { error: 'Synthetic read failure' }, status));
      if (status === 200) await screen.findByRole('button', { name: manageName() });
      else await screen.findByRole('alert');
      await new Promise((resolve) => window.setTimeout(resolve, 20));
      expect(outside).toHaveFocus();
      expectSingleBoundWrite();
    },
  );

  it.each(['user', 'organization round trip', 'permission loss', 'unmount'])(
    'retires pending focus after %s',
    async (change) => {
      const { setReader, view } = await startSavedRead();
      if (change === 'user') setReader({ user: 'another-maintainer', allowed: true });
      else if (change === 'permission loss')
        setReader({ user: 'access-maintainer', allowed: false });
      else if (change === 'unmount') view.unmount();
      else {
        setOrgID('estate/south');
        eventBus.emit('org_switched', 'estate/south');
        setOrgID('estate/north');
        eventBus.emit('org_switched', 'estate/north');
      }
      const outside = screen.queryByRole('button', { name: 'Outside task' });
      if (change === 'permission loss') outside?.focus();
      releaseRead(json(users));
      if (['user', 'organization round trip'].includes(change))
        await screen.findByRole('button', { name: manageName() });
      else if (change === 'permission loss')
        await screen.findByText('User access is unavailable for this session.');
      await new Promise((resolve) => window.setTimeout(resolve, 20));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      if (change === 'permission loss' && outside) expect(outside).toHaveFocus();
      else if (change !== 'unmount')
        expect(screen.getByRole('button', { name: manageName() })).not.toHaveFocus();
      expect(screen.queryByText(`Roles updated for ${selected.username}`)).not.toBeInTheDocument();
      expectSingleBoundWrite();
    },
  );

  it('leaves a rejected draft in its editor', async () => {
    let releaseWrite!: (response: Response) => void;
    writeResponse = new Promise<Response>((resolve) => {
      releaseWrite = resolve;
    });
    const { dialog, operator } = await openEditor();
    // Real browsers can blur a disabled submit control. jsdom's blur() on a
    // disabled button is a no-op, so explicitly establish the observed BODY
    // focus before resolving the HTTP reply. Do not mock the recovery itself.
    const save = dialog.getByRole('button', { name: 'Applying...' });
    expect(save).toBeDisabled();
    document.body.tabIndex = -1;
    document.body.focus();
    document.body.removeAttribute('tabindex');
    expect(document.body).toHaveFocus();
    releaseWrite(json({ error: 'Synthetic rejection' }, 403));
    const notice = await dialog.findByRole('alert');
    await waitFor(() => expect(notice).toHaveFocus());
    expect(screen.getByRole('dialog')).toContainElement(document.activeElement as HTMLElement);
    expect(operator).toBeChecked();
    expect(userReads).toBe(1);
    expectSingleBoundWrite();
  });

  it('leaves uncertainty recovery in charge rather than refocusing a previous user', async () => {
    writeStatus = 503;
    await openEditor();
    const notice = await screen.findByRole('alert');
    await waitFor(() => expect(notice).toHaveFocus());
    expect(notice).toHaveTextContent('could not be confirmed');
    expect(within(notice).getByRole('button', { name: 'Check saved access' })).toBeVisible();
    expect(userReads).toBe(1);
    expectSingleBoundWrite();
  });
});
