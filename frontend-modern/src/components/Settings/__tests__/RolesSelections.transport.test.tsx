import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { Role } from '@/types/rbac';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { RolesPanel } from '../RolesPanel';

// Keep the real grid, dialogs, state owner, RBACAPI and HTTP client connected.
// Single-role fixtures cannot catch a callback editing or deleting a namesake.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));

const orgID = 'estate/north';
const authToken = 'synthetic-only-role-selection-token';
const csrfToken = 'synthetic-only-role-selection-csrf';
const originalRoles: Role[] = ['first', 'second', 'third', 'built-in'].map((identity, index) => ({
  id: `policy/${identity}?estate=north#saved`,
  name: 'Fleet auditor',
  description: `Saved ${identity} policy`,
  isBuiltIn: identity === 'built-in',
  parentId: 'viewer',
  priority: index + 10,
  createdAt: '2026-10-01T09:00:00Z',
  updatedAt: '2026-11-01T09:00:00Z',
  permissions: [
    { action: 'read', resource: 'nodes' },
    {
      action: 'delete',
      resource: 'users',
      effect: 'deny',
      conditions: { tag: identity, owner: '${user}' },
    },
    { action: 'future:inspect', resource: `future/${identity}` },
  ],
}));
const selected = originalRoles[1];
const rolePath = (id: string) => `/api/admin/roles/${encodeURIComponent(id)}`;
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}
const fetchMock = vi.fn<typeof fetch>();
const reads = () =>
  fetchMock.mock.calls.filter(
    ([url, init]) => url === '/api/admin/roles' && (init?.method ?? 'GET') === 'GET',
  );
const writes = () => fetchMock.mock.calls.filter(([, init]) => (init?.method ?? 'GET') !== 'GET');
let saved: Role[];
let completion: ReturnType<typeof deferred<Response>>;
let reload: ReturnType<typeof deferred<Response>> | undefined;

function rowFor(role: Role) {
  const row = screen.getByText(role.id).closest('tr');
  expect(row).not.toBeNull();
  return within(row!);
}
function expectBoundRequests() {
  for (const [, init] of fetchMock.mock.calls)
    expect(init).toMatchObject({
      credentials: 'include',
      signal: expect.any(AbortSignal),
      headers: { 'X-Pulse-Org-ID': orgID, 'X-API-Token': authToken },
    });
  for (const [, init] of writes())
    expect(init?.headers).toMatchObject({ 'X-CSRF-Token': csrfToken });
  expect(getOrgID()).toBe(orgID);
  expect(getApiToken()).toBe(authToken);
}
async function openPanel() {
  render(() => <RolesPanel currentUser="role-maintainer" canManage />);
  await screen.findByText(selected.description);
  expect(screen.getAllByText('Fleet auditor')).toHaveLength(4);
  expect(reads()).toHaveLength(1);
  expect(writes()).toHaveLength(0);
  expectSavedRows(originalRoles);
}
async function openEditor(role = selected) {
  fireEvent.click(
    rowFor(role).getByRole('button', { name: `Edit role ${role.name} (Role ID: ${role.id})` }),
  );
  const dialog = await screen.findByRole('dialog', {
    name: `Edit role: ${role.name} (Role ID: ${role.id})`,
  });
  const editor = within(dialog);
  expect(editor.getByLabelText('Role ID')).toHaveValue(role.id);
  expect(editor.getByLabelText('Role ID')).toBeDisabled();
  expect(editor.getByLabelText('Role Name')).toHaveValue(role.name);
  expect(editor.getByLabelText('Description')).toHaveValue(role.description);
  expect(editor.getAllByRole('combobox')).toHaveLength(role.permissions.length * 2);
  role.permissions.forEach((permission, index) => {
    expect(editor.getByLabelText(`Permission ${index + 1} action`)).toHaveValue(permission.action);
    expect(editor.getByLabelText(`Permission ${index + 1} resource`)).toHaveValue(
      permission.resource,
    );
  });
  return dialog;
}
async function openDeletion(role = selected) {
  fireEvent.click(
    rowFor(role).getByRole('button', { name: `Delete role ${role.name} (Role ID: ${role.id})` }),
  );
  const dialog = await screen.findByRole('dialog', {
    name: `Delete role: ${role.name} (Role ID: ${role.id})`,
  });
  expect(dialog).toHaveTextContent(`Are you sure you want to delete the role "${role.name}"?`);
  expect(within(dialog).getByText(role.id)).toBeInTheDocument();
  return dialog;
}
async function submit(dialog: HTMLElement, operation: 'edit' | 'delete', role = selected) {
  fireEvent.click(
    within(dialog).getByRole('button', {
      name: operation === 'edit' ? 'Update Role' : 'Delete Role',
    }),
  );
  await waitFor(() => expect(writes()).toHaveLength(1));
  const [url, init] = writes()[0];
  expect(url).toBe(rolePath(role.id));
  expect(init?.method).toBe(operation === 'edit' ? 'PUT' : 'DELETE');
  expect(init?.signal?.aborted).toBe(false);
  expect(within(dialog).getByRole('button', { name: 'Cancel' })).toBeDisabled();
  expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
  expect(saved).toEqual(originalRoles);
  expect(reads()).toHaveLength(1);
  expectBoundRequests();
}
// Only the actual dispatched opaque identity/body changes saved fixture state.
// Do not make a wrong-row request look correct by applying the intended role.
function applyDispatchedWrite() {
  const [url, init] = writes()[0];
  const id = decodeURIComponent(String(url).slice('/api/admin/roles/'.length));
  expect(saved.some((role) => role.id === id)).toBe(true);
  if (init?.method === 'DELETE') saved = saved.filter((role) => role.id !== id);
  else {
    const body = JSON.parse(String(init?.body)) as Role;
    expect(body.id).toBe(id);
    saved = saved.map((role) => (role.id === id ? body : role));
  }
  // A saved reread may reorder the grid. Identity must not become row position.
  saved.reverse();
}
function expectSavedRows(expected: Role[]) {
  expect(saved).toEqual(expect.arrayContaining(expected));
  expect(saved).toHaveLength(expected.length);
  for (const role of expected) {
    const row = rowFor(role);
    expect(row.getByText(role.name)).toBeInTheDocument();
    expect(row.getByText(role.id)).toBeInTheDocument();
    for (const list of row.getAllByRole('list', {
      name: `Permissions for ${role.name} (Role ID: ${role.id})`,
    })) {
      expect(within(list).getAllByRole('listitem')).toHaveLength(role.permissions.length);
      for (const permission of role.permissions)
        expect(
          within(list).getByText(`${permission.action}:${permission.resource}`),
        ).toBeInTheDocument();
    }
    if (role.isBuiltIn) {
      expect(row.getByText('Read-only')).toBeInTheDocument();
      expect(row.queryByRole('button')).not.toBeInTheDocument();
    } else {
      expect(
        row.getByRole('button', { name: `Edit role ${role.name} (Role ID: ${role.id})` }),
      ).toBeEnabled();
      expect(
        row.getByRole('button', { name: `Delete role ${role.name} (Role ID: ${role.id})` }),
      ).toBeEnabled();
    }
  }
  for (const role of originalRoles)
    if (!expected.some((candidate) => candidate.id === role.id))
      expect(screen.queryByText(role.description)).not.toBeInTheDocument();
  expectBoundRequests();
}
async function expectReadback(expected: Role[]) {
  await waitFor(() => {
    expect(reads()).toHaveLength(2);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New Role' })).toBeEnabled();
  });
  expectSavedRows(expected);
  expect(writes()).toHaveLength(1);
}

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID(orgID);
  setApiToken(authToken);
  vi.stubEnv('MODE', 'production');
  document.cookie = `pulse_csrf=${csrfToken}; Path=/`;
  saved = structuredClone(originalRoles);
  completion = deferred<Response>();
  reload = undefined;
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    const path = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET' && path === '/api/admin/roles') return reload?.promise ?? json(saved);
    if (['PUT', 'DELETE'].includes(method) && saved.some((role) => path === rolePath(role.id)))
      return completion.promise;
    throw new Error(`Unexpected synthetic role-selection request: ${method} ${path}`);
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

describe('saved role identities through their rendered rows and real transport', () => {
  it('distinguishes roles differing only by saved ID before any edit or deletion', async () => {
    saved = [
      structuredClone(originalRoles[0]),
      { ...structuredClone(originalRoles[0]), id: originalRoles[1].id },
    ];
    render(() => <RolesPanel currentUser="role-maintainer" canManage />);
    await screen.findByText(saved[1].id);
    expect(screen.getAllByText(saved[0].name)).toHaveLength(2);
    expect(screen.getAllByText(saved[0].description)).toHaveLength(2);
    for (const role of saved) {
      expect(rowFor(role).getByText(role.id)).toBeInTheDocument();
      expect(
        screen.getAllByRole('button', { name: `Edit role ${role.name} (Role ID: ${role.id})` }),
      ).toHaveLength(1);
      expect(
        screen.getAllByRole('button', { name: `Delete role ${role.name} (Role ID: ${role.id})` }),
      ).toHaveLength(1);
    }
    const role = saved[1];
    const editor = await openEditor(role);
    expect(within(editor).getByText(role.id)).toBeInTheDocument();
    fireEvent.click(within(editor).getByRole('button', { name: 'Cancel' }));
    const confirmation = await openDeletion(role);
    expect(within(confirmation).getByText(role.id)).toBeInTheDocument();
    fireEvent.click(within(confirmation).getByRole('button', { name: 'Cancel' }));
    expect(writes()).toHaveLength(0);
    expectBoundRequests();
  });

  it('edits only the selected namesake and reopens its complete saved policy after reordered reads', async () => {
    await openPanel();
    const dialog = await openEditor();
    fireEvent.input(within(dialog).getByLabelText('Role Name'), {
      target: { value: 'Renamed selected auditor' },
    });
    fireEvent.input(within(dialog).getByLabelText('Description'), {
      target: { value: 'Updated second policy' },
    });
    const expected = {
      ...selected,
      name: 'Renamed selected auditor',
      description: 'Updated second policy',
    };
    await submit(dialog, 'edit');
    expect(JSON.parse(String(writes()[0][1]?.body))).toEqual(expected);
    applyDispatchedWrite();
    completion.resolve(json(expected));
    await expectReadback(originalRoles.map((role) => (role.id === selected.id ? expected : role)));
    await openEditor(expected);
    expect(writes()).toHaveLength(1);
    expect(reads()).toHaveLength(2);
  });

  it('discards a cancelled namesake draft before editing a different saved row', async () => {
    await openPanel();
    const dialog = await openEditor();
    fireEvent.input(within(dialog).getByLabelText('Role Name'), {
      target: { value: 'Discarded draft name' },
    });
    fireEvent.click(within(dialog).getAllByRole('button', { name: 'Remove permission' })[1]);
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(saved).toEqual(originalRoles);
    expect(writes()).toHaveLength(0);
    expect(reads()).toHaveLength(1);
    const third = originalRoles[2];
    const next = await openEditor(third);
    fireEvent.input(within(next).getByLabelText('Description'), {
      target: { value: 'Updated third policy' },
    });
    const expected = { ...third, description: 'Updated third policy' };
    await submit(next, 'edit', third);
    expect(JSON.parse(String(writes()[0][1]?.body))).toEqual(expected);
    applyDispatchedWrite();
    completion.resolve(json(expected));
    await expectReadback(originalRoles.map((role) => (role.id === third.id ? expected : role)));
  });

  it('deletes only the confirmed namesake, preserving sibling policies and the read-only role', async () => {
    await openPanel();
    const dialog = await openDeletion();
    expect(writes()).toHaveLength(0);
    await submit(dialog, 'delete');
    expect(writes()[0][1]?.body).toBeUndefined();
    applyDispatchedWrite();
    completion.resolve(new Response(null, { status: 204 }));
    await expectReadback(originalRoles.filter((role) => role.id !== selected.id));
    // After the inventory reorders, the same grid must still supply the newly
    // selected identity rather than a captured index or the deleted role.
    await openEditor(originalRoles[2]);
    expect(writes()).toHaveLength(1);
  });

  it('cancels one deletion before confirming a different namesake without reusing its identity', async () => {
    await openPanel();
    const dialog = await openDeletion();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(saved).toEqual(originalRoles);
    expect(writes()).toHaveLength(0);
    expect(reads()).toHaveLength(1);
    const third = originalRoles[2];
    await submit(await openDeletion(third), 'delete', third);
    applyDispatchedWrite();
    completion.resolve(new Response(null, { status: 204 }));
    await expectReadback(originalRoles.filter((role) => role.id !== third.id));
  });

  it.each([
    { operation: 'edit', applied: true },
    { operation: 'edit', applied: false },
    { operation: 'delete', applied: true },
    { operation: 'delete', applied: false },
  ] as const)(
    'rereads the saved namesakes after an uncertain $operation (applied=$applied), never replaying the write',
    async ({ operation, applied }) => {
      await openPanel();
      const dialog = operation === 'edit' ? await openEditor() : await openDeletion();
      const updated = { ...selected, description: 'Saved second policy after uncertain edit' };
      if (operation === 'edit')
        fireEvent.input(within(dialog).getByLabelText('Description'), {
          target: { value: updated.description },
        });
      await submit(dialog, operation);
      if (operation === 'edit') expect(JSON.parse(String(writes()[0][1]?.body))).toEqual(updated);
      else expect(writes()[0][1]?.body).toBeUndefined();
      if (applied) applyDispatchedWrite();
      const expected = !applied
        ? originalRoles
        : operation === 'edit'
          ? originalRoles.map((role) => (role.id === selected.id ? updated : role))
          : originalRoles.filter((role) => role.id !== selected.id);
      completion.resolve(json({ error: 'SYNTHETIC PRIVATE FAILURE' }, 503));
      const check = await screen.findByRole('button', { name: 'Check saved roles' });
      const notice = screen.getByRole('alert');
      expect(notice).toHaveTextContent(
        `The ${operation === 'edit' ? 'save' : 'deletion'} of role ${selected.name} (Role ID: ${selected.id}) could not be confirmed.`,
      );
      expect(notice).not.toHaveTextContent('PRIVATE');
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(screen.queryByText('No roles available.')).not.toBeInTheDocument();
      expect(screen.queryByRole('status')).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', {
          name: `Edit role ${selected.name} (Role ID: ${selected.id})`,
        }),
      ).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
      expect(reads()).toHaveLength(1);
      expect(writes()).toHaveLength(1);
      reload = deferred<Response>();
      fireEvent.click(check);
      await waitFor(() => expect(reads()).toHaveLength(2));
      expect(screen.queryByRole('button', { name: 'Check saved roles' })).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
      expect(writes()).toHaveLength(1);
      reload.resolve(json([...saved].reverse()));
      await expectReadback(expected);
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(screen.queryByRole('status')).not.toBeInTheDocument();
    },
  );
});
