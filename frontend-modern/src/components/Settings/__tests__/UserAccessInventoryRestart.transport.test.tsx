import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { eventBus } from '@/stores/events';
import type { Role, UserRoleAssignment } from '@/types/rbac';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';

const gate = vi.hoisted(() => ({ ready: true }));
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => gate.ready,
    rbacEnabled: () => gate.ready,
    paywallVisible: () => false,
  }),
}));

const USERS = '/api/admin/users';
const ROLES = '/api/admin/roles';
const savedRole: Role = {
  id: 'reader',
  name: 'Fleet Reader',
  description: 'Read the fleet',
  permissions: [],
};
const extraRole: Role = {
  id: 'operator',
  name: 'Fleet Operator',
  description: 'Operate the fleet',
  permissions: [],
};
const account: UserRoleAssignment = { username: 'north.reader', roleIds: [savedRole.id] };
const json = (data: unknown, status = 200) =>
  new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
const deferredResponse = () => {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};

const fetchMock = vi.fn<typeof fetch>();
const organizationOf = (init?: RequestInit) => new Headers(init?.headers).get('X-Pulse-Org-ID');
type Read = (attempt: number, org: string | null) => Response | Promise<Response>;
let usersRead: Read;
let rolesRead: Read;
let usersReads: number;
let rolesReads: number;
const inventory = () => screen.getByRole('region', { name: 'User Access' });
const restart = () => screen.getByRole('button', { name: 'Restart loading' });
const edit = (user = account.username) =>
  screen.getByRole('button', { name: `Manage access for ${user}` });
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => !['GET', 'HEAD'].includes(init?.method ?? 'GET'));

beforeEach(() => {
  gate.ready = true;
  clearAuth();
  sessionStorage.clear();
  setOrgID('north');
  setApiToken('synthetic-inventory-restart');
  document.cookie = 'pulse_csrf=synthetic-restart; Path=/';
  vi.stubEnv('MODE', 'production');
  usersReads = rolesReads = 0;
  usersRead = () => json([account]);
  rolesRead = () => json([savedRole, extraRole]);
  fetchMock.mockReset().mockImplementation(async (input, init) => {
    const path = String(input);
    const method = init?.method ?? 'GET';
    if (method === 'GET' && path === USERS) return usersRead(++usersReads, organizationOf(init));
    if (method === 'GET' && path === ROLES) return rolesRead(++rolesReads, organizationOf(init));
    if (method === 'GET' && path === `${USERS}/${account.username}/permissions`) return json([]);
    if (method === 'PUT' && path === `${USERS}/${account.username}/roles`) {
      return json({ code: 'synthetic_write_indeterminate' }, 503);
    }
    throw new Error(`Unexpected fixture request: ${method} ${path}`);
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

describe('User Access inventory restart', () => {
  it('replaces a stalled catalogue read without retaining old rows or losing keyboard focus', async () => {
    const stalledRoles = deferredResponse();
    const currentUsers = deferredResponse();
    rolesRead = (attempt) => (attempt === 1 ? stalledRoles.promise : json([savedRole]));
    usersRead = (attempt) =>
      attempt === 1
        ? json([{ ...account, username: 'superseded.directory' }])
        : currentUsers.promise;
    render(() => <UserAssignmentsPanel currentUser="maintainer" canManage />);
    await waitFor(() => expect(rolesReads).toBe(1));
    expect(screen.queryByText('superseded.directory')).not.toBeInTheDocument();
    expect(screen.queryByRole('status', { name: 'User search results' })).not.toBeInTheDocument();

    const firstPair = fetchMock.mock.calls.slice();
    restart().focus();
    fireEvent.click(restart());
    await waitFor(() => expect(rolesReads).toBe(2));
    expect(inventory()).toHaveFocus();
    expect(firstPair).toHaveLength(2);
    expect(firstPair.every(([, init]) => init?.signal?.aborted)).toBe(true);
    expect(fetchMock.mock.calls.every(([, init]) => organizationOf(init) === 'north')).toBe(true);

    // The transport can still finish locally after cancellation. It must not
    // publish the completed old pair or mark the replacement as finished.
    stalledRoles.resolve(json([extraRole]));
    await stalledRoles.promise;
    expect(restart()).toBeEnabled();
    expect(screen.queryByText('superseded.directory')).not.toBeInTheDocument();
    currentUsers.resolve(json([account]));
    await waitFor(() => expect(edit()).toBeEnabled());
    expect(screen.getByText('Fleet Reader')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Restart loading' })).not.toBeInTheDocument();
    expect(inventory()).toHaveFocus();
    expect(writes()).toEqual([]);
    expect(usersReads).toBe(2);
    expect(rolesReads).toBe(2);
  });

  it('does not revive a replacement that finishes after the organization is withdrawn', async () => {
    const formerUsers = deferredResponse();
    const replacementUsers = deferredResponse();
    usersRead = (attempt, org) =>
      org === 'south'
        ? json([{ ...account, username: 'south.reader' }])
        : attempt === 1
          ? formerUsers.promise
          : replacementUsers.promise;
    render(() => <UserAssignmentsPanel currentUser="maintainer" canManage />);
    fireEvent.click(await screen.findByRole('button', { name: 'Restart loading' }));
    await waitFor(() => expect(usersReads).toBe(2));
    const replacement = fetchMock.mock.calls[2][1]?.signal;
    setOrgID('south');
    eventBus.emit('org_switched', 'south');
    await waitFor(() => expect(edit('south.reader')).toBeEnabled());
    expect(replacement?.aborted).toBe(true);
    replacementUsers.resolve(json([account]));
    formerUsers.resolve(json([account]));
    await Promise.all([replacementUsers.promise, formerUsers.promise]);
    expect(screen.queryByText(account.username)).not.toBeInTheDocument();
    expect(edit('south.reader')).toBeEnabled();
    expect(writes()).toEqual([]);
  });

  it('keeps an unconfirmed change guarded after a restarted check is denied', async () => {
    const stalledRoles = deferredResponse();
    rolesRead = (attempt) => {
      if (attempt === 2) return stalledRoles.promise;
      if (attempt === 3) return json({ code: 'forbidden' }, 403);
      return json([savedRole, extraRole]);
    };
    render(() => <UserAssignmentsPanel currentUser="maintainer" canManage />);
    await waitFor(() => expect(edit()).toBeEnabled());
    fireEvent.click(edit());
    const dialog = within(await screen.findByRole('dialog'));
    fireEvent.click(dialog.getByRole('checkbox', { name: /Operate the fleet/ }));
    fireEvent.click(dialog.getByRole('button', { name: 'Save Changes' }));

    fireEvent.click(await screen.findByRole('button', { name: 'Check saved access' }));
    const pending = await screen.findByRole('button', { name: 'Restart loading' });
    fireEvent.click(pending);
    const retry = await screen.findByRole('button', { name: 'Check saved access' });
    expect(screen.getByText(/role update.*could not be confirmed/)).toBeVisible();
    expect(screen.getByText('Failed to load user assignments')).toBeVisible();
    expect(screen.queryByText(account.username)).not.toBeInTheDocument();
    expect(writes()).toHaveLength(1);

    stalledRoles.resolve(json([savedRole]));
    await stalledRoles.promise;
    expect(retry).toBeEnabled();
    fireEvent.click(retry);
    await waitFor(() => expect(edit()).toBeEnabled());
    expect(screen.queryByText(/role update.*could not be confirmed/)).not.toBeInTheDocument();
    expect(writes()).toHaveLength(1);
    expect(rolesReads).toBe(4);
  });

  it('does not offer a restart while the RBAC gate is unresolved', () => {
    gate.ready = false;
    render(() => <UserAssignmentsPanel currentUser="maintainer" canManage />);
    expect(screen.getByRole('status', { name: 'Loading user access' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Restart loading' })).not.toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
