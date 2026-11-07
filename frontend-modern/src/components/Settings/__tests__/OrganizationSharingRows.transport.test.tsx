import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { IncomingOrganizationShare, OrganizationShare } from '@/api/orgs';
import { eventBus } from '@/stores/events';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { OrganizationSharingPanel } from '../OrganizationSharingPanel';

// Existing transport tests call the state owner directly; component tests mock
// OrgsAPI. Keep the real grid, row callback, state and HTTP client connected so
// a namesake row cannot silently supply the identity of a different grant.
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/hooks/useResources', () => ({ useResources: () => ({ resources: () => [] }) }));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const orgID = 'operations/east';
const owner = 'owner@example.test';
const storedToken = 'synthetic-unrelated-api-token';
const csrfToken = 'synthetic-session-csrf';
const base = `/api/orgs/${encodeURIComponent(orgID)}`;
const operations = ['remove outgoing', 'accept incoming', 'decline incoming'] as const;
type Operation = (typeof operations)[number];
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
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

function outgoingRows(): OrganizationShare[] {
  return ['first', 'second', 'third'].map((identity) => ({
    id: `outgoing/${identity}?share=vm::101`,
    targetOrgId: `peer/${identity}`,
    resourceType: 'vm',
    resourceId: `outgoing::${identity}/101`,
    resourceName: 'Namesake VM',
    accessRole: 'viewer',
    status: 'pending',
    createdAt: '2026-11-01T00:00:00Z',
    createdBy: owner,
  }));
}
function incomingRows(): IncomingOrganizationShare[] {
  return outgoingRows().map((share, index) => ({
    ...share,
    id: share.id.replace('outgoing/', 'incoming/'),
    resourceId: share.resourceId.replace('outgoing::', 'incoming::'),
    targetOrgId: orgID,
    sourceOrgId: `source/${['first', 'second', 'third'][index]}`,
    sourceOrgName: 'Namesake organization',
  }));
}
const fetchMock = vi.fn<typeof fetch>();
const confirmMock = vi.fn(() => true);
const writes = () =>
  fetchMock.mock.calls.filter(([, options]) => (options?.method ?? 'GET') !== 'GET');
const sharingReads = () =>
  fetchMock.mock.calls.filter(
    ([url, options]) =>
      (options?.method ?? 'GET') === 'GET' &&
      (url === '/api/orgs' || String(url).startsWith(`${base}`)),
  );
let outgoing: OrganizationShare[];
let incoming: IncomingOrganizationShare[];
let completion: ReturnType<typeof deferred<Response>>;
let reload: ReturnType<typeof deferred<Response>> | undefined;

function rowFor(share: OrganizationShare): HTMLElement {
  const section = screen.getByRole('heading', {
    name: share.resourceId.startsWith('outgoing::') ? 'Outgoing Shares' : 'Incoming Shares',
  }).parentElement!;
  const row = within(section).getByText(`${share.resourceType}:${share.resourceId}`).closest('tr');
  expect(row).not.toBeNull();
  return row!;
}
function rowAction(operation: Operation, share = selectedShare(operation)) {
  return within(rowFor(share)).getByRole('button', {
    name:
      operation === 'remove outgoing'
        ? 'Remove'
        : operation === 'accept incoming'
          ? 'Accept'
          : 'Decline',
  });
}
function selectedShare(operation: Operation) {
  return operation === 'remove outgoing' ? outgoingRows()[1] : incomingRows()[1];
}
function expectBoundRequest() {
  expect(writes()).toHaveLength(1);
  for (const [, options] of [...sharingReads(), ...writes()]) {
    expect(options).toMatchObject({
      credentials: 'include',
      signal: expect.any(AbortSignal),
      headers: { 'X-Pulse-Org-ID': orgID },
    });
    expect(options?.headers).not.toHaveProperty('X-API-Token');
  }
  expect(writes()[0][1]?.headers).toMatchObject({ 'X-CSRF-Token': csrfToken });
  expect(getOrgID()).toBe(orgID);
  expect(getApiToken()).toBe(storedToken);
}
async function openPanel(operation: Operation) {
  render(() => <OrganizationSharingPanel currentUser={owner} />);
  await screen.findByRole('heading', { name: 'Outgoing Shares' });
  expect(screen.getAllByText('Namesake VM')).toHaveLength(6);
  expect(sharingReads()).toHaveLength(5);
  const selected = selectedShare(operation);
  const button = rowAction(operation);
  expect(button).toBeEnabled();
  fireEvent.click(button);
  await waitFor(() => expect(writes()).toHaveLength(1));
  const [url, options] = writes()[0];
  const path = `${base}/shares${operation === 'remove outgoing' ? '' : '/incoming'}/${encodeURIComponent(selected.id)}${operation === 'accept incoming' ? '/accept' : ''}`;
  expect(url).toBe(path);
  expect(options?.method).toBe(operation === 'accept incoming' ? 'POST' : 'DELETE');
  expect(options?.body).toBeUndefined();
  expectBoundRequest();
  if (operation === 'remove outgoing')
    expect(confirmMock).toHaveBeenCalledExactlyOnceWith(
      `Remove share for vm:${selected.resourceId}?`,
    );
  else if (operation === 'decline incoming')
    expect(confirmMock).toHaveBeenCalledExactlyOnceWith(
      'Decline the share request for Namesake VM?',
    );
  else expect(confirmMock).not.toHaveBeenCalled();
  return selected;
}
// Apply the actual dispatched identity, not the test's intended identity. A
// wrong-row callback must change the namesake in the fixture, not look correct.
function applyDispatchedWrite() {
  const [url] = writes()[0];
  const path = String(url);
  const id = decodeURIComponent(path.split('/').at(path.endsWith('/accept') ? -2 : -1)!);
  if (path.includes('/incoming/')) {
    if (path.endsWith('/accept'))
      incoming = incoming.map((share) =>
        share.id === id
          ? { ...share, status: 'accepted', acceptedAt: '2026-11-07T10:00:00Z', acceptedBy: owner }
          : share,
      );
    else incoming = incoming.filter((share) => share.id !== id);
  } else outgoing = outgoing.filter((share) => share.id !== id);
}
function expectSavedRows(operation: Operation, applied: boolean) {
  const selected = selectedShare(operation);
  const siblings = operation === 'remove outgoing' ? outgoingRows() : incomingRows();
  for (const sibling of [siblings[0], siblings[2]]) {
    expect(rowFor(sibling)).toHaveTextContent('Pending approval');
    expect(rowFor(sibling)).toHaveTextContent('Namesake VM');
    expect(
      within(rowFor(sibling)).getByRole('button', {
        name: operation === 'remove outgoing' ? 'Remove' : 'Accept',
      }),
    ).toBeEnabled();
  }
  const isRemoved = applied && operation !== 'accept incoming';
  if (isRemoved) expect(screen.queryByText(`vm:${selected.resourceId}`)).not.toBeInTheDocument();
  else {
    const row = rowFor(selected);
    expect(row).toHaveTextContent(applied ? 'Accepted' : 'Pending approval');
    if (operation === 'accept incoming' || operation === 'decline incoming') {
      if (applied)
        expect(within(row).queryByRole('button', { name: 'Accept' })).not.toBeInTheDocument();
      else expect(within(row).getByRole('button', { name: 'Accept' })).toBeEnabled();
    }
  }
  const otherDirection = operation === 'remove outgoing' ? incomingRows() : outgoingRows();
  for (const share of otherDirection) expect(rowFor(share)).toHaveTextContent('Pending approval');
  expect(screen.getAllByText('Namesake VM')).toHaveLength(isRemoved ? 5 : 6);
  expectBoundRequest();
}

beforeEach(async () => {
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('confirm', confirmMock);
  vi.stubEnv('MODE', 'production');
  sessionStorage.clear();
  clearAuth();
  setOrgID(orgID);
  setApiToken(storedToken);
  syncSessionPresentationPolicy(null);
  document.cookie = `pulse_csrf=${csrfToken}; Path=/`;
  outgoing = outgoingRows();
  incoming = incomingRows();
  completion = deferred<Response>();
  reload = undefined;
  fetchMock.mockImplementation(async (input, options = {}) => {
    const url = String(input);
    if ((options.method ?? 'GET') !== 'GET') {
      const method = options.method;
      const known =
        (method === 'DELETE' &&
          outgoing.some((share) => url === `${base}/shares/${encodeURIComponent(share.id)}`)) ||
        incoming.some(
          (share) =>
            (method === 'POST' &&
              url === `${base}/shares/incoming/${encodeURIComponent(share.id)}/accept`) ||
            (method === 'DELETE' &&
              url === `${base}/shares/incoming/${encodeURIComponent(share.id)}`),
        );
      return known ? completion.promise : json({ error: 'Unknown synthetic share' }, 404);
    }
    if (url === '/api/license/runtime-capabilities')
      return json({ capabilities: ['multi_tenant'], limits: [], max_history_days: 7 });
    if (url === base) {
      if (reload) return reload.promise;
      return json({ id: orgID, displayName: 'East operations', ownerUserId: owner });
    }
    if (url === '/api/orgs')
      return json([
        { id: orgID, displayName: 'East operations' },
        { id: 'peer/first', displayName: 'Namesake organization' },
        { id: 'peer/second', displayName: 'Namesake organization' },
        { id: 'peer/third', displayName: 'Namesake organization' },
      ]);
    if (url === `${base}/members`) return json([{ userId: owner, role: 'owner', addedAt: '' }]);
    if (url === `${base}/shares`) return json(outgoing);
    if (url === `${base}/shares/incoming`) return json(incoming);
    throw new Error(`Unexpected synthetic request: ${options.method ?? 'GET'} ${url}`);
  });
  await loadRuntimeCapabilities(true);
});
afterEach(() => {
  cleanup();
  completion.resolve(new Response(null, { status: 204 }));
  reload?.resolve(json({ id: orgID }));
  clearAuth();
  setOrgID(null);
  syncSessionPresentationPolicy(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('rendered namesake sharing rows through the real transport', () => {
  it.each(operations)(
    'confirms only the selected %s and renders its saved result',
    async (operation) => {
      await openPanel(operation);
      applyDispatchedWrite();
      completion.resolve(new Response(null, { status: 204 }));
      await waitFor(() => expect(sharingReads()).toHaveLength(10));
      await screen.findByRole('heading', { name: 'Outgoing Shares' });
      expectSavedRows(operation, true);
      expect(notices.success).toHaveBeenCalledTimes(1);
      expect(notices.error).not.toHaveBeenCalled();
    },
  );

  it.each([
    { operation: 'remove outgoing', applied: true },
    { operation: 'accept incoming', applied: false },
  ] as const)(
    'checks saved $operation after an uncertain response (applied: $applied) without replay',
    async ({ operation, applied }) => {
      const selected = await openPanel(operation);
      expect(rowAction(operation)).toBeDisabled();
      eventBus.emit('organizations_changed');
      expect(sharingReads()).toHaveLength(5);
      expect(writes()[0][1]?.signal?.aborted).toBe(false);
      if (applied) applyDispatchedWrite();
      completion.resolve(
        new Response('{"error":"synthetic-unconfirmed"}', {
          status: operation === 'accept incoming' ? 403 : 429,
          headers: { 'Retry-After': '0.001', 'X-CSRF-Token': 'synthetic-updated-csrf' },
        }),
      );
      const alert = await screen.findByRole('alert');
      expect(alert).toHaveTextContent('The last sharing change could not be confirmed.');
      expect(alert).toHaveTextContent(
        `Reload checks saved incoming and outgoing shares for ${orgID}.`,
      );
      expect(screen.queryByText(`vm:${selected.resourceId}`)).not.toBeInTheDocument();
      expect(screen.queryByRole('heading', { name: 'Create Share' })).not.toBeInTheDocument();
      expect(sharingReads()).toHaveLength(5);
      expect(writes()).toHaveLength(1);
      expect(notices.success).not.toHaveBeenCalled();
      expect(notices.error).toHaveBeenCalledTimes(1);

      reload = deferred<Response>();
      const button = screen.getByRole('button', { name: 'Reload sharing' });
      fireEvent.click(button);
      await waitFor(() => expect(sharingReads()).toHaveLength(10));
      expect(screen.queryByText(`vm:${selected.resourceId}`)).not.toBeInTheDocument();
      expect(writes()).toHaveLength(1);
      reload.resolve(json({ id: orgID, displayName: 'East operations', ownerUserId: owner }));
      await screen.findByRole('heading', { name: 'Outgoing Shares' });
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expectSavedRows(operation, applied);
      expect(notices.success).not.toHaveBeenCalled();
    },
  );
});
