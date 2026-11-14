import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import type { Role, UserRoleAssignment } from '@/types/rbac';
import { UserAssignmentsPanel } from '../UserAssignmentsPanel';

vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));

const roles: Role[] = [
  {
    id: 'read/north',
    name: 'Reader',
    description: 'Read monitoring',
    permissions: [{ action: 'read', resource: 'nodes' }],
  },
  {
    id: 'operate/north',
    name: 'Operator',
    description: 'Operate selected resources',
    permissions: [{ action: 'write', resource: 'selected-nodes' }],
  },
];
const original: UserRoleAssignment = {
  username: 'sso:oidc:lab:subject/north?realm=read#saved',
  displayName: 'Alex Example',
  email: 'alex@example.invalid',
  providerType: 'oidc',
  providerId: 'lab',
  roleIds: [roles[0].id, 'saved/unlisted:constraint'],
};
const userPath = `/api/admin/users/${encodeURIComponent(original.username)}`;
const fetchMock = vi.fn<typeof fetch>();
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
let savedUser: UserRoleAssignment;
let permissionsFailed: boolean;
let release: (response: Response) => void;
let mutationReply: () => Promise<Response>;
const permissionReads = () =>
  fetchMock.mock.calls.filter(([url]) => url === `${userPath}/permissions`);
const writes = () => fetchMock.mock.calls.filter(([, init]) => init?.method === 'PUT');

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID('north');
  setApiToken('synthetic-only-access-token');
  vi.stubEnv('MODE', 'production');
  document.cookie = 'pulse_csrf=synthetic-only-csrf; Path=/';
  savedUser = structuredClone(original);
  permissionsFailed = false;
  const pending = new Promise<Response>((resolve) => {
    release = resolve;
  });
  mutationReply = () => pending;
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    const path = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET' && path === '/api/admin/users') return json([savedUser]);
    if (method === 'GET' && path === '/api/admin/roles') return json(roles);
    if (method === 'GET' && path === `${userPath}/permissions`)
      return permissionsFailed
        ? json({ error: 'synthetic private read detail' }, 503)
        : json([{ action: 'read', resource: 'nodes' }]);
    if (method === 'PUT' && path === `${userPath}/roles`) return mutationReply();
    throw new Error(`Unexpected synthetic access request: ${method} ${path}`);
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

async function startSave() {
  render(() => <UserAssignmentsPanel currentUser="access-maintainer" canManage />);
  fireEvent.click(
    await screen.findByRole('button', {
      name: `Manage access for Alex Example (${original.username})`,
    }),
  );
  const dialog = await screen.findByRole('dialog', {
    name: `Manage access: Alex Example (${original.username})`,
  });
  const editor = within(dialog);
  await waitFor(() => expect(permissionReads()).toHaveLength(1));
  if (permissionsFailed) await editor.findByRole('button', { name: 'Retry saved permissions' });
  else await editor.findByText('nodes');
  const operator = editor.getByRole('checkbox', { name: /Operator \(Role ID:/ });
  fireEvent.click(operator);
  expect(operator).toBeChecked();
  fireEvent.click(editor.getByRole('button', { name: 'Save Changes' }));
  await waitFor(() => expect(writes()).toHaveLength(1));
  return { dialog, editor };
}

function expectFrozenEditor(dialog: HTMLElement) {
  const editor = within(dialog);
  const controls = [
    ...editor.getAllByRole('checkbox'),
    editor.getByRole('button', { name: 'Close user assignments' }),
    editor.getByRole('button', { name: 'Cancel' }),
    editor.getByRole('button', { name: 'Applying...' }),
    ...editor.queryAllByRole('button', { name: 'Retry saved permissions' }),
  ];
  const before = editor
    .getAllByRole('checkbox')
    .map((input) => (input as HTMLInputElement).checked);
  for (const control of controls) {
    expect(control).toBeDisabled();
    // Native activation must not change a checkbox's pixels independently of
    // the draft already sent, close the editor, or start another request.
    control.click();
  }
  fireEvent.keyDown(document, { key: 'Escape' });
  expect(dialog).toBeInTheDocument();
  expect(
    editor.getAllByRole('checkbox').map((input) => (input as HTMLInputElement).checked),
  ).toEqual(before);
  expect(writes()).toHaveLength(1);
  expect(permissionReads()).toHaveLength(1);
  const [url, init] = writes()[0];
  expect(url).toBe(`${userPath}/roles`);
  expect(JSON.parse(String(init?.body))).toEqual({ roleIds: [...original.roleIds, roles[1].id] });
  expect(init).toMatchObject({
    credentials: 'include',
    signal: expect.any(AbortSignal),
    headers: {
      'X-Pulse-Org-ID': 'north',
      'X-API-Token': 'synthetic-only-access-token',
      'X-CSRF-Token': 'synthetic-only-csrf',
    },
  });
  expect(editor.getByText(original.username)).toBeInTheDocument();
  expect(editor.getByText('saved/unlisted:constraint')).toBeInTheDocument();
}

describe('User Access pending controls through the real state owner and transport', () => {
  it('freezes the submitted choices until success and then reads saved access', async () => {
    const { dialog } = await startSave();
    expectFrozenEditor(dialog);
    savedUser = { ...original, roleIds: [...original.roleIds, roles[1].id] };
    release(new Response(null, { status: 204 }));
    await screen.findByText(`Roles updated for ${original.username}`);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(writes()).toHaveLength(1);
    expect(fetchMock.mock.calls.filter(([url]) => url === '/api/admin/users')).toHaveLength(2);
  });

  it('restores a correctable draft and the same read-only retry after a definite refusal', async () => {
    permissionsFailed = true;
    const { dialog, editor } = await startSave();
    expectFrozenEditor(dialog);
    release(json({ error: 'synthetic private mutation detail' }, 409));
    await editor.findByText('Failed to update user roles');
    for (const input of editor.getAllByRole('checkbox')) expect(input).toBeEnabled();
    expect(editor.getByRole('checkbox', { name: /Operator \(Role ID:/ })).toBeChecked();
    expect(editor.getByRole('button', { name: 'Cancel' })).toBeEnabled();
    expect(editor.getByRole('button', { name: 'Close user assignments' })).toBeEnabled();
    permissionsFailed = false;
    fireEvent.click(editor.getByRole('button', { name: 'Retry saved permissions' }));
    await editor.findByText('nodes');
    expect(writes()).toHaveLength(1);
    expect(editor.getByRole('checkbox', { name: /Operator \(Role ID:/ })).toBeChecked();
    expect(document.body).not.toHaveTextContent('synthetic private');
    fireEvent.click(editor.getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Manage access for/ }));
    const reopened = within(await screen.findByRole('dialog'));
    expect(reopened.getByRole('checkbox', { name: /Operator \(Role ID:/ })).not.toBeChecked();
    expect(reopened.getByRole('button', { name: 'Save Changes' })).toBeDisabled();
  });

  it('withdraws an uncertain edit and checks saved access without replaying the write', async () => {
    const { dialog } = await startSave();
    expectFrozenEditor(dialog);
    release(json({ error: 'synthetic private mutation detail' }, 502));
    const check = await screen.findByRole('button', { name: 'Check saved access' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('It may already have been applied.');
    expect(screen.queryByRole('button', { name: /Manage access for/ })).not.toBeInTheDocument();
    fireEvent.click(check);
    await screen.findByRole('button', { name: /Manage access for/ });
    expect(writes()).toHaveLength(1);
    expect(document.body).not.toHaveTextContent('synthetic private');
  });
});
