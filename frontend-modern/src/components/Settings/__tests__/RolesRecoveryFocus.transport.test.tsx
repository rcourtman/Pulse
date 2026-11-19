import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { RolesPanel } from '../RolesPanel';

// Keep the panel, state, RBAC API and authenticated HTTP client connected.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));
const roles = ['first', 'second'].map((identity, index) => ({
  id: `policy/${identity}?estate=north#saved`,
  name: 'Fleet auditor',
  description: `Saved ${identity} policy`,
  parentId: 'viewer',
  priority: index + 10,
  createdAt: '2026-10-01T09:00:00Z',
  permissions: [
    { action: 'read', resource: 'nodes' },
    { action: 'delete', resource: 'users', effect: 'deny', conditions: { owner: '${user}' } },
    { action: 'future:inspect', resource: `future/${identity}` },
  ],
}));
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const deferred = () => {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((yes) => (resolve = yes));
  return { promise, resolve };
};
const fetchMock = vi.fn<typeof fetch>();
const reads = () =>
  fetchMock.mock.calls.filter(
    ([url, init]) => url === '/api/admin/roles' && (init?.method ?? 'GET') === 'GET',
  );
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => ['PUT', 'POST', 'DELETE'].includes(init?.method ?? ''));
let readReply: () => Promise<Response>;

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID('estate/north');
  setApiToken('synthetic-role-recovery-token');
  document.cookie = 'pulse_csrf=synthetic-role-recovery-csrf; Path=/';
  readReply = async () => json({ error: 'Synthetic private upstream detail' }, 503);
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url, init) => {
    if (url === '/api/admin/roles' && (init?.method ?? 'GET') === 'GET') return readReply();
    if (
      url === `/api/admin/roles/${encodeURIComponent(roles[1].id)}` &&
      ['PUT', 'DELETE'].includes(init?.method ?? '')
    )
      return json({ error: 'Synthetic private upstream detail' }, 502);
    throw new Error(`Unexpected synthetic role-recovery request: ${init?.method ?? 'GET'} ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  cleanup();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  document.cookie = 'pulse_org_id=; Max-Age=0; Path=/';
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
function mount() {
  const [user, setUser] = createSignal('reader-a');
  const [allowed, setAllowed] = createSignal(true);
  const view = render(() => (
    <>
      <button>Outside task</button>
      <RolesPanel currentUser={user()} canManage={allowed()} />
    </>
  ));
  return { view, setUser, setAllowed };
}
async function retry() {
  const button = await screen.findByRole('button', { name: 'Retry' });
  const pending = deferred();
  readReply = () => pending.promise;
  button.focus();
  fireEvent.click(button);
  const inventory = screen.getByRole('region', { name: 'Roles inventory' });
  expect(inventory).toHaveFocus();
  expect(inventory).toHaveAttribute('tabindex', '-1');
  expect(inventory).toHaveAttribute('aria-busy', 'true');
  await waitFor(() => expect(reads()).toHaveLength(2));
  expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull();
  expect(writes()).toHaveLength(0);
  expect(reads()[1][1]).toMatchObject({
    headers: { 'X-Pulse-Org-ID': 'estate/north', 'X-API-Token': 'synthetic-role-recovery-token' },
    signal: expect.any(AbortSignal),
  });
  return { pending, inventory, button };
}
async function checkAfterUnknownWrite(operation: 'save' | 'delete') {
  readReply = async () => json(roles);
  mount();
  fireEvent.click(
    await screen.findByRole('button', {
      name: `${operation === 'save' ? 'Edit' : 'Delete'} role Fleet auditor (Role ID: ${roles[1].id})`,
    }),
  );
  const dialog = await screen.findByRole('dialog');
  const action = within(dialog).getByRole('button', {
    name: operation === 'save' ? 'Update Role' : 'Delete Role',
  });
  action.focus();
  fireEvent.click(action);
  const check = await screen.findByRole('button', { name: 'Check saved roles' });
  await waitFor(() => expect(screen.getByRole('alert')).toHaveFocus());
  const pending = deferred();
  readReply = () => pending.promise;
  check.focus();
  fireEvent.click(check);
  const inventory = screen.getByRole('region', { name: 'Roles inventory' });
  expect(inventory).toHaveFocus();
  expect(inventory).toHaveAttribute('aria-busy', 'true');
  await waitFor(() => expect(reads()).toHaveLength(2));
  expect(writes()).toHaveLength(1);
  expect(writes()[0]).toMatchObject([
    `/api/admin/roles/${encodeURIComponent(roles[1].id)}`,
    {
      method: operation === 'save' ? 'PUT' : 'DELETE',
      headers: { 'X-Pulse-Org-ID': 'estate/north', 'X-CSRF-Token': 'synthetic-role-recovery-csrf' },
    },
  ]);
  if (operation === 'save') expect(JSON.parse(String(writes()[0][1]?.body))).toEqual(roles[1]);
  return { pending, inventory };
}

async function restartPending(replacement: ReturnType<typeof deferred>, count: number) {
  const oldSignal = reads().at(-1)?.[1]?.signal;
  readReply = () => replacement.promise;
  const restart = screen.getByRole('button', { name: 'Restart role check' });
  restart.focus();
  fireEvent.click(restart);
  const inventory = screen.getByRole('region', { name: 'Roles inventory' });
  expect(inventory).toHaveFocus();
  expect(inventory).toHaveAttribute('aria-busy', 'true');
  await waitFor(() => expect(reads()).toHaveLength(count));
  expect(oldSignal).toBeInstanceOf(AbortSignal);
  expect(oldSignal?.aborted).toBe(true);
  expect(reads().at(-1)?.[1]).toMatchObject({
    headers: { 'X-Pulse-Org-ID': 'estate/north' },
    signal: expect.objectContaining({ aborted: false }),
  });
  return inventory;
}

describe('keyboard-safe saved Roles recovery through the consuming transport', () => {
  it('does not autofocus the inventory during automatic initial loading', async () => {
    const pending = deferred();
    readReply = () => pending.promise;
    mount();
    const outside = screen.getByRole('button', { name: 'Outside task' });
    outside.focus();
    pending.resolve(json(roles));
    await screen.findByText(roles[1].id, { exact: true });
    expect(outside).toHaveFocus();
    expect(reads()).toHaveLength(1);
    expect(writes()).toHaveLength(0);
  });

  it.each([200, 503])(
    'replaces a stalled inventory and ignores an obsolete %s response',
    async (status) => {
      const pending = deferred();
      readReply = () => pending.promise;
      mount();
      await waitFor(() => expect(reads()).toHaveLength(1));
      expect(screen.getByText('Loading roles…')).toBeVisible();
      const replacement = deferred();
      const inventory = await restartPending(replacement, 2);

      pending.resolve(json(status === 200 ? [{ ...roles[0], name: 'Obsolete role' }] : {}, status));
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(inventory).toHaveAttribute('aria-busy', 'true');
      expect(screen.queryByText('Obsolete role')).not.toBeInTheDocument();
      expect(screen.queryByText('Failed to load roles')).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();

      const outside = screen.getByRole('button', { name: 'Outside task' });
      outside.focus();
      replacement.resolve(json(roles));
      await screen.findByText(roles[1].id, { exact: true });
      expect(screen.getByRole('button', { name: 'New Role' })).toBeEnabled();
      expect(screen.queryByRole('button', { name: 'Restart role check' })).toBeNull();
      expect(outside).toHaveFocus();
      expect(writes()).toHaveLength(0);
    },
  );

  it('retains local focus when a naturally settling read removes its focused restart', async () => {
    const pending = deferred();
    readReply = () => pending.promise;
    mount();
    screen.getByRole('button', { name: 'Restart role check' }).focus();
    pending.resolve(json(roles));
    await screen.findByText(roles[1].id, { exact: true });
    expect(screen.getByRole('region', { name: 'Roles inventory' })).toHaveFocus();
    expect(reads()).toHaveLength(1);
    expect(writes()).toHaveLength(0);
  });

  it('retires the replacement on access withdrawal', async () => {
    const pending = deferred();
    readReply = () => pending.promise;
    const { setAllowed } = mount();
    await waitFor(() => expect(reads()).toHaveLength(1));
    const replacement = deferred();
    await restartPending(replacement, 2);
    setAllowed(false);
    expect(reads()[1][1]?.signal?.aborted).toBe(true);
    expect(screen.queryByRole('button', { name: 'Restart role check' })).toBeNull();
    pending.resolve(json(roles));
    replacement.resolve(json(roles));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.getByText('Roles are unavailable for this session.')).toBeVisible();
    expect(screen.queryByText(roles[0].id, { exact: true })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
    expect(writes()).toHaveLength(0);
  });

  it.each(['save', 'delete'] as const)(
    'keeps an uncertain %s withdrawn when a restarted read fails',
    async (operation) => {
      const { pending } = await checkAfterUnknownWrite(operation);
      const replacement = deferred();
      const inventory = await restartPending(replacement, 3);
      pending.resolve(json(roles));
      replacement.resolve(json({ error: 'Synthetic private upstream detail' }, 503));
      const check = await screen.findByRole('button', { name: 'Check saved roles' });
      expect(screen.getByText(/It may already have been applied/)).toBeVisible();
      expect(screen.getByText('Failed to load roles', { exact: true })).toBeVisible();
      expect(screen.queryByText(roles[1].id, { exact: true })).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
      expect(inventory).toHaveFocus();

      readReply = async () => json(roles);
      fireEvent.click(check);
      await screen.findByText(roles[1].id, { exact: true });
      expect(screen.queryByText(/It may already have been applied/)).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'New Role' })).toBeEnabled();
      expect(reads()).toHaveLength(4);
      expect(writes()).toHaveLength(1);
    },
  );

  it.each([200, 503])(
    'keeps the same local focus target after a retry returns %s',
    async (status) => {
      mount();
      const { pending, inventory } = await retry();
      pending.resolve(
        json(status === 200 ? roles : { error: 'Synthetic private upstream detail' }, status),
      );
      await waitFor(() => expect(inventory).toHaveAttribute('aria-busy', 'false'));
      expect(inventory).toHaveFocus();
      expect(screen.getByRole('region', { name: 'Roles inventory' })).toBe(inventory);
      if (status === 200) {
        expect(await screen.findByText(roles[1].id, { exact: true })).toBeInTheDocument();
        expect(screen.getAllByText('Fleet auditor', { exact: true })).toHaveLength(2);
        expect(screen.getByRole('button', { name: 'New Role' })).toBeEnabled();
      } else expect(screen.getByRole('button', { name: 'Retry' })).toBeEnabled();
      expect(screen.queryByText('Synthetic private upstream detail')).not.toBeInTheDocument();
      expect(reads()).toHaveLength(2);
      expect(writes()).toHaveLength(0);
    },
  );

  it.each(['save', 'delete'] as const)(
    'checks an uncertain %s without repeating the write',
    async (operation) => {
      const { pending, inventory } = await checkAfterUnknownWrite(operation);
      pending.resolve(json(roles));
      await waitFor(() => expect(inventory).toHaveAttribute('aria-busy', 'false'));
      expect(inventory).toHaveFocus();
      expect(screen.queryByRole('button', { name: 'Check saved roles' })).toBeNull();
      expect(screen.getByText(roles[1].id, { exact: true })).toBeInTheDocument();
      expect(writes()).toHaveLength(1);
      expect(reads()).toHaveLength(2);
    },
  );

  it.each(['save', 'delete'] as const)(
    'retries only the saved inventory after an uncertain %s and failed recovery read',
    async (operation) => {
      const { pending, inventory } = await checkAfterUnknownWrite(operation);
      pending.resolve(json({ error: 'Synthetic private upstream detail' }, 503));
      const check = await screen.findByRole('button', { name: 'Check saved roles' });
      expect(inventory).toHaveFocus();
      expect(check).toBeEnabled();
      expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
      expect(screen.getByText(/It may already have been applied/)).toBeInTheDocument();
      expect(screen.getByText('Failed to load roles', { exact: true })).toBeInTheDocument();
      expect(screen.queryByText('Synthetic private upstream detail')).not.toBeInTheDocument();
      expect(writes()).toHaveLength(1);
      expect(reads()).toHaveLength(2);

      const nextRead = deferred();
      readReply = () => nextRead.promise;
      check.focus();
      fireEvent.click(check);
      expect(inventory).toHaveFocus();
      expect(inventory).toHaveAttribute('aria-busy', 'true');
      await waitFor(() => expect(reads()).toHaveLength(3));
      expect(reads()[2][1]).toMatchObject({
        headers: {
          'X-Pulse-Org-ID': 'estate/north',
          'X-API-Token': 'synthetic-role-recovery-token',
        },
        signal: expect.any(AbortSignal),
      });
      expect(writes()).toHaveLength(1);

      nextRead.resolve(json(roles));
      await waitFor(() => expect(inventory).toHaveAttribute('aria-busy', 'false'));
      expect(inventory).toHaveFocus();
      expect(screen.queryByRole('button', { name: 'Check saved roles' })).toBeNull();
      expect(screen.getByRole('button', { name: 'New Role' })).toBeEnabled();
      expect(screen.getByText(roles[1].id, { exact: true })).toBeInTheDocument();
      expect(reads()).toHaveLength(3);
      expect(writes()).toHaveLength(1);
      expect(fetchMock).toHaveBeenCalledTimes(4);
    },
  );

  it('does not pull focus back after the reader deliberately moves elsewhere', async () => {
    mount();
    const { pending, inventory } = await retry();
    const outside = screen.getByRole('button', { name: 'Outside task' });
    outside.focus();
    pending.resolve(json(roles));
    await waitFor(() => expect(inventory).toHaveAttribute('aria-busy', 'false'));
    expect(outside).toHaveFocus();
  });

  it('does not dispatch a duplicate GET when an obsolete recovery control fires again', async () => {
    mount();
    const { pending, inventory, button } = await retry();
    fireEvent.click(button);
    expect(reads()).toHaveLength(2);
    pending.resolve(json(roles));
    await waitFor(() => expect(inventory).toHaveAttribute('aria-busy', 'false'));
    expect(writes()).toHaveLength(0);
  });

  it.each(['user', 'organization', 'permission', 'unmount'])(
    'does not refocus or recreate saved data when %s retires a pending reader',
    async (change) => {
      const { view, setUser, setAllowed } = mount();
      const { pending } = await retry();
      readReply = async () => json([]);
      if (change === 'user') setUser('reader-b');
      else if (change === 'organization') {
        setOrgID('estate/south');
        eventBus.emit('org_switched', 'estate/south');
      } else if (change === 'permission') setAllowed(false);
      else view.unmount();
      const target = document.createElement('button');
      document.body.append(target);
      target.focus();
      pending.resolve(json(roles));
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(target).toHaveFocus();
      expect(screen.queryByText(roles[1].id, { exact: true })).not.toBeInTheDocument();
      expect(writes()).toHaveLength(0);
      target.remove();
    },
  );
});
