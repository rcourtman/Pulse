import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import type { Role } from '@/types/rbac';
import { RolesPanel } from '../RolesPanel';

// Exercise the rendered form, state, RBACAPI and authenticated serialization.
// Native keyboard default actions are covered separately in real browsers.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));

const originals = ['first', 'second'].map((identity, index) => ({
  id: `policy/${identity}?estate=north#saved`,
  name: 'Fleet auditor',
  description: `Saved ${identity} policy`,
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
const selected = originals[1];
const roleURL = `/api/admin/roles/${encodeURIComponent(selected.id)}`;
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
const fetchMock = vi.fn<typeof fetch>();
const writes = () => fetchMock.mock.calls.filter(([, init]) => (init?.method ?? 'GET') !== 'GET');
const reads = () =>
  fetchMock.mock.calls.filter(
    ([url, init]) => url === '/api/admin/roles' && (init?.method ?? 'GET') === 'GET',
  );
let saved: Role[];
let reply: ((init: RequestInit) => Promise<Response>) | undefined;
let release: (() => void) | undefined;

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID('estate/north');
  setApiToken('synthetic-role-submit-token');
  document.cookie = 'pulse_csrf=synthetic-role-submit-csrf; Path=/';
  saved = structuredClone(originals);
  reply = undefined;
  release = undefined;
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url, init) => {
    if (url === '/api/admin/roles' && !init?.method) return json(saved);
    if (
      (url === roleURL && init?.method === 'PUT') ||
      (url === '/api/admin/roles' && init?.method === 'POST')
    ) {
      if (reply) return reply(init);
      const body = JSON.parse(String(init.body)) as Role;
      saved =
        init.method === 'PUT'
          ? saved.map((role) => (role.id === body.id ? body : role))
          : [...saved, body];
      return json(body);
    }
    throw new Error(
      `Unexpected synthetic role-submission request: ${init?.method ?? 'GET'} ${url}`,
    );
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  cleanup();
  release?.();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  document.cookie = 'pulse_org_id=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
});

async function openEdit() {
  fireEvent.click(
    await screen.findByRole('button', {
      name: `Edit role ${selected.name} (Role ID: ${selected.id})`,
    }),
  );
  return screen.getByRole('dialog', {
    name: `Edit role: ${selected.name} (Role ID: ${selected.id})`,
  });
}
async function openCreate() {
  const button = await screen.findByRole('button', { name: 'New Role' });
  await waitFor(() => expect(button).toBeEnabled());
  fireEvent.click(button);
  return screen.getByRole('dialog', { name: 'New role' });
}
function submit(dialog: HTMLElement) {
  const input = within(dialog).getByLabelText('Role Name') as HTMLInputElement;
  expect(input.form).not.toBeNull();
  // The existing visible submit is also the browser's implicit Enter target.
  const save = within(dialog).getByRole('button', { name: /^(Update|Create) Role$/ });
  expect(save).toHaveAttribute('type', 'submit');
  expect((save as HTMLButtonElement).form).toBe(input.form);
  expect(fireEvent.submit(input.form!)).toBe(false); // Prevent navigation.
}
function expectBoundWrite() {
  expect(writes()).toHaveLength(1);
  expect(writes()[0][1]).toMatchObject({
    credentials: 'include',
    signal: expect.any(AbortSignal),
    headers: {
      'Content-Type': 'application/json',
      'X-Pulse-Org-ID': 'estate/north',
      'X-API-Token': 'synthetic-role-submit-token',
      'X-CSRF-Token': 'synthetic-role-submit-csrf',
    },
  });
}

describe('native role-editor submission through its existing guarded action', () => {
  it('submits only the selected namesake with its complete saved metadata and constraints', async () => {
    render(() => <RolesPanel currentUser="synthetic-maintainer" canManage />);
    const dialog = await openEdit();
    const editor = within(dialog);
    expect(editor.getByLabelText('Role ID')).toBeDisabled();
    fireEvent.input(editor.getByLabelText('Description'), {
      target: { value: 'Reviewed second policy' },
    });
    submit(dialog);
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expectBoundWrite();
    expect(writes()[0][0]).toBe(roleURL);
    const expected = { ...selected, description: 'Reviewed second policy' };
    expect(JSON.parse(String(writes()[0][1]?.body))).toEqual(expected);
    expect(saved).toEqual([originals[0], expected]);
    expect(reads()).toHaveLength(2);
  });

  it('creates a separate normalized draft without inheriting metadata from an earlier edit', async () => {
    render(() => <RolesPanel currentUser="synthetic-maintainer" canManage />);
    fireEvent.click(within(await openEdit()).getByRole('button', { name: 'Cancel' }));
    const dialog = await openCreate();
    const editor = within(dialog);
    fireEvent.input(editor.getByLabelText('Role ID'), { target: { value: '  New Auditor  ' } });
    fireEvent.input(editor.getByLabelText('Role Name'), { target: { value: 'New auditor' } });
    submit(dialog);
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expectBoundWrite();
    expect(writes()[0][0]).toBe('/api/admin/roles');
    expect(writes()[0][1]?.method).toBe('POST');
    expect(JSON.parse(String(writes()[0][1]?.body))).toEqual({
      id: 'new-auditor',
      name: 'New auditor',
      description: '',
      permissions: [{ action: 'read', resource: 'nodes' }],
    });
    expect(saved.slice(0, 2)).toEqual(originals);
  });

  it.each([
    { entered: '  Fleet   Reader ', existingId: 'fleet-reader', isBuiltIn: false },
    { entered: 'Viewer', existingId: 'viewer', isBuiltIn: true },
  ])('does not send a create for the saved ID $existingId', async (example) => {
    const original = { ...selected, id: example.existingId, isBuiltIn: example.isBuiltIn };
    saved = [original];
    render(() => <RolesPanel currentUser="synthetic-maintainer" canManage />);
    const dialog = await openCreate();
    const editor = within(dialog);
    const id = editor.getByLabelText('Role ID');
    fireEvent.input(id, { target: { value: example.entered } });
    fireEvent.input(editor.getByLabelText('Role Name'), { target: { value: 'New policy' } });
    fireEvent.input(editor.getByLabelText('Description'), {
      target: { value: 'Draft description' },
    });
    submit(dialog);
    const error = editor.getByRole('alert');
    expect(error).toHaveTextContent(`Role ID already exists: ${example.existingId}`);
    expect(id).toHaveValue(example.entered);
    expect(editor.getByLabelText('Description')).toHaveValue('Draft description');
    expect(writes()).toHaveLength(0);
    expect(reads()).toHaveLength(1);
    expect(saved).toEqual([original]);

    fireEvent.input(id, { target: { value: 'New Policy' } });
    expect(editor.queryByRole('alert')).toBeNull();
    submit(dialog);
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expectBoundWrite();
    expect(writes()[0][1]?.method).toBe('POST');
    expect(JSON.parse(String(writes()[0][1]?.body))).toMatchObject({
      id: 'new-policy',
      description: 'Draft description',
    });
    expect(saved[0]).toEqual(original);
  });

  it('does not case-fold a saved identity when testing for a collision', async () => {
    const original = { ...selected, id: 'Ops' };
    saved = [original];
    render(() => <RolesPanel currentUser="synthetic-maintainer" canManage />);
    const dialog = await openCreate();
    const editor = within(dialog);
    fireEvent.input(editor.getByLabelText('Role ID'), { target: { value: '  OPS ' } });
    fireEvent.input(editor.getByLabelText('Role Name'), { target: { value: 'Night ops' } });
    submit(dialog);
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expectBoundWrite();
    expect(writes()[0][1]?.method).toBe('POST');
    expect(saved.map((role) => role.id)).toEqual(['Ops', 'ops']);
  });

  it('shares blank-name and missing-ID validation rather than dispatching an incomplete form', async () => {
    render(() => <RolesPanel currentUser="synthetic-maintainer" canManage />);
    const dialog = await openCreate();
    const editor = within(dialog);
    fireEvent.input(editor.getByLabelText('Role Name'), { target: { value: '   ' } });
    submit(dialog);
    expect(writes()).toHaveLength(0);
    expect(editor.queryByRole('alert')).toBeNull();
    fireEvent.input(editor.getByLabelText('Role Name'), { target: { value: 'Missing ID' } });
    submit(dialog);
    expect(editor.getByRole('alert')).toHaveTextContent('ID and Name are required');
    expect(writes()).toHaveLength(0);
    expect(saved).toEqual(originals);
  });

  it('keeps permission, close and cancel controls out of the submit path', async () => {
    render(() => <RolesPanel currentUser="synthetic-maintainer" canManage />);
    const dialog = await openEdit();
    const editor = within(dialog);
    const buttons = editor.getAllByRole('button');
    const save = editor.getByRole('button', { name: 'Update Role' });
    for (const button of buttons.filter((button) => button !== save))
      expect(button).toHaveAttribute('type', 'button');
    fireEvent.click(editor.getByRole('button', { name: 'Add Permission' }));
    fireEvent.click(editor.getByRole('button', { name: 'Remove permission 4: read:nodes' }));
    fireEvent.click(editor.getByRole('button', { name: 'Cancel' }));
    expect(writes()).toHaveLength(0);
    expect(saved).toEqual(originals);
    fireEvent.click(within(await openEdit()).getByRole('button', { name: 'Close role editor' }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(writes()).toHaveLength(0);
  });

  it('serializes repeated submit events while its write is pending', async () => {
    reply = () =>
      new Promise<Response>((resolve) => {
        release = () => resolve(json(selected));
      });
    render(() => <RolesPanel currentUser="synthetic-maintainer" canManage />);
    const dialog = await openEdit();
    const form = (within(dialog).getByLabelText('Role Name') as HTMLInputElement).form;
    submit(dialog);
    await waitFor(() => expect(writes()).toHaveLength(1));
    fireEvent.submit(form!);
    fireEvent.submit(form!);
    expect(within(dialog).getByRole('button', { name: 'Saving...' })).toBeDisabled();
    expect(writes()).toHaveLength(1);
    release!();
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(reads()).toHaveLength(2);
    expectBoundWrite();
  });

  it('keeps a definitely rejected draft editable without a read or automatic second write', async () => {
    reply = async () => json({ error: 'Synthetic draft rejected' }, 422);
    render(() => <RolesPanel currentUser="synthetic-maintainer" canManage />);
    const dialog = await openEdit();
    fireEvent.input(within(dialog).getByLabelText('Role Name'), {
      target: { value: 'Rejected draft' },
    });
    submit(dialog);
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Failed to save role');
    expect(within(dialog).getByLabelText('Role Name')).toHaveValue('Rejected draft');
    expect(within(dialog).getByRole('button', { name: 'Update Role' })).toBeEnabled();
    expect(reads()).toHaveLength(1);
    expectBoundWrite();
    expect(saved).toEqual(originals);
  });

  it('requires a read-only inventory check after an uncertain response', async () => {
    reply = async (init) => {
      const body = JSON.parse(String(init.body)) as Role;
      saved = saved.map((role) => (role.id === body.id ? body : role));
      return json({ error: 'Synthetic response unavailable' }, 503);
    };
    render(() => <RolesPanel currentUser="synthetic-maintainer" canManage />);
    const dialog = await openEdit();
    fireEvent.input(within(dialog).getByLabelText('Description'), {
      target: { value: 'Applied but not confirmed' },
    });
    submit(dialog);
    const check = await screen.findByRole('button', { name: 'Check saved roles' });
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
    expectBoundWrite();
    fireEvent.click(check);
    await screen.findByText('Applied but not confirmed');
    expect(reads()).toHaveLength(2);
    expectBoundWrite();
  });

  it.each(['user', 'organization', 'permission', 'unmount'])(
    'does not reuse a detached form after %s replaces its reader',
    async (change) => {
      const [user, setUser] = createSignal('synthetic-maintainer');
      const [allowed, setAllowed] = createSignal(true);
      const view = render(() => <RolesPanel currentUser={user()} canManage={allowed()} />);
      const dialog = await openEdit();
      const form = (within(dialog).getByLabelText('Role Name') as HTMLInputElement).form;
      expect(form).not.toBeNull();
      if (change === 'user') setUser('replacement-reader');
      else if (change === 'permission') setAllowed(false);
      else if (change === 'organization') {
        setOrgID('estate/south');
        eventBus.emit('org_switched', 'estate/south');
      } else view.unmount();
      await waitFor(() => expect(dialog).not.toBeInTheDocument());
      fireEvent.submit(form!);
      expect(writes()).toHaveLength(0);
      expect(saved).toEqual(originals);
    },
  );
});
