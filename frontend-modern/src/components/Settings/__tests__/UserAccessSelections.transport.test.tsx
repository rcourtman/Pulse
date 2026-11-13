import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { Permission, Role, UserRoleAssignment } from '@/types/rbac';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';

// Keep the actual grid, dialogs, state owner, RBACAPI and HTTP client connected.
// Calling handlers directly or mocking RBACAPI cannot detect a miswired control
// that submits the identity of a different same-name user or role.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));

const orgID = 'estate/north';
const authToken = 'synthetic-only-user-access-token';
const csrfToken = 'synthetic-only-user-access-csrf';
const roles: Role[] = [
  {
    id: 'read/shared:monitoring',
    name: 'Fleet role',
    description: 'Read saved monitoring',
    permissions: [{ action: 'read', resource: 'nodes' }],
  },
  {
    id: 'write/selected:resources',
    name: 'Fleet role',
    description: 'Operate selected resources',
    permissions: [{ action: 'write', resource: 'selected-nodes' }],
  },
];
const unlistedRole = 'saved/unlisted:constraint';
const originalUsers: UserRoleAssignment[] = ['first', 'second', 'third'].map((part) => ({
  username: `sso:oidc:lab:subject/${part}?realm=north#saved`,
  displayName: 'Alex Example',
  email: `${part}@${part === 'first' ? 'other' : 'shared'}.invalid`,
  providerType: 'oidc',
  providerId: 'lab',
  roleIds: [roles[0].id, unlistedRole],
}));
const userPath = (username: string) => `/api/admin/users/${encodeURIComponent(username)}`;
const fetchMock = vi.fn<typeof fetch>();
const json = (body: unknown) =>
  new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } });
let savedUsers: UserRoleAssignment[];
let catalogue: Role[];
const reads = (path: string) =>
  fetchMock.mock.calls.filter(([url, init]) => url === path && !init?.method);
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => ['PUT', 'DELETE'].includes(init?.method ?? 'GET'));

function rowFor(user: UserRoleAssignment) {
  const row = screen.getByTitle(user.username).closest('tr');
  expect(row).not.toBeNull();
  return within(row!);
}

function expectBoundRequests() {
  for (const [, init] of fetchMock.mock.calls) {
    expect(init).toMatchObject({
      credentials: 'include',
      signal: expect.any(AbortSignal),
      headers: { 'X-Pulse-Org-ID': orgID, 'X-API-Token': authToken },
    });
  }
  for (const [, init] of writes())
    expect(init?.headers).toMatchObject({ 'X-CSRF-Token': csrfToken });
  expect(getOrgID()).toBe(orgID);
}

async function openPanel() {
  render(() => <UserAssignmentsPanel currentUser="access-maintainer" canManage />);
  await screen.findByTitle(originalUsers[2].username);
  expect(screen.getAllByText('Alex Example')).toHaveLength(3);
  expect(reads('/api/admin/users')).toHaveLength(1);
  expect(reads('/api/admin/roles')).toHaveLength(1);
  expect(writes()).toHaveLength(0);
}

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID(orgID);
  setApiToken(authToken);
  vi.stubEnv('MODE', 'production');
  document.cookie = `pulse_csrf=${csrfToken}; Path=/`;
  savedUsers = structuredClone(originalUsers);
  catalogue = structuredClone(roles);
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    const path = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET' && path === '/api/admin/users') return json(savedUsers);
    if (method === 'GET' && path === '/api/admin/roles') return json(catalogue);
    // Derive fixture mutations and permission reads from the actual dispatched
    // identity and body, never from the account the test intended to select.
    const user = savedUsers.find((candidate) =>
      [
        userPath(candidate.username),
        `${userPath(candidate.username)}/roles`,
        `${userPath(candidate.username)}/permissions`,
      ].includes(path),
    );
    if (user && method === 'GET' && path.endsWith('/permissions')) {
      const permissions: Permission[] = [{ action: 'read', resource: `saved:${user.username}` }];
      return json(permissions);
    }
    if (user && method === 'PUT' && path.endsWith('/roles')) {
      const { roleIds } = JSON.parse(String(init.body));
      savedUsers = savedUsers.map((candidate) =>
        candidate.username === user.username ? { ...candidate, roleIds } : candidate,
      );
      return new Response(null, { status: 204 });
    }
    if (user && method === 'DELETE' && path === userPath(user.username)) {
      savedUsers = savedUsers.filter((candidate) => candidate.username !== user.username);
      return new Response(null, { status: 204 });
    }
    throw new Error(`Unexpected synthetic user-access request: ${method} ${path}`);
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

describe('User Access selection through rendered controls and real transport', () => {
  it('keeps both namesakes distinct and an unlisted saved key visible without making it toggleable', async () => {
    catalogue = [
      structuredClone(roles[0]),
      { ...structuredClone(roles[0]), id: roles[1].id },
      { ...structuredClone(roles[0]), id: 'known/other', name: unlistedRole },
      { ...structuredClone(roles[0]), id: 'known/unnamed', name: '' },
    ];
    const selected = originalUsers[1];
    savedUsers[1].roleIds = [roles[0].id, roles[1].id, unlistedRole];
    await openPanel();
    const row = rowFor(selected);
    expect(row.getAllByText('Fleet role')).toHaveLength(2);
    expect(row.getByText(roles[0].id)).toBeInTheDocument();
    expect(row.getByText(roles[1].id)).toBeInTheDocument();
    expect(row.getByText(unlistedRole)).toBeInTheDocument();
    expect(row.getByText('Not in role list')).toBeInTheDocument();
    fireEvent.click(
      row.getByRole('button', {
        name: `Manage access for Alex Example (${selected.username})`,
      }),
    );
    const dialog = within(
      await screen.findByRole('dialog', {
        name: `Manage access: Alex Example (${selected.username})`,
      }),
    );
    const labels = [
      `Fleet role (Role ID: ${roles[0].id}) Read saved monitoring`,
      `Fleet role (Role ID: ${roles[1].id}) Read saved monitoring`,
      `${unlistedRole} (Role ID: known/other) Read saved monitoring`,
      'Role ID: known/unnamed Read saved monitoring',
    ];
    for (const name of labels) expect(dialog.getAllByRole('checkbox', { name })).toHaveLength(1);
    expect(dialog.getByRole('checkbox', { name: labels[0] })).toBeChecked();
    expect(dialog.getByRole('checkbox', { name: labels[1] })).toBeChecked();
    expect(dialog.getByRole('checkbox', { name: labels[2] })).not.toBeChecked();
    const kept = within(dialog.getByRole('list', { name: 'Kept saved role IDs' }));
    expect(kept.getByText(unlistedRole)).toBeInTheDocument();
    expect(kept.queryByRole('checkbox')).not.toBeInTheDocument();
    fireEvent.click(dialog.getByRole('checkbox', { name: labels[1] }));
    expect(writes()).toHaveLength(0);
    fireEvent.click(dialog.getByRole('button', { name: 'Save Changes' }));
    await screen.findByText(`Roles updated for ${selected.username}`);
    expect(writes()).toHaveLength(1);
    const [url, init] = writes()[0];
    expect(url).toBe(`${userPath(selected.username)}/roles`);
    expect(JSON.parse(String(init?.body))).toEqual({ roleIds: [roles[0].id, unlistedRole] });
    expect(savedUsers[0]).toEqual(originalUsers[0]);
    expect(savedUsers[2]).toEqual(originalUsers[2]);
    expect(rowFor(selected).getByText(unlistedRole)).toBeInTheDocument();
    expectBoundRequests();
  });

  it('saves only the selected namesake account and role IDs, preserving an unlisted saved grant', async () => {
    await openPanel();
    const selected = originalUsers[1];
    fireEvent.click(
      rowFor(selected).getByRole('button', {
        name: `Manage access for Alex Example (${selected.username})`,
      }),
    );
    const dialog = await screen.findByRole('dialog', {
      name: `Manage access: Alex Example (${selected.username})`,
    });
    const editor = within(dialog);
    expect(editor.getByText(selected.email!)).toBeInTheDocument();
    expect(editor.getByText(selected.username)).toBeInTheDocument();
    await editor.findByText(`saved:${selected.username}`);
    expect(reads(`${userPath(selected.username)}/permissions`)).toHaveLength(1);
    for (const sibling of [originalUsers[0], originalUsers[2]])
      expect(reads(`${userPath(sibling.username)}/permissions`)).toHaveLength(0);

    const reader = editor.getByRole('checkbox', {
      name: `Fleet role (Role ID: ${roles[0].id}) Read saved monitoring`,
    });
    const operator = editor.getByRole('checkbox', {
      name: `Fleet role (Role ID: ${roles[1].id}) Operate selected resources`,
    });
    expect(reader).toBeChecked();
    expect(operator).not.toBeChecked();
    fireEvent.click(reader);
    fireEvent.click(operator);
    expect(reader).not.toBeChecked();
    expect(operator).toBeChecked();
    // A draft does not change saved permissions or issue any mutation.
    expect(editor.getByText(`saved:${selected.username}`)).toBeInTheDocument();
    expect(writes()).toHaveLength(0);
    expect(savedUsers).toEqual(originalUsers);

    fireEvent.click(editor.getByRole('button', { name: 'Save Changes' }));
    await screen.findByText(`Roles updated for ${selected.username}`);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(writes()).toHaveLength(1);
    const [url, init] = writes()[0];
    const expectedRoleIds = [unlistedRole, roles[1].id];
    expect(url).toBe(`${userPath(selected.username)}/roles`);
    expect(init?.method).toBe('PUT');
    expect(JSON.parse(String(init?.body))).toEqual({ roleIds: expectedRoleIds });
    expect(savedUsers).toEqual(
      originalUsers.map((user) =>
        user.username === selected.username ? { ...user, roleIds: expectedRoleIds } : user,
      ),
    );
    expect(reads('/api/admin/users')).toHaveLength(2);
    expect(reads('/api/admin/roles')).toHaveLength(2);
    for (const user of originalUsers) {
      expect(rowFor(user).getByText(unlistedRole)).toBeInTheDocument();
      expect(rowFor(user).getAllByText('Fleet role')).toHaveLength(1);
    }

    // Reopen from the reread grid proxy: checkbox state must reflect the saved
    // IDs, not the identically named catalogue entry or an old row object.
    fireEvent.click(
      rowFor(selected).getByRole('button', {
        name: `Manage access for Alex Example (${selected.username})`,
      }),
    );
    const reopened = within(
      await screen.findByRole('dialog', {
        name: `Manage access: Alex Example (${selected.username})`,
      }),
    );
    await reopened.findByText(`saved:${selected.username}`);
    expect(
      reopened.getByRole('checkbox', {
        name: `Fleet role (Role ID: ${roles[0].id}) Read saved monitoring`,
      }),
    ).not.toBeChecked();
    expect(
      reopened.getByRole('checkbox', {
        name: `Fleet role (Role ID: ${roles[1].id}) Operate selected resources`,
      }),
    ).toBeChecked();
    expect(writes()).toHaveLength(1);
    expectBoundRequests();
  });

  it('cancels one removal and confirms a different filtered namesake without revoking either sibling', async () => {
    await openPanel();
    fireEvent.input(screen.getByPlaceholderText('Search users...'), {
      target: { value: 'shared.invalid' },
    });
    expect(screen.queryByTitle(originalUsers[0].username)).not.toBeInTheDocument();
    expect(screen.getByRole('status', { name: 'User search results' })).toHaveTextContent(
      'Showing 2 of 3 users',
    );
    const cancelled = originalUsers[1];
    fireEvent.click(
      rowFor(cancelled).getByRole('button', {
        name: `Remove Alex Example (${cancelled.username})`,
      }),
    );
    const firstConfirmation = await screen.findByRole('dialog', {
      name: `Remove user access: Alex Example (${cancelled.username})`,
    });
    expect(within(firstConfirmation).getByText(cancelled.username)).toBeInTheDocument();
    expect(writes()).toHaveLength(0);
    fireEvent.click(within(firstConfirmation).getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(savedUsers).toEqual(originalUsers);

    const selected = originalUsers[2];
    fireEvent.click(
      rowFor(selected).getByRole('button', { name: `Remove Alex Example (${selected.username})` }),
    );
    const confirmation = await screen.findByRole('dialog', {
      name: `Remove user access: Alex Example (${selected.username})`,
    });
    expect(confirmation).toHaveTextContent('revokes active Pulse sessions');
    expect(within(confirmation).getByText(selected.username)).toBeInTheDocument();
    expect(within(confirmation).queryByText(cancelled.username)).not.toBeInTheDocument();
    expect(confirmation).toHaveTextContent('does not disable the account at the identity provider');
    expect(writes()).toHaveLength(0);
    fireEvent.click(within(confirmation).getByRole('button', { name: 'Remove User Access' }));
    await waitFor(() => expect(reads('/api/admin/users')).toHaveLength(2));
    await screen.findByText(`User access removed for Alex Example (${selected.username})`);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(writes()).toHaveLength(1);
    const [url, init] = writes()[0];
    expect(url).toBe(userPath(selected.username));
    expect(init).toMatchObject({ method: 'DELETE' });
    expect(init?.body).toBeUndefined();
    expect(savedUsers).toEqual(originalUsers.slice(0, 2));
    expect(screen.queryByTitle(selected.username)).not.toBeInTheDocument();
    expect(rowFor(cancelled).getByText(unlistedRole)).toBeInTheDocument();

    fireEvent.input(screen.getByPlaceholderText('Search users...'), { target: { value: '' } });
    for (const sibling of originalUsers.slice(0, 2))
      expect(
        rowFor(sibling).getByRole('button', {
          name: `Manage access for Alex Example (${sibling.username})`,
        }),
      ).toBeEnabled();
    expect(screen.getAllByText('Alex Example')).toHaveLength(2);
    expect(reads('/api/admin/users')).toHaveLength(2);
    expect(reads('/api/admin/roles')).toHaveLength(2);
    for (const user of originalUsers)
      expect(reads(`${userPath(user.username)}/permissions`)).toHaveLength(0);
    expectBoundRequests();
  });
});

describe('User Access writes only changed role membership', () => {
  const openSelected = async () => {
    const selected = originalUsers[1];
    await openPanel();
    fireEvent.click(
      rowFor(selected).getByRole('button', {
        name: `Manage access for Alex Example (${selected.username})`,
      }),
    );
    const editor = within(
      await screen.findByRole('dialog', {
        name: `Manage access: Alex Example (${selected.username})`,
      }),
    );
    await editor.findByText(`saved:${selected.username}`);
    return editor;
  };
  const readerName = `Fleet role (Role ID: ${roles[0].id}) Read saved monitoring`;
  const operatorName = `Fleet role (Role ID: ${roles[1].id}) Operate selected resources`;

  it.each(
    [[], [roles[0].id, unlistedRole], [roles[0].id, roles[0].id, unlistedRole]].map(
      (roleIds) => ({ roleIds }),
    ),
  )(
    'keeps an unchanged saved selection read-only, including empty or repeated IDs: %j',
    async ({ roleIds }) => {
      savedUsers[1].roleIds = [...roleIds];
      const editor = await openSelected();
      const save = editor.getByRole('button', { name: 'Save Changes' });
      expect(save).toBeDisabled();
      fireEvent.click(save);
      expect(writes()).toHaveLength(0);
      expect(reads('/api/admin/users')).toHaveLength(1);
      expect(editor.getByText(`saved:${originalUsers[1].username}`)).toBeInTheDocument();
      fireEvent.click(editor.getByRole('button', { name: 'Cancel' }));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(savedUsers[1].roleIds).toEqual(roleIds);
      expectBoundRequests();
    },
  );

  it('disables a reverted selection even when toggling changed its order and keeps unlisted IDs', async () => {
    savedUsers[1].roleIds = [roles[0].id, roles[1].id, unlistedRole];
    const editor = await openSelected();
    const save = editor.getByRole('button', { name: 'Save Changes' });
    const reader = editor.getByRole('checkbox', { name: readerName });
    expect(save).toBeDisabled();
    fireEvent.click(reader);
    expect(save).toBeEnabled();
    fireEvent.click(reader);
    expect(save).toBeDisabled();
    fireEvent.click(save);
    expect(writes()).toHaveLength(0);
    expect(editor.getByRole('list', { name: 'Kept saved role IDs' })).toHaveTextContent(unlistedRole);
    // A real removal is still admitted once with the original unlisted key.
    fireEvent.click(editor.getByRole('checkbox', { name: operatorName }));
    fireEvent.click(save);
    await screen.findByText(`Roles updated for ${originalUsers[1].username}`);
    expect(writes()).toHaveLength(1);
    expect(JSON.parse(String(writes()[0][1]?.body))).toEqual({
      roleIds: [unlistedRole, roles[0].id],
    });
    expectBoundRequests();
  });

  it('still admits an explicit clear to an empty array and starts the next editor unchanged', async () => {
    savedUsers[1].roleIds = [roles[0].id];
    const editor = await openSelected();
    fireEvent.click(editor.getByRole('checkbox', { name: readerName }));
    const save = editor.getByRole('button', { name: 'Save Changes' });
    expect(save).toBeEnabled();
    fireEvent.click(save);
    await screen.findByText(`Roles updated for ${originalUsers[1].username}`);
    expect(writes()).toHaveLength(1);
    expect(JSON.parse(String(writes()[0][1]?.body))).toEqual({ roleIds: [] });
    expect(savedUsers[1].roleIds).toEqual([]);
    fireEvent.click(
      rowFor(originalUsers[1]).getByRole('button', {
        name: `Manage access for Alex Example (${originalUsers[1].username})`,
      }),
    );
    expect(await screen.findByRole('button', { name: 'Save Changes' })).toBeDisabled();
    expect(writes()).toHaveLength(1);
    expectBoundRequests();
  });

  it('compares opaque IDs without case folding or trimming and leaves the submitted keys intact', async () => {
    const distinctIDs = [roles[0].id.toUpperCase(), ` ${roles[0].id}`];
    catalogue.push(
      ...distinctIDs.map((id) => ({ ...roles[0], id, name: 'Exact role key' })),
    );
    const editor = await openSelected();
    const choices = editor.getAllByRole('checkbox', { name: /^Exact role key/ });
    expect(choices).toHaveLength(distinctIDs.length);
    for (const choice of choices) fireEvent.click(choice);
    fireEvent.click(editor.getByRole('button', { name: 'Save Changes' }));
    await screen.findByText(`Roles updated for ${originalUsers[1].username}`);
    expect(writes()).toHaveLength(1);
    expect(JSON.parse(String(writes()[0][1]?.body))).toEqual({
      roleIds: [roles[0].id, unlistedRole, ...distinctIDs],
    });
    expectBoundRequests();
  });
});
