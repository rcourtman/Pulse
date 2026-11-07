import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { Organization, OrganizationMember, OrganizationShare } from '@/api/orgs';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { OrganizationAccessPanel } from '../OrganizationAccessPanel';
import { OrganizationOverviewPanel } from '../OrganizationOverviewPanel';
import { OrganizationSharingPanel } from '../OrganizationSharingPanel';

// PR 4093 refuses obsolete saved snapshots with 409 organization_changed.
// Exercise its consuming controls and real HTTP client, not mocked OrgsAPI
// methods. The winning fixture is independent of the refused request: a
// conflict is neither an applied write nor permission to restore old grants.
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/hooks/useResources', () => ({ useResources: () => ({ resources: () => [] }) }));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const panels = ['overview', 'access', 'sharing'] as const;
type Panel = (typeof panels)[number];
const recoveries = ['manager', 'viewer', 'denied', 'partial'] as const;
type Recovery = (typeof recoveries)[number];
const orgID = 'north-estate';
const user = 'operator@example.test';
const memberID = 'colleague@example.test';
const nextOwner = 'next-owner@example.test';
const storedToken = 'synthetic-unrelated-saved-token';
const csrf = 'synthetic-conflict-csrf';
const orgPath = `/api/orgs/${orgID}`;
const shareID = 'share/second?resource=vm::101';
const sharePath = `${orgPath}/shares/${encodeURIComponent(shareID)}`;
const winningName = 'Current saved estate';
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
const writes = () =>
  fetchMock.mock.calls.filter(([, options]) => (options?.method ?? 'GET') !== 'GET');
const reads = () =>
  fetchMock.mock.calls.filter(
    ([url, options]) => (options?.method ?? 'GET') === 'GET' && String(url).startsWith('/api/org'),
  );
let winning: boolean;
let recovery: Recovery;
let completion: ReturnType<typeof deferred<Response>>;
let readGate: ReturnType<typeof deferred<void>> | undefined;
let freshChange: boolean;
let unexpected: string[];

function members(): OrganizationMember[] {
  return [
    {
      userId: user,
      role: winning ? (recovery === 'viewer' ? 'viewer' : 'admin') : 'owner',
      addedAt: '',
    },
    {
      userId: memberID,
      role: winning ? (freshChange ? 'admin' : 'editor') : 'viewer',
      addedAt: '',
    },
    ...(winning ? [{ userId: nextOwner, role: 'owner' as const, addedAt: '' }] : []),
  ];
}
function organization(): Organization {
  return {
    id: orgID,
    displayName: winning
      ? freshChange
        ? 'Deliberately reviewed name'
        : winningName
      : 'Old estate',
    ownerUserId: winning ? nextOwner : user,
    members: members(),
  };
}
function outgoing(): OrganizationShare[] {
  return [
    {
      id: shareID,
      targetOrgId: 'peer-estate',
      resourceType: 'vm',
      resourceId: winning ? 'vm::202' : 'vm::101',
      resourceName: winning ? 'Current saved VM' : 'Old VM',
      accessRole: winning ? 'editor' : 'viewer',
      status: winning ? 'accepted' : 'pending',
      createdAt: '2026-11-01T00:00:00Z',
      createdBy: user,
    },
    {
      id: 'sibling-share',
      targetOrgId: 'peer-estate',
      resourceType: 'node',
      resourceId: 'node::sibling',
      resourceName: 'Unchanged sibling',
      accessRole: 'viewer',
      status: 'pending',
      createdAt: '2026-11-01T00:00:00Z',
      createdBy: user,
    },
  ].filter((share) => !freshChange || share.id !== shareID) as OrganizationShare[];
}
function expectBoundRequests() {
  for (const [url, options] of [...reads(), ...writes()]) {
    expect(String(url)).not.toContain('/default');
    expect(options).toMatchObject({ credentials: 'include', signal: expect.any(AbortSignal) });
    const headers = new Headers(options?.headers);
    expect(headers.get('X-Pulse-Org-ID')).toBe(orgID);
    expect(headers.has('X-API-Token')).toBe(false);
    if ((options?.method ?? 'GET') !== 'GET') expect(headers.get('X-CSRF-Token')).toBe(csrf);
  }
  expect(getOrgID()).toBe(orgID);
  expect(getApiToken()).toBe(storedToken);
}
function expectUnknownSnapshot() {
  expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
  expect(screen.queryByRole('combobox', { name: /^Role for / })).not.toBeInTheDocument();
  expect(screen.queryByLabelText('Invite user ID')).not.toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: 'Create Share' })).not.toBeInTheDocument();
  expect(screen.queryByText('Old VM')).not.toBeInTheDocument();
  expect(screen.queryByText('Current saved VM')).not.toBeInTheDocument();
  for (const empty of [
    'No members found.',
    'No organization members found.',
    'No pending invitations for this organization.',
    'No outgoing shares configured.',
    'No incoming shares from other organizations.',
  ])
    expect(screen.queryByText(empty)).not.toBeInTheDocument();
}
function shareRemoveButton() {
  const row = screen.getByText(winning ? 'Current saved VM' : 'Old VM').closest('tr')!;
  return within(row).getByRole('button', { name: 'Remove' });
}
async function openAndSubmit(panel: Panel) {
  if (panel === 'overview') {
    render(() => <OrganizationOverviewPanel currentUser={user} />);
    const input = await screen.findByLabelText('Display Name');
    expect(input).toHaveValue('Old estate');
    fireEvent.input(input, { target: { value: 'Refused stale name' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
  } else if (panel === 'access') {
    render(() => <OrganizationAccessPanel currentUser={user} />);
    const selector = await screen.findByRole('combobox', { name: `Role for ${memberID}` });
    fireEvent.change(selector, { target: { value: 'admin' } });
  } else {
    render(() => <OrganizationSharingPanel currentUser={user} />);
    await screen.findByText('Old VM');
    fireEvent.click(shareRemoveButton());
  }
  await waitFor(() => expect(writes()).toHaveLength(1));
  const [path, options] = writes()[0];
  expect(path).toBe(
    panel === 'overview' ? orgPath : panel === 'access' ? `${orgPath}/members` : sharePath,
  );
  expect(options?.method).toBe(
    panel === 'overview' ? 'PUT' : panel === 'access' ? 'POST' : 'DELETE',
  );
  expect(options?.body ? JSON.parse(String(options.body)) : undefined).toEqual(
    panel === 'overview'
      ? { displayName: 'Refused stale name' }
      : panel === 'access'
        ? { userId: memberID, role: 'admin' }
        : undefined,
  );
  expectBoundRequests();
}
async function expectWinningSnapshot(panel: Panel) {
  if (panel === 'overview') {
    expect(await screen.findByLabelText('Display Name')).toHaveValue(winningName);
    expect(screen.queryByDisplayValue('Refused stale name')).not.toBeInTheDocument();
    expect(screen.getByText(nextOwner)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled();
  } else if (panel === 'access') {
    expect(await screen.findByRole('combobox', { name: `Role for ${memberID}` })).toHaveValue(
      'editor',
    );
    expect(screen.getByRole('combobox', { name: `Role for ${user}` })).toHaveValue('admin');
    expect(screen.getByRole('combobox', { name: `Role for ${nextOwner}` })).toHaveValue('owner');
    expect(
      screen.getByRole('group', { name: 'Pending invitation for current-invite@example.test' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('old-invite@example.test')).not.toBeInTheDocument();
  } else {
    const row = (await screen.findByText('Current saved VM')).closest('tr')!;
    expect(row).toHaveTextContent('vm:vm::202');
    expect(row).toHaveTextContent('Accepted');
    expect(row).toHaveTextContent('editor');
    expect(screen.queryByText('Old VM')).not.toBeInTheDocument();
    expect(screen.getByText('Unchanged sibling')).toBeInTheDocument();
    expect(shareRemoveButton()).toBeEnabled();
  }
}
async function deliberatelyChangeCurrent(panel: Panel) {
  if (panel === 'overview') {
    fireEvent.input(screen.getByLabelText('Display Name'), {
      target: { value: 'Deliberately reviewed name' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
  } else if (panel === 'access') {
    fireEvent.change(screen.getByRole('combobox', { name: `Role for ${memberID}` }), {
      target: { value: 'admin' },
    });
  } else fireEvent.click(shareRemoveButton());
  await waitFor(() => expect(notices.success).toHaveBeenCalledTimes(1));
  expect(writes()).toHaveLength(2);
  if (panel === 'overview')
    await waitFor(() =>
      expect(screen.getByLabelText('Display Name')).toHaveValue('Deliberately reviewed name'),
    );
  else if (panel === 'access')
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: `Role for ${memberID}` })).toHaveValue('admin'),
    );
  else {
    await screen.findByText('Unchanged sibling');
    expect(screen.queryByText('Current saved VM')).not.toBeInTheDocument();
    expect(vi.mocked(window.confirm)).toHaveBeenLastCalledWith('Remove share for vm:vm::202?');
  }
  expect(freshChange).toBe(true);
  expectBoundRequests();
}

beforeEach(async () => {
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal(
    'confirm',
    vi.fn(() => true),
  );
  vi.stubEnv('MODE', 'production');
  sessionStorage.clear();
  clearAuth();
  setOrgID(orgID);
  setApiToken(storedToken);
  syncSessionPresentationPolicy(null);
  document.cookie = `pulse_csrf=${csrf}; Path=/`;
  winning = false;
  recovery = 'manager';
  completion = deferred<Response>();
  readGate = undefined;
  freshChange = false;
  unexpected = [];
  fetchMock.mockImplementation(async (input, options = {}) => {
    const path = String(input);
    const method = options.method ?? 'GET';
    if (method !== 'GET') {
      const known =
        (path === orgPath && method === 'PUT') ||
        (path === `${orgPath}/members` && method === 'POST') ||
        (path === sharePath && method === 'DELETE');
      if (known) {
        if (writes().length === 1) return completion.promise;
        // A second request is only legitimate after deliberate review. Its
        // actual payload, not the intended test operation, drives readback.
        if (method === 'PUT')
          freshChange =
            JSON.parse(String(options.body)).displayName === 'Deliberately reviewed name';
        else if (method === 'POST') {
          const payload = JSON.parse(String(options.body));
          freshChange = payload.userId === memberID && payload.role === 'admin';
        } else freshChange = path === sharePath;
        return method === 'DELETE'
          ? new Response(null, { status: 204 })
          : json(method === 'PUT' ? organization() : { kind: 'member', member: members()[1] });
      }
    } else {
      if (path === '/api/license/runtime-capabilities')
        return json({ capabilities: ['multi_tenant'], limits: [] });
      if (path.startsWith('/api/org')) {
        if (readGate) await readGate.promise;
        if (winning && recovery === 'denied')
          return json({ error: 'Forbidden', code: 'forbidden' }, 403);
        if (path === orgPath) return json(organization());
        if (path === `${orgPath}/members`)
          return winning && recovery === 'partial'
            ? json({ error: 'Synthetic saved members unavailable' }, 503)
            : json(members());
        if (path === '/api/orgs')
          return json([organization(), { id: 'peer-estate', displayName: 'Peer estate' }]);
        if (path === `${orgPath}/shares`) return json(outgoing());
        if (path === `${orgPath}/shares/incoming` || path === '/api/org-invitations')
          return json([]);
        if (path === `${orgPath}/invitations`)
          return json([
            {
              userId: winning ? 'current-invite@example.test' : 'old-invite@example.test',
              role: 'viewer',
              invitedAt: '',
              invitedBy: nextOwner,
            },
          ]);
      }
    }
    unexpected.push(`${method} ${path}`);
    throw new Error(`Unexpected synthetic conflict request: ${method} ${path}`);
  });
  await loadRuntimeCapabilities(true);
});
afterEach(() => {
  cleanup();
  readGate?.resolve();
  clearAuth();
  syncSessionPresentationPolicy(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  expect(unexpected).toEqual([]);
});

describe('organization save conflict recovery through real controls and transport', () => {
  it.each(panels.flatMap((panel) => recoveries.map((next) => ({ panel, next }))))(
    'reloads the winning $next snapshot after a refused $panel change, without replay or restored grants',
    async ({ panel, next }) => {
      recovery = next;
      await openAndSubmit(panel);
      const originalReads = reads().length;
      // Another operation won on the server. The refused request has not
      // changed this fixture and must never be merged into it by the client.
      winning = true;
      completion.resolve(
        json(
          {
            error:
              'Organization changed while this action was pending. Reload and review it before trying again.',
            code: 'organization_changed',
            status_code: 409,
            timestamp: 1794079800,
            details: {},
          },
          409,
        ),
      );
      const alert = await screen.findByRole('alert');
      expect(alert).toHaveTextContent('could not be confirmed.');
      expect(alert).toHaveTextContent(orgID);
      expect(alert).toHaveTextContent('It does not repeat');
      expectUnknownSnapshot();
      expect(notices.success).not.toHaveBeenCalled();
      expect(notices.error).toHaveBeenCalledTimes(1);
      expect(reads()).toHaveLength(originalReads);
      expect(writes()).toHaveLength(1);
      expectBoundRequests();

      readGate = deferred<void>();
      const reloadLabel = `Reload ${panel === 'overview' ? 'organization' : panel}`;
      fireEvent.click(screen.getByRole('button', { name: reloadLabel }));
      await waitFor(() => expect(reads().length).toBeGreaterThan(originalReads));
      expectUnknownSnapshot();
      expect(writes()).toHaveLength(1);
      readGate.resolve();
      if (next === 'denied' || next === 'partial') {
        expect(await screen.findByRole('alert')).toHaveTextContent('Unable to load');
        expectUnknownSnapshot();
        expect(screen.getByRole('button', { name: reloadLabel })).toBeEnabled();
      } else if (next === 'viewer') {
        if (panel === 'sharing') await screen.findByText('Current saved VM');
        else await screen.findByText(nextOwner);
        expect(screen.queryByRole('combobox', { name: /^Role for / })).not.toBeInTheDocument();
        expect(screen.queryByLabelText('Invite user ID')).not.toBeInTheDocument();
        if (panel === 'overview') {
          expect(screen.getByLabelText('Display Name')).toHaveValue(winningName);
          expect(screen.getByLabelText('Display Name')).toBeDisabled();
          expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
          fireEvent.click(screen.getByRole('button', { name: 'Save' }));
        } else if (panel === 'sharing') {
          expect(screen.getByText('Current saved VM')).toBeInTheDocument();
          expect(screen.queryByRole('button', { name: 'Remove' })).not.toBeInTheDocument();
          expect(screen.queryByRole('heading', { name: 'Create Share' })).not.toBeInTheDocument();
        }
      } else {
        await expectWinningSnapshot(panel);
      }
      expect(reads()).toHaveLength(
        originalReads + (panel === 'access' && next === 'denied' ? 1 : originalReads),
      );
      expect(writes()).toHaveLength(1);
      expect(notices.success).not.toHaveBeenCalled();
      expectBoundRequests();
      if (next === 'manager') await deliberatelyChangeCurrent(panel);
    },
  );
});
