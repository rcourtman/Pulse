import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
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
const deferred = () => {
  let resolve!: () => void;
  const promise = new Promise<void>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
};
const fetchMock = vi.fn<typeof fetch>();
let readGate: ReturnType<typeof deferred> | undefined;
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
const reader = () => screen.getByRole('region', { name: 'Organization sharing', exact: true });
const warning = () => screen.getByRole('alert', { name: 'Organization sharing unavailable' });
const reload = () => screen.getByRole('button', { name: 'Reload sharing' });
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
      expect(stable).toHaveAttribute('aria-busy', 'true');
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
      expect(stable).toHaveAttribute('aria-busy', 'false');
      expect(reads()).toHaveLength(10);
      expect(writes()).toHaveLength(0);
      expectBoundReads();
    },
  );

  it('retains the same busy reader through a failed manual read', async () => {
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
    expect(stable).toHaveAttribute('aria-busy', 'false');
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
      readGate = deferred();
      reload().focus();
      fireEvent.click(reload());
      expect(reader()).toHaveFocus();
      expect(reader()).toHaveAttribute('aria-busy', 'true');
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
      expect(reads()).toHaveLength(10);
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
