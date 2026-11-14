import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { clearAuth, setOrgID } from '@/utils/apiClient';
import { RolesPanel } from '../RolesPanel';

// Keep the rendered editor, indexed draft operations, RBACAPI and serialization
// connected. Hook-only permission edits cannot catch a miswired form callback.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));

const original = {
  id: 'scoped/auditor:production',
  name: 'Constrained auditor',
  description: 'Synthetic saved role',
  parentId: 'viewer',
  priority: 17,
  createdAt: '2026-10-01T09:00:00Z',
  updatedAt: '2026-11-01T09:00:00Z',
  permissions: [
    { action: 'read', resource: 'nodes' },
    {
      action: 'delete',
      resource: 'users',
      effect: 'deny',
      conditions: { tag: 'production', owner: '${user}' },
    },
    { action: 'future:inspect', resource: 'future/resource', conditions: { tenant: 'north' } },
  ],
};
const roleURL = `/api/admin/roles/${encodeURIComponent(original.id)}`;
const fetchMock = vi.fn<typeof fetch>();
const writes = () => fetchMock.mock.calls.filter(([, init]) => init?.method === 'PUT');
const reads = () =>
  fetchMock.mock.calls.filter(([url, init]) => url === '/api/admin/roles' && !init?.method);
const json = (body: unknown) =>
  new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } });
let saved: unknown;

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID('estate/north');
  document.cookie = 'pulse_csrf=synthetic-editor-csrf; Path=/';
  saved = structuredClone(original);
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url, init) => {
    if (url === '/api/admin/roles' && !init?.method) return json([saved]);
    if (url === roleURL && init?.method === 'PUT') {
      // Only the actual serialized body determines the saved readback. Do not
      // fabricate the intended permissions independently of the user's edits.
      saved = JSON.parse(String(init.body));
      return json(saved);
    }
    throw new Error(`Unexpected synthetic role-editor request: ${init?.method ?? 'GET'} ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  cleanup();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  document.cookie = 'pulse_org_id=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
});

async function openEditor() {
  fireEvent.click(
    await screen.findByRole('button', {
      name: `Edit role ${original.name} (Role ID: ${original.id})`,
    }),
  );
  return screen.findByRole('dialog', {
    name: `Edit role: ${original.name} (Role ID: ${original.id})`,
  });
}

function expectPermissions(dialog: HTMLElement, permissions: typeof original.permissions) {
  const editor = within(dialog);
  expect(editor.getAllByRole('combobox')).toHaveLength(permissions.length * 2);
  permissions.forEach((permission, index) => {
    expect(editor.getByLabelText(`Permission ${index + 1} action`)).toHaveValue(permission.action);
    expect(editor.getByLabelText(`Permission ${index + 1} resource`)).toHaveValue(
      permission.resource,
    );
  });
}

async function saveAndReopen(dialog: HTMLElement, expected: unknown) {
  expect(writes()).toHaveLength(0);
  expect(saved).toEqual(original);
  fireEvent.click(within(dialog).getByRole('button', { name: 'Update Role' }));
  await waitFor(() => {
    expect(writes()).toHaveLength(1);
    expect(reads()).toHaveLength(2);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
  const [url, init] = writes()[0];
  expect(url).toBe(roleURL);
  expect(JSON.parse(String(init?.body))).toEqual(expected);
  expect(saved).toEqual(expected);
  return openEditor();
}

describe('saved permission transformations through the role editor', () => {
  it('keeps the same focused controls through repeated action and resource edits', async () => {
    render(() => <RolesPanel currentUser="role-maintainer" canManage />);
    const dialog = await openEditor();
    const editor = within(dialog);
    const action = editor.getByLabelText('Permission 2 action');
    const resource = editor.getByLabelText('Permission 2 resource');

    action.focus();
    for (const value of ['write', 'admin', 'read']) {
      fireEvent.change(action, { target: { value } });
      expect(editor.getByLabelText('Permission 2 action')).toBe(action);
      expect(action).toHaveFocus();
      expect(action).toHaveValue(value);
      expect(
        editor.getByRole('button', { name: `Remove permission 2: ${value}:users` }),
      ).toBeEnabled();
    }
    resource.focus();
    for (const value of ['audit_logs', 'settings', 'nodes']) {
      fireEvent.change(resource, { target: { value } });
      expect(editor.getByLabelText('Permission 2 resource')).toBe(resource);
      expect(resource).toHaveFocus();
      expect(resource).toHaveValue(value);
      expect(
        editor.getByRole('button', { name: `Remove permission 2: read:${value}` }),
      ).toBeEnabled();
    }
    // Stable DOM must not create a write or mutate another saved rule.
    expect(writes()).toHaveLength(0);
    expect(saved).toEqual(original);
    expectPermissions(dialog, [
      original.permissions[0],
      { ...original.permissions[1], action: 'read', resource: 'nodes' },
      original.permissions[2],
    ]);
    const reopened = await saveAndReopen(dialog, {
      ...original,
      permissions: [
        original.permissions[0],
        { ...original.permissions[1], action: 'read', resource: 'nodes' },
        original.permissions[2],
      ],
    });
    expectPermissions(reopened, [
      original.permissions[0],
      { ...original.permissions[1], action: 'read', resource: 'nodes' },
      original.permissions[2],
    ]);
  });

  it('keeps unknown options editable without replacing the select or a sibling rule', async () => {
    render(() => <RolesPanel currentUser="role-maintainer" canManage />);
    const editor = within(await openEditor());
    const action = editor.getByLabelText('Permission 3 action');
    const resource = editor.getByLabelText('Permission 3 resource');
    expect(
      editor.getByRole('button', { name: 'Remove permission 3: future:inspect:future/resource' }),
    ).toBeEnabled();
    action.focus();
    fireEvent.change(action, { target: { value: 'read' } });
    expect(action).toHaveFocus();
    expect(editor.getByLabelText('Permission 3 action')).toBe(action);
    expect(editor.queryByRole('option', { name: 'future:inspect' })).not.toBeInTheDocument();
    resource.focus();
    fireEvent.change(resource, { target: { value: 'nodes' } });
    expect(resource).toHaveFocus();
    expect(editor.getByLabelText('Permission 3 resource')).toBe(resource);
    expect(editor.queryByRole('option', { name: 'future/resource' })).not.toBeInTheDocument();
    expect(editor.getByRole('button', { name: 'Remove permission 3: read:nodes' })).toBeEnabled();
    expect(editor.getByLabelText('Permission 2 action')).toHaveValue('delete');
    expect(editor.getByLabelText('Permission 2 resource')).toHaveValue('users');
    expect(writes()).toHaveLength(0);
    expect(saved).toEqual(original);
  });

  it('distinguishes duplicate removal controls and updates their names after reindexing', async () => {
    saved = {
      ...structuredClone(original),
      permissions: [original.permissions[0], original.permissions[0]],
    };
    render(() => <RolesPanel currentUser="role-maintainer" canManage />);
    const dialog = await openEditor();
    const editor = within(dialog);
    expect(editor.getByRole('button', { name: 'Remove permission 1: read:nodes' })).toBeEnabled();
    fireEvent.click(editor.getByRole('button', { name: 'Remove permission 2: read:nodes' }));
    expect(editor.getByRole('button', { name: 'Remove permission 1: read:nodes' })).toBeDisabled();
    expect(
      editor.queryByRole('button', { name: 'Remove permission 2: read:nodes' }),
    ).not.toBeInTheDocument();
    fireEvent.click(editor.getByRole('button', { name: 'Add Permission' }));
    const action = editor.getByLabelText('Permission 2 action');
    action.focus();
    fireEvent.change(action, { target: { value: 'delete' } });
    expect(action).toHaveFocus();
    expect(editor.getByRole('button', { name: 'Remove permission 2: delete:nodes' })).toBeEnabled();
    fireEvent.click(editor.getByRole('button', { name: 'Remove permission 1: read:nodes' }));
    expect(editor.getByLabelText('Permission 1 action')).toHaveValue('delete');
    expect(
      editor.getByRole('button', { name: 'Remove permission 1: delete:nodes' }),
    ).toBeDisabled();
    expect(writes()).toHaveLength(0);
  });

  it('edits only the middle rule through both fields and round-trips every saved constraint', async () => {
    render(() => <RolesPanel currentUser="role-maintainer" canManage />);
    const dialog = await openEditor();
    expectPermissions(dialog, original.permissions);
    const editor = within(dialog);
    fireEvent.change(editor.getByLabelText('Permission 2 action'), { target: { value: 'write' } });
    fireEvent.change(editor.getByLabelText('Permission 2 resource'), {
      target: { value: 'audit_logs' },
    });
    const permissions = original.permissions.map((permission, index) =>
      index === 1 ? { ...permission, action: 'write', resource: 'audit_logs' } : permission,
    );
    expectPermissions(dialog, permissions);
    const reopened = await saveAndReopen(dialog, { ...original, permissions });
    expectPermissions(reopened, permissions);
    expect(writes()).toHaveLength(1);
    expect(reads()).toHaveLength(2);
  });

  it('reindexes surviving rules after removal and appends an independent least-privilege rule', async () => {
    render(() => <RolesPanel currentUser="role-maintainer" canManage />);
    const dialog = await openEditor();
    const editor = within(dialog);
    expectPermissions(dialog, original.permissions);
    fireEvent.click(editor.getAllByRole('button', { name: /^Remove permission / })[0]);
    expectPermissions(dialog, original.permissions.slice(1));
    // The original middle rule is now first. Its constraints and the unlisted
    // sibling must survive a change through the newly indexed controls.
    fireEvent.change(editor.getByLabelText('Permission 1 action'), { target: { value: 'admin' } });
    fireEvent.change(editor.getByLabelText('Permission 1 resource'), {
      target: { value: 'settings' },
    });
    fireEvent.click(editor.getByRole('button', { name: 'Add Permission' }));
    const permissions = [
      { ...original.permissions[1], action: 'admin', resource: 'settings' },
      original.permissions[2],
      { action: 'read', resource: 'nodes' },
    ];
    expectPermissions(dialog, permissions);
    fireEvent.change(editor.getByLabelText('Permission 3 resource'), {
      target: { value: 'license' },
    });
    const expected = [...permissions.slice(0, 2), { action: 'read', resource: 'license' }];
    expectPermissions(dialog, expected);
    const reopened = await saveAndReopen(dialog, { ...original, permissions: expected });
    expectPermissions(reopened, expected);
    expect(writes()).toHaveLength(1);
    expect(reads()).toHaveLength(2);
  });
});
