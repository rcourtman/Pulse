import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { Role } from '@/types/rbac';
import { loadRuntimeCapabilities, runtimeCapabilitiesLoadError } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { RolesPanel } from '../RolesPanel';

// Existing Roles transport/lifetime tests replace the feature gate. Keep the
// actual capability producer, store, gate, panel, state and RBAC transport
// connected here, including overlapping capability reads. Only HTTP is synthetic.
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const orgID = 'estate/north?realm=lab#saved';
const token = 'synthetic-roles-capability-token';
const csrf = 'synthetic-roles-capability-csrf';
const capabilityPath = '/api/license/runtime-capabilities';
const rolesPath = '/api/admin/roles';
const originals: Role[] = ['first', 'second', 'third', 'built-in'].map((identity, index) => ({
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
const selected = originals[1];
const rolePath = (role = selected) => `${rolesPath}/${encodeURIComponent(role.id)}`;
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
const capabilityBody = (enabled: boolean) => ({
  capabilities: enabled ? ['rbac'] : [],
  limits: [],
});
const fetchMock = vi.fn<typeof fetch>();
const reads = (path: string) =>
  fetchMock.mock.calls.filter(([url, init]) => url === path && (init?.method ?? 'GET') === 'GET');
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) =>
    ['POST', 'PUT', 'DELETE'].includes(init?.method ?? 'GET'),
  );
const turn = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
type Operation = 'create' | 'edit' | 'delete';
type Capability = 'enabled' | 'withdrawn' | 'failed' | 'malformed';
let saved: Role[];
let capabilityReply: () => Promise<Response>;
let healthReply: () => Promise<Response>;
let mutationReply: (init: RequestInit) => Response | Promise<Response>;
let unexpected: string[];
let releases: (() => void)[];
let loads: Promise<void>[];

function deferred<T>(cleanupValue: T) {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  releases.push(() => resolve(cleanupValue));
  return { promise, resolve, reject };
}
function heldBody() {
  const body = deferred('');
  const response = json(null);
  const read = vi.spyOn(response, 'text').mockImplementation(() => body.promise);
  return { ...body, response, read };
}
function refresh(reply: () => Promise<Response>) {
  capabilityReply = reply;
  const pending = loadRuntimeCapabilities(true);
  loads.push(pending);
  return pending;
}
function capabilities(kind: Capability) {
  return refresh(async () => {
    if (kind === 'failed') throw new TypeError('synthetic-private-capability-unavailable');
    return json(
      kind === 'malformed'
        ? { capabilities: 'rbac', limits: [] }
        : capabilityBody(kind === 'enabled'),
    );
  });
}
function rowFor(role: Role) {
  const row = screen.getByText(role.id).closest('tr');
  expect(row).not.toBeNull();
  return within(row!);
}
async function openPanel() {
  render(() => <RolesPanel currentUser="synthetic-maintainer" canManage />);
  await screen.findByText(selected.description);
  expect(screen.getByRole('button', { name: 'New Role' })).toBeEnabled();
  expect(reads(rolesPath)).toHaveLength(1);
  expect(writes()).toHaveLength(0);
}
function openEditor(role = selected) {
  fireEvent.click(
    rowFor(role).getByRole('button', { name: `Edit role ${role.name} (Role ID: ${role.id})` }),
  );
  const dialog = screen.getByRole('dialog', {
    name: `Edit role: ${role.name} (Role ID: ${role.id})`,
  });
  expect(within(dialog).getByLabelText('Role ID')).toHaveValue(role.id);
  expect(within(dialog).getByLabelText('Role ID')).toBeDisabled();
  return within(dialog);
}
function prepare(operation: Operation) {
  if (operation === 'edit') {
    const editor = openEditor();
    fireEvent.input(editor.getByLabelText('Description'), {
      target: { value: 'Updated second policy' },
    });
    return editor.getByRole('button', { name: 'Update Role' });
  }
  if (operation === 'create') {
    fireEvent.click(screen.getByRole('button', { name: 'New Role' }));
    const editor = within(screen.getByRole('dialog', { name: 'New role' }));
    fireEvent.input(editor.getByLabelText('Role ID'), { target: { value: '  New Auditor  ' } });
    fireEvent.input(editor.getByLabelText('Role Name'), { target: { value: 'New auditor' } });
    return editor.getByRole('button', { name: 'Create Role' });
  }
  fireEvent.click(
    rowFor(selected).getByRole('button', {
      name: `Delete role ${selected.name} (Role ID: ${selected.id})`,
    }),
  );
  const dialog = within(
    screen.getByRole('dialog', { name: `Delete role: ${selected.name} (Role ID: ${selected.id})` }),
  );
  expect(dialog.getByText(selected.id)).toBeInTheDocument();
  return dialog.getByRole('button', { name: 'Delete Role' });
}
function expectWithdrawn() {
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
  for (const role of originals) expect(screen.queryByText(role.id)).toBeNull();
  expect(screen.queryByText('No roles available.')).toBeNull();
  expect(screen.queryByRole('button', { name: 'Check saved roles' })).toBeNull();
  expect(screen.queryByRole('link', { name: 'View plans' })).toBeNull();
}
function expectNoObsoleteFeedback() {
  expect(screen.queryByText(/Role Fleet auditor .* (saved|deleted)/)).toBeNull();
  expect(
    screen.queryByText(/could not be confirmed|Failed to save role|Failed to delete role/),
  ).toBeNull();
  expect(screen.queryByText(/synthetic-private/)).toBeNull();
}
function expectBoundRequests() {
  expect(unexpected).toEqual([]);
  for (const [url, init] of fetchMock.mock.calls) {
    expect(init?.credentials).toBe('include');
    if (url === '/api/health') continue;
    const headers = new Headers(init?.headers);
    expect(headers.get('X-Pulse-Org-ID')).toBe(orgID);
    expect(headers.get('X-API-Token')).toBe(token);
    if (url !== capabilityPath) expect(init?.signal).toBeInstanceOf(AbortSignal);
    if (['POST', 'PUT', 'DELETE'].includes(init?.method ?? 'GET'))
      expect(headers.get('X-CSRF-Token')).toBe(csrf);
    for (const internal of ['expectedOrgID', 'retry', 'preferSessionAuth', 'skipOrgContext'])
      expect(init).not.toHaveProperty(internal);
  }
  expect(getOrgID()).toBe(orgID);
  expect(getApiToken()).toBe(token);
}
async function expectFreshInventory(expected: Role[]) {
  await waitFor(() => {
    expect(reads(rolesPath)).toHaveLength(2);
    expect(screen.getByRole('button', { name: 'New Role' })).toBeEnabled();
    for (const role of expected)
      expect(rowFor(role).getByText(role.description)).toBeInTheDocument();
  });
  expect(screen.queryByRole('dialog')).toBeNull();
  expectNoObsoleteFeedback();
  expect(saved).toEqual(expected);
}
function expectedSaved(operation: 'edit' | 'delete') {
  return operation === 'edit'
    ? originals.map((role) =>
        role.id === selected.id ? { ...role, description: 'Updated second policy' } : role,
      )
    : originals.filter((role) => role.id !== selected.id);
}
function expectDispatched(operation: Operation) {
  expect(writes()).toHaveLength(1);
  const [path, init] = writes()[0];
  expect(path).toBe(operation === 'create' ? rolesPath : rolePath());
  expect(init?.method).toBe(
    operation === 'edit' ? 'PUT' : operation === 'create' ? 'POST' : 'DELETE',
  );
  if (operation === 'edit')
    expect(JSON.parse(String(init?.body))).toEqual({
      ...selected,
      description: 'Updated second policy',
    });
  else if (operation === 'delete') expect(init?.body).toBeUndefined();
  else
    expect(JSON.parse(String(init?.body))).toEqual({
      id: 'new-auditor',
      name: 'New auditor',
      description: '',
      permissions: [{ action: 'read', resource: 'nodes' }],
    });
}

beforeEach(async () => {
  clearAuth();
  sessionStorage.clear();
  localStorage.clear();
  setOrgID(orgID);
  setApiToken(token);
  vi.stubEnv('MODE', 'production');
  document.cookie = `pulse_csrf=${csrf}; Path=/`;
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode: false,
      readOnly: false,
      hideCommercial: false,
      hideUpgrade: true,
    },
  });
  saved = structuredClone(originals);
  unexpected = [];
  releases = [];
  loads = [];
  healthReply = async () => json({});
  mutationReply = async () => new Response(null, { status: 204 });
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    const path = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET') {
      if (path === capabilityPath) return capabilityReply();
      if (path === '/api/health') return healthReply();
      if (path === rolesPath) return json(saved);
    }
    // Apply only the dispatched opaque identity and complete body. Aborting
    // its local reader later cannot undo a write already applied by a server.
    if (path === rolesPath && method === 'POST') {
      saved.push(JSON.parse(String(init.body)) as Role);
      return mutationReply(init);
    }
    const role = saved.find((candidate) => rolePath(candidate) === path);
    if (role && method === 'PUT') {
      const replacement = JSON.parse(String(init.body)) as Role;
      saved = saved.map((candidate) => (candidate.id === role.id ? replacement : candidate));
      return mutationReply(init);
    }
    if (role && method === 'DELETE') {
      saved = saved.filter((candidate) => candidate.id !== role.id);
      return mutationReply(init);
    }
    unexpected.push(`${method} ${path}`);
    throw new Error('Unexpected synthetic Roles request');
  });
  vi.stubGlobal('fetch', fetchMock);
  await capabilities('enabled');
});

afterEach(async () => {
  cleanup();
  for (const release of releases) release();
  if (vi.isFakeTimers()) await vi.runOnlyPendingTimersAsync();
  vi.useRealTimers();
  await Promise.allSettled(loads);
  await turn();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  syncSessionPresentationPolicy(null);
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('Roles through real runtime capability transitions', () => {
  it('preserves the admitted deny-policy draft across an equal capability refresh, then saves it once', async () => {
    await openPanel();
    const confirmation = prepare('edit');
    await capabilities('enabled');
    const editor = within(screen.getByRole('dialog'));
    expect(editor.getByLabelText('Description')).toHaveValue('Updated second policy');
    expect(editor.getByLabelText('Permission 2 action')).toHaveValue('delete');
    expect(editor.getByLabelText('Permission 3 resource')).toHaveValue('future/second');
    expect(reads(rolesPath)).toHaveLength(1);
    expect(writes()).toHaveLength(0);
    fireEvent.click(confirmation);
    await waitFor(() => expect(reads(rolesPath)).toHaveLength(2));
    expectDispatched('edit');
    expect(saved).toEqual(expectedSaved('edit'));
    expectBoundRequests();
  });

  it.each(
    (['create', 'edit', 'delete'] as const).flatMap((operation) =>
      (['withdrawn', 'failed', 'malformed'] as const).map((kind) => ({ operation, kind })),
    ),
  )(
    'retires a cold $operation confirmation on $kind capability before dispatch',
    async ({ operation, kind }) => {
      const health = deferred(json({}));
      healthReply = () => health.promise;
      await openPanel();
      const confirmation = prepare(operation);
      document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
      fireEvent.click(confirmation);
      await waitFor(() => expect(reads('/api/health')).toHaveLength(1));
      expect(writes()).toHaveLength(0);
      vi.useFakeTimers();
      await capabilities(kind);
      expectWithdrawn();
      expect(Boolean(runtimeCapabilitiesLoadError())).toBe(kind === 'failed');
      document.cookie = `pulse_csrf=${csrf}; Path=/`;
      health.resolve(json({}));
      // Advance only the existing cookie-preparation delay; no production
      // retries, polling intervals or arbitrary sleeps are part of this proof.
      await vi.advanceTimersByTimeAsync(25);
      vi.useRealTimers();
      expect(writes()).toHaveLength(0);
      expect(saved).toEqual(originals);
      expect(reads(rolesPath)).toHaveLength(1);
      expectNoObsoleteFeedback();
      await capabilities('enabled');
      await expectFreshInventory(originals);
      expect(writes()).toHaveLength(0);
      expectBoundRequests();
    },
  );

  it('does not re-admit Roles when an older capability grant arrives after a newer withdrawal', async () => {
    await openPanel();
    prepare('edit');
    const oldReply = deferred(json(capabilityBody(false)));
    const oldLoad = refresh(() => oldReply.promise);
    await capabilities('withdrawn');
    expectWithdrawn();
    oldReply.resolve(json(capabilityBody(true)));
    await oldLoad;
    expectWithdrawn();
    expect(reads(rolesPath)).toHaveLength(1);
    expect(writes()).toHaveLength(0);
    expect(saved).toEqual(originals);
    expectNoObsoleteFeedback();
    expectBoundRequests();
  });

  it.each(['withdrawn', 'failed'] as const)(
    'does not discard the current saved-role draft when an older %s capability read settles late',
    async (outcome) => {
      await openPanel();
      const confirmation = prepare('edit');
      const oldReply = deferred(json(capabilityBody(false)));
      const oldLoad = refresh(() => oldReply.promise);
      await capabilities('enabled');
      if (outcome === 'withdrawn') oldReply.resolve(json(capabilityBody(false)));
      else oldReply.reject(new TypeError('synthetic-private-obsolete-capability-error'));
      await oldLoad;
      expect(runtimeCapabilitiesLoadError()).toBeNull();
      const editor = within(screen.getByRole('dialog'));
      expect(editor.getByLabelText('Description')).toHaveValue('Updated second policy');
      expect(editor.getByLabelText('Permission 3 action')).toHaveValue('future:inspect');
      expect(reads(rolesPath)).toHaveLength(1);
      fireEvent.click(confirmation);
      await waitFor(() => expect(reads(rolesPath)).toHaveLength(2));
      expectDispatched('edit');
      expect(saved).toEqual(expectedSaved('edit'));
      expectBoundRequests();
    },
  );

  it.each(
    (['edit', 'delete'] as const).flatMap((operation) =>
      (['success', 'unreadable'] as const).map((outcome) => ({ operation, outcome })),
    ),
  )(
    'rereads applied $operation state without importing its late $outcome body into a new draft',
    async ({ operation, outcome }) => {
      const reply = heldBody();
      mutationReply = () => reply.response;
      await openPanel();
      fireEvent.click(prepare(operation));
      await waitFor(() => expect(reply.read).toHaveBeenCalledOnce());
      expectDispatched(operation);
      const signal = writes()[0][1]?.signal;
      expect(signal?.aborted).toBe(false);
      const expected = expectedSaved(operation);
      expect(saved).toEqual(expected);
      await capabilities('withdrawn');
      expectWithdrawn();
      expect(signal?.aborted).toBe(true);
      await capabilities('enabled');
      await expectFreshInventory(expected);
      if (operation === 'delete') expect(screen.queryByText(selected.id)).toBeNull();
      const current = expected.find((role) => role.id === selected.id) ?? expected[0];
      const editor = openEditor(current);
      expect(editor.getByLabelText('Description')).toHaveValue(current.description);
      fireEvent.input(editor.getByLabelText('Description'), {
        target: { value: 'Current independent draft' },
      });
      reply.resolve(
        outcome === 'success' ? JSON.stringify(current) : '{"synthetic-private-partial":',
      );
      await turn();
      // A cached `within` can still find controls in a detached dialog. Check
      // the live surface as well, so an old response cannot silently close it.
      expect(screen.getByRole('dialog')).toContainElement(editor.getByLabelText('Role ID'));
      expect(editor.getByLabelText('Description')).toHaveValue('Current independent draft');
      expect(editor.getByLabelText('Permission 2 action')).toHaveValue('delete');
      expectNoObsoleteFeedback();
      expect(writes()).toHaveLength(1);
      expect(reads(rolesPath)).toHaveLength(2);
      expect(saved).toEqual(expected);
      expectBoundRequests();
    },
  );

  it.each(['edit', 'delete'] as const)(
    'reads the applied %s result after capability withdrawal actually aborts the dispatched fetch',
    async (operation) => {
      const response = deferred(new Response(null, { status: 204 }));
      const observedAbort = vi.fn();
      mutationReply = (init) => {
        init.signal!.addEventListener(
          'abort',
          () => {
            observedAbort();
            response.reject(new DOMException('Synthetic request aborted', 'AbortError'));
          },
          { once: true },
        );
        return response.promise;
      };
      await openPanel();
      fireEvent.click(prepare(operation));
      await waitFor(() => expect(writes()).toHaveLength(1));
      expectDispatched(operation);
      const signal = writes()[0][1]?.signal;
      expect(signal?.aborted).toBe(false);
      const expected = expectedSaved(operation);
      expect(saved).toEqual(expected);
      await capabilities('withdrawn');
      expectWithdrawn();
      await waitFor(() => expect(observedAbort).toHaveBeenCalledOnce());
      expect(signal?.aborted).toBe(true);
      expectNoObsoleteFeedback();
      expect(reads(rolesPath)).toHaveLength(1);
      await capabilities('enabled');
      await expectFreshInventory(expected);
      expect(writes()).toHaveLength(1);
      expectBoundRequests();
    },
  );
});
