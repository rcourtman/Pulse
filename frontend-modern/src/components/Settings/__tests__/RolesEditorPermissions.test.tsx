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
  fireEvent.click(await screen.findByRole('button', { name: 'Edit role' }));
  return screen.findByRole('dialog', { name: 'Edit role' });
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
    fireEvent.click(editor.getAllByRole('button', { name: 'Remove permission' })[0]);
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
