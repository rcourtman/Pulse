import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { OrganizationSharingPanel } from '../OrganizationSharingPanel';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import type { IncomingOrganizationShare, Organization, OrganizationShare } from '@/api/orgs';

// Keep the panel, state, event bus, capability owner, OrgsAPI and session client
// connected. Only resource inventory, synthetic HTTP and notice edges are supplied.
vi.mock('@/hooks/useResources', () => ({ useResources: () => ({ resources: () => [] }) }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const owner = 'owner@example.test';
const orgID = 'operations/east?realm=lab#saved';
const peerID = 'operations/west?realm=lab#saved';
const csrf = 'synthetic-sharing-csrf';
const token = 'synthetic-unrelated-api-token';
const org: Organization = { id: orgID, displayName: 'Operations East', ownerUserId: owner };
const peer: Organization = { id: peerID, displayName: 'Operations West', ownerUserId: owner };
const base = (id = orgID) => `/api/orgs/${encodeURIComponent(id)}`;
const share: OrganizationShare = {
  id: 'outgoing/one?opaque="::]',
  resourceType: 'vm',
  resourceId: 'opaque::vm/101',
  resourceName: 'Saved VM',
  targetOrgId: peerID,
  accessRole: 'viewer',
  status: 'pending',
  createdAt: '2026-11-01T00:00:00Z',
  createdBy: owner,
};
const incomingShare: IncomingOrganizationShare = {
  ...share,
  id: 'incoming/one?opaque="::]',
  resourceId: 'opaque::vm/202',
  targetOrgId: orgID,
  sourceOrgId: peerID,
  sourceOrgName: peer.displayName,
};
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
const deferred = <T = void,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
};
const fetchMock = vi.fn<typeof fetch>();
let readGate: ReturnType<typeof deferred<void>> | undefined;
let outgoingGate: Promise<Response> | undefined;
let failRead: boolean;
let enabled: boolean;
let outgoing: OrganizationShare[];
let incoming: IncomingOrganizationShare[];
const writes = () => fetchMock.mock.calls.filter(([, init]) => (init?.method ?? 'GET') !== 'GET');
const reads = () =>
  fetchMock.mock.calls.filter(
    ([url, init]) =>
      (init?.method ?? 'GET') === 'GET' && (url === '/api/orgs' || String(url).startsWith(base())),
  );
const reader = () => screen.getByRole('region', { name: 'Organization sharing' });
const results = () => screen.getByRole('group', { name: 'Saved shares' });
const warning = () => screen.getByRole('alert', { name: 'Organization sharing unavailable' });
const reload = () => screen.getByRole('button', { name: 'Reload sharing' });
const restart = () => screen.getByRole('button', { name: 'Restart sharing check' });
const settle = () => new Promise<void>((resolve) => setTimeout(resolve, 15));
const policy = (kind: 'ordinary' | 'read-only' | 'hidden') =>
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode: kind === 'hidden',
      readOnly: kind !== 'ordinary',
      hideCommercial: false,
      hideUpgrade: true,
    },
  });
function open() {
  const [user, setUser] = createSignal(owner);
  const view = render(() => (
    <>
      <button type="button">Outside sharing</button>
      <OrganizationSharingPanel currentUser={user()} />
    </>
  ));
  return { ...view, setUser };
}
async function unavailable() {
  failRead = true;
  const view = open();
  await screen.findByRole('alert');
  return view;
}
async function saved() {
  await screen.findByRole('heading', { name: 'Outgoing Shares' });
  await settle();
}
function expectBoundReads() {
  for (const [, init] of reads()) {
    expect(init).toMatchObject({ credentials: 'include', signal: expect.any(AbortSignal) });
    const headers = new Headers(init?.headers);
    expect(headers.get('X-Pulse-Org-ID')).toBe(orgID);
    expect(headers.has('X-API-Token')).toBe(false);
  }
  expect(getOrgID()).toBe(orgID);
  expect(getApiToken()).toBe(token);
}

beforeEach(async () => {
  fetchMock.mockReset();
  clearAuth();
  setOrgID(orgID);
  setApiToken(token);
  document.cookie = `pulse_csrf=${csrf}; Path=/`;
  policy('ordinary');
  readGate = undefined;
  outgoingGate = undefined;
  failRead = false;
  enabled = true;
  outgoing = [share];
  incoming = [incomingShare];
  vi.stubGlobal(
    'confirm',
    vi.fn(() => true),
  );
  vi.stubGlobal('fetch', fetchMock);
  fetchMock.mockImplementation(async (input, init = {}) => {
    const url = String(input);
    if (url === '/api/license/runtime-capabilities')
      return json({ capabilities: enabled ? ['multi_tenant'] : [], limits: [] });
    if ((init.method ?? 'GET') !== 'GET') {
      if (init.method === 'DELETE') outgoing = [];
      else incoming = [{ ...incomingShare, status: 'accepted' }];
      return json({ error: 'Synthetic response unavailable after saved change' }, 503);
    }
    if (url === `${base()}/shares` && outgoingGate) return outgoingGate;
    if (readGate) await readGate.promise;
    if (url === '/api/orgs') return json([org, peer]);
    for (const organization of [org, peer]) {
      if (url === base(organization.id))
        return failRead ? json({ error: 'Synthetic read unavailable' }, 503) : json(organization);
      if (url === `${base(organization.id)}/members`)
        return json([{ userId: owner, role: 'owner', addedAt: '' }]);
      if (url === `${base(organization.id)}/shares`)
        return json(organization.id === orgID ? outgoing : []);
      if (url === `${base(organization.id)}/shares/incoming`)
        return json(organization.id === orgID ? incoming : []);
    }
    throw new Error(`Unexpected synthetic read: ${url}`);
  });
  await loadRuntimeCapabilities(true);
});
afterEach(async () => {
  cleanup();
  readGate?.resolve();
  await settle();
  clearAuth();
  setOrgID(null);
  syncSessionPresentationPolicy(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
});

describe('manual sharing rereads retain keyboard context through real transport', () => {
  it.each(['focused button', 'unfocused activation'])(
    'keeps the stable reader focused through pending and successful %s',
    async (activation) => {
      await unavailable();
      const stable = reader();
      const button = reload();
      readGate = deferred();
      failRead = false;
      if (activation === 'focused button') button.focus();
      fireEvent.click(button);
      expect(stable).toHaveFocus();
      expect(results()).toHaveAttribute('aria-busy', 'true');
      expect(results()).not.toContainElement(restart());
      expect(stable).toHaveAttribute('tabindex', '-1');
      expect(button).not.toBeInTheDocument();
      expect(
        screen.getByRole('status', { name: 'Loading organization sharing' }),
      ).toBeInTheDocument();
      await settle();
      expect(stable).toHaveFocus();
      readGate.resolve();
      await saved();
      expect(reader()).toBe(stable);
      expect(stable).toHaveFocus();
      expect(results()).toHaveAttribute('aria-busy', 'false');
      expect(reads()).toHaveLength(10);
      expect(writes()).toHaveLength(0);
      expectBoundReads();
    },
  );

  it('replaces the whole pending scoped read and ignores a late outgoing result', async () => {
    const firstOutgoing = deferred<Response>();
    outgoingGate = firstOutgoing.promise;
    open();
    const stable = reader();
    const action = restart();
    expect(action).toHaveAccessibleDescription(
      `Checking saved shares for ${orgID}. Restarting does not repeat a sharing change.`,
    );
    expect(action).toBeEnabled();
    expect(results()).toHaveAttribute('aria-busy', 'true');
    expect(results()).not.toContainElement(action);
    expect(document.body).toHaveFocus();

    await waitFor(() => expect(reads()).toHaveLength(5));
    outgoingGate = undefined;
    readGate = deferred();
    fireEvent.click(action);
    expect(stable).toHaveFocus();
    await waitFor(() => expect(reads()).toHaveLength(10));
    expect(
      reads()
        .slice(0, 5)
        .every(([, init]) => init?.signal?.aborted),
    ).toBe(true);
    expect(
      reads()
        .slice(5)
        .every(([, init]) => init?.signal?.aborted === false),
    ).toBe(true);

    firstOutgoing.resolve(json([{ ...share, resourceName: 'Retired result' }]));
    await settle();
    expect(results()).toHaveAttribute('aria-busy', 'true');
    expect(screen.queryByText('Retired result')).not.toBeInTheDocument();
    expect(within(results()).queryByRole('heading', { name: 'Outgoing Shares' })).toBeNull();

    const outside = screen.getByRole('button', { name: 'Outside sharing' });
    outside.focus();
    readGate.resolve();
    await saved();
    expect(outside).toHaveFocus();
    expect(screen.queryByRole('button', { name: 'Restart sharing check' })).toBeNull();
    expect(screen.getByRole('heading', { name: 'Incoming Shares' })).toBeVisible();
    expect(screen.queryByText('Retired result')).not.toBeInTheDocument();
    expect(results()).toHaveAttribute('aria-busy', 'false');
    expect(writes()).toHaveLength(0);
    expectBoundReads();
  });

  it('retains the same reader through a failed manual read', async () => {
    await unavailable();
    const stable = reader();
    readGate = deferred();
    reload().focus();
    fireEvent.click(reload());
    expect(stable).toHaveFocus();
    readGate.resolve();
    await screen.findByRole('alert');
    await settle();
    expect(reader()).toBe(stable);
    expect(stable).toHaveFocus();
    expect(results()).toHaveAttribute('aria-busy', 'false');
    expect(reload()).toBeEnabled();
    expect(writes()).toHaveLength(0);
    expectBoundReads();
  });

  it.each(['outside', 'outside then blur'])(
    'does not steal focus after deliberate departure: %s',
    async (departure) => {
      await unavailable();
      readGate = deferred();
      failRead = false;
      reload().focus();
      fireEvent.click(reload());
      const outside = screen.getByRole('button', { name: 'Outside sharing' });
      outside.focus();
      if (departure === 'outside then blur') outside.blur();
      readGate.resolve();
      await saved();
      expect(departure === 'outside' ? outside : document.body).toHaveFocus();
      expect(writes()).toHaveLength(0);
    },
  );

  it.each(['remove outgoing', 'accept incoming'])(
    'keeps confirmed-context %s uncertainty focused, then checks saved state without replay',
    async (operation) => {
      open();
      await saved();
      const action = screen.getByRole('button', {
        name:
          operation === 'remove outgoing'
            ? /^Remove share of vm:opaque::vm\/101/
            : /^Accept share of vm:opaque::vm\/202/,
      });
      action.focus();
      fireEvent.click(action);
      if (operation === 'remove outgoing') {
        const dialog = screen.getByRole('dialog', { name: 'Remove outgoing share?' });
        fireEvent.click(within(dialog).getByRole('button', { name: 'Remove share' }));
      }
      await waitFor(() => expect(warning()).toHaveFocus());
      expect(warning()).toHaveTextContent('The last sharing change could not be confirmed.');
      expect(writes()).toHaveLength(1);
      const [url, init] = writes()[0];
      const id = operation === 'remove outgoing' ? share.id : incomingShare.id;
      expect(url).toBe(
        `${base()}/shares/${operation === 'remove outgoing' ? '' : 'incoming/'}${encodeURIComponent(id)}${operation === 'remove outgoing' ? '' : '/accept'}`,
      );
      expect(init?.method).toBe(operation === 'remove outgoing' ? 'DELETE' : 'POST');
      expect(new Headers(init?.headers).get('X-CSRF-Token')).toBe(csrf);
      const firstRecovery = deferred();
      readGate = firstRecovery;
      reload().focus();
      fireEvent.click(reload());
      expect(reader()).toHaveFocus();
      expect(results()).toHaveAttribute('aria-busy', 'true');
      await waitFor(() => expect(reads()).toHaveLength(10));
      readGate = deferred();
      fireEvent.click(restart());
      await waitFor(() => expect(reads()).toHaveLength(15));
      firstRecovery.resolve();
      await settle();
      expect(results()).toHaveAttribute('aria-busy', 'true');
      readGate.resolve();
      await saved();
      expect(reader()).toHaveFocus();
      if (operation === 'remove outgoing')
        expect(
          screen.queryByRole('button', { name: /^Remove share of vm:opaque::vm\/101/ }),
        ).not.toBeInTheDocument();
      else
        expect(
          screen.getByRole('button', { name: /^Remove share of vm:opaque::vm\/202/ }),
        ).toBeEnabled();
      expect(reads()).toHaveLength(15);
      expect(writes()).toHaveLength(1);
      expectBoundReads();
    },
  );

  it('never focuses an untouched initial or background read', async () => {
    open();
    await saved();
    expect(document.body).toHaveFocus();
    eventBus.emit('organizations_changed');
    await saved();
    expect(document.body).toHaveFocus();
    expect(writes()).toHaveLength(0);
  });

  it.each(['organization', 'principal', 'capability', 'read-only', 'hidden', 'unmount'])(
    'does not revive manual intent after a %s withdrawal and late completion',
    async (kind) => {
      const view = await unavailable();
      readGate = deferred();
      reload().focus();
      fireEvent.click(reload());
      expect(reader()).toHaveFocus();
      screen.getByRole('button', { name: 'Outside sharing' }).focus();
      if (kind === 'organization') {
        setOrgID(peerID);
        eventBus.emit('org_switched', peerID);
        setOrgID(orgID);
        eventBus.emit('org_switched', orgID);
      }
      if (kind === 'principal') {
        view.setUser('other@example.test');
        view.setUser(owner);
      }
      if (kind === 'capability') {
        enabled = false;
        await loadRuntimeCapabilities(true);
        enabled = true;
        await loadRuntimeCapabilities(true);
      }
      if (kind === 'read-only' || kind === 'hidden') {
        policy(kind);
        policy('ordinary');
      }
      if (kind === 'unmount') view.unmount();
      failRead = false;
      readGate.resolve();
      if (kind !== 'unmount') await saved();
      else await settle();
      expect(
        screen.queryByRole('button', { name: 'Outside sharing' }) ?? document.body,
      ).toHaveFocus();
      expect(writes()).toHaveLength(0);
    },
  );
});
