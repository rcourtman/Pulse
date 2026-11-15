import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { Role, UserRoleAssignment } from '@/types/rbac';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';

// Keep the actual dialog, state, RBAC API and HTTP client connected. Only
// entitlement availability and the final network boundary are synthetic.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));
const roles: Role[] = [
  { id: 'read/nodes', name: 'Reader', description: 'Read monitoring', permissions: [] },
  {
    id: 'operate/selected',
    name: 'Operator',
    description: 'Operate selected nodes',
    permissions: [],
  },
];
const user: UserRoleAssignment = {
  username: 'sso:oidc:lab:subject/second?realm=north#saved',
  displayName: 'Alex Example',
  providerType: 'oidc',
  providerId: 'lab',
  roleIds: [roles[0].id, 'saved/unlisted:constraint'],
};
const permissionsPath = `/api/admin/users/${encodeURIComponent(user.username)}/permissions`;
const fetchMock = vi.fn<typeof fetch>();
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
let release: (response: Response) => void;
const permissionReads = () => fetchMock.mock.calls.filter(([url]) => url === permissionsPath);
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => !['GET', 'HEAD'].includes(init?.method ?? 'GET'));

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID('estate/north');
  setApiToken('synthetic-only-permission-recovery-auth');
  vi.stubEnv('MODE', 'production');
  document.cookie = 'pulse_csrf=synthetic-only-permission-recovery-csrf; Path=/';
  const pending = new Promise<Response>((resolve) => {
    release = resolve;
  });
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    const path = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET' && path === '/api/admin/users') return json([user]);
    if (method === 'GET' && path === '/api/admin/roles') return json(roles);
    if (method === 'GET' && path === permissionsPath) {
      if (permissionReads().length === 1) return json({ error: 'synthetic read failure' }, 503);
      return pending;
    }
    throw new Error(`Unexpected synthetic permission-recovery request: ${method} ${path}`);
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

async function openFailedRead() {
  const [reader, setReader] = createSignal({ user: 'access-maintainer', allowed: true });
  const view = render(() => (
    <>
      <button type="button">Outside task</button>
      <UserAssignmentsPanel currentUser={reader().user} canManage={reader().allowed} />
    </>
  ));
  const trigger = await screen.findByRole('button', {
    name: `Manage access for Alex Example (${user.username})`,
  });
  trigger.focus();
  fireEvent.click(trigger);
  const dialog = within(await screen.findByRole('dialog'));
  const retry = await dialog.findByRole('button', { name: 'Retry saved permissions' });
  return { dialog, retry, trigger, setReader, view };
}

async function startRetry() {
  const view = await openFailedRead();
  view.retry.focus();
  fireEvent.click(view.retry);
  await waitFor(() => expect(permissionReads()).toHaveLength(2));
  return view;
}

function expectReadOnlyBoundRequests() {
  expect(writes()).toHaveLength(0);
  for (const [, init] of fetchMock.mock.calls) {
    expect(init).toMatchObject({
      credentials: 'include',
      signal: expect.any(AbortSignal),
      headers: {
        'X-Pulse-Org-ID': 'estate/north',
        'X-API-Token': 'synthetic-only-permission-recovery-auth',
      },
    });
  }
}

describe('Saved-permissions retry keyboard recovery through real transport', () => {
  it.each([
    { outcome: 'saved grants', reply: [{ action: 'read', resource: 'saved/selected:nodes' }] },
    { outcome: 'confirmed empty', reply: [] },
    { outcome: 'malformed response', reply: [{ action: 'read' }] },
  ])(
    'keeps keyboard review at the result during and after $outcome',
    async ({ outcome, reply }) => {
      const { dialog, retry } = await startRetry();
      const region = dialog.getByRole('region', { name: 'Saved permissions' });
      expect(retry).not.toBeInTheDocument();
      expect(region).toHaveAttribute('tabindex', '-1');
      expect(region).toHaveAttribute('aria-busy', 'true');
      expect(region).toHaveFocus();
      release(json(reply));
      await waitFor(() => expect(region).toHaveAttribute('aria-busy', 'false'));
      expect(region).toHaveFocus();
      if (outcome === 'saved grants')
        expect(within(region).getByText('saved/selected:nodes')).toBeVisible();
      else if (outcome === 'confirmed empty')
        expect(
          within(region).getByText("No permissions were returned for this user's saved roles."),
        ).toBeVisible();
      else {
        expect(within(region).getByRole('alert')).toHaveTextContent('Access is unknown.');
        expect(within(region).queryByText(/No permissions were returned/)).not.toBeInTheDocument();
        expect(
          within(region).getByRole('button', { name: 'Retry saved permissions' }),
        ).toBeVisible();
      }
      expect(dialog.getByText(user.username)).toBeVisible();
      expect(dialog.getByRole('checkbox', { name: /Reader \(Role ID:/ })).toBeChecked();
      expect(dialog.getByRole('checkbox', { name: /Operator \(Role ID:/ })).not.toBeChecked();
      expect(dialog.getByText('saved/unlisted:constraint')).toBeVisible();
      expect(permissionReads()).toHaveLength(2);
      expectReadOnlyBoundRequests();
    },
  );

  it('does not steal a deliberate focus move or replace an unsaved role choice', async () => {
    const { dialog } = await startRetry();
    const operator = dialog.getByRole('checkbox', { name: /Operator \(Role ID:/ });
    operator.focus();
    fireEvent.click(operator);
    release(json([{ action: 'read', resource: 'saved/selected:nodes' }]));
    await dialog.findByText('saved/selected:nodes');
    expect(operator).toHaveFocus();
    expect(operator).toBeChecked();
    expect(dialog.getByText(/Unsaved role selections are not included/)).toBeVisible();
    expectReadOnlyBoundRequests();
  });

  it('does not move initial editor focus when a read fails without an explicit retry', async () => {
    const { dialog } = await openFailedRead();
    expect(dialog.getByRole('button', { name: 'Close user assignments' })).toHaveFocus();
    expectReadOnlyBoundRequests();
  });

  it('cancels an in-flight read on Escape without reopening or refocusing the old result', async () => {
    const { trigger } = await startRetry();
    const signal = permissionReads()[1][1]?.signal;
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(signal?.aborted).toBe(true);
    expect(trigger).toHaveFocus();
    release(json([{ action: 'read', resource: 'late/old-reader' }]));
    await Promise.resolve();
    expect(screen.queryByText('late/old-reader')).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
    expectReadOnlyBoundRequests();
  });

  it.each(['permission', 'principal', 'organization', 'unmount'] as const)(
    'never refocuses a withdrawn %s reader after a late completion',
    async (change) => {
      const { setReader, view } = await startRetry();
      const signal = permissionReads()[1][1]?.signal;
      if (change === 'permission') setReader({ user: 'access-maintainer', allowed: false });
      if (change === 'principal') setReader({ user: 'different-maintainer', allowed: true });
      if (change === 'organization') {
        setOrgID('estate/south');
        eventBus.emit('org_switched', 'estate/south');
      }
      if (change === 'unmount') view.unmount();
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(signal?.aborted).toBe(true);
      const deliberateTarget = document.createElement('button');
      deliberateTarget.textContent = 'New task';
      document.body.append(deliberateTarget);
      deliberateTarget.focus();
      release(json([{ action: 'read', resource: 'late/old-reader' }]));
      await Promise.resolve();
      expect(deliberateTarget).toHaveFocus();
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(screen.queryByText('late/old-reader')).not.toBeInTheDocument();
      expect(writes()).toHaveLength(0);
      deliberateTarget.remove();
    },
  );
});
