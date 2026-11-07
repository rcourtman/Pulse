import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import type { LicenseStatus } from '@/api/license';
import type { SecurityStatusPresentationPolicy } from '@/types/config';
import { eventBus } from '@/stores/events';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { OrganizationBillingPanel } from '../OrganizationBillingPanel';

// Keep the panel, both API clients, HTTP parsing, event bus and capability /
// presentation stores connected. The neighboring lifetime tests replace the
// clients, so cannot detect a dropped selection, session preference or retry flag.
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const north = 'north/operations';
const south = 'south/support';
const storedToken = 'synthetic-unrelated-api-token';
const capabilityURL = '/api/license/runtime-capabilities';
const sources = ['license', 'organizations', 'members'] as const;
type Source = (typeof sources)[number];
const pathFor = (source: Source, org = north) =>
  ({
    license: '/api/license/status',
    organizations: '/api/orgs',
    members: `/api/orgs/${encodeURIComponent(org)}/members`,
  })[source];
const emailFor = (org: string) => `${org === north ? 'north' : 'south'}@example.test`;
const licenseFor = (org: string): LicenseStatus => ({
  valid: true,
  tier: 'cloud',
  email: emailFor(org),
  is_lifetime: false,
  days_remaining: 30,
  expires_at: '2027-01-01T00:00:00Z',
  features: [],
  max_guests: 17,
});
const dataFor = (source: Source, org: string) => {
  if (source === 'license') return licenseFor(org);
  if (source === 'organizations')
    return [north, south].map((id) => ({ id, displayName: id }));
  return Array.from({ length: org === north ? 3 : 1 }, (_, index) => ({
    userId: `synthetic-member-${index}`,
    role: 'viewer',
    addedAt: '2026-11-01T00:00:00Z',
  }));
};
const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
const settle = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
const fetchMock = vi.fn<typeof fetch>();
const billingReads = () => fetchMock.mock.calls.filter(([url]) => url !== capabilityURL);
let reply: (source: Source, org: string) => Promise<Response>;
let capabilityAvailable: boolean;
let capabilityFailed: boolean;

const mount = () => render(() => <OrganizationBillingPanel nodeUsage={5} guestUsage={2} />);
const switchOrg = (id: string) => {
  setOrgID(id);
  eventBus.emit('org_switched', id);
};
const setPolicy = (policy: Partial<SecurityStatusPresentationPolicy>) =>
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode: false,
      readOnly: false,
      hideCommercial: false,
      hideUpgrade: true,
      ...policy,
    },
  });
const accepted = async (org = north) => {
  await screen.findByText(emailFor(org));
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
};
function expectUnknown() {
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Unable to load billing and plan details.',
  );
  expect(screen.queryByText('Plan Tier')).not.toBeInTheDocument();
  expect(screen.queryByText('Not configured')).not.toBeInTheDocument();
  expect(screen.queryByText(emailFor(north))).not.toBeInTheDocument();
  expect(document.body).not.toHaveTextContent('SYNTHETIC_PRIVATE_FAILURE');
}
function expectBoundReads(org = north, calls = billingReads()) {
  expect(calls.map(([url]) => url)).toEqual(sources.map((source) => pathFor(source, org)));
  const signal = calls[0][1]?.signal;
  expect(signal).toBeInstanceOf(AbortSignal);
  for (const [, options] of calls) {
    expect(options).toMatchObject({
      signal,
      credentials: 'include',
      headers: { 'X-Pulse-Org-ID': org, Accept: 'application/json' },
    });
    expect(options?.method ?? 'GET').toBe('GET');
    expect(options?.headers).not.toHaveProperty('X-API-Token');
    expect(options).not.toHaveProperty('expectedOrgID');
    expect(options).not.toHaveProperty('retry');
    expect(options).not.toHaveProperty('preferSessionAuth');
  }
  expect(getOrgID()).toBe(org);
  expect(getApiToken()).toBe(storedToken);
}

beforeEach(async () => {
  fetchMock.mockReset();
  vi.clearAllMocks();
  vi.stubGlobal('fetch', fetchMock);
  sessionStorage.clear();
  localStorage.clear();
  clearAuth();
  setOrgID(north);
  setApiToken(storedToken);
  syncSessionPresentationPolicy(null);
  capabilityAvailable = true;
  capabilityFailed = false;
  reply = async (source, org) => json(dataFor(source, org));
  fetchMock.mockImplementation(async (input, options = {}) => {
    const url = String(input);
    if (url === capabilityURL) {
      if (capabilityFailed) throw new TypeError('Synthetic unavailable capability read');
      return json({ capabilities: capabilityAvailable ? ['multi_tenant'] : [], limits: [] });
    }
    const source = sources.find((candidate) =>
      candidate === 'members' ? /^\/api\/orgs\/[^/]+\/members$/.test(url) : url === pathFor(candidate),
    );
    if (!source) throw new Error(`Unexpected synthetic billing request: ${url}`);
    return reply(source, new Headers(options.headers).get('X-Pulse-Org-ID') ?? 'default');
  });
  await loadRuntimeCapabilities(true);
});
afterEach(() => {
  cleanup();
  syncSessionPresentationPolicy(null);
  clearAuth();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('organization billing through its real read transport', () => {
  it('consumes one complete session-bound snapshot without substituting the stored API token', async () => {
    mount();
    await accepted();
    expectBoundReads();
    expect(screen.getByText('Organizations').parentElement).toHaveTextContent('2');
    expect(screen.getByText('Members (Current Org)').parentElement).toHaveTextContent('3');
    expect(screen.getByText('5 / Not metered')).toBeInTheDocument();
    expect(screen.getByText('2 / 17')).toBeInTheDocument();
  });

  it.each(sources)('does not publish a partial snapshot while the %s body is still unread', async (held) => {
    const body = deferred<string>();
    reply = async (source, org) => {
      const response = json(dataFor(source, org));
      if (source === held) vi.spyOn(response, 'text').mockReturnValueOnce(body.promise);
      return response;
    };
    mount();
    await waitFor(() => expect(billingReads()).toHaveLength(3));
    await settle();
    expect(screen.getByLabelText('Loading organization billing')).toBeInTheDocument();
    expect(screen.queryByText('Plan Tier')).not.toBeInTheDocument();
    expect(screen.queryByText(emailFor(north))).not.toBeInTheDocument();
    body.resolve(JSON.stringify(dataFor(held, north)));
    await accepted();
    expectBoundReads();
  });

  it.each(
    sources.flatMap((source) => [400, 403, 429, 503].map((status) => ({ source, status }))),
  )('keeps HTTP $status from $source unknown without retry or workspace fallback', async ({ source, status }) => {
    reply = async (candidate, org) =>
      candidate === source
        ? json(
            {
              error: status === 400 ? 'invalid_org' : 'default_org_access_denied',
              detail: 'SYNTHETIC_PRIVATE_FAILURE',
            },
            status,
            { 'X-CSRF-Token': 'synthetic-refreshed-csrf', 'Retry-After': '0.001' },
          )
        : json(dataFor(candidate, org));
    mount();
    await screen.findByRole('alert');
    await settle();
    expectUnknown();
    expectBoundReads();
    for (const [, options] of billingReads()) expect(options?.signal?.aborted).toBe(true);
  });

  it.each(sources)('does not turn malformed %s JSON into a free plan or empty count', async (broken) => {
    reply = async (source, org) =>
      source === broken
        ? new Response('{"SYNTHETIC_PRIVATE_FAILURE":', { status: 200 })
        : json(dataFor(source, org));
    mount();
    await screen.findByRole('alert');
    expectUnknown();
    expectBoundReads();
  });

  it('aborts unread siblings on failure and accepts genuine free/zero data only on explicit reload', async () => {
    const sibling = deferred<void>();
    reply = async (source, org) => {
      if (source === 'license') return json({ error: 'SYNTHETIC_PRIVATE_FAILURE' }, 503);
      await sibling.promise;
      return json(dataFor(source, org));
    };
    mount();
    await screen.findByRole('alert');
    expectUnknown();
    expectBoundReads();
    for (const [, options] of billingReads()) expect(options?.signal?.aborted).toBe(true);
    // These deliberately ignore abort. Their late results cannot publish counts.
    sibling.resolve();
    await settle();
    expectUnknown();
    expect(billingReads()).toHaveLength(3);
    const next = deferred<Response>();
    reply = async (source) =>
      source === 'license'
        ? next.promise
        : json([]);
    const reload = screen.getByRole('button', { name: 'Reload billing' });
    fireEvent.click(reload);
    fireEvent.click(reload);
    await waitFor(() => expect(billingReads()).toHaveLength(6));
    next.resolve(json({ ...licenseFor(north), tier: 'free', valid: false, email: undefined }));
    await screen.findByText('Not configured');
    expect(screen.getByText('Community')).toBeInTheDocument();
    expect(screen.getByText('Organizations').parentElement).toHaveTextContent('0');
    expect(screen.getByText('Members (Current Org)').parentElement).toHaveTextContent('0');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expectBoundReads(north, billingReads().slice(3));
  });

  it('rejects an old snapshot even after selecting the same workspace again', async () => {
    const first = deferred<Response>();
    let held = false;
    reply = async (source, org) => {
      if (source === 'license' && !held) {
        held = true;
        return first.promise;
      }
      return json(dataFor(source, org));
    };
    mount();
    await waitFor(() => expect(billingReads()).toHaveLength(3));
    const oldSignal = billingReads()[0][1]?.signal;
    switchOrg(south);
    await accepted(south);
    expect(screen.getByText('Members (Current Org)').parentElement).toHaveTextContent('1');
    expect(screen.queryByText(emailFor(north))).not.toBeInTheDocument();
    switchOrg(north);
    await accepted();
    expect(oldSignal?.aborted).toBe(true);
    first.resolve(json({ ...licenseFor(north), email: 'obsolete@example.test' }));
    await settle();
    expect(screen.queryByText('obsolete@example.test')).not.toBeInTheDocument();
    expect(screen.getByText(emailFor(north))).toBeInTheDocument();
    expect(screen.getByText('Members (Current Org)').parentElement).toHaveTextContent('3');
    expectBoundReads(north, billingReads().slice(-3));
  });

  it('checks the actual selection before accepting a body when the switch event has not arrived', async () => {
    const pending = deferred<Response>();
    reply = async (source, org) =>
      source === 'license' && org === north ? pending.promise : json(dataFor(source, org));
    mount();
    await waitFor(() => expect(billingReads()).toHaveLength(3));
    setOrgID(south);
    pending.resolve(json(licenseFor(north)));
    await accepted(south);
    expect(screen.queryByText(emailFor(north))).not.toBeInTheDocument();
    expectBoundReads(south, billingReads().slice(-3));
  });

  it.each(['capability', 'capability-read-failure', 'demo'] as const)(
    'withdraws the pending snapshot on %s and reads afresh only when available again',
    async (withdrawal) => {
      const pending = deferred<Response>();
      reply = async (source, org) =>
        source === 'license' ? pending.promise : json(dataFor(source, org));
      mount();
      await waitFor(() => expect(billingReads()).toHaveLength(3));
      const oldSignal = billingReads()[0][1]?.signal;
      if (withdrawal === 'demo') setPolicy({ demoMode: true });
      else {
        capabilityAvailable = false;
        capabilityFailed = withdrawal === 'capability-read-failure';
        await loadRuntimeCapabilities(true);
      }
      await screen.findByText('Organization settings are not available on this server.');
      expect(oldSignal?.aborted).toBe(true);
      pending.resolve(json({ ...licenseFor(north), email: 'obsolete@example.test' }));
      await settle();
      expect(billingReads()).toHaveLength(3);
      expect(screen.queryByText('obsolete@example.test')).not.toBeInTheDocument();
      reply = async (source, org) => json(dataFor(source, org));
      capabilityAvailable = true;
      capabilityFailed = false;
      if (withdrawal === 'demo') syncSessionPresentationPolicy(null);
      else await loadRuntimeCapabilities(true);
      await accepted();
      expect(screen.queryByText('obsolete@example.test')).not.toBeInTheDocument();
      expect(billingReads()).toHaveLength(6);
      expectBoundReads(north, billingReads().slice(3));
    },
  );

  it('does not refetch or hide readable billing on an unchanged capability or read-only policy refresh', async () => {
    const pending = deferred<Response>();
    reply = async (source, org) =>
      source === 'license' ? pending.promise : json(dataFor(source, org));
    mount();
    await waitFor(() => expect(billingReads()).toHaveLength(3));
    const signal = billingReads()[0][1]?.signal;
    await loadRuntimeCapabilities(true);
    setPolicy({ readOnly: true });
    expect(signal?.aborted).toBe(false);
    expect(billingReads()).toHaveLength(3);
    pending.resolve(json(licenseFor(north)));
    await accepted();
    expectBoundReads();
  });

  it('detaches a disposed consumer without accepting late data or starting more billing reads', async () => {
    const pending = deferred<Response>();
    reply = async (source, org) =>
      source === 'license' ? pending.promise : json(dataFor(source, org));
    const view = mount();
    await waitFor(() => expect(billingReads()).toHaveLength(3));
    const signal = billingReads()[0][1]?.signal;
    view.unmount();
    expect(signal?.aborted).toBe(true);
    eventBus.emit('organizations_changed');
    switchOrg(south);
    pending.resolve(json(licenseFor(north)));
    await settle();
    expect(billingReads()).toHaveLength(3);
    expect(screen.queryByText(emailFor(north))).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
