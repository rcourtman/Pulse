import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { RolesPanel } from '../RolesPanel';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';
import { PermissionPolicyDetails } from '../PermissionPolicyDetails';
import type { Permission, Role } from '@/types/rbac';
import { clearAuth, setOrgID } from '@/utils/apiClient';

// Preserve the real consumers, draft owner, serialization and HTTP client.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));
const permissions: Permission[] = [
  { action: 'read', resource: 'nodes' },
  {
    action: 'delete',
    resource: 'nodes',
    effect: 'deny',
    conditions: { tag: 'production', owner: '${user}' },
  },
  {
    action: 'future:inspect',
    resource: 'future/resource',
    effect: 'future-effect',
    conditions: { Ops: '', ops: '<img src=x onerror=alert(1)>' },
  },
];
const original: Role & { parentId: string; priority: number } = {
  id: 'scoped/auditor:production',
  name: 'Constrained auditor',
  description: 'Saved policy',
  createdAt: '2026-10-01T09:00:00Z',
  parentId: 'viewer',
  priority: 17,
  permissions,
};
const json = (body: unknown) =>
  new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } });
const fetchMock = vi.fn<typeof fetch>();
let saved: typeof original;
let permissionReply: unknown;
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => !['GET', 'HEAD'].includes(init?.method ?? 'GET'));

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID('estate/north');
  document.cookie = 'pulse_csrf=synthetic-policy-csrf; Path=/';
  saved = structuredClone(original);
  permissionReply = permissions;
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url, init = {}) => {
    if (url === '/api/admin/roles' && !init.method) return json([saved]);
    if (url === `/api/admin/roles/${encodeURIComponent(original.id)}` && init.method === 'PUT') {
      saved = JSON.parse(String(init.body));
      return json(saved);
    }
    if (url === '/api/admin/users')
      return json([{ username: 'sso:lab/saved', roleIds: [original.id] }]);
    if (url === `/api/admin/users/${encodeURIComponent('sso:lab/saved')}/permissions`)
      return json(permissionReply);
    throw new Error(`Unexpected fixture request: ${init.method ?? 'GET'} ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  cleanup();
  clearAuth();
  setOrgID(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
});

async function editor() {
  fireEvent.click(await screen.findByRole('button', { name: /^Edit role Constrained auditor/ }));
  return within(await screen.findByRole('dialog'));
}

describe('saved rule policy review through consuming access panels', () => {
  it('shows saved deny and exact conditions before editing', async () => {
    render(() => <RolesPanel currentUser="policy-reviewer" canManage />);
    const lists = await screen.findAllByRole('list', {
      name: /^Permissions for Constrained auditor/,
    });
    for (const list of lists) {
      const rules = within(list).getAllByRole('listitem');
      expect(rules).toHaveLength(3);
      expect(rules[0]).toHaveTextContent('read:nodesAllow');
      expect(rules[1]).toHaveTextContent('delete:nodesDenyConditions (all):');
      expect(rules[1]).toHaveTextContent('tag = "production"');
      expect(rules[1]).toHaveTextContent('owner = "${user}"');
      expect(rules[2]).toHaveTextContent('Unknown effect: future-effect');
      expect(rules[2]).toHaveTextContent('Ops = ""');
      expect(rules[2]).toHaveTextContent('ops = "<img src=x onerror=alert(1)>"');
      expect(list.querySelector('img')).toBeNull();
    }
    expect(writes()).toHaveLength(0);
  });

  it('keeps effect and conditions visible while editing without altering their saved bytes', async () => {
    render(() => <RolesPanel currentUser="policy-reviewer" canManage />);
    const dialog = await editor();
    expect(dialog.getByText('Deny')).toBeVisible();
    expect(dialog.getByText('"${user}"')).toBeVisible();
    expect(
      dialog.getByRole('button', { name: 'Remove permission 2: delete:nodes' }),
    ).toHaveAccessibleDescription(/Deny.*owner = "\$\{user\}".*tag = "production"/);
    const action = dialog.getByLabelText('Permission 2 action');
    action.focus();
    fireEvent.change(action, { target: { value: 'write' } });
    expect(action).toHaveFocus();
    expect(dialog.getByLabelText('Permission 2 action')).toBe(action);
    expect(dialog.getByText('Deny')).toBeVisible();
    expect(dialog.getByText('"${user}"')).toBeVisible();
    expect(writes()).toHaveLength(0);
    expect(saved).toEqual(original);
    fireEvent.click(dialog.getByRole('button', { name: 'Update Role' }));
    await waitFor(() => expect(writes()).toHaveLength(1));
    expect(saved).toEqual({
      ...original,
      permissions: [permissions[0], { ...permissions[1], action: 'write' }, permissions[2]],
    });
    expect(saved.permissions[0]).not.toHaveProperty('effect');
    expect(writes()[0][1]).toMatchObject({
      method: 'PUT',
      credentials: 'include',
      headers: { 'X-Pulse-Org-ID': 'estate/north', 'X-CSRF-Token': 'synthetic-policy-csrf' },
    });
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    const reopened = await editor();
    expect(reopened.getByText('Deny')).toBeVisible();
    expect(reopened.getByLabelText('Permission 2 action')).toHaveValue('write');
    expect(reopened.getByText('"${user}"')).toBeVisible();
  });

  it('describes returned saved rules, not an unconditional grant or a client authorization verdict', async () => {
    render(() => <UserAssignmentsPanel currentUser="policy-reviewer" canManage />);
    fireEvent.click(await screen.findByRole('button', { name: /^Manage access for/ }));
    const region = await screen.findByRole('region', { name: 'Saved permissions' });
    await waitFor(() => expect(region).toHaveAttribute('aria-busy', 'false'));
    expect(region).toHaveTextContent('This summary can omit duplicate and inherited rules.');
    expect(region).toHaveTextContent('Review individual rules in Roles.');
    expect(region).toHaveTextContent('This is not an access check for a specific resource.');
    expect(writes()).toHaveLength(0);
  });

  it('qualifies each returned rule without deciding access or incorporating an unsaved draft', async () => {
    permissionReply = [
      { action: 'read', resource: 'nodes' },
      { action: 'delete', resource: 'nodes', effect: 'deny' },
      { action: 'delete', resource: 'nodes', effect: 'deny', conditions: { tag: 'critical' } },
      { action: 'write', resource: 'storage', effect: 'allow', conditions: { tag: '' } },
      { action: 'read', resource: 'tasks', effect: '', conditions: {} },
      { action: 'read', resource: 'guests', effect: 'future-effect', conditions: { owner: 'ops' } },
      { action: 'read', resource: 'events', effect: null },
      { action: 'write', resource: 'settings', effect: 'deny', conditions: { tag: 1 } },
      { action: 'read', resource: 'metrics', conditions: ['tag'] },
      { action: 'read', resource: 'backups', conditions: null },
    ];
    render(() => <UserAssignmentsPanel currentUser="policy-reviewer" canManage />);
    fireEvent.click(await screen.findByRole('button', { name: /^Manage access for/ }));
    const dialog = within(await screen.findByRole('dialog'));
    const returned = await dialog.findByRole('list', { name: 'Returned rules' });
    const rules = within(returned).getAllByRole('listitem');
    const expected = [
      'read:nodes Allow',
      'delete:nodes Deny',
      'delete:nodes Deny · Conditional',
      'write:storage Allow · Conditional',
      'read:tasks Allow',
      'read:guests Effect unknown · Conditional',
      'read:events Effect unknown',
      'write:settings Deny · Conditions unknown',
      'read:metrics Allow · Conditions unknown',
      'read:backups Allow · Conditions unknown',
    ];
    expect(rules).toHaveLength(expected.length);
    rules.forEach((rule, index) => expect(rule).toHaveTextContent(expected[index]));
    expect(returned).not.toHaveTextContent('critical');
    const role = dialog.getByRole('checkbox', { name: /^Constrained auditor/ });
    expect(role).toBeChecked();
    fireEvent.click(role);
    expect(role).not.toBeChecked();
    expect(dialog.getByRole('button', { name: 'Save Changes' })).toBeEnabled();
    expect(within(returned).getAllByRole('listitem')).toEqual(rules);
    const path = `/api/admin/users/${encodeURIComponent('sso:lab/saved')}/permissions`;
    const reads = fetchMock.mock.calls.filter(([url]) => url === path);
    expect(reads).toHaveLength(1);
    expect(reads[0][1]).toMatchObject({
      credentials: 'include',
      headers: { 'X-Pulse-Org-ID': 'estate/north' },
    });
    expect(writes()).toHaveLength(0);
  });

  it.each([
    { effect: undefined, expected: 'Allow' },
    { effect: '', expected: 'Allow' },
    { effect: 'allow', expected: 'Allow' },
    { effect: 'deny', expected: 'Deny' },
    { effect: 'future', expected: 'Unknown effect: future' },
  ])('labels a saved $effect effect without changing it', ({ effect, expected }) => {
    const value = { action: 'read', resource: 'nodes', effect };
    render(() => <PermissionPolicyDetails permission={value} />);
    expect(screen.getByText(expected)).toBeVisible();
    expect(value.effect).toBe(effect);
  });

  it('updates indexed condition metadata without retaining a removed rule or interpreting markup', () => {
    const [value, setValue] = createSignal(permissions[1]);
    render(() => <PermissionPolicyDetails permission={value()} />);
    expect(screen.getByText('Deny')).toBeVisible();
    setValue(permissions[2]);
    expect(screen.queryByText('Deny')).not.toBeInTheDocument();
    expect(screen.queryByText('"production"')).not.toBeInTheDocument();
    expect(screen.getByText('"<img src=x onerror=alert(1)>"')).toBeVisible();
    expect(document.querySelector('img')).toBeNull();
  });

  it.each([42, 'tag=production', ['tag'], { tag: 42 }].map((conditions) => ({ conditions })))(
    'does not describe malformed conditions $conditions as an unconditional rule',
    ({ conditions }) => {
      const value = {
        action: 'read',
        resource: 'nodes',
        effect: 42,
        conditions,
      } as unknown as Permission;
      render(() => <PermissionPolicyDetails permission={value} />);
      expect(screen.getByText('Effect unavailable')).toBeVisible();
      expect(screen.getByText('Conditions unavailable')).toBeVisible();
      expect(screen.queryByText('Allow')).not.toBeInTheDocument();
    },
  );
});
