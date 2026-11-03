import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { RolesPanel } from '../RolesPanel';
import type { Role } from '@/types/rbac';

const api = vi.hoisted(() => ({
  getRoles: vi.fn(),
  saveRole: vi.fn(),
  deleteRole: vi.fn(),
}));

vi.mock('@/api/rbac', () => ({ RBACAPI: api }));
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));

const custom: Role = {
  id: 'night-ops',
  name: 'Night operators',
  description: 'Read nodes; manage alerts.',
  createdAt: '2026-10-01T00:00:00Z',
  permissions: [
    { action: 'read', resource: 'nodes' },
    { action: 'write', resource: 'alerts' },
    { action: 'inspect', resource: 'storage/retained-custom-scope' },
  ],
};
const builtIn: Role = {
  id: 'admin',
  name: 'Administrator',
  description: 'Manage the Pulse instance.',
  isBuiltIn: true,
  permissions: [{ action: '*', resource: '*' }],
};
const empty: Role = { ...custom, id: 'empty', name: 'Unassigned', permissions: [] };

async function getRoleRow(name: string) {
  const label = await screen.findByText(name, { exact: true });
  return within(label.closest('tr')!);
}

describe('Roles inventory permission visibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getRoles.mockResolvedValue([builtIn, custom, empty]);
  });
  afterEach(cleanup);

  it('keeps every saved permission in the role cell used by the narrow layout', async () => {
    render(() => <RolesPanel currentUser="fixture-admin" />);
    const row = await getRoleRow(custom.name);
    const roleCell = within(row.getAllByRole('cell')[0]);
    const list = within(roleCell.getByRole('list', { name: 'Permissions' }));

    expect(list.getAllByRole('listitem').map((item) => item.textContent)).toEqual([
      'read:nodes',
      'write:alerts',
      'inspect:storage/retained-custom-scope',
    ]);
    expect(row.getByRole('button', { name: 'Edit role' })).toBeEnabled();
    expect(row.getByRole('button', { name: 'Delete role' })).toBeEnabled();
    expect(api.saveRole).not.toHaveBeenCalled();
    expect(api.deleteRole).not.toHaveBeenCalled();
  });

  it('shows built-in wildcard permissions without adding built-in mutation controls', async () => {
    render(() => <RolesPanel currentUser="fixture-admin" />);
    const row = await getRoleRow(builtIn.name);
    const roleCell = within(row.getAllByRole('cell')[0]);

    expect(roleCell.getByText('*:*', { exact: true })).toBeInTheDocument();
    expect(row.getByText('Read-only', { exact: true })).toBeInTheDocument();
    expect(row.queryByRole('button')).not.toBeInTheDocument();
  });

  it('distinguishes an empty saved permission list from an unread inventory', async () => {
    render(() => <RolesPanel currentUser="fixture-admin" />);
    const row = await getRoleRow(empty.name);
    const roleCell = within(row.getAllByRole('cell')[0]);

    expect(roleCell.getByText('No permissions', { exact: true })).toBeInTheDocument();
    expect(screen.queryByText('Roles are unavailable for this session.')).not.toBeInTheDocument();
  });

  it('withdraws the added permission list with the existing reader boundary', async () => {
    const [allowed, setAllowed] = createSignal(true);
    render(() => <RolesPanel currentUser="fixture-admin" canManage={allowed()} />);
    const row = await getRoleRow(custom.name);
    const roleCell = within(row.getAllByRole('cell')[0]);
    expect(roleCell.getByText('write:alerts', { exact: true })).toBeInTheDocument();

    setAllowed(false);
    expect(screen.getByText('Roles are unavailable for this session.')).toBeInTheDocument();
    expect(screen.queryByText('write:alerts', { exact: true })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
  });

  it('opens and cancels the existing editor without altering saved custom scopes', async () => {
    render(() => <RolesPanel currentUser="fixture-admin" />);
    const row = await getRoleRow(custom.name);
    fireEvent.click(row.getByRole('button', { name: 'Edit role' }));
    const dialog = within(screen.getByRole('dialog', { name: 'Edit role' }));

    expect(dialog.getByRole('combobox', { name: 'Permission 3 action' })).toHaveValue('inspect');
    expect(dialog.getByRole('combobox', { name: 'Permission 3 resource' })).toHaveValue(
      'storage/retained-custom-scope',
    );
    fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(api.saveRole).not.toHaveBeenCalled();
    expect(api.deleteRole).not.toHaveBeenCalled();
  });
});
